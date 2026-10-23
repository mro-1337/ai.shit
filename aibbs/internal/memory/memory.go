// Package memory keeps what the bot learned about a BBS between sessions:
// its account, which messages it already read, what it posted, what it
// downloaded, and free-form notes for future visits.
package memory

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AreaState remembers the last visit to a message area.
type AreaState struct {
	Name        string    `json:"name"`
	LastVisit   time.Time `json:"last_visit"`
	LastSubject string    `json:"last_subject,omitempty"`
}

// Post records something the bot posted.
type Post struct {
	At      time.Time `json:"at"`
	Area    string    `json:"area"`
	Subject string    `json:"subject"`
	Body    string    `json:"body"`
}

// Download records a received file.
type Download struct {
	At   time.Time `json:"at"`
	Name string    `json:"name"`
	Path string    `json:"path"`
	Size int64     `json:"size"`
}

// Counters track per-day etiquette limits.
type Counters struct {
	Day            string `json:"day"`
	PostsToday     int    `json:"posts_today"`
	DownloadsToday int    `json:"downloads_today"`
	BytesToday     int64  `json:"bytes_today"`
}

// Store is the persisted state for one BBS.
type Store struct {
	BBS            string                `json:"bbs"`
	Handle         string                `json:"handle,omitempty"`
	Password       string                `json:"password,omitempty"`
	Joined         bool                  `json:"joined"`
	LastSeen       time.Time             `json:"last_seen"`
	Areas          map[string]*AreaState `json:"areas"`
	SeenMessages   map[string]bool       `json:"seen_messages"`
	RepliedThreads map[string]bool       `json:"replied_threads"`
	Posts          []Post                `json:"posts"`
	Downloads      []Download            `json:"downloads"`
	DoorScores     map[string]string     `json:"door_scores"`
	Notes          []string              `json:"notes"`
	Counters       Counters              `json:"counters"`

	path string
	mu   sync.Mutex
}

// Load reads the store at path (or returns an empty one if it is missing).
func Load(path, bbsName string) (*Store, error) {
	s := &Store{
		BBS:            bbsName,
		Areas:          map[string]*AreaState{},
		SeenMessages:   map[string]bool{},
		RepliedThreads: map[string]bool{},
		DoorScores:     map[string]string{},
		path:           path,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse memory %s: %w", path, err)
	}
	s.path = path
	if s.Areas == nil {
		s.Areas = map[string]*AreaState{}
	}
	if s.SeenMessages == nil {
		s.SeenMessages = map[string]bool{}
	}
	if s.RepliedThreads == nil {
		s.RepliedThreads = map[string]bool{}
	}
	if s.DoorScores == nil {
		s.DoorScores = map[string]string{}
	}
	s.touchDay()
	return s, nil
}

// Path returns the file backing the store.
func (s *Store) Path() string { return s.path }

// Save writes the store atomically with owner-only permissions.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	s.LastSeen = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		// Windows cannot always rename over an existing file.
		_ = os.Remove(s.path)
		if err2 := os.Rename(tmp, s.path); err2 != nil {
			return err2
		}
	}
	return nil
}

// SetAccount remembers the credentials we use.
func (s *Store) SetAccount(handle, password string, joined bool) {
	s.Handle, s.Password, s.Joined = handle, password, joined
}

