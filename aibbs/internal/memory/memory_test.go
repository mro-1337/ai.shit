package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s, err := Load(path, "test-bbs")
	if err != nil {
		t.Fatal(err)
	}
	if !s.CanPost(2) || !s.CanDownload(2, 10<<20) {
		t.Error("fresh store should allow actions")
	}
	s.SetAccount("Tester", "secretpass", true)
	s.AddPost("General", "Hello", "This is my first post here.")
	s.AddDownload("ART.ANS", "/tmp/ART.ANS", 164)
	s.AddNote("The sysop likes retro hardware.")
	s.SetDoorScore("LORD", "12 gold, level 1")
	if s.Counters.PostsToday != 1 || s.Counters.DownloadsToday != 1 {
		t.Errorf("counters not updated: %+v", s.Counters)
	}
	if s.CanPost(1) {
		t.Error("post limit not enforced")
	}
	if s.CanDownload(1, 0) {
		t.Error("download limit not enforced")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("memory permissions = %o, want 600", info.Mode().Perm())
	}

	again, err := Load(path, "test-bbs")
	if err != nil {
		t.Fatal(err)
	}
	if again.Handle != "Tester" || !again.Joined {
		t.Errorf("account not persisted: %+v", again)
	}
	if len(again.Posts) != 1 || again.Posts[0].Subject != "Hello" {
		t.Errorf("posts not persisted: %+v", again.Posts)
	}
	if len(again.Downloads) != 1 || again.Downloads[0].Name != "ART.ANS" {
		t.Errorf("downloads not persisted: %+v", again.Downloads)
	}
	if again.DoorScores["LORD"] == "" {
		t.Error("door score not persisted")
	}
	summary := again.Summary(5)
	if !strings.Contains(summary, "Tester") || !strings.Contains(summary, "retro hardware") {
		t.Errorf("summary missing data: %q", summary)
	}
}

func TestHashTextIsStable(t *testing.T) {
	a := HashText("Hello   World\r\nThis is a test")
	b := HashText("hello world this is a test")
	if a != b {
		t.Errorf("hashing is not normalised: %s vs %s", a, b)
	}
	if HashText("one") == HashText("two") {
		t.Error("different text hashed identically")
	}
}

func TestDailyReset(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "m.json"), "bbs")
	s.Counters = Counters{Day: "2000-01-01", PostsToday: 99, DownloadsToday: 99, BytesToday: 1 << 30}
	if !s.CanPost(3) {
		t.Error("a new day should reset the post counter")
	}
	if s.Counters.PostsToday != 0 {
		t.Errorf("counters not reset: %+v", s.Counters)
	}
}
