package config

import (
	"encoding/json"
	"testing"
)

func TestDefaultsAreValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.BBS.Port != 23 || cfg.BBS.Cols != 80 || cfg.BBS.Rows != 24 {
		t.Errorf("unexpected BBS defaults: %+v", cfg.BBS)
	}
	if len(cfg.Behavior.Goals) == 0 {
		t.Error("no default goals")
	}
	if cfg.Downloads.ProtocolKeys["zmodem"] != "Z" {
		t.Errorf("protocol key missing: %+v", cfg.Downloads.ProtocolKeys)
	}
	if cfg.AI.Model == "" || cfg.AI.BaseURL == "" {
		t.Errorf("AI defaults incomplete: %+v", cfg.AI)
	}
	if cfg.StateDir() == "" || cfg.Downloads.Dir == "" {
		t.Error("directories not resolved")
	}
}

func TestExampleRoundTrip(t *testing.T) {
	data, err := Example()
	if err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Registration.Answers) == 0 {
		t.Error("registration answers lost")
	}
}

func TestNormalizeRejectsBadPort(t *testing.T) {
	cfg := Default()
	cfg.BBS.Port = 70000
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected an error for an out-of-range port")
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Vertrauen":         "vertrauen",
		"The Keep  BBS!!":   "the-keep-bbs",
		"  ":                "bbs",
		"Synchronet / Test": "synchronet-test",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
