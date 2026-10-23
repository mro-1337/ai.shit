// Package telnet implements the slice of Telnet that BBSes actually use:
// IAC escaping, option negotiation, terminal type and window size.
//
// Reads are synchronous: the caller asks for a byte (or whatever is
// available) with a timeout, which is how the agent decides that the screen
// has settled. Data is 8-bit clean for ZMODEM/XMODEM transfers.
package telnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Telnet protocol bytes.
const (
	IAC  = 255
	DONT = 254
	DO   = 253
	WONT = 252
	WILL = 251
	SB   = 250
	SE   = 240
)

// Telnet options we care about.
const (
	OptBinary     = 0
	OptEcho       = 1
	OptSGA        = 3
	OptStatus     = 5
	OptTTYPE      = 24
	OptNAWS       = 31
	OptLinemode   = 34
	OptNewEnviron = 39
)

const (
	ttypeIs   = 0
	ttypeSend = 1
)

// ErrTimeout is returned when no bytes arrived before the deadline.
var ErrTimeout = errors.New("telnet: read timeout")

// Options configures a client.
type Options struct {
	Cols         int
	Rows         int
	TerminalType string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	Trace        func(dir string, data []byte)
}

func (o *Options) normalize() {
	if o.Cols < 20 {
		o.Cols = 80
	}
	if o.Rows < 5 {
		o.Rows = 24
	}
	if o.TerminalType == "" {
		o.TerminalType = "ANSI"
	}
	if o.DialTimeout <= 0 {
		o.DialTimeout = 15 * time.Second
	}
	if o.ReadTimeout <= 0 {
		o.ReadTimeout = 30 * time.Second
	}
}

// Client is a blocking Telnet connection.
type Client struct {
	Host string
	Port int
	Opts Options

	conn net.Conn

	payload []byte
	raw     []byte

	state      int
	pendingCmd byte
	sbBuf      []byte

	refusedDo   map[byte]bool
	refusedWill map[byte]bool

	closed bool
}

const (
	stateData = iota
	stateIAC
	stateNegotiate
	stateSB
	stateSBIAC
)

// Dial connects to a BBS.
func Dial(ctx context.Context, host string, port int, opts Options) (*Client, error) {
	opts.normalize()
	d := net.Dialer{Timeout: opts.DialTimeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return nil, fmt.Errorf("connect %s:%d: %w", host, port, err)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
		_ = tc.SetKeepAlive(true)
	}
	c := &Client{
		Host:        host,
		Port:        port,
		Opts:        opts,
		conn:        conn,
		refusedDo:   map[byte]bool{},
		refusedWill: map[byte]bool{},
	}
	return c, nil
}

// Close terminates the connection.
func (c *Client) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// SetWindowSize updates the advertised screen size (NAWS).
func (c *Client) SetWindowSize(cols, rows int) {
	c.Opts.Cols, c.Opts.Rows = cols, rows
	c.sendNAWS()
}

// ---------------------------------------------------------------- reading

// ReadByteTimeout returns the next payload byte. ErrTimeout means "nothing yet",
// io.EOF means the BBS closed the connection.
func (c *Client) ReadByteTimeout(timeout time.Duration) (byte, error) {
	if len(c.payload) > 0 {
		b := c.payload[0]
		c.payload = c.payload[1:]
		return b, nil
	}
	if err := c.fill(timeout); err != nil {
		return 0, err
	}
	if len(c.payload) == 0 {
		return 0, ErrTimeout
	}
	b := c.payload[0]
	c.payload = c.payload[1:]
	return b, nil
}

// ReadAvailable returns all payload bytes currently available. It returns
// (nil, ErrTimeout) when nothing arrived before the deadline.
func (c *Client) ReadAvailable(timeout time.Duration) ([]byte, error) {
	if len(c.payload) == 0 {
		if err := c.fill(timeout); err != nil {
			if errors.Is(err, ErrTimeout) {
				return nil, ErrTimeout
			}
			return nil, err
		}
	}
	if len(c.payload) == 0 {
		return nil, ErrTimeout
	}
	data := c.payload
	c.payload = nil
	return data, nil
}

func (c *Client) fill(timeout time.Duration) error {
	if c.closed {
		return io.EOF
	}
	if timeout <= 0 {
		timeout = c.Opts.ReadTimeout
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 8192)
	n, err := c.conn.Read(buf)
	if n > 0 {
		if c.Opts.Trace != nil {
			c.Opts.Trace("recv", buf[:n])
		}
		c.parse(buf[:n])
		return nil
	}
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return ErrTimeout
		}
		if errors.Is(err, io.EOF) {
			c.closed = true
			return io.EOF
		}
		c.closed = true
		return fmt.Errorf("telnet read: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------- writing

// Write sends payload bytes, escaping literal IAC so the peer does not
// mistake data for Telnet commands.
func (c *Client) Write(data []byte) error {
	out := make([]byte, 0, len(data))
	for _, b := range data {
		out = append(out, b)
		if b == IAC {
			out = append(out, IAC)
		}
	}
	return c.writeRaw(out)
}

// writeRaw writes bytes verbatim (used for Telnet commands and subnegotiation,
// which must not be escaped).
func (c *Client) writeRaw(data []byte) error {
	if c.closed {
		return io.ErrClosedPipe
	}
	if c.Opts.Trace != nil {
		c.Opts.Trace("send", data)
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.Opts.ReadTimeout))
	_, err := c.conn.Write(data)
	return err
}

