// Package bbs drives one Telnet session with a BBS.
//
// It owns the screen, settles on "quiet" periods so the agent always reasons
// about a stable picture, handles login/sign-up, and hands the raw stream to
// the file transfer receivers when a download starts.
package bbs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/doors"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
	"github.com/mro-1337/ai.shit/aibbs/internal/telnet"
	"github.com/mro-1337/ai.shit/aibbs/internal/term"
)

// Observation is what the agent sees when it wakes up.
type Observation struct {
	Snapshot     term.Snapshot
	Recent       string
	Door         *doors.Profile
	Disconnected bool
}

// Session is a live connection to a BBS.
type Session struct {
	Cfg          *config.Config
	Mem          *memory.Store
	Log          *slog.Logger
	DoorProfiles []doors.Profile

	client       *telnet.Client
	screen       *term.Screen
	pending      []string
	password     string
	generated    bool
	disconnected bool
	lastPager    string
}

// Dial connects and prepares the virtual screen.
func Dial(ctx context.Context, cfg *config.Config, mem *memory.Store, log *slog.Logger) (*Session, error) {
	if log == nil {
		log = slog.Default()
	}
	opts := telnet.Options{
		Cols:         cfg.BBS.Cols,
		Rows:         cfg.BBS.Rows,
		TerminalType: cfg.BBS.Terminal,
		DialTimeout:  time.Duration(cfg.BBS.DialTimeoutSec) * time.Second,
		ReadTimeout:  time.Duration(cfg.BBS.MaxSettleSeconds+10) * time.Second,
	}
	client, err := telnet.Dial(ctx, cfg.BBS.Host, cfg.BBS.Port, opts)
	if err != nil {
		return nil, err
	}
	s := &Session{
		Cfg:          cfg,
		Mem:          mem,
		Log:          log,
		client:       client,
		screen:       term.New(cfg.BBS.Cols, cfg.BBS.Rows, 2000),
		DoorProfiles: doors.WithOverrides(cfg.Doors.Overrides),
	}
	s.screen.OnScroll(func(line string) {
		s.pending = append(s.pending, line)
		if len(s.pending) > 400 {
			s.pending = s.pending[len(s.pending)-400:]
		}
	})
	return s, nil
}

// Close terminates the connection.
func (s *Session) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Disconnected reports whether the BBS hung up.
func (s *Session) Disconnected() bool { return s.disconnected }

// Screen exposes the virtual terminal (read-only use).
func (s *Session) Screen() *term.Screen { return s.screen }

// Snapshot returns the current screen without waiting.
func (s *Session) Snapshot() term.Snapshot { return s.screen.Snapshot() }

// ---------------------------------------------------------------- settling

// Settle reads until the line has been quiet for the configured idle time,
// or until the maximum settle time elapses. Auto-paging (--More--, "hit any
// key") is handled here so the model never sees a stuck pager.
func (s *Session) Settle() error {
	idle := time.Duration(s.Cfg.BBS.IdleMillis) * time.Millisecond
	maxWait := time.Duration(s.Cfg.BBS.MaxSettleSeconds) * time.Second
	deadline := time.Now().Add(maxWait)
	lastData := time.Now()
	lastPager := ""

	for {
		data, err := s.client.ReadAvailable(60 * time.Millisecond)
		if err != nil {
			if errors.Is(err, telnet.ErrTimeout) {
				if time.Since(lastData) >= idle {
					break
				}
			} else if errors.Is(err, io.EOF) {
				s.disconnected = true
				break
			} else {
				s.disconnected = true
				return err
			}
		} else if len(data) > 0 {
			s.screen.Feed(data)
			lastData = time.Now()
		}

		if s.Cfg.Behavior.AutoPager && time.Since(lastData) >= 120*time.Millisecond {
			if prompt := s.pagerPrompt(); prompt != "" && prompt != lastPager {
				lastPager = prompt
				_ = s.SendKeys("<SPACE>")
				lastData = time.Now()
			}
		}
		if time.Now().After(deadline) {
			break
		}
	}
	s.Log.Debug("screen settled", "text", s.screen.Text())
	return nil
}

func (s *Session) pagerPrompt() string {
	lines := strings.Split(s.screen.Text(), "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-3; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		for _, re := range pagerPatterns {
			if re.MatchString(line) {
				return line
			}
		}
		return ""
	}
	return ""
}

var pagerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\[?\s*-+\s*more\s*-+\s*\]?$`),
	regexp.MustCompile(`(?i)^\((more|press any key|hit any key)[^)]*\)$`),
	regexp.MustCompile(`(?i)^-{0,2}\s*more\s*:?\s*-{0,2}$`),
	regexp.MustCompile(`(?i)hit (any|a) key`),
	regexp.MustCompile(`(?i)press (any|a) key`),
	regexp.MustCompile(`(?i)press \[?enter\]?`),
}

// Observe waits for a stable screen and reports what is on it.
func (s *Session) Observe() (Observation, error) {
	if err := s.Settle(); err != nil {
		return Observation{}, err
	}
	recent := strings.Join(s.pending, "\n")
	s.pending = nil
	text := s.screen.Text()
	door := doors.Detect(s.DoorProfiles, text+"\n"+recent)
	return Observation{
		Snapshot:     s.screen.Snapshot(),
		Recent:       recent,
		Door:         door,
		Disconnected: s.disconnected,
	}, nil
}

// ---------------------------------------------------------------- sending

// SendText writes text without a newline.
func (s *Session) SendText(text string) error {
	return s.client.Write([]byte(text))
}

// SendLine writes text followed by CRLF.
func (s *Session) SendLine(text string) error {
	return s.client.SendLine(text)
}

var keyToken = regexp.MustCompile(`<([A-Za-z0-9_-]+)>`)

// SendKeys writes key sequences. Tokens like <ENTER>, <UP> and <CTRL-C> are
// translated; everything else is sent literally.
func (s *Session) SendKeys(keys string) error {
	var b strings.Builder
	last := 0
	for _, loc := range keyToken.FindAllStringSubmatchIndex(keys, -1) {
		b.WriteString(keys[last:loc[0]])
		token := strings.ToUpper(keys[loc[2]:loc[3]])
		b.WriteString(keyValue(token))
		last = loc[1]
	}
	b.WriteString(keys[last:])
	if b.Len() == 0 {
		return nil
	}
	return s.client.Write([]byte(b.String()))
}

func keyValue(token string) string {
	switch token {
	case "ENTER", "CR", "RETURN":
		return "\r"
	case "LF":
		return "\n"
	case "TAB":
		return "\t"
	case "ESC", "ESCAPE":
		return "\x1b"
	case "BS", "BACKSPACE":
		return "\x08"
	case "SPACE":
		return " "
	case "UP":
		return "\x1b[A"
	case "DOWN":
		return "\x1b[B"
	case "RIGHT":
		return "\x1b[C"
	case "LEFT":
		return "\x1b[D"
	case "HOME":
		return "\x1b[H"
	case "END":
		return "\x1b[F"
	case "PGUP":
		return "\x1b[5~"
	case "PGDN":
		return "\x1b[6~"
	case "INSERT":
		return "\x1b[2~"
	case "DELETE":
		return "\x1b[3~"
	case "F1":
		return "\x1bOP"
	case "F2":
		return "\x1bOQ"
	case "F3":
		return "\x1bOR"
	case "F4":
		return "\x1bOS"
	case "F5":
		return "\x1b[15~"
	case "F6":
		return "\x1b[17~"
	case "F7":
		return "\x1b[18~"
	case "F8":
		return "\x1b[19~"
	case "F9":
		return "\x1b[20~"
	case "F10":
		return "\x1b[21~"
	case "CTRL-C", "CTRLC", "^C":
		return "\x03"
	case "CTRL-X":
		return "\x18"
	case "CTRL-Z":
		return "\x1a"
	}
	if strings.HasPrefix(token, "CTRL-") && len(token) == 6 {
		c := token[5]
		if c >= 'A' && c <= 'Z' {
			return string(rune(c - 'A' + 1))
		}
	}
	return "<" + token + ">"
}

// WaitFor waits until one of the patterns matches the screen.
func (s *Session) WaitFor(patterns []string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	for {
		if err := s.Settle(); err != nil {
			return "", err
		}
		text := s.screen.Text()
		for i, re := range compiled {
			if re.MatchString(text) {
				return patterns[i], nil
			}
		}
		if s.disconnected {
			return "", io.EOF
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for %v", patterns)
		}
	}
}

// DrainFor reads and discards input for d (used before a transfer).
func (s *Session) DrainFor(d time.Duration) { s.client.Drain(d) }
