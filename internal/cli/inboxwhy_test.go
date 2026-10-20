package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gh "github.com/google/go-github/v90/github"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// ghRefusal is a write GitHub refused, as go-github returns it.
func ghRefusal(method, path string, status int, message string) error {
	u, _ := url.Parse("https://api.github.com" + path)
	return Wrap(ExitExternal, &gh.ErrorResponse{Response: &http.Response{StatusCode: status, Request: &http.Request{Method: method, URL: u}}, Message: message})
}

// flatView is the frame's words, the card pane's on their own first (at a
// wide size the card is the pane right of │), then every line's.
func flatView(m *inboxModel) string {
	m.dirty = true
	var card, all []string
	for _, l := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		all = append(all, l)
		if _, right, ok := strings.Cut(l, "│"); ok {
			card = append(card, right)
		}
	}
	return strings.Join(strings.Fields(strings.Join(card, " ")), " ") + " | " + strings.Join(strings.Fields(strings.Join(all, " ")), " ")
}

// noTech is every technical word in a person's line: a link, an HTTP method
// or status, a commit, or Go error text.
func noTech(s string) []string {
	var bad []string
	for _, re := range []*regexp.Regexp{regexp.MustCompile(`https?://|api\.github\.com`), regexp.MustCompile(`\b(GET|PUT|POST|PATCH|DELETE)\b`),
		regexp.MustCompile(`\b[1-5][0-9][0-9]\b`), reHex, regexp.MustCompile(`(?i)\b(dial tcp|EOF|nil|context deadline|x509|tls:|ErrMoved|upsert|disposition|rung)\b`)} {
		if m := re.FindString(s); m != "" {
			bad = append(bad, m)
		}
	}
	return bad
}

