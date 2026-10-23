// Package fakebbs is a small, self-contained BBS used for demos and tests.
//
// It speaks Telnet (TTYPE/NAWS negotiation), paints ANSI screens, supports
// new-user registration, message areas with posting and replies, a file area
// with real ZMODEM/XMODEM downloads, and two door games. That gives the agent
// a complete playground without touching a real BBS.
package fakebbs

import (
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// User is a registered account.
type User struct {
	Handle   string
	Password string
	RealName string
	Email    string
	Location string
}

// Message is a posted message.
type Message struct {
	ID      int
	Area    string
	From    string
	To      string
	Subject string
	Body    string
	At      time.Time
}

// Stats counts what test clients did, which lets tests assert behaviour.
type Stats struct {
	Registrations int
	Logins        int
	Reads         int
	Posts         int
	Transfers     int
	DoorPlays     int
	Logoffs       int
}

// Server is the mock BBS.
type Server struct {
	ln  net.Listener
	log *slog.Logger

	mu     sync.Mutex
	users  map[string]*User
	msgs   []*Message
	nextID int
	stats  Stats
	conns  map[net.Conn]struct{}
	files  map[string]*FileEntry
	closed bool
}

// FileEntry is a downloadable file.
type FileEntry struct {
	Name string
	Desc string
	Data []byte
}

// Start listens on addr (e.g. "127.0.0.1:0") and serves until Close.
func Start(addr string) (*Server, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		ln:    ln,
		log:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		users: map[string]*User{},
		conns: map[net.Conn]struct{}{},
		files: map[string]*FileEntry{},
	}
	s.seed()
	go s.accept()
	return s, nil
}

// Addr returns the host:port the server is bound to.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Close stops the listener and all connections.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	for c := range s.conns {
		_ = c.Close()
	}
	s.conns = map[net.Conn]struct{}{}
	s.mu.Unlock()
	return s.ln.Close()
}

// Stats returns a snapshot of server counters.
func (s *Server) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// Messages returns the messages in an area.
func (s *Server) Messages(area string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Message
	for _, m := range s.msgs {
		if m.Area == area {
			out = append(out, *m)
		}
	}
	return out
}

// SeedUser adds a pre-existing account.
func (s *Server) SeedUser(handle, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[strings.ToLower(handle)] = &User{Handle: handle, Password: password, RealName: handle}
}

func (s *Server) accept() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns[conn] = struct{}{}
		s.mu.Unlock()
		go func() {
			defer func() {
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
				_ = conn.Close()
			}()
			s.serve(conn)
		}()
	}
}

func (s *Server) serve(conn net.Conn) {
	t := newTConn(conn)
	// A real BBS probes the terminal; doing the same keeps the client's
	// negotiation code honest.
	t.writeRaw([]byte{255, 251, 3, 255, 251, 1, 255, 253, 24, 255, 253, 31})
	c := &connState{srv: s, t: t}
	c.run()
}
