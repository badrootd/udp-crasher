package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"time"
)

// single stream client
type client struct {
	conn   quic.Connection
	stream quic.Stream
}

func NewClient(addr string) (*client, error) {
	config := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{protocol},
	}

	conn, err := quic.DialAddr(context.Background(), addr, config, nil)
	if err != nil {
		return nil, err
	}

	return &client{conn: conn}, nil
}

func (c *client) PingPong(text string) time.Duration {
	var err error
	c.stream, err = c.conn.OpenStream()
	if err != nil {
		return 0
	}

	timeoutGlobal := time.After(10 * time.Second)

	start := time.Now()
	go func() {
		for {
			message := Message{Text: text}
			message.Write(c.stream)
			select {
			case <-timeoutGlobal:
				return
			default:
			}
		}
	}()

	for {
		msgCh, errCh := c.receive(context.Background())

		select {
		case err = <-errCh:
			fmt.Printf("Client err: %v\n", err)
		case msg := <-msgCh:
			fmt.Printf("Client read: %s\n", msg)
		case <-time.After(time.Second * 3):
			fmt.Println("Client read timeout")
		case <-timeoutGlobal:
			fmt.Println("Timeout")
			return time.Since(start)
		}
	}
}

func (c *client) send(text string) error {
	message := Message{Text: text}
	return message.Write(c.stream)
}

func (c *client) receive(ctx context.Context) (<-chan Message, <-chan error) {
	messages, errs := make(chan Message, 1000), make(chan error, 100)
	go func() {
		defer close(messages)
		defer close(errs)
		for {
			var message Message
			err := message.Read(c.stream)
			if err != nil {
				errs <- err
			}
			messages <- message

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return messages, errs
}

func (c *client) Close() error {
	return c.stream.Close()
}
