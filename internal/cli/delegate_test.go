package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

func delegateFixture(t *testing.T) (*fake.GitHub, map[string]string, Options, string, string) {
	t.Helper()
	g := readyTree(t)
	dir, bare := ledgerCheckout(t)
	git(t, dir, "config", "user.name", "Builder")
	git(t, dir, "config", "user.email", "builder@example.com")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "AAYU_SECRET_OPENAI": "model-key", "AAYU_SECRET_XAI": "xai-key"}
	monday := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, CheckpointVerify: holds, NotifySink: &notify.Fake{}, Now: func() time.Time { return monday },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	return g, env, opts, dir, bare
}

func delegateRun(t *testing.T, env map[string]string, opts Options, args ...string) (int, BuildResult, string) {
	t.Helper()
	code, out, errOut := run(t, env, opts, append([]string{"build", "2", "--repo", "o/r", "--json"}, args...)...)
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	return code, res, errOut
}

// fakeCursor serves Cursor's cloud agents API from the recorded fixtures and
// records each request; finished turns the run FINISHED with the PR.
type fakeCursor struct {
	mu       sync.Mutex
	reqs     []map[string]any
	finished int // the PR number the finished run reports; 0 while running
}

func (f *fakeCursor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.reqs = append(f.reqs, map[string]any{"method": r.Method, "path": r.URL.Path, "body": body, "auth": r.Header.Get("Authorization")})
	read := func(p string) string {
		b, _ := os.ReadFile(filepath.Join("..", "delegate", "testdata", "cursor", p))
		return string(b)
	}
	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/agents":
		_, _ = io.WriteString(w, read("create.json"))
	case r.Method == "GET" && f.finished == 0:
		_, _ = io.WriteString(w, read("run-running.json"))
	case r.Method == "GET":
		_, _ = io.WriteString(w, strings.ReplaceAll(read("run-finished.json"), "{{PR}}", strconv.Itoa(f.finished)))
	default:
		_, _ = io.WriteString(w, read("followup.json"))
	}
}

