package audit

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// writeKinds are the timeline events that record a write by their actor.
// Mirrors and consequences are left out: parent_issue_added mirrors the
// parent's sub_issue_added, head_ref_deleted follows a merge when the
// repository deletes head branches, and closed accompanies merged.
var writeKinds = []string{"commented", "labeled", "unlabeled", "closed", "reopened", "renamed", "merged", "reviewed",
	"sub_issue_added", "assigned", "unassigned", "milestoned", "demilestoned", "locked", "unlocked", "review_requested",
	"review_request_removed", "ready_for_review", "convert_to_draft", "pinned", "unpinned", "transferred",
	"issue_type_added", "issue_type_changed", "issue_type_removed"}

// Tolerance is how far an actions record's time may be from GitHub's event time.
const Tolerance = 10 * time.Minute

// writes lists the GitHub writes in the window from the event history and
// reconciles each against the actions segments: a workflow or App write is its
// run's record; any other write needs a record of its own, else it is unlogged.
// Writes are matched oldest first and each record accounts for one write (one
// per label for a labels request), so a logged write cannot cover a direct one
// next to it.
func (d *Data) writes(in Inputs) []Write {
	type cand struct {
		w        Write
		kind     string
		n        int
		label    string
		id       int64
		isLedger bool
		sha      string
	}
	var cs []cand
	for _, it := range d.Items {
		target := fmt.Sprintf("#%d", it.Number)
		what := map[bool]string{true: "opened PR", false: "opened issue"}[it.PR]
		cs = append(cs, cand{w: Write{Target: target, Kind: what, Actor: it.Author, At: it.CreatedAt}, kind: "opened", n: it.Number})
		for _, e := range it.Events {
			if !slices.Contains(writeKinds, e.Kind) || (e.Kind == "closed" && it.PR && mergedNear(it.Events, e.At)) {
				continue
			}
			kind := e.Kind
			if e.Label != "" {
				kind += " " + e.Label
			}
			cs = append(cs, cand{w: Write{Target: target, Kind: kind, Actor: e.Actor, At: e.At}, kind: e.Kind, n: it.Number, label: e.Label, id: e.ID})
		}
	}
	for _, r := range d.Runs {
		if r.Event == "workflow_dispatch" {
			cs = append(cs, cand{w: Write{Target: fmt.Sprintf("run %d (%s)", r.ID, r.Workflow), Kind: "workflow dispatch", Actor: r.Actor, At: r.CreatedAt},
				kind: "workflow_dispatch", label: path.Base(r.Workflow)})
		}
	}
	for _, e := range d.LedgerLog {
		cs = append(cs, cand{w: Write{Target: "ledger " + short(e.SHA), Kind: "ledger push: " + e.Subject, Actor: e.Author, At: e.At}, isLedger: true, sha: e.SHA})
	}
	cs = slices.DeleteFunc(cs, func(c cand) bool { return !d.in(in, c.w.At) })
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i].w, cs[j].w
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Kind < b.Kind
	})
	used := map[string]bool{}
	out := make([]Write, 0, len(cs))
	for _, c := range cs {
		w := c.w
		switch {
		case c.isLedger:
			if strings.HasPrefix(strings.TrimPrefix(w.Kind, "ledger push: "), "actions: segment ") {
				w.Logged = "(the ledger commit itself)"
			} else {
				w.Logged = d.segmentOfLedgerPush(c.sha)
			}
		case strings.HasSuffix(w.Actor, "[bot]") && !slices.ContainsFunc(d.Bots, func(b string) bool { return strings.EqualFold(b, w.Actor) }):
			w.CI = true
		default:
			w.Logged = d.logged(in, c.kind, c.n, c.label, c.id, w.At, used)
		}
		out = append(out, w)
	}
	return out
}

func mergedNear(es []Event, at time.Time) bool {
	return slices.ContainsFunc(es, func(e Event) bool { return e.Kind == "merged" && absDur(e.At.Sub(at)) <= 10*time.Second })
}

func (d *Data) segmentOfLedgerPush(sha string) string {
	for _, r := range d.Records {
		if r.Kind == "ledger.push" && resultField(r.Result, "commit") == sha {
			return r.Segment
		}
	}
	return ""
}

// logged returns the segment holding the record of a write, or "", and marks
// the record used. A comment record whose result is the comment's ID matches
// only that comment.
func (d *Data) logged(in Inputs, kind string, n int, label string, id int64, at time.Time, used map[string]bool) string {
	pre := "/repos/" + in.Repo
	issue := fmt.Sprintf("%s/issues/%d", pre, n)
	pull := fmt.Sprintf("%s/pulls/%d", pre, n)
	for i, r := range d.Records {
		key := fmt.Sprintf("%d/%s", i, label)
		if used[key] || r.Status != actions.StatusDone || r.Request == nil || absDur(r.Time.Sub(at)) > Tolerance {
			continue
		}
		m, p := r.Request.Method, r.Request.Path
		if u, err := url.PathUnescape(p); err == nil {
			p = u
		}
		var ok bool
		switch kind {
		case "opened":
			ok = m == http.MethodPost && (p == pre+"/issues" || p == pre+"/pulls") && resultField(r.Result, "number") == fmt.Sprint(n)
		case "commented":
			rid := resultField(r.Result, "id")
			ok = m == http.MethodPost && p == issue+"/comments" && (rid == "" || id == 0 || rid == fmt.Sprint(id))
		case "labeled":
			ok = m == http.MethodPost && p == issue+"/labels" && payloadHas(r.Payload, "labels", label)
		case "unlabeled":
			ok = m == http.MethodDelete && p == issue+"/labels/"+label
		case "closed", "reopened", "renamed":
			ok = m == http.MethodPatch && (p == issue || p == pull)
		case "merged":
			ok = m == http.MethodPut && p == pull+"/merge"
		case "reviewed":
			ok = m == http.MethodPost && p == pull+"/reviews"
		case "sub_issue_added":
			ok = m == http.MethodPost && p == issue+"/sub_issues"
		case "workflow_dispatch":
			ok = m == http.MethodPost && p == pre+"/actions/workflows/"+label+"/dispatches" // label: the workflow file
		default:
			ok = m != http.MethodGet && r.Target == fmt.Sprintf("%s#%d", in.Repo, n)
		}
		if ok {
			used[key] = true
			return r.Segment
		}
	}
	return ""
}

// payloadHas reports whether payload[key] lists v; a record without the key
// matches any value.
func payloadHas(payload map[string]any, key, v string) bool {
	raw, ok := payload[key]
	if !ok {
		return true
	}
	switch l := raw.(type) {
	case []any:
		return slices.ContainsFunc(l, func(x any) bool { return x == v })
	case []string:
		return slices.Contains(l, v)
	}
	return false
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