// inboxFailures is the inventory of Story #797: every way an inbox write can
// fail, with the words it must say. The PR's table is this list.
func inboxFailures() []struct {
	name, kind  string
	err         error
	stage       string
	wrote       []string
	what, next  string // in the first line, and the next line
	changed     string // the second line's start
	landed      bool
	optionShips bool
} {
	head := strings.Repeat("e51e073", 5) + "abcde"
	conn := &url.Error{Op: "Put", URL: "https://api.github.com/repos/o/r/pulls/661/merge", Err: errors.New("dial tcp 140.82.112.6:443: connect: connection refused")}
	reset := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	rate := &gh.RateLimitError{Rate: gh.Rate{Reset: gh.Timestamp{Time: reset}}, Response: &http.Response{StatusCode: 403, Request: &http.Request{Method: "GET", URL: &url.URL{Path: "/x"}}}, Message: "API rate limit exceeded"}
	retry := 2 * time.Minute
	abuse := &gh.AbuseRateLimitError{Response: &http.Response{StatusCode: 403, Request: &http.Request{Method: "POST", URL: &url.URL{Path: "/x"}}}, Message: "You have exceeded a secondary rate limit", RetryAfter: &retry}
	none := "Nothing was changed on GitHub."
	return []struct {
		name, kind  string
		err         error
		stage       string
		wrote       []string
		what, next  string
		changed     string
		landed      bool
		optionShips bool
	}{
		// GitHub refusing the merge (405), by its message.
		{"merge refused: conflict (405)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 405, "Pull Request is not mergeable"), "merging", nil,
			"#661 wasn't merged: it conflicts with dev.", "its builder rebases it onto dev; nothing for you yet.", none, false, true},
		{"merge refused after the bypass comment (405)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 405, "Pull Request is not mergeable"), "merging", []string{"posted a comment"},
			"#661 wasn't merged: it conflicts with dev.", "its builder rebases it onto dev", "Already on GitHub: it posted a comment.", false, true},
		{"merge refused: behind dev (405)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 405, "Head branch is out of date"), "merging", nil,
			"is behind dev, and dev only takes pull requests that are up to date", "its builder updates it from dev", none, false, true},
		{"merge refused: review required (405)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 405, "At least 1 approving review is required by reviewers with write access."), "merging", nil,
			"GitHub wants an approving review first", "you approve it on GitHub (o opens it)", none, false, true},
		{"merge refused: required check (405)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 405, `Required status check "lint" is expected.`), "merging", nil,
			"GitHub still waits for a required check", "comes back as Ready to ship", none, false, true},
		{"merge refused: other (405)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 405, "Merge commits are not allowed on this repository."), "merging", nil,
			"GitHub refused the merge", "you open it on GitHub (o) to see why", none, false, true},
		{"merge refused: head modified (409)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 409, "Head branch was modified. Review and try the merge again."), "merging", nil,
			"new commits were pushed while it was merging", "its builder runs aayu ship on the new commits", none, false, true},
		{"write conflict (409)", InboxFixed, ghRefusal("PATCH", "/repos/o/r/issues/661", 409, "Conflict"), "updating the issue", []string{"changed labels"},
			"it changed on GitHub at the same moment", "R reads it again", "Already on GitHub: it changed labels.", false, false},
		{"not valid (422)", InboxReplaced, ghRefusal("POST", "/repos/o/r/issues/661/comments", 422, "Validation Failed"), "posting a comment", nil,
			"GitHub turned the change down as not valid", "an agent looks at the details (d)", none, false, false},
		{"no permission (403)", InboxUnshipped, ghRefusal("POST", "/repos/o/r/issues/661/labels", 403, "Resource not accessible by integration"), "updating labels", nil,
			"your GitHub account isn't allowed to do this here", "a repository admin gives your account access", none, false, false},
		{"sign-in expired (401)", InboxDecision, ghRefusal("PATCH", "/repos/o/r/issues/661", 401, "Bad credentials"), "updating the issue", nil,
			"GitHub no longer accepts this session's sign-in", "you quit (q) and open aayu inbox again", none, false, false},
		{"gone (404)", InboxFixed, ghRefusal("PATCH", "/repos/o/r/issues/661", 404, "Not Found"), "updating the issue", nil,
			"GitHub can't find it any more", "R reads the inbox again", none, false, false},
		{"GitHub's side (502)", InboxReady, ghRefusal("PUT", "/repos/o/r/pulls/661/merge", 502, "Server Error"), "merging", nil,
			"GitHub had a problem on its side", "Enter tries again in a minute", none, false, true},
		{"rate limit", InboxReady, rate, "checking the required checks", nil,
			"GitHub is limiting requests from this account for now", "wait until 12:00, then Enter tries again.", none, false, true},
		{"secondary rate limit", InboxFixed, abuse, "posting a comment", nil,
			"GitHub is limiting requests from this account for now", "then Enter tries again.", none, false, false},
		{"call budget", InboxReady, fmt.Errorf("%w (500 calls)", github.ErrBudget), "reading GitHub", nil,
			"this session used up its GitHub calls", "you quit (q) and open aayu inbox again", none, false, true},
		{"network", InboxReady, Wrap(ExitExternal, conn), "merging", nil,
			"aayu couldn't reach GitHub (the network dropped or timed out)", "you check the connection", none, false, true},
		{"no answer to a write", InboxFixed, Wrap(ExitExternal, conn), "updating the issue", []string{"changed labels", "may have updated the issue"},
			"the connection to GitHub dropped in the middle", "you open it on GitHub (o) to check what landed",
			"Already on GitHub: it changed labels; GitHub didn't answer one more, so it may also have updated the issue.", false, false},
		// aayu ship's refusals, made before it writes.
		{"ship: head moved", InboxReady, headMoved(661, head, strings.Repeat("f", 40)), "reading it again from GitHub", nil,
			"new commits were pushed after you opened it", "its builder runs aayu ship on the new commits", none, false, true},
		{"ship: already merged", InboxReady, Errorf(ExitUsage, "ship #661: the PR is already merged"), "reading it again from GitHub", nil,
			"#661 is already merged", "R reads the inbox again", none, false, true},
		{"ship: closed", InboxReady, Errorf(ExitUsage, "ship #661: the PR is closed"), "reading it again from GitHub", nil,
			"#661 is closed", "R reads the inbox again", none, false, true},
		{"ship: no Story", InboxCapped, Errorf(ExitBlocked, "ship #661: no story (Refs #n in the PR body); ship proves a story"), "reading it again from GitHub", nil,
			"it names no Story", "its builder adds the Story it delivers", none, false, true},
		{"ship: gate check failed", InboxReady, Errorf(ExitBlocked, "ship #661: aayudha/inspect is failure on e51e073; the merge gate must be green"), "checking the required checks", nil,
			"its inspect check failed on its latest commit", "its builder fixes it", none, false, true},
		{"ship: gate check running", InboxReady, Errorf(ExitBlocked, "ship #661: aayudha/merge-gate is in_progress on e51e073; the merge gate must be green"), "checking the required checks", nil,
			"its merge-gate check is still running", "it comes back as Ready to ship when it is green", none, false, true},
		{"ship: required check failed", InboxReady, Errorf(ExitBlocked, "ship #661: required check lint is failure on e51e073; never merge with a red required check"), "checking the required checks", nil,
			"its lint check failed on its latest commit", "its builder fixes it", none, false, true},
		{"ship: required check missing", InboxReady, Errorf(ExitBlocked, "ship #661: required check docs is missing on e51e073; never merge with a red required check"), "checking the required checks", nil,
			"its docs check hasn't run on its latest commit", "its builder re-runs its checks", none, false, true},
		{"ship: no pack", InboxReady, Errorf(ExitBlocked, "ship #661: no merge gate pack for the head e51e073: run 1: it kept no aayudha-pack artifact"), "finding the merge gate's run", nil,
			"the merge gate's proof for its latest commit is missing or expired", "its builder re-runs the merge gate", none, false, true},
		{"ship: pack does not verify", InboxReady, Errorf(ExitBlocked, "ship #661: the merge gate's pack does not verify: no Ready snapshot on the ledger"), "downloading the merge gate's proof pack", nil,
			"its proof doesn't match what the Story asked for", "its builder fixes the proof", none, false, true},
		{"ship: a floor fails", InboxReady, Errorf(ExitBlocked, "ship #661: floors do not hold in the merge gate's pack: coverage (a floor-waiver Decision under #618 waives one, WP4.9)"), "downloading the merge gate's proof pack", nil,
			"a quality floor fails in its proof", "you accept a floor waiver Decision", none, false, true},
		{"ship: plan review needed", InboxReady, Errorf(ExitBlocked, "ship #661: the footprint touches never layer(s) L03, so the Engineering Manager's plan-time review is required: `aayu plan 618 --draft <file> --create`"), "reading reviews, files, and commits", nil,
			"needs an approved plan first", "then you approve it (aayu plan <story> --approve)", none, false, true},
		{"ship: no human pass", InboxCapped, Errorf(ExitBlocked, "ship #661: no human pass: no approving review (bot reviews and the author's own never count)"), "reading reviews, files, and commits", nil,
			"it needs an approving review on GitHub first", "you approve its latest commit on GitHub", none, false, true},
		{"ship: changed while judged", InboxReady, Errorf(ExitBlocked, "ship #661: the PR changed while ship judged it (now open, head ffffff1, into dev); run ship again"), "reading it again from GitHub", nil,
			"changed while ship was checking it, so nothing was merged", "R reads it again", none, false, true},
		{"ship: dev moved while judged", InboxReady, Errorf(ExitBlocked, "ship #661: dev moved from aaaaaaa to bbbbbbb while ship judged the PR on proof; run ship again so its current policy judges it"), "merging", nil,
			"changed while ship was checking it", "R reads it again", none, false, true},
		{"ship: not solo mode", InboxReady, soloMode(&soloCfg, "rahulnakmol", "inbox"), "reading it again from GitHub", nil,
			"solo mode doesn't apply here", "you ask @sam for an approving review", none, false, true},
		{"ship: no bypass note", InboxReady, Errorf(ExitBlocked, "ship --solo-bypass: post a PR comment starting %q that says why, from your own account, first", SoloBypassPrefix), "reading comments", nil,
			"it needs your Solo-mode bypass note on the pull request first", "Enter posts it and ships again", none, false, true},
		{"ship: the ledger", InboxReady, Errorf(ExitExternal, "ledger: fetch origin ledger: exit status 128"), "reading GitHub", nil,
			"aayu couldn't read or write its records (the ledger branch)", "an agent runs aayu doctor", none, false, true},
		// Merged, then a later step stopped: done, with a warning.
		{"merged, the Story's record failed", InboxReady, ghRefusal("PATCH", "/repos/o/r/issues/618", 422, "Validation Failed"), "updating labels", []string{"merged the pull request"},
			"#661 is merged into dev, but a step after the merge didn't finish (update the labels).", "an agent finishes the record on Story #618", "Already on GitHub: it merged the pull request.", true, true},
		{"merged, the ledger write failed", InboxReady, Wrap(ExitExternal, fmt.Errorf("merged, but the actions segment did not reach the ledger: %w", errors.New("push rejected"))), "reading GitHub", []string{"merged the pull request", "posted a comment"},
			"#661 is merged into dev, but its ship record didn't reach the ledger.", "nothing to undo", "Already on GitHub: it merged the pull request and posted a comment.", true, true},
		// decide --accept and inspect --approve refusals.
		{"decide: closed", InboxDecision, Errorf(ExitBlocked, "decide: #661 is closed"), "reading it again from GitHub", nil,
			"#661 is closed", "R reads the inbox again", none, false, false},
		{"decide: not the authority", InboxDecision, Errorf(ExitBlocked, "decide: the authority for #661 is @priya (seat product-strategist = paired); rahulnakmol cannot accept it"), "reading it again from GitHub", nil,
			"this Decision is for @priya to accept", "you ask them", none, false, false},
		{"decide: incomplete", InboxDecision, Errorf(ExitBlocked, "decide: #661 is not Decision-ready: reversible: Reversible is required at Decision-ready"), "reading it again from GitHub", nil,
			"the Decision isn't complete enough to accept", "you open it on GitHub (o) and complete it", none, false, false},
		{"decide: batch open", InboxDecision, WithNext(Errorf(ExitBlocked, "batch 1a2b3c is still open on #661; one batch at a time per issue"), "x"), "reading it again from GitHub", nil,
			"another approval request is still open on it", "you answer that one on GitHub first", none, false, false},
		{"approve: head moved", InboxHumanReview, Errorf(ExitBlocked, "inspect --approve: PR #661's head moved from e51e073 to ffffff1 since you read it, so nothing was recorded; review the new head"), "reading it again from GitHub", nil,
			"#661 wasn't approved: new commits were pushed after you opened it", "its builder runs aayu ship on the new commits", none, false, false},
		{"approve: not the approver", InboxHumanReview, Errorf(ExitBlocked, "inspect --approve: the [autonomy] approver is rahulnakmol, not sam"), "reading it again from GitHub", nil,
			"only rahulnakmol's review counts here", "you ask them to approve it", none, false, false},
		{"approve: no approver", InboxHumanReview, Errorf(ExitBlocked, "inspect --approve: no [autonomy] approver is set; name the person who approves in .aayudha/config.toml"), "reading it again from GitHub", nil,
			"no approver is named for this repository", "a person names one under [autonomy] approver", none, false, false},
		{"unknown refusal", InboxReady, Errorf(ExitBlocked, "ship #661: something new"), "merging", nil,
			"aayu stopped before writing anything it wasn't sure of", "you read the details (d)", none, false, true},
		{"unknown error", InboxFixed, errors.New("boom: nil pointer"), "posting a comment", nil,
			"something went wrong inside aayu", "an agent looks at the details (d)", none, false, false},
	}
}

