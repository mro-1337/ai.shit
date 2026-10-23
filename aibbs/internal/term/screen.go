// Package term implements a small VT100/ANSI terminal emulator.
//
// BBSes paint their interface with ANSI escape sequences (cursor addressing,
// colours, line clears). A language model cannot read those raw bytes, so the
// agent keeps an in-memory screen that mirrors what a human would see.
//
// Only the subset used by BBS software is implemented, but that subset is
// implemented properly: cursor movement, erase, insert/delete, scrolling
// regions, alternate screen, save/restore cursor.
package term

import (
	"strings"
	"unicode/utf8"
)

// Snapshot is an immutable view of the screen at one point in time.
type Snapshot struct {
	Text          string `json:"text"`
	CursorX       int    `json:"cursor_x"`
	CursorY       int    `json:"cursor_y"`
	Cols          int    `json:"cols"`
	Rows          int    `json:"rows"`
	CursorVisible bool   `json:"cursor_visible"`
}

// LastLine returns the last non-blank line of the snapshot.
func (s Snapshot) LastLine() string {
	lines := strings.Split(s.Text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimRight(lines[i], " \t")
		}
	}
	return ""
}

// Screen is a virtual ANSI terminal.
type Screen struct {
	Cols      int
	Rows      int
	scrollMax int

	grid           [][]rune
	x, y           int
	top, bottom    int
	savedX, savedY int
	cursorVisible  bool
	autoWrap       bool

	// parser state
	state       int
	paramBuf    []byte
	private     byte
	inter       byte
	depth       int
	utf8Pending []byte

	// alternate screen
	altGrid    [][]rune
	altX, altY int
	altActive  bool

	scrollback []string
	onScroll   func(string)
}

const (
	stGround = iota
	stEsc
	stCSI
	stOSC
	stOSCEsc
	stString
	stStringEsc
	stCharset
)

// New creates a screen of cols x rows with a bounded scrollback buffer.
func New(cols, rows, scrollback int) *Screen {
	if cols < 20 {
		cols = 20
	}
	if rows < 5 {
		rows = 5
	}
	if scrollback < 0 {
		scrollback = 0
	}
	s := &Screen{Cols: cols, Rows: rows, scrollMax: scrollback}
	s.Reset()
	return s
}

// Reset clears the screen and parser state.
func (s *Screen) Reset() {
	s.grid = blankGrid(s.Cols, s.Rows)
	s.x, s.y = 0, 0
	s.top, s.bottom = 0, s.Rows-1
	s.savedX, s.savedY = 0, 0
	s.cursorVisible = true
	s.autoWrap = true
	s.state = stGround
	s.paramBuf = s.paramBuf[:0]
	s.private, s.inter = 0, 0
	s.utf8Pending = s.utf8Pending[:0]
	s.altGrid = nil
	s.altActive = false
}

func blankGrid(cols, rows int) [][]rune {
	g := make([][]rune, rows)
	for i := range g {
		g[i] = make([]rune, cols)
		for j := range g[i] {
			g[i][j] = ' '
		}
	}
	return g
}

// OnScroll registers a callback invoked with each line that scrolls off.
func (s *Screen) OnScroll(fn func(string)) { s.onScroll = fn }

// Resize changes the screen dimensions, preserving overlapping content.
func (s *Screen) Resize(cols, rows int) {
	if cols < 20 {
		cols = 20
	}
	if rows < 5 {
		rows = 5
	}
	if cols == s.Cols && rows == s.Rows {
		return
	}
	g := blankGrid(cols, rows)
	for y := 0; y < s.Rows && y < rows; y++ {
		for x := 0; x < s.Cols && x < cols; x++ {
			g[y][x] = s.grid[y][x]
		}
	}
	s.grid, s.Cols, s.Rows = g, cols, rows
	s.top, s.bottom = 0, rows-1
	if s.x >= cols {
		s.x = cols - 1
	}
	if s.y >= rows {
		s.y = rows - 1
	}
}

// Feed decodes bytes into the screen. It is safe to feed partial UTF-8
// sequences: incomplete runes are buffered until the rest arrives.
func (s *Screen) Feed(data []byte) {
	buf := append(s.utf8Pending, data...)
	s.utf8Pending = s.utf8Pending[:0]
	for len(buf) > 0 {
		r, size := utf8.DecodeRune(buf)
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(buf) {
				s.utf8Pending = append(s.utf8Pending, buf...)
				return
			}
			// Invalid byte: draw a replacement char but keep going.
			buf = buf[1:]
			s.consume('\uFFFD')
			continue
		}
		buf = buf[size:]
		s.consume(r)
	}
}

