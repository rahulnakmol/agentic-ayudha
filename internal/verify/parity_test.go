package verify

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// legacyCalc is the legacy build: it prints its argument doubled, and its
// suite test "neg" passes only for a positive argument.
const legacyCalc = "#!/bin/sh\nif [ \"$1\" = neg ]; then exit 0; fi\necho $(( $1 * 2 ))\n"

// parityRoot is a checkout with a committed legacy capture of two case sets
// (G1: cases one and two; S1: suite test neg in area signs) and a new build
// at build/calc written by the build command from build/calc.src.
func parityRoot(t *testing.T, next, evidence string) string {
	t.Helper()
	root := t.TempDir()
	write := func(p, s string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(s), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("legacy/calc", legacyCalc, 0o755)
	write("build/calc.src", next, 0o644)
	write(".aayudha/survey/cases.json", `{"cases": [
  {"id": "one", "set": "G1", "argv": ["{bin}", "1"]},
  {"id": "two", "set": "G1", "argv": ["{bin}", "2"]}
]}`, 0o644)
	write(".aayudha/survey/suite.json", `{"cases": [{"id": "neg", "set": "S1", "area": "signs", "argv": ["{bin}", "neg"]}]}`, 0o644)
	write(".aayudha/evidence/9.toml", evidence, 0o644)
	ctx := context.Background()
	for _, f := range []string{"cases", "suite"} {
		b, _ := os.ReadFile(filepath.Join(root, ".aayudha/survey", f+".json"))
		cs, probs, err := survey.ParseCases(b)
		if err != nil || len(probs) > 0 {
			t.Fatal(err, probs)
		}
		if _, err := survey.Capture(ctx, root, filepath.Join(root, "legacy/calc"), "abc1234", filepath.Join(root, ".aayudha/survey/golden", f), cs); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const parityBuild = `build = ["sh", "-c", "cp build/calc.src build/calc && chmod +x build/calc"]`

// TestParityFloor (WP5.9, D-5.9m): a modernisation Story whose criterion runs
// `aayu survey --parity` or `--compat` cannot merge while the new build
// differs from the legacy capture, regresses a legacy-passing test, or has no
// run in the evidence map; when it holds, the criterion is evidenced.
func TestParityFloor(t *testing.T) {
	ctx := context.Background()
	parity := "[[criterion]]\nid = 1\n[[criterion.parity]]\ncases = \".aayudha/survey/cases.json\"\nbin = \"build/calc\"\n" + parityBuild + "\n"
	compat := "[[criterion]]\nid = 2\n[[criterion.compat]]\ncases = \".aayudha/survey/suite.json\"\nbin = \"build/calc\"\n" + parityBuild + "\n"
	crit := []workitems.Criterion{
		{N: 1, Evidence: "api-trace", State: "the Rust build and case set G1", Expected: "aayu survey --parity reports no difference"},
		{N: 2, Evidence: "metric", State: "suite S1", Expected: "aayu survey --compat reports no regression"},
		{N: 3, Evidence: "screenshot", State: "the about page", Expected: "it shows the version"},
	}
	// The new build doubles like the legacy one; it differs only on 2 and on neg.
	same := "#!/bin/sh\nif [ \"$1\" = neg ]; then exit 0; fi\necho $(( $1 * 2 ))\n"
	wrongTwo := "#!/bin/sh\nif [ \"$1\" = neg ]; then exit 0; fi\nif [ \"$1\" = 2 ]; then echo 5; exit 0; fi\necho $(( $1 * 2 ))\n"
	failsNeg := "#!/bin/sh\nif [ \"$1\" = neg ]; then exit 1; fi\necho $(( $1 * 2 ))\n"
	tests := []struct {
		name, next, evidence string
		result               string
		evidenced            []int
		detail               string
	}{
		{"both hold", same, parity + compat, proof.Pass, []int{1, 2}, "criterion 1: 2 cases match the legacy capture; criterion 2: 1 of 1 legacy-passing tests pass"},
		{"one case differs", wrongTwo, parity + compat, proof.Fail, []int{2}, "criterion 1: 1 of 2 cases differ from the legacy capture (two stdout"},
		{"the Story ports only case one", wrongTwo, strings.Replace(parity, "\nbin", "\nids = [\"one\"]\nbin", 1) + compat, proof.Pass, []int{1, 2}, "criterion 1: 1 cases match"},
		{"a legacy-passing test regresses", failsNeg, parity + compat, proof.Fail, []int{1}, "criterion 2: 1 test(s) the legacy build passes fail on the new build"},
		{"a criterion has no run", same, parity, proof.Blocked, nil, "criterion 2 runs `aayu survey --compat` but .aayudha/evidence/9.toml gives it no [[criterion.compat]] run"},
		{"the build fails", same, strings.Replace(parity, "cp build", "false && cp build", 1) + compat, proof.Blocked, nil, "exited 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := parityRoot(t, tc.next, tc.evidence)
			r := &Run{Root: root, Story: 9, Criteria: crit, Work: t.TempDir(), Sandbox: &Sandbox{Home: t.TempDir()}}
			if ok, why := (Parity{}).Applies(r); !ok {
				t.Fatalf("does not apply: %s", why)
			}
			rep := Execute(ctx, r, []Floor{Parity{}})
			c := rep.Checks[0]
			if c.Result != tc.result || !strings.Contains(c.Detail, tc.detail) {
				t.Fatalf("got %s %q; want %s containing %q", c.Result, c.Detail, tc.result, tc.detail)
			}
			p, _ := Pack(r, rep, proof.Pack{})
			var got []int
			for _, pc := range p.Criteria {
				got = append(got, pc.ID)
			}
			if !slices.Equal(got, tc.evidenced) {
				t.Fatalf("evidenced criteria %v; want %v", got, tc.evidenced)
			}
			if _, err := os.Stat(filepath.Join(root, ".aayudha/survey/golden/suite/compat-history.jsonl")); err == nil {
				t.Fatal("verify appended to the checkout's compat history")
			}
		})
	}
	r := &Run{Root: t.TempDir(), Story: 9, Criteria: crit[2:]}
	if ok, _ := (Parity{}).Applies(r); ok {
		t.Fatal("applies to a Story with no parity or compat criterion")
	}
}
