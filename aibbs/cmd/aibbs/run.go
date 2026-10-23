package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mro-1337/ai.shit/aibbs/internal/agent"
	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

func cmdRun(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "aibbs.json", "path to the JSON config")
	brainMode := fs.String("brain", "auto", "auto, llm or heuristic")
	goals := fs.String("goals", "", "semicolon-separated goals for this run")
	maxSteps := fs.Int("max-steps", 0, "override behavior.max_steps")
	dryRun := fs.Bool("dry-run", false, "navigate but never post")
	readOnly := fs.Bool("read-only", false, "no posting and no downloads")
	timeout := fs.Duration("timeout", 0, "overall session deadline")
	verbose := fs.Bool("verbose", false, "debug logging")
	logFile := fs.String("log-file", "", "append logs to this file")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 2
	}
	if *maxSteps > 0 {
		cfg.Behavior.MaxSteps = *maxSteps
	}
	if *goals != "" {
		var list []string
		for _, g := range strings.Split(*goals, ";") {
			if g = strings.TrimSpace(g); g != "" {
				list = append(list, g)
			}
		}
		if len(list) > 0 {
			cfg.Behavior.Goals = list
		}
	}
	cfg.Behavior.DryRun = cfg.Behavior.DryRun || *dryRun
	cfg.Behavior.ReadOnly = cfg.Behavior.ReadOnly || *readOnly

	log, closeLog := newLogger(*verbose, *logFile)
	defer closeLog()

	mem, err := memory.Load(filepath.Join(cfg.StateDir(), "memory.json"), cfg.BBS.Name)
	if err != nil {
		log.Error("load memory", "err", err)
		return 1
	}
	brain, err := ai.NewBrain(cfg, log, strings.EqualFold(*brainMode, "heuristic"))
	if err != nil {
		if strings.EqualFold(*brainMode, "auto") {
			log.Warn("no language model configured; using the offline heuristic brain", "reason", err)
			brain = ai.NewHeuristicBrain()
		} else {
			log.Error("brain", "err", err)
			return 1
		}
	}

	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	log.Info("connecting", "bbs", cfg.BBS.Name, "host", cfg.BBS.Host, "port", cfg.BBS.Port, "brain", brain.Name())
	sess, err := bbs.Dial(ctx, cfg, mem, log)
	if err != nil {
		log.Error("connect", "err", err)
		return 1
	}
	defer sess.Close()

	login, err := sess.Login(ctx)
	if err != nil {
		log.Error("login", "err", err)
		fmt.Fprintln(os.Stderr, "\nLast screen:\n"+sess.Snapshot().Text)
		return 1
	}
	log.Info("logged in", "handle", login.Handle, "registered", login.Registered)
	if login.GeneratedPassword {
		log.Info("a password was generated and saved in the state file")
	}

	report, runErr := agent.New(cfg, sess, brain, mem, log).Run(ctx)
	fmt.Println()
	fmt.Print(report.Summary())
	if runErr != nil {
		log.Error("session ended with an error", "err", runErr)
		if report.FinalScreen != "" && *verbose {
			fmt.Fprintln(os.Stderr, "\nFinal screen:\n"+report.FinalScreen)
		}
		return 1
	}
	if !report.Completed {
		return 1
	}
	return 0
}
