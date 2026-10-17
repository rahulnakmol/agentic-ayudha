package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
)

// SurveyGoldenResult reports a characterisation run: the cases checked, and
// with --run what was captured (--golden) or which cases differ (--parity).
type SurveyGoldenResult struct {
	Mode     string         `json:"mode"` // golden, parity, shadow, or compat
	Cases    string         `json:"cases"`
	Count    int            `json:"count"`
	Sets     map[string]int `json:"sets"`
	Problems []string       `json:"problems,omitempty"`
	Ran      bool           `json:"ran"`
	Bin      string         `json:"bin,omitempty"`
	Dir      string         `json:"dir"`
	Golden   *survey.Golden `json:"golden,omitempty"`
	Diffs    []survey.Diff  `json:"diffs,omitempty"`
	// Shadow is the new program run beside the legacy one (--shadow).
	Shadow string `json:"shadow,omitempty"`
	// Compat is a legacy test suite's score on the new build (--compat).
	Compat *survey.CompatReport `json:"compat,omitempty"`
	// Probes is the runtime evidence (--probe), written to probes.json.
	Probes *survey.Probes `json:"probes,omitempty"`
	Next   string         `json:"next"`
}

// surveyGolden checks a cases file and, with --run and --bin, runs it: --golden
// captures the legacy build's outputs under <out>/golden/<cases file name>; --parity replays the
// same cases against the new build and exits 3 on any difference, so a
// modernisation Story's parity criterion is a deterministic floor.
func (e *Env) surveyGolden(ctx context.Context, in Values, root, out, indexFile string) error {
	var mode, file string
	for _, m := range []string{"golden", "parity", "compat"} {
		if in.Has(m) {
			if mode != "" {
				return Errorf(ExitUsage, "survey: --golden captures, --parity compares, and --compat scores a suite; pass one")
			}
			mode, file = m, in.String(m)
		}
	}
	if !in.Has("run") && in.Has("bin") || in.Bool("run") && in.String("bin") == "" {
		return Errorf(ExitUsage, "survey --%s: --run and --bin <program> go together; without them the cases file is only checked", mode)
	}
	if (in.Has("shadow") || in.Has("probe")) && mode != "golden" {
		return Errorf(ExitUsage, "survey --shadow and --probe run the legacy build: use them with --golden <cases> --bin <legacy program> --run")
	}
	if (in.Has("check-bin") || in.Has("coverage")) && !in.Bool("probe") {
		return Errorf(ExitUsage, "survey --check-bin and --coverage go with --probe")
	}
	if in.Has("shadow") {
		mode = "shadow"
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	cs, probs, err := survey.ParseCases(b)
	if err != nil {
		return Errorf(ExitUsage, "%s: %v", file, err)
	}
	// Each cases file has its own capture, so cases that run different
	// programs (a test runner, a command-line tool) never overwrite each other.
	stem := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	res := SurveyGoldenResult{Mode: mode, Cases: file, Count: len(cs.Cases), Sets: map[string]int{}, Problems: probs, Dir: filepath.Join(out, "golden", stem)}
	for _, k := range cs.Cases {
		res.Sets[k.Set]++
		if mode == "compat" && (!strings.HasPrefix(k.Set, "S") || k.Area == "") {
			res.Problems = append(res.Problems, fmt.Sprintf("case %s: a compatibility suite's test names its suite (S1, …) and its area", k.ID))
		}
	}
	probs = res.Problems
	if len(probs) == 0 && in.Bool("run") {
		bin, err := resolveBin(e.Dir, in.String("bin"))
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		res.Bin, res.Ran = bin, true
		ix, _ := survey.Load(indexFile)
		switch mode {
		case "golden":
			commit := ""
			if ix != nil {
				commit = ix.Commit
			}
			res.Golden, err = survey.Capture(ctx, root, bin, commit, res.Dir, cs)
			if err == nil && in.Bool("probe") {
				err = e.surveyProbe(ctx, in, ix, root, bin, file, cs, &res)
			}
		case "shadow":
			var next string
			if next, err = resolveBin(e.Dir, in.String("shadow")); err != nil {
				return Wrap(ExitUsage, err)
			}
			res.Shadow = next
			res.Diffs, err = survey.Shadow(ctx, root, bin, next, cs)
		case "parity":
			res.Diffs, res.Golden, err = survey.Replay(ctx, root, bin, res.Dir, cs)
		case "compat":
			res.Compat, err = survey.Compat(ctx, root, bin, res.Dir, cs)
		}
		if err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	switch {
	case len(probs) > 0:
		res.Next = "fix the cases file, then run again"
	case !res.Ran:
		res.Next = fmt.Sprintf("run it: `aayu survey --%s %s --bin <program> --run`", mode, file)
	case mode == "golden" && res.Probes != nil:
		res.Next = "cite the probe facts (probe:…) in the survey's trap findings and claims; commit the golden directory with the cases file"
	case mode == "golden":
		res.Next = "commit the golden directory with the cases file; each modernisation Story's parity criterion runs `aayu survey --parity " + file + " --bin <new build> --run`"
	case mode == "shadow" && len(res.Diffs) > 0:
		res.Next = "the two builds differ on the same inputs; fix the new build, or record the difference in the compatibility contract (level differ, with the decision)"
	case mode == "shadow":
		res.Next = "both builds agree on every case; keep shadowing on real inputs until the cut-over criteria hold"
	case mode == "compat" && res.Compat.Regressions > 0:
		res.Next = "the new build fails tests the legacy build passes; fix them, or mark a test unfixable in the cases file with its reason"
	case mode == "compat":
		res.Next = "the new build passes every test the legacy build passes; the score is appended to compat-history.jsonl"
	case len(res.Diffs) > 0:
		res.Next = "the new build differs from the legacy build; fix it, or record the dropped behaviour in the survey's case set (exclude) with the question that decided it"
	default:
		res.Next = "parity holds for every case"
	}
	if err := e.Emit(res, func(w io.Writer) error { return writeGolden(w, res) }); err != nil {
		return err
	}
	switch {
	case len(probs) > 0:
		return Errorf(ExitBlocked, "the cases file has %d problem(s)", len(probs))
	case len(res.Diffs) > 0 && mode == "shadow":
		return Errorf(ExitFloor, "shadow: %d difference(s) between the legacy and the new build", len(res.Diffs))
	case len(res.Diffs) > 0:
		return Errorf(ExitFloor, "parity: %d difference(s) from the golden outputs", len(res.Diffs))
	case res.Compat != nil && res.Compat.Regressions > 0:
		return Errorf(ExitFloor, "compat: %d test(s) the legacy build passes fail on the new build", res.Compat.Regressions)
	}
	return nil
}

// surveyProbe gathers runtime evidence from the legacy build and writes it to
// probes.json next to the capture, where `--draft` finds it.
func (e *Env) surveyProbe(ctx context.Context, in Values, ix *survey.Index, root, bin, file string, cs survey.Cases, res *SurveyGoldenResult) error {
	if ix == nil {
		return fmt.Errorf("--probe reads the index to name unreached functions: run `aayu survey` first")
	}
	opt := survey.ProbeOptions{Coverage: in.String("coverage")}
	if c := in.String("check-bin"); c != "" {
		var err error
		if opt.CheckBin, err = resolveBin(e.Dir, c); err != nil {
			return err
		}
	}
	if opt.Coverage != "" && !filepath.IsAbs(opt.Coverage) {
		opt.Coverage = filepath.Join(e.Dir, opt.Coverage)
	}
	p, err := survey.Probe(ctx, ix, root, bin, filepath.Base(file), cs, opt)
	if err != nil {
		return err
	}
	res.Probes = p
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(res.Dir, "probes.json"), append(b, '\n'), 0o644)
}

// resolveBin makes a path with a separator absolute and looks a bare name up
// on PATH, so {bin} means the same program in every case's directory.
func resolveBin(dir, bin string) (string, error) {
	if !strings.ContainsRune(bin, filepath.Separator) {
		return exec.LookPath(bin)
	}
	if !filepath.IsAbs(bin) {
		bin = filepath.Join(dir, bin)
	}
	if _, err := os.Stat(bin); err != nil {
		return "", err
	}
	return bin, nil
}

func writeGolden(w io.Writer, r SurveyGoldenResult) error {
	fmt.Fprintf(w, "survey --%s %s: %d cases (%s)\n", r.Mode, r.Cases, r.Count, countsText(r.Sets))
	for _, p := range r.Problems {
		fmt.Fprintf(w, "  problem: %s\n", p)
	}
	if r.Ran {
		fmt.Fprintf(w, "  ran %s\n", r.Bin)
		switch r.Mode {
		case "golden":
			fmt.Fprintf(w, "  captured %d cases in %s\n", len(r.Golden.Cases), r.Dir)
			if p := r.Probes; p != nil {
				for _, x := range p.Ran {
					fmt.Fprintf(w, "  probe ran: %s\n", x)
				}
				for _, x := range p.Skipped {
					fmt.Fprintf(w, "  probe skipped: %s\n", x)
				}
				fmt.Fprintf(w, "  %d probe fact(s) in %s:\n", len(p.Facts), filepath.Join(r.Dir, "probes.json"))
				fmt.Fprintln(w, "<untrusted source=\"program-output\">")
				for _, f := range p.Facts {
					fmt.Fprintf(untrustedWriter{w}, "  %s: %s\n", f.ID, f.Detail)
				}
				fmt.Fprintln(w, "</untrusted>")
			}
		case "compat":
			c := r.Compat
			fmt.Fprintf(w, "  passes %d of the %d tests the legacy build passes; %d regression(s)\n", c.Pass, c.LegacyPass, c.Regressions)
			for _, a := range c.Areas {
				fmt.Fprintf(w, "    %s: %d of %d (%d tests, %d unfixable, %d skipped)", a.Area, a.Pass, a.LegacyPass, a.Tests, a.Unfixable, a.Skipped)
				if len(a.Regressions) > 0 {
					fmt.Fprintf(w, "; regressions: %s", strings.Join(a.Regressions, ", "))
				}
				if len(a.Fixed) > 0 {
					fmt.Fprintf(w, "; now passing: %s", strings.Join(a.Fixed, ", "))
				}
				fmt.Fprintln(w)
			}
		default:
			if r.Mode == "shadow" {
				fmt.Fprintf(w, "  beside %s\n", r.Shadow)
			}
			fmt.Fprintf(w, "  %d of %d cases differ\n", diffCases(r.Diffs), r.Count)
			if len(r.Diffs) > 0 {
				fmt.Fprintln(w, "<untrusted source=\"program-output\">")
				for _, d := range r.Diffs {
					fmt.Fprintf(untrustedWriter{w}, "  %s (%s) %s: %s\n", d.ID, d.Set, d.Stream, d.Detail)
				}
				fmt.Fprintln(w, "</untrusted>")
			}
		}
	}
	fmt.Fprintf(w, "  next: %s\n", r.Next)
	return nil
}

func diffCases(ds []survey.Diff) int {
	seen := map[string]bool{}
	for _, d := range ds {
		seen[d.ID] = true
	}
	return len(seen)
}