// soloCfg is a pod with a second paired human, where solo mode does not hold.
var soloCfg = config.Config{Seats: map[string]config.Seat{"quality-architect": {Mode: "paired", Human: "sam"}}}

// TestInboxFailureWords checks every row of the inventory: what happened,
// whether anything changed, and who acts next, with none of the technical
// words, and the detail behind d holding GitHub's own.
func TestInboxFailureWords(t *testing.T) {
	head := strings.Repeat("e51e073", 5) + "abcde"
	for _, f := range inboxFailures() {
		it := InboxItem{Kind: f.kind, Number: 661, Story: 618, f: inboxFacts{head: head, devBr: "dev"}}
		a := InboxAction{N: 1, Command: "inbox: close as fixed"}
		if f.optionShips {
			a.Command = "aayu ship 661"
		}
		w := explainFailure(it, a, f.err, f.stage, f.wrote)
		if !strings.HasPrefix(w.What, "#661 ") || !strings.Contains(w.What, f.what) {
			t.Errorf("%s: what %q, want %q", f.name, w.What, f.what)
		}
		if !strings.HasPrefix(w.Changed, f.changed) {
			t.Errorf("%s: changed %q, want %q", f.name, w.Changed, f.changed)
		}
		if !strings.Contains(w.Next, f.next) {
			t.Errorf("%s: next %q, want %q", f.name, w.Next, f.next)
		}
		if w.Landed != f.landed {
			t.Errorf("%s: landed %v", f.name, w.Landed)
		}
		if w.Retry != strings.Contains(w.Next, "Enter") || strings.Contains(w.Next, "enter ") {
			t.Errorf("%s: retry %v, but the next line says %q", f.name, w.Retry, w.Next)
		}
		if bad := noTech(strings.Join(w.lines(), " ")); len(bad) > 0 {
			t.Errorf("%s: technical words %q in %q", f.name, bad, w.lines())
		}
		if detail := strings.Join(w.Detail, "\n"); !strings.Contains(detail, f.err.Error()) || !strings.Contains(detail, "Stopped at: ") {
			t.Errorf("%s: the details lack the error or the step: %q", f.name, detail)
		}
	}
}

