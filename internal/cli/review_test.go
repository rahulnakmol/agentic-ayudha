package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/uireview"
)

// laneAReply is a lane A answer reporting the given kinds at severity 3.
func laneAReply(kinds ...string) models.Response {
	fs := []map[string]any{}
	for _, k := range kinds {
		fs = append(fs, map[string]any{"kind": k, "heuristic": 4, "severity": 3, "element": "button", "summary": k, "fix": "fix " + k})
	}
	b, _ := json.Marshal(map[string]any{"findings": fs, "nielsen": make([]int, 10), "personas": []any{}, "rebuild": false})
	return models.Response{Text: string(b)}
}

// TestReviewAndPolish drives `aayu review` and `aayu polish` on a PR whose
// story has one screenshot criterion at `/page`. polish without a critique and
// review on a checkout that is not the PR head are refused (exit 4). With the
// floor tools (AAYU_FLOOR_WEB=1), review of the planted page writes the
// critique, posts the replace-in-place comment, and exits 4 (fix); after the
// page is fixed, polish reports every finding resolved and exits 0. Lane A is
// a fake model; its replies are scripted.
func TestReviewAndPolish(t *testing.T) {
	g := readyTree(t)
	g.Mu.Lock()
	g.Issues[2].Body = fixtureBody(t, "story", func(f map[string]string) {
		f["completion_goal"] = "1. [screenshot] state: `/page` signed in as the seeded customer; expected: the delivery preferences form"
	})
	g.Mu.Unlock()
	dir, _ := ledgerCheckout(t)
	nodePath, _ := filepath.Abs("../../packages/floors/node_modules")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_SECRET_GOOGLE": "model-key", "PATH": os.Getenv("PATH"), "NODE_PATH": nodePath}
	fake := &models.Fake{Replies: []models.Response{laneAReply("focus-visible", "contrast", "target-size"), laneAReply("focus-visible", "contrast", "target-size"),
		laneAReply("contrast", "target-size"), laneAReply()}}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, ModelClient: func(models.Route, string) models.Client { return fake }}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	var mu sync.Mutex
	page := "planted"
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		name := page
		mu.Unlock()
		b, err := os.ReadFile("../uireview/testdata/" + name + ".html")
		if err != nil || r.URL.Path != "/page" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(b)
	}))
	defer site.Close()
	args := []string{"--repo", "o/r", "--set", "verify.environments.local.url=" + site.URL}

	pr := g.AddPR("Delivery preferences", "Refs #2", "founder", "story/2-delivery", strings.Repeat("c", 40), "dev")
	if code, _, errOut := run(t, env, opts, append([]string{"polish", "1"}, args...)...); code != ExitBlocked || !strings.Contains(errOut, "run `aayu review 1` first") {
		t.Fatalf("polish without a critique: exit %d: %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, append([]string{"review", itoa(pr.Number)}, args...)...); code != ExitBlocked || !strings.Contains(errOut, "check out the PR head first") {
		t.Fatalf("review off the PR head: exit %d: %s", code, errOut)
	}
	if os.Getenv("AAYU_FLOOR_WEB") != "1" {
		t.Skip("the browser half needs AAYU_FLOOR_WEB=1 with node, playwright, axe-core, impeccable, and Chromium (packages/floors)")
	}
	g.Mu.Lock()
	pr.Head.SHA = git(t, dir, "rev-parse", "HEAD")
	g.Mu.Unlock()

	code, out, errOut := run(t, env, opts, append([]string{"review", itoa(pr.Number)}, args...)...)
	if code != ExitBlocked || !strings.Contains(errOut, "fix") {
		t.Fatalf("review of the planted page: exit %d: %s\n%s", code, errOut, out)
	}
	raw, err := os.ReadFile(filepath.Join(reviewDir(dir, pr.Number), "critique.json"))
	if err != nil {
		t.Fatal(err)
	}
	var crit uireview.Critique
	_ = json.Unmarshal(raw, &crit)
	if crit.LaneA.Disposition != uireview.Fix || crit.LaneB.Disposition != uireview.Fix || len(crit.LaneA.Findings) != 3 || crit.Head != pr.Head.SHA {
		t.Fatalf("critique: %s", raw)
	}
	if len(fake.Requests) != uireview.K || len(fake.Requests[0].Messages[0].Images) != 2 || fake.Requests[0].Effort != "high" {
		t.Fatalf("lane A calls: %d, effort %q", len(fake.Requests), fake.Requests[0].Effort)
	}
	g.Mu.Lock()
	comments := g.Issues[pr.Number].Comments
	g.Mu.Unlock()
	if len(comments) != 1 || !strings.Contains(comments[0].Body, uireview.Marker) || !strings.Contains(comments[0].Body, "### UI review: fix") {
		t.Fatalf("review comment: %+v", comments)
	}

	mu.Lock()
	page = "clean" // the host applied the critique
	mu.Unlock()
	code, out, errOut = run(t, env, opts, append([]string{"polish", itoa(pr.Number), "--json"}, args...)...)
	if code != 0 {
		t.Fatalf("polish: exit %d: %s\n%s", code, errOut, out)
	}
	var res ReviewResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Polish == nil || len(res.Polish.Remaining) != 0 || len(res.Polish.Resolved) < 6 || res.Critique.Disposition != uireview.Ship {
		t.Fatalf("polish result: %v %s", err, out)
	}
	g.Mu.Lock()
	comments = g.Issues[pr.Number].Comments
	g.Mu.Unlock()
	if len(comments) != 1 || !strings.Contains(comments[0].Body, "### UI review: ship") || !strings.Contains(comments[0].Body, "0 remaining") {
		t.Fatalf("comment after polish: %+v", comments)
	}
}

