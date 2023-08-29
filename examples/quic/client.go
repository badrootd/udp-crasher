package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
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

func (c *client) PingPong(text string, duration time.Duration) int {
	var err error
	c.stream, err = c.conn.OpenStream()
	if err != nil {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	go func() {
		for {
			message := Message{Text: text, Timestamp: time.Now()}
			_, err := c.write(message)
			if err != nil {
				fmt.Printf("Error during message write %v\n", err)
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	msgCount := 0
	msgCh, errCh := c.receiveLoop(ctx)
	tick := time.NewTicker(time.Second)
	start := time.Now()
	for {
		select {
		case err = <-errCh:
			fmt.Printf("Client err: %v\n", err)
		case <-msgCh:
			//fmt.Printf("Client read: %s\n", msg)
			msgCount++
		case <-time.After(time.Second * 3):
			fmt.Println("Client read timeout")
		case <-tick.C:
			fmt.Printf("Messages read: %d, time left %s\n", msgCount, (duration - time.Since(start)).Truncate(time.Second))
		case <-ctx.Done():
			fmt.Println("Timeout")
			return msgCount
		}
	}
}

func (c *client) Close() error {
	return c.stream.Close()
}

func (c *client) write(message Message) (int, error) {
	buffer := bytes.Buffer{}
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(message)
	if err != nil {
		fmt.Printf("Error during encode %v\n", err)
		return 0, err
	}

	// lets assume len fits single byte for simplicity
	lenByte := make([]byte, 1)
	lenByte[0] = byte(len(buffer.Bytes()))

	return c.stream.Write(append(lenByte, buffer.Bytes()...))
}

func (c *client) receiveLoop(ctx context.Context) (<-chan Message, <-chan error) {
	messages, errs := make(chan Message, 1000), make(chan error, 100)
	go func() {
		defer close(messages)
		defer close(errs)
		for {
			var message Message
			err := c.readMessage(&message)
			if err != nil {
				errs <- err
				return
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

func (c *client) readMessage(message *Message) error {
	lenByte := make([]byte, 1)
	_, err := c.stream.Read(lenByte)
	if err != nil {
		return err
	}

	msgSize := int(lenByte[0])
	b := make([]byte, msgSize)
	read, err := c.stream.Read(b)
	if err != nil {
		return err
	}

	for read < msgSize {
		n, err := c.stream.Read(b[read:msgSize])
		if err != nil {
			return err
		}
		read += n
	}

	err = gob.NewDecoder(bytes.NewReader(b)).Decode(message)
	return nil
}
