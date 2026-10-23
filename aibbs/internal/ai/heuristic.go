package ai

import (
	"context"
	"math/rand"
	"regexp"
	"strings"
)

// HeuristicBrain is a deliberate, non-LLM fallback. It understands the
// common shapes of BBS menus (single-key main menus, numbered area lists,
// subject/body composition) and is what "aibbs demo" uses by default.
type HeuristicBrain struct {
	lastAction      string
	bodySent        bool
	dotSent         bool
	saveSent        bool
	downloadStarted bool
	guesses         int
	subject         string
}

var mainMenuRe = regexp.MustCompile(`(?im)^\s*main\s+menu\b`)

// NewHeuristicBrain returns the offline brain.
func NewHeuristicBrain() *HeuristicBrain { return &HeuristicBrain{} }

// Name implements Brain.
func (h *HeuristicBrain) Name() string { return "heuristic" }

// Decide implements Brain.
func (h *HeuristicBrain) Decide(_ context.Context, obs Observation) (Decision, error) {
	lower := strings.ToLower(obs.Screen)
	goal := strings.ToLower(obs.Goal)

	wantsMessages := hasAny(goal, "message", "mail", "read", "reply", "post", "thread")
	wantsFiles := hasAny(goal, "file", "download")
	wantsDoors := hasAny(goal, "door", "game", "play")

	if h.saveSent && !strings.Contains(lower, "save this message") {
		h.resetCompose()
		return h.done("The post was submitted; this goal is complete.")
	}
	if h.downloadStarted {
		h.downloadStarted = false
		return h.done("The file transfer finished; this goal is complete.")
	}
	if h.guesses > 0 && strings.Contains(lower, "door games") {
		h.guesses = 0
		return h.done("The door game session ended; this goal is complete.")
	}

	pick := func(d Decision) (Decision, error) {
		d.Normalize()
		h.lastAction = d.Action
		return d, d.Validate()
	}

	switch {
	case strings.Contains(lower, "protocol (z=") || strings.Contains(lower, "protocol (zmodem"):
		h.downloadStarted = true
		return pick(Decision{Thought: "A protocol prompt is showing; start a ZMODEM transfer.", Action: ActionDownload, Protocol: "zmodem"})

	case strings.Contains(lower, "download which file"):
		if !wantsFiles {
			return pick(Decision{Thought: "Not here for files; back out of the file area.", Action: ActionText, Value: "Q"})
		}
		return pick(Decision{Thought: "Pick the first file in the list.", Action: ActionText, Value: "1"})

	case strings.Contains(lower, "save this message"):
		h.saveSent = true
		return pick(Decision{Thought: "Confirm the post.", Action: ActionText, Value: "Y"})

	case strings.Contains(lower, "subject:"):
		h.subject = "A note from " + obs.Handle
		return pick(Decision{Thought: "Give the post a clear subject.", Action: ActionText, Value: h.subject})

	case strings.Contains(lower, "enter your message") || strings.Contains(lower, "message body"):
		if !h.bodySent {
			h.bodySent = true
			return pick(Decision{Thought: "Write a short, on-topic reply.", Action: ActionText,
				Value: "This is a great thread - the single-phone-line era shaped a whole culture. Thanks for posting it."})
		}
		if !h.dotSent {
			h.dotSent = true
			return pick(Decision{Thought: "Finish the message with the terminator line.", Action: ActionText, Value: "."})
		}
		return pick(Decision{Thought: "Waiting for the editor to finish.", Action: ActionWait, Value: "1"})

	case strings.Contains(lower, "[r]eply"):
		if wantsMessages {
			return pick(Decision{Thought: "Reply to this message.", Action: ActionText, Value: "R"})
		}
		return pick(Decision{Thought: "Move on from this message.", Action: ActionText, Value: "Q"})

	case strings.Contains(lower, "message posted"):
		h.bodySent, h.dotSent = false, false
		return pick(Decision{Thought: "The post is saved; this goal is complete.", Action: ActionDone})

	case strings.Contains(lower, "transfer complete"):
		return pick(Decision{Thought: "The file arrived; this goal is complete.", Action: ActionDone})

	case strings.Contains(lower, "final score"), strings.Contains(lower, "out of guesses"),
		strings.Contains(lower, "you got it in"):
		return pick(Decision{Thought: "The door game session is over; this goal is complete.", Action: ActionDone})

	case strings.Contains(lower, "hit any key") || strings.Contains(lower, "press any key"):
		return pick(Decision{Thought: "Dismiss the pause prompt.", Action: ActionKeys, Value: "<SPACE>"})

	case mainMenuRe.MatchString(obs.Screen):
		switch {
		case wantsMessages:
			return pick(Decision{Thought: "Open the message areas.", Action: ActionKeys, Value: "M"})
		case wantsFiles:
			return pick(Decision{Thought: "Open the file areas.", Action: ActionKeys, Value: "F"})
		case wantsDoors:
			return pick(Decision{Thought: "Open the door games.", Action: ActionKeys, Value: "G"})
		default:
			return pick(Decision{Thought: "Nothing left to do here.", Action: ActionDisconnect})
		}

	case strings.Contains(lower, "message areas") && strings.Contains(lower, "area:"):
		if !wantsMessages {
			return pick(Decision{Thought: "Leave the message areas.", Action: ActionText, Value: "Q"})
		}
		return pick(Decision{Thought: "Enter the general area.", Action: ActionText, Value: "1"})

	case strings.Contains(lower, "read #"):
		if wantsMessages {
			return pick(Decision{Thought: "Read the first message in the area.", Action: ActionText, Value: "1"})
		}
		return pick(Decision{Thought: "Leave this area.", Action: ActionText, Value: "Q"})

	case strings.Contains(lower, "door games") && strings.Contains(lower, "choice:"):
		if !wantsDoors {
			return pick(Decision{Thought: "Leave the door menu.", Action: ActionText, Value: "Q"})
		}
		return pick(Decision{Thought: "Play Guess the Number.", Action: ActionText, Value: "1"})

	case strings.Contains(lower, "your guess:"):
		h.guesses++
		n := rand.Intn(100) + 1
		return pick(Decision{Thought: "Take a guess.", Action: ActionText, Value: itoa(n)})

	case strings.Contains(lower, "high, low"):
		return pick(Decision{Thought: "Bet high this round.", Action: ActionKeys, Value: "H"})

	case strings.Contains(lower, "file areas") && !strings.Contains(lower, "protocol"):
		if wantsFiles {
			return pick(Decision{Thought: "Choose the first file.", Action: ActionText, Value: "1"})
		}
		return pick(Decision{Thought: "No files this time; go back.", Action: ActionText, Value: "Q"})

	case strings.Contains(lower, "choose a password"), strings.Contains(lower, "password:"):
		return pick(Decision{Thought: "The session should already be authenticated; wait for the menu.", Action: ActionWait, Value: "1"})
	}

	return pick(Decision{Thought: "Nothing familiar is on screen; wait and look again.", Action: ActionWait, Value: "1"})
}

func hasAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func (h *HeuristicBrain) resetCompose() {
	h.bodySent, h.dotSent, h.saveSent = false, false, false
}

func (h *HeuristicBrain) done(thought string) (Decision, error) {
	d := Decision{Thought: thought, Action: ActionDone}
	h.lastAction = d.Action
	return d, nil
}
