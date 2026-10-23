package term

import (
	"strings"
	"testing"
)

func TestCursorAddressingAndColors(t *testing.T) {
	s := New(40, 10, 100)
	s.Feed([]byte("\x1b[2J\x1b[1;36mHELLO\x1b[0m\x1b[3;5HWORLD"))
	got := s.Text()
	if !strings.Contains(got, "HELLO") || !strings.Contains(got, "WORLD") {
		t.Fatalf("missing text: %q", got)
	}
	lines := strings.Split(got, "\n")
	if strings.TrimSpace(lines[0]) != "HELLO" {
		t.Errorf("first line = %q, want HELLO", lines[0])
	}
	// "WORLD" starts at row 3, column 5 -> four leading spaces.
	var worldLine string
	for _, line := range lines {
		if strings.Contains(line, "WORLD") {
			worldLine = line
		}
	}
	if !strings.HasPrefix(worldLine, "    WORLD") {
		t.Errorf("cursor addressing wrong: %q", worldLine)
	}
	if s.x != 9 || s.y != 2 {
		t.Errorf("cursor = (%d,%d), want (9,2)", s.x, s.y)
	}
}

func TestEraseLineAndScrollback(t *testing.T) {
	s := New(20, 5, 100)
	for i := 0; i < 10; i++ {
		s.Feed([]byte("line " + string(rune('0'+i)) + "\r\n"))
	}
	if !strings.Contains(s.Recent(20), "line 0") {
		t.Errorf("scrollback missing early lines: %q", s.Recent(20))
	}
	if !strings.Contains(s.Text(), "line 9") {
		t.Errorf("latest line not visible: %q", s.Text())
	}
	s.Feed([]byte("XXX\r\x1b[2K"))
	if strings.Contains(s.Text(), "XXX") {
		t.Errorf("erase-line did not clear the current line: %q", s.Text())
	}
	if !strings.Contains(s.Text(), "line 9") {
		t.Errorf("erase-line cleared too much: %q", s.Text())
	}
}

func TestUTF8SplitAcrossFeeds(t *testing.T) {
	s := New(30, 5, 10)
	full := []byte("caf\u00e9 na\u00efve")
	s.Feed(full[:4]) // splits the é
	s.Feed(full[4:])
	if !strings.Contains(s.Text(), "café") {
		t.Errorf("UTF-8 not decoded across feeds: %q", s.Text())
	}
}

func TestAltScreen(t *testing.T) {
	s := New(20, 5, 10)
	s.Feed([]byte("main\r\n"))
	s.Feed([]byte("\x1b[?1049h"))
	s.Feed([]byte("alt"))
	if !strings.Contains(s.Text(), "alt") || strings.Contains(s.Text(), "main") {
		t.Errorf("alt screen not active: %q", s.Text())
	}
	s.Feed([]byte("\x1b[?1049l"))
	if !strings.Contains(s.Text(), "main") {
		t.Errorf("main screen not restored: %q", s.Text())
	}
}

func TestWrappingAndBackspace(t *testing.T) {
	s := New(20, 6, 10)
	s.Feed([]byte("abcdefghijklmnopqrstuvwxy"))
	lines := strings.Split(s.Text(), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "abcdefghijklmnopqrst") {
		t.Fatalf("wrap failed: %q", s.Text())
	}
	if !strings.HasPrefix(lines[1], "uvwxy") {
		t.Errorf("continuation line = %q", lines[1])
	}
	s2 := New(20, 4, 10)
	s2.Feed([]byte("abc\x08\x08X"))
	if strings.TrimSpace(s2.Text()) != "aXc" {
		t.Errorf("backspace handling: %q", s2.Text())
	}
}
