package toxiproxy

import (
	"errors"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
	tomb "gopkg.in/tomb.v1"

	"udpcrusher/internal/stream"
)

// Proxy represents the proxy in its entirety with all its links. The main
// responsibility of Proxy is to accept new client and create Links between the
// client and upstream.
//
// Client <-> toxiproxy <-> Upstream.
type Proxy struct {
	sync.Mutex

	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`

	//listener net.Listener
	listener net.PacketConn
	started  chan error

	tomb        tomb.Tomb
	connections ConnectionList
	Toxics      *ToxicCollection `json:"-"`
	Logger      *zerolog.Logger

	portToClient map[string]UDPReader
}

type UDPReader struct {
	incoming chan []byte
}

func (u UDPReader) Read(p []byte) (n int, err error) {
	v := <-u.incoming
	cc := copy(p, v)
	return cc, nil
}

type UDPWriter struct {
	outgoing net.PacketConn
	rAddr    net.Addr
}

func (u UDPWriter) Write(p []byte) (n int, err error) {
	n, err = u.outgoing.WriteTo(p, u.rAddr)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func (u UDPWriter) Close() error {
	return nil
}

type ConnectionList struct {
	list map[string]net.PacketConn
	lock sync.Mutex
}

func (c *ConnectionList) Lock() {
	c.lock.Lock()
}

func (c *ConnectionList) Unlock() {
	c.lock.Unlock()
}

var ErrProxyAlreadyStarted = errors.New("Proxy already started")

func NewProxy(name, listen, upstream string) *Proxy {
	l := setupLogger()

	proxy := &Proxy{
		Name:        name,
		Listen:      listen,
		Upstream:    upstream,
		started:     make(chan error),
		connections: ConnectionList{list: make(map[string]net.PacketConn)},
		Logger:      &l,
	}
	proxy.Toxics = NewToxicCollection(proxy)
	proxy.portToClient = make(map[string]UDPReader)
	return proxy
}

func (proxy *Proxy) Start() error {
	proxy.Lock()
	defer proxy.Unlock()

	return start(proxy)
}

func (proxy *Proxy) Update(input *Proxy) error {
	proxy.Lock()
	defer proxy.Unlock()

	if input.Listen != proxy.Listen || input.Upstream != proxy.Upstream {
		stop(proxy)
		proxy.Listen = input.Listen
		proxy.Upstream = input.Upstream
	}

	if input.Enabled != proxy.Enabled {
		if input.Enabled {
			return start(proxy)
		}
		stop(proxy)
	}
	return nil
}

func (proxy *Proxy) Stop() {
	proxy.Lock()
	defer proxy.Unlock()

	stop(proxy)
}

func (proxy *Proxy) listen() error {
	var err error
	proxy.listener, err = net.ListenPacket("udp", proxy.Listen)
	if err != nil {
		proxy.started <- err
		return err
	}
	proxy.Listen = proxy.listener.LocalAddr().String()
	proxy.started <- nil

	proxy.Logger.Info().Str("addr", proxy.listener.LocalAddr().String()).Msg("Started proxy")

	return nil
}

func (proxy *Proxy) close() {
	// Unblock proxy.listener.Accept()
	err := proxy.listener.Close()
	if err != nil {
		proxy.Logger.Warn().Err(err).Msg("Attempted to close an already closed proxy server")
	}
}

// This channel is to kill the blocking Accept() call below by closing the
// net.Listener.
func (proxy *Proxy) freeBlocker(acceptTomb *tomb.Tomb) {
	<-proxy.tomb.Dying()

	// Notify ln.Accept() that the shutdown was safe
	acceptTomb.Killf("Shutting down from stop()")

	proxy.close()

	// Wait for the accept loop to finish processing
	acceptTomb.Wait()
	proxy.tomb.Done()
}

// server runs the Proxy server, accepting new clients and creating Links to
// connect them to upstreams.
func (proxy *Proxy) server() {
	err := proxy.listen()
	if err != nil {
		return
	}

	acceptTomb := &tomb.Tomb{}
	defer acceptTomb.Done()

	// This channel is to kill the blocking Accept() call below by closing the
	// net.Listener.
	go proxy.freeBlocker(acceptTomb)

	for {
		//client, err := proxy.listener.Accept()

		// Create a buffer to hold incoming packets
		buffer := make([]byte, 65535)
		n, clientAddr, err := proxy.listener.ReadFrom(buffer)
		if err != nil {
			// This is to confirm we're being shut down in a legit way. Unfortunately,
			// Go doesn't export the error when it's closed from Close() so we have to
			// sync up with a channel here.
			//
			// See http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-acceptTomb.Dying():
			default:
				proxy.Logger.Warn().Err(err).Msg("Error while accepting client")
			}
			return
		}

		clientReader, ok := proxy.portToClient[clientAddr.String()]
		dst := make([]byte, n)
		copy(dst, buffer[:n])
		if ok {
			clientReader.incoming <- dst
			continue
		}

		// add new client
		readerCh := make(chan []byte, 1000)
		clientReader = UDPReader{
			incoming: readerCh,
		}

		clientReader.incoming <- dst
		proxy.portToClient[clientAddr.String()] = clientReader

		// create new UDP downstream client
		//downClient, err := net.Dial("udp", clientAddr.String())
		//if err != nil {
		//	proxy.Logger.Err(err).Str("client", downClient.RemoteAddr().String()).Msg("Failed to create UDP connection")
		//	return
		//}
		//proxy.Logger.Info().Str("client", downClient.RemoteAddr().String()).Msg("Accepted client")

		clientWriter := UDPWriter{
			outgoing: proxy.listener,
			rAddr:    clientAddr,
		}

		// create new UDP upstream client
		upstreamAddress, err := net.ResolveUDPAddr("udp", proxy.Upstream)
		if err != nil {
			proxy.Logger.Err(err).Str("client", proxy.Upstream).Msg("Unable to resolved upstream")
			continue
		}
		upstream, err := net.DialUDP("udp", nil, upstreamAddress)
		if err != nil {
			proxy.Logger.Err(err).Str("client", proxy.Upstream).Msg("Unable to open connection to upstream")
			continue
		}

		name := clientAddr.String()
		proxy.connections.Lock()
		proxy.connections.list[name+"upstream"] = upstream
		proxy.connections.list[name+"downstream"] = proxy.listener
		proxy.connections.Unlock()
		proxy.Toxics.StartLink(name+"upstream", clientReader, upstream, stream.Upstream)
		proxy.Toxics.StartLink(name+"downstream", upstream, clientWriter, stream.Downstream)
	}
}

func (proxy *Proxy) RemoveConnection(name string) {
	proxy.connections.Lock()
	defer proxy.connections.Unlock()
	delete(proxy.connections.list, name)
}

// Starts a proxy, assumes the lock has already been taken.
func start(proxy *Proxy) error {
	if proxy.Enabled {
		return ErrProxyAlreadyStarted
	}

	proxy.tomb = tomb.Tomb{} // Reset tomb, from previous starts/stops
	go proxy.server()
	err := <-proxy.started
	// Only enable the proxy if it successfully started
	proxy.Enabled = err == nil
	return err
}

// Stops a proxy, assumes the lock has already been taken.
func stop(proxy *Proxy) {
	if !proxy.Enabled {
		return
	}
	proxy.Enabled = false

	proxy.tomb.Killf("Shutting down from stop()")
	proxy.tomb.Wait() // Wait until we stop accepting new connections

	proxy.connections.Lock()
	defer proxy.connections.Unlock()
	for _, conn := range proxy.connections.list {
		conn.Close()
	}

	proxy.Logger.Info().Msg("Terminated proxy")
}

func setupLogger() zerolog.Logger {
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		return file + ":" + strconv.Itoa(line)
	}

	logger := zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()

	val, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		return logger
	}

	lvl, err := zerolog.ParseLevel(val)
	if err == nil {
		logger = logger.Level(lvl)
	} else {
		l := &logger
		l.Err(err).Msgf("unknown LOG_LEVEL value: \"%s\"", val)
	}

	return logger
}
