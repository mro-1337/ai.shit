// Package agent runs the observe/decide/act loop and enforces etiquette.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

// BlockedError explains why an action was not allowed.
type BlockedError struct{ Reason string }

func (e *BlockedError) Error() string { return e.Reason }

var postingScreenRe = regexp.MustCompile(`(?i)(subject\s*:|enter your message|message body|type your message|save this message|post a message)`)

// IsPostingScreen reports whether the screen looks like message composition.
func IsPostingScreen(screen string) bool { return postingScreenRe.MatchString(screen) }

// Policy enforces rate limits, dry-run/read-only modes and basic decency.
type Policy struct {
	cfg *config.Config
	mem *memory.Store
	log *slog.Logger

	lastAction       time.Time
	sessionPosts     int
	sessionDownloads int
}

// NewPolicy builds a policy for one session.
func NewPolicy(cfg *config.Config, mem *memory.Store, log *slog.Logger) *Policy {
	return &Policy{cfg: cfg, mem: mem, log: log}
}

// Apply validates and possibly rewrites a decision before it is executed.
func (p *Policy) Apply(ctx context.Context, d ai.Decision, screen string) (ai.Decision, error) {
	if p.cfg.Behavior.MinActionDelayMS > 0 {
		wait := time.Duration(p.cfg.Behavior.MinActionDelayMS)*time.Millisecond - time.Since(p.lastAction)
		if wait > 0 {
			t := time.NewTimer(wait)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return d, ctx.Err()
			case <-t.C:
			}
		}
	}

	posting := IsPostingScreen(screen)
	composing := len([]rune(d.Value)) > 3 && d.Value != "Y" && d.Value != "N" && d.Value != "."
	lowerScreen := strings.ToLower(screen)
	bodyText := composing && (strings.Contains(lowerScreen, "enter your message") ||
		strings.Contains(lowerScreen, "message body") ||
		strings.Contains(lowerScreen, "type your message"))

	if d.Action == ai.ActionDownload {
		if p.cfg.Behavior.ReadOnly {
			return d, &BlockedError{Reason: "downloads are disabled in read-only mode"}
		}
		if p.sessionDownloads >= p.cfg.Behavior.MaxDownloadsPerSession {
			return d, &BlockedError{Reason: "session download limit reached"}
		}
		if !p.mem.CanDownload(p.cfg.Behavior.MaxDownloadsPerSession, int64(p.cfg.Behavior.MaxDownloadMB)<<20) {
			return d, &BlockedError{Reason: "daily download budget exhausted"}
		}
	}

	if d.Action == ai.ActionText {
		if err := p.checkSecrets(d.Value); err != nil {
			return d, err
		}
		for _, word := range p.cfg.Behavior.BlockedWords {
			if word != "" && strings.Contains(strings.ToLower(d.Value), strings.ToLower(word)) {
				return d, &BlockedError{Reason: fmt.Sprintf("message contains blocked word %q", word)}
			}
		}
	}

	if posting && composing {
		if p.cfg.Behavior.ReadOnly {
			return d, &BlockedError{Reason: "posting is disabled in read-only mode"}
		}
		if bodyText {
			if p.sessionPosts >= p.cfg.Behavior.MaxPostsPerSession {
				return d, &BlockedError{Reason: "session post limit reached"}
			}
			if !p.mem.CanPost(p.cfg.Behavior.MaxPostsPerSession) {
				return d, &BlockedError{Reason: "daily post limit reached"}
			}
			n := len([]rune(d.Value))
			if n < p.cfg.Behavior.MinPostChars {
				return d, &BlockedError{Reason: fmt.Sprintf("post is too short (%d < %d characters)", n, p.cfg.Behavior.MinPostChars)}
			}
			if n > p.cfg.Behavior.MaxPostChars {
				return d, &BlockedError{Reason: fmt.Sprintf("post is too long (%d > %d characters)", n, p.cfg.Behavior.MaxPostChars)}
			}
			if p.tooSimilar(d.Value) {
				return d, &BlockedError{Reason: "post is too similar to something already posted"}
			}
		}
		if p.cfg.Behavior.DryRun {
			p.log.Info("DRY RUN: not sending post", "text", d.Value)
			return ai.Decision{
				Thought: "Dry run: would have sent: " + d.Value,
				Action:  ai.ActionNote,
				Note:    "dry-run post: " + truncateForNote(d.Value),
			}, nil
		}
	}

	p.lastAction = time.Now()
	return d, nil
}

// OnAction updates counters after an action ran.
func (p *Policy) OnAction(d ai.Decision, screen string, err error) {
	p.lastAction = time.Now()
	if err != nil {
		return
	}
	switch d.Action {
	case ai.ActionText:
		lower := strings.ToLower(screen)
		if IsPostingScreen(screen) && len([]rune(d.Value)) > 3 && d.Value != "Y" && d.Value != "N" && d.Value != "." {
			if strings.Contains(lower, "enter your message") ||
				strings.Contains(lower, "message body") ||
				strings.Contains(lower, "type your message") {
				p.sessionPosts++
			}
		}
	case ai.ActionDownload:
		p.sessionDownloads++
	}
}

// SessionPosts exposes the counter for reports.
func (p *Policy) SessionPosts() int { return p.sessionPosts }

// SessionDownloads exposes the counter for reports.
func (p *Policy) SessionDownloads() int { return p.sessionDownloads }

func (p *Policy) checkSecrets(value string) error {
	secrets := []string{p.cfg.Account.Password, p.cfg.AI.APIKey}
	if p.mem != nil {
		secrets = append(secrets, p.mem.Password)
	}
	for _, s := range secrets {
		if len(s) >= 6 && strings.Contains(value, s) {
			return &BlockedError{Reason: "refusing to send a credential to the BBS"}
		}
	}
	return nil
}

func (p *Policy) tooSimilar(text string) bool {
	for _, post := range p.mem.RecentPosts(10) {
		if jaccard(text, post.Body) > 0.85 {
			return true
		}
	}
	return false
}

func jaccard(a, b string) float64 {
	ta, tb := tokens(a), tokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for w := range ta {
		if tb[w] {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,!?;:\"'()[]")
		if len(w) > 2 {
			out[w] = true
		}
	}
	return out
}

func truncateForNote(s string) string {
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

// LimitSummary is used by prompts and reports.
func LimitSummary(cfg *config.Config) string {
	return fmt.Sprintf("%d posts/session, %d downloads/session, %d MB/day, max %d steps",
		cfg.Behavior.MaxPostsPerSession, cfg.Behavior.MaxDownloadsPerSession,
		cfg.Behavior.MaxDownloadMB, cfg.Behavior.MaxSteps)
}

// SessionLimits returns the remaining action budget for prompts.
func (p *Policy) SessionLimits() (postsLeft, downloadsLeft int) {
	postsLeft = p.cfg.Behavior.MaxPostsPerSession - p.sessionPosts
	downloadsLeft = p.cfg.Behavior.MaxDownloadsPerSession - p.sessionDownloads
	if postsLeft < 0 {
		postsLeft = 0
	}
	if downloadsLeft < 0 {
		downloadsLeft = 0
	}
	return
}