// WriteString sends a string.
func (c *Client) WriteString(s string) error { return c.Write([]byte(s)) }

// SendLine sends text terminated with CRLF, as BBSes expect.
func (c *Client) SendLine(s string) error { return c.Write([]byte(s + "\r\n")) }

func (c *Client) sendCommand(cmd, opt int) {
	_ = c.writeRaw([]byte{IAC, byte(cmd), byte(opt)})
}

func (c *Client) sendSubneg(opt int, payload []byte) {
	buf := []byte{IAC, SB, byte(opt)}
	buf = append(buf, payload...)
	buf = append(buf, IAC, SE)
	_ = c.writeRaw(buf)
}

func (c *Client) sendNAWS() {
	cols, rows := c.Opts.Cols, c.Opts.Rows
	c.sendSubneg(OptNAWS, []byte{byte(cols >> 8), byte(cols), byte(rows >> 8), byte(rows)})
}

// --------------------------------------------------------------- protocol

func (c *Client) parse(chunk []byte) {
	for _, b := range chunk {
		switch c.state {
		case stateData:
			if b == IAC {
				c.state = stateIAC
			} else {
				c.payload = append(c.payload, b)
			}
		case stateIAC:
			switch b {
			case IAC:
				c.payload = append(c.payload, IAC)
				c.state = stateData
			case DO, DONT, WILL, WONT:
				c.pendingCmd = b
				c.state = stateNegotiate
			case SB:
				c.sbBuf = c.sbBuf[:0]
				c.state = stateSB
			default:
				c.state = stateData
			}
		case stateNegotiate:
			c.negotiate(c.pendingCmd, b)
			c.state = stateData
		case stateSB:
			if b == IAC {
				c.state = stateSBIAC
			} else {
				c.sbBuf = append(c.sbBuf, b)
			}
		case stateSBIAC:
			switch b {
			case SE:
				c.handleSubneg(c.sbBuf)
				c.state = stateData
			case IAC:
				c.sbBuf = append(c.sbBuf, IAC)
				c.state = stateSB
			default:
				c.state = stateSB
			}
		}
	}
}

func (c *Client) negotiate(cmd, opt byte) {
	switch cmd {
	case DO:
		switch opt {
		case OptBinary, OptSGA, OptEcho, OptTTYPE, OptNAWS, OptNewEnviron, OptStatus:
			c.sendCommand(WILL, int(opt))
			if opt == OptNAWS {
				c.sendNAWS()
			}
		case OptLinemode:
			c.sendCommand(WONT, int(opt))
		default:
			if !c.refusedDo[opt] {
				c.refusedDo[opt] = true
				c.sendCommand(WONT, int(opt))
			}
		}
	case WILL:
		switch opt {
		case OptBinary, OptSGA, OptEcho:
			c.sendCommand(DO, int(opt))
		default:
			if !c.refusedWill[opt] {
				c.refusedWill[opt] = true
				c.sendCommand(DONT, int(opt))
			}
		}
	case WONT, DONT:
		// Nothing to do.
	}
}

func (c *Client) handleSubneg(data []byte) {
	if len(data) == 0 {
		return
	}
	opt, body := data[0], data[1:]
	switch opt {
	case OptTTYPE:
		if len(body) == 0 || body[0] == ttypeSend {
			payload := []byte{ttypeIs}
			payload = append(payload, []byte(c.Opts.TerminalType)...)
			c.sendSubneg(OptTTYPE, payload)
		}
	case OptNewEnviron:
		if len(body) > 0 && (body[0] == 0x01 || body[0] == 0x00) {
			payload := []byte{0x00}
			payload = append(payload, []byte("TERM\x01")...)
			payload = append(payload, []byte(c.Opts.TerminalType)...)
			c.sendSubneg(OptNewEnviron, payload)
		}
	}
}

// Drain reads and discards pending input for up to d.
func (c *Client) Drain(d time.Duration) []byte {
	deadline := time.Now().Add(d)
	var out []byte
	for time.Now().Before(deadline) {
		data, err := c.ReadAvailable(20 * time.Millisecond)
		if err != nil {
			if errors.Is(err, ErrTimeout) {
				continue
			}
			break
		}
		out = append(out, data...)
	}
	return out
}
