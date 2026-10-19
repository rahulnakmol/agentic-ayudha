package cli

import (
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
}

func (w *inboxWork) begin(key string, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.key, w.stage, w.calls, w.started, w.wrote = key, "reading it again from GitHub", 0, now, nil
}

func (w *inboxWork) end() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	wrote := w.wrote
	w.key, w.wrote = "", nil
	return wrote
}

// snapshot is the stage, the calls so far, and when it started, for View.
func (w *inboxWork) snapshot() (key, stage string, calls int, started time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.key, w.stage, w.calls, w.started
}

func (w *inboxWork) saw(req *http.Request) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.key == "" {
		return
	}
	w.calls++
	stage, wrote := inboxStage(req.Method, req.URL.Path, req.URL.Query().Get("state"))
	w.stage = stage
	if wrote != "" && !slices.Contains(w.wrote, wrote) {
		w.wrote = append(w.wrote, wrote)
	}
}

// inboxWatch is the inbox's HTTP transport: it passes every request on and
// tells the write under way about it.
type inboxWatch struct {
	next http.RoundTripper
	work *inboxWork
}

func (t *inboxWatch) RoundTrip(req *http.Request) (*http.Response, error) {
	t.work.saw(req)
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(req)
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
