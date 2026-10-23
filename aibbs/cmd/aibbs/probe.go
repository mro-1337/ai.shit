package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mro-1337/ai.shit/aibbs/internal/bbs"
	"github.com/mro-1337/ai.shit/aibbs/internal/config"
	"github.com/mro-1337/ai.shit/aibbs/internal/memory"
)

func cmdProbe(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	configPath := fs.String("config", "", "optional config file to take BBS settings from")
	host := fs.String("host", "", "BBS host")
	port := fs.Int("port", 23, "BBS port")
	cols := fs.Int("cols", 80, "terminal columns")
	rows := fs.Int("rows", 24, "terminal rows")
	seconds := fs.Float64("seconds", 4, "how long to wait for the screen to settle")
	verbose := fs.Bool("verbose", false, "debug logging")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config:", err)
			return 2
		}
	} else {
		cfg = config.Default()
		cfg.BBS.Name = "probe"
		cfg.BBS.Cols, cfg.BBS.Rows = *cols, *rows
		if *host != "" {
			cfg.BBS.Host = *host
		}
		cfg.BBS.Port = *port
		cfg.BBS.MaxSettleSeconds = int(*seconds) + 2
		cfg.BBS.DialTimeoutSec = 15
		if err := cfg.Normalize(); err != nil {
			fmt.Fprintln(os.Stderr, "config:", err)
			return 2
		}
	}
	if *host != "" {
		cfg.BBS.Host = *host
	}
	if *port != 23 {
		cfg.BBS.Port = *port
	}
	if *configPath == "" && *host == "" {
		fmt.Fprintln(os.Stderr, "probe needs --host or --config")
		return 2
	}

	log, closeLog := newLogger(*verbose, "")
	defer closeLog()
	mem, _ := memory.Load(filepath.Join(os.TempDir(), "aibbs-probe-memory.json"), "probe")
	sess, err := bbs.Dial(ctx, cfg, mem, log)
	if err != nil {
		log.Error("connect", "err", err)
		return 1
	}
	defer sess.Close()
	if err := sess.Settle(); err != nil {
		log.Error("read", "err", err)
	}
	fmt.Printf("--- %s (%s:%d) %dx%d ---\n", cfg.BBS.Name, cfg.BBS.Host, cfg.BBS.Port, cfg.BBS.Cols, cfg.BBS.Rows)
	fmt.Println(sess.Snapshot().Text)
	return 0
}

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	out := fs.String("out", "aibbs.json", "where to write the example config")
	force := fs.Bool("force", false, "overwrite an existing file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if _, err := os.Stat(*out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "%s already exists (use --force to overwrite)\n", *out)
		return 1
	}
	data, err := config.Example()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		return 1
	}
	fmt.Printf("Wrote %s. Edit the bbs/account/ai sections, then run:\n  aibbs run --config %s\n", *out, *out)
	return 0
}
