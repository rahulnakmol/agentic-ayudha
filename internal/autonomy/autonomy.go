// Package autonomy computes the autonomy streak (WP5.7, V4): for each layer,
// the number of consecutive correction-free agent PRs merged on it, counted
// back from the newest. It is a pure function of merged-PR history, so no
// agent-writable counter exists; the protected aayudha-streak workflow runs it
// and records the corrections it finds on the ledger.
package autonomy

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// Correction kinds (WP5.7).
const (
	KindChangesRequested = "changes-requested" // a review requested changes
	KindHumanCommit      = "non-bot-commit"    // a human pushed a commit to the agent's PR
	KindPostPass         = "post-pass"         // an inspect/review disposition of fix or rebuild after a human pass
	KindRevert           = "revert"            // a later merged PR reverted it
	KindLinkedIssue      = "linked-issue"      // a Bug or Incident linked to it within [autonomy] correction_days
	KindManual           = "manual"            // the aayu:correction label
)

// Labels the streak reads.
const (
	LabelCorrection    = "aayu:correction"
	LabelAgentAuthored = "agent-authored"
	LabelHumanFallback = "identity:human-fallback"
	CheckpointTrailer  = "Aayudha-Checkpoint:"
	// HostTrailer and ModelTrailer mark a commit an agent made outside aayu
	// build, as build's checkpoints carry them too: the host and the model that
	// wrote it, self-reported like the agent-authored label.
	HostTrailer  = "Aayudha-Host:"
	ModelTrailer = "Aayudha-Model:"
)

