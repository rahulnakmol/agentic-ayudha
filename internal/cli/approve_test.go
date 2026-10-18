package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// Story #481 (Decision #488): an agent starts `decide a,b --accept x,y` and
// the batch goes to the person as one comment. With a bot identity, a
// recorded approver, and no approver credential in reach, an approver's
// reply approves it through the aayudha-approve workflow (inbox --ci);
// every refusal says why and names the next step, and nothing is accepted.
func TestDecideBatchApprovedByReply(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	g.TokenLogins = map[string]fake.Account{"bot-token": {Login: "aayudha-bot[bot]", Type: "Bot"}}
	g.Permissions = map[string]string{"founder": "admin", "mallory": "write"}
	d1 := g.AddIssue("Choose the rules engine", decisionBody("true", ""), "type:decision", "status:ready")
	d2 := g.AddIssue("Choose the queue", decisionBody("true", ""), "type:decision", "status:ready")
	d4 := g.AddIssue("Choose the cache", decisionBody("true", ""), "type:decision", "status:ready")
	d5 := g.AddIssue("Choose the CDN", decisionBody("true", ""), "type:decision", "status:ready")
	d6 := g.AddIssue("Choose the CI runner", decisionBody("true", ""), "type:decision", "status:ready")
	d3 := g.AddIssue("Choose the data store", decisionBody("false", ""), "type:decision", "status:ready")
	sets := []string{"--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder"}
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "founder-token", "AAYU_BOT_TOKEN": "bot-token", "CODEX_THREAD_ID": "019a"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond}
	comments := func(n int) []*fake.Comment {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		return append([]*fake.Comment{}, g.Issues[n].Comments...)
	}
	last := func(n int) string { cs := comments(n); return cs[len(cs)-1].Body }
	thread := d1.Number // the issue the batch and the replies are on
	nextID := int64(5000)
	reply := func(login, body string, edited bool, at ...time.Time) int64 {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		is := g.Issues[thread]
		when := time.Now().UTC().Add(time.Second)
		if len(at) > 0 {
			when = at[0]
		}
		nextID++
		c := &fake.Comment{ID: nextID, Body: body, CreatedAt: when,
			HTMLURL: "https://github.com/o/r/issues/" + strconv.Itoa(is.Number) + "#issuecomment-" + strconv.FormatInt(nextID, 10)}
		if edited {
			c.UpdatedAt = c.CreatedAt.Add(time.Minute)
		}
		c.User.Login, c.User.Type = login, "User"
		is.Comments = append(is.Comments, c)
		return c.ID
	}
	// workflow runs the aayudha-approve job on reply id: event name and action
	// as GitHub sends them unless given, and the repository's config.
	workflow := func(id int64, over ...string) (int, string) {
		t.Helper()
		name, action, cfg := "issue_comment", "created", sets
		switch {
		case len(over) == 2:
			name, action = over[0], over[1]
		case len(over) > 2:
			cfg = over
		}
		ev := filepath.Join(t.TempDir(), "event.json")
		raw, _ := json.Marshal(map[string]any{"action": action, "comment": map[string]any{"id": id}, "issue": map[string]any{"number": thread}})
		_ = os.WriteFile(ev, raw, 0o644)
		ci := map[string]string{"AAYU_GITHUB_TOKEN": "workflow-token", "AAYU_BOT_TOKEN": "bot-token", "GITHUB_EVENT_NAME": name, "GITHUB_EVENT_PATH": ev}
		code, out, errOut := run(t, ci, opts, append([]string{"inbox", "--ci"}, cfg...)...)
		return code, out + errOut
	}
	accepted := func(n int) bool {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		return g.Issues[n].State == "closed"
	}

	// The agent's batch: #d1 and #d4 at the recommendation (A), #d2 at another option.
	list := strconv.Itoa(d1.Number) + "," + strconv.Itoa(d2.Number) + "," + strconv.Itoa(d4.Number)
	code, _, errOut := run(t, agent, opts, append([]string{"decide", list, "--accept", "A,B,A"}, sets...)...)
	batch := last(d1.Number)
	if code != ExitBlocked || !strings.Contains(errOut, "nothing is accepted until an approver replies") || accepted(d1.Number) ||
		!strings.Contains(batch, "@founder **3 human-only steps wait for your approval**") ||
		!strings.Contains(batch, "1. **Accepts option A of Decision #"+strconv.Itoa(d1.Number)+": Choose the rules engine.** Two-way door") ||
		!strings.Contains(batch, "This is not the recommendation.") || !strings.Contains(batch, "`/aayu approve 2`") || strings.Contains(batch, "login/device") {
		t.Fatalf("batch: exit %d %s\n%s", code, errOut, batch)
	}
	if cs := comments(d1.Number); cs[len(cs)-1].User.Login != "aayudha-bot[bot]" {
		t.Fatalf("the batch was posted as @%s, not the bot", cs[len(cs)-1].User.Login)
	}
	// A batch is one option per Decision, each Decision once.
	for args, want := range map[string]string{
		list:                   "a list of Decisions is a batch for --accept",
		list + " --accept A,A": "3 Decision(s) but 2 option(s)",
		strconv.Itoa(d2.Number) + "," + strconv.Itoa(d2.Number) + " --accept A,A": "is in the batch twice",
		"1,2,3,4,5,6,7,8,9,10,11 --accept A,A,A,A,A,A,A,A,A,A,A":                  "at most 10 Decisions in one batch",
	} {
		if code, _, errOut := run(t, agent, opts, append(append([]string{"decide"}, strings.Fields(args)...), sets...)...); code != ExitUsage || !strings.Contains(errOut, want) {
			t.Fatalf("decide %s: %d %s", args, code, errOut)
		}
	}
	// A second batch on the same issue waits for the first.
	if code, _, errOut := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d1.Number), "--accept", "A"}, sets...)...); code != ExitBlocked || !strings.Contains(errOut, "is still open") {
		t.Fatalf("second batch: %d %s", code, errOut)
	}

	none := func() bool { return !accepted(d1.Number) && !accepted(d2.Number) && !accepted(d4.Number) }
	// Refusals: each is answered on the thread, and nothing is accepted.
	g.Mu.Lock()
	body1 := g.Issues[d1.Number].Body
	g.Mu.Unlock()
	for _, tt := range []struct {
		login, body string
		edited      bool
		before      bool   // the reply is dated before the batch
		edit        string // the Decision's body is edited first
		want        string
	}{
		{login: "mallory", body: "/aayu approve all", want: "@mallory is not a recorded approver"},
		{login: "founder", body: "/aayu approve all", edited: true, want: "your reply was edited"},
		{login: "founder", body: "/aayu approve all", before: true, want: "your reply is older than the batch"},
		{login: "founder", body: "/aayu approve 4", want: "the batch has items 1 to 3, not 4"},
		{login: "founder", body: "/aayu skip 1 3", want: "Item 2 was left out: it is not the recommendation"},
		{login: "founder", body: "/aayu approve 1 3", edit: body1 + "\nEdited.", want: "changed after the batch was posted"},
	} {
		at := []time.Time{}
		if tt.before {
			at = append(at, time.Now().UTC().Add(-time.Hour))
		}
		g.Mu.Lock()
		if tt.edit != "" {
			g.Issues[d1.Number].Body = tt.edit
		}
		g.Mu.Unlock()
		code, out := workflow(reply(tt.login, tt.body, tt.edited, at...))
		g.Mu.Lock()
		g.Issues[d1.Number].Body = body1
		g.Mu.Unlock()
		if code != ExitBlocked || !strings.Contains(last(d1.Number), tt.want) || !strings.Contains(last(d1.Number), "Next: ") || !none() {
			t.Fatalf("%s %q: exit %d %s\n%s", tt.login, tt.body, code, out, last(d1.Number))
		}
	}
	// The repository's own setup: no bot identity, or no approver recorded.
	for cfg, want := range map[string]string{
		"approvals.approvers=founder": "this repository has no bot identity",
		"identity.app_id=42":          "no approver is recorded",
	} {
		args := []string{"--repo", "o/r", "--set", cfg}
		if cfg == "identity.app_id=42" {
			args = append(args, "--set", "identity.app_slug=aayudha-bot")
		}
		if code, out := workflow(reply("founder", "/aayu approve all", false), args...); code != ExitBlocked || !strings.Contains(last(d1.Number), want) || !none() {
			t.Fatalf("%s: %d %s\n%s", cfg, code, out, last(d1.Number))
		}
	}
	// The job cannot read its event: nothing is applied.
	for name, raw := range map[string]string{"missing": "", "malformed": "{"} {
		ev := filepath.Join(t.TempDir(), "event.json")
		if raw != "" {
			_ = os.WriteFile(ev, []byte(raw), 0o644)
		}
		ci := map[string]string{"AAYU_GITHUB_TOKEN": "workflow-token", "AAYU_BOT_TOKEN": "bot-token", "GITHUB_EVENT_NAME": "issue_comment", "GITHUB_EVENT_PATH": ev}
		if code, _, errOut := run(t, ci, opts, append([]string{"inbox", "--ci"}, sets...)...); code != ExitUsage || !strings.Contains(errOut, "inbox --ci:") || !none() {
			t.Fatalf("%s event: %d %s", name, code, errOut)
		}
	}
	// Only a new reply on an issue counts: another event or an edit is not an approval.
	id := reply("founder", "/aayu approve all", false)
	if code, _ := workflow(id, "pull_request_review", "submitted"); code != ExitUsage || !none() {
		t.Fatalf("another event: %d", code)
	}
	if code, out := workflow(id, "issue_comment", "edited"); code != 0 || !strings.Contains(out, "a edited comment is never an approval") || !none() {
		t.Fatalf("an edit event: %d %s", code, out)
	}
	// The Decision's authority is someone else: the reply is not theirs to give.
	authority := append(append([]string{}, sets...), "--set", "seats.founder.mode=paired", "--set", "seats.founder.human=boss")
	if code, out := workflow(reply("founder", "/aayu approve 1", false), authority...); code != ExitBlocked || !strings.Contains(last(d1.Number), "the authority for #"+strconv.Itoa(d1.Number)+" is boss") || !none() {
		t.Fatalf("not the authority: %d %s\n%s", code, out, last(d1.Number))
	}
	// A bot's reply, one without write access, and one that is not a command get no answer and change nothing.
	g.Mu.Lock()
	g.Permissions["visitor"] = "read"
	g.Mu.Unlock()
	for _, who := range []string{"aayudha-bot[bot]", "visitor", "founder", "founder "} {
		body := "/aayu approve all"
		switch who {
		case "founder":
			body = "thanks!"
		case "founder ":
			who, body = "founder", "/aayu skip all"
		}
		n := len(comments(d1.Number))
		if code, out := workflow(reply(who, body, false)); code != 0 || len(comments(d1.Number)) != n+1 || !none() {
			t.Fatalf("%s %q: %d %s", who, body, code, out)
		}
	}

	// `/aayu approve 1 3` accepts exactly those items; the result names who
	// approved and where.
	id = reply("founder", "/aayu approve 1 3", false)
	if code, out := workflow(id); code != 0 || !accepted(d1.Number) || accepted(d2.Number) || !accepted(d4.Number) {
		t.Fatalf("approve 1 3: %d %s", code, out)
	}
	url := "https://github.com/o/r/issues/" + strconv.Itoa(d1.Number) + "#issuecomment-" + strconv.FormatInt(id, 10)
	res := last(d1.Number)
	g.Mu.Lock()
	body := g.Issues[d1.Number].Body
	g.Mu.Unlock()
	if !strings.Contains(res, "1. #"+strconv.Itoa(d1.Number)+" accepted: option A, by @founder in "+url) || !strings.Contains(res, "3. #"+strconv.Itoa(d4.Number)+" accepted: option A") ||
		!strings.Contains(res, "Skipped: 2.") || !strings.Contains(body, "accepted by @founder") || !strings.Contains(body, "Approved in "+url) {
		t.Fatalf("result:\n%s\nbody:\n%s", res, body)
	}
	// The batch is answered: a later reply approves nothing more.
	if code, _ := workflow(reply("founder", "/aayu approve 2", false)); code != ExitBlocked || !strings.Contains(last(d1.Number), "there is no open batch") || accepted(d2.Number) {
		t.Fatalf("reply after the result: %d\n%s", code, last(d1.Number))
	}

	// An edited batch never counts: it may not say what was asked.
	thread = d2.Number
	if code, _, errOut := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d2.Number), "--accept", "A"}, sets...)...); code != ExitBlocked {
		t.Fatalf("batch on #%d: %d %s", d2.Number, code, errOut)
	}
	g.Mu.Lock()
	cs := g.Issues[d2.Number].Comments
	cs[len(cs)-1].UpdatedAt = cs[len(cs)-1].CreatedAt.Add(time.Minute)
	g.Mu.Unlock()
	if code, _ := workflow(reply("founder", "/aayu approve all", false)); code != ExitBlocked || !strings.Contains(last(d2.Number), "the batch comment was edited") || accepted(d2.Number) {
		t.Fatalf("edited batch: %d\n%s", code, last(d2.Number))
	}

	// A batch the bot's comment does not match (its shown hash is not its
	// record's) never counts.
	thread = d5.Number
	if code, _, _ := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d5.Number) + "," + strconv.Itoa(d6.Number), "--accept", "A,A"}, sets...)...); code != ExitBlocked {
		t.Fatalf("batch on #%d: %d", d5.Number, code)
	}
	g.Mu.Lock()
	bc := g.Issues[d5.Number].Comments[len(g.Issues[d5.Number].Comments)-1]
	real := bc.Body
	_, rest, _ := strings.Cut(real, "\n")
	bc.Body = batchMarker + "000000000000 -->\n" + rest
	g.Mu.Unlock()
	if code, _ := workflow(reply("founder", "/aayu approve all", false)); code != ExitBlocked || !strings.Contains(last(d5.Number), "the batch's hash does not match its record") || accepted(d5.Number) {
		t.Fatalf("forged hash: %d\n%s", code, last(d5.Number))
	}
	// GitHub fails on the second item: the first stays accepted, and the
	// result says the rest was not applied.
	g.Mu.Lock()
	bc.Body = real
	g.Fail = "PATCH /repos/o/r/issues/" + strconv.Itoa(d6.Number)
	g.Mu.Unlock()
	if code, _ := workflow(reply("founder", "/aayu approve all", false)); code != ExitExternal || !accepted(d5.Number) || accepted(d6.Number) ||
		!strings.Contains(last(d5.Number), "2. #"+strconv.Itoa(d6.Number)+" was not accepted") || !strings.Contains(last(d5.Number), "The rest was not applied.") {
		t.Fatalf("partial failure: %d\n%s", code, last(d5.Number))
	}

	// An agent that loosens its own config (device_for = []) gets a one-way
	// door into a reply batch, and the workflow, which reads the repository's
	// config, still refuses it.
	thread = d3.Number
	loose := t.TempDir()
	_ = os.MkdirAll(filepath.Join(loose, "aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(loose, "aayudha", "config.toml"), []byte("version = 1\n[approvals]\ndevice_for = []\n"), 0o644)
	agentLoose := map[string]string{"XDG_CONFIG_HOME": loose}
	for k, v := range agent {
		agentLoose[k] = v
	}
	if code, _, errOut := run(t, agentLoose, opts, append([]string{"decide", strconv.Itoa(d3.Number), "--accept", "A"}, sets...)...); code != ExitBlocked || !strings.Contains(last(d3.Number), "/aayu approve all") {
		t.Fatalf("loosened batch: %d %s\n%s", code, errOut, last(d3.Number))
	}
	if code, _ := workflow(reply("founder", "/aayu approve all", false)); code != ExitBlocked || !strings.Contains(last(d3.Number), "item 1 is a one-way door") || accepted(d3.Number) {
		t.Fatalf("one-way door by reply: %d\n%s", code, last(d3.Number))
	}

	// A one-way door stays on the device code (device_for = ["one-way"] by
	// default): the batch carries the code, and a denial says so on the thread.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "deny"}
	g.Mu.Unlock()
	code, _, errOut = run(t, agent, opts, append([]string{"decide", strconv.Itoa(d3.Number), "--accept", "A"}, sets...)...)
	if code != ExitBlocked || accepted(d3.Number) || !strings.Contains(last(d3.Number), "The request was cancelled on GitHub; nothing was done.") ||
		!strings.Contains(last(d3.Number), "item 1 is a one-way door") {
		t.Fatalf("one-way door, denied: %d %s\n%s", code, errOut, last(d3.Number))
	}
	// Approved: the code covers the batch, and the result goes back to the thread.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Mu.Unlock()
	code, out, errOut := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d3.Number), "--accept", "A"}, sets...)...)
	if code != 0 || !accepted(d3.Number) || !strings.Contains(out, "accepted: A, by @founder through the device code") ||
		!strings.Contains(errOut, "Approving: Accepts option A of Decision #"+strconv.Itoa(d3.Number)) ||
		!strings.Contains(last(d3.Number), "accepted: option A, by @founder through the device code") {
		t.Fatalf("one-way door, approved: %d %s %s\n%s", code, out, errOut, last(d3.Number))
	}
}

