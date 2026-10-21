package cli

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// What a write from the inbox is doing while it runs, and what it wrote once
// it is done. A write (ship, decide, a tidy-up) can take minutes, so the
// inbox names its stage from the GitHub request it is making, and keeps the
// screen live meanwhile. Nothing here decides anything: it only watches.

// inboxWork is the write under way: its item, its stage, and what it wrote.
// The write's goroutine updates it; View reads it under the lock.
type inboxWork struct {
	mu      sync.Mutex
	key     string
	stage   string
	calls   int
	started time.Time
	wrote   []string
	stages  []string // every stage seen, in order; the last one seen is the current one
	cur     int
}

func (w *inboxWork) begin(key string, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.key, w.stage, w.calls, w.started, w.wrote = key, "reading it again from GitHub", 0, now, nil
	w.stages, w.cur = []string{w.stage}, 0
}

func (w *inboxWork) end() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	wrote := w.wrote
	w.key, w.wrote = "", nil
	return wrote
}

// sofar is the stage and what was written so far, leaving the work as it is.
func (w *inboxWork) sofar() (stage string, wrote []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stage, slices.Clone(w.wrote)
}

// steps are the stages seen so far and the index of the current one.
func (w *inboxWork) steps() ([]string, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.stages), w.cur
}

// snapshot is the stage, the calls so far, and when it started, for View.
func (w *inboxWork) snapshot() (key, stage string, calls int, started time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.key, w.stage, w.calls, w.started
}

// saw names the request's stage before it is sent.
func (w *inboxWork) saw(req *http.Request) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.key == "" {
		return
	}
	w.calls++
	stage, _ := inboxStage(req.Method, req.URL.Path, req.URL.Query().Get("state"))
	if stage == "reading GitHub" || stage == "reading comments" {
		return // a plain read is part of the step under way, not a step of its own
	}
	w.stage = stage
	if w.cur = slices.Index(w.stages, stage); w.cur < 0 {
		w.stages, w.cur = append(w.stages, stage), len(w.stages)
	}
}

// answered records what a write wrote once GitHub has answered it: only a
// success counts as written, and a request that got no answer may or may
// not have landed, which the record says.
func (w *inboxWork) answered(req *http.Request, resp *http.Response, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, wrote := inboxStage(req.Method, req.URL.Path, req.URL.Query().Get("state"))
	switch {
	case w.key == "" || wrote == "":
		return
	case err != nil:
		wrote = "may have " + wrote
	case resp.StatusCode >= 300:
		return
	}
	if !slices.Contains(w.wrote, wrote) {
		w.wrote = append(w.wrote, wrote)
	}
}

// inboxWorkKey carries a write's work in the context of its requests, so
// that writes running at once each hear only their own.
type inboxWorkKey struct{}

// inboxWatch is the inbox's HTTP transport: it passes every request on and
// tells the write it belongs to, if any, about it.
type inboxWatch struct {
	next http.RoundTripper
}

func (t *inboxWatch) RoundTrip(req *http.Request) (*http.Response, error) {
	w, _ := req.Context().Value(inboxWorkKey{}).(*inboxWork)
	if w != nil {
		w.saw(req)
	}
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	resp, err := next.RoundTrip(req)
	if w != nil {
		w.answered(req, resp, err)
	}
	return resp, err
}

var (
	reStagePR      = regexp.MustCompile(`/pulls/[0-9]+$`)
	reStageMerge   = regexp.MustCompile(`/pulls/[0-9]+/merge$`)
	reStageReviews = regexp.MustCompile(`/pulls/[0-9]+/(reviews|files|commits)$`)
	reStageIssue   = regexp.MustCompile(`/issues/[0-9]+$`)
)

