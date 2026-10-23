package bbs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/telnet"
	"github.com/mro-1337/ai.shit/aibbs/internal/transfer"
)

// telnetStream adapts the Telnet client to transfer.Stream and translates
// the timeout sentinel so the transfer package stays protocol-only.
type telnetStream struct{ c *telnet.Client }

func (t telnetStream) ReadByteTimeout(d time.Duration) (byte, error) {
	b, err := t.c.ReadByteTimeout(d)
	if errors.Is(err, telnet.ErrTimeout) {
		return 0, transfer.ErrTimeout
	}
	return b, err
}

func (t telnetStream) Write(data []byte) error { return t.c.Write(data) }

// Download starts a file transfer and drives the selected protocol.
//
// The BBS is normally already sitting at a "protocol?" prompt, so the key is
// sent without a newline and the receiver takes over the raw stream at once.
func (s *Session) Download(protocol string) ([]transfer.ReceivedFile, error) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		protocol = "zmodem"
	}
	key := s.Cfg.Downloads.ProtocolKeys[protocol]
	if key == "" {
		switch protocol {
		case "zmodem", "z":
			protocol, key = "zmodem", "Z"
		case "ymodem", "y":
			protocol, key = "ymodem", "Y"
		case "xmodem", "x":
			protocol, key = "xmodem", "X"
		default:
			return nil, fmt.Errorf("unsupported transfer protocol %q", protocol)
		}
	}

	allowed := false
	for _, p := range s.Cfg.Downloads.Protocols {
		if strings.EqualFold(p, protocol) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("protocol %q is disabled in downloads.protocols", protocol)
	}

	s.Log.Info("starting download", "protocol", protocol, "key", key)
	if err := s.SendText(key); err != nil {
		return nil, err
	}

	stream := telnetStream{c: s.client}
	idle := time.Duration(s.Cfg.Downloads.IdleSeconds) * time.Second
	logf := func(format string, args ...any) {
		s.Log.Debug("transfer: " + fmt.Sprintf(format, args...))
	}

	var (
		files []transfer.ReceivedFile
		err   error
	)
	switch protocol {
	case "zmodem":
		files, err = transfer.ReceiveZModem(stream, s.Cfg.Downloads.Dir, idle, logf)
	default:
		files, err = transfer.ReceiveXModem(stream, s.Cfg.Downloads.Dir, transfer.XModemOptions{
			YModem:   protocol == "ymodem",
			Timeout:  10 * time.Second,
			MaxBytes: int64(s.Cfg.Downloads.MaxFileMB) << 20,
		}, logf)
	}

	// Consume whatever the BBS printed after the transfer so the next
	// observation starts from a clean screen.
	if serr := s.Settle(); serr != nil && err == nil {
		err = serr
	}
	for _, f := range files {
		s.Mem.AddDownload(f.Name, f.Path, f.Bytes)
	}
	if len(files) > 0 {
		if serr := s.Mem.Save(); serr != nil {
			s.Log.Warn("could not save download history", "err", serr)
		}
	}
	if err != nil {
		return files, fmt.Errorf("download (%s): %w", protocol, err)
	}
	return files, nil
}

// Logoff politely leaves the BBS: Q for quit, then confirm if asked.
func (s *Session) Logoff() error {
	if s.disconnected || s.client == nil {
		return nil
	}
	for attempt := 0; attempt < 4; attempt++ {
		if err := s.SendKeys("Q"); err != nil {
			break
		}
		if err := s.Settle(); err != nil {
			break
		}
		text := strings.ToLower(s.screen.Text())
		if s.disconnected || strings.Contains(text, "logg") || strings.Contains(text, "goodbye") ||
			strings.Contains(text, "thanks for calling") || strings.Contains(text, "call back") {
			break
		}
		if yesNoRe.MatchString(text) {
			_ = s.SendLine("Y")
			_ = s.Settle()
		}
	}
	return s.Close()
}

// WaitForLoginPrompt waits for a sign-on screen without sending anything.
func (s *Session) WaitForLoginPrompt(timeout time.Duration) (string, error) {
	patterns := []string{
		`(?i)enter your name`,
		`(?i)login`,
		`(?i)handle`,
		`(?i)password`,
		`(?i)press any key`,
		`(?i)new user`,
		`(?i)main menu`,
	}
	return s.WaitFor(patterns, timeout)
}

// SaveMemory persists session state.
func (s *Session) SaveMemory() error { return s.Mem.Save() }
