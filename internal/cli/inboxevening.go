package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/audit"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// The evening review (Story #1176, Decision #1168). At the end of [time]
// business_hours, `aayu inbox` shows one card for each story agreed this
// morning: each story of a mission milestone that started today, by its
// mission's start or its previous milestone's gate. The card shows each
// criterion of the story, its evidence kind, and its evidence from the proof
// pack on the ledger at the head that merged, never from a comment. A person
// accepts it, which adds a line to missions/<f>/reviews.jsonl, or says what is
// wrong in a comment on the story, the input of the mission's feedback pass
// (#1014). A story that did not merge says so, with the mission's reason and
// the next step. An agent lists the cards and never accepts one.

// MissionReviewsPath is where a mission's evening reviews live on the ledger.
func MissionReviewsPath(f int) string { return fmt.Sprintf("missions/%d/reviews.jsonl", f) }

// Evening review verdicts.
const (
	ReviewAccepted  = "accepted"
	ReviewCommented = "commented"
)

// The evening card's own steps.
const (
	eveningAccept  = "inbox: accept the story"
	eveningComment = "inbox: comment on the story"
)

// StoryReview is one person's evening review of a story: a line of
// missions/<f>/reviews.jsonl.
type StoryReview struct {
	Story   int    `json:"story"`
	PR      int    `json:"pr,omitempty"`
	Head    string `json:"head,omitempty"` // the merged head the card showed the evidence of
	Pack    string `json:"pack,omitempty"` // the pack the card showed, by its ledger path and SHA-256
	Verdict string `json:"verdict"`        // accepted or commented
	By      string `json:"by"`
	At      string `json:"at"`
	Comment string `json:"comment,omitempty"` // a comment's URL on the story
}

// EveningCriterion is one criterion of a story on its evening card, with its
// evidence from the pack at the merged head (none when the pack maps none).
type EveningCriterion struct {
	N        int               `json:"n"`
	Kind     string            `json:"evidence_kind"`
	Text     string            `json:"text"`
	Evidence []EveningEvidence `json:"evidence"`
}

// EveningEvidence is one artifact of the pack, where a person finds it.
type EveningEvidence struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Link string `json:"link"`
}

// eveningStory is a story agreed today, as the ledger has it.
type eveningStory struct {
	feature   int
	milestone string
	number    int
	plan      *MissionPlan
	pack      *proof.Entry      // its pack at the merged head; nil when there is none, or it did not merge
	packErr   string            // why its packs cannot be read
	reviewed  bool              // a person reviewed it today at that head
	images    map[string][]byte // its screenshots by artifact path, read only where the terminal shows images
}

// eveningDay is today's local day and the end of its business hours, when now
// is at or past that end on a business day.
func eveningDay(clock config.Clock, now time.Time) (day, end time.Time, ok bool) {
	now = now.In(clock.Loc)
	y, m, d := now.Date()
	day = time.Date(y, m, d, 0, 0, 0, 0, clock.Loc)
	end = time.Date(y, m, d, clock.End/60, clock.End%60, 0, 0, clock.Loc)
	return day, end, clock.Days[now.Weekday()] && !now.Before(end)
}

// milestoneStart is when milestone i of p started: the mission's start for
// the first, else the pass of the gate before it; ok is false before then.
func milestoneStart(p *MissionPlan, i int) (time.Time, bool) {
	at := p.StartedAt
	if i > 0 {
		if g := p.Milestones[i-1].Gate; g.State == GatePassed {
			at = g.At
		} else {
			at = ""
		}
	}
	t, err := time.Parse(time.RFC3339, at)
	return t, err == nil
}

// showsImages reports whether the terminal draws an inline image (the iTerm2
// image protocol); elsewhere a card gives each image's path and link.
func (e *Env) showsImages() bool {
	switch e.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm":
		return true
	}
	return false
}

// inlineImage is b drawn inline by the iTerm2 image protocol.
func inlineImage(name string, b []byte) string {
	return fmt.Sprintf("\x1b]1337;File=name=%s;size=%d;inline=1:%s\a", base64.StdEncoding.EncodeToString([]byte(name)), len(b), base64.StdEncoding.EncodeToString(b))
}

