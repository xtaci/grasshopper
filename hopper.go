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
	mrand "math/rand"

	"encoding/binary"
	"hash/crc32"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/xtaci/gaio"
)

const (
	// nonceSize defines the size of the additional nonce (16 bytes) added to each UDP packet.
	nonceSize   = 12
	nonceOffset = 0

	// checksumSize defines the size of the checksum (4 bytes) added to each UDP packet.
	checksumSize   = 4
	checksumOffset = nonceSize

	// headerSize defines the size of the header (nonce + checksum) added to each UDP packet.
	// | nonce(12 bytes) | checksum(4 bytes) | data |
	headerSize = nonceSize + checksumSize

	// mtuLimit specifies the maximum transmission unit (MTU) size for a packet.
	mtuLimit = 1500
)

type (
	// Listener represents a UDP server that listens for incoming connections and relays them to the next hop.
	Listener struct {
		logger     *log.Logger   // logger
		crypterIn  BlockCrypt    // crypter for incoming packets
		crypterOut BlockCrypt    // crypter for outgoing packets
		conn       *net.UDPConn  // the socket to listen on
		timeout    time.Duration // session timeout
		sockbuf    int           // socket buffer size for the `conn`

		// connection pairing
		nextHops                []string            // the outgoing addresses, the switcher will forward packets to one of them randomly.
		watcher                 *gaio.Watcher       // I/O watcher for asynchronous operations.
		incomingConnections     map[string]net.Conn // client address -> {connection to next hop}
		incomingConnectionsLock sync.RWMutex

		die     chan struct{} // Channel to signal listener termination.
		dieOnce sync.Once     // Ensures the close operation is executed only once.
	}
)

// ListenWithOptions initializes a new Listener with the provided options.
// Parameters:
// - laddr: Address to listen on.
// - nexthop: Addresses to forward packets to.
// - sockbuf: Socket buffer size in bytes.
// - timeout: Session timeout duration.
// - crypterIn: Cryptographic handler for decrypting incoming packets.
// - crypterOut: Cryptographic handler for encrypting outgoing packets.
// - logger: Logger instance for logging.
func ListenWithOptions(laddr string, nexthops []string, sockbuf int, timeout time.Duration, crypterIn BlockCrypt, crypterOut BlockCrypt, logger *log.Logger) (*Listener, error) {
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
	l.nextHops = nexthops
	l.die = make(chan struct{})
	l.crypterIn = crypterIn
	l.crypterOut = crypterOut
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
			l.packetIn(buf[:n], from)
		} else {
			l.logger.Fatal("Start:", err)
			return
		}
	}
}

// packetIn processes incoming packets and forwards them to the next hop.
func (l *Listener) packetIn(data []byte, raddr net.Addr) {
	// decrypt the packet if crypterIn is set
	data, err := decryptPacket(l.crypterIn, data)
	if err != nil {
		l.logger.Println("decrypt error:", err)
		return
	}

	// encrypt or re-encrypt the packet if crypterOut is set(with new nonce)
	data = encryptPacket(l.crypterOut, data)

	// load the connection from the incoming connections
	l.incomingConnectionsLock.RLock()
	conn, ok := l.incomingConnections[raddr.String()]
	l.incomingConnectionsLock.RUnlock()

	if ok { // existing connection
		l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout))
	} else { // new connection
		// pick random next hop
		nextHop := l.nextHops[mrand.Intn(len(l.nextHops))]
		conn, err := net.Dial("udp", nextHop)
		if err != nil {
			l.logger.Println("dial target error:", err)
			return
		}

		// add the connection to the incoming connections
		l.addClient(raddr, conn)
		// log new connection
		l.logger.Printf("new connection from %s to %s", raddr.String(), nextHop)

		// watch the connection
		// the context is the address of incoming packet
		ctx := raddr
		l.watcher.ReadTimeout(ctx, conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
		l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout)) // write needs not to specify the context(where the packet from)
	}
}

// switcher handles bidirectional communication between the client and the next hop.
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
					l.logger.Printf("[switcher]write error: %v, %v, %v, %v", res.Error, res.Conn.RemoteAddr(), res.Conn.LocalAddr(), res.Context)
					l.removeClient(res.Conn.RemoteAddr())
					continue
				}

			case gaio.OpRead:
				// any read error from the proxy connection cleans the other side(client).
				if res.Error != nil {
					l.logger.Printf("[switcher]read error: %v, %v, %v, %v", res.Error, res.Conn.RemoteAddr(), res.Conn.LocalAddr(), res.Context)
					l.removeClient(res.Conn.RemoteAddr())
					continue
				}

				// received data from the proxy connection.
				dataFromProxy := res.Buffer[:res.Size]

				// decrypt data from the proxy connection if crypterOut is set.
				dataFromProxy, err := decryptPacket(l.crypterOut, dataFromProxy)
				if err != nil {
					l.logger.Println("decrypt error:", err)
					continue
				}

				// re-encrypt data if crypterIn is set.
				dataFromProxy = encryptPacket(l.crypterIn, dataFromProxy)

				// forward the data to client via the listener.
				l.conn.WriteTo(dataFromProxy, res.Context.(net.Addr))

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
		checksum := crc32.ChecksumIEEE(packet[headerSize:])
		if checksum != binary.LittleEndian.Uint32(packet[checksumOffset:]) {
			return nil, errors.New("checksum mismatch")
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
		// fill the nonce(12 bytes)
		_, _ = io.ReadFull(rand.Reader, packet[nonceOffset:nonceOffset+nonceSize])
		// fill the checksum(4 bytes)
		checksum := crc32.ChecksumIEEE(data)
		binary.LittleEndian.PutUint32(packet[checksumOffset:], checksum)
		// encrypt the packet
		crypter.Encrypt(packet, packet)
	} else {
		packet = data
	}
	return
}
