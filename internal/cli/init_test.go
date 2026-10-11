package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

const ciYML = "name: ci\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make test # the product's own CI\n"

// checkout makes a clone-like working tree of o/r with the product's own CI.
func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "--quiet", "-b", "dev"}, {"remote", "add", "origin", "https://github.com/o/r.git"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte(ciYML), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type initReport struct {
	Complete bool              `json:"complete"`
	Changes  []initrepo.Change `json:"changes"`
	Counts   map[string]int    `json:"counts"`
	GitHub   struct {
		REST int `json:"rest"`
	} `json:"github"`
}

func runInitCmd(t *testing.T, g *fake.GitHub, dir string, args ...string) (int, initReport, string) {
	t.Helper()
	code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}, append(args, "--json")...)
	var rep initReport
	if out != "" {
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&rep); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	return code, rep, errOut
}

// snapshot reads every file of the checkout outside .git.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "telemetry") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(dir, p)
			out[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	return out
}

func treeFiles(g *fake.GitHub, ref string) []string {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	c := g.Commits[g.Refs[ref]]
	var names []string
	for _, e := range g.Trees[c["tree"].(string)] {
		names = append(names, e["path"].(string))
	}
	return names
}

// WP2.5 accept: run twice → zero changes; --dry-run writes nothing; a public
// repository ends with no .lfsconfig on ledger and proof push --media exits 4;
// the product's existing ci.yml stays byte-identical.
func TestInitTwiceDryRunPublicCoexistence(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Labels = []*fake.Label{{ID: 7, Name: "bug", Color: "d73a4a"}, {ID: 8, Name: "status:ready", Color: "ffffff"}}
	dir := checkout(t)

	before := snapshot(t, dir)
	code, rep, errOut := runInitCmd(t, g, dir, "init", "--dry-run")
	if code != 0 {
		t.Fatalf("dry run: exit %d: %s", code, errOut)
	}
	if w := g.Writes(); len(w) != 0 || rep.Counts["planned"] == 0 {
		t.Fatalf("dry run sent %d writes, planned %d", len(w), rep.Counts["planned"])
	}
	if after := snapshot(t, dir); len(after) != len(before) {
		t.Fatalf("dry run wrote files: %d -> %d", len(before), len(after))
	}

	code, rep, errOut = runInitCmd(t, g, dir, "init", "--yes")
	if code != 0 || !rep.Complete {
		t.Fatalf("first run: exit %d complete %t: %s", code, rep.Complete, errOut)
	}
	nLabels := len(workitems.Labels())
	if rep.Counts["created"] != nLabels-1+2 || rep.Counts["adopted"] != 1 { // status:ready adopted; ledger and ruleset created
		t.Fatalf("first run counts %v, want %d labels created, 1 adopted, plus ledger and ruleset", rep.Counts, nLabels-1)
	}
	first := snapshot(t, dir)
	if first[".github/workflows/ci.yml"] != ciYML {
		t.Fatalf("ci.yml changed:\n%s", first[".github/workflows/ci.yml"])
	}
	for _, typ := range workitems.Types {
		if first[initrepo.FormPath(typ)] != string(workitems.Form(typ, workitems.FormOptions{})) {
			t.Errorf("form %s missing or wrong", typ.ID)
		}
	}
	if !strings.Contains(first[".aayudha/config.toml"], `media = "" # required: private bucket`) {
		t.Errorf("public repository config:\n%s", first[".aayudha/config.toml"])
	}
	if !strings.Contains(first["AGENTS.md"], "<!-- BEGIN aayudha:work-items -->") {
		t.Errorf("AGENTS.md:\n%s", first["AGENTS.md"])
	}
	if files := treeFiles(g, "heads/ledger"); slices.Contains(files, ".lfsconfig") || slices.Contains(files, ".gitattributes") {
		t.Errorf("public ledger has LFS config: %v", files)
	}

	g.Reset()
	code, rep, errOut = runInitCmd(t, g, dir, "init", "--yes")
	if code != 0 || len(g.Writes()) != 0 || rep.Counts["created"]+rep.Counts["written"]+rep.Counts["planned"] != 0 {
		t.Fatalf("second run: exit %d, %d writes, counts %v: %s", code, len(g.Writes()), rep.Counts, errOut)
	}
	if second := snapshot(t, dir); !mapsEqual(first, second) {
		t.Fatal("second run changed files")
	}

	if code, _, errOut := run(t, nil, Options{Dir: dir}, "proof", "push", "--media"); code != ExitBlocked || !strings.Contains(errOut, "V3") {
		t.Errorf("proof push --media on a public repo: exit %d %s", code, errOut)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// WP2.5 accept: a partial run is resumed from state without duplicates.
func TestInitResume(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Private = true
	g.Fail, g.FailAfter = "POST /repos/o/r/labels", 5
	dir := checkout(t)
	if code, _, _ := runInitCmd(t, g, dir, "init", "--yes"); code == 0 {
		t.Fatal("injected failure did not stop init")
	}
	st, err := initrepo.LoadState(dir)
	if err != nil || st == nil || st.Complete || len(st.Objects) != 5 || !st.IsDone("identity") || st.IsDone("labels") {
		t.Fatalf("state after the failure: %+v, %v", st, err)
	}
	if code, _, errOut := runInitCmd(t, g, dir, "init", "--yes"); code != ExitUsage || !strings.Contains(errOut, "--resume") {
		t.Fatalf("rerun without --resume: exit %d %s", code, errOut)
	}
	code, rep, errOut := runInitCmd(t, g, dir, "init", "--yes", "--resume")
	if code != 0 || !rep.Complete {
		t.Fatalf("resume: exit %d: %s", code, errOut)
	}
	creates := 0
	for _, w := range g.Writes() {
		if w.Method == "POST" && w.Path == "/repos/o/r/labels" {
			creates++
		}
	}
	if len(g.Labels) != len(workitems.Labels()) || creates != len(workitems.Labels())+1 { // +1: the failed attempt
		t.Fatalf("%d labels, %d create requests; want %d labels and no duplicate creates", len(g.Labels), creates, len(workitems.Labels()))
	}
	// Private repository: the ledger tracks media in LFS.
	if files := treeFiles(g, "heads/ledger"); !slices.Contains(files, ".lfsconfig") || !slices.Contains(files, ".gitattributes") {
		t.Errorf("private ledger files: %v", files)
	}
}

// The ledger branch and a ruleset protecting it already exist (aayudha's own
// repository): init adopts both and adds nothing alongside; uninit removes
// only what init created.
func TestInitAdoptsLedgerAndUninit(t *testing.T) {
	g := fake.New(t, "o/r", "Organization")
	g.Refs["heads/ledger"] = "abc123"
	g.Rulesets = []map[string]any{{"id": 99, "name": "ledger-writers", "target": "branch", "enforcement": "active",
		"conditions": map[string]any{"ref_name": map[string]any{"include": []any{"refs/heads/ledger"}}},
		"rules":      []any{map[string]any{"type": "deletion"}, map[string]any{"type": "non_fast_forward"}, map[string]any{"type": "update"}}}}
	g.IssueTypes = []*fake.IssueType{{ID: 3, Name: "Bug"}}
	dir := checkout(t)
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Product\n\nOur rules.\n"), 0o644)
	code, rep, errOut := runInitCmd(t, g, dir, "init", "--yes")
	if code != 0 {
		t.Fatalf("init: exit %d: %s", code, errOut)
	}
	for _, w := range g.Writes() {
		if strings.Contains(w.Path, "/git/") || strings.Contains(w.Path, "/rulesets") {
			t.Errorf("init wrote %s %s; the ledger and its ruleset already exist", w.Method, w.Path)
		}
	}
	if len(g.IssueTypes) != 10 || !strings.Contains(snapshot(t, dir)[initrepo.FormPath(workitems.Types[3])], "\ntype: \"Story\"\n") {
		t.Errorf("organization: %d issue types; forms must set the type", len(g.IssueTypes))
	}
	_ = rep
	if code, out, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "x"}, Options{GitHubAPI: g.URL(), Dir: dir}, "uninit", "--external"); code != 0 || !strings.Contains(out, "no external resources") {
		t.Errorf("uninit --external: %d %s", code, out)
	}
	if code, _, errOut := runInitCmd(t, g, dir, "uninit", "--yes"); code != 0 {
		t.Fatalf("uninit: exit %d: %s", code, errOut)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if len(g.IssueTypes) != 1 || g.Refs["heads/ledger"] != "abc123" || len(g.Rulesets) != 1 || len(g.Labels) != 0 {
		t.Errorf("after uninit: %d types, ledger %q, %d rulesets, %d labels", len(g.IssueTypes), g.Refs["heads/ledger"], len(g.Rulesets), len(g.Labels))
	}
	after := snapshot(t, dir)
	if after["AGENTS.md"] != "# Product\n\nOur rules.\n" || after[".github/workflows/ci.yml"] != ciYML || len(after) != 2 {
		t.Errorf("files after uninit: %v", slices.Sorted(func(y func(string) bool) {
			for k := range after {
				if !y(k) {
					return
				}
			}
		}))
	}
}