// eveningStories reads the stories agreed today from the ledger, each with
// its pack at the head that head says merged ("" when it did not merge). It
// reads nothing before the end of today's business hours.
func (e *Env) eveningStories(ctx context.Context, head func(story int) string) ([]eveningStory, error) {
	if config.RepoRoot(e.Dir) == "" {
		return nil, nil
	}
	l, err := e.Config()
	if err != nil { // `aayu inbox` itself refuses such a config; `--list` shows the rest without its clock
		return nil, nil
	}
	clock, err := l.Config.Clock()
	if err != nil {
		return nil, err
	}
	day, end, ok := eveningDay(clock, e.Now())
	if !ok {
		return nil, nil
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return nil, err
	}
	repo, w, plans, err := e.ledgerPlans(ctx, tok, "human")
	if err != nil || len(plans) == 0 {
		return nil, err
	}
	t, err := w.Read(ctx)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	packs := &proof.Store{Repo: repo, Ledger: w}
	var media *proof.Store // the media bucket's store, opened once where the terminal draws images
	var out []eveningStory
	for _, p := range plans {
		if p.StartedBy == "" {
			continue
		}
		reviews, err := readReviews(ctx, t, p.Feature)
		if err != nil {
			return nil, err
		}
		for i, m := range p.Milestones {
			if at, ok := milestoneStart(p, i); !ok || at.Before(day) || !at.Before(end) {
				continue
			}
			for _, n := range m.Stories {
				st := eveningStory{feature: p.Feature, milestone: m.Name, number: n, plan: p}
				h := head(n)
				for _, r := range reviews {
					if at, err := time.Parse(time.RFC3339, r.At); err == nil && r.Story == n && r.Head == h && !at.Before(day) {
						st.reviewed = true
					}
				}
				if h != "" && !st.reviewed {
					es, err := packs.Packs(ctx, t, n)
					if err != nil {
						st.packErr = oneLineErr(err)
					}
					for _, x := range es { // best first: an authoritative pack before an advisory one
						if x.Pack.PRHeadSHA == h {
							st.pack = &x
							break
						}
					}
				}
				if st.pack != nil && e.showsImages() {
					if media == nil {
						if media, err = e.proofStore(ctx, c, tok, "human", ""); err != nil {
							media = packs // no bucket to read: each image keeps its path and link
						}
					}
					open := media.Opener(ctx, t, *st.pack)
					for _, a := range st.pack.Pack.Artifacts {
						if a.Kind == "screenshot" {
							if b, err := open(a); err == nil {
								if st.images == nil {
									st.images = map[string][]byte{}
								}
								st.images[a.Path] = b
							}
						}
					}
				}
				out = append(out, st)
			}
		}
	}
	return out, nil
}

// readReviews reads missions/<f>/reviews.jsonl; a line it cannot read is no review.
func readReviews(ctx context.Context, t *ledger.Tree, f int) ([]StoryReview, error) {
	raw, err := t.File(ctx, MissionReviewsPath(f))
	if err := ignoreMissing(err); err != nil {
		return nil, err
	}
	var out []StoryReview
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r StoryReview
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Story != 0 {
			out = append(out, r)
		}
	}
	return out, nil
}

