package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// inboxRepo seeds one human-only step of each kind (Story #431, criterion 2)
// and two items that are not steps: a Story whose PR is still open and a
// Story with no PR.
func inboxRepo(t *testing.T) *fake.GitHub {
	g := fake.New(t, "o/r", "User")
	t.Helper()
	tw, _ := workitems.FindType("decision")
	title, fields := decide.WaiverItem("eval-bar", 10, "")
	g.AddIssue(title, workitems.Render(tw, fields), "type:decision", "status:planned", workitems.LabelWaiver) // #1
	prov := strings.Replace(decisionBody("true", ""), "### Links", "### Decision\n\nProvisional: the recommendation (A) applies. Ratify or revert by 2026-10-22T09:00:00Z (Thu) with `aayu decide 2 --accept <option>`.\n\n### Links", 1)
	g.AddIssue("Choose the rules engine", prov, "type:decision", "status:validation", workitems.LabelProvisional)   // #2
	g.AddIssue("Old story", "x", "type:story", "status:backlog")                                                    // #3
	g.AddIssue("New story", "Replaces #3, which lacked the Ready fields.", "type:story", "status:ready")            // #4
	g.AddIssue("Merged story", "x", "type:story", "status:in-progress", LeaseLabel)                                 // #5
	merged := g.AddPR("feat: merged story", "Refs #5", "o", "story/5-merged-story", strings.Repeat("a", 40), "dev") // #6
	g.AddIssue("A bug", "x", "type:bug", "status:planned")                                                          // #7
	fix := g.AddPR("fix: the bug", "Refs #7", "o", "story/7-a-bug", strings.Repeat("c", 40), "dev")                 // #8
	capped := g.AddPR("fix(cli): capped", "Refs #170", "o", "fix/170", strings.Repeat("d", 40), "dev")              // #9
	g.AddIssue("Story in review", "x", "type:story", "status:in-progress")                                          // #10
	g.AddPR("feat: in review", "Refs #10", "o", "story/10-in-review", strings.Repeat("e", 40), "dev")               // #11
	g.Mu.Lock()
	for _, pr := range []*fake.PR{merged, fix} {
		pr.State, pr.Merged, pr.MergeCommitSHA = "closed", true, strings.Repeat("f", 40)
		g.Issues[pr.Number].State = "closed"
	}
	st, _ := json.Marshal(inspect.State{Cycles: inspect.MaxCycles, Result: &inspect.Result{Disposition: "fix"}})
	g.Issues[capped.Number].Comments = append(g.Issues[capped.Number].Comments, &fake.Comment{ID: 77, Body: inspect.Marker + "\n### inspect: fix\n\n<!-- aayudha:inspect-state " + string(st) + " -->\n"})
	g.Mu.Unlock()
	return g
}

