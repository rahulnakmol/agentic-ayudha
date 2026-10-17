package cli

import (
	"context"
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
	Mode     string         `json:"mode"` // golden or parity
	Cases    string         `json:"cases"`
	Count    int            `json:"count"`
	Sets     map[string]int `json:"sets"`
	Problems []string       `json:"problems,omitempty"`
	Ran      bool           `json:"ran"`
	Bin      string         `json:"bin,omitempty"`
	Dir      string         `json:"dir"`
	Golden   *survey.Golden `json:"golden,omitempty"`
	Diffs    []survey.Diff  `json:"diffs,omitempty"`
	Next     string         `json:"next"`
}

// surveyGolden checks a cases file and, with --run and --bin, runs it: --golden
// captures the legacy build's outputs under <out>/golden/<cases file name>; --parity replays the
// same cases against the new build and exits 3 on any difference, so a
// modernisation Story's parity criterion is a deterministic floor.
func (e *Env) surveyGolden(ctx context.Context, in Values, root, out, indexFile string) error {
	mode, file := "golden", in.String("golden")
	if in.Has("parity") {
		if in.Has("golden") {
			return Errorf(ExitUsage, "survey: --golden captures and --parity compares; pass one")
		}
		mode, file = "parity", in.String("parity")
	}
	if !in.Has("run") && in.Has("bin") || in.Bool("run") && in.String("bin") == "" {
		return Errorf(ExitUsage, "survey --%s: --run and --bin <program> go together; without them the cases file is only checked", mode)
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
	}
	if len(probs) == 0 && in.Bool("run") {
		bin, err := resolveBin(e.Dir, in.String("bin"))
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		res.Bin, res.Ran = bin, true
		if mode == "golden" {
			commit := ""
			if ix, err := survey.Load(indexFile); err == nil {
				commit = ix.Commit
			}
			res.Golden, err = survey.Capture(ctx, root, bin, commit, res.Dir, cs)
		} else {
			res.Diffs, res.Golden, err = survey.Replay(ctx, root, bin, res.Dir, cs)
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
	case mode == "golden":
		res.Next = "commit the golden directory with the cases file; each modernisation Story's parity criterion runs `aayu survey --parity " + file + " --bin <new build> --run`"
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
	case len(res.Diffs) > 0:
		return Errorf(ExitFloor, "parity: %d difference(s) from the golden outputs", len(res.Diffs))
	}
	return nil
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
		if r.Mode == "golden" {
			fmt.Fprintf(w, "  captured %d cases in %s\n", len(r.Golden.Cases), r.Dir)
		} else {
			fmt.Fprintf(w, "  %d of %d cases differ from %s\n", diffCases(r.Diffs), r.Count, r.Dir)
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
