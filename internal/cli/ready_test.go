package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// ledgerCheckout is a clone whose origin is a local bare repository with a ledger branch.
func ledgerCheckout(t *testing.T) (dir, bare string) {
	t.Helper()
	root := t.TempDir()
	bare = filepath.Join(root, "origin.git")
	git(t, root, "init", "--quiet", "--bare", bare)
	seed := filepath.Join(root, "seed")
	git(t, root, "clone", "--quiet", bare, seed)
	git(t, seed, "checkout", "--quiet", "--orphan", "ledger")
	_ = os.WriteFile(filepath.Join(seed, "README.md"), []byte("ledger\n"), 0o644)
	git(t, seed, "add", ".")
	git(t, seed, "commit", "--quiet", "-m", "ledger")
	git(t, seed, "push", "--quiet", "origin", "ledger")
	git(t, seed, "checkout", "--quiet", "--orphan", "dev")
	git(t, seed, "commit", "--quiet", "--allow-empty", "-m", "dev")
	git(t, seed, "push", "--quiet", "origin", "dev")
	dir = filepath.Join(root, "work")
	git(t, root, "clone", "--quiet", "--branch", "dev", bare, dir)
	return dir, bare
}

func fixtureBody(t *testing.T, typ string, f func(map[string]string)) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join("..", "workitems", "testdata", "items", typ+".json"))
	var m map[string]workitems.Item
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	fields := m["ready"].Fields
	if f != nil {
		f(fields)
	}
	ty, _ := workitems.FindType(typ)
	return workitems.Render(ty, fields)
}

// tree builds a Feature (#1) with Stories under it, plus a Decision.
func readyTree(t *testing.T) *fake.GitHub {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	feature := g.AddIssue("Eligibility questionnaire", fixtureBody(t, "feature", nil), "type:feature", "status:planned")
	story := g.AddIssue("Show empty list", fixtureBody(t, "story", nil), "type:story", "status:planned", "effort:S")
	xl := g.AddIssue("Rebuild everything", fixtureBody(t, "story", nil), "type:story", "effort:XL")
	unmapped := g.AddIssue("Unmapped", fixtureBody(t, "story", func(f map[string]string) {
		f[workitems.FieldCriteria] = "1. [screenshot] state: empty; expected: message\n2. state: full; expected: total"
	}), "type:story", "effort:S")
	g.AddIssue("Engine", fixtureBody(t, "decision", func(f map[string]string) { delete(f, workitems.FieldReversible) }), "type:decision")
	for _, s := range []*fake.Issue{story, xl, unmapped} {
		feature.Subs = append(feature.Subs, s.Number)
		s.Parent = feature.Number
	}
	g.AddIssue("Orphan story", fixtureBody(t, "story", nil), "type:story", "effort:S") // #6, no parent
	return g
}

// WP2.7 accept: fixtures pass or fail Ready with the expected messages.
func TestReadyCheckFixtures(t *testing.T) {
	g := readyTree(t)
	// A Decision offers a real choice: one numbered option fails Ready, two pass.
	g.AddIssue("One way only", fixtureBody(t, "decision", func(f map[string]string) { f["options"] = "1. **In-house.** We build the engine." }), "type:decision") // #7
	g.AddIssue("Two ways", fixtureBody(t, "decision", nil), "type:decision")                                                                                      // #8
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	tests := []struct {
		issue string
		code  int
		want  []string
	}{
		{"2", 0, nil},
		{"3", ExitBlocked, []string{"effort: effort:XL blocks Ready; slice it into XS-L stories"}},
		{"4", ExitBlocked, []string{"completion_goal: criterion 2 is unmapped (no evidence kind); start it with one of [screenshot], [recording], [sr-transcript], [api-trace], [metric]"}},
		{"5", ExitBlocked, []string{"reversible: Reversible is required at Decision-ready"}},
		{"6", 0, nil}, // a Story without a parent Feature is Ready (D-2.1c)
		{"7", ExitBlocked, []string{"options: 1 numbered option(s); a Decision offers at least 2, and aims for 3, so the person makes a real choice: " +
			"write each as a top-level line \"N. **Name.** what it does\", including the option of not doing it"}},
		{"8", 0, nil},
	}
	for _, tt := range tests {
		code, out, errOut := run(t, env, opts, "ready", tt.issue, "--check", "--json", "--repo", "o/r")
		var res ready.Result
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		var got []string
		for _, p := range res.Problems {
			got = append(got, p.String())
		}
		if code != tt.code || !slices.Equal(got, tt.want) {
			t.Errorf("ready %s --check: exit %d %q, want %d %q (%s)", tt.issue, code, got, tt.code, tt.want, errOut)
		}
		if tt.issue == "2" && (res.Threshold != "2 of 2" || res.Environment != "staging with seed data" || res.Scenario != "Open the list with no items.") {
			t.Errorf("ready 2 --check: threshold %q, scenario %q, environment %q; want the story's fields (D-3.4i)", res.Threshold, res.Scenario, res.Environment)
		}
	}
	if w := g.Writes(); len(w) != 0 {
		t.Errorf("--check wrote %v", w)
	}
}

