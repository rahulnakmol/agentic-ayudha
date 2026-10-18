package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// TestBrainInitCheckpointCheck runs the verb end to end against a local
// remote: init creates the orphan branch, a person's note survives a
// checkpoint that fills its generated blocks, and check fails a broken note.
func TestBrainInitCheckpointCheck(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	feature := g.AddIssue("Proof packs", "", "type:feature")
	story := g.AddIssue("Push a pack", "", "type:story")
	story.Parent = feature.Number
	if g.Releases == nil {
		g.Releases = map[string]*fake.Release{}
	}
	g.Releases["v0.1.0"] = &fake.Release{ID: 1, TagName: "v0.1.0", HTMLURL: "https://github.com/o/r/releases/tag/v0.1.0", PublishedAt: "2026-10-15T18:19:06Z"}
	dir, bare := ledgerCheckout(t)
	rung := filepath.Join(t.TempDir(), "ledger")
	git(t, filepath.Dir(rung), "clone", "--quiet", "--branch", "ledger", bare, rung)
	_ = os.MkdirAll(filepath.Join(rung, "releases", "9"), 0o755)
	_ = os.WriteFile(filepath.Join(rung, "releases", "9", "main.json"), []byte(`{"release": 9, "version": "v0.1.0", "target": "main", "tag": "v0.1.0", "stories_released": [2]}`), 0o644)
	git(t, rung, "add", ".")
	git(t, rung, "commit", "--quiet", "-m", "rung")
	git(t, rung, "push", "--quiet", "origin", "ledger")

	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Assets: aayudha.Assets}
	brainRun := func(want int, args ...string) string {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"brain"}, append(args, "--repo", "o/r")...)...)
		if code != want {
			t.Fatalf("brain %v: exit %d, want %d: %s\n%s", args, code, want, errOut, out)
		}
		return out + errOut
	}
	brainRun(4, "checkpoint") // no brain branch yet
	if out := brainRun(0, "init"); !strings.Contains(out, "brain: created") {
		t.Fatalf("init: %s", out)
	}
	if out := brainRun(0, "init"); !strings.Contains(out, "brain: exists") {
		t.Fatalf("a second init: %s", out)
	}
	brainRun(0, "init", "--workstream", "payments")

	vault := filepath.Join(t.TempDir(), "vault")
	git(t, dir, "fetch", "--quiet", "origin", "brain")
	git(t, dir, "worktree", "add", "--quiet", vault, "FETCH_HEAD")
	if b, _ := os.ReadFile(filepath.Join(vault, "Workstreams", "Workstreams.md")); !strings.Contains(string(b), "- [[payments Index]]") {
		t.Errorf("a workstream added to an existing brain is not on the root Workstreams map:\n%s", b)
	}
	human := "# Proof packs\n\n## What it does\n\nIt keeps the evidence for each change.\n"
	capNote := "---\nid: capability-proof\ntype: capability\ntitle: Proof packs\nowner: product-strategist\nstatus: live\nupdated: 2026-10-01\nreviewed: 2026-10-01\nsources: [\"issue #1\"]\nup: \"[[Capabilities]]\"\nissues: [1]\ntags: [type/capability]\n---\n\n" + human
	_ = os.WriteFile(filepath.Join(vault, "Capabilities", "Proof packs.md"), []byte(capNote), 0o644)
	brainRun(0, "check", "--dir", vault)
	git(t, vault, "add", ".")
	git(t, vault, "commit", "--quiet", "-m", "a capability")
	git(t, vault, "push", "--quiet", "origin", "HEAD:refs/heads/brain")

	if out := brainRun(0, "checkpoint"); !strings.Contains(out, "needs review: Capabilities/Proof packs.md") {
		t.Fatalf("checkpoint did not flag the reviewed note whose facts changed: %s", out)
	}
	brainRun(0, "check")
	git(t, vault, "pull", "--quiet", "origin", "brain")
	b, _ := os.ReadFile(filepath.Join(vault, "Capabilities", "Proof packs.md"))
	for _, want := range []string{human, "- Recorded in #1 Feature: Proof packs.", "- Delivered in [[v0.1.0]] by #2."} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the capability lacks %q:\n%s", want, b)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(vault, "Releases", "v0.1.0.md")); !strings.Contains(string(b), "- #1 Feature: Proof packs [[Proof packs]]\n  - #2 Story: Push a pack") {
		t.Errorf("the release note does not link what shipped:\n%s", b)
	}

	_ = os.WriteFile(filepath.Join(vault, "Capabilities", "Proof packs.md"), []byte(strings.Replace(capNote, human, human+"See [[Nowhere]].\n", 1)), 0o644)
	if out := brainRun(3, "check", "--dir", vault); !strings.Contains(out, "[[Nowhere]] does not resolve") {
		t.Errorf("check --dir passed a broken link: %s", out)
	}
	if out := brainRun(0, "open"); !strings.Contains(out, "git worktree add ../r-brain brain") {
		t.Errorf("open: %s", out)
	}
}

// TestBrainRefusesAnUnrelatedBranch: a branch named brain that is not a vault
// is someone else's, and neither init nor checkpoint writes to it.
func TestBrainRefusesAnUnrelatedBranch(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	dir, bare := ledgerCheckout(t)
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "--quiet", bare, other)
	git(t, other, "checkout", "--quiet", "--orphan", "brain")
	_ = os.WriteFile(filepath.Join(other, "README.md"), []byte("my notes\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(other, ".obsidian"), 0o755) // Obsidian writes types.json in any vault with properties
	_ = os.WriteFile(filepath.Join(other, ".obsidian", "types.json"), []byte(`{"types":{"owner":"text"}}`+"\n"), 0o644)
	git(t, other, "add", ".")
	git(t, other, "commit", "--quiet", "-m", "mine")
	git(t, other, "push", "--quiet", "origin", "brain")
	tip := git(t, other, "rev-parse", "HEAD")
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Assets: aayudha.Assets}
	for _, args := range [][]string{{"brain", "init", "--workstream", "x"}, {"brain", "checkpoint"}} {
		code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, append(args, "--repo", "o/r")...)
		if code != ExitBlocked || !strings.Contains(errOut, "not an aayudha vault") {
			t.Errorf("%v on an unrelated brain branch: exit %d: %s%s", args, code, out, errOut)
		}
	}
	if got := git(t, bare, "rev-parse", "refs/heads/brain"); got != tip {
		t.Errorf("the unrelated brain branch moved to %s", got)
	}
}
