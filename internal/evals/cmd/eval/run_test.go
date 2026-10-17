package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/evals"
)

// eval runs the command from the repository root with args and returns what
// it printed and its error.
func eval(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return evalIn(t, repoRoot, func() error { return run(context.Background()) }, args...)
}

// evalIn runs entry in dir with args as the command line. The flag set is
// fresh per call, as for a new process, but returns parse errors instead of
// exiting.
func evalIn(t *testing.T, dir string, entry func() error, args ...string) (string, error) {
	t.Helper()
	t.Chdir(dir)
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	oldArgs, oldFlags, oldStdout := os.Args, flag.CommandLine, os.Stdout
	t.Cleanup(func() { os.Args, flag.CommandLine, os.Stdout = oldArgs, oldFlags, oldStdout })
	flag.CommandLine = flag.NewFlagSet("eval", flag.ContinueOnError)
	flag.CommandLine.SetOutput(out)
	os.Args = append([]string{"eval"}, args...)
	os.Stdout = out
	err = entry()
	os.Stdout = oldStdout
	b, _ := os.ReadFile(out.Name())
	_ = out.Close()
	return string(b), err
}

// repoRoot is the repository root, resolved before any test changes directory.
var repoRoot, _ = filepath.Abs(filepath.Join("..", "..", "..", ".."))

const testdata = "internal/evals/testdata/"

// The entry point prints the tier-2 group names for the workflow's matrix.
func TestGroups(t *testing.T) {
	out, err := evalIn(t, repoRoot, func() error { main(); return nil }, "-groups")
	if err != nil || strings.TrimSpace(out) != `["brief-plain","brief","personas","review","conversations","tell","host-skills","ux","skills"]` {
		t.Fatalf("-groups: %v %q", err, out)
	}
	// evals-gate compares only the groups that gate (D-7.3h).
	out, err = evalIn(t, repoRoot, func() error { main(); return nil }, "-groups", "-gating")
	if err != nil || strings.TrimSpace(out) != `["brief","skills"]` {
		t.Fatalf("-groups -gating: %v %q", err, out)
	}
}

// The review group (WP4.8b) runs on dispatch only: on a push or the nightly
// schedule its tier-2 run does nothing and writes no results.
func TestReviewGroupDispatchOnly(t *testing.T) {
	out := filepath.Join(t.TempDir(), "r.json")
	t.Setenv("EVENT", "push")
	got, err := evalIn(t, repoRoot, func() error { main(); return nil }, "-tier", "2", "-group", "review", "-sha", "abc", "-out", out)
	if err != nil || !strings.Contains(got, "runs on workflow_dispatch only") {
		t.Fatalf("%v %q", err, got)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("results written on a push")
	}
}

