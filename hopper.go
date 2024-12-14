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
	"bytes"
	"crypto/md5"
	"crypto/rand"
	mrand "math/rand"

	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/xtaci/gaio"
)

const (
	// nonceSize defines the size of the additional nonce (8 bytes) added to each UDP packet.
	// Based on shannon's theory of cryptography, this nonce introduces 2^(8*8) confusion bits.
	nonceSize   = 8
	nonceOffset = 0

	// checksumSize defines the size of the checksum (8 bytes) added to each UDP packet.
	// The checksum and nonce together plays the role of an authentication tag.
	checksumSize   = 8
	checksumOffset = nonceSize

	// headerSize defines the size of the header (nonce + checksum) added to each UDP packet.
	// | nonce(8 bytes) | checksum(8 bytes) | data |
	headerSize = nonceSize + checksumSize

	// mtuLimit specifies the maximum transmission unit (MTU) size for a packet.
	mtuLimit = 1500
)

var (
	errNoNextHop = errors.New("no next hop provided")
	errChecksum  = errors.New("checksum mismatch")
)

type (
	// OnClientInCallback is a callback function that processes incoming packets from clients
	OnClientInCallback func(client net.Addr, in []byte) (out []byte)

	// OnNextHopInCallback is a callback function that processes incoming packets from the next hop.
	OnNextHopInCallback func(hop net.Addr, client net.Addr, in []byte) (out []byte)

	// Listener represents a UDP server that listens for incoming connections and relays them to the next hop.
	Listener struct {
		logger     *log.Logger // logger
		crypterIn  BlockCrypt  // crypter for incoming packets
		crypterOut BlockCrypt  // crypter for outgoing packets

		// callbacks for bidirectional communication
		onClientIn  OnClientInCallback  // callback on incoming packets from clients
		onNextHopIn OnNextHopInCallback // callback on incoming packets from next hops

		conn    *net.UDPConn  // the socket to listen on
		timeout time.Duration // session timeout
		sockbuf int           // socket buffer size for the `conn`

		// connection pairing
		nextHops                []string            // the outgoing addresses, the switcher will forward packets to one of them randomly.
		watcher                 *gaio.Watcher       // I/O watcher for asynchronous operations.
		incomingConnections     map[string]net.Conn // client address -> {connection to next hop}
		incomingConnectionsLock sync.Mutex

		die     chan struct{} // Channel to signal listener termination.
		dieOnce sync.Once     // Ensures the close operation is executed only once.
	}
)

func init() {
	mrand.Seed(time.Now().UnixNano())
}

