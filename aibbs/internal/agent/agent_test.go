package agent_test

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mro-1337/ai.shit/aibbs/internal/agent"
	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/fakebbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

func TestHeuristicAgentCompletesDemo(t *testing.T) {
	srv, err := fakebbs.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	host, portStr, _ := net.SplitHostPort(srv.Addr())
	port, _ := strconv.Atoi(portStr)

	dir := t.TempDir()
	cfg := config.Default()
	cfg.BBS.Name = "agent-test"
	cfg.BBS.Host, cfg.BBS.Port = host, port
	cfg.BBS.IdleMillis = 200
	cfg.BBS.MaxSettleSeconds = 4
	cfg.Account.Handle = "AgentBot"
	cfg.Behavior.StateDir = dir
	cfg.Behavior.MinActionDelayMS = 10
	cfg.Behavior.MaxSteps = 50
	cfg.Behavior.Goals = []string{
		"Read a message and reply to it.",
		"Download a small file.",
		"Play a door game for a few turns.",
	}
	cfg.Downloads.Dir = filepath.Join(dir, "downloads")
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	mem, err := memory.Load(filepath.Join(cfg.StateDir(), "memory.json"), cfg.BBS.Name)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := bbs.Dial(context.Background(), cfg, mem, log)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if _, err := sess.Login(context.Background()); err != nil {
		t.Fatalf("login: %v\n%s", err, sess.Snapshot().Text)
	}

	ag := agent.New(cfg, sess, ai.NewHeuristicBrain(), mem, log)
	report, err := ag.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v\nfinal screen:\n%s", err, report.FinalScreen)
	}
	if !report.Completed {
		t.Fatalf("agent did not complete: %+v\nfinal screen:\n%s", report, report.FinalScreen)
	}
	if report.GoalsCompleted != 3 {
		t.Errorf("goals = %d, want 3", report.GoalsCompleted)
	}
	if report.Posts == 0 || report.Downloads == 0 {
		t.Errorf("expected a post and a download: %+v", report)
	}
	if len(report.Files) == 0 {
		t.Errorf("no files recorded")
	}
	if _, err := os.Stat(report.Files[0].Path); err != nil {
		t.Errorf("downloaded file missing: %v", err)
	}

	stats := srv.Stats()
	if stats.Posts == 0 || stats.Transfers == 0 || stats.DoorPlays == 0 {
		t.Errorf("mock BBS did not see all activities: %+v", stats)
	}
	if len(srv.Messages("General")) < 4 {
		t.Errorf("reply was not stored")
	}
}
