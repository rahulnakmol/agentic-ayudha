package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// buildLoopFixture is a checkout with the build-ready loop defined and an
// agent session, whose build --next runs return what they decided.
func buildLoopFixture(t *testing.T, g *fake.GitHub) (next func(extra map[string]string, args ...string) (int, BuildResult, BuildPick, string), env map[string]string, opts Options, dir, bare string) {
	t.Helper()
	dir, bare = ledgerCheckout(t)
	b, err := os.ReadFile(filepath.Join("..", "..", "loops", BuildLoop+".toml"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", BuildLoop+".toml"), b, 0o644)
	// The loop's definition and [build] are read from dev's tip on GitHub.
	g.Mu.Lock()
	g.Refs["heads/dev"] = devTip
	if g.RefFiles == nil {
		g.RefFiles = map[string]map[string]string{}
	}
	g.RefFiles[devTip] = map[string]string{"loops/" + BuildLoop + ".toml": string(b), ".aayudha/config.toml": "[build]\nagents = 1\n"}
	g.Mu.Unlock()
	// An agent session: the asked gate holds every story the person did not name.
	env = map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "CODEX_THREAD_ID": "loop-run"}
	opts = Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, CheckpointVerify: holds}
	next = func(extra map[string]string, args ...string) (int, BuildResult, BuildPick, string) {
		t.Helper()
		e := map[string]string{}
		for k, v := range env {
			e[k] = v
		}
		for k, v := range extra {
			e[k] = v
		}
		code, out, errOut := run(t, e, opts, append([]string{"build", "--repo", "o/r", "--json"}, args...)...)
		var res BuildResult
		var pick BuildPick
		_ = json.Unmarshal([]byte(out), &res)
		_ = json.Unmarshal([]byte(out), &pick)
		if res.Pick != nil {
			pick = *res.Pick
		}
		return code, res, pick, out + errOut
	}
	return next, env, opts, dir, bare
}

const devTip = "d0000000000000000000000000000000000000de"

// commitOnDev sets a file at dev's tip, as merged; "" removes it.
func commitOnDev(g *fake.GitHub, path, content string) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if content == "" {
		delete(g.RefFiles[devTip], path)
		return
	}
	g.RefFiles[devTip][path] = content
}

func readyAll(t *testing.T, g *fake.GitHub, env map[string]string, opts Options, ns ...int) {
	t.Helper()
	g.Mu.Lock() // a Story is Ready only under a Feature approved to build (Story #996)
	for _, n := range ns {
		if g.Issues[n].Parent == 0 {
			g.Issues[n].Parent, g.Issues[1].Subs = 1, append(g.Issues[1].Subs, n)
		}
	}
	g.Mu.Unlock()
	approveFor(t, g, opts.Dir, 1)
	person := map[string]string{"AAYU_GITHUB_TOKEN": env["AAYU_GITHUB_TOKEN"], "PATH": env["PATH"]}
	for _, n := range ns {
		if code, _, errOut := run(t, person, opts, "ready", strconv.Itoa(n), "--repo", "o/r"); code != 0 {
			t.Fatalf("ready %d: %s", n, errOut)
		}
	}
}

func hasLine(xs []string, prefix, part string) bool {
	return slices.ContainsFunc(xs, func(x string) bool { return strings.HasPrefix(x, prefix) && strings.Contains(x, part) })
}

