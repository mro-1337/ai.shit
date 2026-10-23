package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
	"github.com/mro-1337/ai.shit/aibbs/internal/transfer"
)

// Agent runs one BBS shift: log in, work through the goals, log off.
type Agent struct {
	Cfg    *config.Config
	Sess   *bbs.Session
	Brain  ai.Brain
	Mem    *memory.Store
	Policy *Policy
	Log    *slog.Logger

	transcript     *os.File
	transcriptPath string
	pendingSubject string
	pendingBody    string
}

// Report summarises a run.
type Report struct {
	Started        time.Time               `json:"started"`
	Ended          time.Time               `json:"ended"`
	Brain          string                  `json:"brain"`
	Steps          int                     `json:"steps"`
	GoalCount      int                     `json:"goal_count"`
	GoalsCompleted int                     `json:"goals_completed"`
	Posts          int                     `json:"posts"`
	Downloads      int                     `json:"downloads"`
	Files          []transfer.ReceivedFile `json:"files,omitempty"`
	Notes          int                     `json:"notes"`
	Completed      bool                    `json:"completed"`
	TranscriptPath string                  `json:"transcript_path,omitempty"`
	FinalScreen    string                  `json:"-"`
}

// Summary renders the report for the terminal.
func (r Report) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Brain:        %s\n", r.Brain)
	fmt.Fprintf(&b, "Duration:     %s\n", r.Ended.Sub(r.Started).Round(time.Second))
	fmt.Fprintf(&b, "Steps:        %d\n", r.Steps)
	fmt.Fprintf(&b, "Goals:        %d/%d completed\n", r.GoalsCompleted, r.GoalCount)
	fmt.Fprintf(&b, "Posts:        %d\n", r.Posts)
	fmt.Fprintf(&b, "Downloads:    %d\n", r.Downloads)
	for _, f := range r.Files {
		fmt.Fprintf(&b, "  - %s (%d bytes) -> %s\n", f.Name, f.Bytes, f.Path)
	}
	if r.Notes > 0 {
		fmt.Fprintf(&b, "Notes saved:  %d\n", r.Notes)
	}
	if r.TranscriptPath != "" {
		fmt.Fprintf(&b, "Transcript:   %s\n", r.TranscriptPath)
	}
	if r.Completed {
		b.WriteString("Result:       completed cleanly\n")
	} else {
		b.WriteString("Result:       stopped before finishing\n")
	}
	return b.String()
}

// New builds an agent.
func New(cfg *config.Config, sess *bbs.Session, brain ai.Brain, mem *memory.Store, log *slog.Logger) *Agent {
	if log == nil {
		log = slog.Default()
	}
	return &Agent{
		Cfg:    cfg,
		Sess:   sess,
		Brain:  brain,
		Mem:    mem,
		Policy: NewPolicy(cfg, mem, log),
		Log:    log,
	}
}

type transcriptEntry struct {
	At       time.Time `json:"at"`
	Step     int       `json:"step"`
	Goal     string    `json:"goal"`
	Screen   string    `json:"screen"`
	Action   string    `json:"action"`
	Value    string    `json:"value,omitempty"`
	Protocol string    `json:"protocol,omitempty"`
	Thought  string    `json:"thought,omitempty"`
	Result   string    `json:"result,omitempty"`
}

func (a *Agent) openTranscript(report *Report) {
	if !a.Cfg.Behavior.Transcript {
		return
	}
	dir := a.Cfg.StateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		a.Log.Warn("cannot create state dir", "err", err)
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("transcript-%s.jsonl", time.Now().Format("20060102-150405")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		a.Log.Warn("cannot open transcript", "err", err)
		return
	}
	a.transcript, a.transcriptPath = f, path
	report.TranscriptPath = path
}

func (a *Agent) closeTranscript() {
	if a.transcript != nil {
		_ = a.transcript.Close()
	}
}

func (a *Agent) writeTranscript(entry transcriptEntry) {
	if a.transcript == nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = a.transcript.Write(append(data, '\n'))
}