// eveningItems are the evening cards: one for each story agreed today that
// no person reviewed yet. delivered is the PR merged into dev that delivers
// a story, as the other cards read it.
func (s *inboxSource) eveningItems(ctx context.Context, delivered func(int) *audit.PR) ([]InboxItem, error) {
	head := func(n int) string {
		if pr := delivered(n); pr != nil {
			return s.pulls[pr.Number].GetHead().GetSHA()
		}
		return ""
	}
	stories, err := s.evening(ctx, head)
	if err != nil {
		return nil, err
	}
	repoURL := ""
	var out []InboxItem
	for _, st := range stories {
		n, is := st.number, s.issues[st.number]
		if st.reviewed || is == nil {
			continue
		}
		if i := strings.LastIndex(is.GetHTMLURL(), "/issues/"); i >= 0 {
			repoURL = is.GetHTMLURL()[:i]
		}
		labels := github.IssueLabels(is)
		w, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), labels)
		crit, _ := workitems.ParseCriteria(w.Fields[workitems.FieldCriteria])
		it := InboxItem{Kind: InboxEvening, Number: n, Title: termText(is.GetTitle()), URL: is.GetHTMLURL(), Seat: verbs.EngineeringManager,
			Evidence: is.GetHTMLURL(), Updated: is.GetUpdatedAt().Time, Feature: st.feature, labels: labels, CounselFrom: "facts"}
		it.Lines = append(it.Lines, fmt.Sprintf("Agreed this morning: %s of #%d's mission.", st.milestone, st.feature))
		pr := delivered(n)
		shown := 0
		if pr == nil {
			why, next := s.notMerged(st.plan, n)
			it.f.problem = "not merged"
			it.Lines = append(it.Lines, "not merged: "+why+".", "Next: "+next+".")
			it.Counsel = fmt.Sprintf("What this is about: #%d was agreed this morning and did not merge: %s. Nothing is built to accept yet. "+
				"Next: %s. If you want something else, a comment on #%d says so, and the mission's feedback pass (#1014) reads it.", n, why, next, n)
			it.Options = []InboxAction{
				{N: 1, Label: fmt.Sprintf("comment on #%d", n), Command: eveningComment, Ask: fmt.Sprintf("What do you want? Your line is posted on #%d as your comment.", n)},
				{N: 2, Label: "not now: nothing is written", Command: inboxLeave},
			}
		} else {
			p := s.pulls[pr.Number]
			it.PR, it.Head, it.Evidence, it.mergeSHA = pr.Number, p.GetHead().GetSHA(), p.GetHTMLURL(), pr.MergeSHA
			if st.pack != nil { // what an accept accepts: this pack, as read
				b, _ := proof.Encode(st.pack.Pack)
				sum := sha256.Sum256(b)
				it.Pack = termText(st.pack.Path) + "@sha256:" + hex.EncodeToString(sum[:])
			}
			it.f.problem = "agreed today: review it"
			switch {
			case st.pack != nil:
				it.Lines = append(it.Lines, fmt.Sprintf("Merged in #%d at %s. Evidence: the %s pack %s on the ledger.", pr.Number, short(it.Head), st.pack.Pack.Authority.Kind, termText(path.Dir(st.pack.Path))))
			case st.packErr != "":
				it.Lines = append(it.Lines, fmt.Sprintf("Merged in #%d at %s. Its packs cannot be read: %s.", pr.Number, short(it.Head), termText(st.packErr)))
			default:
				it.Lines = append(it.Lines, fmt.Sprintf("Merged in #%d at %s. No proof pack on the ledger is for that head.", pr.Number, short(it.Head)))
			}
			it.Options = []InboxAction{
				{N: 1, Label: fmt.Sprintf("accept #%d: it does what you agreed", n), Command: eveningAccept},
				{N: 2, Label: fmt.Sprintf("say what is wrong, in a comment on #%d", n), Command: eveningComment, Ask: fmt.Sprintf("What is wrong? Your line is posted on #%d as your comment.", n)},
				{N: 3, Label: "not now: nothing is written", Command: inboxLeave},
			}
		}
		for _, c := range crit {
			ec := EveningCriterion{N: c.N, Kind: termText(c.Evidence), Text: termText(fmt.Sprintf("state: %s; expected: %s", c.State, c.Expected)), Evidence: []EveningEvidence{}}
			if st.pack != nil {
				for _, pc := range st.pack.Pack.Criteria {
					if pc.ID != c.N {
						continue
					}
					for _, rel := range pc.Evidence {
						ev := EveningEvidence{Path: termText(rel), Link: termText(evidenceLink(repoURL, *st.pack, rel))}
						if a, ok := st.pack.Pack.Artifact(rel); ok {
							ev.Kind = termText(a.Kind)
						}
						ec.Evidence = append(ec.Evidence, ev)
					}
				}
			}
			it.Lines = append(it.Lines, fmt.Sprintf("%d. [%s] %s", ec.N, ec.Kind, ec.Text))
			if len(ec.Evidence) == 0 {
				it.Lines = append(it.Lines, "   No evidence in the pack for this criterion.")
			} else {
				shown++
			}
			for _, ev := range ec.Evidence {
				it.Lines = append(it.Lines, fmt.Sprintf("   %s: %s · %s", or(ev.Kind, "artifact"), ev.Path, ev.Link))
				if b := st.images[ev.Path]; b != nil {
					if it.images == nil {
						it.images = map[int][]byte{}
					}
					it.images[len(it.Lines)-1] = b
				}
			}
			it.Criteria = append(it.Criteria, ec)
		}
		if pr != nil {
			it.Why = fmt.Sprintf("#%d was agreed this morning and merged in #%d; its pack at the merged head shows evidence for %d of %d criteria.", n, pr.Number, shown, len(crit))
			it.Counsel = fmt.Sprintf("What this is about: whether #%d does what you agreed this morning. It merged in #%d at %s, and its pack at that head "+
				"shows evidence for %d of %d criteria. Only you accept it. If something is wrong, a comment says what, stays on #%d, and the mission's feedback pass (#1014) reads it.",
				n, pr.Number, short(it.Head), shown, len(crit), n)
		} else {
			it.Why = fmt.Sprintf("#%d was agreed this morning and did not merge.", n)
		}
		out = append(out, it)
	}
	return out, nil
}

// evidenceLink is where a person opens an artifact of pack x: its file on the
// ledger branch, else the merge gate run that holds its bytes, else the pack.
func evidenceLink(repoURL string, x proof.Entry, rel string) string {
	dir := path.Dir(x.Path)
	if a, ok := x.Pack.Artifact(rel); ok && (a.Store == proof.StoreGit || a.Store == proof.StoreLFS) {
		return repoURL + "/blob/ledger/" + path.Join(dir, "artifacts", rel)
	}
	if id := x.Pack.Authority.RunID; id != 0 {
		return fmt.Sprintf("%s/actions/runs/%d", repoURL, id)
	}
	return repoURL + "/blob/ledger/" + x.Path
}

