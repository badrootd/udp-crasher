package main

import (
	"fmt"
	toxiproxy "github.com/badrootd/udpcrusher"
	"github.com/badrootd/udpcrusher/stream"
	"github.com/badrootd/udpcrusher/toxics"
	"time"
)

func main() {
	srv := NewServer()
	go func() {
		err := srv.Start()
		if err != nil {
			fmt.Printf("Failed to start Echo server: %v\n", err)
		}
	}()
	<-srv.started

	// increasing rateLimit improves throughput
	rateLimit := 35
	proxyAddr, err := setupProxy(addr, int64(rateLimit))
	if err != nil {
		fmt.Printf("Failed to setup UDP crusher: %v\n", err)
		return
	}

	cl, err := NewClient(proxyAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cl.Close()

	duration := 15 * time.Second
	count := cl.PingPong("Hello world", duration)

	fmt.Printf("Messages processed %d, rateLimit %d, duration %s\n", count, rateLimit, duration)
}

func setupProxy(upstream string, rateLimit int64) (string, error) {
	proxy := toxiproxy.NewProxy("quic-test", "localhost:0", upstream)
	proxy.Start()

	toxic := &toxics.BandwidthToxic{Rate: rateLimit}
	tw := &toxics.ToxicWrapper{
		Toxic:     toxic,
		Type:      "bandwidth",
		Direction: stream.Upstream,
	}
	err := proxy.Toxics.AddToxic(tw)
	if err != nil {
		return "", fmt.Errorf("AddToxic returned error: %v", err)
	}

	return proxy.Listen, nil
}
