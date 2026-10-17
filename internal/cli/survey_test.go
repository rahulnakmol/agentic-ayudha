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
// with the repository's text inside an untrusted block; a later part's flag
// is refused as n/a.
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
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Filed) != 4 || len(res.Waiting) != 2 {
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
	// The second run files the two that waited, with the dependency's number, and edits nothing.
	g.Reset()
	code, stdout, _ = run(t, env, opts, create...)
	res = SurveyBacklogResult{}
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Filed) != 2 || len(res.Existing) != 4 || len(res.Waiting) != 0 {
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
	// A third run finds every key by its marker and files nothing.
	g.Reset()
	if code, stdout, _ = run(t, env, opts, create...); code != 0 || len(g.Writes()) != 0 {
		t.Fatalf("third create wrote: %d %v %s", code, g.Writes(), stdout)
	}
}
