package agent

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

func testPolicy(t *testing.T) (*Policy, *memory.Store) {
	t.Helper()
	cfg := config.Default()
	cfg.BBS.Name = "policy-test"
	cfg.Behavior.StateDir = t.TempDir()
	cfg.Behavior.MinActionDelayMS = 0
	cfg.Behavior.BlockedWords = []string{"spamword"}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	mem, err := memory.Load(filepath.Join(cfg.StateDir(), "memory.json"), cfg.BBS.Name)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewPolicy(cfg, mem, log), mem
}

const protocolScreen = "File: ART.ANS\nProtocol (Z=ZMODEM, X=XMODEM, Y=YMODEM, Q=Quit): "
const composeScreen = "Subject: hello\nEnter your message. End with a line containing only a period (.)."

func TestDownloadNeedsProtocolPrompt(t *testing.T) {
	p, _ := testPolicy(t)
	d := ai.Decision{Action: ai.ActionDownload, Protocol: "zmodem"}
	if _, err := p.Apply(context.Background(), d, "Main Menu\nChoice:"); err == nil {
		t.Error("download without a protocol prompt should be blocked")
	}
	if _, err := p.Apply(context.Background(), d, protocolScreen); err != nil {
		t.Errorf("download at a protocol prompt was blocked: %v", err)
	}
}

func TestReadOnlyBlocksPostingAndDownloads(t *testing.T) {
	p, _ := testPolicy(t)
	p.cfg.Behavior.ReadOnly = true
	post := ai.Decision{Action: ai.ActionText, Value: "This is a long enough reply to pass the length check."}
	if _, err := p.Apply(context.Background(), post, composeScreen); err == nil {
		t.Error("read-only should block posting")
	}
	if _, err := p.Apply(context.Background(), ai.Decision{Action: ai.ActionDownload, Protocol: "zmodem"}, protocolScreen); err == nil {
		t.Error("read-only should block downloads")
	}
}

func TestDryRunTurnsPostIntoNote(t *testing.T) {
	p, _ := testPolicy(t)
	p.cfg.Behavior.DryRun = true
	d := ai.Decision{Action: ai.ActionText, Value: "This is a long enough reply to pass the length check."}
	got, err := p.Apply(context.Background(), d, composeScreen)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != ai.ActionNote {
		t.Errorf("action = %q, want note", got.Action)
	}
	if !strings.Contains(got.Note, "dry-run") {
		t.Errorf("note = %q", got.Note)
	}
}

func TestBlockedWordsAndSecrets(t *testing.T) {
	p, _ := testPolicy(t)
	bad := ai.Decision{Action: ai.ActionText, Value: "please buy spamword now, it is great"}
	if _, err := p.Apply(context.Background(), bad, composeScreen); err == nil {
		t.Error("blocked word was allowed")
	}
	p.cfg.Account.Password = "supersecret"
	secret := ai.Decision{Action: ai.ActionText, Value: "my password is supersecret"}
	if _, err := p.Apply(context.Background(), secret, composeScreen); err == nil {
		t.Error("credential was allowed")
	}
}

func TestTooShortAndTooSimilar(t *testing.T) {
	p, mem := testPolicy(t)
	short := ai.Decision{Action: ai.ActionText, Value: "hi"}
	if _, err := p.Apply(context.Background(), short, composeScreen); err == nil {
		t.Error("short post was allowed")
	}
	body := "I really enjoyed reading about the history of this board and its phone lines."
	mem.AddPost("General", "history", body)
	dup := ai.Decision{Action: ai.ActionText, Value: body}
	if _, err := p.Apply(context.Background(), dup, composeScreen); err == nil {
		t.Error("duplicate post was allowed")
	}
}

func TestSessionLimits(t *testing.T) {
	p, _ := testPolicy(t)
	p.cfg.Behavior.MaxPostsPerSession = 1
	p.cfg.Behavior.MaxDownloadsPerSession = 1
	post := ai.Decision{Action: ai.ActionText, Value: "A perfectly reasonable first post, long enough to pass."}
	if _, err := p.Apply(context.Background(), post, composeScreen); err != nil {
		t.Fatalf("first post blocked: %v", err)
	}
	p.OnAction(post, composeScreen, nil)
	if _, err := p.Apply(context.Background(), post, composeScreen); err == nil {
		t.Error("second post should hit the session limit")
	}
}