// copyTree copies the directory src to dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if fi.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Outside a repository with evals/eval.toml the command refuses to run; a
// failing case fails tier 1, and a case that does not load stops both tiers.
func TestTier1(t *testing.T) {
	isolate(t, "")
	root := t.TempDir()
	if _, err := evalIn(t, root, func() error { return run(context.Background()) }, "-tier", "1"); err == nil || !strings.Contains(err.Error(), "eval.toml") {
		t.Fatalf("-tier 1 outside a repository: %v", err)
	}
	for _, d := range []string{"evals/schemas", "evals/cases/stub/csv-export"} {
		copyTree(t, filepath.Join(repoRoot, filepath.FromSlash(d)), filepath.Join(root, filepath.FromSlash(d)))
	}
	b, _ := os.ReadFile(filepath.Join(repoRoot, "evals", "eval.toml"))
	_ = os.WriteFile(filepath.Join(root, "evals", "eval.toml"), b, 0o644)
	if out, err := evalIn(t, root, func() error { return run(context.Background()) }, "-tier", "1"); err != nil || !strings.Contains(out, "stub/csv-export") {
		t.Fatalf("-tier 1 on a passing case: %v\n%s", err, out)
	}
	_ = os.Remove(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	out, err := evalIn(t, root, func() error { return run(context.Background()) }, "-tier", "1")
	if err == nil || err.Error() != "tier 1: 1 of 1 case(s) failed" || !strings.Contains(out, "output/issue.json is missing") {
		t.Fatalf("-tier 1 with a deleted artifact: %v\n%s", err, out)
	}

	broken := filepath.Join(root, "evals", "cases", "stub", "broken")
	_ = os.MkdirAll(broken, 0o755)
	_ = os.WriteFile(filepath.Join(broken, "case.toml"), []byte("skill = \"stub\"\n"), 0o644)
	for _, args := range [][]string{{"-tier", "1"}, {"-tier", "2", "-sha", "abc", "-out", filepath.Join(root, "r.json"), "-group", "skills"}} {
		if _, err := evalIn(t, root, func() error { return run(context.Background()) }, args...); err == nil || !strings.Contains(err.Error(), "stub/broken: skill and input are required") {
			t.Errorf("%v with a case that does not load: %v", args, err)
		}
	}
}

// -compare checks a results file against the bar and the baseline without
// running anything; a missing baseline file means no baseline.
func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		fail bool
		want string
	}{
		{"drop 0.4", []string{"-compare", testdata + "drop-0.4.json", "-baseline", testdata + "baseline.json"}, true, "FAIL: mean rubric score 4.00 is 0.40 below the baseline 4.40"},
		{"drop 0.2", []string{"-compare", testdata + "drop-0.2.json", "-baseline", testdata + "baseline.json"}, false, "(baseline 1111111111111111111111111111111111111111): pass"},
		{"no baseline file", []string{"-compare", testdata + "drop-0.4.json", "-baseline", testdata + "none.json"}, false, "(no baseline): pass"},
		{"per-run lines", []string{"-compare", testdata + "passk-fall.json"}, true, "stub/csv-export on "},
	} {
		out, err := eval(t, tc.args...)
		if (err != nil) != tc.fail || !strings.Contains(out, tc.want) {
			t.Errorf("%s: %v, want failure %v with %q:\n%s", tc.name, err, tc.fail, tc.want, out)
		}
	}
	if _, err := eval(t, "-compare", testdata+"none.json"); err == nil || !strings.Contains(err.Error(), "no such results file") {
		t.Errorf("-compare of a missing file: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("{"), 0o644)
	if _, err := eval(t, "-compare", testdata+"drop-0.2.json", "-baseline", bad); err == nil || !strings.Contains(err.Error(), "bad.json") {
		t.Errorf("unreadable baseline: %v", err)
	}
}

// Usage: a mode is required, an unknown flag prints the flags, and tier 2
// needs -sha, -out, a known -group, and a valid -cases pattern.
func TestUsage(t *testing.T) {
	isolate(t, "http://127.0.0.1:1")
	out := filepath.Join(t.TempDir(), "r.json")
	for _, tc := range []struct {
		args       []string
		want, help string
	}{
		{nil, "pass -tier 1, -tier 2, or -compare <file>", ""},
		{[]string{"-bogus"}, "pass -tier 1, -tier 2, or -compare <file>", "flag provided but not defined: -bogus"},
		{[]string{"-tier", "2", "-group", "skills"}, "tier 2 needs -sha and -out", ""},
		{[]string{"-tier", "2", "-sha", "abc", "-out", out, "-group", "nope"}, `tier 2 needs -group: no tier-2 group "nope"`, ""},
		{[]string{"-tier", "2", "-sha", "abc", "-out", out, "-group", "skills", "-cases", "("}, "-cases: ", ""},
	} {
		printed, err := eval(t, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(printed, tc.help) {
			t.Errorf("%v: %v, want %q; printed:\n%s", tc.args, err, tc.want, printed)
		}
	}
	t.Setenv("AAYU_MODELS__GATEWAY__BASE_URL", "http://gateway.example/v1")
	if _, err := eval(t, "-tier", "2", "-sha", "abc", "-out", out, "-group", "skills"); err == nil || !strings.Contains(err.Error(), "[models.gateway] base_url") {
		t.Errorf("a plain-http gateway: %v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a refused tier-2 run wrote results")
	}
}

// isolate points the command's configuration at a gateway URL and away from
// the developer's own configuration and secrets.
func isolate(t *testing.T, gateway string) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "AAYU_") {
			t.Setenv(k, "")
			_ = os.Unsetenv(k)
		}
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AAYU_MODELS__GATEWAY__BASE_URL", gateway)
	t.Setenv("AAYU_MODELS__GATEWAY__SECRET", "eval-gw")
	t.Setenv("AAYU_MODELS__GATEWAY__API_STYLE", "chat")
	t.Setenv("AAYU_SECRET_EVAL_GW", "gw-key")
}

// Tier 2 through [models.gateway] from the environment, against a fake
// gateway speaking OpenAI Chat Completions: one case, k runs, the judge's
// first reply unusable twice (one errored run, which the bar allows), the
// results written with the group and baseline, and the verdict printed.
func TestTier2FakeGateway(t *testing.T) {
	item, err := os.ReadFile(filepath.Join(repoRoot, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := map[string]int{}
	auth := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		calls[req.Model]++
		n := calls[req.Model]
		auth[r.URL.Path+" "+r.Header.Get("Authorization")] = true
		mu.Unlock()
		text := string(item)
		if req.Model == "gpt-6.1-sol" {
			text = `{"score": 5, "reason": "covers the request"}`
			if n <= 2 {
				text = "I think it is good."
			}
		}
		finish := "stop"
		if req.Model == "gpt-6.1-sol" && n <= 2 {
			finish = ""
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": req.Model,
			"choices": []any{map[string]any{"message": map[string]any{"content": text}, "finish_reason": finish}},
			"usage":   map[string]any{"prompt_tokens": 1000, "completion_tokens": 200}})
	}))
	defer srv.Close()
	isolate(t, srv.URL)

	res := filepath.Join(t.TempDir(), "results", "abc-skills.json")
	out, err := eval(t, "-tier", "2", "-group", "skills", "-cases", "^stub/csv-export$", "-sha", "abc", "-ref", "dev", "-out", res, "-baseline", testdata+"baseline.json")
	if err != nil {
		t.Fatalf("tier 2: %v\n%s", err, out)
	}
	var r evals.Results
	b, _ := os.ReadFile(res)
	if err := json.Unmarshal(b, &r); err != nil || r.Verdict != evals.Pass || r.Group != "skills" || r.Ref != "dev" || r.Baseline != "1111111111111111111111111111111111111111" ||
		len(r.Cases) != 1 || r.Cases[0].Case != "stub/csv-export" || len(r.Cases[0].Runs) != 5 || r.Cases[0].Errors != 1 || r.CostUSD <= 0 {
		t.Fatalf("results: %v\n%s", err, b)
	}
	if calls["gpt-6.1-sol"] != 6 || calls[r.Cases[0].Model] != 5 || !auth["/chat/completions Bearer gw-key"] || len(auth) != 1 {
		t.Fatalf("gateway calls %v, auth %v", calls, auth)
	}
	for _, want := range []string{"wrote " + res, "tier 2 abc: pass^5 1.00, mean 5.00 (baseline 1111111111111111111111111111111111111111): pass",
		"run 1 error score 0:", "judge gpt-6.1-sol stop=- out=200/2000", `reply: "I think it is good."`, "run 2 pass score 5: skill "} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	// A judge from the skill model's family is refused before any call.
	t.Setenv("AAYU_MODELS__SPEC__CLAUDE__MODEL", "gpt-6.1-sol")
	before := calls["gpt-6.1-sol"]
	if _, err := eval(t, "-tier", "2", "-group", "skills", "-cases", "^stub/csv-export$", "-sha", "abc", "-out", res); err == nil || !strings.Contains(err.Error(), "judge gpt-6.1-sol") || calls["gpt-6.1-sol"] != before {
		t.Fatalf("same-family judge: %v after %d calls", err, calls["gpt-6.1-sol"]-before)
	}
}

// A failing result of an advisory-gate group is reported and exits 0; the
// same result in a gating group fails (D-7.3h).
func TestCompareAdvisoryGate(t *testing.T) {
	dir := t.TempDir()
	for _, g := range []string{"brief-plain", "brief"} {
		r := evals.Results{SHA: "662cd99", Group: g, K: 5, Mean: 3.1, Cases: []evals.CaseResult{{Case: "brief/eli5-explain", Mean: 3.2}, {Case: "brief/ste-explain", Mean: 3.0}}}
		b, _ := json.Marshal(r)
		f := filepath.Join(dir, g+".json")
		_ = os.WriteFile(f, b, 0o644)
		out, err := eval(t, "-compare", f)
		switch g {
		case "brief-plain":
			if err != nil || !strings.Contains(out, ": fail") || !strings.Contains(out, "group brief-plain is advisory") {
				t.Errorf("advisory group: %v\n%s", err, out)
			}
		default:
			if err == nil || !strings.Contains(out, "mean rubric score 3.20 is below 3.50") {
				t.Errorf("gating group: %v\n%s", err, out)
			}
		}
	}
}
