package toxics_test

import (
	"bytes"
	"context"
	"fmt"
	"gopkg.in/tomb.v1"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
	toxiproxy "udpcrusher/internal"
	"udpcrusher/internal/stream"
	"udpcrusher/internal/testhelper"

	//"udpcrusher/internal/testhelper"
	"udpcrusher/internal/toxics"
)

const udp_payload_size = 9216 // darwin max UDP package size is 9216 (sysctl net.inet.udp.maxdgram)

func AssertDeltaTime(t *testing.T, message string, actual, expected, delta time.Duration) {
	diff := actual - expected
	if diff < 0 {
		diff *= -1
	}
	if diff > delta {
		t.Errorf(
			"[%s] Time was more than %v off: got %v expected %v",
			message,
			delta,
			actual,
			expected,
		)
	} else {
		t.Logf("[%s] Time was correct: %v (expected %v)", message, actual, expected)
	}
}

func DoLatencyTest(t *testing.T, upLatency, downLatency *toxics.LatencyToxic) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		if upLatency == nil {
			upLatency = &toxics.LatencyToxic{}
		} else {
			tw := &toxics.ToxicWrapper{
				Toxic:     upLatency,
				Name:      "latency_up",
				Type:      "latency",
				Direction: stream.Upstream,
			}
			err := proxy.Toxics.AddToxic(tw)
			if err != nil {
				t.Error("AddToxic returned error:", err)
			}
		}
		if downLatency == nil {
			downLatency = &toxics.LatencyToxic{}
		} else {
			tw := &toxics.ToxicWrapper{
				Toxic:     downLatency,
				Name:      "latency_down",
				Type:      "latency",
				Direction: stream.Downstream,
			}
			err := proxy.Toxics.AddToxic(tw)
			if err != nil {
				t.Error("AddToxicJson returned error:", err)
			}
		}
		t.Logf(
			"Using latency: Up: %dms +/- %dms, Down: %dms +/- %dms",
			upLatency.Latency,
			upLatency.Jitter,
			downLatency.Latency,
			downLatency.Jitter,
		)

		helloWorld := "hello world "
		msg := []byte(helloWorld + strings.Repeat("a", udp_payload_size-len(helloWorld)))

		timer := time.Now()
		_, err := conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to UDP server", err)
		}

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read correct bytes from client:", string(resp))
		}
		AssertDeltaTime(t,
			"Server read",
			time.Since(timer),
			time.Duration(upLatency.Latency)*time.Millisecond,
			time.Duration(upLatency.Jitter+10)*time.Millisecond,
		)
		timer2 := time.Now()

		buf := make([]byte, udp_payload_size)
		n, err := conn.Read(buf)
		resp = buf[:n]
		if err != nil {
			t.Error("Failed to read UDP", err)
		}
		modifiedMsg := []byte(strings.Replace(string(msg), "a", "b", -1))
		if !bytes.Equal(resp, modifiedMsg) {
			t.Error("Client didn't read correct bytes from server:", string(resp))
		}

		AssertDeltaTime(t,
			"Client read",
			time.Since(timer2),
			time.Duration(downLatency.Latency)*time.Millisecond,
			time.Duration(downLatency.Jitter+10)*time.Millisecond,
		)
		AssertDeltaTime(t,
			"Round trip",
			time.Since(timer),
			time.Duration(upLatency.Latency+downLatency.Latency)*time.Millisecond,
			time.Duration(upLatency.Jitter+downLatency.Jitter+20)*time.Millisecond,
		)

		ctx := context.Background()
		proxy.Toxics.RemoveToxic(ctx, "latency_up")
		proxy.Toxics.RemoveToxic(ctx, "latency_down")

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close UDP connection", err)
		}
	})
}

func TestUpstreamLatency(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 1000}, nil)
}

func TestDownstreamLatency(t *testing.T) {
	DoLatencyTest(t, nil, &toxics.LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyEven(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 100}, &toxics.LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyBiasUp(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 1000}, &toxics.LatencyToxic{Latency: 100})
}

func TestFullstreamLatencyBiasDown(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 100}, &toxics.LatencyToxic{Latency: 1000})
}

func TestZeroLatency(t *testing.T) {
	DoLatencyTest(t, &toxics.LatencyToxic{Latency: 0}, &toxics.LatencyToxic{Latency: 0})
}

// TODO: migrate to UDP
//func TestLatencyToxicCloseRace(t *testing.T) {
//	ln, err := net.Listen("tcp", "localhost:0")
//	if err != nil {
//		t.Fatal("Failed to create TCP server", err)
//	}
//
//	defer ln.Close()
//
//	proxy := NewTestProxy("test", ln.Addr().String())
//	proxy.Start()
//	defer proxy.Stop()
//
//	go func() {
//		for {
//			_, err := ln.Accept()
//			if err != nil {
//				return
//			}
//		}
//	}()
//
//	// Check for potential race conditions when interrupting toxics
//	for i := 0; i < 1000; i++ {
//		proxy.Toxics.AddToxicJson(
//			ToxicToJson(t, "", "latency", "upstream", &toxics.LatencyToxic{Latency: 10}),
//		)
//		conn, err := net.Dial("tcp", proxy.Listen)
//		if err != nil {
//			t.Error("Unable to dial TCP server", err)
//		}
//		conn.Write([]byte("hello"))
//		conn.Close()
//		proxy.Toxics.RemoveToxic(context.Background(), "latency")
//	}
//}

