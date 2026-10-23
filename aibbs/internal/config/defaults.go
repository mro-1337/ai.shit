package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Default returns a config with sane values and a friendly persona.
func Default() *Config {
	return &Config{
		BBS: BBSConfig{
			Name:             "localhost",
			Host:             "127.0.0.1",
			Port:             23,
			Cols:             80,
			Rows:             24,
			Terminal:         "ANSI",
			DialTimeoutSec:   15,
			IdleMillis:       450,
			MaxSettleSeconds: 8,
		},
		Account: AccountConfig{
			Handle:        "SynthMind",
			RealName:      "Synth Mind",
			Email:         "synthmind@example.invalid",
			Location:      "Cyberspace",
			AllowRegister: true,
		},
		AI: AIConfig{
			Provider:       "openai",
			BaseURL:        "https://api.openai.com/v1",
			APIKeyEnv:      "OPENAI_API_KEY",
			Model:          "gpt-4o-mini",
			Temperature:    0.7,
			MaxTokens:      700,
			TimeoutSec:     60,
			JSONMode:       true,
			TrimScreenRows: 60,
		},
		Behavior: BehaviorConfig{
			Persona: "You are SynthMind, a curious, friendly AI that hangs out on BBSes. " +
				"You love old-school text interfaces, phreaking history, retro computing, " +
				"science fiction, and good conversation. You are honest about being an AI.",
			Style: "Warm, concise, a little playful. No emoji, no hashtags. Plain text, " +
				"80 columns. Never pretend to be human if asked directly.",
			Goals: []string{
				"Read the message areas, find a thread that interests you, and reply to it thoughtfully.",
				"Browse the file areas and download one small file that looks interesting.",
				"Play a door game for a few turns and then return to the main menu.",
			},
			Interests:              []string{"retro computing", "BBS culture", "science fiction", "programming", "space"},
			MaxSteps:               80,
			MinActionDelayMS:       900,
			MaxPostsPerSession:     3,
			MaxDownloadsPerSession: 2,
			MaxDownloadMB:          20,
			MaxReadMessages:        25,
			MinPostChars:           12,
			MaxPostChars:           1800,
			AllowDoors:             true,
			AutoPager:              true,
			StateDir:               "state",
			Transcript:             true,
		},
		Registration: RegistrationConfig{
			NewUserCommand: "NEW",
			TerminalAnswer: "ANSI",
			PasswordLength: 14,
			Answers: []AnswerRule{
				{Match: `(?i)(real|full)\s+name`, Value: "AUTO:realname"},
				{Match: `(?i)e-?mail`, Value: "AUTO:email"},
				{Match: `(?i)(location|city|where\s+are\s+you\s+from)`, Value: "AUTO:location"},
				{Match: `(?i)(phone|birth|age)`, Value: "AUTO:blank"},
				{Match: `(?i)(terminal|ansi|graphics)`, Value: "AUTO:terminal"},
				{Match: `(?i)(\(y/n\)|\[y/n\]|yes/no|continue\?|correct\?|ok\?)`, Value: "Y"},
			},
		},
		Downloads: DownloadsConfig{
			Dir:          "downloads",
			Protocols:    []string{"zmodem", "ymodem", "xmodem"},
			ProtocolKeys: map[string]string{"zmodem": "Z", "ymodem": "Y", "xmodem": "X"},
			IdleSeconds:  90,
			MaxFileMB:    20,
		},
		Doors: DoorsConfig{Enabled: true, MaxActions: 40},
	}
}

// Load reads a config file, applying defaults for anything omitted.
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Slug turns a BBS name into a filename-safe directory name.
func Slug(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "bbs"
	}
	return out
}

// StateDir is the per-BBS directory for memory and transcripts.
func (c *Config) StateDir() string {
	return filepath.Join(c.Behavior.StateDir, Slug(c.BBS.Name))
}