// The approver's own credential in this session turns comment approval off:
// an agent here could write the reply. The batch falls back to the device
// code and says why; so does `[approvals] device_for = ["all"]`.
func TestBatchFallsBackWhenApproverCredentialIsReachable(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	g.TokenLogins = map[string]fake.Account{"bot-token": {Login: "aayudha-bot[bot]", Type: "Bot"}}
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "deny"}
	d := g.AddIssue("Choose the rules engine", decisionBody("true", ""), "type:decision", "status:ready")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "founder-token", "AAYU_BOT_TOKEN": "bot-token", "AMP_THREAD_ID": "T-1", "GH_TOKEN": "ghu_reachablefortest"}
	code, _, errOut := run(t, env, Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond},
		"decide", strconv.Itoa(d.Number), "--accept", "A", "--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder")
	g.Mu.Lock()
	body := g.Issues[d.Number].Comments[len(g.Issues[d.Number].Comments)-1].Body
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(body, "this session can reach @founder's GitHub credential (GH_TOKEN)") || strings.Contains(body, "/aayu approve all") || strings.Contains(body, "ghu_") {
		t.Fatalf("exit %d %s\n%s", code, errOut, body)
	}
	// device_for = ["all"] keeps every batch on the code, whatever the session holds.
	d2 := g.AddIssue("Choose the queue", decisionBody("true", ""), "type:decision", "status:ready")
	delete(env, "GH_TOKEN")
	code, _, errOut = run(t, env, Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond},
		"decide", strconv.Itoa(d2.Number), "--accept", "A", "--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder", "--set", "approvals.device_for=all")
	g.Mu.Lock()
	body = g.Issues[d2.Number].Comments[len(g.Issues[d2.Number].Comments)-1].Body
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(body, "keeps every batch on the device code") {
		t.Fatalf("device_for all: exit %d %s\n%s", code, errOut, body)
	}
}

