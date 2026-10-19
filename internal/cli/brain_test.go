package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/brain"
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
	_ = os.MkdirAll(filepath.Join(rung, "releases", "v0.2.0"), 0o755) // the first baseline is number 0
	_ = os.WriteFile(filepath.Join(rung, "releases", "v0.2.0", "baseline-0.json"), []byte(`{"version": "v0.2.0", "release_issue": 10, "baselined_at": "2026-10-16T09:00:00Z", "budget_pct": 10, "items": [{"number": 2, "type": "story", "title": "Push a pack", "parent": 1}]}`), 0o644)
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
	if b, _ := os.ReadFile(filepath.Join(vault, "Releases", "v0.2.0.md")); !strings.Contains(string(b), "Baseline 0 of Release #10") {
		t.Errorf("the release note lacks the plan from baseline 0:\n%s", b)
	}

	_ = os.WriteFile(filepath.Join(vault, "Capabilities", "Proof packs.md"), []byte(strings.Replace(capNote, human, human+"See [[Nowhere]].\n", 1)), 0o644)
	if out := brainRun(3, "check", "--dir", vault); !strings.Contains(out, "[[Nowhere]] does not resolve") {
		t.Errorf("check --dir passed a broken link: %s", out)
	}
	// A symlink in the vault could carry a read or a write out of it: aayu refuses it on the branch and in --dir.
	_ = os.WriteFile(filepath.Join(vault, "Capabilities", "Proof packs.md"), []byte(capNote), 0o644)
	if err := os.Symlink(t.TempDir(), filepath.Join(vault, "Releases", "elsewhere")); err != nil {
		t.Fatal(err)
	}
	if out := brainRun(2, "check", "--dir", vault); !strings.Contains(out, "Releases/elsewhere is not a regular file") {
		t.Errorf("check --dir read through a symlink: %s", out)
	}
	git(t, vault, "add", ".")
	git(t, vault, "commit", "--quiet", "-m", "a symlink")
	git(t, vault, "push", "--quiet", "origin", "HEAD:refs/heads/brain")
	for _, action := range []string{"checkpoint", "check"} {
		if out := brainRun(4, action); !strings.Contains(out, "holds Releases/elsewhere, which is not a regular file (mode 120000)") {
			t.Errorf("%s over a symlink on the branch: %s", action, out)
		}
	}
	// The printed steps open the vault as printed, in a single-branch clone too
	// (CI and orbs clone one branch, so a bare `git fetch origin brain` makes no origin/brain).
	single := filepath.Join(t.TempDir(), "single")
	git(t, filepath.Dir(single), "clone", "--quiet", "--single-branch", "--branch", "dev", bare, single)
	out := brainRun(0, "open")
	var steps []string
	for _, l := range strings.Split(out, "\n") {
		if _, cmd, ok := strings.Cut(l, ". git "); ok {
			cmd, _, _ = strings.Cut(cmd, "  #")
			steps = append(steps, strings.Fields(cmd)...)
			steps = append(steps, "\x00")
		}
	}
	for _, step := range strings.Split(strings.Join(steps, " "), "\x00") {
		if args := strings.Fields(step); len(args) > 0 {
			git(t, single, args...)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(single), "r-brain", brain.HomePath)); err != nil {
		t.Errorf("the steps of open did not open the vault (%v):\n%s", err, out)
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

// TestBrainRetriesAfterAConcurrentPush: a push lands between aayu's read of
// the brain tip and its write. Each retry reads the tree again and runs every
// check again. A retry with nothing left to write succeeds. A branch that
// stopped being a vault gets no commit.
func TestBrainRetriesAfterAConcurrentPush(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	dir, bare := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Assets: aayudha.Assets}
	if code, out, errOut := run(t, env, opts, "brain", "init", "--repo", "o/r"); code != 0 {
		t.Fatalf("init: exit %d: %s%s", code, out, errOut)
	}
	other := filepath.Join(t.TempDir(), "other") // another writer's checkout of the brain branch
	git(t, filepath.Dir(other), "clone", "--quiet", "--branch", "brain", bare, other)
	pushOnce := func(change func()) func(int, string) {
		done := false
		return func(int, string) {
			if done {
				return
			}
			done = true
			git(t, other, "pull", "--quiet", "origin", "brain")
			change()
			git(t, other, "add", "--all", ".")
			git(t, other, "commit", "--quiet", "-m", "a concurrent push")
			git(t, other, "push", "--quiet", "origin", "HEAD:refs/heads/brain")
		}
	}

	// Another init adds the same workstream first: the retry finds it there and succeeds.
	race := opts
	race.LedgerAttempt = pushOnce(func() {
		for p, b := range brain.Workstream("o/r", "ops", "2026-10-19") {
			_ = os.MkdirAll(filepath.Join(other, filepath.Dir(p)), 0o755)
			_ = os.WriteFile(filepath.Join(other, p), b, 0o644)
		}
	})
	if code, out, errOut := run(t, env, race, "brain", "init", "--workstream", "ops", "--repo", "o/r"); code != 0 || !strings.Contains(out, "brain: workstream exists") {
		t.Errorf("init after another init added the workstream: exit %d: %s%s", code, out, errOut)
	}

	// A person turns the branch into something else meanwhile: the checkpoint writes nothing to it.
	race.LedgerAttempt = pushOnce(func() {
		git(t, other, "rm", "--quiet", "-r", brain.MarkerPath, brain.HomePath)
	})
	g.AddIssue("Proof packs", "", "type:feature") // a record, so the checkpoint has a note to change
	_ = os.WriteFile(filepath.Join(other, "Capabilities", "Proof packs.md"), []byte("---\nid: capability-proof\ntype: capability\ntitle: Proof packs\nowner: product-strategist\nstatus: live\nupdated: 2026-10-01\nsources: [\"issue #1\"]\nup: \"[[Capabilities]]\"\nissues: [1]\ntags: [type/capability]\n---\n\n# Proof packs\n"), 0o644)
	git(t, other, "add", "--all", ".")
	git(t, other, "commit", "--quiet", "-m", "a capability")
	git(t, other, "push", "--quiet", "origin", "HEAD:refs/heads/brain")
	code, out, errOut := run(t, env, race, "brain", "checkpoint", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "not an aayudha vault") {
		t.Errorf("checkpoint after the branch stopped being a vault: exit %d: %s%s", code, out, errOut)
	}
	git(t, other, "fetch", "--quiet", "origin", "brain")
	if msg := git(t, other, "log", "-1", "--format=%s", "FETCH_HEAD"); msg != "a concurrent push" {
		t.Errorf("the checkpoint wrote to a branch that is not a vault: the tip is %q", msg)
	}
}