// WP5.5b on fixtures, end to end: a story that is not Ready is refused; the
// Ready story is handed to Cursor's cloud agent with the brief; sync waits
// for the PR, finds it by the run's PR URL, waits for inspect, sends inspect's
// findings once per head, and reports merge-ready for a human's ship. The
// story's delegation comment, the ledger state, and the actions log record it.
func TestDelegateCursorReviewLoop(t *testing.T) {
	g, env, opts, dir, bare := delegateFixture(t)
	cur := &fakeCursor{}
	srv := httptest.NewServer(cur)
	defer srv.Close()
	env["AAYU_SECRET_CURSOR_API_KEY"] = "cursor-canary-key"
	set := []string{"--delegate", "cursor", "--set", "delegate.cursor_api=" + srv.URL}
	transcript := func(step string, code int, res BuildResult, errOut string) {
		d := res.Delegation
		if d == nil {
			t.Logf("%s: exit %d %s", step, code, strings.TrimSpace(errOut))
			return
		}
		t.Logf("%s: exit %d, %s (%s) %s, PR #%d, next: %s", step, code, d.Receiver, d.Mode, d.Status, d.PR, res.Next)
	}

	for _, r := range []string{"codex", "cursor", "claude", "amp"} {
		code, _, errOut := run(t, env, opts, "build", "6", "--delegate", r, "--repo", "o/r")
		if code != ExitBlocked || !strings.Contains(errOut, "status:ready") && !strings.Contains(errOut, "does not pass Ready") {
			t.Fatalf("%s on a story that is not Ready: %d %s", r, code, errOut)
		}
	}

	code, res, errOut := delegateRun(t, env, opts, set...)
	transcript("hand-off", code, res, errOut)
	if code != 0 || res.Delegation == nil || res.Delegation.Status != dlgHandedOff || res.Delegation.Mode != "cloud" {
		t.Fatalf("hand-off: %d %+v %s", code, res, errOut)
	}
	cur.mu.Lock()
	first := cur.reqs[0]
	cur.mu.Unlock()
	brief := first["body"].(map[string]any)["prompt"].(map[string]any)["text"].(string)
	if first["auth"] != "Bearer cursor-canary-key" || !strings.Contains(brief, "Refs #2") || !strings.Contains(brief, "# Delegated story #2: Show empty list") ||
		strings.Contains(brief, "cursor-canary-key") || strings.Contains(brief, "human-token") || strings.Contains(brief, "model-key") {
		t.Fatalf("hand-off request: %v\n%s", first["auth"], brief)
	}
	var st delegate.State
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+delegate.LedgerPath(2))), &st); err != nil || st.Receiver != "cursor" || st.BriefHash != delegate.BriefHash(brief) || st.Task == "" {
		t.Fatalf("ledger: %v %+v", err, st)
	}
	if c := comment(g, 2, delegateMarker); !strings.Contains(c, "Delegated to cursor") || !strings.Contains(c, "handed-off") || !strings.Contains(c, "no merge rights") {
		t.Fatalf("story comment:\n%s", c)
	}

	code, res, errOut = delegateRun(t, env, opts, set...)
	transcript("sync while the agent works", code, res, errOut)
	if code != 0 || res.Delegation.Status != dlgWorking || !strings.Contains(res.Next, "still working") {
		t.Fatalf("working: %d %+v %s", code, res, errOut)
	}

	head := strings.Repeat("c", 40)
	pr := g.AddPR("feat: show empty list", "Refs #2\n\nCursor's change.", "founder", "cursor/show-empty-list-7f3a", head, "dev")
	g.Mu.Lock()
	pr.Diff = defectDiff
	g.Mu.Unlock()
	cur.mu.Lock()
	cur.finished = pr.Number
	cur.mu.Unlock()
	code, res, errOut = delegateRun(t, env, opts, set...)
	transcript("sync after the agent's PR", code, res, errOut)
	if code != 0 || res.Delegation.PR != pr.Number || !strings.Contains(res.Next, "waiting for the merge gate's inspect") {
		t.Fatalf("PR found: %d %+v %s", code, res, errOut)
	}

	// The merge gate's inspect, in general mode for a delegated PR (D-4.8b.7):
	// one reviewer raises a finding; it is posted once, inline.
	writeInspectPack(t, dir, pr.Number, head, nil)
	model := &models.Fake{Replies: []models.Response{inspectReply("fix", "P1")}}
	icode, ires, ierr := inspectRun(t, env, opts, model, pr.Number)
	t.Logf("inspect #%d: exit %d, %s (%s: %s), findings %d", pr.Number, icode, ires.Disposition, ires.Mode, strings.Join(ires.Reviewers, " + "), len(ires.Findings))
	if icode != ExitBlocked || ires.Mode != inspect.ModeGeneral || len(ires.Reviewers) != 1 || len(ires.Findings) != 1 {
		t.Fatalf("inspect: %d %+v %s", icode, ires, ierr)
	}

	code, res, errOut = delegateRun(t, env, opts, set...)
	transcript("sync after inspect asked for a fix", code, res, errOut)
	cur.mu.Lock()
	last := cur.reqs[len(cur.reqs)-1]
	cur.mu.Unlock()
	msg := last["body"].(map[string]any)["prompt"].(map[string]any)["text"].(string)
	if code != 0 || res.Delegation.Status != dlgFix || !res.Delegation.FollowUp || last["method"] != "POST" || !strings.HasSuffix(last["path"].(string), "/runs") ||
		!strings.Contains(msg, ires.Findings[0].ID) || !strings.Contains(msg, "Suggested fix: return 0 when total == 0") {
		t.Fatalf("follow-up: %d %+v %s\n%v\n%s", code, res, errOut, last, msg)
	}
	n := len(cur.reqs)
	code, res, _ = delegateRun(t, env, opts, set...)
	transcript("sync again on the same head", code, res, "")
	if code != 0 || !strings.Contains(res.Next, "were sent to cursor") || len(cur.reqs) != n+1 { // one GET of the run, no second follow-up
		t.Fatalf("second sync: %d %+v, %d new requests", code, res, len(cur.reqs)-n)
	}

	// The agent pushes a fix; inspect says merge; the story is ready for a human.
	head2 := strings.Repeat("d", 40)
	g.Mu.Lock()
	g.Pulls[pr.Number].Head.SHA = head2
	pr.Diff = cleanDiff
	g.Mu.Unlock()
	writeInspectPack(t, dir, pr.Number, head2, nil)
	model.Replies = []models.Response{inspectReply("merge", "")}
	icode, ires, _ = inspectRun(t, env, opts, model, pr.Number)
	t.Logf("inspect #%d at the fix: exit %d, %s", pr.Number, icode, ires.Disposition)
	code, res, errOut = delegateRun(t, env, opts, set...)
	transcript("sync after inspect merged", code, res, errOut)
	if code != 0 || res.Delegation.Status != dlgReady || !strings.Contains(res.Next, "aayu ship "+strconv.Itoa(pr.Number)) {
		t.Fatalf("merge-ready: %d %+v %s", code, res, errOut)
	}
	_, work, _ := run(t, env, opts, "status", "--work", "--repo", "o/r")
	t.Logf("status --work:\n%s", work)
	if !strings.Contains(work, "cursor") || !strings.Contains(work, "cloud") || !strings.Contains(work, "merge-ready") {
		t.Fatalf("status --work lacks the cloud delegation:\n%s", work)
	}
	if out, _ := os.ReadFile(filepath.Join(dir, ".aayudha", "telemetry", "actions.jsonl")); !strings.Contains(string(out), `"delegate.handoff"`) || !strings.Contains(string(out), `"delegate.follow-up"`) {
		t.Fatalf("actions log lacks the hand-off or the follow-up")
	}
}

