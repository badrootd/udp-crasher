package testhelper

import (
	"net"
	"testing"
)

type Upstream struct {
	Connections chan net.Addr
	clientBufs  map[string](chan []byte)
	listener    net.PacketConn
	logger      testing.TB
}

func NewUpstream(t testing.TB, ignoreData bool) *Upstream {
	result := &Upstream{
		logger: t,
	}

	result.clientBufs = make(map[string](chan []byte))

	result.listen()
	result.accept(ignoreData)

	return result
}

func (u *Upstream) listen() {
	listener, err := net.ListenPacket("udp", "localhost:0")
	if err != nil {
		u.logger.Fatalf("Failed to create UDP server: %v", err)
	}
	u.listener = listener
}

func (u *Upstream) accept(ignoreData bool) {
	u.Connections = make(chan net.Addr)
	go func(u *Upstream) {
		var err error
		for err == nil {
			buf := make([]byte, 32768)
			n, addr, err := u.listener.ReadFrom(buf)
			if err != nil {
				u.logger.Logf("Connectino has been closed: %v", err)
				break
			}

			if !ignoreData {
				c, ok := u.clientBufs[addr.String()]
				if !ok {
					readerCh := make(chan []byte, 1000)
					u.clientBufs[addr.String()] = readerCh
					c = readerCh
					u.Connections <- addr
					u.logger.Logf("New client: %s", addr.String())
				}

				c <- buf[:n]
			}
		}
	}(u)
}

func (u *Upstream) Close() {
	u.listener.Close()
}

func (u *Upstream) Addr() string {
	return u.listener.LocalAddr().String()
}

func (u *Upstream) Write(payload []byte, conn net.Addr) (int, error) {
	return u.listener.WriteTo(payload, conn)
}
func (u *Upstream) ReadAtLeast(conn net.Addr, buf []byte, n int) (int, error) {
	c, ok := u.clientBufs[conn.String()]

	if ok {
		read := 0
		for v := range c {
			cn := copy(buf[read:], v)
			read += cn
			if read >= n {
				break
			}
		}
	}

	return len(buf), nil
}