// Story #481: with an approver recorded but no bot identity, an agent's batch of three Decisions asks
// for one device code in one comment, which says why replies are off. Each
// failure (expired, cancelled, another account, too little access) writes no
// Decision and replaces the code with what happened and the next step; each
// run asks for a new code. One approval accepts all three, the record lists
// them, and each Decision gets the result.
func TestDecideBatchDeviceCode(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	var ds []*fake.Issue
	for _, title := range []string{"Choose the rules engine", "Choose the queue", "Choose the data store"} {
		ds = append(ds, g.AddIssue(title, decisionBody(strconv.FormatBool(title != "Choose the data store"), ""), "type:decision", "status:ready"))
	}
	list := strconv.Itoa(ds[0].Number) + "," + strconv.Itoa(ds[1].Number) + "," + strconv.Itoa(ds[2].Number)
	state := t.TempDir()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "founder-token", "CURSOR_AGENT": "1", "XDG_STATE_HOME": state}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond}
	batch := func() string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		for _, c := range g.Issues[ds[0].Number].Comments {
			if strings.HasPrefix(c.Body, batchMarker) {
				return c.Body
			}
		}
		return ""
	}
	closed := func() (n int) {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		for _, d := range ds {
			if g.Issues[d.Number].State == "closed" {
				n++
			}
		}
		return n
	}
	for _, tt := range []struct {
		name string
		dev  fake.DeviceFlow
		perm string
		want string
	}{
		{"expired", fake.DeviceFlow{Approver: "founder", ExpiresIn: 1}, "", "The code expired at"},
		{"cancelled", fake.DeviceFlow{Approver: "founder", Outcome: "deny"}, "", "The request was cancelled on GitHub; nothing was done."},
		{"another account", fake.DeviceFlow{Approver: "mallory", Outcome: "approve"}, "", "@mallory approved it, but aayu acts as @founder here; nothing was done."},
		{"too little access", fake.DeviceFlow{Approver: "founder", Outcome: "approve"}, "triage", "@founder has triage access to o/r, and this needs write; nothing was done."},
	} {
		d := tt.dev
		d.ClientID = DefaultDeviceClientID
		g.Mu.Lock()
		g.Device = &d
		g.Permissions = map[string]string{"founder": orDefault(tt.perm, "admin")}
		g.Mu.Unlock()
		code, _, errOut := run(t, env, opts, "decide", list, "--accept", "A,A,B", "--repo", "o/r", "--set", "approvals.approvers=founder")
		if b := batch(); code != ExitBlocked || closed() != 0 || d.Started != 1 || !strings.Contains(b, tt.want) || !strings.Contains(b, "\nNext: ") ||
			strings.Contains(b, "WDJB-MJHT") {
			t.Fatalf("%s: exit %d, %d closed, %d codes: %s\n%s", tt.name, code, closed(), d.Started, errOut, b)
		}
	}

	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", Pending: 1}
	g.Permissions = map[string]string{"founder": "admin"}
	g.Mu.Unlock()
	g.Reset()
	code, out, errOut := run(t, env, opts, "decide", list, "--accept", "A,A,B", "--repo", "o/r", "--set", "approvals.approvers=founder")
	// The code reached the batch comment before aayu waited for it.
	shown := ""
	for _, w := range g.Writes() {
		if body, _ := w.Body["body"].(string); strings.HasPrefix(body, batchMarker) && strings.Contains(body, "WDJB-MJHT") {
			shown = body
		}
	}
	if code != 0 || closed() != 3 || !strings.Contains(out, "accepted: B, by @founder through the device code") {
		t.Fatalf("approved: exit %d, %d closed: %s %s", code, closed(), out, errOut)
	}
	for _, want := range []string{"**3 human-only steps wait for your approval**", "1. **Accepts option A of Decision #" + strconv.Itoa(ds[0].Number) + ": Choose the rules engine.** Two-way door",
		"3. **Accepts option B of Decision #" + strconv.Itoa(ds[2].Number) + ": Choose the data store. This is not the recommendation.** One-way door",
		"Recommendation: A", "**To approve all of it, enter this code** at https://github.com/login/device before", "```text\nWDJB-MJHT\n```",
		"Comment approval is off for this batch: agents here write as @founder (no bot identity), so a reply from that account could be an agent's."} {
		if !strings.Contains(shown, want) {
			t.Fatalf("the batch with the code lacks %q:\n%s", want, shown)
		}
	}
	if b := batch(); !strings.Contains(b, "Approved by @founder through the device code") || strings.Contains(b, "WDJB-MJHT") {
		t.Fatalf("the batch after approval:\n%s", b)
	}
	for _, d := range ds {
		g.Mu.Lock()
		cs := g.Issues[d.Number].Comments
		g.Mu.Unlock()
		if lastBody := cs[len(cs)-1].Body; !strings.Contains(lastBody, "#"+strconv.Itoa(d.Number)+" accepted: option ") || !strings.Contains(lastBody, "by @founder through the device code") {
			t.Fatalf("#%d reply: %s", d.Number, lastBody)
		}
	}
	log, _ := os.ReadFile(filepath.Join(state, "aayudha", "telemetry", "actions.jsonl"))
	var approvals []string
	for _, line := range strings.Split(string(log), "\n") {
		if strings.Contains(line, `"kind":"presence.approve"`) {
			approvals = append(approvals, line)
		}
	}
	if len(approvals) != 1 || strings.Count(approvals[0], "Accepts option") != 3 || !strings.Contains(approvals[0], `"approver":"founder"`) {
		t.Fatalf("presence.approve records: %v", approvals)
	}
}