func TestInbox(t *testing.T) {
	tty := func(any) bool { return true }
	newEnv := func() map[string]string {
		return map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": t.TempDir()}
	}

	// Criterion 1: refused for agents and without a terminal, before any write.
	g := inboxRepo(t)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, IsTerminal: tty, Assets: aayudha.Assets}
	agent := newEnv()
	agent["AAYU_AGENT"] = "1"
	if code, _, errOut := runIn(t, agent, opts, "a", "inbox", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "inbox is human-only") {
		t.Errorf("agent: exit %d: %s", code, errOut)
	}
	if code, _, errOut := runIn(t, newEnv(), Options{GitHubAPI: g.URL(), GitHubGap: -1}, "a", "inbox", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "needs a terminal") {
		t.Errorf("no terminal: exit %d: %s", code, errOut)
	}
	if w := g.Writes(); len(w) != 0 {
		t.Fatalf("refused inbox wrote: %v", w)
	}

	// Criteria 2 and 3: the six steps, in order, each with its options and its
	// choices in words; accept the recommendation on each (the waiver, a
	// one-way door, after its confirmation).
	env := newEnv()
	code, out, errOut := runIn(t, env, opts, "ay aaaaa", "inbox", "--repo", "o/r")
	if code != 0 {
		t.Fatalf("inbox: exit %d: %s\n%s", code, errOut, out)
	}
	for _, want := range []string{
		"o/r: 6 human-only step(s) wait for you, @o. Each step lists its choices.",
		"[1/6] Decision #1: Waive the eval-bar floor on #10\n  a floor waiver: only a person accepts it, and it is never applied provisionally (rule 7).\n  1. Waive\n  2. Don't waive (recommended)\n" +
			"  a or Enter = accept the recommendation (option 2) · 1-2 = accept that option · s = skip for now · o = open on GitHub · q = quit\n  > a\n" +
			"  This accepts option 2 (Don't waive) as the answer to Decision #1 and closes it; it is a one-way door, so it is not undone later. Apply it? y = yes · n = no, go back\n  > y\n",
		"[2/6] Provisional Decision #2: Choose the rules engine\n  its recommendation applies provisionally (V8): ratify it by accepting it, or revert to another option, by 2026-10-22 09:00 UTC.\n  1. A (recommended)\n  2. B\n" +
			"  a or Enter = accept the recommendation (option 1) · 1-2 = accept that option · s = skip for now · o = open on GitHub · q = quit\n  > a\n#2 accepted: A, by @o",
		"[3/6] PR at inspect's cycle cap #9: fix(cli): capped\n  inspect ran 2 of 2 fix/rebuild cycles and stopped (last disposition: fix); a person reviews it before anything else runs.\n" +
			"  Drona (Engineering Manager) recommends option 1.\n  1. open it and review it (recommended)\n  2. ship PR #9 now (ship's gates still apply)\n" +
			"  a or Enter = do the recommendation (option 1) · 1-2 = do that option · s = skip for now · o = open on GitHub · q = quit\n",
		"[4/6] Story merged without ship #5: Merged story\n  PR #6 merged into dev (fffffff) without `aayu ship`, so no ship record or human pass exists; the Story is still status:in-progress.\n  Vidhi (Auditor) recommends: mark #5 merged without proof (it stays uncovered)\n" +
			"  a or Enter = mark #5 merged without proof (it stays uncovered) · s = skip for now · o = open on GitHub · q = quit\n",
		"[5/6] Bug with its fix merged #7: A bug\n  its fix, PR #8, merged into dev (fffffff), and the Bug is still open.\n  Drona (Engineering Manager) recommends: close #7 as completed, fixed in #8\n" +
			"  a or Enter = close #7 as completed, fixed in #8 · s = skip for now · o = open on GitHub · q = quit\n",
		"[6/6] Replaced #3: Old story\n  #4 says it replaces #3, and #3 is still open.\n  Arjun (Product Strategist) recommends: close #3 as not planned, replaced by #4\n" +
			"  a or Enter = close #3 as not planned, replaced by #4 · s = skip for now · o = open on GitHub · q = quit\n",
		"inbox: 5 done, 0 skipped, 0 left",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inbox output lacks %q:\n%s", want, out)
		}
	}
	for _, not := range []string{"#10:", "#4:", "#11:"} {
		if strings.Contains(out, not) {
			t.Errorf("inbox lists %s, which is not a human-only step:\n%s", not, out)
		}
	}
	g.Mu.Lock()
	is := func(n int) fake.Issue { return *g.Issues[n] }
	if d := is(1); d.State != "closed" || !strings.Contains(d.Body, "Don't waive, accepted by @o") {
		t.Errorf("#1 after accept: %s\n%s", d.State, d.Body)
	}
	if s := is(3); s.State != "closed" || s.StateReason != "not_planned" || len(s.Comments) != 1 || !strings.Contains(s.Comments[0].Body, "Replaced by #4. Closed as not planned by @o") {
		t.Errorf("#3 after accept: %s %s %+v", s.State, s.StateReason, s.Comments)
	}
	if s := is(5); s.State != "open" || len(s.Comments) != 1 || !strings.HasPrefix(s.Comments[0].Body, PostGateMarker+"\n**Merged without proof (post-gate).** This story's change merged in #6 (`fffffff`)") {
		t.Errorf("#5 after accept: %s %+v", s.State, s.Comments)
	}
	if b := is(7); b.State != "closed" || b.StateReason != "completed" || len(b.Comments) != 1 || !strings.Contains(b.Comments[0].Body, "Fixed in #8 (`fffffff`, merged into dev). Closed as completed by @o") {
		t.Errorf("#7 after accept: %s %s %+v", b.State, b.StateReason, b.Comments)
	}
	if b := is(9); len(b.Comments) != 1 || g.Pulls[9].State != "open" {
		t.Errorf("accepting the capped PR's recommendation (open it) wrote to it: %+v", b.Comments)
	}
	g.Mu.Unlock()
	for n, want := range map[int][]string{3: {"status:cancelled"}, 5: {"status:merged"}, 7: {"status:done"}, 1: {"status:done"}} {
		l := g.LabelNames(n)
		for _, w := range want {
			if !slices.Contains(l, w) {
				t.Errorf("#%d labels %v, want %s", n, l, w)
			}
		}
		if slices.Contains(l, LeaseLabel) || len(slices.DeleteFunc(slices.Clone(l), func(s string) bool { return !strings.HasPrefix(s, "status:") })) != 1 {
			t.Errorf("#%d labels %v: one status, no lease", n, l)
		}
	}
	// Every write went through aayu's actions log under the person's identity.
	f, err := os.Open(filepath.Join(env["XDG_STATE_HOME"], "aayudha", "telemetry", "actions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	writes := 0
	for sc := bufio.NewScanner(f); sc.Scan(); {
		var r actions.Record
		if json.Unmarshal(sc.Bytes(), &r) != nil || !strings.HasPrefix(r.Kind, "github.") {
			continue
		}
		writes++
		if r.Identity != "human" {
			t.Errorf("write %s on %s under identity %q, want human", r.Kind, r.Target, r.Identity)
		}
	}
	if writes < 10 {
		t.Errorf("%d writes logged, want each of the four acts' writes", writes)
	}

	// Criterion 4: option 2 on the provisional Decision, confirmed because it
	// is not the recommendation, reverts it to B and clears its label; skip on
	// the waiver writes nothing.
	g = inboxRepo(t)
	opts.GitHubAPI = g.URL()
	if code, out, errOut := runIn(t, newEnv(), opts, "s 2 y q", "inbox", "--repo", "o/r"); code != 0 || !strings.Contains(out, "#2 accepted: B, by @o") || !strings.Contains(out, "inbox: 1 done, 1 skipped, 4 left") {
		t.Fatalf("skip, choose another: exit %d: %s\n%s", code, errOut, out)
	}
	g.Mu.Lock()
	if g.Issues[1].State != "open" || len(g.Issues[1].Comments) != 0 || g.Issues[2].State != "closed" || !strings.Contains(g.Issues[2].Body, "B, accepted by @o") {
		t.Errorf("after skip and choose another: #1 %s, #2 %s\n%s", g.Issues[1].State, g.Issues[2].State, g.Issues[2].Body)
	}
	g.Mu.Unlock()
	if slices.Contains(g.LabelNames(2), workitems.LabelProvisional) || slices.Contains(g.LabelNames(1), "status:done") {
		t.Errorf("labels after choose another: #2 %v, #1 %v", g.LabelNames(2), g.LabelNames(1))
	}
	// The keys on steps that lack an answer: an incomplete Decision has no
	// option to accept, and an unknown key (c among them: it is no key now)
	// shows the step's choices again. A digit is the Decision's own option
	// number: with the recommendation on option 2, 3 accepts option 3, after a
	// confirmation that n declines. A failed act (ship's gates) is reported
	// and counts as not done, and the end of input stops the inbox with the
	// rest left.
	g = fake.New(t, "o/r", "User")
	tw, _ := workitems.FindType("decision")
	st, _ := json.Marshal(inspect.State{Cycles: inspect.MaxCycles})
	g.AddIssue("Pick a queue", "x", "type:decision", "status:planned") // #1
	g.AddIssue("Choose the cache", workitems.Render(tw, map[string]string{workitems.FieldSummary: "Which cache?",
		"options": "1. **Keep it**: the current one.\n2. **Swap it** for a managed one.\n3. **Drop it** entirely.", "evaluation": "Swap is cheaper.",
		"recommendation": "Option 2, because it is cheaper.", workitems.FieldReversible: "true"}), "type:decision", "status:planned") // #2
	g.AddIssue("Old story", "x", "type:story", "status:backlog") // #3
	g.AddIssue("New story", "Replaces #3.", "type:story", "status:ready")
	g.AddIssue("A bug with no fix yet", "x", "type:bug", "status:planned")                             // #4
	capped := g.AddPR("fix(cli): capped", "Refs #170", "o", "fix/170", strings.Repeat("d", 40), "dev") // #6
	g.Mu.Lock()
	g.Issues[capped.Number].Comments = append(g.Issues[capped.Number].Comments, &fake.Comment{ID: 78, Body: inspect.Marker + "\n<!-- aayudha:inspect-state " + string(st) + " -->\n"})
	g.Mu.Unlock()
	// Without persona assets a step names the seat alone.
	bare := Options{GitHubAPI: g.URL(), GitHubGap: -1, IsTerminal: tty}
	code, out, errOut = runIn(t, newEnv(), bare, "x a c o s  9 3 n 3 y  2 y", "inbox", "--repo", "o/r")
	if code != 0 {
		t.Fatalf("inbox: exit %d: %s\n%s", code, errOut, out)
	}
	for _, want := range []string{
		"[1/4] Decision #1: Pick a queue\n  not Decision-ready (",
		"  Founder has no option to accept here.\n  s = skip for now · o = open on GitHub to complete it · q = quit\n",
		"  > x\n  s = skip for now · o = open on GitHub to complete it · q = quit\n  > a\n  s = skip for now · o = open on GitHub to complete it · q = quit\n" +
			"  > c\n  s = skip for now · o = open on GitHub to complete it · q = quit\n  > o\n  open: ",
		"  1. Keep it\n  2. Swap it (recommended)\n  3. Drop it\n  a or Enter = accept the recommendation (option 2) · 1-3 = accept that option · s = skip for now · o = open on GitHub · q = quit\n",
		"  > 9\n  a or Enter = accept the recommendation (option 2) · 1-3 = accept that option · s = skip for now · o = open on GitHub · q = quit\n" +
			"  > 3\n  This accepts option 3 (Drop it) as the answer to Decision #2 and closes it, instead of the recommendation (option 2, Swap it). Apply it? y = yes · n = no, go back\n" +
			"  > n\n  not applied\n  a or Enter = accept the recommendation (option 2) · 1-3 = accept that option · s = skip for now · o = open on GitHub · q = quit\n" +
			"  > 3\n  This accepts option 3 (Drop it)",
		"  > y\n#2 accepted: Drop it, by @o",
		"[3/4] PR at inspect's cycle cap #6",
		"  > 2\n  This will ship PR #6 now (ship's gates still apply), instead of the recommendation (option 1, open it and review it). Apply it? y = yes · n = no, go back\n  > y\n  not done: ",
		"[4/4] Replaced #3: Old story",
		"inbox: 1 done, 1 skipped, 1 left",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inbox output lacks %q:\n%s", want, out)
		}
	}
	g.Mu.Lock()
	if g.Issues[1].State != "open" || g.Issues[3].State != "open" || !strings.Contains(g.Issues[2].Body, "Drop it, accepted by @o") {
		t.Errorf("after the picks: #1 %s, #3 %s, #2:\n%s", g.Issues[1].State, g.Issues[3].State, g.Issues[2].Body)
	}
	g.Mu.Unlock()
	// With nothing waiting, it says so; under a bot's token it refuses.
	g = fake.New(t, "o/r", "User")
	opts.GitHubAPI = g.URL()
	if code, out, _ := runIn(t, newEnv(), opts, "", "inbox", "--repo", "o/r"); code != 0 || out != "o/r: nothing waits for you.\n" {
		t.Errorf("empty inbox: exit %d: %q", code, out)
	}
	// An option marked "(recommended)" is the recommendation; c is no key (it
	// read as "cancel"), so it shows the choices; input that ends leaves the
	// step undone.
	g.AddIssue("Choose the CDN", workitems.Render(tw, map[string]string{workitems.FieldSummary: "Which CDN?",
		"options": "1. **Ours**\n2. **Theirs** (recommended)\n3. **None**", "evaluation": "Theirs is faster.",
		"recommendation": "The managed one.", workitems.FieldReversible: "true"}), "type:decision", "status:planned")
	if code, out, _ := runIn(t, newEnv(), opts, "c", "inbox", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "  1. Ours\n  2. Theirs (recommended)\n  3. None\n  a or Enter = accept the recommendation (option 2) · 1-3 = accept that option · s = skip for now · o = open on GitHub · q = quit\n"+
			"  > c\n  a or Enter = accept the recommendation (option 2) · 1-3 = accept that option · s = skip for now · o = open on GitHub · q = quit\n  > \n\ninbox: 0 done, 0 skipped, 1 left") {
		t.Errorf("recommended option, input ends at the pick: exit %d:\n%s", code, out)
	}
	g.ViewerType = "Bot"
	if code, _, errOut := runIn(t, newEnv(), opts, "", "inbox", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "is a bot identity") {
		t.Errorf("inbox under a bot token: exit %d: %s", code, errOut)
	}

	// Criterion 6: a host agent's `aayu inbox` is blocked at the pre-tool hook.
	if code, _, errOut := runIn(t, nil, Options{}, `{"tool_name":"Bash","tool_input":{"command":"aayu inbox"}}`, "hook", "pre-tool", "--host", "claude"); code != 2 || !strings.Contains(errOut, "aayu inbox is for a person, not an agent") {
		t.Errorf("pre-tool hook on aayu inbox: exit %d: %s", code, errOut)
	}
}

