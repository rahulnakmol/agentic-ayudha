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
	s, err := store.Load()
	if err != nil {
		return err
	}
	w := stdout
	var f *os.File
	if *out != "" {
		if f, err = os.Create(*out); err != nil {
			return err
		}
		w = f
	}
	st, err := analytics.Simulate(analytics.Config{Channel: *channel, Seed: *seed, From: start, Days: *days, Users: *users}, s, w)
	if f != nil {
		if cerr := f.Close(); err == nil {
			err = cerr // a failed close can lose the last writes
		}
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "simulate: %s, seed %d: %d users, %d questions, %d events\n", *channel, *seed, st.Users, st.Questions, st.Events)
	return err
}