// WP2.7 accept: on the day-one path the snapshot is written under the human
// identity, tagged identity:human-fallback, and the actions log (what audit
// reconciles, WP7.6) lists it; a guard strips status:ready after an edit.
func TestReadyHumanFallbackAndGuard(t *testing.T) {
	g := readyTree(t)
	dir, bare := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	code, out, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r")
	if code != 0 {
		t.Fatalf("ready 2: exit %d: %s\n%s", code, errOut, out)
	}
	labels := g.LabelNames(2)
	if !slices.Contains(labels, "status:ready") || !slices.Contains(labels, "identity:human-fallback") || slices.Contains(labels, "status:planned") {
		t.Fatalf("labels after ready: %v", labels)
	}
	g.Mu.Lock()
	comments := g.Issues[2].Comments
	g.Mu.Unlock()
	if len(comments) != 1 || !strings.Contains(comments[0].Body, ready.SnapshotMarker) || !strings.Contains(comments[0].Body, "`identity:human-fallback`") ||
		!strings.Contains(comments[0].Body, "| Technical Owner | the Code Architect twin |") {
		t.Fatalf("snapshot comment: %+v", comments)
	}
	snap := git(t, bare, "show", "ledger:specs/2/ready.md")
	if !strings.Contains(snap, "identity:human-fallback") || !strings.Contains(snap, "## Body at Ready") {
		t.Fatalf("ledger snapshot:\n%s", snap)
	}
	if msg := git(t, bare, "log", "-1", "--format=%B", "ledger"); !strings.Contains(msg, "Aayudha-Identity: identity:human-fallback") {
		t.Fatalf("ledger commit message: %s", msg)
	}
	// The actions log lists the human-fallback writes (audit's input, n/a until WP7.6).
	code, out, _ = run(t, env, opts, "actions", "grep", `"identity":"human-fallback"`)
	if code != 0 || strings.Count(out, "\n") < 3 || !strings.Contains(out, "github.comment.ready-snapshot") || !strings.Contains(out, "ledger.push") {
		t.Fatalf("human-fallback writes in the actions log:\n%s", out)
	}

	// A second ready changes nothing.
	g.Reset()
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 || len(g.Writes()) != 0 {
		t.Fatalf("second ready: exit %d, writes %v: %s", code, g.Writes(), errOut)
	}

	ci := map[string]string{"GITHUB_TOKEN": "workflow-token"}
	guard := func(n string) (int, string) {
		code, out, errOut := run(t, ci, Options{GitHubAPI: g.URL(), GitHubGap: -1}, "ready", n, "--guard", "--ci", "--repo", "o/r")
		return code, out + errOut
	}
	// Unchanged: the guard leaves it.
	if code, out := guard("2"); code != 0 || !slices.Contains(g.LabelNames(2), "status:ready") {
		t.Fatalf("guard on an unchanged Ready issue: %d %s", code, out)
	}
	// Edited after Ready: stripped, with a comment.
	g.Mu.Lock()
	g.Issues[2].Body += "\nOne more thing.\n"
	g.Mu.Unlock()
	if code, out := guard("2"); code != 0 || slices.Contains(g.LabelNames(2), "status:ready") || !strings.Contains(out, "body changed after Ready") {
		t.Fatalf("guard after an edit: %d %s, labels %v", code, out, g.LabelNames(2))
	}
	g.Mu.Lock()
	last := g.Issues[2].Comments[len(g.Issues[2].Comments)-1].Body
	g.Mu.Unlock()
	if !strings.Contains(last, ready.GuardMarker) || !strings.Contains(last, "Removed `status:ready`") {
		t.Fatalf("guard comment: %s", last)
	}
	// Labelled by hand, without a snapshot: stripped.
	g.Mu.Lock()
	g.Issues[6].Parent = 1
	g.Issues[6].Labels = append(g.Issues[6].Labels, fake.Label{Name: "status:ready"})
	g.Mu.Unlock()
	if code, out := guard("6"); code != 0 || slices.Contains(g.LabelNames(6), "status:ready") || !strings.Contains(out, "no Ready snapshot") {
		t.Fatalf("guard on a hand-labelled issue: %d %s", code, out)
	}
	// No longer valid (XL added after a snapshot would also be caught): #3 labelled ready by hand.
	g.Mu.Lock()
	g.Issues[3].Labels = append(g.Issues[3].Labels, fake.Label{Name: "status:ready"})
	g.Mu.Unlock()
	if code, out := guard("3"); code != 0 || slices.Contains(g.LabelNames(3), "status:ready") || !strings.Contains(out, "effort:XL blocks Ready") {
		t.Fatalf("guard on an XL issue: %d %s", code, out)
	}
}

