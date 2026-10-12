// Command eval runs the skill eval harness outside go test (WP3.4). It is the
// entry point of .github/workflows/evals.yml and of the promotion gate:
//
//	go run ./internal/evals/cmd/eval -tier 1                                     # deterministic, no model calls
//	go run ./internal/evals/cmd/eval -tier 2 -group <group> -sha <sha> -out evals/results/<sha>-<group>.json [-baseline <file>]
//	go run ./internal/evals/cmd/eval -groups                                     # the group names, as JSON
//	go run ./internal/evals/cmd/eval -compare evals/results/<sha>.json [-baseline <file>]
//
// Tier 2 makes live model calls through [models.gateway] (AAYU_MODELS__GATEWAY__BASE_URL and
// AAYU_MODELS__GATEWAY__SECRET, key in AAYU_SECRET_<NAME>), capped by [budget] and the -group's max_usd in evals/eval.toml.
// It exits 1 when the results fail the bar or the baseline comparison.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/evals"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	tier := flag.Int("tier", 0, "1: deterministic checks; 2: rubric runs with live model calls")
	sha := flag.String("sha", "", "commit the results are for (tier 2)")
	ref := flag.String("ref", "", "branch or ref, recorded in the results (tier 2)")
	out := flag.String("out", "", "results file to write (tier 2)")
	compare := flag.String("compare", "", "results file to check against the bar and -baseline, without running anything")
	baseline := flag.String("baseline", "", "baseline results file (evals/results/<main-sha>.json); a missing file means no baseline")
	group := flag.String("group", "", "tier 2: the group to run ([[groups]] in evals/eval.toml), with its own max_usd")
	groups := flag.Bool("groups", false, "print the tier-2 group names as a JSON array and exit")
	only := flag.String("cases", "", "tier 2: run only the cases whose ID (<skill>/<name>) matches this regular expression")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := evals.LoadConfig(root)
	if err != nil {
		return err
	}
	base, err := readResults(*baseline)
	if err != nil {
		return err
	}
	switch {
	case *groups:
		var names []string
		for _, g := range cfg.Groups {
			names = append(names, g.Name)
		}
		b, _ := json.Marshal(names)
		fmt.Println(string(b))
		return nil
	case *compare != "":
		cur, err := readResults(*compare)
		if err != nil {
			return err
		}
		if cur == nil {
			return fmt.Errorf("-compare %s: no such results file", *compare)
		}
		return verdict(cur, base, cfg)
	case *tier == 1:
		cases, err := evals.LoadCases(root)
		if err != nil {
			return err
		}
		failed := 0
		for _, c := range cases {
			r := evals.Tier1(ctx, root, c, aayudha.Assets)
			fmt.Print(r)
			if !r.Passed() {
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("tier 1: %d of %d case(s) failed", failed, len(cases))
		}
		return nil
	case *tier == 2:
		if *sha == "" || *out == "" {
			return errors.New("tier 2 needs -sha and -out")
		}
		cases, err := evals.LoadCases(root)
		if err != nil {
			return err
		}
		if cfg, cases, err = cfg.ForGroup(*group, cases); err != nil {
			return fmt.Errorf("tier 2 needs -group: %w", err)
		}
		h, err := harness(root, cfg)
		if err != nil {
			return err
		}
		if *only != "" {
			re, err := regexp.Compile(*only)
			if err != nil {
				return fmt.Errorf("-cases: %w", err)
			}
			cases = slices.DeleteFunc(cases, func(c evals.Case) bool { return !re.MatchString(c.ID) })
		}
		res, err := h.Tier2(ctx, cases, *sha, *ref, time.Now())
		if err != nil {
			return err
		}
		res.Group = *group
		res.Verdict, res.Reasons = evals.Compare(res, base, cfg)
		if base != nil {
			res.Baseline = base.SHA
		}
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%.4f USD at the charged rate)\n", *out, res.CostUSD)
		return verdict(res, base, cfg)
	}
	return errors.New("pass -tier 1, -tier 2, or -compare <file>")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func readResults(path string) (*evals.Results, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r evals.Results
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &r, nil
}

func verdict(cur, base *evals.Results, cfg evals.Config) error {
	v, reasons := evals.Compare(cur, base, cfg)
	against := "no baseline"
	if base != nil {
		against = "baseline " + base.SHA
	}
	fmt.Printf("tier 2 %s: pass^%d %.2f, mean %.2f (%s): %s\n", cur.SHA, cur.K, cur.PassK, cur.Mean, against, v)
	for _, c := range cur.Cases {
		fmt.Printf("  %s on %s (%s): pass^%d %.0f, mean %.2f over %d completed run(s), %d error(s)\n", c.Case, c.Model, c.Family, cur.K, c.PassK, c.Mean, len(c.Runs)-c.Errors, c.Errors)
		for i, r := range c.Runs {
			var calls []string
			for _, l := range r.Calls {
				calls = append(calls, fmt.Sprintf("%s %s stop=%s out=%d/%d in=%d", l.Role, l.Model, orDash(l.StopReason), l.TokensOut, l.MaxTokens, l.TokensIn))
			}
			fmt.Printf("    run %d %s score %d: %s\n", i+1, r.Status, r.Score, strings.Join(calls, "; "))
			if r.Status == evals.RunError {
				fmt.Printf("      %s; reply: %q\n", r.Error, r.Excerpt)
			}
		}
	}
	for _, r := range reasons {
		fmt.Println("  FAIL:", r)
	}
	if v != evals.Pass {
		return errors.New("tier 2 failed")
	}
	return nil
}

// harness wires the model gateway: configuration layers (the env layer carries
// [models.gateway]), the env secrets backend, a scratch actions log, and the
// [budget] breaker.
func harness(root string, cfg evals.Config) (*evals.Harness, error) {
	l, err := config.Load(config.DefaultSources(os.Getenv, root, os.Environ(), nil))
	if err != nil {
		return nil, err
	}
	clock, err := l.Config.Clock()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "aayu-evals-")
	if err != nil {
		return nil, err
	}
	log, err := actions.Open(filepath.Join(tmp, "actions.jsonl"), actions.Options{Host: string(verbs.DirectHost)})
	if err != nil {
		return nil, err
	}
	store := secrets.New(secrets.Env{Getenv: os.Getenv, Environ: os.Environ()})
	m := evals.JudgeModels(l.Config.Models, cfg.Judge)
	gw := &models.Gateway{
		Models:   m,
		Resolver: models.Resolver{Models: m, Log: func(s string) { fmt.Fprintln(os.Stderr, s) }},
		Actions:  log,
		Breaker:  &models.Breaker{Budget: l.Config.Budget, Clock: clock, Now: time.Now, PauseFile: filepath.Join(tmp, "paused")},
		Key: func(name string) (string, error) {
			v, _, err := store.Get(name)
			return v, err
		},
	}
	return &evals.Harness{Root: root, Config: cfg, Gateway: gw, Assets: aayudha.Assets}, nil
}