// TestBaseline: baseline refuses a --sha the checkout is not at (exit 4); with
// the floor tools it captures each [review] baseline path and pins the
// screenshots in baselines/<sha>/manifest.json on the ledger (hash only on
// this public fixture repository, V3).
func TestBaseline(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	dir, bare := ledgerCheckout(t)
	first := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "second")
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := os.ReadFile("../uireview/testdata/clean.html")
		_, _ = w.Write(b)
	}))
	defer site.Close()
	nodePath, _ := filepath.Abs("../../packages/floors/node_modules")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "NODE_PATH": nodePath}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	args := []string{"--repo", "o/r", "--set", "verify.environments.local.url=" + site.URL, "--set", "review.baseline=/,/settings"}
	if code, _, errOut := run(t, env, opts, append([]string{"baseline", "--sha", first}, args...)...); code != ExitBlocked || !strings.Contains(errOut, "check it out") {
		t.Fatalf("baseline of another commit: exit %d: %s", code, errOut)
	}
	if os.Getenv("AAYU_FLOOR_WEB") != "1" {
		t.Skip("the capture needs AAYU_FLOOR_WEB=1 with node, playwright, and Chromium (packages/floors)")
	}
	head := git(t, dir, "rev-parse", "HEAD")
	if code, out, errOut := run(t, env, opts, append([]string{"baseline"}, args...)...); code != 0 {
		t.Fatalf("baseline: exit %d: %s\n%s", code, errOut, out)
	}
	var m struct {
		SHA   string `json:"sha"`
		Files []struct {
			Path, SHA256, Store string
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:baselines/"+head+"/manifest.json")), &m); err != nil {
		t.Fatal(err)
	}
	if m.SHA != head || len(m.Files) != 2 || m.Files[0].Path != "root.png" || m.Files[1].Path != "settings.png" || m.Files[0].Store != "none" || len(m.Files[0].SHA256) != 64 {
		t.Fatalf("baseline manifest: %+v", m)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// polish off the PR head prepares .aayudha/worktrees/pr-<n> on the PR's
// branch with the critique copied in, instead of asking for the main
// checkout to switch branches (WP5.5b); the worktree sees review's critique
// (reviewDir is shared by the clone's worktrees); gc removes it once the PR closes.
func TestPolishPreparesWorktree(t *testing.T) {
	g := readyTree(t)
	dir, _ := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	git(t, dir, "checkout", "--quiet", "-b", "story/2-delivery")
	_ = os.WriteFile(filepath.Join(dir, "page.html"), []byte("<p>form</p>\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "feat: delivery preferences")
	git(t, dir, "push", "--quiet", "origin", "story/2-delivery")
	head := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "--quiet", "dev")
	pr := g.AddPR("Delivery preferences", "Refs #2", "founder", "story/2-delivery", head, "dev")
	_ = os.MkdirAll(reviewDir(dir, pr.Number), 0o755)
	_ = os.WriteFile(filepath.Join(reviewDir(dir, pr.Number), "critique.json"), []byte(`{"disposition":"fix"}`), 0o644)
	code, _, errOut := run(t, env, opts, "polish", itoa(pr.Number), "--repo", "o/r")
	wt := filepath.Join(dir, ".aayudha", "worktrees", "pr-"+itoa(pr.Number))
	if code != ExitBlocked || !strings.Contains(errOut, "applied in an aayu worktree: "+wt) || git(t, wt, "rev-parse", "HEAD") != head {
		t.Fatalf("polish off the head: %d %s", code, errOut)
	}
	if b, _ := os.ReadFile(filepath.Join(reviewDir(wt, pr.Number), "critique.json")); string(b) != `{"disposition":"fix"}` {
		t.Fatalf("the worktree does not see the critique: %q", b)
	}
	if git(t, dir, "rev-parse", "--abbrev-ref", "HEAD") != "dev" {
		t.Fatal("polish switched the main checkout")
	}
	g.Mu.Lock()
	g.Issues[pr.Number].State = "closed"
	g.Mu.Unlock()
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r"); code != 0 || !strings.Contains(out, "worktree #"+itoa(pr.Number)+" removed") || dirExists(wt) {
		t.Fatalf("gc: %d %s %s", code, out, errOut)
	}
}
