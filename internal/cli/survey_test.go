package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
)

// WP5.9: `aayu survey <path>` indexes a checkout with no model and writes
// index.json under --out; `--module` prints one module's pack from that index
// with the repository's text inside an untrusted block.
func TestSurveyIndexAndPack(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "survey", "testdata", "legacy"))); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "survey")
	opts := Options{Dir: dir, GitHubGap: -1}
	code, stdout, stderr := run(t, nil, opts, "survey", dir, "--out", out, "--json")
	var res SurveyResult
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil {
		t.Fatalf("survey: %d %v %s", code, err, stderr)
	}
	if res.Entries["main"] != 2 || res.Markers != 4 || len(res.Modules) == 0 || res.Index != filepath.Join(out, "index.json") {
		t.Fatalf("result: entries %v, markers %d, modules %d, index %s", res.Entries, res.Markers, len(res.Modules), res.Index)
	}
	ix, err := survey.Load(res.Index)
	if err != nil || ix.Resolve("sym:src/engine.cpp#Engine::eval") != nil {
		t.Fatalf("reload: %v", err)
	}
	code, stdout, _ = run(t, nil, opts, "survey", dir, "--out", out, "--module", "mod:src", "--plain")
	if code != 0 || !strings.Contains(stdout, "<untrusted source=\"repo\">") || !strings.Contains(stdout, "marker marker:src/engine.cpp:5: known-issue: Division by zero crashes") {
		t.Fatalf("pack: %d\n%s", code, stdout)
	}
	if code, _, stderr = run(t, nil, opts, "survey", dir, "--out", out, "--module", "mod:nope"); code != ExitUsage || !strings.Contains(stderr, "mod:src") {
		t.Fatalf("unknown module: %d %s", code, stderr)
	}

	// --draft checks the analysis against the index and renders SURVEY.md; a
	// product claim marked observed blocks (exit 4) and the report lists it.
	draft := filepath.Join("..", "survey", "testdata", "draft.json")
	code, stdout, stderr = run(t, nil, opts, "survey", dir, "--out", out, "--draft", draft, "--json")
	var dr SurveyDraftResult
	if err := json.Unmarshal([]byte(stdout), &dr); code != 0 || err != nil || !dr.Pass {
		t.Fatalf("draft: %d %v %s %s", code, err, stdout, stderr)
	}
	if md, err := os.ReadFile(dr.Survey); err != nil || !strings.Contains(string(md), "## Capability map") {
		t.Fatalf("SURVEY.md: %v", err)
	}
	b, _ := os.ReadFile(draft)
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(strings.Replace(string(b), `"basis": "stated", "cites": ["doc:README.md"]`, `"basis": "observed", "cites": ["doc:README.md"]`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, _ = run(t, nil, opts, "survey", dir, "--out", out, "--draft", bad, "--plain"); code != ExitBlocked || !strings.Contains(stdout, "a product claim cannot be observed") {
		t.Fatalf("bad draft: %d %s", code, stdout)
	}
}

// WP5.9: `survey --backlog` renders backlog.md and its digest and files
// nothing; --create files only the reviewed digest, at most --limit issues a
// run, each with the survey marker and label under its parent; a re-run files
// only what is missing and never edits an issue.
func TestSurveyBacklogCreate(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "survey", "testdata", "legacy"))); err != nil {
		t.Fatal(err)
	}
	g := fake.New(t, "o/r", "User")
	out := filepath.Join(t.TempDir(), "survey")
	opts := Options{Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	draft := filepath.Join("..", "survey", "testdata", "draft.json")
	backlog := filepath.Join("..", "survey", "testdata", "backlog.json")
	if code, _, stderr := run(t, env, opts, "survey", dir, "--out", out); code != 0 {
		t.Fatalf("index: %d %s", code, stderr)
	}
	args := []string{"survey", dir, "--out", out, "--repo", "o/r", "--backlog", backlog, "--draft", draft, "--json"}
	code, stdout, stderr := run(t, env, opts, args...)
	var res SurveyBacklogResult
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || !res.Pass || len(res.Digest) != 16 {
		t.Fatalf("check: %d %v %s %s", code, err, stdout, stderr)
	}
	if w := g.Writes(); len(w) != 0 {
		t.Fatalf("a check wrote to GitHub: %v", w)
	}
	if code, _, stderr = run(t, env, opts, append(args, "--create", "--reviewed", "0123456789abcdef")...); code != ExitBlocked || !strings.Contains(stderr, "files only the backlog the person reviewed") {
		t.Fatalf("wrong digest: %d %s", code, stderr)
	}
	create := append(args, "--create", "--reviewed", res.Digest, "--limit", "4")
	code, stdout, stderr = run(t, env, opts, create...)
	res = SurveyBacklogResult{}
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Filed) != 4 || len(res.Waiting) != 3 {
		t.Fatalf("first create: %d %v filed %v waiting %v %s", code, err, res.Filed, res.Waiting, stderr)
	}
	g.Mu.Lock()
	story := g.Issues[res.Filed["capture-golden"]]
	g.Mu.Unlock()
	if !strings.HasPrefix(story.Body, "<!-- aayudha:survey product=calc key=capture-golden -->") || !slices.Contains(g.LabelNames(story.Number), "survey:calc") || !slices.Contains(g.LabelNames(story.Number), "type:story") {
		t.Fatalf("filed story: %s %v", story.Body, g.LabelNames(story.Number))
	}
	subs := 0
	for _, w := range g.Writes() {
		if strings.HasSuffix(w.Path, "/sub_issues") {
			subs++
		}
	}
	if subs != 3 {
		t.Errorf("sub-issue links = %d, want 3 (initiative, feature, story)", subs)
	}
	// The second run files the three that waited, with their links' numbers, and edits nothing.
	g.Reset()
	code, stdout, _ = run(t, env, opts, create...)
	res = SurveyBacklogResult{}
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Filed) != 3 || len(res.Existing) != 4 || len(res.Waiting) != 0 {
		t.Fatalf("second create: %d %v %+v", code, err, res)
	}
	for _, w := range g.Writes() {
		if w.Method == "PATCH" {
			t.Errorf("a re-run edited an issue: %v", w)
		}
	}
	g.Mu.Lock()
	port := g.Issues[res.Filed["port-eval"]]
	g.Mu.Unlock()
	if !strings.Contains(port.Body, fmt.Sprintf("depends-on #%d", res.Existing["capture-golden"])) {
		t.Errorf("port-eval body lacks its dependency:\n%s", port.Body)
	}
	// The do-not-carry Decision is filed after the Stories it constrains, with their numbers.
	g.Mu.Lock()
	dec := g.Issues[res.Filed["no-env-debug"]]
	g.Mu.Unlock()
	if !strings.Contains(dec.Body, fmt.Sprintf("constrains #%d", res.Existing["capture-golden"])) || !strings.Contains(dec.Body, fmt.Sprintf("constrains #%d", res.Filed["port-eval"])) ||
		!slices.Contains(g.LabelNames(dec.Number), "type:decision") {
		t.Errorf("decision body or labels:\n%s %v", dec.Body, g.LabelNames(dec.Number))
	}
	// A third run finds every key by its marker and files nothing.
	g.Reset()
	if code, stdout, _ = run(t, env, opts, create...); code != 0 || len(g.Writes()) != 0 {
		t.Fatalf("third create wrote: %d %v %s", code, g.Writes(), stdout)
	}
}

