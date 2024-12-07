package udpcast

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/xtaci/gaio"
)

const (
	// 8-bytes UDP nonce for each packet
	nonceSize = 8

	// overall crypto header size
	cryptHeaderSize = nonceSize

	// maximum packet size
	mtuLimit = 1500
)

type (
	// Listener defines a server which will be waiting to accept incoming connections
	Listener struct {
		block   BlockCrypt    // block encryption
		conn    *net.UDPConn  // the underlying packet connection
		timeout time.Duration // session timeout

		// connection pairing
		target                  string              // target address
		watcher                 *gaio.Watcher       // the watcher
		incomingConnections     map[string]net.Conn // client address -> {connection to target}
		incomingConnectionsLock sync.RWMutex

		die     chan struct{} // notify the listener has closed
		dieOnce sync.Once
	}
)

func ListenWithOptions(laddr string, target string, timeout time.Duration, block BlockCrypt) (*Listener, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return serveConn(block, conn, timeout)
}

func serveConn(block BlockCrypt, conn *net.UDPConn, timeout time.Duration) (*Listener, error) {
	// backend switcher
	watcher, err := gaio.NewWatcher()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	l := new(Listener)
	l.conn = conn
	l.die = make(chan struct{})
	l.block = block
	l.watcher = watcher
	l.timeout = timeout
	go l.switcher()
	go l.porter()
	return l, nil
}

// handling incoming UDP packets to the listener
func (l *Listener) porter() {
	for {
		buf := make([]byte, mtuLimit)
		if n, from, err := l.conn.ReadFrom(buf); err == nil {
			l.packetInput(buf[:n], from)
		} else {
			log.Fatal(err)
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
				log.Println("dial target error:", err)
				return
			}

			// initate full-duplex from and to target
			l.incomingConnectionsLock.Lock()
			l.incomingConnections[raddr.String()] = conn
			l.incomingConnectionsLock.Unlock()
			l.watcher.ReadTimeout(nil, conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
			l.watcher.WriteTimeout(nil, conn, data, time.Now().Add(l.timeout))
		}
	}
}

// packet switcher from clients to targets
func (l *Listener) switcher() {
	// use listener connection as the context to identify the connection
	w := l.watcher
	w.Read(nil, l.conn, make([]byte, mtuLimit)) // genesis read request

	for {
		results, err := w.WaitIO()
		if err != nil {
			log.Println("wait io error:", err)
			return
		}

		for _, res := range results {
			switch res.Operation {
			case gaio.OpWrite:
				// write to target complete
				if res.Error != nil {
					log.Println(res.Error)
					l.cleanClient(res.Conn.RemoteAddr())
					continue
				}

			case gaio.OpRead:
				if res.Error != nil { // any error discontinues the connection
					log.Println(res.Error)
					l.cleanClient(res.Conn.RemoteAddr())
					continue
				}

				// data read from target, forward to client
				l.conn.WriteToUDP(res.Buffer, l.conn.RemoteAddr().(*net.UDPAddr))
				// initiate continuous reading
				l.watcher.ReadTimeout(nil, res.Conn, make([]byte, mtuLimit), time.Now().Add(l.timeout))
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