// Story #990: an agent drafts a Feature's intent baseline from its thread;
// only a person approves it, through GitHub's device flow, at the hash the
// draft pinned. An edit while the code waits baselines nothing.
func TestReadyFeatureBaseline(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	feature := g.AddIssue("Feature baselines", fixtureBody(t, "feature", nil), "type:feature") // #1
	dir, bare := ledgerCheckout(t)
	state := filepath.Join(dir, "state")
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "AMP_THREAD_ID": "T-agent-thread", "XDG_STATE_HOME": state}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": state} // a terminal with no agent marker
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond}
	readyRun := func(env map[string]string, args ...string) (int, string, string) {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"ready", "1", "--repo", "o/r"}, args...)...)
		return code, out, errOut
	}
	commits := func() []string { // ledger commits that wrote a baseline of #1
		t.Helper()
		if out := git(t, bare, "log", "--format=%H", "ledger", "--", "specs/1/baselines"); out != "" {
			return strings.Split(out, "\n")
		}
		return nil
	}
	files := func() []string {
		t.Helper()
		return strings.Fields(git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "--", "specs/1/baselines"))
	}
	record := func(path string) ready.Baseline {
		t.Helper()
		var b ready.Baseline
		if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+path)), &b); err != nil {
			t.Fatalf("ledger %s: %v", path, err)
		}
		return b
	}
	markers := func() []string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		var out []string
		for _, c := range feature.Comments {
			if strings.Contains(c.Body, "<!-- aayudha:baseline part=intent -->") {
				out = append(out, c.Body)
			}
		}
		return out
	}
	device := func(onPoll func(int)) {
		g.Mu.Lock()
		g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", OnPoll: onPoll}
		g.Mu.Unlock()
	}
	started := func() int {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if g.Device == nil {
			return 0
		}
		return g.Device.Started
	}
	// The pins' hash, computed here from its definition: one "<ref> <hash>" line per pin.
	pinsHash := func(body string) string {
		sum := sha256.Sum256([]byte("#1 body " + workitems.BodyHash(body) + "\n"))
		return hex.EncodeToString(sum[:])
	}
	body := feature.Body
	want := pinsHash(body)

	// 1, 2. The agent's request writes the draft and the marker comment, and never asks for a code.
	device(nil)
	if code, out, errOut := readyRun(agent, "--part", "intent", "--request"); code != 0 || started() != 0 {
		t.Fatalf("--part intent --request: exit %d, device codes %d: %s%s", code, started(), out, errOut)
	}
	if c, f := commits(), files(); len(c) != 1 || !slices.Equal(f, []string{"specs/1/baselines/intent-v1.draft.json"}) {
		t.Fatalf("after the request: %d commits, files %v", len(c), f)
	}
	d := record("specs/1/baselines/intent-v1.draft.json")
	if d.Part != "intent" || d.Version != 1 || d.Owner != "product-strategist" || d.Hash != want ||
		len(d.Pins) != 1 || d.Pins[0] != (ready.Pin{Ref: "#1 body", Hash: workitems.BodyHash(body)}) ||
		d.Summary != "A five-question eligibility check." || d.SourceSession != "T-agent-thread" || d.ApprovedBy != "" {
		t.Fatalf("draft: %+v", d)
	}
	if m := markers(); len(m) != 1 || !strings.Contains(m[0], "Intent v1 of #1 waits for a person: `aayu ready 1 --part intent`") {
		t.Fatalf("marker comment after the request: %q", m)
	}

	// 3. An agent lane is refused before any read or write.
	g.Reset()
	code, out, errOut := readyRun(agent, "--part", "intent")
	g.Mu.Lock()
	reqs := 0 // the halt check reads a variable first; the issue is never read
	for _, r := range g.Requests {
		if strings.Contains(r.Path, "/issues") {
			reqs++
		}
	}
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(out+errOut, "ready --part is a person's decision and is refused for an agent lane (AAYU_AGENT=1)") || reqs != 0 || started() != 0 {
		t.Fatalf("agent --part intent: exit %d, %d issue requests, device codes %d: %s%s", code, reqs, started(), out, errOut)
	}
	if len(commits()) != 1 || len(markers()) != 1 {
		t.Fatalf("the agent's refusal wrote: %d commits, %d marker comments", len(commits()), len(markers()))
	}
	// --request as a person drafts again (unchanged), and never asks for a code either.
	if code, out, errOut := readyRun(person, "--part", "intent", "--request"); code != 0 || started() != 0 || len(commits()) != 1 {
		t.Fatalf("person --request: exit %d, device codes %d, %d commits: %s%s", code, started(), len(commits()), out, errOut)
	}

	// 5. The body is edited while the code waits: nothing is baselined.
	edited := body + "\nOne more question.\n"
	device(func(int) { feature.Body = edited })
	code, out, errOut = readyRun(person, "--part", "intent")
	changed := "The intent of #1 changed after the draft (hash " + want[:12] + " → " + pinsHash(edited)[:12] + "). Nothing was baselined. Draft it again."
	if code != ExitBlocked || !strings.Contains(out+errOut, changed) || started() != 1 {
		t.Fatalf("edited during the code: exit %d, device codes %d: %s%s", code, started(), out, errOut)
	}
	if len(commits()) != 1 || slices.Contains(files(), "specs/1/baselines/intent-v1.json") {
		t.Fatalf("edited during the code wrote: files %v", files())
	}
	// Still edited: refused before a code is asked for.
	device(nil)
	if code, out, errOut := readyRun(person, "--part", "intent"); code != ExitBlocked || !strings.Contains(out+errOut, changed) || started() != 0 {
		t.Fatalf("edited before the code: exit %d, device codes %d: %s%s", code, started(), out, errOut)
	}

	// 4. The body as drafted again: a person approves at the draft's hash, the effect shown before the code.
	g.Mu.Lock()
	feature.Body = body
	g.Mu.Unlock()
	code, out, errOut = readyRun(person, "--part", "intent")
	effect, enter := strings.Index(errOut, "Approving: "), strings.Index(errOut, "/login/device")
	if code != 0 || started() != 1 || effect < 0 || enter < effect || !strings.Contains(errOut[effect:enter], want[:12]) {
		t.Fatalf("person --part intent: exit %d, device codes %d: %s%s", code, started(), out, errOut)
	}
	if c, f := commits(), files(); len(c) != 2 || !slices.Contains(f, "specs/1/baselines/intent-v1.json") {
		t.Fatalf("after the approval: %d commits, files %v", len(c), f)
	}
	b := record("specs/1/baselines/intent-v1.json")
	if b.Hash != d.Hash || b.ApprovedBy != "founder" || b.ApprovedAt == "" || b.Presence != "device-flow" || b.Part != "intent" || b.Version != 1 {
		t.Fatalf("baseline: %+v", b)
	}
	if m := markers(); len(m) != 1 || !strings.Contains(m[0], "Intent v1 of #1 baselined by @founder · hash "+want[:12]) || strings.Contains(m[0], "waits for a person") {
		t.Fatalf("marker comment after the approval: %q", m)
	}
	// Nothing changed since v1: no new draft.
	if code, out, errOut := readyRun(agent, "--part", "intent", "--request"); code != 0 || !strings.Contains(out, "Nothing changed since intent v1 of #1. No draft written.") || len(commits()) != 2 {
		t.Fatalf("request after v1: exit %d, %d commits: %s%s", code, len(commits()), out, errOut)
	}

	// 6. No spec draft waits: nothing to approve, nothing written.
	g.Reset()
	device(nil)
	code, out, errOut = readyRun(person, "--part", "spec")
	if code != ExitBlocked || !strings.Contains(out+errOut, "No spec draft of #1 waits for a person. An agent drafts it: aayu ready 1 --part spec --request") ||
		len(g.Writes()) != 0 || len(commits()) != 2 || started() != 0 {
		t.Fatalf("--part spec with no draft: exit %d, writes %v: %s%s", code, g.Writes(), out, errOut)
	}
}
