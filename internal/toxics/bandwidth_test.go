package toxics_test

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"
	"udpcrusher/internal/stream"
	"udpcrusher/internal/testhelper"
	"udpcrusher/internal/toxics"
)

func TestBandwidthToxic(t *testing.T) {
	upstream := testhelper.NewUpstream(t, false)
	defer upstream.Close()

	proxy := NewTestProxy("test", upstream.Addr())
	proxy.Start()
	defer proxy.Stop()

	client, err := net.Dial("udp", proxy.Listen)
	if err != nil {
		t.Fatalf("Unable to dial UDP server: %v", err)
	}

	// UDP is connectionless so lets dial via fake msg
	testMsg := "test msg"
	_, err = client.Write([]byte(testMsg))
	if err != nil {
		t.Fatalf("Unable to send UDP test msg: %v", err)
	}

	upstreamConn := <-upstream.Connections

	rate := 5 // 1KB/s
	toxic := &toxics.BandwidthToxic{Rate: int64(rate)}

	toxicWrapper := &toxics.ToxicWrapper{
		Toxic:     toxic,
		Type:      "bandwidth",
		Direction: stream.Upstream,
	}
	_ = proxy.Toxics.AddToxic(toxicWrapper)

	msg := "hello world "
	writtenPayload := []byte(strings.Repeat(msg, udp_payload_size/len(msg)))
	go func() {
		n, err := client.Write(writtenPayload)
		client.Close()
		if n != len(writtenPayload) || err != nil {
			t.Errorf("Failed to write buffer: (%d == %d) %v", n, len(writtenPayload), err)
		}
	}()
	serverRecvPayload := make([]byte, len(testMsg)+len(writtenPayload))
	start := time.Now()

	_, err = upstream.ReadAtLeast(upstreamConn, serverRecvPayload, len(serverRecvPayload))
	serverRecvPayload = serverRecvPayload[len(testMsg):] // cut testMsg head
	if err != nil {
		t.Errorf("Proxy read failed: %v", err)
	} else if !bytes.Equal(writtenPayload, serverRecvPayload) {
		t.Errorf("Server did not read correct buffer from client!")
	} else {
		t.Logf("Server read correct payload")
	}

	AssertDeltaTime(t,
		"Bandwidth",
		time.Since(start),
		time.Duration(len(testMsg)+len(writtenPayload))*time.Second/time.Duration(rate*1000),
		500*time.Millisecond,
	)
}

func BenchmarkBandwidthToxic100MB(b *testing.B) {
	upstream := testhelper.NewUpstream(b, true)
	defer upstream.Close()

	proxy := NewTestProxy("test", upstream.Addr())
	proxy.Start()
	defer proxy.Stop()

	client, err := net.Dial("udp", proxy.Listen)
	if err != nil {
		b.Error("Unable to dial UDP server", err)
	}

	writtenPayload := []byte(strings.Repeat("hello world ", 1000))

	toxic := &toxics.BandwidthToxic{Rate: 100 * 1000}
	toxicWrapper := &toxics.ToxicWrapper{
		Toxic:     toxic,
		Name:      "",
		Type:      "bandwidth",
		Direction: stream.Upstream,
	}
	_ = proxy.Toxics.AddToxic(toxicWrapper)

	b.SetBytes(int64(len(writtenPayload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n, err := client.Write(writtenPayload)
		if err != nil || n != len(writtenPayload) {
			b.Errorf("%v, %d == %d", err, n, len(writtenPayload))
			break
		}
	}

	err = client.Close()
	if err != nil {
		b.Error("Failed to close UDP connection", err)
	}
}