// Text returns the visible screen as plain text with trailing blanks removed.
func (s *Screen) Text() string {
	lines := make([]string, 0, s.Rows)
	for _, row := range s.grid {
		var b strings.Builder
		for _, r := range row {
			if r != 0 {
				b.WriteRune(r)
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

// Recent returns the last n lines that scrolled off the screen.
func (s *Screen) Recent(n int) string {
	if n <= 0 || len(s.scrollback) == 0 {
		return ""
	}
	if n > len(s.scrollback) {
		n = len(s.scrollback)
	}
	return strings.Join(s.scrollback[len(s.scrollback)-n:], "\n")
}

// Snapshot returns the current visible state.
func (s *Screen) Snapshot() Snapshot {
	return Snapshot{
		Text:          s.Text(),
		CursorX:       s.x,
		CursorY:       s.y,
		Cols:          s.Cols,
		Rows:          s.Rows,
		CursorVisible: s.cursorVisible,
	}
}

// ---------------------------------------------------------------- cell ops

func (s *Screen) blank(y, x0, x1 int) {
	if y < 0 || y >= s.Rows {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= s.Cols {
		x1 = s.Cols - 1
	}
	for x := x0; x <= x1; x++ {
		s.grid[y][x] = ' '
	}
}

func runeWidth(r rune) int {
	// BBS content is overwhelmingly single width; treat CJK wide runes as 2
	// so art using them still lines up.
	if r >= 0x1100 && (r <= 0x115F || r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0xA4CF) || (r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF00 && r <= 0xFF60) || (r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x1F300 && r <= 0x1F64F) || (r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x20000 && r <= 0x3FFFD)) {
		return 2
	}
	return 1
}

func (s *Screen) put(r rune) {
	w := runeWidth(r)
	if w == 2 && s.x >= s.Cols-1 {
		if s.autoWrap {
			s.x = 0
			s.lineFeed()
		}
	}
	if s.grid[s.y][s.x] == 0 && s.x > 0 {
		s.grid[s.y][s.x-1] = ' '
	}
	if s.x+1 < s.Cols && s.grid[s.y][s.x+1] == 0 {
		s.grid[s.y][s.x+1] = ' '
	}
	s.grid[s.y][s.x] = r
	if w == 2 && s.x+1 < s.Cols {
		s.grid[s.y][s.x+1] = 0
	}
	s.x += w
	if s.x >= s.Cols {
		if s.autoWrap {
			s.x = 0
			s.lineFeed()
		} else {
			s.x = s.Cols - 1
		}
	}
}

func (s *Screen) lineFeed() {
	if s.y == s.bottom {
		s.scrollUp(1)
	} else if s.y < s.Rows-1 {
		s.y++
	}
}

func (s *Screen) reverseLineFeed() {
	if s.y == s.top {
		s.scrollDown(1)
	} else if s.y > 0 {
		s.y--
	}
}

func (s *Screen) scrollUp(n int) {
	for i := 0; i < n; i++ {
		removed := s.grid[s.top]
		var b strings.Builder
		for _, r := range removed {
			if r != 0 {
				b.WriteRune(r)
			}
		}
		line := strings.TrimRight(b.String(), " ")
		if line != "" && s.top == 0 && s.scrollMax > 0 {
			s.scrollback = append(s.scrollback, line)
			if len(s.scrollback) > s.scrollMax {
				s.scrollback = s.scrollback[len(s.scrollback)-s.scrollMax:]
			}
			if s.onScroll != nil {
				s.onScroll(line)
			}
		}
		copy(s.grid[s.top:], s.grid[s.top+1:])
		s.grid[s.bottom] = make([]rune, s.Cols)
		for j := range s.grid[s.bottom] {
			s.grid[s.bottom][j] = ' '
		}
	}
}

func (s *Screen) scrollDown(n int) {
	for i := 0; i < n; i++ {
		copy(s.grid[s.top+1:], s.grid[s.top:])
		s.grid[s.top] = make([]rune, s.Cols)
		for j := range s.grid[s.top] {
			s.grid[s.top][j] = ' '
		}
	}
}

func (s *Screen) eraseDisplay(mode int) {
	switch mode {
	case 0:
		s.blank(s.y, s.x, s.Cols-1)
		for y := s.y + 1; y < s.Rows; y++ {
			s.blank(y, 0, s.Cols-1)
		}
	case 1:
		for y := 0; y < s.y; y++ {
			s.blank(y, 0, s.Cols-1)
		}
		s.blank(s.y, 0, s.x)
	case 3:
		s.scrollback = nil
		fallthrough
	case 2:
		for y := 0; y < s.Rows; y++ {
			s.blank(y, 0, s.Cols-1)
		}
	}
}

func (s *Screen) eraseLine(mode int) {
	switch mode {
	case 0:
		s.blank(s.y, s.x, s.Cols-1)
	case 1:
		s.blank(s.y, 0, s.x)
	case 2:
		s.blank(s.y, 0, s.Cols-1)
	}
}

// ------------------------------------------------------------------ parser

func (s *Screen) consume(r rune) {
	switch s.state {
	case stGround:
		s.ground(r)
	case stEsc:
		s.escape(r)
	case stCSI:
		s.csi(r)
	case stOSC:
		if r == 0x07 {
			s.state = stGround
		} else if r == 0x1b {
			s.state = stOSCEsc
		}
	case stOSCEsc:
		s.state = stGround
		if r != '\\' {
			s.state = stOSC
		}
	case stString:
		if r == 0x1b {
			s.state = stStringEsc
		}
	case stStringEsc:
		s.state = stString
		if r == '\\' {
			s.state = stGround
		}
	}
}

func (s *Screen) ground(r rune) {
	switch {
	case r == 0x1b:
		s.state = stEsc
		s.paramBuf = s.paramBuf[:0]
		s.private, s.inter = 0, 0
	case r == '\n' || r == 0x0b || r == 0x0c:
		s.lineFeed()
		if r != 0x0b {
			s.x = 0
		}
	case r == '\r':
		s.x = 0
	case r == '\b':
		if s.x > 0 {
			s.x--
		}
	case r == '\t':
		s.x = (s.x/8 + 1) * 8
		if s.x >= s.Cols {
			s.x = s.Cols - 1
		}
	case r < 0x20 || r == 0x7f:
		// BEL and other control characters are ignored.
	default:
		s.put(r)
	}
}

func (s *Screen) escape(r rune) {
	s.state = stGround
	switch r {
	case '[':
		s.state = stCSI
		s.paramBuf = s.paramBuf[:0]
		s.private, s.inter = 0, 0
	case ']':
		s.state = stOSC
	case 'P', 'X', '^', '_':
		s.state = stString
	case '7':
		s.savedX, s.savedY = s.x, s.y
	case '8':
		s.x, s.y = s.savedX, s.savedY
		s.clampCursor()
	case 'D':
		s.lineFeed()
	case 'M':
		s.reverseLineFeed()
	case 'E':
		s.x = 0
		s.lineFeed()
	case 'c':
		s.Reset()
	case '(', ')', '*', '+', '#', '%':
		s.state = stCharset
	}
}

func (s *Screen) clampCursor() {
	if s.x < 0 {
		s.x = 0
	}
	if s.x >= s.Cols {
		s.x = s.Cols - 1
	}
	if s.y < 0 {
		s.y = 0
	}
	if s.y >= s.Rows {
		s.y = s.Rows - 1
	}
}

func (s *Screen) csi(r rune) {
	switch {
	case r >= 0x20 && r <= 0x2f:
		s.inter = byte(r)
	case r == '?' || r == '<' || r == '=' || r == '>':
		s.private = byte(r)
	case (r >= '0' && r <= '9') || r == ';' || r == ':':
		s.paramBuf = append(s.paramBuf, byte(r))
	case r >= 0x40 && r <= 0x7e:
		s.state = stGround
		s.dispatchCSI(r)
	default:
		s.state = stGround
	}
}

func (s *Screen) params() []int {
	if len(s.paramBuf) == 0 {
		return nil
	}
	raw := strings.ReplaceAll(string(s.paramBuf), ":", ";")
	parts := strings.Split(raw, ";")
	out := make([]int, len(parts))
	for i, p := range parts {
		if p == "" {
			continue
		}
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				n = 0
				break
			}
			n = n*10 + int(c-'0')
			if n > 100000 {
				n = 100000
			}
		}
		out[i] = n
	}
	return out
}

func paramOr(p []int, idx, def int) int {
	if idx < len(p) && p[idx] != 0 {
		return p[idx]
	}
	return def
}

func (s *Screen) dispatchCSI(final rune) {
	p := s.params()
	n := paramOr(p, 0, 1)
	switch final {
	case 'A':
		s.y -= n
	case 'B', 'e':
		s.y += n
	case 'C', 'a':
		s.x += n
	case 'D':
		s.x -= n
	case 'E':
		s.x = 0
		s.y += n
	case 'F':
		s.x = 0
		s.y -= n
	case 'G', '`':
		s.x = n - 1
	case 'd':
		s.y = n - 1
	case 'H', 'f':
		s.y = paramOr(p, 0, 1) - 1
		s.x = paramOr(p, 1, 1) - 1
	case 'J':
		s.eraseDisplay(paramOr(p, 0, 0))
	case 'K':
		s.eraseLine(paramOr(p, 0, 0))
	case 'L':
		if s.top <= s.y && s.y <= s.bottom {
			for i := 0; i < n; i++ {
				copy(s.grid[s.y+1:s.bottom+1], s.grid[s.y:s.bottom])
				s.grid[s.y] = make([]rune, s.Cols)
				for j := range s.grid[s.y] {
					s.grid[s.y][j] = ' '
				}
			}
		}
	case 'M':
		if s.top <= s.y && s.y <= s.bottom {
			for i := 0; i < n; i++ {
				copy(s.grid[s.y:s.bottom], s.grid[s.y+1:s.bottom+1])
				s.grid[s.bottom] = make([]rune, s.Cols)
				for j := range s.grid[s.bottom] {
					s.grid[s.bottom][j] = ' '
				}
			}
		}
	case 'P':
		row := s.grid[s.y]
		if s.x < len(row) {
			end := s.x + n
			if end > len(row) {
				end = len(row)
			}
			copy(row[s.x:], row[end:])
			for j := len(row) - (end - s.x); j < len(row); j++ {
				row[j] = ' '
			}
		}
	case '@':
		row := s.grid[s.y]
		if s.x < len(row) {
			copy(row[s.x+n:], row[s.x:])
			for j := s.x; j < s.x+n && j < len(row); j++ {
				row[j] = ' '
			}
		}
	case 'X':
		s.blank(s.y, s.x, s.x+n-1)
	case 'S':
		s.scrollUp(n)
	case 'T':
		s.scrollDown(n)
	case 'h', 'l':
		s.setModes(p, s.private == '?', final == 'h')
	case 'r':
		top := paramOr(p, 0, 1) - 1
		bottom := paramOr(p, 1, s.Rows) - 1
		if top < 0 {
			top = 0
		}
		if bottom >= s.Rows {
			bottom = s.Rows - 1
		}
		if bottom > top {
			s.top, s.bottom = top, bottom
			s.x, s.y = 0, 0
		}
	case 's':
		s.savedX, s.savedY = s.x, s.y
	case 'u':
		s.x, s.y = s.savedX, s.savedY
		// 'm' (SGR), 'n' (DSR), 't', 'q' and friends are intentionally ignored:
		// colours do not matter to the model, only the text does.
	}
	s.clampCursor()
}

func (s *Screen) setModes(p []int, private, enable bool) {
	for _, mode := range p {
		if !private {
			continue
		}
		switch mode {
		case 25:
			s.cursorVisible = enable
		case 7:
			s.autoWrap = enable
		case 47, 1047, 1049:
			s.toggleAltScreen(enable)
		}
	}
}

func (s *Screen) toggleAltScreen(enable bool) {
	if enable {
		if !s.altActive {
			s.altGrid = s.grid
			s.altX, s.altY = s.x, s.y
			s.altActive = true
			s.grid = blankGrid(s.Cols, s.Rows)
			s.top, s.bottom = 0, s.Rows-1
			s.x, s.y = 0, 0
		}
		return
	}
	if s.altActive {
		s.grid = s.altGrid
		s.x, s.y = s.altX, s.altY
		s.altGrid = nil
		s.altActive = false
		s.top, s.bottom = 0, s.Rows-1
		s.clampCursor()
	}
}
