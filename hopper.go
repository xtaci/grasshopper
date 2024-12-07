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
	// Listener defines a server which will be waiting to accept incoming connections
	Listener struct {
		logger     *log.Logger   // logger
		crypterIn  BlockCrypt    // crypter for incoming packets
		crypterOut BlockCrypt    // crypter for outgoing packets
		conn       *net.UDPConn  // the underlying packet connection
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
		logger.Println("SetReadBuffer error:", err)
	}

	err = conn.SetWriteBuffer(sockbuf)
	if err != nil {
		logger.Println("SetWriteBuffer error:", err)
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

// Start the listener
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
	} else if l.crypterIn == nil {
		packetOk = true
	}

	if packetOk {
		l.incomingConnectionsLock.RLock()
		conn, ok := l.incomingConnections[raddr.String()]
		l.incomingConnectionsLock.RUnlock()

		// encrypt or re-encrypt the packet if crypterOut is set(with new nonce)
		if l.crypterOut != nil {
			dataOut := make([]byte, len(data)+nonceSize)
			copy(dataOut[nonceSize:], data)
			_, _ = io.ReadFull(rand.Reader, dataOut[:nonceSize])
			l.crypterOut.Encrypt(dataOut, dataOut)
			data = dataOut
		}

		if ok { // existing connection
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout))
		} else { // new connection
			// dial target
			conn, err := net.Dial("udp", l.nextHop)
			if err != nil {
				l.logger.Println("dial target error:", err)
				return
			}

			// log new connection
			log.Printf("new connection from %s to %s", raddr.String(), l.nextHop)

			// the context is the address of incoming packet
			// register the address
			ctx := raddr
			l.incomingConnectionsLock.Lock()
			l.incomingConnections[raddr.String()] = conn
			l.incomingConnectionsLock.Unlock()

			// watch the connection
			l.watcher.ReadTimeout(ctx, conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout)) // write needs not to specify the context(where the packet from)
		}
	}
}

// packet switcher from clients to targets
func (l *Listener) switcher() {
	// use listener connection as the context to identify the connection

	for {
		results, err := l.watcher.WaitIO()
		if err != nil {
			l.logger.Println("wait io error:", err)
			return
		}

		for _, res := range results {
			switch res.Operation {
			case gaio.OpWrite:
				// write to target complete
				if res.Error != nil {
					l.logger.Println("gaio write error: %+v", res)
					l.cleanClient(res.Conn.RemoteAddr())
					continue
				}

			case gaio.OpRead:
				if res.Error != nil { // any error discontinues the connection
					l.logger.Printf("gaio read error: %+v", res)
					l.cleanClient(res.Conn.RemoteAddr())
					continue
				}

				// received data from the next hop
				dataFromTarget := res.Buffer[:res.Size]

				// decrypt data from target if crypterOut is set
				if l.crypterOut != nil {
					l.crypterOut.Decrypt(dataFromTarget, dataFromTarget)
					dataFromTarget = dataFromTarget[nonceSize:]
				}

				// re-encrypt data to client if crypterIn is set
				if l.crypterIn != nil {
					data := make([]byte, len(dataFromTarget)+nonceSize)
					copy(data[nonceSize:], dataFromTarget)
					_, _ = io.ReadFull(rand.Reader, data[:nonceSize])
					l.crypterIn.Encrypt(data, data)
					dataFromTarget = data
				}

				// forward data to client
				l.conn.WriteTo(dataFromTarget, res.Context.(net.Addr))

				// fire another read-request to the connection
				l.watcher.ReadTimeout(res.Context, res.Conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			}
		}
	}
}

func (l *Listener) cleanClient(raddr net.Addr) {
	l.incomingConnectionsLock.Lock()
	delete(l.incomingConnections, raddr.String())
	l.incomingConnectionsLock.Unlock()
}

func (l *Listener) Close() error {
	l.dieOnce.Do(func() {
		close(l.die)
		l.conn.Close()
		l.watcher.Close()
	})
	return nil
}
