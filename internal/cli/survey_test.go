package cli

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
	"github.com/rahulnakmol/agentic-ayudha/internal/testbin"
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

// A survey written in sections: each section in a --draft file is checked and
// saved under <out>/sections at once, a later run resumes the saved ones, a
// failing later section never loses them, and with every section present the
// merged survey.json and SURVEY.md are written. A section saved against
// another index commit is stale and must be written again.
func TestSurveyDraftSections(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "survey", "testdata", "legacy"))); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "survey")
	opts := Options{Dir: dir, GitHubGap: -1}
	if code, _, stderr := run(t, nil, opts, "survey", dir, "--out", out); code != 0 {
		t.Fatalf("index: %d %s", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join("..", "survey", "testdata", "draft.json"))
	if err != nil {
		t.Fatal(err)
	}
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(b, &whole); err != nil {
		t.Fatal(err)
	}
	first := []string{"product", "request", "engineering", "traps", "capabilities"}
	write := func(name string, keep func(k string) bool, edit func(string) string) string {
		part := map[string]json.RawMessage{}
		for k, v := range whole {
			if keep(k) {
				part[k] = v
			}
		}
		b, _ := json.Marshal(part)
		f := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(f, []byte(edit(string(b))), 0o644); err != nil {
			t.Fatal(err)
		}
		return f
	}
	same := func(s string) string { return s }
	a := write("a.json", func(k string) bool { return slices.Contains(first, k) }, same)
	rest := func(k string) bool { return !slices.Contains(first, k) }
	draft := func(args ...string) (int, SurveyDraftResult) {
		t.Helper()
		code, stdout, stderr := run(t, nil, opts, append([]string{"survey", dir, "--out", out, "--json"}, args...)...)
		var r SurveyDraftResult // the result, then the error when it blocks
		if err := json.NewDecoder(strings.NewReader(stdout)).Decode(&r); err != nil {
			t.Fatalf("%v: %d %v %s %s", args, code, err, stdout, stderr)
		}
		return code, r
	}
	saved := func(name string) survey.SavedSection {
		t.Helper()
		var s survey.SavedSection
		b, err := os.ReadFile(filepath.Join(out, "sections", name+".json"))
		if err != nil || json.Unmarshal(b, &s) != nil {
			t.Fatalf("section %s: %v", name, err)
		}
		return s
	}

	code, r := draft("--draft", a)
	if code != 0 || !slices.Equal(r.Saved, []string{"engineering", "capabilities"}) || !slices.Equal(r.Missing, []string{"product", "modernisation", "questions"}) || r.Merged != "" {
		t.Fatalf("first sections: %d %+v", code, r)
	}
	// A later section that fails blocks, is saved with its findings, and the
	// earlier sections stay saved and resumed.
	bad := write("bad.json", rest, func(s string) string {
		return strings.Replace(s, `"basis":"stated","cites":["doc:README.md"]`, `"basis":"observed","cites":["doc:README.md"]`, 1)
	})
	code, r = draft("--draft", bad)
	if p := saved("product"); code != ExitBlocked || !slices.Equal(r.Resumed, []string{"engineering", "capabilities"}) || p.Pass || len(p.Problems) == 0 || !strings.Contains(p.Problems[0], "cannot be observed") {
		t.Fatalf("failing section: %d %+v %+v", code, r, p)
	}
	if e := saved("engineering"); !e.Pass || len(e.Content) == 0 {
		t.Fatalf("engineering lost: %+v", e)
	}
	code, r = draft("--draft", write("b.json", rest, same))
	if code != 0 || !r.Pass || len(r.Missing) != 0 || r.Merged == "" {
		t.Fatalf("complete: %d %+v", code, r)
	}
	mb, err := os.ReadFile(r.Merged)
	if err != nil {
		t.Fatal(err)
	}
	if merged, err := survey.ParseDraft(mb); err != nil || len(merged.Engineering) == 0 || len(merged.Capabilities) == 0 || merged.Modernisation == nil {
		t.Fatalf("survey.json: %v %s", err, mb)
	}
	if md, err := os.ReadFile(r.Survey); err != nil || !strings.Contains(string(md), "## Capability map") {
		t.Fatalf("SURVEY.md: %v", err)
	}
	// --sections alone resumes every saved section and passes again.
	if code, r = draft("--sections"); code != 0 || !r.Pass || len(r.Resumed) != 5 || len(r.Saved) != 0 {
		t.Fatalf("resume: %d %+v", code, r)
	}
	// A section saved against another index commit is stale, not resumed.
	p := saved("product")
	p.Commit = "0123456789abcdef"
	if err := survey.SaveSection(filepath.Join(out, "sections"), p); err != nil {
		t.Fatal(err)
	}
	if code, r = draft("--sections"); code != 0 || !slices.Equal(r.Stale, []string{"product"}) || !slices.Equal(r.Missing, []string{"product"}) || r.Merged != "" {
		t.Fatalf("stale: %d %+v", code, r)
	}
	// The merged survey of the earlier run is removed: a backlog is never
	// drafted from sections that no longer hold.
	for _, f := range []string{"survey.json", "SURVEY.md"} {
		if _, err := os.Stat(filepath.Join(out, f)); !os.IsNotExist(err) {
			t.Fatalf("%s kept after a section went stale: %v", f, err)
		}
	}
	// So is every section once the checkout's content changes, at the same
	// commit (this checkout has no git): the claims were checked against
	// other code.
	src := filepath.Join(dir, "src", "util.cpp")
	b, err = os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, append(b, []byte("// edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(t, nil, opts, "survey", dir, "--out", out); code != 0 {
		t.Fatalf("re-index: %d %s", code, stderr)
	}
	if code, r = draft("--sections"); code != 0 || len(r.Stale) != 5 || len(r.Resumed) != 0 {
		t.Fatalf("content changed: %d %+v", code, r)
	}

	// A survey that describes the product without a modernisation plan
	// submits the section empty, and completes.
	out = filepath.Join(t.TempDir(), "survey")
	if code, _, stderr := run(t, nil, opts, "survey", dir, "--out", out); code != 0 {
		t.Fatalf("index: %d %s", code, stderr)
	}
	describe := write("describe.json", func(k string) bool {
		return k != "modernisation" && k != "compatibility" && k != "do_not_carry"
	}, func(s string) string {
		return strings.Replace(strings.Replace(s, "{", `{"modernisation":null,`, 1), `"affects":["N1","E4"]`, `"affects":["E4"]`, 1)
	})
	if code, r = draft("--draft", describe); code != 0 || len(r.Missing) != 0 || r.Merged == "" {
		t.Fatalf("no modernisation: %d %+v", code, r)
	}
}

// survey --docs indexes manuals and notes from outside the checkout: each is
// copied under <out>/manuals and cited by line as manual:<name>:<line>, so a
// claim a manual states is traced to it; a line past its end is not a cite.
func TestSurveyDocs(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "survey", "testdata", "legacy"))); err != nil {
		t.Fatal(err)
	}
	docs := t.TempDir()
	if err := os.WriteFile(filepath.Join(docs, "operator-notes.md"), []byte("# Operator notes\n\nThe night shift runs calc on each batch total.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "survey")
	opts := Options{Dir: dir, GitHubGap: -1}
	if code, _, stderr := run(t, nil, opts, "survey", dir, "--out", out, "--docs", docs); code != 0 {
		t.Fatalf("index: %d %s", code, stderr)
	}
	if b, err := os.ReadFile(filepath.Join(out, "manuals", "operator-notes.md")); err != nil || !strings.Contains(string(b), "night shift") {
		t.Fatalf("manual copy: %v", err)
	}
	b, err := os.ReadFile(filepath.Join("..", "survey", "testdata", "draft.json"))
	if err != nil {
		t.Fatal(err)
	}
	cite := func(at string) string {
		f := filepath.Join(t.TempDir(), "draft.json")
		claim := `"claim": "The operator notes say the night shift runs calc on each batch total.", "basis": "stated", "cites": ["` + at + `"]`
		if err := os.WriteFile(f, []byte(strings.Replace(string(b), `"claim": "The README calls it a calculator.", "basis": "stated", "cites": ["doc:README.md"]`, claim, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		return f
	}
	code, stdout, stderr := run(t, nil, opts, "survey", dir, "--out", out, "--draft", cite("manual:operator-notes.md:3"), "--plain")
	if code != 0 {
		t.Fatalf("manual cite: %d %s %s", code, stdout, stderr)
	}
	if md, err := os.ReadFile(filepath.Join(out, "SURVEY.md")); err != nil || !strings.Contains(string(md), "manual:operator-notes.md:3") || !strings.Contains(string(md), "fact, stated") {
		t.Fatalf("SURVEY.md: %v\n%s", err, md)
	}
	if code, stdout, _ = run(t, nil, opts, "survey", dir, "--out", out, "--draft", cite("manual:operator-notes.md:9"), "--plain"); code != ExitBlocked || !strings.Contains(stdout, "manual:operator-notes.md:9") {
		t.Fatalf("cite past the end: %d %s", code, stdout)
	}
	if code, stdout, _ = run(t, nil, opts, "survey", dir, "--out", out, "--draft", cite("doc:manual:operator-notes.md"), "--plain"); code != ExitBlocked || !strings.Contains(stdout, "cite the lines of manual operator-notes.md") {
		t.Fatalf("the whole manual cited: %d %s", code, stdout)
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

// D-5.9n: survey filing against GitHub runs on the permanent private
// repository rahulnakmol/aayu-survey-scratch with AAYU_SURVEY_SCRATCH_TOKEN, a
// fine-grained token limited to it (Issues read and write), never the admin
// token. Each run files under a product of its own, so it never meets an
// earlier run's issues; it closes what it filed and deletes its label after.
func TestSurveyBacklogCreateLive(t *testing.T) {
	repo := cmp.Or(os.Getenv("AAYU_SURVEY_SCRATCH_REPO"), "rahulnakmol/aayu-survey-scratch")
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("live test: set AAYU_LIVE=1")
	}
	tok := os.Getenv("AAYU_SURVEY_SCRATCH_TOKEN")
	if tok == "" {
		t.Skip("AAYU_SURVEY_SCRATCH_TOKEN is not set: survey filing runs only with the fine-grained token limited to " + repo +
			" (Issues read and write), which the founder stores in the live-tests environment")
	}
	dir, state := t.TempDir(), t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "survey", "testdata", "legacy"))); err != nil {
		t.Fatal(err)
	}
	product := fmt.Sprintf("calc-live-%d", time.Now().UnixNano())
	files := map[string]string{}
	for _, f := range []string{"draft.json", "backlog.json"} {
		b, err := os.ReadFile(filepath.Join("..", "survey", "testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		files[f] = filepath.Join(state, f)
		if err := os.WriteFile(files[f], []byte(strings.Replace(string(b), `"product": "calc"`, `"product": "`+product+`"`, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { closeSurveyLive(t, tok, repo, survey.Label(product)) })
	out := filepath.Join(state, "survey")
	opts := Options{Dir: dir}
	env := map[string]string{"AAYU_GITHUB_TOKEN": tok, "XDG_STATE_HOME": filepath.Join(state, "xdg")}
	if code, _, stderr := run(t, env, opts, "survey", dir, "--out", out); code != 0 {
		t.Fatalf("index: %d %s", code, stderr)
	}
	args := []string{"survey", dir, "--out", out, "--repo", repo, "--backlog", files["backlog.json"], "--draft", files["draft.json"], "--json"}
	var res SurveyBacklogResult
	code, stdout, stderr := run(t, env, opts, args...)
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || !res.Pass {
		t.Fatalf("check: %d %v %s %s", code, err, stdout, stderr)
	}
	create := append(args, "--create", "--reviewed", res.Digest, "--limit", "4")
	for i, want := range []struct{ filed, existing int }{{4, 0}, {3, 4}, {0, 7}} {
		res = SurveyBacklogResult{}
		code, stdout, stderr = run(t, env, opts, create...)
		if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Filed) != want.filed || len(res.Existing) != want.existing {
			t.Fatalf("create %d: %d %v filed %v existing %v %s", i+1, code, err, res.Filed, res.Existing, stderr)
		}
	}
}

// closeSurveyLive closes the issues a live run filed and deletes their
// label. Test scaffolding, not a verb write, so it calls the API directly.
func closeSurveyLive(t *testing.T, tok, repo, label string) {
	call := func(method, path string, body any) (int, []byte) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, "https://api.github.com/repos/"+repo+path, rd)
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, nil
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	_, b := call("GET", "/issues?state=open&per_page=100&labels="+url.QueryEscape(label), nil)
	var issues []struct {
		Number int `json:"number"`
	}
	_ = json.Unmarshal(b, &issues)
	for _, is := range issues {
		if code, _ := call("PATCH", fmt.Sprintf("/issues/%d", is.Number), map[string]string{"state": "closed", "state_reason": "not_planned"}); code != http.StatusOK {
			t.Errorf("close %s#%d: HTTP %d", repo, is.Number, code)
		}
	}
	if code, _ := call("DELETE", "/labels/"+url.PathEscape(label), nil); code != http.StatusNoContent {
		t.Errorf("delete label %s: HTTP %d", label, code)
	}
}

// WP5.9: `survey --golden` captures a legacy program's outputs in a scrubbed
// environment; `--parity` replays the same cases against a new program and
// exits 3 only on a stream the case compares.
func TestSurveyGoldenParity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GH_TOKEN", "sekrit-token")
	// The programs are Go fixtures, so survey runs them by path as it runs a legacy build, on every OS.
	script := func(name, warn string) string {
		return testbin.Build(t, dir, name, `package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	x, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Printf("out:%s:%s:%s%s\n", os.Args[1], strings.TrimRight(x, "\r\n"), os.Getenv("GH_TOKEN"), os.Getenv("MODE"))
	fmt.Fprintln(os.Stderr, "`+warn+`")
	code, _ := strconv.Atoi(os.Args[2])
	os.Exit(code)
}
`)
	}
	legacy, same, changed := script("legacy", "warn"), script("same", "warn"), script("new", "WARN")
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
	// fails t1 (a regression), passes t2 (which the legacy build fails)
	port := testbin.Build(t, dir, "port", "package main\n\nimport \"os\"\n\nfunc main() {\n\tif os.Args[1] == \"t1\" {\n\t\tos.Exit(1)\n\t}\n}\n")
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