// Review is a PR review.
type Review struct {
	Login string    `json:"login"`
	Bot   bool      `json:"bot"`
	State string    `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	At    time.Time `json:"at"`
}

// Commit is a commit on a PR.
type Commit struct {
	SHA     string `json:"sha"`
	Login   string `json:"login"` // GitHub login of the author; "" when the email is not linked to an account
	Bot     bool   `json:"bot"`
	Message string `json:"message"`
	Parents int    `json:"parents"`
}

// Comment is a PR conversation comment.
type Comment struct {
	Body string    `json:"body"`
	At   time.Time `json:"at"`
}

// PR is a merged pull request with what the corrections need.
type PR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	AuthorBot bool      `json:"author_bot"`
	Labels    []string  `json:"labels"`
	Head      string    `json:"head,omitempty"`
	Rung      string    `json:"rung,omitempty"` // the review ladder's rung at head (GateRung), "" when unknown
	MergedAt  time.Time `json:"merged_at"`
	MergeSHA  string    `json:"merge_sha"`
	Files     []string  `json:"files"`
	Reviews   []Review  `json:"reviews"`
	Commits   []Commit  `json:"commits"`
	Comments  []Comment `json:"comments"`
}

// Issue is a Bug or Incident that may link to a PR.
type Issue struct {
	Number    int       `json:"number"`
	Kind      string    `json:"kind"` // bug or incident
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Correction is one detected correction of an agent PR.
type Correction struct {
	PR     int       `json:"pr"`
	Kind   string    `json:"kind"`
	Ref    string    `json:"ref"` // what shows it: a review, commit, comment, PR, issue, or label
	At     time.Time `json:"at"`
	Layers []string  `json:"layers"`
}

// Key identifies a correction for de-duplication on the ledger.
func (c Correction) Key() string { return fmt.Sprintf("%d/%s/%s", c.PR, c.Kind, c.Ref) }

// Rules configure detection.
type Rules struct {
	Bots   []string // [identity] bot_logins
	Days   int      // [autonomy] correction_days
	Layers verify.LayerGlobs
}

func (r Rules) bot(login string, typeBot bool) bool {
	return typeBot || strings.HasSuffix(login, "[bot]") ||
		slices.ContainsFunc(r.Bots, func(b string) bool { return strings.EqualFold(b, login) })
}

// Agent reports whether pr is agent work the streak counts: opened by a bot,
// or labelled agent-authored or identity:human-fallback (the day-one path).
func (r Rules) Agent(pr PR) bool {
	return r.bot(pr.Author, pr.AuthorBot) || slices.Contains(pr.Labels, LabelAgentAuthored) || slices.Contains(pr.Labels, LabelHumanFallback)
}

var dispositionRE = regexp.MustCompile(`(?i)\bdisposition\b\W{0,5}(fix|rebuild)\b`)

// Detect finds the corrections of the agent PRs in prs (merged, any order);
// issues are the Bug and Incident issues that may link them.
func (r Rules) Detect(prs []PR, issues []Issue) []Correction {
	var out []Correction
	for _, p := range prs {
		if !r.Agent(p) {
			continue
		}
		add := func(kind, ref string, at time.Time) {
			out = append(out, Correction{PR: p.Number, Kind: kind, Ref: ref, At: at.UTC(), Layers: footprint(r.Layers, p.Files)})
		}
		var pass time.Time
		for _, rv := range p.Reviews {
			if rv.State == "CHANGES_REQUESTED" {
				add(KindChangesRequested, "review by "+rv.Login, rv.At)
			}
			if rv.State == "APPROVED" && !r.bot(rv.Login, rv.Bot) && (pass.IsZero() || rv.At.Before(pass)) {
				pass = rv.At
			}
		}
		for _, c := range p.Commits {
			if c.Parents <= 1 && !r.bot(c.Login, c.Bot) && !agentCommit(c.Message) {
				who := c.Login
				if who == "" {
					who = "an unlinked author"
				}
				add(KindHumanCommit, "commit "+short(c.SHA)+" by "+who, p.MergedAt)
			}
		}
		for _, c := range p.Comments {
			if m := dispositionRE.FindStringSubmatch(c.Body); m != nil && strings.Contains(c.Body, "<!-- aayudha:") && !pass.IsZero() && c.At.After(pass) {
				add(KindPostPass, "disposition "+strings.ToLower(m[1]), c.At)
			}
		}
		for _, q := range prs {
			if q.Number != p.Number && q.MergedAt.After(p.MergedAt) && reverts(q, p) {
				add(KindRevert, "#"+strconv.Itoa(q.Number), q.MergedAt)
			}
		}
		for _, is := range issues {
			if !is.CreatedAt.Before(p.MergedAt) && is.CreatedAt.Before(p.MergedAt.AddDate(0, 0, r.Days)) && refers(is.Body, p.Number) {
				add(KindLinkedIssue, fmt.Sprintf("%s #%d", is.Kind, is.Number), is.CreatedAt)
			}
		}
		if slices.Contains(p.Labels, LabelCorrection) {
			add(KindManual, "label "+LabelCorrection, p.MergedAt)
		}
	}
	return out
}

var agentTrailer = regexp.MustCompile(`(?m)^(?:` + HostTrailer + `|` + ModelTrailer + `)`)

// agentCommit reports whether a commit message carries a trailer only an
// agent's commit has: a build checkpoint, or an Aayudha-Host or Aayudha-Model
// line.
func agentCommit(msg string) bool {
	return strings.Contains(msg, CheckpointTrailer) || agentTrailer.MatchString(msg)
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// reverts reports whether q reverts p: GitHub's revert PR (title `Revert
// "<title>"`, body "Reverts owner/repo#N"), or a commit that reverts p's merge.
func reverts(q, p PR) bool {
	if !strings.HasPrefix(strings.ToLower(q.Title), "revert") {
		return false
	}
	if q.Title == `Revert "`+p.Title+`"` || regexp.MustCompile(`(?i)\breverts\b[^\n]*#`+strconv.Itoa(p.Number)+`\b`).MatchString(q.Body) {
		return true
	}
	for _, c := range q.Commits {
		if p.MergeSHA != "" && strings.Contains(c.Message, "This reverts commit "+p.MergeSHA) {
			return true
		}
	}
	return false
}

// refers reports whether text links PR n: "#n" or ".../pull/n".
func refers(text string, n int) bool {
	return regexp.MustCompile(`(^|[^\w&/])#` + strconv.Itoa(n) + `\b|/pull/` + strconv.Itoa(n) + `\b`).MatchString(text)
}

// LayerStreak is the streak on one layer.
type LayerStreak struct {
	Layer      string `json:"layer"`
	Streak     int    `json:"streak"`
	Merges     int    `json:"agent_merges"`      // agent PRs on this layer in the scanned history
	Skipped    int    `json:"skipped,omitempty"` // agent merges since the last correction whose review was not proven (rung C or unknown), not counted
	Never      bool   `json:"never"`             // [autonomy] never: always needs a human pass
	Autonomous bool   `json:"autonomous"`        // streak >= threshold and not a never-layer
	LastBreak  int    `json:"last_correction_pr,omitempty"`
}

// Streaks counts, per layer, the agent PRs on that layer merged after its
// newest corrected agent PR whose review was proven (Proven: a
// different-family or a person's review). An agent PR reviewed same-family
// (rung C), or with no rung on record, is skipped: it neither counts nor
// breaks the streak, though a correction on it still breaks it (Decision
// #469, detail 8). threshold is [autonomy] streak.
func (r Rules) Streaks(prs []PR, corrections []Correction, threshold int, never []string) []LayerStreak {
	corrected := map[int]bool{}
	for _, c := range corrections {
		corrected[c.PR] = true
	}
	ordered := slices.Clone(prs)
	slices.SortStableFunc(ordered, func(a, b PR) int { return b.MergedAt.Compare(a.MergedAt) })
	var out []LayerStreak
	for _, id := range ids(r.Layers) {
		s := LayerStreak{Layer: id, Never: slices.Contains(never, id)}
		broken := false
		for _, p := range ordered {
			if !r.Agent(p) || !slices.Contains(r.Layers.Footprint(p.Files), id) {
				continue
			}
			s.Merges++
			if broken {
				continue
			}
			if corrected[p.Number] {
				broken, s.LastBreak = true, p.Number
				continue
			}
			if !Proven(p.Rung) {
				s.Skipped++
				continue
			}
			s.Streak++
		}
		s.Autonomous = !s.Never && s.Streak >= threshold
		out = append(out, s)
	}
	return out
}

// Autonomous reports whether a story footprint may merge without a human pass:
// every layer in it is autonomous. An empty footprint never is.
func Autonomous(streaks []LayerStreak, footprint []string) bool {
	if len(footprint) == 0 {
		return false
	}
	for _, id := range footprint {
		i := slices.IndexFunc(streaks, func(s LayerStreak) bool { return s.Layer == id })
		if i < 0 || !streaks[i].Autonomous {
			return false
		}
	}
	return true
}

// ids returns the layer IDs, sorted.
func ids(l verify.LayerGlobs) []string {
	var out []string
	for id := range l {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// footprint is the layers files touch, never nil.
func footprint(l verify.LayerGlobs, files []string) []string {
	if f := l.Footprint(files); f != nil {
		return f
	}
	return []string{}
}

// ReadLayers reads the layer globs from LAYERS.md at root, the table every
// aayudha repository keeps (generated on aayudha itself); without one it falls
// back to the registry's layer table.
func ReadLayers(root string) verify.LayerGlobs {
	if l := verify.ReadLayers(root); len(l) > 0 && root != "" {
		return l
	}
	out := verify.LayerGlobs{}
	for _, l := range verbs.LayerTable {
		out[string(l.ID)] = l.Paths
	}
	return out
}
