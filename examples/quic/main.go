package main

import (
	"fmt"
	toxiproxy "github.com/badrootd/udpcrusher"
	"github.com/badrootd/udpcrusher/stream"
	"github.com/badrootd/udpcrusher/toxics"
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

	proxyAddr, err := setupProxy(addr)
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

	rtt := cl.PingPong("Hello world")

	fmt.Printf("Time: %s\n", rtt)
}

func setupProxy(upstream string) (string, error) {
	proxy := toxiproxy.NewProxy("quic-test", "localhost:0", upstream)
	proxy.Start()

	toxic := &toxics.BandwidthToxic{Rate: int64(15)}
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
