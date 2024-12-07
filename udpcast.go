package udpcast

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

	// overall crypto header size
	cryptHeaderSize = nonceSize

	// maximum packet size
	mtuLimit = 1500
)

type (
	// Listener defines a server which will be waiting to accept incoming connections
	Listener struct {
		logger  *log.Logger   // logger
		block   BlockCrypt    // block encryption
		conn    *net.UDPConn  // the underlying packet connection
		timeout time.Duration // session timeout
		sockbuf int           // socket buffer size

		// connection pairing
		target                  string              // target address
		watcher                 *gaio.Watcher       // the watcher
		incomingConnections     map[string]net.Conn // client address -> {connection to target}
		incomingConnectionsLock sync.RWMutex

		die     chan struct{} // notify the listener has closed
		dieOnce sync.Once
	}
)

func ListenWithOptions(laddr string, target string, sockbuf int, timeout time.Duration, block BlockCrypt, logger *log.Logger) (*Listener, error) {
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
	l.target = target
	l.die = make(chan struct{})
	l.block = block
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
			l.packetInput(buf[:n], from)
		} else {
			l.logger.Fatal("Start:", err)
			return
		}
	}
}

func (l *Listener) packetInput(data []byte, raddr net.Addr) {
	decrypted := false
	if l.block != nil && len(data) >= cryptHeaderSize {
		l.block.Decrypt(data, data)
		decrypted = true
	} else if l.block == nil {
		decrypted = true
	}

	if decrypted {
		l.incomingConnectionsLock.RLock()
		conn, ok := l.incomingConnections[raddr.String()]
		l.incomingConnectionsLock.RUnlock()

		if ok { // existing connection
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout))
		} else { // new connection
			// dial target
			conn, err := net.Dial("udp", l.target)
			if err != nil {
				l.logger.Println("dial target error:", err)
				return
			}

			// initate full-duplex from and to target
			ctx := raddr
			l.incomingConnectionsLock.Lock()
			l.incomingConnections[raddr.String()] = conn
			l.incomingConnectionsLock.Unlock()
			l.watcher.ReadTimeout(ctx, conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout))
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

				var dataFromTarget []byte
				if l.block == nil {
					dataFromTarget = make([]byte, res.Size)
					copy(dataFromTarget, res.Buffer)
				} else { // encrypt the packet
					dataFromTarget = make([]byte, res.Size+nonceSize)
					copy(dataFromTarget[nonceSize:], res.Buffer)
					io.ReadFull(rand.Reader, dataFromTarget[:nonceSize])
					l.block.Encrypt(dataFromTarget, dataFromTarget)
				}

				// forward data to client
				l.conn.WriteTo(dataFromTarget, res.Context.(net.Addr))

				// initiate consecutive reading from the target
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
