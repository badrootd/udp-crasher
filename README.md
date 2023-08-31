# UDP Crusher

UDP Crusher is a Golang library designed to simulate UDP network conditions. It is inspired by the [Toxiproxy](https://github.com/Shopify/toxiproxy) project, with code copied and adapted filling the gap for UDP simulation as Toxiproxy primarily focuses on TCP.

### Usage

```go
    upstream := "8.8.8.8:53"

    proxy := toxiproxy.NewProxy("proxy-name", "localhost:0", upstream)
    proxy.Start()
    
    toxic := &toxics.LatencyToxic{
        Latency: 750 // in milliseconds
    }

    tw := &toxics.ToxicWrapper{
        Toxic:     toxic,
        Name:      "latency_up",
        Type:      "latency",
        Direction: stream.Upstream,
    }
    _ = proxy.Toxics.AddToxic(tw)

	// your client application code
    _, _ = net.Dial("udp", proxy.Listen)
```

### Why not just fork Toxiproxy?

Currently toxiproxy has already tightly coupled with golang TCP socket API and cannot be extended that easily.

### Which parts are removed from Toxiproxy?

Certain components, such as the metric collector and JSON toxic serialization/deserialization, have been removed from the original Toxiproxy project for the sake of development. These can be re-implemented in the future if needed.

### Who can find it useful?

Anyone who develops client/protocol implementations over UDP or applications using above in golang, for example QUIC, Aeron, DNS and etc

UDP Crusher is beneficial for developers working on various aspects of UDP communication, including UDP clients, protocols over UDP, and applications that utilize UDP. Some examples of potential use cases include:

- Implementing and testing QUIC-based applications
- Simulating Aeron network conditions for robustness testing
- Testing DNS communication behavior under different network conditions