// The build-ready loop with no accepted baseline: build --next pick claims the
// first free Ready item by priority, then age, never one that drifted, has an
// open PR, or another's live claim; the asked gate lets the loop's own pick
// through; [build] agents counts the loop's open PRs as well as its claims; a claim
// whose lease ran out with no PR is blocked, never picked again; --stop blocks
// a build with its reason; ship lists the loop's open PRs and changes nothing.
func TestBuildNextPicksClaimsAndLimits(t *testing.T) {
	g := readyTree(t)
	body := fixtureBody(t, "story", nil)
	low := g.AddIssue("No priority", body, "type:story", "effort:S")                    // #7: lowest number, no priority
	newer := g.AddIssue("P1 newer", body, "type:story", "effort:S", "priority:P1")      // #8
	older := g.AddIssue("P1 older", body, "type:story", "effort:S", "priority:P1")      // #9: higher number, created first
	drifted := g.AddIssue("P0 drifted", body, "type:story", "effort:S", "priority:P0")  // #10
	withPR := g.AddIssue("P0 with a PR", body, "type:story", "effort:S", "priority:P0") // #11
	claimed := g.AddIssue("P0 claimed", body, "type:story", "effort:S", "priority:P0")  // #12
	g.Mu.Lock()
	older.CreatedAt = "2026-10-01T09:00:00Z"
	g.Mu.Unlock()
	next, env, opts, dir, bare := buildLoopFixture(t, g)

	// Without the loop's definition on dev there is no standing ask: --next
	// refuses, though the checkout has the file.
	b, _ := os.ReadFile(filepath.Join("..", "..", "loops", BuildLoop+".toml"))
	commitOnDev(g, "loops/"+BuildLoop+".toml", "")
	if code, _, _, out := next(nil, "--next", "pick"); code != ExitUsage || !strings.Contains(out, "dev has no loops/build-ready.toml") {
		t.Fatalf("--next without the loop on dev: %d %s", code, out)
	}
	commitOnDev(g, "loops/"+BuildLoop+".toml", string(b))

	readyAll(t, g, env, opts, low.Number, newer.Number, older.Number, drifted.Number, withPR.Number, claimed.Number)
	g.Mu.Lock()
	drifted.Body += "\nOne more line after Ready."
	claimed.Comments = append(claimed.Comments, &fake.Comment{ID: 901, Body: leaseBody("other@laptop", time.Now().Add(time.Hour))})
	claimed.Labels = append(claimed.Labels, fake.Label{Name: LeaseLabel})
	g.Mu.Unlock()
	g.AddPR("Their PR", "Refs #11\n", "someone", "feat/x", strings.Repeat("a", 40), "dev")

	// Another pick on this machine holds the lock: this one claims nothing. A
	// lock left by a run that died is taken over once it is old.
	lock := filepath.Join(dir, ".aayudha", "state", "build-next.lock")
	_ = os.MkdirAll(filepath.Dir(lock), 0o755)
	_ = os.WriteFile(lock, []byte("pid 1\n"), 0o644)
	if code, res, _, out := next(nil, "--next", "pick"); code != ExitBlocked || res.Issue != 0 || !strings.Contains(out, "another build --next run on this machine") || contains(g.LabelNames(older.Number), LeaseLabel) {
		t.Fatalf("pick under a live lock: %d %s", code, out)
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(lock, old, old)
	// #12 is Ready with a live lease: a claim another pick is still setting up.
	// It holds the one slot, so this pick claims nothing.
	if code, res, pick, out := next(nil, "--next", "pick"); code != 0 || res.Issue != 0 || len(pick.InFlight) != 1 || pick.InFlight[0].Story != claimed.Number ||
		!strings.Contains(pick.Reason, "counting fresh claims") {
		t.Fatalf("a fresh claim does not hold a slot: %d %s", code, out)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatalf("the pick kept its lock: %v", err)
	}
	g.Mu.Lock()
	claimed.Labels = slices.DeleteFunc(claimed.Labels, func(l fake.Label) bool { return l.Name == "status:ready" })
	claimed.Labels = append(claimed.Labels, fake.Label{Name: labelInProgress}) // its builder took it
	g.Mu.Unlock()
	// --dry-run previews the pick on a ledger with no claims file yet, and
	// claims nothing.
	// Dev's committed agents holds over the embedded default (5), which is no local value.
	commitOnDev(g, ".aayudha/config.toml", "[build]\nagents = 7\n")
	if code, _, pick, out := next(nil, "--next", "pick", "--dry-run"); code != 0 || pick.Limit != 7 {
		t.Fatalf("--next pick --dry-run under dev's agents 7: %d %s", code, out)
	}
	commitOnDev(g, ".aayudha/config.toml", "[build]\nagents = 2\n")
	if code, _, pick, out := next(nil, "--next", "pick", "--dry-run"); code != 0 || pick.Picked != older.Number || contains(g.LabelNames(older.Number), LeaseLabel) {
		t.Fatalf("--next pick --dry-run: %d %s", code, out)
	}
	commitOnDev(g, ".aayudha/config.toml", "[build]\nagents = 1\n")
	// The ledger orders picks across machines: a recent claim line of another
	// pick holds the one slot, and one for the same story was first.
	claimLine := func(story int, at time.Time) string {
		return `{"story":` + strconv.Itoa(story) + `,"token":"elsewhere","at":"` + at.UTC().Format(time.RFC3339) + `"}` + "\n"
	}
	for _, tc := range []struct {
		story int
		want  string
	}{{99, "another pick took the last of 1 slot(s) first (#99"}, {older.Number, "another pick claimed #9 first"}} {
		putLedger(t, bare, LoopClaimsPath, claimLine(tc.story, time.Now()))
		if code, res, pick, out := next(nil, "--next", "pick"); code != 0 || res.Issue != 0 || !strings.HasPrefix(pick.Reason, tc.want) || contains(g.LabelNames(older.Number), LeaseLabel) {
			t.Fatalf("a pick ordered after another's claim: %d %s", code, out)
		}
		if lines := git(t, bare, "show", "ledger:"+LoopClaimsPath); !strings.Contains(lines, `"release":true`) {
			t.Fatalf("the losing pick did not release its line:\n%s", lines)
		}
	}
	putLedger(t, bare, LoopClaimsPath, claimLine(99, time.Now().Add(-time.Hour))) // a pick that died long ago holds nothing
	// A loop build takes its budget from dev's committed config: a local
	// override may tighten a limit, never loosen or turn it off. Nor does an
	// override move its worktree's base or its PR's base off the trusted
	// branch: feat/mine does not exist yet, so a build cut from it fails.
	loosen := map[string]string{"AAYU_AUTONOMY__BUDGET__WALL_HOURS": "0", "AAYU_AUTONOMY__BUDGET__ATTEMPTS": "9", "AAYU_AUTONOMY__BUDGET__COST_USD": "1", "AAYU_BRANCHES__DEV": "feat/mine"}
	code, res, pick, out := next(loosen, "--next", "pick")
	if code != 0 || res.Issue != older.Number || pick.Picked != older.Number || !res.Created {
		t.Fatalf("first pick: want #%d, got %d %+v %s", older.Number, code, pick, out)
	}
	if res.Budget.MaxWall != 8 || res.Budget.MaxAttempts != 3 || res.Budget.MaxCost != 1 {
		t.Fatalf("a local override loosened the loop build's committed budget: %+v", res.Budget)
	}
	for _, want := range []struct {
		n   int
		why string
	}{{drifted.Number, "body changed after Ready"}, {withPR.Number, "open pull request"}} {
		if !hasLine(pick.Skipped, "#"+strconv.Itoa(want.n)+":", want.why) {
			t.Fatalf("skipped %v lacks #%d (%s)", pick.Skipped, want.n, want.why)
		}
	}
	if labels := g.LabelNames(older.Number); !contains(labels, LeaseLabel) || !contains(labels, labelInProgress) {
		t.Fatalf("the pick is not claimed: %v", labels)
	}
	var st BuildState
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(older.Number))), &st); err != nil || st.Loop != BuildLoop {
		t.Fatalf("ledger state of the pick: %v %+v", err, st)
	}
	if log, _ := os.ReadFile(filepath.Join(dir, ".aayudha", "telemetry", "actions.jsonl")); !strings.Contains(string(log), "the build-ready loop picked #9") {
		t.Fatalf("the loop's ask is not in the actions log:\n%s", log)
	}

	// One in flight, the default limit: nothing new, and the queue is untouched.
	if code, res, pick, out := next(nil, "--next", "pick"); code != 0 || res.Issue != 0 || pick.Reason != "1 of 1 in flight: nothing new until one merges or stops" {
		t.Fatalf("second pick at the limit: %d %s", code, out)
	}
	if contains(g.LabelNames(newer.Number), LeaseLabel) {
		t.Fatal("a pick at the limit claimed another item")
	}
	// ship lists the loop's open PRs only, and its PR still counts in flight. An
	// older PR that names the story from another branch is never the loop's.
	g.AddPR("Someone else's", "Refs #9\n", "someone", "feat/other", strings.Repeat("c", 40), "dev")
	if code, _, pick, out := next(nil, "--next", "ship"); code != 0 || len(pick.Ship) != 0 {
		t.Fatalf("ship with no loop PR: %d %s", code, out)
	}
	pr := g.AddPR("P1 older", "Refs #9\n", "founder", "story/9-p1-older", strings.Repeat("b", 40), "dev")
	if code, _, pick, out := next(nil, "--next", "ship"); code != 0 || len(pick.Ship) != 1 || pick.Ship[0].PR != pr.Number || pick.Ship[0].Story != older.Number {
		t.Fatalf("ship lists the loop's PR: %d %s", code, out)
	}
	if code, _, pick, out := next(nil, "--next", "pick"); code != 0 || pick.Picked != 0 || len(pick.InFlight) != 1 || pick.InFlight[0].PR != pr.Number {
		t.Fatalf("the loop's open PR counts in flight: %d %s", code, out)
	}
	// Its story is status:blocked though the ledger did not record the block
	// (the write failed): the PR is not shipped, and it keeps its slot.
	g.Mu.Lock()
	g.Issues[older.Number].Labels = append(g.Issues[older.Number].Labels, fake.Label{Name: "status:blocked"})
	g.Mu.Unlock()
	if code, _, pick, out := next(nil, "--next", "ship"); code != 0 || len(pick.Ship) != 0 || len(pick.InFlight) != 1 {
		t.Fatalf("a status:blocked story's PR was listed to ship: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Issues[older.Number].Labels = slices.DeleteFunc(g.Issues[older.Number].Labels, func(l fake.Label) bool { return l.Name == "status:blocked" })
	g.Mu.Unlock()
	// Its story closed while the PR is open: the PR is still the loop's to
	// ship, and still in flight.
	g.Mu.Lock()
	g.Issues[older.Number].State = "closed"
	g.Mu.Unlock()
	if code, _, pick, out := next(nil, "--next", "ship"); code != 0 || len(pick.Ship) != 1 || pick.Ship[0].PR != pr.Number || len(pick.InFlight) != 1 {
		t.Fatalf("a closed story's open PR left the loop: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Issues[older.Number].State = "open"
	g.Mu.Unlock()
	// Its builder stops it: the open PR is not the loop's to ship any more,
	// but it still holds its slot until a person settles it.
	if code, _, _, out := next(nil, strconv.Itoa(older.Number), "--stop", "the PR needs a person"); code != ExitBlocked {
		t.Fatalf("--stop on the loop's PR story: %d %s", code, out)
	}
	if code, _, pick, out := next(nil, "--next", "ship"); code != 0 || len(pick.Ship) != 0 || len(pick.InFlight) != 1 || pick.InFlight[0].PR != pr.Number {
		t.Fatalf("a blocked story's open PR left the in-flight count: %d %s", code, out)
	}
	// A person makes it Ready again and its build resumes: it is still the
	// loop's build, and its PR is the loop's to ship again.
	readyAll(t, g, env, opts, older.Number)
	person := map[string]string{"AAYU_GITHUB_TOKEN": env["AAYU_GITHUB_TOKEN"], "PATH": env["PATH"]}
	_, _, _ = run(t, person, opts, "build", strconv.Itoa(older.Number), "--repo", "o/r", "--resume")
	if code, _, pick, out := next(nil, "--next", "ship"); code != 0 || len(pick.Ship) != 1 || pick.Ship[0].PR != pr.Number {
		t.Fatalf("a resumed loop build left the loop: %d %s", code, out)
	}
	// [build] agents 0 stops the picks.
	if code, _, pick, _ := next(map[string]string{"AAYU_BUILD__AGENTS": "0"}, "--next", "pick"); code != 0 || pick.Picked != 0 || !strings.Contains(pick.Reason, "agents is 0") {
		t.Fatalf("agents 0: %d %+v", code, pick)
	}

	// The cap is dev's committed [build] agents: a local AAYU_* or --set may lower
	// it, never raise it.
	two := map[string]string{"AAYU_BUILD__AGENTS": "2"}
	if code, res, pick, out := next(two, "--next", "pick"); code != 0 || res.Issue != 0 || pick.Reason != "1 of 1 in flight: nothing new until one merges or stops" {
		t.Fatalf("a local agents value raised the committed cap: %d %s", code, out)
	}
	// The integration branch is never the run's to name: AAYU_BRANCHES__DEV
	// pointing at a branch the agent can push does not lend it that branch's cap.
	g.Mu.Lock()
	g.Refs["heads/feat/mine"] = "f0000000000000000000000000000000000000fe"
	g.RefFiles["f0000000000000000000000000000000000000fe"] = map[string]string{"loops/" + BuildLoop + ".toml": g.RefFiles[devTip]["loops/"+BuildLoop+".toml"], ".aayudha/config.toml": "[build]\nagents = 2\n"}
	g.Mu.Unlock()
	if code, res, pick, out := next(map[string]string{"AAYU_BUILD__AGENTS": "2", "AAYU_BRANCHES__DEV": "feat/mine"}, "--next", "pick"); code != 0 || res.Issue != 0 || pick.Reason != "1 of 1 in flight: nothing new until one merges or stops" {
		t.Fatalf("a run-named branch lent the loop its cap: %d %s", code, out)
	}
	// The person names it: in the account's user config, or in the default
	// branch's committed [branches] dev. Either way its cap counts.
	home := t.TempDir()
	userCfg := filepath.Join(home, ".config", "aayudha", "config.toml")
	_ = os.MkdirAll(filepath.Dir(userCfg), 0o755)
	_ = os.WriteFile(userCfg, []byte("[branches]\ndev = \"feat/mine\"\n"), 0o644)
	if code, _, pick, out := next(map[string]string{"HOME": home, "AAYU_BUILD__AGENTS": "2"}, "--next", "ship"); code != 0 || pick.Limit != 2 {
		t.Fatalf("the user config's integration branch is not trusted: %d %s", code, out)
	}
	// The account's user config is read even when the run's HOME is elsewhere.
	_ = os.WriteFile(userCfg, []byte("[branches\n"), 0o644)
	acct := opts
	acct.UserHome = func() (string, error) { return home, nil }
	if code, _, errOut := run(t, env, acct, "build", "--repo", "o/r", "--next", "ship"); code != ExitUsage || !strings.Contains(errOut, "config:") {
		t.Fatalf("a broken account user config: %d %s", code, errOut)
	}
	g.Mu.Lock()
	g.Default, g.Refs["heads/main"] = "main", "a000000000000000000000000000000000000a00"
	g.RefFiles["a000000000000000000000000000000000000a00"] = map[string]string{".aayudha/config.toml": "[branches]\ndev = \"feat/mine\"\n"}
	g.Mu.Unlock()
	if code, _, pick, out := next(two, "--next", "ship"); code != 0 || pick.Limit != 2 {
		t.Fatalf("the default branch's mapped integration branch is not trusted: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r", 0
	g.Mu.Unlock()
	if code, _, _, out := next(nil, "--next", "ship"); code == 0 || !strings.Contains(out, "500") {
		t.Fatalf("a loop run when the default branch cannot be read: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Fail, g.Default = "", "dev"
	delete(g.Refs, "heads/main")
	g.Mu.Unlock()
	commitOnDev(g, ".aayudha/config.toml", "[build]\nagents = 2\n")
	if code, res, _, out := next(two, "--next", "pick"); code != 0 || res.Issue != newer.Number {
		t.Fatalf("pick under agents 2: want #%d: %d %s", newer.Number, code, out)
	}
	// The claim on #8 runs out with no PR: the next pick blocks it, reports it, and takes the next item.
	// Within the grace period after its lease ran out, the claim may be a
	// builder about to resume: it keeps its slot and is not blocked.
	setLease(g, newer.Number, "founder@gone", time.Now().Add(-time.Hour))
	if code, res, pick, out := next(two, "--next", "pick"); code != 0 || res.Issue != 0 || len(pick.Stale) != 0 || len(pick.InFlight) != 2 || contains(g.LabelNames(newer.Number), "status:blocked") {
		t.Fatalf("a claim within the grace period was blocked or lost its slot: %d %s", code, out)
	}
	// gc reclaimed a lease that was renewed long after the last checkpoint: the
	// grace runs from the released lease's expiry, not from the checkpoint.
	later := opts
	later.Now = func() time.Time { return time.Now().Add(9 * time.Hour) }
	setLease(g, newer.Number, "founder@gone", time.Now().Add(6*time.Hour))
	if code, out, errOut := run(t, env, later, "gc", "--repo", "o/r"); code != 0 || !strings.Contains(out, "lease #8 reclaimed") {
		t.Fatalf("gc: %d %s %s", code, out, errOut)
	}
	envTwo := map[string]string{"AAYU_BUILD__AGENTS": "2"}
	for k, v := range env {
		envTwo[k] = v
	}
	if code, out, errOut := run(t, envTwo, later, "build", "--repo", "o/r", "--json", "--next", "pick"); code != 0 || strings.Contains(out, "claim ran out") || contains(g.LabelNames(newer.Number), "status:blocked") {
		t.Fatalf("a claim gc released within the grace period was blocked: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	for _, c := range g.Issues[newer.Number].Comments {
		if releasedTag.MatchString(c.Body) {
			c.Body = leaseBody("founder@gone", time.Now())
		}
	}
	g.Mu.Unlock()
	setLease(g, newer.Number, "founder@gone", time.Now().Add(-StaleGrace-time.Minute))
	// A dry run shows the stale claim it would block, and blocks and pages nothing.
	notifyState := filepath.Join(dir, ".aayudha", "state", "notify.json")
	before, _ := os.ReadFile(notifyState)
	if code, _, pick, out := next(two, "--next", "pick", "--dry-run"); code != 0 || !hasLine(pick.Stale, "#8:", "would be blocked") || contains(g.LabelNames(newer.Number), "status:blocked") {
		t.Fatalf("--dry-run over a stale claim: %d %s", code, out)
	}
	if after, _ := os.ReadFile(notifyState); string(after) != string(before) {
		t.Fatalf("a dry run wrote the notification state:\n%s", after)
	}
	code, res, pick, out = next(two, "--next", "pick")
	if code != 0 || res.Issue != low.Number || !hasLine(pick.Stale, "#8:", "claim ran out with no pull request") {
		t.Fatalf("stale claim: want #8 blocked and #%d picked: %d %s", low.Number, code, out)
	}
	if labels := g.LabelNames(newer.Number); !contains(labels, "status:blocked") || contains(labels, "status:ready") {
		t.Fatalf("the stale claim is not blocked: %v", labels)
	}
	if p := comment(g, newer.Number, progressMarker); !strings.Contains(p, "**Blocked:** the build-ready loop's claim ran out") {
		t.Fatalf("the stale claim's reason is not on the story:\n%s", p)
	}

	// Another builder's live lease is a build at work: --stop leaves it alone.
	setLease(g, low.Number, "other@laptop", time.Now().Add(time.Hour))
	if code, _, _, out := next(nil, "7", "--stop", "not mine"); code != ExitBlocked || !strings.Contains(out, "other@laptop holds a live lease") || contains(g.LabelNames(low.Number), "status:blocked") {
		t.Fatalf("--stop over another's live lease: %d %s", code, out)
	}
	setLease(g, low.Number, "other@laptop", time.Now().Add(-time.Minute))
	// A dry run of --stop says what it would do, and blocks and pages nothing.
	before, _ = os.ReadFile(notifyState)
	if code, _, _, out := next(nil, "7", "--stop", "not yet", "--dry-run"); code != ExitBlocked || !strings.Contains(out, "would block") || contains(g.LabelNames(low.Number), "status:blocked") {
		t.Fatalf("--stop --dry-run: %d %s", code, out)
	}
	if after, _ := os.ReadFile(notifyState); string(after) != string(before) {
		t.Fatalf("--stop --dry-run wrote the notification state:\n%s", after)
	}
	// The builder gives up on #7: --stop blocks it with the reason, and it leaves the in-flight count.
	if code, _, _, out := next(nil, "7", "--stop", "the tests need a database this machine lacks"); code != ExitBlocked || !strings.Contains(out, "stopped by its builder: the tests need a database") {
		t.Fatalf("--stop: %d %s", code, out)
	}
	if labels := g.LabelNames(low.Number); !contains(labels, "status:blocked") {
		t.Fatalf("--stop did not block: %v", labels)
	}
	if code, res, pick, out := next(two, "--next", "pick"); code != 0 || res.Issue != 0 || len(pick.InFlight) != 1 || !strings.HasPrefix(pick.Reason, "nothing to build") {
		t.Fatalf("after the stop, nothing free is left: %d %s", code, out)
	}
	if code, _, _, out := next(nil, "--next", "pick", "7"); code != ExitUsage {
		t.Fatalf("--next with an issue: %d %s", code, out)
	}
}

// With an accepted baseline for the open Release of the lowest version, the
// loop builds only that baseline's Stories and Bugs, in the baseline's order,
// whatever their priority; a Ready item outside it is never picked; an older
// baseline of the same version and a closed Release's baseline are ignored.
func TestBuildNextFollowsTheCurrentBaseline(t *testing.T) {
	g := readyTree(t)
	body := fixtureBody(t, "story", nil)
	first := g.AddIssue("P0 second in the baseline", body, "type:story", "effort:S", "priority:P0") // #7
	second := g.AddIssue("P3 first in the baseline", body, "type:story", "effort:S", "priority:P3") // #8
	outside := g.AddIssue("P0 outside", body, "type:story", "effort:S", "priority:P0")              // #9
	rel := g.AddIssue("Release v0.3.0", "x", "type:release")                                        // #10
	old := g.AddIssue("Release v0.2.0", "x", "type:release")                                        // #11
	g.Mu.Lock()
	old.State = "closed"
	g.Mu.Unlock()
	next, env, opts, _, bare := buildLoopFixture(t, g)
	readyAll(t, g, env, opts, first.Number, second.Number, outside.Number)
	baseline := func(version string, release, n int, items ...int) string {
		var its []map[string]any
		for _, i := range items {
			its = append(its, map[string]any{"number": i, "type": "story"})
		}
		its = append(its, map[string]any{"number": 1, "type": "feature"})
		b, _ := json.Marshal(map[string]any{"schema": "aayudha.scope.baseline/v1", "version": version, "release_issue": release, "baseline": n, "items": its})
		return string(b)
	}
	putLedger(t, bare, "releases/v0.2.0/baseline-1.json", baseline("v0.2.0", old.Number, 1, outside.Number))
	putLedger(t, bare, "releases/v0.3.0/baseline-1.json", baseline("v0.3.0", rel.Number, 1, outside.Number))
	putLedger(t, bare, "releases/v0.3.0/baseline-2.json", baseline("v0.3.0", rel.Number, 2, second.Number, first.Number))

	commitOnDev(g, ".aayudha/config.toml", "[build]\nagents = 3\n")
	three := map[string]string{"AAYU_BUILD__AGENTS": "3"}
	for _, want := range []int{second.Number, first.Number} {
		code, res, pick, out := next(three, "--next", "pick")
		if code != 0 || res.Issue != want || pick.Release == nil || pick.Release.Version != "v0.3.0" || pick.Release.Baseline != 2 {
			t.Fatalf("baseline pick: want #%d: %d %s", want, code, out)
		}
	}
	code, res, pick, out := next(three, "--next", "pick")
	if code != 0 || res.Issue != 0 || !strings.HasPrefix(pick.Reason, "nothing to build") || contains(g.LabelNames(outside.Number), LeaseLabel) {
		t.Fatalf("an item outside the baseline was picked, or the queue did not end: %d %s", code, out)
	}
}
