// Package ai turns screen observations into the next BBS action.
//
// The wire format is a single JSON object so it works with any model that
// can follow instructions, and the offline HeuristicBrain keeps the bot
// usable (and testable) without an API key.
package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Action names the agent can execute.
const (
	ActionKeys       = "keys"       // raw keystrokes, supports <ENTER>, <UP>, ...
	ActionText       = "text"       // a line of input followed by Enter
	ActionDownload   = "download"   // start a file transfer
	ActionWait       = "wait"       // pause, then look again
	ActionNote       = "note"       // remember something for later visits
	ActionDone       = "done"       // current goal is complete
	ActionDisconnect = "disconnect" // log off and end the session
)

// Decision is one turn of the agent loop.
type Decision struct {
	Thought  string `json:"thought"`
	Action   string `json:"action"`
	Value    string `json:"value,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Normalize lower-cases the action and applies protocol defaults.
func (d *Decision) Normalize() {
	d.Action = strings.ToLower(strings.TrimSpace(d.Action))
	d.Protocol = strings.ToLower(strings.TrimSpace(d.Protocol))
	d.Value = strings.TrimSpace(d.Value)
	if d.Action == ActionDownload && d.Protocol == "" {
		d.Protocol = "zmodem"
	}
}

// Validate reports whether the decision can be executed.
func (d *Decision) Validate() error {
	switch d.Action {
	case ActionKeys:
		if d.Value == "" {
			return fmt.Errorf("keys action needs a value")
		}
	case ActionText:
		if d.Value == "" {
			return fmt.Errorf("text action needs a value")
		}
	case ActionDownload:
		switch d.Protocol {
		case "", "zmodem", "ymodem", "xmodem":
		default:
			return fmt.Errorf("unsupported protocol %q", d.Protocol)
		}
	case ActionWait, ActionNote, ActionDone, ActionDisconnect:
	default:
		return fmt.Errorf("unknown action %q", d.Action)
	}
	return nil
}

// Describe renders a short human readable summary for transcripts.
func (d Decision) Describe() string {
	switch d.Action {
	case ActionKeys:
		return "keys " + d.Value
	case ActionText:
		return "text " + d.Value
	case ActionDownload:
		return "download " + d.Protocol
	case ActionNote:
		if d.Note != "" {
			return "note " + d.Note
		}
		return "note " + d.Value
	case ActionWait:
		return "wait"
	default:
		return d.Action
	}
}

// ParseDecision extracts a JSON decision from a model response, tolerating
// code fences and leading commentary.
func ParseDecision(content string) (Decision, error) {
	var d Decision
	text := strings.TrimSpace(content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	if start < 0 {
		return d, fmt.Errorf("no JSON object in model response: %q", truncate(content, 200))
	}
	// Find the matching closing brace, ignoring braces inside strings.
	depth, inString, escaped := 0, false, false
	end := -1
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end > 0 {
			break
		}
	}
	if end < 0 {
		return d, fmt.Errorf("unterminated JSON object in model response")
	}
	if err := json.Unmarshal([]byte(text[start:end]), &d); err != nil {
		return d, fmt.Errorf("invalid decision JSON: %w", err)
	}
	if d.Action == "" && d.Note != "" {
		d.Action = ActionNote
	}
	d.Normalize()
	if err := d.Validate(); err != nil {
		return d, err
	}
	return d, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
