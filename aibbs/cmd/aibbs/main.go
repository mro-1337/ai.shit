// Command aibbs is an AI user for Telnet BBSes.
//
// It logs in (or registers), reads and posts messages, downloads files over
// ZMODEM/XMODEM, and plays door games, driven by an OpenAI-compatible model
// or the built-in offline heuristics.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

const version = "0.1.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args))
}

func run(ctx context.Context, argv []string) int {
	if len(argv) < 2 {
		usage(os.Stderr)
		return 2
	}
	switch argv[1] {
	case "run":
		return cmdRun(ctx, argv[2:])
	case "probe":
		return cmdProbe(ctx, argv[2:])
	case "demo":
		return cmdDemo(ctx, argv[2:])
	case "init":
		return cmdInit(argv[2:])
	case "version", "--version", "-v":
		fmt.Println("aibbs " + version)
		return 0
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", argv[1])
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `aibbs - an AI user for Telnet BBSes

Usage:
  aibbs run   --config aibbs.json [options]   log in and work through goals
  aibbs probe --host HOST --port PORT         connect and print the screen
  aibbs demo  [--llm]                         run against the built-in mock BBS
  aibbs init  [--out aibbs.json] [--force]    write an example config
  aibbs version

Run options:
  --brain auto|llm|heuristic   auto uses the LLM when a key is configured
  --goals "a;b;c"              replace the configured goals for this run
  --max-steps N                override the step budget
  --dry-run                    navigate, but never actually post
  --read-only                  no posting and no downloads
  --timeout 10m                overall session deadline
  --verbose                    debug logging
  --log-file path              also append logs to a file

Examples:
  aibbs init
  OPENAI_API_KEY=sk-... aibbs run --config aibbs.json
  aibbs demo
  aibbs probe --host vert.synchro.net --port 23

Configuring a model:
  Any OpenAI-compatible endpoint works. Set ai.base_url, ai.model and either
  ai.api_key or ai.api_key_env in the config. For a local Ollama server use
  "provider": "ollama" (base_url defaults to http://127.0.0.1:11434/v1).
`)
}

func newLogger(verbose bool, logFile string) (*slog.Logger, func()) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	var writers []io.Writer
	writers = append(writers, os.Stderr)
	closer := func() {}
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err == nil {
			writers = append(writers, f)
			closer = func() { _ = f.Close() }
		}
	}
	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	return slog.New(handler), closer
}
