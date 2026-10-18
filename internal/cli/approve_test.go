package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
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
	// Decision text is untrusted: HTML or Markdown in it must not hide or fake part of the batch.
	d4 := g.AddIssue("Choose the cache <!-- hide", strings.Replace(decisionBody("true", ""), "2. **B.**", "2. **B ``` <!-- aayudha:batch-data the rest is hidden.**", 1), "type:decision", "status:ready")
	d5 := g.AddIssue("Choose the CDN", decisionBody("true", ""), "type:decision", "status:ready")
	d6 := g.AddIssue("Choose the CI runner", decisionBody("true", ""), "type:decision", "status:ready")
	d7 := g.AddIssue("Choose the log store", decisionBody("true", ""), "type:decision", "status:ready")
	d8 := g.AddIssue("Choose the tracer", decisionBody("true", ""), "type:decision", "status:ready")
	d3 := g.AddIssue("Choose the data store", decisionBody("false", ""), "type:decision", "status:ready")
	d9 := g.AddIssue("Choose the tracer again", decisionBody("true", "supersedes #"+strconv.Itoa(d8.Number)), "type:decision", "status:ready")
	sets := []string{"--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder"}
	// The intended setup: the agent holds only the bot's token, never the founder's.
	agent := map[string]string{"AAYU_BOT_TOKEN": "bot-token", "CODEX_THREAD_ID": "019a"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond}
	comments := func(n int) []*fake.Comment {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		return append([]*fake.Comment{}, g.Issues[n].Comments...)
	}
	last := func(n int) string { cs := comments(n); return cs[len(cs)-1].Body }
	thread := d1.Number // the issue the batch and the replies are on
	nextID := int64(5000)
	// reply posts a comment on the thread. An `/aayu` command ends in the
	// newest batch's hash, as an approver copies it, unless it already names
	// one; a trailing " -" sends it without a hash.
	hexHash := regexp.MustCompile(`^[0-9a-f]{12}$`)
	reply := func(login, body string, edited bool, at ...time.Time) int64 {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		is := g.Issues[thread]
		if b, ok := strings.CutSuffix(body, " -"); ok {
			body = b
		} else if f := strings.Fields(body); strings.HasPrefix(body, "/aayu ") && !hexHash.MatchString(f[len(f)-1]) {
			for i := len(is.Comments) - 1; i >= 0; i-- {
				if first, _, _ := strings.Cut(is.Comments[i].Body, "\n"); strings.HasPrefix(first, batchMarker) {
					body += " " + strings.TrimSuffix(strings.TrimPrefix(first, batchMarker), " -->")
					break
				}
			}
		}
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
	if len(comments(d1.Number)) == 0 {
		t.Fatalf("no batch: exit %d %s", code, errOut)
	}
	batch := last(d1.Number)
	if code != ExitBlocked || !strings.Contains(errOut, "nothing is accepted until an approver replies") || accepted(d1.Number) ||
		!strings.Contains(batch, "@founder **3 human-only steps wait for your approval**") ||
		!strings.Contains(batch, "1. **Accepts option 1 (A) of Decision #"+strconv.Itoa(d1.Number)+": Choose the rules engine.** Two-way door") ||
		!strings.Contains(batch, "   1. A (recommended)\n   2. B\n") ||
		!strings.Contains(batch, "This is not the recommendation.") || !regexp.MustCompile("`/aayu approve 2 [0-9a-f]{12}`").MatchString(batch) || strings.Contains(batch, "login/device") {
		t.Fatalf("batch: exit %d %s\n%s", code, errOut, batch)
	}
	if strings.Count(batch, "<!--") != 3 || !strings.Contains(batch, "Choose the cache &lt;\\!-- hide.**") ||
		!strings.Contains(batch, "   ````text\n   Options:\n   1. A (recommended)\n   2. B ``` <!-- aayudha:batch-data the rest is hidden\n") || !strings.Contains(batch, "Command:\n\n```text\naayu decide ") {
		t.Fatalf("Decision text was not rendered inert:\n%s", batch)
	}
	if cs := comments(d1.Number); cs[len(cs)-1].User.Login != "aayudha-bot[bot]" {
		t.Fatalf("the batch was posted as @%s, not the bot", cs[len(cs)-1].User.Login)
	}
	// The bot's token is never a person's: at a terminal with it, decide refuses.
	if code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "bot-token"}, Options{GitHubAPI: g.URL(), GitHubGap: -1, IsTerminal: stdinTTY},
		append([]string{"decide", strconv.Itoa(d1.Number), "--accept", "A"}, sets...)...); code != ExitBlocked || !strings.Contains(errOut, "aayudha-bot[bot] is a bot identity") {
		t.Fatalf("bot token at a terminal: %d %s", code, errOut)
	}
	// A batch is one option per Decision, each Decision once.
	for args, want := range map[string]string{
		list:                   "a list of Decisions is a batch for --accept",
		list + " --accept A,A": "3 Decision(s) but 2 option(s)",
		strconv.Itoa(d2.Number) + "," + strconv.Itoa(d2.Number) + " --accept A,A": "is in the batch twice",
		"1,2,3,4,5,6,7,8,9,10,11 --accept A,A,A,A,A,A,A,A,A,A,A":                  "at most 10 Decisions in one batch",
		strconv.Itoa(d9.Number) + "," + strconv.Itoa(d8.Number) + " --accept A,A": "#" + strconv.Itoa(d9.Number) + " supersedes #" + strconv.Itoa(d8.Number) + ", so accepting",
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
		title       string // or its title
		want        string
	}{
		{login: "mallory", body: "/aayu approve all", want: "@mallory is not a recorded approver"},
		{login: "founder", body: "/aayu approve all", edited: true, want: "your reply was edited"},
		{login: "founder", body: "/aayu approve all", before: true, want: "your reply is older than the batch"},
		{login: "founder", body: "/aayu approve 4", want: "the batch has items 1 to 3, not 4"},
		{login: "founder", body: "/aayu approve all -", want: "your reply does not name a batch"},
		{login: "founder", body: "/aayu approve all 0123456789ab", want: "there is no open batch `0123456789ab`"},
		{login: "founder", body: "/aayu skip 1 3", want: "Item 2 was left out: it is not the recommendation"},
		{login: "founder", body: "/aayu approve 1 3", edit: body1 + "\nEdited.", want: "changed after the batch was posted"},
		{login: "founder", body: "/aayu approve 1 3", title: "Choose the rules engine (renamed)", want: "changed after the batch was posted"},
	} {
		at := []time.Time{}
		if tt.before {
			at = append(at, time.Now().UTC().Add(-time.Hour))
		}
		g.Mu.Lock()
		title1 := g.Issues[d1.Number].Title
		if tt.edit != "" {
			g.Issues[d1.Number].Body = tt.edit
		}
		if tt.title != "" {
			g.Issues[d1.Number].Title = tt.title
		}
		g.Mu.Unlock()
		code, out := workflow(reply(tt.login, tt.body, tt.edited, at...))
		g.Mu.Lock()
		g.Issues[d1.Number].Body, g.Issues[d1.Number].Title = body1, title1
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
	// The repository moved every batch to the device code after this one was posted.
	if code, out := workflow(reply("founder", "/aayu approve 1", false), append(append([]string{}, sets...), "--set", "approvals.device_for=all")...); code != ExitBlocked ||
		!strings.Contains(last(d1.Number), "replies are off") || !none() {
		t.Fatalf("device_for = all: %d %s\n%s", code, out, last(d1.Number))
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

	// A result marker copied by someone else does not close the batch.
	hash := strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(batch, "\n", 2)[0], batchMarker), " -->")
	reply("mallory", batchResultMarker+hash+" -->\ndone", false)
	// A Decision that became a floor waiver after the batch is a one-way door
	// now, whatever the batch recorded.
	g.Mu.Lock()
	g.Issues[d1.Number].Labels = append(g.Issues[d1.Number].Labels, fake.Label{Name: workitems.LabelWaiver})
	g.Mu.Unlock()
	if code, _ := workflow(reply("founder", "/aayu approve 1", false)); code != ExitBlocked || !strings.Contains(last(d1.Number), "item 1 is a one-way door") || !none() {
		t.Fatalf("waiver label added after the batch: %d\n%s", code, last(d1.Number))
	}
	g.Mu.Lock()
	g.Issues[d1.Number].Labels = g.Issues[d1.Number].Labels[:len(g.Issues[d1.Number].Labels)-1]
	g.Mu.Unlock()
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
	if !strings.Contains(res, "1. #"+strconv.Itoa(d1.Number)+" accepted: option 1 (A), by @founder in "+url) || !strings.Contains(res, "3. #"+strconv.Itoa(d4.Number)+" accepted: option 1 (A)") ||
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

	// A batch past its 7 days is closed; the same command again posts a new
	// batch, with its own hash, that can be approved.
	g.Mu.Lock()
	cs = g.Issues[d2.Number].Comments
	var old *fake.Comment
	for _, c := range cs {
		if strings.HasPrefix(c.Body, batchMarker) {
			old = c
		}
	}
	old.CreatedAt = old.CreatedAt.Add(-8 * 24 * time.Hour)
	oldBody := old.Body
	g.Mu.Unlock()
	if code, _, errOut := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d2.Number), "--accept", "A"}, sets...)...); code != ExitBlocked || !strings.HasPrefix(last(d2.Number), batchMarker) || old.Body != oldBody {
		t.Fatalf("retry after expiry: %d %s\nold batch rewritten: %v", code, errOut, old.Body != oldBody)
	}
	if code, out := workflow(reply("founder", "/aayu approve all", false)); code != 0 || !accepted(d2.Number) {
		t.Fatalf("new batch after expiry: %d %s\n%s", code, out, last(d2.Number))
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
	// A record that says something other than its visible text never counts,
	// even with a matching hash: anyone with the bot's token could post one.
	forged, _, _ := parseBatch(real)
	oldHash := forged.hash()
	// Hidden: the record targets #d7 (with its hash), the comment still shows #d6.
	g.Mu.Lock()
	forged.Items[1].Issue, forged.Items[1].BodyHash = d7.Number, decisionHash(decide.Decision{Title: g.Issues[d7.Number].Title, Body: g.Issues[d7.Number].Body})
	g.Mu.Unlock()
	middle := real[strings.Index(real, "\n")+1 : strings.Index(real, batchDataMarker)]
	g.Mu.Lock()
	bc.Body = forged.marker() + "\n" + strings.ReplaceAll(middle, oldHash, forged.hash()) + batchDataMarker + base64.StdEncoding.EncodeToString(forged.data()) + " -->\n"
	g.Mu.Unlock()
	if code, _ := workflow(reply("founder", "/aayu approve all", false)); code != ExitBlocked || !strings.Contains(last(d5.Number), "the batch comment does not show what its record holds (its text)") ||
		accepted(d5.Number) || accepted(d6.Number) || accepted(d7.Number) {
		t.Fatalf("forged record: %d\n%s", code, last(d5.Number))
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

	// Each Decision is read again at its write: one closed while an earlier
	// one was written is not accepted, and the result says so.
	thread = d7.Number
	if code, _, _ := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d7.Number) + "," + strconv.Itoa(d8.Number), "--accept", "A,A"}, sets...)...); code != ExitBlocked {
		t.Fatalf("batch on #%d: %d", d7.Number, code)
	}
	// The reply approves the batch it names, not a newer one: here a second
	// batch (one item) posted as the bot after the approver read the first.
	first, _, _ := strings.Cut(last(d7.Number), "\n")
	read := strings.TrimSuffix(strings.TrimPrefix(first, batchMarker), " -->")
	newer, _, _ := parseBatch(last(d7.Number))
	newer.Posted, newer.Items, newer.Mention = time.Now().UTC().Format(time.RFC3339Nano), newer.Items[1:], []string{"founder"}
	reply("aayudha-bot[bot]", newer.render(batchStatus{}), false)
	id = reply("founder", "/aayu approve all "+read, false)
	g.Mu.Lock()
	g.Hook = func(method, path string) {
		if method == "PATCH" && path == "/repos/o/r/issues/"+strconv.Itoa(d7.Number) {
			g.Issues[d8.Number].State = "closed"
			g.Hook = nil
		}
	}
	g.Mu.Unlock()
	if code, _ := workflow(id); code != ExitBlocked || !accepted(d7.Number) || strings.Contains(g.Issues[d8.Number].Body, "accepted by") ||
		!strings.Contains(last(d7.Number), "2. #"+strconv.Itoa(d8.Number)+" was not accepted: it was closed since the batch was posted") {
		t.Fatalf("edited between writes: %d\n%s", code, last(d7.Number))
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
	if code, _ := workflow(reply("founder", "/aayu approve all", false)); code != ExitBlocked || !strings.Contains(last(d3.Number), "Item 1 was left out: it is a one-way door") || accepted(d3.Number) {
		t.Fatalf("one-way door by `all`: %d\n%s", code, last(d3.Number))
	}
	if code, _ := workflow(reply("founder", "/aayu approve 1", false)); code != ExitBlocked || !strings.Contains(last(d3.Number), "item 1 is a one-way door") || accepted(d3.Number) {
		t.Fatalf("one-way door by reply: %d\n%s", code, last(d3.Number))
	}

	// A one-way door stays on the device code (device_for = ["one-way"] by
	// default). The bot-only agent cannot ask for it: the code proves the
	// person's own account, which it does not hold.
	if code, _, errOut := run(t, agent, opts, append([]string{"decide", strconv.Itoa(d3.Number), "--accept", "A"}, sets...)...); code != ExitBlocked ||
		!strings.Contains(errOut, "the device code needs the person's own GitHub account, which this session does not hold") || accepted(d3.Number) {
		t.Fatalf("one-way door from a bot-only agent: %d %s", code, errOut)
	}
	// In the person's own marked shell (an orb's Terminal tab) the batch
	// carries the code, and a denial says so on the thread.
	person := map[string]string{"AAYU_GITHUB_TOKEN": "founder-token"}
	for k, v := range agent {
		person[k] = v
	}
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "deny"}
	g.Mu.Unlock()
	code, _, errOut = run(t, person, opts, append([]string{"decide", strconv.Itoa(d3.Number), "--accept", "A"}, sets...)...)
	if code != ExitBlocked || accepted(d3.Number) || !strings.Contains(last(d3.Number), "The request was cancelled on GitHub; nothing was done.") ||
		!strings.Contains(last(d3.Number), "item 1 is a one-way door") {
		t.Fatalf("one-way door, denied: %d %s\n%s", code, errOut, last(d3.Number))
	}
	// Approved: the code covers the batch, and the result goes back to the thread.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Mu.Unlock()
	code, out, errOut := run(t, person, opts, append([]string{"decide", strconv.Itoa(d3.Number), "--accept", "A"}, sets...)...)
	if code != 0 || !accepted(d3.Number) || !strings.Contains(out, "accepted: A, by @founder through the device code") ||
		!strings.Contains(errOut, "Approving: Accepts option 1 (A) of Decision #"+strconv.Itoa(d3.Number)) ||
		!strings.Contains(last(d3.Number), "accepted: option 1 (A), by @founder through the device code") {
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
	// gh stores the founder as an inactive account: `gh auth token` gives only
	// the active one, but `--user founder` reaches the founder's token.
	d6 := g.AddIssue("Choose the log store", decisionBody("true", ""), "type:decision", "status:ready")
	g.Mu.Lock()
	g.TokenLogins["ghu_otheractive"] = fake.Account{Login: "someone-else", Type: "User"}
	g.Mu.Unlock()
	accounts := func(context.Context) (map[string]string, error) {
		return map[string]string{"someone-else": "ghu_otheractive", "founder": "ghu_inactivefounder"}, nil
	}
	code, _, errOut = run(t, map[string]string{"AAYU_BOT_TOKEN": "bot-token", "AMP_THREAD_ID": "T-1"},
		Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond, GHAccounts: accounts, GHToken: func(context.Context) (string, error) { return "ghu_otheractive", nil }},
		"decide", strconv.Itoa(d6.Number), "--accept", "A", "--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder")
	g.Mu.Lock()
	body = g.Issues[d6.Number].Comments[len(g.Issues[d6.Number].Comments)-1].Body
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(body, "this session can reach @founder's GitHub credential (gh auth token --user founder)") || strings.Contains(body, "ghu_") || strings.Contains(body, "/aayu approve all") {
		t.Fatalf("inactive gh account: exit %d %s\n%s", code, errOut, body)
	}
	// A token in aayu's own secrets store (`aayu auth set-secret github-token`) is reachable too.
	d3 := g.AddIssue("Choose the cache", decisionBody("true", ""), "type:decision", "status:ready")
	stored := map[string]string{"AAYU_SECRET_GITHUB_TOKEN": "founder-token", "AAYU_BOT_TOKEN": "bot-token", "AMP_THREAD_ID": "T-1"}
	code, _, errOut = run(t, stored, Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond},
		"decide", strconv.Itoa(d3.Number), "--accept", "A", "--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder")
	g.Mu.Lock()
	body = g.Issues[d3.Number].Comments[len(g.Issues[d3.Number].Comments)-1].Body
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(body, "this session can reach @founder's GitHub credential (aayu secrets store") {
		t.Fatalf("stored token: exit %d %s\n%s", code, errOut, body)
	}
	// A machine-user bot cannot run the approval workflow (it mints only the
	// App's token), so its batches stay on the device code.
	d4 := g.AddIssue("Choose the CDN", decisionBody("true", ""), "type:decision", "status:ready")
	mu := map[string]string{"AAYU_GITHUB_TOKEN": "founder-token", "AAYU_SECRET_MU_TOKEN": "mu-token", "AMP_THREAD_ID": "T-1"}
	g.Mu.Lock()
	g.TokenLogins["mu-token"] = fake.Account{Login: "aayu-machine", Type: "User"}
	g.Mu.Unlock()
	code, _, errOut = run(t, mu, Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond},
		"decide", strconv.Itoa(d4.Number), "--accept", "A", "--repo", "o/r", "--set", "identity.machine_user=aayu-machine", "--set", "identity.machine_user_secret=mu-token", "--set", "approvals.approvers=founder")
	g.Mu.Lock()
	body = g.Issues[d4.Number].Comments[len(g.Issues[d4.Number].Comments)-1].Body
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(body, "the bot is the machine user @aayu-machine") {
		t.Fatalf("machine user: exit %d %s\n%s", code, errOut, body)
	}
	// An App with neither its token nor a working broker: the batch says the
	// bot is unavailable and asks for the code.
	d5 := g.AddIssue("Choose the tracer", decisionBody("true", ""), "type:decision", "status:ready")
	code, _, errOut = run(t, map[string]string{"AAYU_GITHUB_TOKEN": "founder-token", "AMP_THREAD_ID": "T-1"}, Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond},
		"decide", strconv.Itoa(d5.Number), "--accept", "A", "--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot", "--set", "approvals.approvers=founder")
	g.Mu.Lock()
	body = g.Issues[d5.Number].Comments[len(g.Issues[d5.Number].Comments)-1].Body
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(body, "Comment approval is off for this batch: the bot identity is not available") {
		t.Fatalf("bot unavailable: exit %d %s\n%s", code, errOut, body)
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
	old := g.AddIssue("Choose the queue (first try)", decisionBody("true", ""), "type:decision", "status:ready")
	for _, title := range []string{"Choose the rules engine", "Choose the queue", "Choose the data store"} {
		links := ""
		if title == "Choose the queue" { // accepting it closes #old too, and the batch must say so
			links = "supersedes #" + strconv.Itoa(old.Number)
		}
		ds = append(ds, g.AddIssue(title, decisionBody(strconv.FormatBool(title != "Choose the data store"), links), "type:decision", "status:ready"))
	}
	list := strconv.Itoa(ds[0].Number) + "," + strconv.Itoa(ds[1].Number) + "," + strconv.Itoa(ds[2].Number)
	state := t.TempDir()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "founder-token", "CURSOR_AGENT": "1", "XDG_STATE_HOME": state}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, DevicePoll: time.Millisecond}
	batch := func() string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		out := "" // the newest batch: each run posts its own
		for _, c := range g.Issues[ds[0].Number].Comments {
			if strings.HasPrefix(c.Body, batchMarker) {
				out = c.Body
			}
		}
		return out
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

	// Each Decision is read again at its write: one edited while an earlier
	// one was written is not accepted, and the batch says what was.
	e1 := g.AddIssue("Choose the log store", decisionBody("true", ""), "type:decision", "status:ready")
	e2 := g.AddIssue("Choose the tracer", decisionBody("true", ""), "type:decision", "status:ready")
	g.Mu.Lock()
	g.Permissions = map[string]string{"founder": "admin"}
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Hook = func(method, path string) {
		if method == "PATCH" && path == "/repos/o/r/issues/"+strconv.Itoa(e1.Number) {
			g.Issues[e2.Number].Body += "\nEdited meanwhile."
			g.Hook = nil
		}
	}
	g.Mu.Unlock()
	pair := strconv.Itoa(e1.Number) + ", " + strconv.Itoa(e2.Number) // as a person types it: the command line quotes it
	code, _, errOut := run(t, env, opts, "decide", pair, "--accept", "A,A", "--repo", "o/r", "--set", "approvals.approvers=founder")
	g.Mu.Lock()
	e1State, e2State := g.Issues[e1.Number].State, g.Issues[e2.Number].State
	cs := g.Issues[e1.Number].Comments
	var eb string
	for _, c := range cs {
		if strings.HasPrefix(c.Body, batchMarker) {
			eb = c.Body
		}
	}
	g.Mu.Unlock()
	want := fmt.Sprintf("Accepted #%d; #%d was not accepted: its title or body changed while the batch was applied", e1.Number, e2.Number)
	if code != ExitBlocked || e1State != "closed" || e2State != "open" || !strings.Contains(errOut, want) || !strings.Contains(eb, want) ||
		!strings.Contains(errOut, "Command: aayu decide '"+pair+"' --accept A,A") {
		t.Fatalf("edited between writes: exit %d %s\n%s", code, errOut, eb)
	}

	// The approval covers what the person saw: a Decision or the batch
	// comment edited while the code waits voids it, and nothing is written.
	g.Mu.Lock()
	g.Permissions = map[string]string{"founder": "admin"}
	g.Mu.Unlock()
	for name, edit := range map[string]func(){
		"Decision #" + strconv.Itoa(ds[1].Number) + " changed while the code was waiting": func() { g.Issues[ds[1].Number].Body += "\nEdited." },
		"the batch comment changed while the code was waiting": func() {
			cs := g.Issues[ds[0].Number].Comments
			cs[len(cs)-1].Body += "\n2. (all of it was fine)"
		},
	} {
		body1 := g.Issues[ds[1].Number].Body
		g.Mu.Lock()
		g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", Pending: 1,
			OnPoll: func(poll int) {
				if poll == 1 {
					edit()
				}
			}}
		g.Mu.Unlock()
		code, _, errOut := run(t, env, opts, "decide", list, "--accept", "A,A,B", "--repo", "o/r", "--set", "approvals.approvers=founder")
		if code != ExitBlocked || closed() != 0 || !strings.Contains(errOut, name) || !strings.Contains(batch(), "Not applied: "+name+"; nothing was done.") {
			t.Fatalf("%s: exit %d, %d closed: %s\n%s", name, code, closed(), errOut, batch())
		}
		g.Mu.Lock()
		g.Issues[ds[1].Number].Body = body1
		g.Mu.Unlock()
	}

	// While the code waits, someone edits an older comment to carry the
	// batch's marker: aayu still checks and updates its own comment.
	g.Mu.Lock()
	decoy := &fake.Comment{ID: 7000, Body: "thanks!", CreatedAt: time.Now().UTC().Add(-time.Hour)}
	decoy.User.Login, decoy.User.Type = "visitor", "User"
	g.Issues[ds[0].Number].Comments = append([]*fake.Comment{decoy}, g.Issues[ds[0].Number].Comments...)
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", Pending: 1,
		OnPoll: func(poll int) {
			cs := g.Issues[ds[0].Number].Comments
			if first, _, _ := strings.Cut(cs[len(cs)-1].Body, "\n"); poll == 1 {
				decoy.Body = first + "\nthanks!"
			}
		}}
	g.Mu.Unlock()
	g.Reset()
	var out string
	code, out, errOut = run(t, env, opts, "decide", list, "--accept", "A,A,B", "--repo", "o/r", "--set", "approvals.approvers=founder")
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
	for _, want := range []string{"**3 human-only steps wait for your approval**", "1. **Accepts option 1 (A) of Decision #" + strconv.Itoa(ds[0].Number) + ": Choose the rules engine.** Two-way door",
		"2. **Accepts option 1 (A) of Decision #" + strconv.Itoa(ds[1].Number) + ": Choose the queue. Also closes #" + strconv.Itoa(old.Number) + " as superseded.**",
		"3. **Accepts option 2 (B) of Decision #" + strconv.Itoa(ds[2].Number) + ": Choose the data store. This is not the recommendation.** One-way door",
		"Recommendation: A", "**To approve all of it, enter this code** at https://github.com/login/device before", "```text\nWDJB-MJHT\n```",
		"Comment approval is off for this batch: agents here write as @founder (no bot identity), so a reply from that account could be an agent's."} {
		if !strings.Contains(shown, want) {
			t.Fatalf("the batch with the code lacks %q:\n%s", want, shown)
		}
	}
	if b := batch(); !strings.Contains(b, "Approved by @founder through the device code") || strings.Contains(b, "WDJB-MJHT") || !strings.HasSuffix(decoy.Body, "\nthanks!") {
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
	// One per approval the person gave: the run stopped between writes, the
	// two voided runs, and the applied one.
	if len(approvals) != 4 || strings.Count(approvals[3], "Accepts option") != 3 || !strings.Contains(approvals[3], `"approver":"founder"`) {
		t.Fatalf("presence.approve records: %v", approvals)
	}
}
