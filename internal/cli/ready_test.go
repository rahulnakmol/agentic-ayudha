package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/chart"
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

	// Usage, UX for a Feature with no UI layer, a Story, a missing issue, no token, and --ci: each refused, nothing written.
	g.AddIssue("A story", fixtureBody(t, "story", nil), "type:story") // #2
	for _, c := range []struct {
		env  map[string]string
		args []string
		code int
		want string
	}{
		{agent, []string{"--request"}, ExitUsage, "--request goes with --part"},
		{agent, []string{"--part", "intent", "--check"}, ExitUsage, "neither --check nor --guard"},
		{agent, []string{"--part", "ux", "--request"}, ExitBlocked, "UX design: not needed (no UI layer in the footprint)"},
		{person, []string{"--part", "intent", "--ci"}, ExitBlocked, "needs GitHub's device-flow presence check"},
		{map[string]string{"XDG_STATE_HOME": state}, []string{"--part", "intent", "--request"}, ExitBlocked, ""},
		{map[string]string{"XDG_STATE_HOME": state}, []string{"--part", "intent"}, ExitBlocked, ""},
	} {
		if code, out, errOut := readyRun(c.env, c.args...); code != c.code || !strings.Contains(errOut, c.want) {
			t.Errorf("ready 1 %v: exit %d, want %d and %q: %s%s", c.args, code, c.code, c.want, out, errOut)
		}
	}
	for n, code := range map[string]int{"2": ExitUsage, "99": ExitExternal} {
		if got, out, errOut := run(t, agent, opts, "ready", n, "--repo", "o/r", "--part", "intent", "--request"); got != code {
			t.Errorf("ready %s --part intent --request: exit %d, want %d: %s%s", n, got, code, out, errOut)
		}
	}
	// Outside a checkout: no repository to name, or no ledger to write.
	away := opts
	away.Dir = t.TempDir()
	for _, args := range [][]string{{"ready", "1", "--part", "intent", "--request"}, {"ready", "1", "--repo", "o/r", "--part", "intent", "--request"}} {
		if code, out, errOut := run(t, agent, away, args...); code == 0 {
			t.Errorf("%v outside a checkout: exit 0: %s%s", args, out, errOut)
		}
	}
	if len(commits()) != 2 {
		t.Fatalf("a refused run wrote: %d commits", len(commits()))
	}

	// An edit after v1: --dry-run plans v2 and writes nothing; the request drafts v2.
	g.Mu.Lock()
	feature.Body = edited
	g.Mu.Unlock()
	changes := filepath.Join(t.TempDir(), "changes.json") // a version after v1 says why (Story #993)
	if err := os.WriteFile(changes, []byte(`[{"ref": "#1 body", "what": "The body asks one more question.", "why": "The check needs it to decide.", "need": "A person decides eligibility in one pass."}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := readyRun(agent, "--part", "intent", "--request", "--changes", changes, "--dry-run"); code != 0 || !strings.Contains(out, "--dry-run") || len(commits()) != 2 {
		t.Fatalf("--dry-run request: exit %d, %d commits: %s%s", code, len(commits()), out, errOut)
	}
	if code, out, errOut := readyRun(agent, "--part", "intent", "--request", "--changes", changes); code != 0 || !strings.Contains(out, "Intent v2 of #1 waits for a person") || len(commits()) != 3 {
		t.Fatalf("request after an edit: exit %d, %d commits: %s%s", code, len(commits()), out, errOut)
	}
	hand := filepath.Join(t.TempDir(), "hand")
	git(t, filepath.Dir(hand), "clone", "--quiet", "--branch", "ledger", bare, hand)
	handDraft := func(b ready.Baseline) { // a ledger write outside aayu
		data, _ := json.Marshal(b)
		git(t, hand, "pull", "--quiet", "origin", "ledger")
		if err := os.WriteFile(filepath.Join(hand, fmt.Sprintf("specs/1/baselines/intent-v%d.draft.json", b.Version)), data, 0o644); err != nil {
			t.Error(err)
		}
		git(t, hand, "commit", "--quiet", "-am", "a draft written by hand")
		git(t, hand, "push", "--quiet", "origin", "ledger")
	}
	v2 := record("specs/1/baselines/intent-v2.draft.json")
	// The person denies the code: nothing is baselined.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "deny"}
	g.Mu.Unlock()
	if code, out, errOut := readyRun(person, "--part", "intent"); code != ExitBlocked || !strings.Contains(errOut, "denied") {
		t.Fatalf("denied: exit %d: %s%s", code, out, errOut)
	}
	// The draft's summary is rewritten while the code waits, its pins unchanged: the record is the draft the person saw.
	rewritten := v2
	rewritten.Summary, rewritten.DraftedBy = "Something else.", "someone-else"
	device(func(int) { handDraft(rewritten) })
	if code, out, errOut := readyRun(person, "--part", "intent"); code != 0 {
		t.Fatalf("summary rewritten during the code: exit %d: %s%s", code, out, errOut)
	}
	if b := record("specs/1/baselines/intent-v2.json"); b.Summary != v2.Summary || b.DraftedBy != v2.DraftedBy || b.ApprovedBy != "founder" {
		t.Fatalf("baseline after a rewrite during the code: %+v", b)
	}

	// An edit after v2, and its v3 draft.
	g.Mu.Lock()
	feature.Body = body + "\nA third question.\n"
	g.Mu.Unlock()
	if code, out, errOut := readyRun(agent, "--part", "intent", "--request", "--changes", changes); code != 0 || !strings.Contains(out, "Intent v3 of #1 waits for a person") {
		t.Fatalf("request after v2: exit %d: %s%s", code, out, errOut)
	}
	v3 := record("specs/1/baselines/intent-v3.draft.json")
	// Another ledger write lands between the last read and the approval: nothing is baselined.
	race, pushed := opts, false
	race.LedgerAttempt = func(int, string) {
		if !pushed {
			pushed = true
			handDraft(v3) // the same record, a new commit
		}
	}
	device(nil)
	if code, out, errOut := run(t, person, race, "ready", "1", "--repo", "o/r", "--part", "intent"); code != ExitBlocked ||
		!strings.Contains(errOut, "The ledger changed while aayu wrote intent v3 of #1. Nothing was baselined.") || slices.Contains(files(), "specs/1/baselines/intent-v3.json") {
		t.Fatalf("a ledger write during the approval: exit %d, files %v: %s%s", code, files(), out, errOut)
	}
	// The draft is drafted again while the code waits: nothing is baselined.
	other := v3
	other.Pins = []ready.Pin{{Ref: "#1 body", Hash: strings.Repeat("1", 64)}}
	other.Hash = ready.DraftHash(other.Pins, other.Changes, other.Fixes)
	device(func(int) { handDraft(other) })
	if code, out, errOut := readyRun(person, "--part", "intent"); code != ExitBlocked || !strings.Contains(errOut, "Intent v3 of #1 was approved or drafted again while the code waited. Nothing was baselined.") {
		t.Fatalf("re-drafted during the code: exit %d: %s%s", code, out, errOut)
	}
	// A draft whose hash matches the body but not its own pins is never approved.
	forged := v3
	forged.Pins = []ready.Pin{{Ref: "#1 body", Hash: strings.Repeat("0", 64)}}
	handDraft(forged)
	device(nil)
	code, out, errOut = readyRun(person, "--part", "intent")
	if code != ExitExternal || !strings.Contains(errOut, "not intent v3 of #1 with the hash of its pins") || started() != 0 ||
		slices.Contains(files(), "specs/1/baselines/intent-v3.json") {
		t.Fatalf("a forged draft: exit %d, device codes %d, files %v: %s%s", code, started(), files(), out, errOut)
	}
}

// Story #992: each part pins only what it owns. The spec pins each child
// story's title, Completion Goal, effort, layers, and dependencies, and the
// set of stories; the technical design pins design.json and the chart notes;
// the UX design pins the stage manifests on the ledger and the head of an
// early-build story's open PR.
func TestReadyFeaturePartPins(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	paths := func(ps ...string) func(map[string]string) {
		return func(f map[string]string) { f["expected_paths"] = "- `" + strings.Join(ps, "`\n- `") + "`" }
	}
	ui := g.AddIssue("A Feature with a UI", fixtureBody(t, "feature", paths("packages/design-system/**", "internal/models/**", "internal/cli/ready.go")), "type:feature") // #1: L01, L02, L05
	noUI := g.AddIssue("A Feature with no UI", fixtureBody(t, "feature", paths("internal/cli/ready.go")), "type:feature")                                                 // #2: L02
	story := func(title string, links string, labels ...string) *fake.Issue {
		s := g.AddIssue(title, fixtureBody(t, "story", func(f map[string]string) {
			f[workitems.FieldLinks], f[workitems.FieldAssumptions] = links, "- The list API exists."
			f[workitems.FieldCriteria] = strings.Replace(f[workitems.FieldCriteria], "cart is empty", title+" is empty", 1) // each story its own goal
		}), append([]string{"type:story"}, labels...)...)
		s.Parent, ui.Subs = ui.Number, append(ui.Subs, s.Number)
		return s
	}
	// Five stories, listed out of number order; labels and links out of order too.
	s3 := story("Story three", "depends-on #6\ndepends-on #4", "effort:M", "layer:L07", "layer:L02")
	s4 := story("Story four", "", "effort:XS", "layer:L05")
	_ = story("Story cancelled", "", "effort:S", "layer:L02", "status:cancelled") // #5: not in the spec
	s6 := story("Story six", "", "effort:L", "layer:L01", "early-build")
	s7 := story("Story seven", "depends-on #3", "effort:S", "layer:L02")
	s8 := story("Story eight", "", "effort:S", "layer:L01")
	ui.Subs = []int{s7.Number, s3.Number, 5, s6.Number, s4.Number, s8.Number}
	dir, bare := ledgerCheckout(t)
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	readyRun := func(n int, args ...string) (int, string) {
		t.Helper()
		code, out, errOut := run(t, agent, opts, append([]string{"ready", fmt.Sprint(n), "--repo", "o/r", "--part"}, args...)...)
		return code, out + errOut
	}
	record := func(path string) ready.Baseline {
		t.Helper()
		var b ready.Baseline
		if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+path)), &b); err != nil {
			t.Fatalf("ledger %s: %v", path, err)
		}
		return b
	}
	commits := func() string { return git(t, bare, "rev-list", "--count", "ledger") }
	sha := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	putLedger := func(files map[string]string) { // a ledger write outside this verb (plan, the design verbs)
		t.Helper()
		hand := filepath.Join(t.TempDir(), "hand")
		git(t, filepath.Dir(hand), "clone", "--quiet", "--branch", "ledger", bare, hand)
		for p, b := range files {
			_ = os.MkdirAll(filepath.Dir(filepath.Join(hand, p)), 0o755)
			if err := os.WriteFile(filepath.Join(hand, p), []byte(b), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git(t, hand, "add", ".")
		git(t, hand, "commit", "--quiet", "-m", "seed")
		git(t, hand, "push", "--quiet", "origin", "ledger")
	}

	// 2. A dry run before and after an edit to only one story's Assumptions: the pin lists are equal byte for byte.
	dryPins := func() string {
		t.Helper()
		code, out, errOut := run(t, agent, opts, "ready", "1", "--repo", "o/r", "--part", "spec", "--request", "--dry-run", "--json")
		var rep struct {
			Baseline struct{ Pins json.RawMessage }
		}
		if err := json.Unmarshal([]byte(out), &rep); code != 0 || err != nil || len(rep.Baseline.Pins) == 0 {
			t.Fatalf("spec --request --dry-run --json: exit %d, %v: %s%s", code, err, out, errOut)
		}
		return string(rep.Baseline.Pins)
	}
	before := dryPins()
	body := s3.Body
	g.Mu.Lock()
	s3.Body = strings.Replace(body, "- The list API exists.", "- The list API exists, and it pages by 50.", 1)
	g.Mu.Unlock()
	if after := dryPins(); after != before || s3.Body == body {
		t.Fatalf("an Assumptions edit changed the spec pins:\n%s\n%s", before, after)
	}
	// A control: a Completion Goal edit to the same story does change them.
	g.Mu.Lock()
	s3.Body = strings.Replace(body, "422 with code cart.empty", "422 with code cart.none", 1)
	g.Mu.Unlock()
	if dryPins() == before {
		t.Fatal("a Completion Goal edit left the spec pins unchanged")
	}
	g.Mu.Lock()
	s3.Body = body
	g.Mu.Unlock()
	if strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger"), "specs/") {
		t.Fatal("a --dry-run wrote to the ledger")
	}

	// 1. The draft has one pin per story (in number order), with its fields, and one for the set of story numbers.
	if code, out := readyRun(1, "spec", "--request"); code != 0 || !strings.Contains(out, "Spec v1 of #1 waits for a person") {
		t.Fatalf("spec --request: exit %d: %s", code, out)
	}
	d := record("specs/1/baselines/spec-v1.draft.json")
	goal := func(s *fake.Issue) string {
		it, _ := workitems.Parse(s.Number, s.Title, s.Body, "", []string{"type:story"})
		return workitems.CompletionGoalHash(it)
	}
	want := []struct {
		s      *fake.Issue
		fields string
	}{
		{s3, `{"number":3,"title":"Story three","completion_goal_hash":"` + goal(s3) + `","effort":"M","layers":["L02","L07"],"depends_on":[4,6]}`},
		{s4, `{"number":4,"title":"Story four","completion_goal_hash":"` + goal(s4) + `","effort":"XS","layers":["L05"],"depends_on":[]}`},
		{s6, `{"number":6,"title":"Story six","completion_goal_hash":"` + goal(s6) + `","effort":"L","layers":["L01"],"depends_on":[]}`},
		{s7, `{"number":7,"title":"Story seven","completion_goal_hash":"` + goal(s7) + `","effort":"S","layers":["L02"],"depends_on":[3]}`},
		{s8, `{"number":8,"title":"Story eight","completion_goal_hash":"` + goal(s8) + `","effort":"S","layers":["L01"],"depends_on":[]}`},
	}
	if len(d.Pins) != len(want)+1 || d.Owner != "product-strategist" {
		t.Fatalf("spec draft: %d pins, want %d: %+v", len(d.Pins), len(want)+1, d)
	}
	for i, w := range want {
		p := d.Pins[i]
		got, _ := json.Marshal(p.Story)
		if p.Ref != fmt.Sprintf("#%d", w.s.Number) || string(got) != w.fields || p.Hash != sha(w.fields) || goal(w.s) == "" || goal(w.s) == workitems.BodyHash(w.s.Body) {
			t.Errorf("spec pin %d: %s %s %s\nwant fields %s", i, p.Ref, p.Hash, got, w.fields)
		}
	}
	if set := d.Pins[len(want)]; set != (ready.Pin{Ref: "#1 stories", Hash: sha("[3,4,6,7,8]")}) {
		t.Errorf("story set pin: %+v", set)
	}
	// The draft read back passes its own check, so a second request finds it and writes nothing.
	n := commits()
	if code, out := readyRun(1, "spec", "--request"); code != 0 || commits() != n {
		t.Fatalf("spec --request again: exit %d, commits %s → %s: %s", code, n, commits(), out)
	}
	// A story's fields edited on the ledger by hand, its hash kept: the draft is refused, never drafted after.
	forged := d
	forged.Pins = slices.Clone(d.Pins)
	f := *forged.Pins[0].Story
	f.Effort, forged.Pins[0].Story = "XS", &f
	b, _ := json.Marshal(forged)
	putLedger(map[string]string{"specs/1/baselines/spec-v1.draft.json": string(b)})
	if code, out := readyRun(1, "spec", "--request"); code != ExitExternal || !strings.Contains(out, "pin #3 of spec v1 of #1 is not the hash of its story fields") {
		t.Fatalf("spec --request on a forged draft: exit %d: %s", code, out)
	}
	b, _ = json.Marshal(d)
	putLedger(map[string]string{"specs/1/baselines/spec-v1.draft.json": string(b)})
	n = commits()

	// 3. No design.json on the ledger: the technical design is refused, and nothing is written.
	if code, out := readyRun(1, "tech", "--request"); code != ExitBlocked ||
		!strings.Contains(out, "The technical design of #1 has no design.json yet. Vishwa writes it first: aayu plan 1 --draft <file> --create") || commits() != n {
		t.Fatalf("tech --request with no design: exit %d: %s", code, out)
	}

	// 4. design.json by the SHA-256 of its ledger bytes, and the chart note by the SHA-256 of its comment body.
	designJSON := `{"feature": 1, "layers": ["L01", "L02", "L05"]}` + "\n"
	putLedger(map[string]string{"specs/1/design.json": designJSON})
	note := chart.Comment(chart.KindData, "## Sources\n\nThe eval set.")
	g.Mu.Lock()
	ui.Comments = append(ui.Comments, &fake.Comment{ID: 901, Body: "Looks good."}, &fake.Comment{ID: 902, Body: note})
	g.Mu.Unlock()
	if code, out := readyRun(1, "tech", "--request"); code != 0 || !strings.Contains(out, "Tech v1 of #1 waits for a person") {
		t.Fatalf("tech --request: exit %d: %s", code, out)
	}
	if tp := record("specs/1/baselines/tech-v1.draft.json"); tp.Owner != "code-architect" ||
		!slices.Equal(tp.Pins, []ready.Pin{{Ref: "specs/1/design.json", Hash: sha(designJSON)}, {Ref: "#1 chart kind=data", Hash: sha(note)}}) {
		t.Fatalf("tech draft pins: %+v", tp.Pins)
	}

	// 5. A Feature with no UI layer in its footprint needs no UX baseline.
	n = commits()
	if code, out := readyRun(noUI.Number, "ux", "--request"); code != ExitBlocked || !strings.Contains(out, "UX design: not needed (no UI layer in the footprint)") || commits() != n {
		t.Fatalf("ux --request with no UI layer: exit %d: %s", code, out)
	}
	// A UI Feature with no stage manifest yet: nothing to pin, and nothing written.
	if code, out := readyRun(1, "ux", "--request"); code != ExitBlocked || !strings.Contains(out, "The UX design of #1 has no stage manifest on the ledger yet") || commits() != n {
		t.Fatalf("ux --request with no manifest: exit %d: %s", code, out)
	}

	// 6. Both manifests by the SHA-256 of their ledger bytes (not the working tree's), and the early-build PR by its head SHA.
	mockup, prototype := `{"stage": "mockup"}`+"\n", `{"stage": "prototype"}`+"\n"
	putLedger(map[string]string{"design/1/mockup/manifest.json": mockup, "design/1/prototype/manifest.json": prototype})
	_ = os.MkdirAll(filepath.Join(dir, "design", "1", "mockup"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "design", "1", "mockup", "manifest.json"), []byte(`{"stage": "edited in the working tree"}`), 0o644)
	head := strings.Repeat("c", 40)
	early := g.AddPR("Prototype the list", "Refs #6", "amp", "feat/list", head, "dev")
	g.AddPR("Build story eight", "Refs #8", "amp", "feat/eight", strings.Repeat("d", 40), "dev") // not an early build: not pinned
	if code, out := readyRun(1, "ux", "--request"); code != 0 || !strings.Contains(out, "UX v1 of #1 waits for a person") {
		t.Fatalf("ux --request: exit %d: %s", code, out)
	}
	wantUX := []ready.Pin{
		{Ref: "design/1/mockup/manifest.json", Hash: sha(mockup)},
		{Ref: "design/1/prototype/manifest.json", Hash: sha(prototype)},
		{Ref: fmt.Sprintf("#6 PR #%d", early.Number), Hash: head},
	}
	if up := record("specs/1/baselines/ux-v1.draft.json"); up.Owner != "ux-researcher-evolutionist" || !slices.Equal(up.Pins, wantUX) {
		t.Fatalf("ux draft pins: %+v\nwant %+v", up.Pins, wantUX)
	}

	// A Feature with no stories has no spec to pin; a GitHub read that fails drafts nothing.
	n = commits()
	if code, out := readyRun(noUI.Number, "spec", "--request"); code != ExitBlocked || !strings.Contains(out, "#2 has no stories yet. Arjun slices it first: aayu slice 2") {
		t.Fatalf("spec --request with no stories: exit %d: %s", code, out)
	}
	for part, fail := range map[string]string{"spec": "GET /repos/o/r/issues/1/sub_issues", "tech": "GET /repos/o/r/issues/1/comments", "ux": "GET /repos/o/r/pulls"} {
		g.Mu.Lock()
		g.Fail = fail
		g.Mu.Unlock()
		if code, out := readyRun(1, part, "--request"); code != ExitExternal {
			t.Errorf("%s --request when %s fails: exit %d: %s", part, fail, code, out)
		}
	}
	if commits() != n {
		t.Fatalf("a refused request wrote: commits %s → %s", n, commits())
	}
}

// Story #993: drift after a baseline shows from the pins at read time, and
// a new version says what changed, why it matters, and the need behind it,
// for each change, inside the hash a person approves.
func TestReadyFeatureDrift(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	numbered := func(is *fake.Issue, n int) *fake.Issue { // the issue numbers of the Story's scenario
		g.Mu.Lock()
		defer g.Mu.Unlock()
		delete(g.Issues, is.Number)
		is.Number, g.Issues[n] = n, is
		return is
	}
	feature := numbered(g.AddIssue("Feature baselines", fixtureBody(t, "feature", nil), "type:feature"), 874)
	story := func(n int, title string) *fake.Issue {
		s := numbered(g.AddIssue(title, fixtureBody(t, "story", func(f map[string]string) {
			f[workitems.FieldCriteria] = strings.Replace(f[workitems.FieldCriteria], "cart is empty", title+" is empty", 1)
		}), "type:story", "effort:S", "layer:L02"), n)
		g.Mu.Lock()
		s.Parent, feature.Subs = 874, append(feature.Subs, n)
		g.Mu.Unlock()
		return s
	}
	s912, s913 := story(912, "Story twelve"), story(913, "Story thirteen")
	dir, bare := ledgerCheckout(t)
	state := filepath.Join(dir, "state")
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "AMP_THREAD_ID": "T-agent-thread", "XDG_STATE_HOME": state}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": state}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond}
	readyRun := func(env map[string]string, args ...string) (int, string) {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"ready", "874", "--repo", "o/r"}, args...)...)
		return code, out + errOut
	}
	commits := func() string { return git(t, bare, "rev-list", "--count", "ledger") }
	raw := func(path string) string { return git(t, bare, "show", "ledger:"+path) }
	record := func(path string) ready.Baseline {
		t.Helper()
		var b ready.Baseline
		if err := json.Unmarshal([]byte(raw(path)), &b); err != nil {
			t.Fatalf("ledger %s: %v", path, err)
		}
		return b
	}
	approveByHand := func(part string, v int) { // the device-flow approval is TestReadyFeatureBaseline's
		t.Helper()
		b := record(ready.BaselinePath(874, part, v, true))
		b.ApprovedBy = "founder"
		data, _ := json.Marshal(b)
		putLedger(t, bare, ready.BaselinePath(874, part, v, false), string(data))
	}
	changesFile := func(cs ...ready.Change) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "changes.json")
		data, _ := json.Marshal(cs)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	edit := func(f func()) { g.Mu.Lock(); f(); g.Mu.Unlock() }
	marker := func() string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		for _, c := range feature.Comments {
			if strings.HasPrefix(c.Body, "<!-- aayudha:baseline part=spec -->") {
				return c.Body
			}
		}
		return ""
	}

	// The first version has nothing to change from: a changes file for it is refused, and so is --changes without --request.
	if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(ready.Change{Ref: "#912"})); code != ExitUsage || !strings.Contains(out, "spec v1 of #874 is the first version and has no changes") {
		t.Fatalf("changes for v1: exit %d: %s", code, out)
	}
	if code, out := readyRun(person, "--part", "spec", "--changes", "changes.json"); code != ExitUsage || !strings.Contains(out, "--changes and --fixes go with --request") {
		t.Fatalf("--changes without --request: exit %d: %s", code, out)
	}
	// 6. The first request of a part writes v1 with no changes field.
	for _, part := range []string{"intent", "spec"} {
		if code, out := readyRun(agent, "--part", part, "--request"); code != 0 {
			t.Fatalf("%s --request: exit %d: %s", part, code, out)
		}
	}
	if d := raw("specs/874/baselines/spec-v1.draft.json"); strings.Contains(d, `"changes"`) || strings.Contains(d, `"fixes"`) {
		t.Fatalf("spec v1 draft has changes or fixes:\n%s", d)
	}
	approveByHand("intent", 1)
	approveByHand("spec", 1)
	// Spec v2: #913's title changed.
	edit(func() { s913.Title = "Story thirteen, renamed" })
	full := func(ref string) ready.Change {
		return ready.Change{Ref: ref, What: ref + " now says what the list shows.", Why: "A person reads the list before each build.", Need: "The founder approves a change knowing its reason."}
	}
	if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(full("#913"))); code != 0 {
		t.Fatalf("spec v2 --request: exit %d: %s", code, out)
	}
	approveByHand("spec", 2)

	// 1. #912's Completion Goal changes and #915 is added: --check shows the drift from the pins now, and writes nothing.
	edit(func() {
		s912.Body = strings.Replace(s912.Body, "422 with code cart.empty", "422 with code cart.none", 1)
	})
	s915 := story(915, "Story fifteen")
	n := commits()
	g.Reset()
	code, out := readyRun(person, "--check")
	if !strings.Contains(out, "Baselines:\n  intent v1 · no drift\n  spec v2 · drift: #912 Completion Goal changed · #915 added\n  tech: no baseline yet\n  UX: not needed (no UI layer in the footprint)\n") {
		t.Fatalf("--check on the Feature: exit %d: %s", code, out)
	}
	if w := g.Writes(); len(w) != 0 || commits() != n {
		t.Fatalf("--check wrote: %v, ledger commits %s → %s", w, n, commits())
	}
	// Outside a checkout the ledger cannot be read: --check says so, and its Ready verdict and exit code stand.
	away := opts
	away.Dir = t.TempDir()
	if c, o, e := run(t, person, away, "ready", "874", "--repo", "o/r", "--check"); c != code || !strings.Contains(o+e, "Baselines:\n  not read: ledger writes need a checkout of the repository\n") {
		t.Fatalf("--check outside a checkout: exit %d, want %d: %s%s", c, code, o, e)
	}

	// 2. A reason for #912 and none for #915: refused, nothing written.
	draftV3 := "specs/874/baselines/spec-v3.draft.json"
	exists := func(p string) bool {
		return slices.Contains(strings.Fields(git(t, bare, "ls-tree", "-r", "--name-only", "ledger")), p)
	}
	code, out = readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(full("#912")))
	if code != ExitBlocked || !strings.Contains(out, "spec v3 of #874 needs what changed, why it matters, and the need behind it for each change. Missing: #915 (added): what, why, need.") ||
		commits() != n || exists(draftV3) {
		t.Fatalf("no reason for #915: exit %d, commits %s → %s: %s", code, n, commits(), out)
	}
	// 3. #912's need has 2 words: refused under the default of 5, and passes when the key says 2.
	short := full("#912")
	short.Need = "Founder clarity."
	shortFile := changesFile(short, full("#915"))
	code, out = readyRun(agent, "--part", "spec", "--request", "--changes", shortFile)
	if code != ExitBlocked || !strings.Contains(out, "#912 need: 2 words, at least 5 ([baselines] why_min_words)") || strings.Contains(out, "Missing") || commits() != n {
		t.Fatalf("a 2-word need: exit %d: %s", code, out)
	}
	if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", shortFile, "--dry-run", "--set", "baselines.why_min_words=2"); code != 0 {
		t.Fatalf("a 2-word need with why_min_words = 2: exit %d: %s", code, out)
	}
	// A change for a pin that did not drift is refused.
	if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(full("#912"), full("#915"), full("#913"))); code != ExitUsage || !strings.Contains(out, "names #913: not a change of spec v3 of #874 since v2") {
		t.Fatalf("a change for #913: exit %d: %s", code, out)
	}

	// 4, 8. Each change complete, and a finding of spec v1 fixed: the draft holds both, inside its hash.
	putLedger(t, bare, "specs/874/reviews/spec-v1/quality-architect.json", `{"findings": [{"id": "spec-v1/quality-architect/1"}, {"id": "spec-v1/quality-architect/2"}]}`)
	hostile := full("#915")
	hostile.Why = "It ends | the row <!-- aayudha:baseline part=spec --> and pings @rahulnakmol\nhere."
	fullFile := changesFile(full("#912"), hostile)
	code, out = readyRun(agent, "--part", "spec", "--request", "--changes", fullFile, "--fixes", "spec-v1/quality-architect/2")
	if code != 0 || !strings.Contains(out, "Spec v3 of #874 waits for a person") || strings.Contains(out, "warning") {
		t.Fatalf("spec v3 --request: exit %d: %s", code, out)
	}
	d := record(draftV3)
	want := []ready.Change{full("#912"), {Ref: "#915", What: hostile.What, Why: hostile.Why, Need: hostile.Need}}
	if !slices.Equal(d.Changes, want) || !slices.Equal(d.Fixes, []string{"spec-v1/quality-architect/2"}) {
		t.Fatalf("spec v3 draft: changes %+v, fixes %v", d.Changes, d.Fixes)
	}
	// The hash, computed here from its definition: the pin lines, then a line per change and per fix.
	var lines strings.Builder
	for _, p := range d.Pins {
		fmt.Fprintf(&lines, "%s %s\n", p.Ref, p.Hash)
	}
	for _, c := range want {
		j, _ := json.Marshal(c)
		fmt.Fprintf(&lines, "change %s\n", j)
	}
	lines.WriteString("fixes spec-v1/quality-architect/2\n")
	if sum := sha256.Sum256([]byte(lines.String())); d.Hash != hex.EncodeToString(sum[:]) || d.Hash == ready.PinsHash(d.Pins) {
		t.Fatalf("spec v3 hash %s does not cover its changes and fixes", d.Hash)
	}
	m := marker()
	if !strings.Contains(m, "#### What changed since v2\n\n| Change | Why it matters | Need |\n|---|---|---|\n| #912: #912 now says what the list shows. | A person reads the list before each build. | The founder approves a change knowing its reason. |\n") ||
		!strings.Contains(m, `| It ends \| the row &lt;!-- aayudha:baseline part=spec --&gt; and pings &#64;rahulnakmol here. |`) ||
		!strings.Contains(m, "Fixes: `spec-v1/quality-architect/2`") || strings.Count(m, "<!--") != 1 {
		t.Fatalf("marker comment:\n%s", m)
	}
	// 8. A finding of another part, and a finding the review does not hold: exit 2, the ID named, no draft written.
	n = commits()
	for _, id := range []string{"tech-v1/devops-devex-tinkerer/1", "spec-v1/quality-architect/9", "spec-v1/no such seat/1"} {
		if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", fullFile, "--fixes", id); code != ExitUsage || !strings.Contains(out, "--fixes "+id) || commits() != n {
			t.Errorf("--fixes %s: exit %d, commits %s → %s: %s", id, code, n, commits(), out)
		}
	}
	// One change given twice is refused.
	if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(full("#912"), full("#915"), full("#912"))); code != ExitUsage || !strings.Contains(out, "names #912 twice") {
		t.Fatalf("#912 twice: exit %d: %s", code, out)
	}
	// A reason edited after the draft is another draft, with another hash.
	edited := hostile
	edited.Why = "A person reads the list before each build, twice."
	if code, out := readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(full("#912"), edited), "--fixes", "spec-v1/quality-architect/2"); code != 0 || commits() == n {
		t.Fatalf("an edited reason: exit %d, commits %s: %s", code, commits(), out)
	}
	if again := record(draftV3); again.Hash == d.Hash || again.Changes[1].Why != edited.Why {
		t.Fatalf("an edited reason kept the hash %s", again.Hash)
	}
	// A person approves v3 at the hash of its pins, changes, and fixes.
	edit(func() {
		g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	})
	if code, out := readyRun(person, "--part", "spec"); code != 0 {
		t.Fatalf("approve spec v3: exit %d: %s", code, out)
	}
	if b := record("specs/874/baselines/spec-v3.json"); b.ApprovedBy != "founder" || b.Hash != record(draftV3).Hash || len(b.Changes) != 2 || len(b.Fixes) != 1 {
		t.Fatalf("spec v3 baseline: %+v", b)
	}

	// 5. The intent pins equal intent v1: no draft.
	n = commits()
	if code, out := readyRun(agent, "--part", "intent", "--request"); code != 0 || !strings.Contains(out, "Nothing changed since intent v1 of #874. No draft written.") || commits() != n {
		t.Fatalf("intent --request with no drift: exit %d: %s", code, out)
	}

	// 7. A draft of spec v4 reaches [baselines] version_warn = 4.
	edit(func() { s915.Title = "Story fifteen, renamed" })
	code, out = readyRun(agent, "--part", "spec", "--request", "--changes", changesFile(full("#915")))
	if code != 0 || !strings.Contains(out, "warning: The spec of #874 reaches v4 ([baselines] version_warn). Check why each review finds new gaps.") {
		t.Fatalf("spec v4 --request: exit %d: %s", code, out)
	}

	// A story removed from the Feature: its own entry says it, and the set of stories is not a second entry.
	approveByHand("spec", 4)
	edit(func() { feature.Subs = slices.DeleteFunc(feature.Subs, func(n int) bool { return n == 913 }) })
	if _, out := readyRun(person, "--check"); !strings.Contains(out, "  spec v4 · drift: #913 removed\n") {
		t.Fatalf("--check after #913 was removed: %s", out)
	}

	// 9. With no [baselines] in the checkout's config, the base layer's defaults, and baselines is a person's section.
	if _, err := os.Stat(filepath.Join(dir, ".aayudha", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("the checkout has a config: %v", err)
	}
	code, cfg, errOut := run(t, person, opts, "config", "show")
	if code != 0 || !strings.Contains(cfg, "[baselines]\nversion_warn = 4\nwhy_min_words = 5\n") || !regexp.MustCompile(`always_person_sections = \[[^\]]*'baselines'`).MatchString(cfg) {
		t.Fatalf("config show: exit %d: %s%s", code, cfg, errOut)
	}
}