// inboxStage names a GitHub request in a person's words: the stage it is
// part of, and for a write what it wrote.
func inboxStage(method, path, state string) (stage, wrote string) {
	read := method == http.MethodGet || method == http.MethodHead
	switch {
	case strings.HasSuffix(path, "/graphql"):
		return "reading GitHub", ""
	case read && strings.HasSuffix(path, "/pulls") && state == "closed":
		return "reading the merged pull requests (the autonomy streak)", ""
	case strings.Contains(path, "/actions/jobs/") && strings.HasSuffix(path, "/logs"):
		return "reading the merge gate's logs", ""
	case strings.Contains(path, "/artifacts"):
		return "downloading the merge gate's proof pack", ""
	case strings.Contains(path, "/actions/"):
		return "finding the merge gate's run", ""
	case strings.Contains(path, "/check-runs") || strings.Contains(path, "/status") || strings.Contains(path, "/rules/") || strings.Contains(path, "/protection"):
		return "checking the required checks", ""
	case read && reStageReviews.MatchString(path):
		return "reading reviews, files, and commits", ""
	case method == http.MethodPut && reStageMerge.MatchString(path):
		return "merging", "merged the pull request"
	case strings.Contains(path, "/comments") && !read:
		return "posting a comment", "posted a comment"
	case strings.Contains(path, "/labels") && !read:
		return "updating labels", "changed labels"
	case strings.Contains(path, "/git/refs") && method == http.MethodDelete:
		return "deleting the branch", "deleted the branch"
	case !read && reStageIssue.MatchString(path):
		return "updating the issue", "updated the issue"
	case !read && reStagePR.MatchString(path):
		return "updating the pull request", "updated the pull request"
	case !read:
		return "writing to GitHub", "wrote to GitHub"
	case strings.Contains(path, "/comments"):
		return "reading comments", ""
	}
	return "reading GitHub", ""
}

// inboxDone is one write the person made from the inbox: who, when, what
// they chose, and what it wrote. The inbox cache keeps it, so a later
// session shows the item as done instead of offering it again.
type inboxDone struct {
	By     string    `json:"by"`
	At     time.Time `json:"at"`
	Choice string    `json:"choice"`
	Wrote  []string  `json:"wrote,omitempty"`
	// Warning is a step after the write's main change that did not finish
	// (a merge that landed before the Story's record failed), in plain words.
	Warning []string `json:"warning,omitempty"`
	// Detail is GitHub's own facts behind Warning (the step, its answer, the
	// commit), shown under it when the person presses d.
	Detail []string `json:"detail,omitempty"`
}

// when is a done write's time in the person's words: the clock time today,
// else the date as well.
func (d inboxDone) when(now time.Time) string {
	at := d.At.In(now.Location())
	if y, m, dd := at.Date(); y == now.Year() && m == now.Month() && dd == now.Day() {
		return at.Format("15:04")
	}
	return at.Format("Jan 2 15:04")
}

// line is the done write in one line: "done by @o at 23:43: ship PR #693".
func (d inboxDone) line(now time.Time) string {
	return "done by @" + d.By + " at " + d.when(now) + ": " + d.Choice
}

// inboxStep is a stage in a person's words: the checklist's line for it, and
// what is happening now while it runs.
type inboxStep struct{ line, now string }

var inboxSteps = map[string]inboxStep{
	"reading it again from GitHub":                           {"Read it again from GitHub", "Checking nothing changed on GitHub since you read it"},
	"finding the merge gate's run":                           {"Find the merge gate's run", "Finding the merge gate's latest run on this head"},
	"reading the merge gate's logs":                          {"Read the merge gate's verdict", "Reading what the merge gate decided"},
	"downloading the merge gate's proof pack":                {"Fetch the proof pack", "Fetching the proof pack the merge gate built"},
	"checking the required checks":                           {"Check the required checks", "Checking the merge gate is still green"},
	"reading reviews, files, and commits":                    {"Read the reviews and commits", "Reading the reviews and commits on the head"},
	"reading the merged pull requests (the autonomy streak)": {"Count the autonomy streak", "Counting recent merges for the autonomy streak; this is the slow step"},
	"merging":                   {"Merge into dev", "Merging the pull request into dev"},
	"posting a comment":         {"Post the record on GitHub", "Posting the record of what you did on GitHub"},
	"updating labels":           {"Update the labels", "Updating the labels"},
	"deleting the branch":       {"Delete the merged branch", "Deleting the merged branch"},
	"updating the issue":        {"Close the issue", "Closing the issue"},
	"updating the pull request": {"Update the pull request", "Updating the pull request"},
	"reading comments":          {"Read the comments", "Reading the comments"},
	"reading GitHub":            {"Read GitHub", "Reading GitHub"},
	"writing to GitHub":         {"Write to GitHub", "Writing to GitHub"},
}