// Criterion 5: the home screen and `aayu next` point to the inbox when it has
// steps, and say nothing of it when it has none.
func TestInboxOnHome(t *testing.T) {
	repo, g := homeRepo(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	opts := Options{Dir: repo, GitHubAPI: g.URL(), GitHubGap: -1, Assets: testAssets(t)}
	for _, args := range [][]string{{"--plain"}, {"next"}} {
		if _, out, _ := run(t, env, opts, append(args, "--repo", "o/r")...); strings.Contains(strings.ToLower(out), "inbox") {
			t.Errorf("%v with nothing waiting mentions the inbox:\n%s", args, out)
		}
	}
	tw, _ := workitems.FindType("decision")
	title, fields := decide.WaiverItem("coverage", 1, "")
	g.AddIssue(title, workitems.Render(tw, fields), "type:decision", "status:planned", workitems.LabelWaiver)
	if _, out, _ := run(t, env, opts, "--plain", "--repo", "o/r"); !strings.Contains(out, "Waiting in your inbox: 1 (#7 Waive the coverage floor on #1)") {
		t.Errorf("home with an inbox step:\n%s", out)
	}
	if _, out, _ := run(t, env, opts, "next", "--repo", "o/r"); !strings.Contains(out, "aayu inbox") || !strings.Contains(out, "for a person to run") {
		t.Errorf("next with an inbox step:\n%s", out)
	}
}

// Story #481: at a terminal under an agent marker, inbox asks for the device
// code once; the person's pick is then applied, not turned into a batch for a
// reply, even where reply approval is on.
func TestInboxMarkedTerminalAppliesThePick(t *testing.T) {
	g := inboxRepo(t)
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "o", Outcome: "approve"}
	g.TokenLogins = map[string]fake.Account{"bot-token": {Login: "aayudha-bot[bot]", Type: "Bot"}}
	g.Mu.Unlock()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_BOT_TOKEN": "bot-token", "AMP_THREAD_ID": "T-1", "XDG_STATE_HOME": t.TempDir()}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, IsTerminal: func(any) bool { return true }, Assets: aayudha.Assets, DevicePoll: time.Millisecond}
	code, out, errOut := runIn(t, env, opts, "saq", "inbox", "--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=someone")
	g.Mu.Lock()
	state := g.Issues[2].State
	g.Mu.Unlock()
	if code != 0 || state != "closed" || !strings.Contains(out, "#2 accepted: A, by @o") || strings.Contains(out+errOut, "posted batch") {
		t.Fatalf("exit %d, #2 %s:\n%s\n%s", code, state, out, errOut)
	}
}

