package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