// Run executes the configured shift. The caller is responsible for login.
func (a *Agent) Run(ctx context.Context) (Report, error) {
	report := Report{Started: time.Now(), Brain: a.Brain.Name(), GoalCount: len(a.Cfg.Behavior.Goals)}
	a.openTranscript(&report)
	defer a.closeTranscript()

	goals := a.Cfg.Behavior.Goals
	goalIndex := 0
	lastActions := make([]string, 0, 10)
	lastScreen := ""
	sameScreen := 0
	failures := 0
	var runErr error

	for step := 1; step <= a.Cfg.Behavior.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			runErr = err
			break
		}
		obs, err := a.Sess.Observe()
		if err != nil {
			runErr = err
			break
		}
		report.Steps = step
		a.noteConfirmedPost(obs.Snapshot.Text)

		if obs.Disconnected {
			report.Completed = goalIndex >= len(goals)
			break
		}

		goal := ""
		if goalIndex < len(goals) {
			goal = goals[goalIndex]
		}
		aiObs := a.buildObservation(obs, goals, goalIndex, step)
		decision, err := a.Brain.Decide(ctx, aiObs)
		if err != nil {
			failures++
			a.Log.Warn("brain error", "err", err, "failures", failures)
			a.writeTranscript(transcriptEntry{At: time.Now(), Step: step, Goal: goal,
				Screen: obs.Snapshot.Text, Action: "error", Result: err.Error()})
			if failures >= 3 {
				runErr = fmt.Errorf("brain failed three times: %w", err)
				break
			}
			sleepContext(ctx, time.Duration(failures)*2*time.Second)
			continue
		}
		failures = 0
		a.Log.Info("decision", "step", step, "action", decision.Action, "value", decision.Value, "thought", decision.Thought)

		switch decision.Action {
		case ai.ActionDone:
			a.Log.Info("goal complete", "goal", goal)
			goalIndex++
			report.GoalsCompleted = goalIndex
			a.writeTranscript(transcriptEntry{At: time.Now(), Step: step, Goal: goal,
				Screen: obs.Snapshot.Text, Action: "done", Thought: decision.Thought})
			if goalIndex >= len(goals) {
				a.Log.Info("all goals complete; logging off")
				_ = a.Sess.Logoff()
				report.Completed = true
				report.Steps = step
				report.FinalScreen = a.Sess.Snapshot().Text
				return a.finish(report), nil
			}
			continue
		case ai.ActionDisconnect:
			_ = a.Sess.Logoff()
			report.Completed = goalIndex >= len(goals)
			report.Steps = step
			report.FinalScreen = a.Sess.Snapshot().Text
			a.writeTranscript(transcriptEntry{At: time.Now(), Step: step, Goal: goal,
				Screen: obs.Snapshot.Text, Action: "disconnect", Thought: decision.Thought})
			return a.finish(report), nil
		}

		prepared, perr := a.Policy.Apply(ctx, decision, obs.Snapshot.Text)
		if perr != nil {
			a.Log.Warn("action blocked by policy", "reason", perr)
			lastActions = appendCapped(lastActions, "BLOCKED: "+perr.Error())
			a.writeTranscript(transcriptEntry{At: time.Now(), Step: step, Goal: goal,
				Screen: obs.Snapshot.Text, Action: decision.Action, Value: decision.Value,
				Thought: decision.Thought, Result: "blocked: " + perr.Error()})
			if _, ok := perr.(*BlockedError); !ok {
				runErr = perr
				break
			}
			continue
		}
		decision = prepared

		execErr := a.execute(decision, &report)
		a.Policy.OnAction(decision, obs.Snapshot.Text, execErr)
		lastActions = appendCapped(lastActions, decision.Describe()+resultSuffix(execErr))
		a.trackPost(decision, obs.Snapshot.Text)
		a.writeTranscript(transcriptEntry{At: time.Now(), Step: step, Goal: goal,
			Screen: obs.Snapshot.Text, Action: decision.Action, Value: decision.Value,
			Protocol: decision.Protocol, Thought: decision.Thought, Result: resultText(execErr)})
		if execErr != nil {
			a.Log.Warn("action failed", "action", decision.Action, "err", execErr)
		}

		if obs.Snapshot.Text == lastScreen {
			sameScreen++
		} else {
			sameScreen = 0
			lastScreen = obs.Snapshot.Text
		}
		if sameScreen >= 4 {
			a.Log.Warn("screen has not changed; nudging with Enter")
			_ = a.Sess.SendKeys("<ENTER>")
			sameScreen = 0
		}
	}

	report.FinalScreen = a.Sess.Snapshot().Text
	if !a.Sess.Disconnected() {
		_ = a.Sess.Logoff()
	}
	return a.finish(report), runErr
}

func (a *Agent) finish(report Report) Report {
	report.Ended = time.Now()
	report.Posts = a.Policy.SessionPosts()
	report.Downloads = a.Policy.SessionDownloads()
	if err := a.Mem.Save(); err != nil {
		a.Log.Warn("could not save memory", "err", err)
	}
	return report
}