// The cap: after inspect's second fix cycle and the receiver's push, inspect
// will not review again; sync pages a human and exits 4 without a follow-up.
func TestDelegateCycleCapPages(t *testing.T) {
	g, env, opts, _, _ := delegateFixture(t)
	cur := &fakeCursor{}
	srv := httptest.NewServer(cur)
	defer srv.Close()
	env["AAYU_SECRET_CURSOR_API_KEY"] = "k"
	set := []string{"--delegate", "cursor", "--set", "delegate.cursor_api=" + srv.URL}
	if code, _, errOut := delegateRun(t, env, opts, set...); code != 0 {
		t.Fatal(errOut)
	}
	pr := g.AddPR("feat: show empty list", "Refs #2", "founder", "cursor/x", strings.Repeat("e", 40), "dev")
	cur.finished = pr.Number
	st := inspect.State{Cycles: inspect.MaxCycles, Result: &inspect.Result{Head: strings.Repeat("0", 40), Disposition: inspect.Fix}}
	j, _ := json.Marshal(st)
	g.Mu.Lock()
	g.Issues[pr.Number].Comments = append(g.Issues[pr.Number].Comments, &fake.Comment{ID: 99, Body: inspect.Marker + "\n<!-- aayudha:inspect-state " + string(j) + " -->"})
	g.Mu.Unlock()
	n := len(cur.reqs)
	code, res, errOut := delegateRun(t, env, opts, set...)
	sink := opts.NotifySink.(*notify.Fake)
	if code != ExitBlocked || !strings.Contains(errOut, "2-cycle cap") || len(sink.Pages) != 1 || res.Delegation.Status != dlgCapped ||
		slices.ContainsFunc(cur.reqs[n:], func(r map[string]any) bool { return r["method"] == "POST" }) {
		t.Fatalf("cap: %d %+v %s, pages %d", code, res, errOut, len(sink.Pages))
	}
}

// The local fallback (--local, or the cloud agent not connected): the
// receiver's CLI runs in .aayudha/worktrees/<n>-<agent> with no GitHub or
// aayu credential in its environment; aayu commits, verifies, pushes, and
// opens the PR with Refs #n.
func TestDelegateLocalFallback(t *testing.T) {
	g, env, opts, dir, bare := delegateFixture(t)
	env["GITHUB_TOKEN"], env["GH_TOKEN"], env["OPENAI_API_KEY"] = "gh-canary", "gh-canary", "openai-own-key"
	opts.LookPath = func(name string) (string, error) {
		if name == "codex" {
			return "/usr/local/bin/codex", nil
		}
		return "", os.ErrNotExist
	}
	var gotEnv []string
	var gotArgs []string
	opts.DelegateExec = func(_ context.Context, wt string, env []string, _, name string, args ...string) (string, error) {
		gotEnv, gotArgs = env, append([]string{name}, args...)
		return "", os.WriteFile(filepath.Join(wt, "empty.txt"), []byte("Your cart is empty\n"), 0o644)
	}
	code, res, errOut := delegateRun(t, env, opts, "--delegate", "codex") // not connected in the cloud: falls back to the CLI
	wt := delegate.Worktree(dir, 2, "codex")
	if code != 0 || res.Worktree != wt || res.PR == 0 || res.Delegation.Mode != "local" || !strings.Contains(res.Delegation.Reason, "not connected") {
		t.Fatalf("local: %d %+v %s", code, res, errOut)
	}
	if gotArgs[0] != "/usr/local/bin/codex" || !slices.Contains(gotArgs, wt) || !slices.Contains(gotArgs, "workspace-write") {
		t.Fatalf("command: %v", gotArgs)
	}
	for _, kv := range gotEnv {
		k, v, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "GITHUB_") || k == "GH_TOKEN" || strings.HasPrefix(k, "AAYU_SECRET") || k == "AAYU_GITHUB_TOKEN" || v == "gh-canary" || v == "human-token" {
			t.Fatalf("the local receiver got %s", k)
		}
	}
	if !slices.Contains(gotEnv, "OPENAI_API_KEY=openai-own-key") {
		t.Fatal("the receiver's own key was dropped")
	}
	msg := git(t, wt, "log", "-1", "--format=%B")
	g.Mu.Lock()
	p := g.Pulls[res.PR]
	g.Mu.Unlock()
	if !strings.Contains(msg, "Aayudha-Delegate: codex") || !strings.Contains(msg, "Signed-off-by:") || !strings.Contains(p.Body, "Refs #2") || !strings.Contains(p.Body, "Built by codex (local") {
		t.Fatalf("commit:\n%s\nPR body:\n%s", msg, p.Body)
	}
	if pushed := git(t, bare, "rev-parse", "refs/heads/story/2-show-empty-list"); pushed != git(t, wt, "rev-parse", "HEAD") {
		t.Fatal("not pushed")
	}
	// status --work lists it: story, agent, worktree, branch, PR.
	code, out, errOut := run(t, env, opts, "status", "--work", "--repo", "o/r", "--json")
	var items []WorkItem
	_ = json.Unmarshal([]byte(out), &items)
	if code != 0 || len(items) != 1 || items[0].Story != 2 || items[0].Agent != "codex" || items[0].Where != ".aayudha/worktrees/2-codex" || items[0].PR != res.PR {
		t.Fatalf("status --work: %d %s %s", code, out, errOut)
	}
}