// The full-screen inbox's failed cards (criteria 1, 3, 4, 5): a refused
// merge, a merge whose later step failed, and an item that changed before
// its write, at 120x34 and 72x34. The card says it in three lines, the
// status line stays short, and d shows GitHub's own words.
func TestInboxFailureCards(t *testing.T) {
	clock := time.Date(2026, 10, 19, 11, 29, 0, 0, time.UTC)
	head := "e51e073" + strings.Repeat("0", 33)
	ready := func(n int) InboxItem {
		return InboxItem{Kind: InboxReady, Number: n, Title: "feat(inbox): a long title that wraps on a narrow terminal and is never cut off", URL: fmt.Sprintf("https://github.com/o/r/pull/%d", n),
			Seat: "engineering-manager", PR: n, Story: n - 43, Why: "green", f: inboxFacts{head: head, devBr: "dev", story: n - 43},
			Options: []InboxAction{{N: 1, Recommended: true, Label: fmt.Sprintf("ship PR #%d", n), Command: fmt.Sprintf("aayu ship %d", n)}, {N: 2, Label: "open it and review it", Command: "open"}}}
	}
	moved := ready(681)
	moved.Kind, moved.Options, moved.Why, moved.Next = InboxNotReady, nil, "can't merge: it conflicts with dev, which took #725 since ship checked it.", "its builder rebases it onto dev; nothing for you yet."
	items := []InboxItem{ready(661), ready(681), ready(707)}
	for _, size := range [][2]int{{120, 34}, {72, 34}} {
		m := newInboxModel("o/r", "rahulnakmol", "", true)
		m.now = func() time.Time { return clock }
		m.gather = func() inboxGatheredMsg { return inboxGatheredMsg{items: m.items, at: clock} }
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.Update(inboxGatheredMsg{items: items, at: clock})
		call := func(w *inboxWork, method, path string, code int) {
			req, _ := http.NewRequestWithContext(contextWith(w), method, "https://api.github.com"+path, nil)
			_, _ = (&inboxWatch{next: roundTrip(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: code, Body: http.NoBody, Request: r}, nil
			})}).RoundTrip(req) //nolint:bodyclose // no body
		}
		m.act = func(it InboxItem, a InboxAction, w *inboxWork) (InboxItem, string, error) {
			switch it.Number {
			case 661: // the bypass comment, then GitHub refuses the merge
				call(w, http.MethodPost, "/repos/o/r/issues/661/comments", 201)
				call(w, http.MethodPut, "/repos/o/r/pulls/661/merge", 405)
				return it, "", ghRefusal(http.MethodPut, "/repos/o/r/pulls/661/merge", 405, "Pull Request is not mergeable")
			case 681:
				return it, "", errInboxChanged{Item: moved}
			default: // merged, then the Story's labels fail
				call(w, http.MethodPut, "/repos/o/r/pulls/707/merge", 200)
				call(w, http.MethodPost, "/repos/o/r/issues/664/labels", 422)
				return it, "", ghRefusal(http.MethodPost, "/repos/o/r/issues/664/labels", 422, "Validation Failed")
			}
		}
		at := func(n int) {
			m.cur = slicesIndex(m, n)
		}
		check := func(name string, want []string, keep func(string) string) {
			t.Helper()
			v := flatView(m)
			for _, w := range want {
				if !strings.Contains(v, w) {
					t.Errorf("%dx%d %s: no %q:\n%s", size[0], size[1], name, w, ansi.Strip(m.View().Content))
				}
			}
			for i, l := range strings.Split(ansi.Strip(m.View().Content), "\n") {
				if ansi.StringWidth(l) != size[0] {
					t.Errorf("%dx%d %s: line %d is %d cells", size[0], size[1], name, i, ansi.StringWidth(l))
				}
			}
			if bad := noTech(keep(v)); len(bad) > 0 {
				t.Errorf("%dx%d %s: technical words %q on the card:\n%s", size[0], size[1], name, bad, v)
			}
		}
		card := func(v string) string { c, _, _ := strings.Cut(v, " | "); return c }
		if size[0] < inboxWide {
			card = func(v string) string { _, all, _ := strings.Cut(v, " | "); return all }
		}
		at(661)
		teaDrive(m, m.enter())
		at(661)
		check("refused merge", []string{"✗ #661 wasn't merged: it conflicts with dev.", "Already on GitHub: it posted a comment.", "Next: its builder rebases it onto dev; nothing for you yet."}, card)
		// Enter again cannot fix a conflict: the card stops offering ship.
		if slices.ContainsFunc(m.choices(), func(c InboxAction) bool { return strings.HasPrefix(c.Command, "aayu ship") }) || !strings.Contains(flatView(m), "not done: see the card") {
			t.Errorf("%dx%d: the refused card still offers ship: %+v", size[0], size[1], m.choices())
		}
		if m.status != "✗ #661 not shipped; the card says why." || !m.readAfter && m.refreshing == false && len(m.queue) > 0 {
			t.Errorf("%dx%d: status %q", size[0], size[1], m.status)
		}
		teaKeys(m, "d")
		check("refused merge, details", []string{"Details (d hides them)", "· Stopped at: Merge into dev", "· GitHub answered 405 to PUT /repos/o/r/pulls/661/merge", "Pull Request is not mergeable", "· Head commit: e51e073"},
			func(string) string { return "" })
		teaKeys(m, "d")
		if strings.Contains(flatView(m), "GitHub answered 405") {
			t.Errorf("%dx%d: d again kept the details", size[0], size[1])
		}

		at(707)
		teaDrive(m, m.enter())
		at(707)
		check("merged, then a step failed", []string{"✓ Merged #707 into dev", "#707 is merged into dev, but a step after the merge didn't finish (update the labels).",
			"Next: nothing to undo; an agent finishes the record on Story #664 from the details (d)."}, func(v string) string {
			_, warning, _ := strings.Cut(v, "by @rahulnakmol at 11:29")
			warning, _, _ = strings.Cut(warning, "What changed on GitHub") // the done view's own link stays
			return warning
		})
		if st := m.state[ready(707).key()]; st != "done" {
			t.Errorf("%dx%d: a merge that landed is %q, not done", size[0], size[1], st)
		}

		at(681)
		teaDrive(m, m.enter())
		at(681)
		check("changed since read", []string{"#681 changed on GitHub since you opened it: it can't merge: it conflicts with dev, which took #725 since ship checked it. Nothing was written; the card shows it as it is now.",
			"Next: its builder rebases it onto dev; nothing for you yet.", "Open #681 on GitHub"}, card)
		if strings.Contains(flatView(m), "Ship PR #681") || !strings.HasPrefix(m.status, "#681 changed on GitHub; nothing was written.") {
			t.Errorf("%dx%d: the changed card still ships, or status %q", size[0], size[1], m.status)
		}
		// A refresh GitHub fails keeps the cards and says so plainly.
		m.refreshing = true
		m.Update(inboxGatheredMsg{err: ghRefusal(http.MethodGet, "/repos/o/r/issues", 502, "Server Error")})
		if m.status != "Not refreshed: GitHub had a problem. R tries again." || len(m.items) != 3 {
			t.Errorf("%dx%d: a failed refresh: status %q, %d cards", size[0], size[1], m.status, len(m.items))
		}
	}
}