// Normalize fills in derived values and validates the config.
func (c *Config) Normalize() error {
	b := &c.BBS
	if strings.TrimSpace(b.Name) == "" {
		b.Name = "bbs"
	}
	if b.Host == "" {
		return fmt.Errorf("bbs.host is required")
	}
	if b.Port == 0 {
		b.Port = 23
	}
	if b.Port < 1 || b.Port > 65535 {
		return fmt.Errorf("bbs.port %d is out of range", b.Port)
	}
	if b.Cols < 40 {
		b.Cols = 80
	}
	if b.Rows < 10 {
		b.Rows = 24
	}
	if b.Terminal == "" {
		b.Terminal = "ANSI"
	}
	if b.DialTimeoutSec <= 0 {
		b.DialTimeoutSec = 15
	}
	if b.IdleMillis < 100 {
		b.IdleMillis = 450
	}
	if b.MaxSettleSeconds <= 0 {
		b.MaxSettleSeconds = 8
	}

	a := &c.Account
	if a.Handle == "" {
		a.Handle = "SynthMind"
	}
	if a.Password == "" && a.PasswordEnv != "" {
		a.Password = os.Getenv(a.PasswordEnv)
	}
	if a.RealName == "" {
		a.RealName = a.Handle
	}
	if a.Location == "" {
		a.Location = "Cyberspace"
	}

	ai := &c.AI
	if ai.Provider == "" {
		ai.Provider = "openai"
	}
	if ai.Provider == "ollama" {
		if ai.BaseURL == "" {
			ai.BaseURL = "http://127.0.0.1:11434/v1"
		}
		if ai.Model == "" {
			ai.Model = "llama3.1"
		}
	}
	if ai.BaseURL == "" {
		ai.BaseURL = "https://api.openai.com/v1"
	}
	if ai.Model == "" {
		ai.Model = "gpt-4o-mini"
	}
	if ai.APIKey == "" && ai.APIKeyEnv != "" {
		ai.APIKey = os.Getenv(ai.APIKeyEnv)
	}
	if ai.Temperature < 0 || ai.Temperature > 2 {
		ai.Temperature = 0.7
	}
	if ai.MaxTokens <= 0 {
		ai.MaxTokens = 700
	}
	if ai.TimeoutSec <= 0 {
		ai.TimeoutSec = 60
	}
	if ai.TrimScreenRows <= 0 {
		ai.TrimScreenRows = 60
	}

	be := &c.Behavior
	if len(be.Goals) == 0 {
		be.Goals = []string{"Explore the BBS, read messages, and leave one thoughtful reply."}
	}
	if be.MaxSteps <= 0 {
		be.MaxSteps = 80
	}
	if be.MinActionDelayMS < 0 {
		be.MinActionDelayMS = 0
	}
	if be.MaxPostsPerSession <= 0 {
		be.MaxPostsPerSession = 3
	}
	if be.MaxDownloadsPerSession <= 0 {
		be.MaxDownloadsPerSession = 2
	}
	if be.MaxDownloadMB <= 0 {
		be.MaxDownloadMB = 20
	}
	if be.MaxReadMessages <= 0 {
		be.MaxReadMessages = 25
	}
	if be.MinPostChars <= 0 {
		be.MinPostChars = 12
	}
	if be.MaxPostChars <= 0 {
		be.MaxPostChars = 1800
	}
	if be.MaxPostChars < be.MinPostChars {
		be.MaxPostChars = be.MinPostChars
	}
	if be.StateDir == "" {
		be.StateDir = "state"
	}

	reg := &c.Registration
	if reg.NewUserCommand == "" {
		reg.NewUserCommand = "NEW"
	}
	if reg.TerminalAnswer == "" {
		reg.TerminalAnswer = "ANSI"
	}
	if reg.PasswordLength < 8 {
		reg.PasswordLength = 14
	}
	for _, rule := range reg.Answers {
		if _, err := regexp.Compile(rule.Match); err != nil {
			return fmt.Errorf("registration answer pattern %q: %w", rule.Match, err)
		}
	}

	d := &c.Downloads
	if d.Dir == "" {
		d.Dir = filepath.Join("downloads", Slug(b.Name))
	}
	if len(d.Protocols) == 0 {
		d.Protocols = []string{"zmodem", "ymodem", "xmodem"}
	}
	if d.ProtocolKeys == nil {
		d.ProtocolKeys = map[string]string{"zmodem": "Z", "ymodem": "Y", "xmodem": "X"}
	}
	if d.IdleSeconds <= 0 {
		d.IdleSeconds = 90
	}
	if d.MaxFileMB <= 0 {
		d.MaxFileMB = 20
	}
	if c.Doors.MaxActions <= 0 {
		c.Doors.MaxActions = 40
	}
	return nil
}

// Example returns the JSON written by "aibbs init".
func Example() ([]byte, error) {
	cfg := Default()
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// WriteExample writes a ready-to-edit config file.
func WriteExample(path string) error {
	data, err := Example()
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