func TestTwoLatencyToxics(t *testing.T) {
	WithEchoProxy(t, func(conn net.Conn, response chan []byte, proxy *toxiproxy.Proxy) {
		toxicArr := []*toxics.LatencyToxic{{Latency: 500}, {Latency: 500}}
		for i, toxic := range toxicArr {
			tw := &toxics.ToxicWrapper{
				Toxic:     toxic,
				Name:      "latency_" + strconv.Itoa(i),
				Type:      "latency",
				Direction: stream.Upstream,
			}
			err := proxy.Toxics.AddToxic(tw)
			if err != nil {
				t.Error("AddToxic returned error:", err)
			}
		}

		helloWorld := "hello world "
		msg := []byte(helloWorld + strings.Repeat("a", udp_payload_size-len(helloWorld)))

		timer := time.Now()
		_, err := conn.Write(msg)
		if err != nil {
			t.Error("Failed writing to UDP server", err)
		}

		resp := <-response
		if !bytes.Equal(resp, msg) {
			t.Error("Server didn't read correct bytes from client:", string(resp))
		}

		AssertDeltaTime(t,
			"Upstream two latency toxics",
			time.Since(timer),
			time.Duration(1000)*time.Millisecond,
			time.Duration(10)*time.Millisecond,
		)

		for i := range toxicArr {
			proxy.Toxics.RemoveToxic(context.Background(), "latency_"+strconv.Itoa(i))
		}

		err = conn.Close()
		if err != nil {
			t.Error("Failed to close UDP connection", err)
		}
	})
}

func TestLatencyToxicBandwidth(t *testing.T) {
	upstream := testhelper.NewUpstream(t, false)
	defer upstream.Close()

	proxy := NewTestProxy("test", upstream.Addr())
	proxy.Start()
	defer proxy.Stop()

	client, err := net.Dial("udp", proxy.Listen)
	if err != nil {
		t.Fatalf("Unable to dial UDP server: %v", err)
	}
	// UDP is connectionless, dial via fake msg
	_, err = client.Write([]byte("test msg"))
	if err != nil {
		t.Fatalf("Unable to send UDP test msg: %v", err)
	}

	msg := "hello world "
	writtenPayload := []byte(strings.Repeat(msg, udp_payload_size/len(msg)))
	upstreamConn := <-upstream.Connections
	go func(conn net.Addr, payload []byte) {
		var err error
		for err == nil {
			_, err = upstream.Write(payload, conn)
			if err != nil {
				t.Logf("Finished writing data due to: %v", err)
			}
		}
	}(upstreamConn, writtenPayload)

	wrapper := &toxics.ToxicWrapper{
		Toxic:  &toxics.LatencyToxic{Latency: 100},
		Name:   "",
		Type:   "latency",
		Stream: "",
	}
	err = proxy.Toxics.AddToxic(wrapper)
	if err != nil {
		t.Fatalf("Unable to register toxic: %v", err)
	}

	time.Sleep(150 * time.Millisecond) // Wait for latency toxic
	response := make([]byte, len(writtenPayload))

	start := time.Now()
	count := 0
	for i := 0; i < 100; i++ {
		n, err := io.ReadFull(client, response)
		if err != nil {
			t.Fatalf("Could not read from socket: %v", err)
			break
		}
		count += n
	}

	// Assert the transfer was at least 100MB/s
	AssertDeltaTime(
		t,
		"Latency toxic bandwidth",
		time.Since(start),
		0,
		time.Duration(count/100000)*time.Millisecond,
	)

	err = client.Close()
	if err != nil {
		t.Error("Failed to close UDP connection", err)
	}
}

func NewTestProxy(name, upstream string) *toxiproxy.Proxy {
	//log := zerolog.Nop()
	//if flag.Lookup("test.v").DefValue == "true" {
	//	log = zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	//}
	//srv := toxiproxy.NewServer(
	//	toxiproxy.NewMetricsContainer(prometheus.NewRegistry()),
	//	log,
	//)
	//srv.Metrics.ProxyMetrics = collectors.NewProxyMetricCollectors()
	proxy := toxiproxy.NewProxy(name, "localhost:0", upstream)

	return proxy
}

func WithEchoProxy(
	t *testing.T,
	f func(proxy net.Conn, response chan []byte, proxyServer *toxiproxy.Proxy),
) {
	WithEchoServer(t, func(upstream string, response chan []byte) {
		proxy := NewTestProxy("test", upstream)
		proxy.Start()

		conn, err := net.Dial("udp", proxy.Listen)
		if err != nil {
			t.Error("Unable to dial UDP server", err)
		}

		f(conn, response, proxy)

		proxy.Stop()
	})
}

func WithEchoServer(t *testing.T, f func(string, chan []byte)) {
	ln, err := net.ListenPacket("udp", "localhost:0")
	if err != nil {
		t.Fatal("Failed to create UDP server", err)
	}

	response := make(chan []byte, 1)
	tomb := tomb.Tomb{}

	go func() {
		defer tomb.Done()
		buffer := make([]byte, 65535)
		n, clientAddr, err := ln.ReadFrom(buffer)
		if err != nil {
			select {
			case <-tomb.Dying():
			default:
				t.Error("Failed to accept client")
				return
			}
			return
		}

		//scan := bufio.NewScanner(src)
		//if scan.Scan() {
		//	received := append(scan.Bytes(), '\n')
		//	response <- received

		//src.Write(received)
		//}

		received := buffer[:n]
		response <- received
		msg := []byte(strings.Replace(string(received), "a", "b", -1))
		_, err = ln.WriteTo(msg, clientAddr)
		if err != nil {
			t.Error("Failed to write client back", err)
		}

		fmt.Printf("Echo written %s to client %s\n", string(msg), clientAddr)
	}()

	f(ln.LocalAddr().String(), response)

	tomb.Killf("Function body finished")
	ln.Close()
	tomb.Wait()

	close(response)
}
