package fakebbs

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// tConn is a minimal Telnet *server* connection: it strips IAC sequences
// from client input and answers the negotiations the client starts.
type tConn struct {
	c net.Conn

	buf      []byte
	state    int
	pending  byte
	sb       []byte
	deadline time.Duration
}

const (
	tData = iota
	tIAC
	tCmd
	tSB
	tSBIAC
)

func newTConn(c net.Conn) *tConn {
	return &tConn{c: c, deadline: 120 * time.Second}
}

func (t *tConn) writeRaw(data []byte) {
	_ = t.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, _ = t.c.Write(data)
}

func (t *tConn) writeString(s string) { t.writeRaw([]byte(s)) }

func (t *tConn) writef(format string, args ...any) { t.writeString(fmt.Sprintf(format, args...)) }

func (t *tConn) unread(b byte) { t.buf = append(t.buf, b) }

func (t *tConn) readByte(timeout time.Duration) (byte, error) {
	if len(t.buf) > 0 {
		b := t.buf[0]
		t.buf = t.buf[1:]
		return b, nil
	}
	_ = t.c.SetReadDeadline(time.Now().Add(timeout))
	chunk := make([]byte, 4096)
	n, err := t.c.Read(chunk)
	if err != nil {
		return 0, err
	}
	t.parse(chunk[:n])
	if len(t.buf) == 0 {
		return 0, errTimeout{}
	}
	b := t.buf[0]
	t.buf = t.buf[1:]
	return b, nil
}

type errTimeout struct{}

func (errTimeout) Error() string   { return "timeout" }
func (errTimeout) Timeout() bool   { return true }
func (errTimeout) Temporary() bool { return true }

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func (t *tConn) parse(chunk []byte) {
	for _, b := range chunk {
		switch t.state {
		case tData:
			if b == 255 {
				t.state = tIAC
			} else {
				t.buf = append(t.buf, b)
			}
		case tIAC:
			switch b {
			case 255:
				t.buf = append(t.buf, 255)
				t.state = tData
			case 251, 252, 253, 254: // WILL WONT DO DONT
				t.pending = b
				t.state = tCmd
			case 250: // SB
				t.sb = t.sb[:0]
				t.state = tSB
			default:
				t.state = tData
			}
		case tCmd:
			t.negotiate(t.pending, b)
			t.state = tData
		case tSB:
			if b == 255 {
				t.state = tSBIAC
			} else {
				t.sb = append(t.sb, b)
			}
		case tSBIAC:
			if b == 240 { // SE
				t.state = tData
			} else {
				t.sb = append(t.sb, b)
				t.state = tSB
			}
		}
	}
}

func (t *tConn) negotiate(cmd, opt byte) {
	if cmd == 251 && opt == 24 { // WILL TTYPE -> ask for it
		t.writeRaw([]byte{255, 250, 24, 1, 255, 240}) // SB TTYPE SEND SE
	}
	// Other options are accepted silently; the test BBS does not need them.
}

func (t *tConn) readLine(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var out []byte
	for {
		if time.Now().After(deadline) {
			return "", errTimeout{}
		}
		b, err := t.readByte(500 * time.Millisecond)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			return "", err
		}
		switch b {
		case '\r', '\n':
			t.consumeLF()
			t.writeString("\r\n")
			return strings.TrimSpace(string(out)), nil
		case 8, 127:
			if len(out) > 0 {
				out = out[:len(out)-1]
				t.writeString("\b \b")
			}
		case 27:
			// Drop ANSI arrow-key sequences; menus here use letters.
			t.dropEscape()
		default:
			if b >= 32 {
				out = append(out, b)
				t.writeRaw([]byte{b})
			}
		}
	}
}

func (t *tConn) consumeLF() {
	b, err := t.readByte(40 * time.Millisecond)
	if err != nil || b != '\n' {
		if err == nil {
			t.unread(b)
		}
	}
}

func (t *tConn) dropEscape() {
	for i := 0; i < 8; i++ {
		b, err := t.readByte(30 * time.Millisecond)
		if err != nil {
			return
		}
		if (b >= 'A' && b <= 'Z') || b == '~' {
			return
		}
	}
}

// readKey reads one key, echoing it.
func (t *tConn) readKey(timeout time.Duration) (byte, error) {
	deadline := time.Now().Add(timeout)
	var b byte
	for {
		if time.Now().After(deadline) {
			return 0, errTimeout{}
		}
		var err error
		b, err = t.readByte(500 * time.Millisecond)
		if err == nil {
			break
		}
		if !isTimeout(err) {
			return 0, err
		}
	}
	if b >= 32 {
		t.writeRaw([]byte{b})
	}
	t.writeString("\r\n")
	return b, nil
}