// Claude Code: the brief as an @claude comment on the story; the action's
// claude/issue-2-… branch, from which aayu opens the PR.
func TestDelegateClaudeMention(t *testing.T) {
	g, env, opts, dir, bare := delegateFixture(t)
	_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "claude.yml"), []byte("jobs:\n  c:\n    steps:\n      - run: bash .agents/setup\n      - uses: anthropics/claude-code-action@v1\n"), 0o644)
	code, res, errOut := delegateRun(t, env, opts, "--delegate", "claude")
	if code != 0 || res.Delegation.Status != dlgHandedOff || !strings.HasPrefix(comment(g, 2, "@claude "), "@claude please build this story") {
		t.Fatalf("hand-off: %d %+v %s", code, res, errOut)
	}
	seed := filepath.Join(t.TempDir(), "s")
	git(t, filepath.Dir(seed), "clone", "--quiet", "--branch", "dev", bare, seed)
	_ = os.WriteFile(filepath.Join(seed, "empty.txt"), []byte("x\n"), 0o644)
	git(t, seed, "add", ".")
	git(t, seed, "commit", "--quiet", "-m", "feat: empty list")
	git(t, seed, "push", "--quiet", "origin", "HEAD:refs/heads/claude/issue-2-20261015-0900")
	code, res, errOut = delegateRun(t, env, opts, "--delegate", "claude")
	g.Mu.Lock()
	p := g.Pulls[res.Delegation.PR]
	g.Mu.Unlock()
	if code != 0 || p == nil || p.Head.Ref != "claude/issue-2-20261015-0900" || !strings.HasPrefix(p.Body, "Refs #2") {
		t.Fatalf("PR from the branch: %d %+v %s", code, res, errOut)
	}
}

// Codex cloud: the task's returned diff is applied in an aayu worktree, then
// committed, verified, pushed, and opened as the PR.
func TestDelegateCodexDiffReturn(t *testing.T) {
	_, env, opts, dir, _ := delegateFixture(t)
	opts.LookPath = func(name string) (string, error) {
		if name == "codex" {
			return "/bin/codex", nil
		}
		return "", os.ErrNotExist
	}
	read := func(p string) string {
		b, _ := os.ReadFile(filepath.Join("..", "delegate", "testdata", "codex", p))
		return string(b)
	}
	status := read("status-running.txt")
	diff := "diff --git a/empty.txt b/empty.txt\nnew file mode 100644\n--- /dev/null\n+++ b/empty.txt\n@@ -0,0 +1 @@\n+Your cart is empty\n"
	opts.DelegateExec = func(_ context.Context, _ string, _ []string, _, _ string, args ...string) (string, error) {
		switch strings.Join(args[:2], " ") {
		case "cloud exec":
			return read("exec.txt"), nil
		case "cloud status":
			return status, nil
		case "cloud diff":
			return diff, nil
		}
		return "", nil
	}
	set := []string{"--delegate", "codex", "--set", "delegate.codex_env=env_123"}
	if code, res, errOut := delegateRun(t, env, opts, set...); code != 0 || res.Delegation.Mode != "cloud" || res.Delegation.Task != "task_e_68f0a1b2c3d4e5f6a7b8c9d0" {
		t.Fatalf("hand-off: %d %+v %s", code, res, errOut)
	}
	status = read("status-done.txt")
	code, res, errOut := delegateRun(t, env, opts, set...)
	wt := delegate.Worktree(dir, 2, "codex")
	if code != 0 || res.PR == 0 || res.Worktree != wt || !strings.Contains(git(t, wt, "log", "-1", "--format=%B"), "Aayudha-Delegate: codex") {
		t.Fatalf("diff return: %d %+v %s", code, res, errOut)
	}
	if b, _ := os.ReadFile(filepath.Join(wt, "empty.txt")); string(b) != "Your cart is empty\n" {
		t.Fatalf("diff not applied: %q", b)
	}
}