func slicesIndex(m *inboxModel, n int) int {
	for r, i := range m.rows {
		if m.items[i].Number == n {
			return r
		}
	}
	return -1
}

func contextWith(w *inboxWork) context.Context {
	return context.WithValue(context.Background(), inboxWorkKey{}, w)
}

// The sign-in's failures (presence.go's refusals), in the inventory too:
// what happened, that nothing was done, and what to do, in plain words.
func TestInboxSigninWords(t *testing.T) {
	for _, c := range []struct{ err, want string }{
		{"inbox on o/r was not approved: the code expired; nothing was done", "The code expired before it was approved. Nothing was done."},
		{"inbox on o/r was not approved: the request was denied; nothing was done", "The request was cancelled on GitHub. Nothing was done."},
		{"inbox: approved by helper[bot], a bot identity; only a person may approve it", "A bot account approved it, and only a person can. Nothing was done; approve it signed in to GitHub as yourself."},
		{"inbox: approved by @sam, but aayu acts as @rahulnakmol here; the person whose account acts must approve it", "@sam approved it, but this terminal acts as @rahulnakmol. Nothing was done; approve it signed in to GitHub as @rahulnakmol."},
		{"inbox on o/r needs write access; @sam has read", "@sam has read access here, and the inbox needs write. Nothing was done; a repository admin can give @sam write access."},
		{"inbox: recording the approval in the actions log: disk full; nothing was done", "aayu couldn't record the approval on this machine (its actions log). Nothing was done."},
		{"Get \"https://api.github.com/user\": dial tcp: i/o timeout", "GitHub gave no approval: "},
		{"inbox was not approved; nothing was done", "You left the sign-in before it was approved. Nothing was done."},
		{"no GitHub token: run gh auth login or set AAYU_GITHUB_TOKEN", "aayu has no GitHub sign-in on this machine yet. Nothing was done."},
	} {
		got := signinWords(errors.New(c.err))
		if !strings.HasPrefix(got, c.want) || len(noTech(got)) > 0 {
			t.Errorf("%q: %q, want %q", c.err, got, c.want)
		}
	}
}