// HashText produces a stable message fingerprint.
func HashText(text string) string {
	norm := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	sum := sha1.Sum([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// MessageSeen reports whether a message fingerprint was already read.
func (s *Store) MessageSeen(hash string) bool { return s.SeenMessages[hash] }

// MarkMessageSeen records a message fingerprint.
func (s *Store) MarkMessageSeen(hash string) {
	if s.SeenMessages == nil {
		s.SeenMessages = map[string]bool{}
	}
	s.SeenMessages[hash] = true
}

// VisitArea updates the last-visit info for an area.
func (s *Store) VisitArea(name string) {
	if s.Areas == nil {
		s.Areas = map[string]*AreaState{}
	}
	s.Areas[name] = &AreaState{Name: name, LastVisit: time.Now()}
}

// AddPost records a post and bumps the daily counter.
func (s *Store) AddPost(area, subject, body string) Post {
	p := Post{At: time.Now(), Area: area, Subject: subject, Body: body}
	s.Posts = append(s.Posts, p)
	if len(s.Posts) > 200 {
		s.Posts = s.Posts[len(s.Posts)-200:]
	}
	s.touchDay()
	s.Counters.PostsToday++
	return p
}

// RecentPosts returns up to n of the most recent posts.
func (s *Store) RecentPosts(n int) []Post {
	if n <= 0 || len(s.Posts) == 0 {
		return nil
	}
	if n > len(s.Posts) {
		n = len(s.Posts)
	}
	return s.Posts[len(s.Posts)-n:]
}

// AddDownload records a download and bumps the daily counters.
func (s *Store) AddDownload(name, path string, size int64) {
	s.Downloads = append(s.Downloads, Download{At: time.Now(), Name: name, Path: path, Size: size})
	if len(s.Downloads) > 200 {
		s.Downloads = s.Downloads[len(s.Downloads)-200:]
	}
	s.touchDay()
	s.Counters.DownloadsToday++
	s.Counters.BytesToday += size
}

// AddNote stores a free-form observation for future sessions.
func (s *Store) AddNote(note string) {
	note = strings.Join(strings.Fields(note), " ")
	if note == "" {
		return
	}
	s.Notes = append(s.Notes, note)
	if len(s.Notes) > 50 {
		s.Notes = s.Notes[len(s.Notes)-50:]
	}
}

// SetDoorScore remembers a score for a door game.
func (s *Store) SetDoorScore(game, score string) {
	if s.DoorScores == nil {
		s.DoorScores = map[string]string{}
	}
	s.DoorScores[game] = score
}

// Summary renders the memory for the model prompt.
func (s *Store) Summary(maxNotes int) string {
	var b strings.Builder
	if s.Handle != "" {
		fmt.Fprintf(&b, "Your account here is %q (joined=%v).\n", s.Handle, s.Joined)
	}
	if len(s.Posts) > 0 {
		fmt.Fprintf(&b, "You have posted %d times; most recent: %q.\n", len(s.Posts), s.Posts[len(s.Posts)-1].Subject)
	}
	if len(s.Downloads) > 0 {
		fmt.Fprintf(&b, "You have downloaded %d files here.\n", len(s.Downloads))
	}
	if len(s.DoorScores) > 0 {
		b.WriteString("Door game results: ")
		first := true
		for game, score := range s.DoorScores {
			if !first {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "%s: %s", game, score)
			first = false
		}
		b.WriteString("\n")
	}
	if len(s.Notes) > 0 {
		if maxNotes <= 0 || maxNotes > len(s.Notes) {
			maxNotes = len(s.Notes)
		}
		b.WriteString("Notes from previous visits:\n")
		for _, n := range s.Notes[len(s.Notes)-maxNotes:] {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	if b.Len() == 0 {
		return "This is your first visit."
	}
	return strings.TrimRight(b.String(), "\n")
}

// CanPost enforces the per-day posting limit.
func (s *Store) CanPost(limit int) bool {
	s.touchDay()
	return s.Counters.PostsToday < limit
}

// CanDownload enforces the per-day count and byte budget.
func (s *Store) CanDownload(limit int, maxBytes int64) bool {
	s.touchDay()
	if s.Counters.DownloadsToday >= limit {
		return false
	}
	return maxBytes <= 0 || s.Counters.BytesToday < maxBytes
}

// PostsRemaining is a convenience for prompts and reports.
func (s *Store) PostsRemaining(limit int) int {
	s.touchDay()
	n := limit - s.Counters.PostsToday
	if n < 0 {
		return 0
	}
	return n
}

// DownloadsRemaining is a convenience for prompts and reports.
func (s *Store) DownloadsRemaining(limit int) int {
	s.touchDay()
	n := limit - s.Counters.DownloadsToday
	if n < 0 {
		return 0
	}
	return n
}

func (s *Store) touchDay() {
	today := time.Now().Format("2006-01-02")
	if s.Counters.Day != today {
		s.Counters = Counters{Day: today}
	}
}
