package transfer

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type pipeStream struct{ c net.Conn }

func (p pipeStream) ReadByteTimeout(d time.Duration) (byte, error) {
	_ = p.c.SetReadDeadline(time.Now().Add(d))
	var b [1]byte
	if _, err := io.ReadFull(p.c, b[:]); err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return 0, ErrTimeout
		}
		return 0, err
	}
	return b[0], nil
}

func (p pipeStream) Write(data []byte) error {
	_, err := p.c.Write(data)
	return err
}

func testPayload(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		// Cycle through values that need every escape rule.
		out[i] = byte(i % 256)
	}
	copy(out, []byte{0x18, 0x7f, 0xff, 0x0d, 0x0a, 0x11, 0x13, 0x00})
	return out
}

func TestZModemRoundTrip(t *testing.T) {
	dir := t.TempDir()
	payload := testPayload(3500)
	name := "test file.bin"
	files := []OutFile{{Name: name, Data: payload}}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	sendErr := make(chan error, 1)
	go func() {
		sendErr <- SendZModem(pipeStream{server}, files, 10*time.Second, nil)
	}()

	got, err := ReceiveZModem(pipeStream{client}, dir, 10*time.Second, t.Logf)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if err := <-sendErr; err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d files, want 1", len(got))
	}
	if got[0].Name != name {
		t.Errorf("name = %q, want %q", got[0].Name, name)
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("downloaded content differs (got %d bytes, want %d)", len(data), len(payload))
	}
}

func TestZModemMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	files := []OutFile{
		{Name: "one.txt", Data: []byte("first file\r\n")},
		{Name: "two.dat", Data: testPayload(2048)},
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	go func() { _ = SendZModem(pipeStream{server}, files, 10*time.Second, nil) }()
	got, err := ReceiveZModem(pipeStream{client}, dir, 10*time.Second, nil)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2", len(got))
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, f.Data) {
			t.Errorf("%s content mismatch", f.Name)
		}
	}
}

func TestZModemRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	files := []OutFile{{Name: "../../evil.txt", Data: []byte("nope")}}
	client, server := net.Pipe()
	defer client.Close()
	go func() { _ = SendZModem(pipeStream{server}, files, 5*time.Second, nil) }()
	_, err := ReceiveZModem(pipeStream{client}, dir, 5*time.Second, nil)
	if err != nil && !errors.Is(err, ErrTransferTimeout) {
		t.Logf("receive ended with %v (acceptable)", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("path traversal wrote outside download dir")
	}
}

func TestXModemRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ymodem bool
	}{
		{"xmodem", false},
		{"ymodem", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			payload := testPayload(700)
			files := []OutFile{{Name: "x-test.bin", Data: payload}}
			client, server := net.Pipe()
			defer client.Close()
			sendErr := make(chan error, 1)
			go func() { sendErr <- SendXModem(pipeStream{server}, files, tc.ymodem, 10*time.Second, nil) }()
			got, err := ReceiveXModem(pipeStream{client}, dir, XModemOptions{YModem: tc.ymodem, Timeout: 10 * time.Second}, t.Logf)
			if err != nil {
				t.Fatalf("receive: %v (sender: %v)", err, <-sendErr)
			}
			if len(got) != 1 {
				t.Fatalf("got %d files, want 1", len(got))
			}
			data, err := os.ReadFile(got[0].Path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, payload) {
				t.Errorf("content mismatch: got %d bytes want %d", len(data), len(payload))
			}
		})
	}
}
