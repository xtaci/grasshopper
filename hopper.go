// The MIT License (MIT)
//
// Copyright (c) 2024 xtaci
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package grasshopper

import (
	"crypto/rand"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/xtaci/gaio"
)

const (
	// 4-bytes extra UDP nonce for each packet
	nonceSize = 4

	// maximum packet size
	mtuLimit = 1450
)

type (
	// Listener defines a server which will be waiting to accept incoming UDP connections
	Listener struct {
		logger     *log.Logger   // logger
		crypterIn  BlockCrypt    // crypter for incoming packets
		crypterOut BlockCrypt    // crypter for outgoing packets
		conn       *net.UDPConn  // the socket to listen on
		timeout    time.Duration // session timeout
		sockbuf    int           // socket buffer size

		// connection pairing
		nextHop                 string              // the outgoing address
		watcher                 *gaio.Watcher       // the watcher
		incomingConnections     map[string]net.Conn // client address -> {connection to next hop}
		incomingConnectionsLock sync.RWMutex

		die     chan struct{} // notify the listener has closed
		dieOnce sync.Once
	}
)

// ListenWithOptions creates a new listener with options
// laddr: the listening address
// target: the next hop address
// sockbuf: the socket buffer size
// timeout: the session timeout
// crypterIn: the crypter for incoming packets
// crypterOut: the crypter for outgoing packets
// logger: the logger
func ListenWithOptions(laddr string, target string, sockbuf int, timeout time.Duration, crypterIn BlockCrypt, crypterOut BlockCrypt, logger *log.Logger) (*Listener, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = conn.SetReadBuffer(sockbuf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = conn.SetWriteBuffer(sockbuf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// initiate backend switcher
	watcher, err := gaio.NewWatcher()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	l := new(Listener)
	l.logger = logger
	l.incomingConnections = make(map[string]net.Conn)
	l.conn = conn
	l.nextHop = target
	l.die = make(chan struct{})
	l.crypterIn = crypterIn
	l.crypterOut = crypterOut
	l.watcher = watcher
	l.timeout = timeout
	return l, nil
}

// Start the listener and wait until it's closed, it returns when the socket is closed.
func (l *Listener) Start() {
	go l.switcher()

	for {
		buf := make([]byte, mtuLimit)
		if n, from, err := l.conn.ReadFrom(buf); err == nil {
			l.packetIn(buf[:n], from)
		} else {
			l.logger.Fatal("Start:", err)
			return
		}
	}
}

// packetIn handles incoming packets
func (l *Listener) packetIn(data []byte, raddr net.Addr) {
	// decrypt incoming packet if crypterIn is set
	packetOk := false
	if l.crypterIn != nil && len(data) >= nonceSize {
		l.crypterIn.Decrypt(data, data)
		data = data[nonceSize:]
		packetOk = true
		// fmt.Println(unsafe.Pointer(l), "decrypted listener in", string(data))
	} else if l.crypterIn == nil {
		packetOk = true
	}

	if packetOk {
		// encrypt or re-encrypt the packet if crypterOut is set(with new nonce)
		if l.crypterOut != nil {
			dataOut := make([]byte, len(data)+nonceSize)
			copy(dataOut[nonceSize:], data)
			_, _ = io.ReadFull(rand.Reader, dataOut[:nonceSize])
			l.crypterOut.Encrypt(dataOut, dataOut)
			//fmt.Println(unsafe.Pointer(l), "encrypted listener out", string(dataOut))
			data = dataOut
		}

		// load the connection from the incoming connections
		l.incomingConnectionsLock.RLock()
		conn, ok := l.incomingConnections[raddr.String()]
		l.incomingConnectionsLock.RUnlock()

		if ok { // existing connection
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout))
		} else { // new connection
			// dial target
			conn, err := net.Dial("udp", l.nextHop)
			if err != nil {
				l.logger.Println("dial target error:", err)
				return
			}

			// add the connection to the incoming connections
			l.addClient(raddr, conn)
			// log new connection
			log.Printf("new connection from %s to %s", raddr.String(), l.nextHop)

			// watch the connection
			// the context is the address of incoming packet
			ctx := raddr
			l.watcher.ReadTimeout(ctx, conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout)) // write needs not to specify the context(where the packet from)
		}
	}
}

// switcher handles the proxy connections to the next hop.
// It acts like a proxy multiplexer.
func (l *Listener) switcher() {
	for {
		results, err := l.watcher.WaitIO()
		if err != nil {
			l.logger.Println("wait io error:", err)
			return
		}

		for _, res := range results {
			switch res.Operation {
			case gaio.OpWrite:
				// done writting to proxy connection.
				if res.Error != nil {
					l.logger.Printf("[switcher]write error: %#v", res)
					l.removeClient(res.Conn.RemoteAddr())
					continue
				}

			case gaio.OpRead:
				// any read error from the proxy connection cleans the other side(client).
				if res.Error != nil {
					l.logger.Printf("[switcher]read error: %#v", res)
					l.removeClient(res.Conn.RemoteAddr())
					continue
				}

				// received data from the proxy connection.
				dataFromProxy := res.Buffer[:res.Size]

				// decrypt data from the proxy connection if crypterOut is set.
				if l.crypterOut != nil {
					l.crypterOut.Decrypt(dataFromProxy, dataFromProxy)
					dataFromProxy = dataFromProxy[nonceSize:]
					//fmt.Println(unsafe.Pointer(l), "proxy crypterOut", string(dataFromProxy))
				}

				// re-encrypt data if crypterIn is set.
				if l.crypterIn != nil {
					data := make([]byte, len(dataFromProxy)+nonceSize)
					copy(data[nonceSize:], dataFromProxy)
					_, _ = io.ReadFull(rand.Reader, data[:nonceSize])
					l.crypterIn.Encrypt(data, data)
					dataFromProxy = data
					//fmt.Println(unsafe.Pointer(l), "proxy crypterIn", string(dataFromProxy))
				}

				// forward the data to client via the listener.
				l.conn.WriteTo(dataFromProxy, res.Context.(net.Addr))

				// fire next read-request to the proxy connection.
				l.watcher.ReadTimeout(res.Context, res.Conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			}
		}
	}
}

// addClient adds the client to the incoming connections map.
func (l *Listener) addClient(raddr net.Addr, conn net.Conn) {
	l.incomingConnectionsLock.Lock()
	l.incomingConnections[raddr.String()] = conn
	l.incomingConnectionsLock.Unlock()
}

// removeClient removes the client from the incoming connections map.
func (l *Listener) removeClient(raddr net.Addr) {
	l.incomingConnectionsLock.Lock()
	delete(l.incomingConnections, raddr.String())
	l.incomingConnectionsLock.Unlock()
}

// Close the listener
func (l *Listener) Close() error {
	l.dieOnce.Do(func() {
		close(l.die)
		l.conn.Close()
		l.watcher.Close()
	})
	return nil
}
