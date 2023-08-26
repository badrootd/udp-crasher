package main

import (
	"fmt"
)

func main() {
	go func() {
		err := EchoServer()
		if err != nil {
			fmt.Println(err)
		}
	}()

	cl, err := NewClient(addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cl.Close()

	rtt := cl.PingPong("Hello world")

	fmt.Printf("Time: %s\n", rtt)
}