// A choice that failed for good leaves the card (CodeRabbit's r4328514894
// on #851): when the person had another choice selected and cleared the
// housekeeping, the selection moves to a choice that is left, so the card
// always shows which one Enter does.
func TestInboxFailedChoiceLeaves(t *testing.T) {
	it := InboxItem{Kind: InboxFixed, Number: 661, Title: "fix: x", Why: "fixed",
		Options: []InboxAction{{N: 1, Recommended: true, Label: "close it as fixed", Command: "inbox: close as fixed"}, {N: 2, Label: "open it", Command: "open"}}}
	m := newInboxModel("o/r", "rahulnakmol", "", true)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m.Update(inboxGatheredMsg{items: []InboxItem{it}})
	m.act = func(it InboxItem, _ InboxAction, _ *inboxWork) (InboxItem, string, error) {
		return it, "", ghRefusal(http.MethodPatch, "/repos/o/r/issues/661", 403, "Resource not accessible by integration")
	}
	m.cur = slicesIndex(m, 661)
	m.sel = 1
	m.askTidy()
	teaDrive(m, m.update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if cs := m.choices(); len(cs) != 1 || m.sel != 0 || m.state[it.key()] != "not done" {
		t.Errorf("after the recommendation failed for good: state %q, choices %+v, selected %d", m.state[it.key()], cs, m.sel)
	}
}

// The ways aayu inbox ends with no card say so in plain words (inspect's
// d06e873051 and f998c626f4 on #845): a sign-in that cannot reach GitHub,
// a sign-in refused before any code, and a first read that cannot reach
// GitHub. GitHub's own words go to the log line above.
func TestInboxNoCardExits(t *testing.T) {
	plain := func(name, errOut, want string) {
		t.Helper()
		line, _, _ := strings.Cut(errOut, "\n")
		if !strings.Contains(line, want) || len(noTech(line)) > 0 || strings.Contains(line, "dial tcp") {
			t.Errorf("%s: the line says %q, want %q and no technical words", name, line, want)
		}
	}
	dial := func(path string) error {
		return &url.Error{Op: "Get", URL: "https://api.github.com" + path, Err: errors.New("dial tcp 140.82.112.6:443: connect: connection refused")}
	}
	env := func() map[string]string {
		return map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": t.TempDir()}
	}
	g := inboxRepo(t)
	via := func(fail func(*http.Request) error) *http.Client {
		return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if err := fail(r); err != nil {
				return nil, err
			}
			return http.DefaultTransport.RoundTrip(r)
		})}
	}
	quit := func(tm tea.Model) error {
		m := tm.(*inboxModel)
		teaDrive(m, m.Init())
		teaKeys(m, "q")
		return nil
	}

	// A terminal signs in before the inbox opens; GitHub cannot be reached.
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, IsTerminal: func(any) bool { return true }, Assets: aayudha.Assets, TUI: quit,
		HTTP: via(func(r *http.Request) error { return dial(r.URL.Path) })}
	code, out, errOut := runIn(t, env(), opts, "", "inbox", "--repo", "o/r")
	plain("a sign-in that cannot reach GitHub", errOut, "GitHub gave no approval: GitHub couldn't be reached. Nothing was done.")
	if code != ExitExternal || !strings.Contains(out, "sign-in failed: ") || !strings.Contains(out, "dial tcp") {
		t.Errorf("a sign-in that cannot reach GitHub: exit %d, log %q", code, out)
	}

	// Signed in, then the first read of the issues cannot reach GitHub.
	opts.HTTP = via(func(r *http.Request) error {
		if strings.HasSuffix(r.URL.Path, "/issues") {
			return dial(r.URL.Path)
		}
		return nil
	})
	code, out, errOut = runIn(t, env(), opts, "", "inbox", "--repo", "o/r")
	plain("a first read that cannot reach GitHub", errOut, "aayu inbox couldn't read GitHub (GitHub couldn't be reached), so nothing was shown or changed")
	if code != ExitExternal || !strings.Contains(out, "read failed: ") || !strings.Contains(out, "dial tcp") {
		t.Errorf("a first read that cannot reach GitHub: exit %d, log %q", code, out)
	}

	// The sign-in screen (an agent marker) is refused before any code.
	opts.HTTP = &http.Client{}
	opts.TUI = func(tm tea.Model) error {
		m := tm.(*inboxModel)
		m.signin = &inboxSignin{run: func() tea.Msg { return nil }}
		m.Update(inboxSignedMsg{err: Wrap(ExitExternal, dial("/user"))})
		return nil
	}
	code, out, errOut = runIn(t, env(), opts, "", "inbox", "--repo", "o/r")
	plain("a sign-in refused before any code", errOut, "GitHub gave no approval: GitHub couldn't be reached. Nothing was done.")
	if code != ExitExternal || !strings.Contains(out, "sign-in failed: ") {
		t.Errorf("a sign-in refused before any code: exit %d, log %q", code, out)
	}
}
