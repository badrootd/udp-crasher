package main

import (
	"fmt"
	"time"

	toxiproxy "github.com/badrootd/udpcrusher"
	"github.com/badrootd/udpcrusher/stream"

	"github.com/badrootd/udpcrusher/toxics"
)

const (
	dnsHttpServer = "127.0.0.1:5380"
	dnsServer     = "127.0.0.1:5453"

	exampleDomain = "example.com"
	exampleIP     = "6.6.6.6"
)

func main() {
	err := setupDNS(dnsHttpServer, exampleDomain, exampleIP)
	if err != nil {
		fmt.Printf("Failed to setup DNS records: %v\n", err)
		return
	}

	proxyAddr, err := setupProxy(dnsServer)
	if err != nil {
		fmt.Printf("Failed to setup UDP crusher: %v\n", err)
		return
	}

	start := time.Now()
	resolvedIP, err := queryDNS(proxyAddr, exampleDomain)
	if err != nil {
		fmt.Printf("Failed to setup DNS records: %v\n", err)
		return
	}
	fmt.Printf("Lookup: %s -> %s\n", exampleDomain, resolvedIP)
	fmt.Printf("RTT: %s\n", time.Since(start))
}

func setupProxy(upstream string) (string, error) {
	proxy := toxiproxy.NewProxy(nil, "dns-test", "localhost:0", upstream)
	proxy.Start()

	toxic := &toxics.LatencyToxic{Latency: 750}
	tw := &toxics.ToxicWrapper{
		Toxic:     toxic,
		Name:      "latency_up",
		Type:      "latency",
		Direction: stream.Upstream,
	}
	err := proxy.Toxics.AddToxic(tw)
	if err != nil {
		return "", fmt.Errorf("AddToxic returned error: %v", err)
	}

	return proxy.Listen, nil
}
