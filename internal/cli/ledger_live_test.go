package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

// TestLiveLedgerWriters proves D-4.2b.h against GitHub (AAYU_LIVE=1;
// AAYU_LIVE_KEEP=1 keeps the repository). On a public personal aayu-scratch-*
// repository, `aayu init` creates the ledger rulesets; then:
//
//   - the admin (covered by the write-role bypass) fast-forwards the ledger;
//   - nobody, the admin included, force-pushes or deletes it;
//   - the Actions workflow token, with contents: write, cannot update it;
//   - with AAYU_LIVE_WRITER_TOKEN (a second, non-managed account), that
//     account, invited as a collaborator (write), fast-forwards it and cannot
//     force-push or delete it;
//   - `aayu uninit` removes the rulesets before the ledger, so the history
//     ruleset does not block its own cleanup.
func TestLiveLedgerWriters(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("live test: set AAYU_LIVE=1")
	}
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin)
	liveAPI(t, admin, "PATCH", "/repos/"+repo, map[string]any{"private": false}) // rulesets on a personal private repository need GitHub Pro
	// GitHub locks a repository for a few seconds after a visibility change; init does not retry a locked write.
	liveAPI(t, admin, "PATCH", "/repos/"+repo, map[string]any{"description": "aayudha live test scratch repository (ledger writers)"})
	time.Sleep(10 * time.Second)
	dir := filepath.Join(t.TempDir(), "clone")
	g := &ledger.Writer{Dir: dir, Token: admin}
	for i := 0; ; i++ { // the repository reads as disabled until the visibility change settles
		err := g.Exec(t.Context(), filepath.Dir(dir), "clone", "--quiet", "https://github.com/"+repo+".git", dir)
		if err == nil {
			break
		}
		if i == 12 {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Second)
	}
	env := map[string]string{"AAYU_GITHUB_TOKEN": admin, "PATH": os.Getenv("PATH")}
	initRepo := func() {
		t.Helper()
		if code, out, errOut := run(t, env, Options{Dir: dir}, "init", "--yes", "--repo", repo); code != 0 {
			t.Fatalf("init: %d %s %s", code, out, errOut)
		}
	}
	initRepo()
	rulesets := func() string {
		var list []struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(liveAPI(t, admin, "GET", "/repos/"+repo+"/rulesets", nil), &list)
		var out []string
		for _, r := range list {
			var d struct {
				Name   string `json:"name"`
				Rules  []struct{ Type string }
				Bypass []struct {
					ActorID   int64  `json:"actor_id"`
					ActorType string `json:"actor_type"`
				} `json:"bypass_actors"`
				CanBypass string `json:"current_user_can_bypass"`
			}
			_ = json.Unmarshal(liveAPI(t, admin, "GET", fmt.Sprintf("/repos/%s/rulesets/%d", repo, r.ID), nil), &d)
			out = append(out, fmt.Sprintf("%s %v bypass %v (admin: %s)", d.Name, d.Rules, d.Bypass, d.CanBypass))
		}
		return strings.Join(out, "; ")
	}
	t.Logf("rulesets after init: %s", rulesets())

	// Uninit removes the rulesets first, so it can delete the untouched ledger; then init again.
	if code, out, errOut := run(t, env, Options{Dir: dir}, "uninit", "--yes", "--repo", repo); code != 0 {
		t.Fatalf("uninit: %d %s %s", code, out, errOut)
	}
	if out, _ := g.Output(t.Context(), dir, "ls-remote", "origin", "refs/heads/ledger"); out != "" || rulesets() != "" {
		t.Fatalf("after uninit: ledger %q, rulesets %s", out, rulesets())
	}
	initRepo()

	// push runs git push as token and returns its output and whether it was refused.
	push := func(token string, args ...string) (string, bool) {
		t.Helper()
		w := &ledger.Writer{Dir: dir, Token: token}
		out, err := w.Output(t.Context(), dir, append([]string{"push", "origin"}, args...)...)
		if err != nil {
			out = err.Error()
		}
		return out, err != nil
	}
	gitDo := func(args ...string) string {
		t.Helper()
		out, err := g.Output(t.Context(), dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	gitDo("config", "user.name", "aayudha live test")
	gitDo("config", "user.email", "aayudha-live@users.noreply.github.com")
	gitDo("fetch", "--quiet", "origin", "ledger")
	gitDo("checkout", "--quiet", "-B", "ledger", "origin/ledger")
	commit := func(msg string) {
		_ = os.WriteFile(filepath.Join(dir, "probe.txt"), []byte(msg+"\n"), 0o644)
		gitDo("add", "probe.txt")
		gitDo("commit", "--quiet", "-m", msg)
	}
	writers := func(who, token string) {
		t.Helper()
		gitDo("fetch", "--quiet", "origin", "ledger")
		gitDo("reset", "--quiet", "--hard", "origin/ledger")
		commit(who + " fast-forward")
		if out, refused := push(token, "HEAD:refs/heads/ledger"); refused {
			t.Fatalf("%s fast-forward refused: %s", who, out)
		}
		gitDo("reset", "--quiet", "--hard", "HEAD~1")
		commit(who + " rewrite")
		if out, refused := push(token, "--force", "HEAD:refs/heads/ledger"); !refused || !strings.Contains(out, "Cannot force-push") {
			t.Fatalf("%s force push: refused %t: %s", who, refused, out)
		}
		if out, refused := push(token, ":refs/heads/ledger"); !refused || !strings.Contains(out, "Cannot delete") {
			t.Fatalf("%s delete: refused %t: %s", who, refused, out)
		}
		t.Logf("%s: fast-forward pushed; force push and deletion refused", who)
	}
	writers("admin", admin)

	if w := os.Getenv("AAYU_LIVE_WRITER_TOKEN"); w != "" {
		var who struct {
			Login string `json:"login"`
		}
		_ = json.Unmarshal(liveAPI(t, w, "GET", "/user", nil), &who)
		var inv struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(liveAPI(t, admin, "PUT", "/repos/"+repo+"/collaborators/"+who.Login, map[string]any{"permission": "push"}), &inv)
		liveAPI(t, w, "PATCH", fmt.Sprintf("/user/repository_invitations/%d", inv.ID), nil)
		writers("collaborator "+who.Login+" (write)", w)
	} else {
		t.Log("AAYU_LIVE_WRITER_TOKEN unset: no second account, so a write collaborator's push is not exercised")
	}

	// The Actions workflow token cannot be a bypass actor (D1), so even with contents: write it cannot update the ledger.
	gitDo("checkout", "--quiet", "-f", "-B", "main", "origin/main")
	probe := `name: ledger-probe
on: workflow_dispatch
permissions:
  contents: write
jobs:
  probe:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ledger
      - run: |
          git config user.name ci && git config user.email ci@example.com
          echo ci > probe.txt && git add probe.txt && git commit -qm "workflow token write"
          git push origin HEAD:ledger
`
	_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ledger-probe.yml"), []byte(probe), 0o644)
	gitDo("add", ".github/workflows/ledger-probe.yml")
	gitDo("commit", "--quiet", "-m", "ci: ledger probe")
	if out, refused := push(admin, "HEAD:refs/heads/main"); refused {
		t.Fatal(out)
	}
	time.Sleep(5 * time.Second)
	liveAPI(t, admin, "POST", "/repos/"+repo+"/actions/workflows/ledger-probe.yml/dispatches", map[string]any{"ref": "main"})
	for deadline := time.Now().Add(10 * time.Minute); ; time.Sleep(10 * time.Second) {
		var runs struct {
			Runs []struct {
				ID         int64  `json:"id"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				URL        string `json:"html_url"`
			} `json:"workflow_runs"`
		}
		_ = json.Unmarshal(liveAPI(t, admin, "GET", "/repos/"+repo+"/actions/workflows/ledger-probe.yml/runs", nil), &runs)
		if len(runs.Runs) > 0 && runs.Runs[0].Status == "completed" {
			if runs.Runs[0].Conclusion != "failure" {
				t.Fatalf("the workflow token updated the ledger: %s", runs.Runs[0].URL)
			}
			t.Logf("workflow token refused: %s", runs.Runs[0].URL)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ledger-probe did not complete")
		}
	}
	if !strings.Contains(rulesets(), initrepo.LedgerHistoryRuleset) {
		t.Fatalf("rulesets: %s", rulesets())
	}
	t.Logf("rulesets: %s", rulesets())
}
