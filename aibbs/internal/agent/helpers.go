package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
)

func (a *Agent) buildObservation(obs bbs.Observation, goals []string, goalIndex, step int) ai.Observation {
	goal := ""
	if goalIndex < len(goals) {
		goal = goals[goalIndex]
	}
	postsLeft, downloadsLeft := a.Policy.SessionLimits()
	if daily := a.Mem.PostsRemaining(a.Cfg.Behavior.MaxPostsPerSession); daily < postsLeft {
		postsLeft = daily
	}
	if daily := a.Mem.DownloadsRemaining(a.Cfg.Behavior.MaxDownloadsPerSession); daily < downloadsLeft {
		downloadsLeft = daily
	}

	doorName, doorHint := "", ""
	if obs.Door != nil {
		doorName = obs.Door.Name
		doorHint = obs.Door.Brief()
		if hint := obs.Door.LaunchHint(); hint != "" {
			doorHint += "\n" + hint
		}
	}

	return ai.Observation{
		BBSName:            a.Cfg.BBS.Name,
		Handle:             a.Cfg.Account.Handle,
		Screen:             obs.Snapshot.Text,
		Recent:             obs.Recent,
		Goals:              goals,
		Goal:               goal,
		GoalIndex:          goalIndex,
		GoalCount:          len(goals),
		Step:               step,
		MaxSteps:           a.Cfg.Behavior.MaxSteps,
		DoorName:           doorName,
		DoorHint:           doorHint,
		Memory:             a.Mem.Summary(8),
		LastActions:        nil,
		PostsRemaining:     postsLeft,
		DownloadsRemaining: downloadsLeft,
		ReadOnly:           a.Cfg.Behavior.ReadOnly,
		DryRun:             a.Cfg.Behavior.DryRun,
		AllowDoors:         a.Cfg.Behavior.AllowDoors,
		DownloadDir:        a.Cfg.Downloads.Dir,
	}
}

func (a *Agent) execute(d ai.Decision, report *Report) error {
	switch d.Action {
	case ai.ActionKeys:
		return a.Sess.SendKeys(d.Value)
	case ai.ActionText:
		return a.Sess.SendLine(d.Value)
	case ai.ActionDownload:
		files, err := a.Sess.Download(d.Protocol)
		if len(files) > 0 {
			report.Files = append(report.Files, files...)
			report.Downloads += len(files)
		}
		return err
	case ai.ActionWait:
		seconds := 2.0
		if v, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64); err == nil && v > 0 {
			seconds = v
		}
		if seconds > 10 {
			seconds = 10
		}
		sleepContext(context.Background(), time.Duration(seconds*float64(time.Second)))
		return a.Sess.Settle()
	case ai.ActionNote:
		note := d.Note
		if note == "" {
			note = d.Value
		}
		if note != "" {
			a.Mem.AddNote(note)
			report.Notes++
		}
		return nil
	default:
		return fmt.Errorf("cannot execute action %q", d.Action)
	}
}

func (a *Agent) trackPost(d ai.Decision, screen string) {
	if d.Action != ai.ActionText {
		return
	}
	lower := strings.ToLower(screen)
	switch {
	case strings.Contains(lower, "subject:"):
		if len(d.Value) > 2 {
			a.pendingSubject = d.Value
		}
	case strings.Contains(lower, "enter your message"), strings.Contains(lower, "message body"),
		strings.Contains(lower, "type your message"):
		if len(d.Value) > 2 && d.Value != "." {
			a.pendingBody = d.Value
		}
	}
}

func (a *Agent) noteConfirmedPost(screen string) {
	if a.pendingBody == "" {
		return
	}
	lower := strings.ToLower(screen)
	if strings.Contains(lower, "message posted") || strings.Contains(lower, "post saved") ||
		strings.Contains(lower, "message saved") {
		a.Mem.AddPost("", a.pendingSubject, a.pendingBody)
		a.Log.Info("post recorded", "subject", a.pendingSubject)
		a.pendingSubject, a.pendingBody = "", ""
	}
}

func appendCapped(list []string, item string) []string {
	list = append(list, item)
	if len(list) > 10 {
		list = list[len(list)-10:]
	}
	return list
}

func resultText(err error) string {
	if err == nil {
		return "ok"
	}
	return "error: " + err.Error()
}

func resultSuffix(err error) string {
	if err == nil {
		return ""
	}
	return " [error: " + err.Error() + "]"
}

func sleepContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