// WP5.9: `survey --golden` captures a legacy program's outputs in a scrubbed
// environment; `--parity` replays the same cases against a new program and
// exits 3 only on a stream the case compares.
func TestSurveyGoldenParity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GH_TOKEN", "sekrit-token")
	script := func(name, warn string) string {
		p := filepath.Join(dir, name)
		body := "#!/bin/sh\nread x\necho \"out:$1:$x:${GH_TOKEN}${MODE}\"\necho " + warn + " >&2\nexit $2\n"
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	legacy, same, changed := script("legacy.sh", "warn"), script("same.sh", "warn"), script("new.sh", "WARN")
	if err := os.WriteFile(filepath.Join(dir, "in.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := filepath.Join(dir, "cases.json")
	if err := os.WriteFile(cases, []byte(`{"cases": [
  {"id": "c1", "set": "G1", "argv": ["{bin}", "a", "0"], "stdin": "in.txt"},
  {"id": "c2", "set": "G1", "argv": ["{bin}", "b", "3"], "stdin": "in.txt", "compare": ["stdout", "exit"], "env": ["MODE=strict"]}
]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "survey")
	opts := Options{Dir: dir, GitHubGap: -1}
	base := []string{"survey", dir, "--out", out}

	// Without --run the cases are only checked; nothing runs.
	if code, stdout, stderr := run(t, nil, opts, append(base, "--golden", cases, "--plain")...); code != 0 || !strings.Contains(stdout, "2 cases (G1 2)") || strings.Contains(stdout, "ran ") {
		t.Fatalf("check only: %d %s %s", code, stdout, stderr)
	}
	if code, _, _ := run(t, nil, opts, append(base, "--golden", cases, "--run")...); code != ExitUsage {
		t.Fatalf("--run without --bin: %d", code)
	}
	code, stdout, stderr := run(t, nil, opts, append(base, "--golden", cases, "--bin", legacy, "--run", "--json")...)
	var res SurveyGoldenResult
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Golden.Cases) != 2 || res.Golden.Cases[1].Exit != 3 {
		t.Fatalf("capture: %d %v %s %s", code, err, stdout, stderr)
	}
	if b, err := os.ReadFile(filepath.Join(out, "golden", "cases", "c1.stdout")); err != nil || string(b) != "out:a:hello:\n" {
		t.Fatalf("golden c1.stdout = %q, %v (the token must not reach the program)", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(out, "golden", "cases", "c2.stdout")); err != nil || string(b) != "out:b:hello:strict\n" {
		t.Fatalf("golden c2.stdout = %q, %v (the case's env reaches its program, and only its own)", b, err)
	}

	if code, _, stderr := run(t, nil, opts, append(base, "--parity", cases, "--bin", same, "--run")...); code != 0 {
		t.Fatalf("parity with the same behaviour: %d %s", code, stderr)
	}
	code, stdout, _ = run(t, nil, opts, append(base, "--parity", cases, "--bin", changed, "--run", "--json")...)
	res = SurveyGoldenResult{}
	if err := json.NewDecoder(strings.NewReader(stdout)).Decode(&res); code != ExitFloor || err != nil || len(res.Diffs) != 1 || res.Diffs[0].ID != "c1" || res.Diffs[0].Stream != "stderr" {
		t.Fatalf("parity with changed stderr: %d %v %+v", code, err, res.Diffs)
	}

	// --shadow runs both builds on the same inputs, with no golden files.
	if code, _, stderr := run(t, nil, opts, append(base, "--golden", cases, "--bin", legacy, "--shadow", same, "--run")...); code != 0 {
		t.Fatalf("shadow with the same behaviour: %d %s", code, stderr)
	}
	code, stdout, _ = run(t, nil, opts, append(base, "--golden", cases, "--bin", legacy, "--shadow", changed, "--run", "--json")...)
	res = SurveyGoldenResult{}
	if err := json.NewDecoder(strings.NewReader(stdout)).Decode(&res); code != ExitFloor || err != nil || res.Mode != "shadow" || len(res.Diffs) != 1 || res.Diffs[0].ID != "c1" {
		t.Fatalf("shadow with changed stderr: %d %v %+v", code, err, res.Diffs)
	}
	if code, _, _ := run(t, nil, opts, append(base, "--parity", cases, "--bin", same, "--probe", "--run")...); code != ExitUsage {
		t.Fatalf("--probe outside --golden: %d", code)
	}

	// --compat scores the legacy suite on the new build against the legacy run.
	suite := filepath.Join(dir, "suite.json")
	if err := os.WriteFile(suite, []byte(`{"cases": [
  {"id": "t1", "set": "S1", "area": "parse", "argv": ["{bin}", "t1", "0"], "stdin": "in.txt"},
  {"id": "t2", "set": "S1", "area": "parse", "argv": ["{bin}", "t2", "1"], "stdin": "in.txt"},
  {"id": "t3", "set": "S1", "area": "print", "argv": ["{bin}", "t3", "0"], "stdin": "in.txt"},
  {"id": "t4", "set": "S1", "area": "print", "argv": ["{bin}", "t4", "1"], "stdin": "in.txt", "unfixable": "intercepts glibc with LD_PRELOAD"}
]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(t, nil, opts, append(base, "--golden", suite, "--bin", legacy, "--run")...); code != 0 {
		t.Fatalf("legacy suite run: %d %s", code, stderr)
	}
	port := filepath.Join(dir, "port.sh") // fails t1 (a regression), passes t2 (which the legacy build fails)
	if err := os.WriteFile(port, []byte("#!/bin/sh\ncase $1 in t1) exit 1;; *) exit 0;; esac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = run(t, nil, opts, append(base, "--compat", suite, "--bin", port, "--run", "--json")...)
	res = SurveyGoldenResult{}
	if err := json.NewDecoder(strings.NewReader(stdout)).Decode(&res); code != ExitFloor || err != nil || res.Compat == nil {
		t.Fatalf("compat: %d %v %s", code, err, stdout)
	}
	if c := res.Compat; c.LegacyPass != 2 || c.Pass != 1 || c.Regressions != 1 || len(c.Areas) != 2 || c.Areas[0].Regressions[0] != "t1" || c.Areas[0].Fixed[0] != "t2" || c.Areas[1].Unfixable != 1 {
		t.Errorf("compat score: %+v", c)
	}
	if b, err := os.ReadFile(filepath.Join(out, "golden", "suite", "compat-history.jsonl")); err != nil || strings.Count(string(b), "\n") != 1 {
		t.Errorf("history: %q %v", b, err)
	}
	if code, _, _ := run(t, nil, opts, append(base, "--compat", cases)...); code != ExitBlocked {
		t.Errorf("a compat run of cases with no suite or area must be blocked: %d", code)
	}
}
