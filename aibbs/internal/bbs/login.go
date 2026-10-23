package bbs

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/mro-1337/ai.shit/aibbs/internal/config"
)

// LoginResult reports how authentication went.
type LoginResult struct {
	Handle            string
	Registered        bool
	GeneratedPassword bool
}

var (
	mainMenuPatterns = []string{
		`(?i)\bmain\s*menu\b`,
		`(?i)\[main\]`,
		`(?i)main\s+(command|prompt)`,
		`(?i)enter your choice`,
		`(?i)your choice\s*[:?]`,
		`(?i)command\s*[:>]`,
	}
	verifyPasswordRe = regexp.MustCompile(`(?i)(verify|re-?enter|confirm|again).*pass|pass.*(verify|again|confirm)`)
	passwordRe       = regexp.MustCompile(`(?i)(pass ?word|passcode)`)
	realNameRe       = regexp.MustCompile(`(?i)(real|full) name|name in real life`)
	emailRe          = regexp.MustCompile(`(?i)e-?mail`)
	locationRe       = regexp.MustCompile(`(?i)(location|city|where are you|where.*from)`)
	terminalRe       = regexp.MustCompile(`(?i)(terminal|ansi|graphics|emulation)`)
	phoneOrBirthRe   = regexp.MustCompile(`(?i)(phone|birth|age|sex|gender)`)
	genericNameRe    = regexp.MustCompile(`(?i)enter your (first )?name|your (first )?name|login\s*[:?]|handle\s*[:?]|enter (your )?handle|what is your name`)
	yesNoRe          = regexp.MustCompile(`(?i)(\(y/n\)|\[y/n\]|yes/no|\? \(y\)|continue\?|correct\?|ok\?|are you sure)`)
	newUserRe        = regexp.MustCompile(`(?i)(new user|create|register|sign ?up|not in use|no such user|unknown user|not found|invalid (login|handle|name)|incorrect|wrong password)`)
	failureRe        = regexp.MustCompile(`(?i)(incorrect|invalid|wrong)\s+(password|login|handle)|unknown user|no such user|access denied|login failed`)
	anyKeyRe         = regexp.MustCompile(`(?i)(hit|press)\s+(any|a)\s+key`)
	namePromptRe     = regexp.MustCompile(`(?i)(enter your name|your name|name \(or|new user|handle|login)\s*[:?]?\s*$`)
)

