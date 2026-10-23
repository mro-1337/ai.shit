package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mro-1337/ai.shit/aibbs/internal/agent"
	"github.com/mro-1337/ai.shit/aibbs/internal/ai"
	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/fakebbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

func cmdDemo(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	useLLM := fs.Bool("llm", false, "use the configured language model instead of heuristics")
	stateDir := fs.String("state-dir", "demo-state", "where to keep demo memory and transcripts")
	maxSteps := fs.Int("max-steps", 45, "step budget")
	verbose := fs.Bool("verbose", false, "debug logging")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	srv, err := fakebbs.Start("127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "start mock BBS:", err)
		return 1
	}
	defer srv.Close()
	host, portStr, err := net.SplitHostPort(srv.Addr())
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock BBS address:", err)
		return 1
	}
	port, _ := strconv.Atoi(portStr)

	cfg := config.Default()
	cfg.BBS.Name = "aibbs-demo"
	cfg.BBS.Host, cfg.BBS.Port = host, port
	cfg.BBS.IdleMillis = 250
	cfg.BBS.MaxSettleSeconds = 5
	cfg.Account.Handle = "SynthMind"
	cfg.Behavior.Goals = []string{
		"Read a message and reply to it.",
		"Download a small file from the file area.",
		"Play a door game for a few turns.",
	}
	cfg.Behavior.MinActionDelayMS = 120
	cfg.Behavior.MaxSteps = *maxSteps
	cfg.Behavior.StateDir = *stateDir
	cfg.Downloads.Dir = filepath.Join(*stateDir, "downloads")
	if err := cfg.Normalize(); err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 2
	}

	log, closeLog := newLogger(*verbose, "")
	defer closeLog()

	var brain ai.Brain
	if *useLLM {
		brain, err = ai.NewBrain(cfg, log, false)
		if err != nil {
			log.Error("language model is not configured", "err", err)
			return 1
		}
	} else {
		brain = ai.NewHeuristicBrain()
	}

	mem, err := memory.Load(filepath.Join(cfg.StateDir(), "memory.json"), cfg.BBS.Name)
	if err != nil {
		log.Error("load memory", "err", err)
		return 1
	}
	sess, err := bbs.Dial(ctx, cfg, mem, log)
	if err != nil {
		log.Error("connect", "err", err)
		return 1
	}
	defer sess.Close()

	fmt.Printf("Mock BBS listening on %s (user %s, brain %s)\n\n", srv.Addr(), cfg.Account.Handle, brain.Name())
	login, err := sess.Login(ctx)
	if err != nil {
		log.Error("login", "err", err)
		fmt.Fprintln(os.Stderr, "\nLast screen:\n"+sess.Snapshot().Text)
		return 1
	}
	if login.Registered {
		fmt.Println("Registered a brand new account on the mock BBS.")
	}

	report, runErr := agent.New(cfg, sess, brain, mem, log).Run(ctx)
	fmt.Println()
	fmt.Print(report.Summary())
	fmt.Printf("\nMock BBS counters: %+v\n", srv.Stats())
	if runErr != nil {
		log.Error("demo ended with an error", "err", runErr)
		fmt.Fprintln(os.Stderr, "\nFinal screen:\n"+report.FinalScreen)
		return 1
	}
	if !report.Completed {
		fmt.Fprintln(os.Stderr, "demo did not complete all goals")
		return 1
	}
	return 0
}
