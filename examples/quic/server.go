package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"math/big"
)

const (
	addr     = "localhost:4242"
	protocol = "simpleproto"
)

type EchoServer struct {
	started chan bool
}

func NewServer() *EchoServer {
	server := EchoServer{}
	server.started = make(chan bool)

	return &server
}

func (s *EchoServer) Start() error {
	listener, err := quic.ListenAddr(addr, generateTLSConfig(), nil)
	if err != nil {
		s.started <- false
		return err
	}
	s.started <- true

	conn, err := listener.Accept(context.Background())
	if err != nil {
		return err
	}

	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(loggingWriter{stream}, stream)
	return err
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{protocol},
	}
}

type loggingWriter struct {
	io.Writer
}

func (w loggingWriter) Write(b []byte) (int, error) {
	br := bytes.NewReader(b)
	m := Message{}
	err := m.Read(br)
	if err != nil {
		fmt.Printf("Server err %v\n", err)
	} else {
		fmt.Printf("Server read %s\n", m)
	}
	return w.Writer.Write(b)
}