// Login authenticates, registering a new account when allowed and needed.
func (s *Session) Login(ctx context.Context) (LoginResult, error) {
	if err := s.Settle(); err != nil {
		return LoginResult{}, err
	}
	result := LoginResult{Handle: s.Cfg.Account.Handle}
	if s.atMainMenu() {
		return result, nil
	}

	s.password = s.Cfg.Account.Password
	if s.password == "" {
		s.password = s.Mem.Password
	}
	if s.password == "" {
		s.password = generatePassword(s.Cfg.Registration.PasswordLength)
		s.generated = true
	}

	registering := false
	sentHandle := false
	for turn := 0; turn < 45; turn++ {
		if s.disconnected {
			return result, fmt.Errorf("connection closed during login")
		}
		if s.atMainMenu() {
			result.Registered = registering
			result.GeneratedPassword = s.generated
			s.rememberAccount(registering)
			return result, nil
		}

		last := s.lastMeaningfulLines(1)
		rule := s.matchAnswerRule(last)
		switch {
		case verifyPasswordRe.MatchString(last):
			s.Log.Debug("login: verifying password")
			if err := s.SendLine(s.password); err != nil {
				return result, err
			}
		case passwordRe.MatchString(last):
			s.Log.Debug("login: sending password")
			if err := s.SendLine(s.password); err != nil {
				return result, err
			}
		case rule != nil:
			answer := s.answerValue(rule, last)
			s.Log.Debug("login: answering signup question", "prompt", last, "answer", answer)
			if err := s.SendLine(answer); err != nil {
				return result, err
			}
			registering = true
		case realNameRe.MatchString(last):
			if err := s.SendLine(s.Cfg.Account.RealName); err != nil {
				return result, err
			}
			registering = true
		case emailRe.MatchString(last):
			if err := s.SendLine(s.Cfg.Account.Email); err != nil {
				return result, err
			}
			registering = true
		case locationRe.MatchString(last):
			if err := s.SendLine(s.Cfg.Account.Location); err != nil {
				return result, err
			}
			registering = true
		case phoneOrBirthRe.MatchString(last):
			if err := s.SendLine(""); err != nil {
				return result, err
			}
			registering = true
		case terminalRe.MatchString(last):
			if err := s.SendLine(s.Cfg.Registration.TerminalAnswer); err != nil {
				return result, err
			}
			registering = true
		case failureRe.MatchString(last):
			if !s.Cfg.Account.AllowRegister {
				return result, fmt.Errorf("login failed: %s", last)
			}
			registering = true
			sentHandle = false
			s.Log.Info("login failed, trying to register", "screen", last)
			if yesNoRe.MatchString(last) {
				if err := s.SendLine("Y"); err != nil {
					return result, err
				}
			} else if err := s.SendLine(s.Cfg.Registration.NewUserCommand); err != nil {
				return result, err
			}
		case genericNameRe.MatchString(last) || namePromptRe.MatchString(last):
			if !sentHandle || registering {
				if err := s.SendLine(s.Cfg.Account.Handle); err != nil {
					return result, err
				}
				sentHandle = true
			} else if err := s.SendLine(""); err != nil {
				return result, err
			}
		case newUserRe.MatchString(last):
			if !s.Cfg.Account.AllowRegister {
				return result, fmt.Errorf("unknown account and registration disabled: %s", last)
			}
			registering = true
			s.Log.Info("login: answering new-user prompt")
			if yesNoRe.MatchString(last) {
				if err := s.SendLine("Y"); err != nil {
					return result, err
				}
			} else if !sentHandle {
				if err := s.SendLine(s.Cfg.Registration.NewUserCommand); err != nil {
					return result, err
				}
				sentHandle = true
			}
		case yesNoRe.MatchString(last):
			if err := s.SendLine("Y"); err != nil {
				return result, err
			}
		case anyKeyRe.MatchString(last):
			if err := s.SendKeys("<SPACE>"); err != nil {
				return result, err
			}
		default:
			// Unknown prompt: a blank line is the least destructive answer.
			if err := s.SendLine(""); err != nil {
				return result, err
			}
		}

		if err := s.Settle(); err != nil {
			return result, err
		}
	}
	return result, fmt.Errorf("login did not reach a main menu; last screen:\n%s", s.screen.Text())
}

func (s *Session) atMainMenu() bool {
	text := s.screen.Text()
	for _, p := range mainMenuPatterns {
		if re, err := regexp.Compile(p); err == nil && re.MatchString(text) {
			return true
		}
	}
	return false
}

func (s *Session) lastMeaningfulLines(n int) string {
	lines := strings.Split(s.screen.Text(), "\n")
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append(out, strings.TrimSpace(lines[i]))
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return strings.Join(out, " | ")
}

func (s *Session) matchAnswerRule(text string) *config.AnswerRule {
	for i := range s.Cfg.Registration.Answers {
		rule := &s.Cfg.Registration.Answers[i]
		if re, err := regexp.Compile(rule.Match); err == nil && re.MatchString(text) {
			return rule
		}
	}
	return nil
}

func (s *Session) answerValue(rule *config.AnswerRule, prompt string) string {
	switch strings.ToUpper(strings.TrimSpace(rule.Value)) {
	case "AUTO:REALNAME":
		return s.Cfg.Account.RealName
	case "AUTO:EMAIL":
		return s.Cfg.Account.Email
	case "AUTO:LOCATION":
		return s.Cfg.Account.Location
	case "AUTO:TERMINAL":
		return s.Cfg.Registration.TerminalAnswer
	case "AUTO:BLANK":
		return ""
	case "":
		return "Y"
	}
	return rule.Value
}

func (s *Session) rememberAccount(registered bool) {
	s.Mem.Handle = s.Cfg.Account.Handle
	s.Mem.Joined = s.Mem.Joined || registered
	if s.generated && s.Mem.Password == "" {
		s.Mem.Password = s.password
	}
	if err := s.Mem.Save(); err != nil {
		s.Log.Warn("could not save memory", "err", err)
	}
}

func generatePassword(length int) string {
	if length < 8 {
		length = 12
	}
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b.WriteByte(alphabet[i%len(alphabet)])
			continue
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String()
}
