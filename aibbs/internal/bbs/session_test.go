package bbs_test

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/fakebbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

func testConfig(t *testing.T, addr, dir string) *config.Config {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	cfg := config.Default()
	cfg.BBS.Name = "fake-test"
	cfg.BBS.Host = host
	cfg.BBS.Port = port
	cfg.BBS.IdleMillis = 200
	cfg.BBS.MaxSettleSeconds = 4
	cfg.Account.Handle = "TestBot"
	cfg.Account.AllowRegister = true
	cfg.Behavior.StateDir = dir
	cfg.Downloads.Dir = filepath.Join(dir, "downloads")
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func newSession(t *testing.T, cfg *config.Config) (*bbs.Session, *memory.Store) {
	t.Helper()
	mem, err := memory.Load(filepath.Join(cfg.StateDir(), "memory.json"), cfg.BBS.Name)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	sess, err := bbs.Dial(context.Background(), cfg, mem, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess, mem
}

func TestRegisterAndLogin(t *testing.T) {
	srv, err := fakebbs.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	cfg := testConfig(t, srv.Addr(), t.TempDir())
	sess, mem := newSession(t, cfg)

	res, err := sess.Login(context.Background())
	if err != nil {
		t.Fatalf("login: %v\nscreen:\n%s", err, sess.Screen().Text())
	}
	if !res.Registered {
		t.Errorf("expected a new registration")
	}
	if mem.Handle != "TestBot" || mem.Password == "" {
		t.Errorf("memory not updated: handle=%q password set=%v", mem.Handle, mem.Password != "")
	}
	if got := srv.Stats(); got.Registrations != 1 {
		t.Errorf("registrations = %d, want 1", got.Registrations)
	}
}

func TestReadPostDownloadAndDoor(t *testing.T) {
	srv, err := fakebbs.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	cfg := testConfig(t, srv.Addr(), t.TempDir())
	sess, _ := newSession(t, cfg)
	if _, err := sess.Login(context.Background()); err != nil {
		t.Fatalf("login: %v\nscreen:\n%s", err, sess.Screen().Text())
	}

	sendLine := func(s string) {
		t.Helper()
		if err := sess.SendLine(s); err != nil {
			t.Fatal(err)
		}
		if _, err := sess.Observe(); err != nil {
			t.Fatal(err)
		}
	}
	sendKey := func(s string) {
		t.Helper()
		if err := sess.SendKeys(s); err != nil {
			t.Fatal(err)
		}
		if _, err := sess.Observe(); err != nil {
			t.Fatal(err)
		}
	}

	// Message areas -> General -> read the long "history" message.
	sendKey("M")
	sendLine("1")
	obs, _ := sess.Observe()
	if !contains(obs.Snapshot.Text, "My BBS history") {
		t.Fatalf("message list missing long post:\n%s", obs.Snapshot.Text)
	}
	sendLine("3")
	obs, _ = sess.Observe()
	if !contains(obs.Snapshot.Text, "Wildcat") {
		t.Fatalf("long message body not reached (pager issue?):\n%s", obs.Snapshot.Text)
	}
	sendLine("Q") // back to area list

	// Post a new message in General.
	sendLine("P")
	sendLine("Hello from the integration test")
	sendLine("This board is a nice place to practise being a BBS user.")
	sendLine(".")
	sendLine("Y")

	found := false
	for _, m := range srv.Messages("General") {
		if m.Subject == "Hello from the integration test" && m.From == "TestBot" {
			found = true
			if !contains(m.Body, "practise") {
				t.Errorf("stored body lost text: %q", m.Body)
			}
		}
	}
	if !found {
		t.Fatalf("posted message not stored: %+v", srv.Messages("General"))
	}

	sendLine("Q") // leave the area
	sendLine("Q") // leave the message menu

	// Download PAYLOAD.BIN with ZMODEM.
	sendKey("F")
	sendLine("2")
	obs, _ = sess.Observe()
	if !contains(obs.Snapshot.Text, "Protocol") {
		t.Fatalf("protocol prompt missing:\n%s", obs.Snapshot.Text)
	}
	files, err := sess.Download("zmodem")
	if err != nil {
		t.Fatalf("download: %v\nscreen:\n%s", err, sess.Screen().Text())
	}
	if len(files) != 1 || files[0].Bytes != 3500 {
		t.Fatalf("unexpected download result: %+v", files)
	}
	data, err := os.ReadFile(files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 3500 || data[0] != 0x18 || data[1] != 0x7f || data[2] != 0xff {
		t.Errorf("payload corrupted: len=%d head=% x", len(data), data[:8])
	}

	// The BBS asks for a key after the transfer; make sure we are back at a
	// menu before navigating on.
	for i := 0; i < 3; i++ {
		_ = sess.SendKeys("<SPACE>")
		if _, err := sess.Observe(); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		obs, _ := sess.Observe()
		if contains(obs.Snapshot.Text, "Main Menu") {
			break
		}
		if err := sess.SendLine("Q"); err != nil {
			t.Fatal(err)
		}
	}
	obs, _ = sess.Observe()
	if !contains(obs.Snapshot.Text, "Main Menu") {
		t.Fatalf("could not return to main menu: %s", obs.Snapshot.Text)
	}
	// Play a door game.
	sendKey("G")
	sendLine("1")
	for i := 0; i < 8; i++ {
		obs, err := sess.Observe()
		if err != nil {
			t.Fatal(err)
		}
		if contains(obs.Snapshot.Text, "Hit any key") || contains(obs.Snapshot.Text, "Out of guesses") ||
			contains(obs.Snapshot.Text, "Correct!") {
			break
		}
		sendLine("50")
	}
	_ = sess.Settle()

	stats := srv.Stats()
	if stats.DoorPlays == 0 {
		t.Errorf("door not played: %+v", stats)
	}
	if stats.Transfers == 0 {
		t.Errorf("transfer not counted: %+v", stats)
	}
	if stats.Reads == 0 {
		t.Errorf("message read not counted: %+v", stats)
	}
}

func TestExistingAccountLogin(t *testing.T) {
	srv, err := fakebbs.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	srv.SeedUser("OldTimer", "hunter2")

	cfg := testConfig(t, srv.Addr(), t.TempDir())
	cfg.Account.Handle = "OldTimer"
	cfg.Account.Password = "hunter2"
	cfg.Account.AllowRegister = false
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	sess, _ := newSession(t, cfg)
	if _, err := sess.Login(context.Background()); err != nil {
		t.Fatalf("login: %v\nscreen:\n%s", err, sess.Screen().Text())
	}
	if got := srv.Stats(); got.Registrations != 0 {
		t.Errorf("unexpected registration: %+v", got)
	}
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