// ListenWithOptions initializes a new Listener with the provided options.
// Parameters:
// - laddr: Address to listen on.
// - nexthop: Addresses to forward packets to.
// - sockbuf: Socket buffer size in bytes.
// - timeout: Session timeout duration.
// - crypterIn: Cryptographic handler for decrypting incoming packets.
// - crypterOut: Cryptographic handler for encrypting outgoing packets.
// - pre: Prerouting function for processing incoming packets.
// - post: Postrouting function before forwarding packets to the next hop.
// - logger: Logger instance for logging.
func ListenWithOptions(laddr string,
	nexthops []string,
	sockbuf int,
	timeout time.Duration,
	crypterIn BlockCrypt, crypterOut BlockCrypt,
	onClientIn OnClientInCallback,
	onNextHopIn OnNextHopInCallback,
	logger *log.Logger) (*Listener, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(nexthops) == 0 {
		return nil, errors.WithStack(errNoNextHop)
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
	l.nextHops = nexthops
	l.die = make(chan struct{})
	l.crypterIn = crypterIn
	l.crypterOut = crypterOut
	l.onClientIn = onClientIn
	l.onNextHopIn = onNextHopIn
	l.watcher = watcher
	l.timeout = timeout
	return l, nil
}

// Start begins the listener loop, handling incoming packets and forwarding them.
// It blocks until the listener is closed or encounters an error.
func (l *Listener) Start() {
	go l.switcher()

	for {
		buf := make([]byte, mtuLimit)
		if n, from, err := l.conn.ReadFrom(buf); err == nil {
			l.clientIn(buf[:n], from)
		} else {
			l.logger.Fatal("Start:", err)
			return
		}
	}
}

// clientIn processes incoming packets and forwards them to the next hop.
func (l *Listener) clientIn(data []byte, raddr net.Addr) {
	// decrypt the packet if crypterIn is set
	data, err := decryptPacket(l.crypterIn, data)
	if err != nil {
		l.logger.Println("[clientIn]decryptPacket:", err)
		return
	}

	// onClientIn callback
	if l.onClientIn != nil {
		data = l.onClientIn(raddr, data)
		// blackhole the packet if the callback returns nil
		if data == nil {
			return
		}
	}

	// encrypt or re-encrypt the packet if crypterOut is set(with new nonce)
	data = encryptPacket(l.crypterOut, data)

	// load the connection from the incoming connections
	l.incomingConnectionsLock.Lock()
	conn, ok := l.incomingConnections[raddr.String()]
	l.incomingConnectionsLock.Unlock()

	ctx := raddr
	if ok { // existing connection
		l.watcher.WriteTimeout(ctx, conn, data, time.Now().Add(l.timeout))
	} else { // new connection
		// pick random next hop
		nextHop := l.nextHops[mrand.Intn(len(l.nextHops))]
		conn, err := net.Dial("udp", nextHop)
		if err != nil {
			l.logger.Println("[clientIn]net.Dial:", err)
			return
		}

		// add the connection to the incoming connections
		l.addClient(raddr, conn)
		// log new connection
		l.logger.Printf("[clientIn]new connection: %v -> %v\n", raddr, conn.RemoteAddr())

		// watch the connection
		// the context is the address of incoming packet
		l.watcher.ReadTimeout(ctx, conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
		l.watcher.WriteTimeout(ctx, conn, data, time.Now().Add(l.timeout))
	}
}

// switcher handles bidirectional communication between the client and the next hop.
func (l *Listener) switcher() {
	for {
		results, err := l.watcher.WaitIO()
		if err != nil {
			l.logger.Println("[switcher]WaitIO():", err)
			return
		}

	RESULTS_LOOP:
		for _, res := range results {
			switch res.Operation {
			case gaio.OpWrite:
				// done writting to proxy connection.
				if res.Error != nil {
					l.logger.Printf("[switcher]gaio.OpWrite: err:%v, hop:%v, local:%v, client:%v", res.Error, res.Conn.RemoteAddr(), res.Conn.LocalAddr(), res.Context)
					l.removeClient(res.Context.(net.Addr))
					continue RESULTS_LOOP
				}

			case gaio.OpRead:
				// any read error from the proxy connection cleans the other side(client).
				if res.Error != nil {
					l.logger.Printf("[switcher]gaio.OpRead: err:%v, hop:%v, local:%v, client:%v", res.Error, res.Conn.RemoteAddr(), res.Conn.LocalAddr(), res.Context)
					l.removeClient(res.Context.(net.Addr))
					continue RESULTS_LOOP
				}

				// received data from the proxy connection.
				dataFromProxy := res.Buffer[:res.Size]

				// decrypt data from the proxy connection if crypterOut is set.
				dataFromProxy, err := decryptPacket(l.crypterOut, dataFromProxy)
				if err != nil {
					l.logger.Println("[switcher]decryptPacket:", err)
					continue RESULTS_LOOP
				}

				// onNextHopIn callback post processing
				if l.onNextHopIn != nil {
					dataFromProxy = l.onNextHopIn(res.Conn.RemoteAddr(), res.Context.(net.Addr), dataFromProxy)
				}

				// forward the data to the client if not nil.
				if dataFromProxy != nil {
					// re-encrypt data if crypterIn is set.
					dataFromProxy = encryptPacket(l.crypterIn, dataFromProxy)

					// forward the data to client via the listener.
					l.conn.WriteTo(dataFromProxy, res.Context.(net.Addr))
				}

				// fire next read-request to the proxy connection.
				l.watcher.ReadTimeout(res.Context, res.Conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			}
		}
	}
}

// addClient registers a new client connection.
func (l *Listener) addClient(raddr net.Addr, conn net.Conn) {
	l.incomingConnectionsLock.Lock()
	l.incomingConnections[raddr.String()] = conn
	l.incomingConnectionsLock.Unlock()
}

// removeClient removes a client connection.
func (l *Listener) removeClient(raddr net.Addr) {
	l.incomingConnectionsLock.Lock()
	delete(l.incomingConnections, raddr.String())
	l.incomingConnectionsLock.Unlock()
}

// Close terminates the listener, releasing resources.
func (l *Listener) Close() error {
	l.dieOnce.Do(func() {
		close(l.die)
		l.conn.Close()
		l.watcher.Close()
	})
	return nil
}

// decryptPacket decrypts the packet using the provided crypter.
// It returns the decrypted data or an error if the checksum does not match.
func decryptPacket(crypter BlockCrypt, packet []byte) (data []byte, err error) {
	if crypter != nil && len(packet) >= headerSize {
		crypter.Decrypt(packet, packet)
		// use the first 8 bytes of md5 digest as the checksum
		checksum := md5.Sum(packet[headerSize:])
		if !bytes.Equal(checksum[:checksumSize], packet[checksumOffset:checksumOffset+checksumSize]) {
			return nil, errChecksum
		}
		data = packet[headerSize:]
	} else if crypter == nil {
		data = packet
	}

	return data, nil
}

// encryptPacket encrypts the packet using the provided crypter.
// It returns the encrypted data or the original data if no crypter is provided.
func encryptPacket(crypter BlockCrypt, data []byte) (packet []byte) {
	if crypter != nil {
		packet = make([]byte, len(data)+headerSize)
		copy(packet[headerSize:], data)
		// fill the nonce(8 bytes)
		_, _ = io.ReadFull(rand.Reader, packet[nonceOffset:nonceOffset+nonceSize])
		// fill in half MD5(8 bytes)
		checksum := md5.Sum(packet[headerSize:])
		copy(packet[checksumOffset:], checksum[:checksumSize])
		// encrypt the packet
		crypter.Encrypt(packet, packet)
	} else {
		packet = data
	}
	return
}