// notMerged is the mission's reason that story n of p did not merge, and the next step.
func (s *inboxSource) notMerged(p *MissionPlan, n int) (why, next string) {
	f, show := p.Feature, fmt.Sprintf("`aayu mission show %d` says what the mission does next", p.Feature)
	if p.State != "" {
		return "the mission " + termText(p.State), fmt.Sprintf("a new mission plan starts it: `aayu ready %d`", f)
	}
	if p.Pause != nil {
		return "the mission is paused: " + termText(p.Pause.Reason), fmt.Sprintf("the person who paused it resumes it: `aayu mission resume %d`", f)
	}
	if st := p.stopped(n); st != nil {
		return fmt.Sprintf("@%s stopped it: %s", st.By, termText(st.Reason)), fmt.Sprintf("`aayu mission resume %d --story %d` when it may go on", f, n)
	}
	if ss := p.stateOf(n); ss != nil && ss.State != "" {
		return "its run is " + termText(ss.summary()), show
	}
	for _, m := range p.Missions {
		for _, ms := range m.Stories {
			if ms.Number == n && ms.Waits != "" {
				return "it " + termText(ms.Waits), "it starts when that merges; " + show
			}
		}
	}
	for _, pr := range s.pulls {
		if pr.GetState() == "open" && StoryOf(pr.GetBody(), "", pr.GetHead().GetRef()) == n {
			return fmt.Sprintf("its PR #%d is open and has not merged", pr.GetNumber()), fmt.Sprintf("its builder and the merge gate move next: `aayu inspect %d` says what waits", pr.GetNumber())
		}
	}
	return "it has not started", "the mission starts it when an agent is free; " + show
}

// eveningAct records a person's evening review of a story: an accept, or
// their comment on the story, each a line of missions/<f>/reviews.jsonl.
func (e *Env) eveningAct(ctx context.Context, c *github.Client, repo, login string, it InboxItem, a InboxAction) error {
	_, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	r := StoryReview{Story: it.Number, PR: it.PR, Head: it.Head, Pack: it.Pack, Verdict: ReviewAccepted, By: login, At: e.Now().UTC().Format(time.RFC3339)}
	if a.Command == eveningComment {
		text := strings.TrimSpace(a.Answer)
		if text == "" {
			return Errorf(ExitUsage, "inbox: a comment on #%d needs your words", it.Number)
		}
		owner, name, _ := github.SplitRepo(repo)
		var url string
		_, err := c.Write(ctx, actions.Action{Kind: "github.comment.create", Target: fmt.Sprintf("%s#%d", repo, it.Number),
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, it.Number)}, Payload: map[string]any{"kind": InboxEvening, "by": login}},
			func(ctx context.Context) (any, error) {
				cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, it.Number, &gh.IssueComment{Body: gh.Ptr(text)})
				url = cm.GetHTMLURL()
				return cm.GetID(), err
			})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		r.Verdict, r.Comment = ReviewCommented, url
	}
	b, _ := json.Marshal(r)
	commit := ledger.Commit{Message: fmt.Sprintf("mission: evening review of #%d by %s (%s)", it.Number, login, r.Verdict),
		Append: map[string][]byte{MissionReviewsPath(it.Feature): append(b, '\n')}}
	if _, err := w.Write(ctx, commit); err != nil {
		if errors.Is(err, ledger.ErrNoLedger) {
			return Errorf(ExitBlocked, "inbox: this repository has no ledger to record the review of #%d on", it.Number)
		}
		return Wrap(ExitExternal, err)
	}
	if r.Verdict == ReviewCommented {
		fmt.Fprintf(e.Out, "  done: your comment is on #%d (%s), and %s records it for the mission's feedback pass\n", it.Number, r.Comment, MissionReviewsPath(it.Feature))
		return nil
	}
	fmt.Fprintf(e.Out, "  done: @%s accepted #%d at %s, recorded in %s\n", login, it.Number, short(it.Head), MissionReviewsPath(it.Feature))
	return nil
}

// withoutAccept is the evening card as `aayu inbox --list` shows it: with no
// accept, which only a person gives, in `aayu inbox`.
func withoutAccept(it InboxItem) InboxItem {
	if it.Kind != InboxEvening {
		return it
	}
	var opts []InboxAction
	for _, o := range it.Options {
		if o.Command != eveningAccept {
			opts = append(opts, o)
		}
	}
	it.Options = opts
	return it
}