func inboxStepOf(stage string) inboxStep {
	if s, ok := inboxSteps[stage]; ok {
		return s
	}
	return inboxStep{upperFirst(stage), upperFirst(stage)}
}

// inboxDoing is what a write on an item does, in a person's words: its
// headline while it runs and once done, the steps it usually takes (the
// stages seen come first, in the order they happen), how long it usually
// takes, and what happens after it.
type inboxDoing struct {
	doing, did, usually, next string
	plan                      []string
}

func inboxDoingFor(it InboxItem, a InboxAction) inboxDoing {
	ship := []string{"reading it again from GitHub", "finding the merge gate's run", "reading the merge gate's logs", "checking the required checks",
		"reading reviews, files, and commits", "merging", "posting a comment", "updating labels", "deleting the branch"}
	n := it.Number
	switch {
	case it.Kind == InboxReady || it.Kind == InboxCapped && strings.HasPrefix(a.Command, "aayu ship"):
		return inboxDoing{fmt.Sprintf("Shipping #%d into dev", n), fmt.Sprintf("Shipped #%d into dev", n), "usually under a minute; up to about 3 minutes when it counts the autonomy streak",
			"It is in dev now, with its ship record and proof; the next rolling snapshot carries it.", ship}
	case it.Kind == InboxDecision:
		return inboxDoing{fmt.Sprintf("Recording your answer on Decision #%d", n), fmt.Sprintf("Recorded your answer on Decision #%d", n), "usually a few seconds",
			"The Decision is closed with your answer; the work that waited on it can go on.", []string{"reading it again from GitHub", "posting a comment", "updating labels", "updating the issue"}}
	case it.Kind == InboxHumanReview:
		return inboxDoing{fmt.Sprintf("Approving #%d's head as your review", n), fmt.Sprintf("Approved #%d's head as your review", n), "usually a few seconds",
			"The merge gate reads your approval on its next run and can merge it.", []string{"reading it again from GitHub", "posting a comment"}}
	case it.Kind == InboxUnshipped:
		return inboxDoing{fmt.Sprintf("Marking #%d merged without proof", n), fmt.Sprintf("Marked #%d merged without proof", n), "usually a few seconds",
			"It stays uncovered in aayu audit; nothing in the code changes.", []string{"reading it again from GitHub", "posting a comment", "updating labels"}}
	case it.Kind == InboxFixed:
		return inboxDoing{fmt.Sprintf("Closing #%d as fixed", n), fmt.Sprintf("Closed #%d as fixed", n), "usually a few seconds",
			"The Bug is closed; reopen it on GitHub if that was wrong.", []string{"reading it again from GitHub", "posting a comment", "updating the issue"}}
	case it.Kind == InboxAudit && a.N == 2:
		return inboxDoing{fmt.Sprintf("Recording a correction on #%d", n), fmt.Sprintf("Recorded a correction on #%d", n), "usually under a minute",
			"The correction label is on it, a PR that reverts it is open, and the sample is on the ledger.",
			[]string{"reading it again from GitHub", "updating labels"}}
	case it.Kind == InboxAudit:
		return inboxDoing{fmt.Sprintf("Recording #%d as looks right", n), fmt.Sprintf("Recorded #%d as looks right", n), "usually a few seconds",
			"A clean sample is on the ledger, and the look is done.", []string{"reading it again from GitHub", "updating labels"}}
	case it.Kind == InboxReplaced:
		return inboxDoing{fmt.Sprintf("Closing #%d as replaced", n), fmt.Sprintf("Closed #%d as replaced", n), "usually a few seconds",
			"It is closed as not planned; its replacement carries the work.", []string{"reading it again from GitHub", "posting a comment", "updating the issue"}}
	}
	return inboxDoing{fmt.Sprintf("Doing #%d: %s", n, it.button(a)), fmt.Sprintf("Did #%d: %s", n, it.button(a)), "usually a few seconds", "", []string{"reading it again from GitHub"}}
}

// inboxFail is a write that stopped: the step it stopped at, why, and what
// GitHub had already answered as written.
type inboxFail struct {
	stage, err string
	wrote      []string
	a          InboxAction
	why        inboxWhy // the same, in a person's words
}
