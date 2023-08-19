package testhelper

import (
	"net"
	"testing"
)

type Upstream struct {
	listener    net.PacketConn
	logger      testing.TB
	Connections chan net.Addr
}

func NewUpstream(t testing.TB, ignoreData bool) *Upstream {
	result := &Upstream{
		logger: t,
	}

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
		buf := make([]byte, 32768)
		_, addr, err := u.listener.ReadFrom(buf)
		if err != nil {
			u.logger.Fatalf("Unable to accept first UDP message: %v", err)
		}
		if ignoreData {
			//buf := make([]byte, 4000)
			//for err == nil {
			//_, err = conn.Read(buf)
			//}
		} else {
			u.logger.Logf("New client: %s", addr.String())
			u.Connections <- addr
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