// Story PRs merge only through ship: an agent's `aayu ship` that stops only
// for the human pass comments Ready to ship, and the person ships both such
// PRs in one inbox session under one device code. In solo mode their pick is
// their solo-mode bypass, which the inbox records in the PR before it ships.
func TestInboxShipsReadyPRs(t *testing.T) {
	g, env, opts, dir, bare, prA, headA := thinFixture(t)
	if code, _, errOut := run(t, env, opts, "ready", "6", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready 6: %s", errOut)
	}
	_, _, _ = run(t, env, opts, "build", "6", "--repo", "o/r")
	_ = os.WriteFile(filepath.Join(BuildWorktree(dir, 6), "orphan.txt"), []byte("orphan\n"), 0o644)
	code, out, errOut := run(t, env, opts, "build", "6", "--repo", "o/r", "--json")
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || res.PR == 0 {
		t.Fatalf("build 6: %d %+v %s", code, res, errOut)
	}
	prB, headB := res.PR, git(t, res.Worktree, "rev-parse", "HEAD")
	g.Mu.Lock()
	g.Pulls[prB].Head.SHA = headB
	g.Mu.Unlock()
	floors := []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"}}
	gateRunFor(t, g, dir, 2, prA, headA, floors)
	gateRunFor(t, g, dir, 6, prB, headB, floors)

	// Anyone may comment: a Ready to ship marker from an account without write
	// access is not ship's word, so the inbox does not list the PR, and ship
	// never edits that look-alike.
	g.Mu.Lock()
	forged := &fake.Comment{ID: 501, Body: ReadyToShipMarker + " head=" + headA + " -->\n**Ready to ship.**"}
	forged.User.Login, forged.User.Type = "mallory", "User"
	g.Issues[prA].Comments = append(g.Issues[prA].Comments, forged)
	g.Mu.Unlock()
	if _, out, _ := runIn(t, env, opts, "s s s", "inbox", "--repo", "o/r"); !strings.Contains(out, "1 human-only step(s)") || strings.Contains(out, "ready to ship") {
		t.Fatalf("a forged marker listed the PR:\n%s", out)
	}

	// The agent ships from its own shell: the hook lets it, and ship's gates
	// hold. The author is the founder, so no review can pass it: Ready to ship.
	agent := maps.Clone(env)
	maps.Copy(agent, hostShells["amp"])
	for _, pr := range []int{prA, prB} {
		code, _, errOut := run(t, agent, opts, "ship", strconv.Itoa(pr), "--repo", "o/r")
		if code != ExitBlocked || !strings.Contains(errOut, "no human pass") || !strings.Contains(errOut, "commented Ready to ship") {
			t.Fatalf("agent ship #%d: %d %s", pr, code, errOut)
		}
	}
	g.Mu.Lock()
	// PR B had hit inspect's cycle cap before its gate went green: ship's
	// Ready to ship on the current head stands over the cap.
	capped, _ := json.Marshal(inspect.State{Cycles: inspect.MaxCycles, Result: &inspect.Result{Disposition: "fix"}})
	g.Issues[prB].Comments = append(g.Issues[prB].Comments, &fake.Comment{ID: 502, Body: inspect.Marker + "\n### inspect: fix\n\n<!-- aayudha:inspect-state " + string(capped) + " -->\n"})
	if forged.Body != ReadyToShipMarker+" head="+headA+" -->\n**Ready to ship.**" {
		t.Errorf("ship edited someone else's marker: %q", forged.Body)
	}
	// A push to PR A after the inbox listed it: the pick was for the head
	// the person saw, so nothing merges or is commented.
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Hook = func(method, path string) {
		if method == "GET" && path == "/repos/o/r/pulls/"+strconv.Itoa(prA) {
			g.Pulls[prA].Head.SHA = strings.Repeat("e", 40)
		}
	}
	g.Mu.Unlock()
	opts.DevicePoll = time.Millisecond
	opts.IsTerminal = func(any) bool { return true }
	_, out, _ = runIn(t, agent, opts, "s a q", "inbox", "--repo", "o/r")
	g.Mu.Lock()
	moved := !g.Pulls[prA].Merged && !slices.ContainsFunc(g.Issues[prA].Comments, func(c *fake.Comment) bool { return strings.HasPrefix(c.Body, SoloBypassPrefix) })
	g.Pulls[prA].Head.SHA, g.Hook = headA, nil
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Mu.Unlock()
	if !moved || !strings.Contains(out, "the head moved to eeeeeee after you chose to ship "+short(headA)+"; nothing was merged") {
		t.Fatalf("ship after a push to the chosen PR:\n%s", out)
	}
	code, out, errOut = runIn(t, agent, opts, "s a a", "inbox", "--repo", "o/r")
	if code != 0 {
		t.Fatalf("inbox: exit %d: %s\n%s", code, errOut, out)
	}
	for _, pr := range []int{prA, prB} {
		want := fmt.Sprintf("Story PR ready to ship #%d", pr)
		if !strings.Contains(out, want) || !strings.Contains(out, fmt.Sprintf("1. ship PR #%d (ship's gates still apply: in solo mode under your solo-mode bypass, otherwise once you have approved its head on GitHub) (recommended)\n  2. open it and review it\n", pr)) {
			t.Errorf("inbox lacks %q with its choices:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "inbox: 2 done, 1 skipped, 0 left") {
		t.Errorf("inbox summary:\n%s", out)
	}
	g.Mu.Lock()
	started := g.Device.Started
	merged := g.Pulls[prA].Merged && g.Pulls[prB].Merged
	var bypasses int
	for _, pr := range []int{prA, prB} {
		for _, c := range g.Issues[pr].Comments {
			if strings.HasPrefix(c.Body, SoloBypassPrefix+" @founder ships this from `aayu inbox`") {
				bypasses++
			}
		}
	}
	g.Mu.Unlock()
	if started != 1 || !merged || bypasses != 2 {
		t.Fatalf("device codes %d (want 1), both merged %v, bypass comments %d (want 2):\n%s\n%s", started, merged, bypasses, out, errOut)
	}
	for story, head := range map[int]string{2: headA, 6: headB} {
		if rec := git(t, bare, "show", fmt.Sprintf("ledger:proof/%d/%s/authoritative/ship.json", story, head)); !strings.Contains(rec, `"kind": "solo-bypass"`) {
			t.Errorf("ship record for #%d: %s", story, rec)
		}
	}
}
