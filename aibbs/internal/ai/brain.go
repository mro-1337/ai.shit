package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Observation is everything the brain gets to see for one turn.
type Observation struct {
	BBSName            string
	Handle             string
	Screen             string
	Recent             string
	Goals              []string
	Goal               string
	GoalIndex          int
	GoalCount          int
	Step               int
	MaxSteps           int
	DoorName           string
	DoorHint           string
	Memory             string
	LastActions        []string
	PostsRemaining     int
	DownloadsRemaining int
	ReadOnly           bool
	DryRun             bool
	AllowDoors         bool
	DownloadDir        string
}

// Brain decides the next action.
type Brain interface {
	Name() string
	Decide(ctx context.Context, obs Observation) (Decision, error)
}

// Rule is a regex-triggered action for scripted bots.
type Rule struct {
	Match    string
	Action   string
	Value    string
	Protocol string
}

// RuleBrain performs actions based on the first matching screen pattern.
// It is useful for smoke tests and for sysops who want a deterministic bot
// without involving a language model.
type RuleBrain struct {
	Rules   []Rule
	Default Decision
	rules   []*regexp.Regexp
}

// NewRuleBrain compiles the rules and returns a brain.
func NewRuleBrain(rules []Rule, def Decision) (*RuleBrain, error) {
	rb := &RuleBrain{Rules: rules, Default: def}
	for _, r := range rules {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", r.Match, err)
		}
		rb.rules = append(rb.rules, re)
	}
	if rb.Default.Action == "" {
		rb.Default = Decision{Action: ActionWait, Thought: "nothing matched"}
	}
	return rb, nil
}

// Name implements Brain.
func (r *RuleBrain) Name() string { return "rules" }

// Decide implements Brain.
func (r *RuleBrain) Decide(_ context.Context, obs Observation) (Decision, error) {
	for i, re := range r.rules {
		if re.MatchString(obs.Screen) {
			rule := r.Rules[i]
			d := Decision{Thought: "rule " + rule.Match, Action: rule.Action, Value: rule.Value, Protocol: rule.Protocol}
			d.Normalize()
			return d, d.Validate()
		}
	}
	d := r.Default
	d.Normalize()
	return d, d.Validate()
}

// AllowedActions lists the action vocabulary for prompts.
func AllowedActions(obs Observation) []string {
	actions := []string{
		`{"action":"keys","value":"<ENTER>","thought":"..."} - send keystrokes (supports <UP>, <DOWN>, <SPACE>, <ESC>, <TAB>, <F1>...)`,
		`{"action":"text","value":"some line","thought":"..."} - type a line and press Enter`,
		`{"action":"wait","value":"2","thought":"..."} - pause for N seconds and look again`,
		`{"action":"note","note":"what to remember","thought":"..."} - store a note for future visits`,
		`{"action":"done","thought":"..."} - the current goal is finished`,
		`{"action":"disconnect","thought":"..."} - log off and end the session`,
	}
	if !obs.ReadOnly {
		actions = append(actions, `{"action":"download","protocol":"zmodem","thought":"..."} - start a transfer at a protocol prompt`)
	}
	return actions
}

// TrimScreen keeps the most useful tail of the screen for the model.
func TrimScreen(text string, maxRows int) string {
	if maxRows <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= maxRows {
		return text
	}
	return "[... earlier output trimmed ...]\n" + strings.Join(lines[len(lines)-maxRows:], "\n")
}
