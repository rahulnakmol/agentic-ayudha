// Command simulate writes a synthetic Application Insights export of the
// benefits-qa example (docs/BUILD-PLAN.md WP8.1): a seeded population of
// fictional employees asking benefits questions through a channel.
//
//	go run ./examples/benefits-qa/analytics/simulate -channel benefits-qa -seed 2 -from 2026-09-14 -out after.jsonl
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/analytics"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "simulate:", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	channel := fs.String("channel", analytics.ChannelBenefitsQA, "hr-ticket or benefits-qa")
	seed := fs.Uint64("seed", 1, "random seed")
	from := fs.String("from", "", "window start, YYYY-MM-DD (UTC)")
	days := fs.Int("days", 28, "window length in days")
	users := fs.Int("users", 120, "synthetic employees")
	out := fs.String("out", "", "output file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	start, err := time.Parse(time.DateOnly, *from)
	if err != nil {
		return fmt.Errorf("-from: %w", err)
	}
	cfg := analytics.Config{Channel: *channel, Seed: *seed, From: start, Days: *days, Users: *users}
	if err := cfg.Validate(); err != nil {
		return err
	}
	s, err := store.Load()
	if err != nil {
		return err
	}
	var st analytics.Stats
	if *out == "" {
		st, err = analytics.Simulate(cfg, s, stdout)
	} else {
		st, err = writeFile(*out, func(w io.Writer) (analytics.Stats, error) { return analytics.Simulate(cfg, s, w) })
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "simulate: %s, seed %d: %d users, %d questions, %d events\n", *channel, *seed, st.Users, st.Questions, st.Events)
	return err
}

// writeFile writes to a temporary file beside path and renames it over path
// only when write and close succeed, so a failed run never truncates or
// half-writes an existing export.
func writeFile(path string, write func(io.Writer) (analytics.Stats, error)) (analytics.Stats, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return analytics.Stats{}, err
	}
	defer func() { _ = os.Remove(f.Name()) }() // a no-op after the rename
	st, err := write(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	return st, err
}
