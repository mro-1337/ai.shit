package telnet

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

func TestNegotiationAndPayload(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	responses := make(chan []byte, 1)
	escaped := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Ask for terminal type and window size, announce SGA.
		conn.Write([]byte{255, 253, 24})
		conn.Write([]byte{255, 250, 24, 1, 255, 240}) // SB TTYPE SEND SE
		conn.Write([]byte{255, 253, 31})
		conn.Write([]byte{255, 251, 3})
		// Payload with an escaped 0xFF and an embedded IAC NOP.
		conn.Write([]byte("hello \xff\xff world\xff\xf1!"))

		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 2048)
		var resp []byte
		for !(bytes.Contains(resp, []byte("ANSI")) && bytes.Contains(resp, []byte{255, 250, 31})) {
			n, err := conn.Read(buf)
			if n > 0 {
				resp = append(resp, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		responses <- resp

		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		var tail []byte
		for !bytes.Contains(tail, []byte{0xff, 0xff, 'x'}) {
			n, err := conn.Read(buf)
			if n > 0 {
				tail = append(tail, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		escaped <- tail
	}()

	_, port, _ := net.SplitHostPort(ln.Addr().String())
	client, err := Dial(context.Background(), "127.0.0.1", atoi(port), Options{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var payload []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := client.ReadAvailable(300 * time.Millisecond)
		if err == nil {
			payload = append(payload, data...)
		}
		if bytes.Contains(payload, []byte("world")) && bytes.Contains(payload, []byte("!")) {
			break
		}
	}
	if !bytes.Contains(payload, []byte("hello \xff world!")) {
		t.Errorf("payload not 8-bit clean: % x", payload)
	}

	resp := <-responses
	if !bytes.Contains(resp, []byte{255, 251, 24}) {
		t.Errorf("client did not answer DO TTYPE with WILL TTYPE: % x", resp)
	}
	if !bytes.Contains(resp, []byte("\xff\xfa\x18\x00ANSI\xff\xf0")) {
		t.Errorf("client did not send TTYPE IS ANSI: % x", resp)
	}
	if !bytes.Contains(resp, []byte{255, 250, 31, 0, 80, 0, 24, 255, 240}) {
		t.Errorf("client did not send NAWS 80x24: % x", resp)
	}

	// Outgoing 0xFF must be doubled.
	if err := client.Write([]byte{0xff, 'x'}); err != nil {
		t.Fatal(err)
	}
	got := <-escaped
	if !bytes.Contains(got, []byte{0xff, 0xff, 'x'}) {
		t.Errorf("outgoing IAC not escaped: % x", got)
	}
}

func TestTimeout(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		time.Sleep(2 * time.Second)
		c.Close()
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	client, err := Dial(context.Background(), "127.0.0.1", atoi(port), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	start := time.Now()
	_, err = client.ReadByteTimeout(150 * time.Millisecond)
	if err != ErrTimeout {
		t.Fatalf("expected timeout, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("timeout took too long")
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
