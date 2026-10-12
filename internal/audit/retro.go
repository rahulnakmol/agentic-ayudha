package audit

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// CheckpointBudget is the most checkpoints a story of each Effort should take
// (one per layer of its footprint, WP5.4); more is an estimate miss.
var CheckpointBudget = map[string]int{"XS": 1, "S": 2, "M": 3, "L": 5}

// CorrectionRateMax is the correction rate above which retro files a finding.
const CorrectionRateMax = 0.2

// FeedbackPrefix starts the title of every Feedback issue retro files; retro
// files a finding once, matched by title.
const FeedbackPrefix = "Feedback: retro: "

// RetroStory is one story delivered in the window.
type RetroStory struct {
	Number      int     `json:"number"`
	Title       string  `json:"title"`
	PR          int     `json:"pr,omitempty"`
	Effort      string  `json:"effort"`
	Budget      int     `json:"checkpoint_budget"`
	Observed    int     `json:"checkpoints_observed"` // 0 when no pack records a checkpoint
	Corrections int     `json:"corrections"`
	CostUSD     float64 `json:"cost_usd"`
	Proof       string  `json:"proof"`
}

// SkillRate is the correction rate of agent PRs that changed one skill.
type SkillRate struct {
	Skill     string `json:"skill"`
	PRs       int    `json:"prs"`
	Corrected int    `json:"corrected"`
}

// Finding is one retro finding, filed as a Feedback issue.
type Finding struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Issue  int    `json:"issue,omitempty"` // the Feedback issue that already holds it, or the one filed
}

// Retro is a built retro.
type Retro struct {
	Inputs      Inputs                 `json:"inputs"`
	Stories     []RetroStory           `json:"stories"`
	AgentMerges int                    `json:"agent_merges"`
	Corrected   int                    `json:"corrected_merges"`
	Corrections []autonomy.Correction  `json:"corrections"`
	Skills      []SkillRate            `json:"skills"`
	Streaks     []autonomy.LayerStreak `json:"streaks"`
	FloorMisses []string               `json:"floor_misses"`
	Findings    []Finding              `json:"findings"`
}

// BuildRetro computes the retro for the window. perStoryUSD is [budget] per_story_usd.
func BuildRetro(in Inputs, d *Data, perStoryUSD float64) *Retro {
	r := &Retro{Inputs: in, Stories: []RetroStory{}, Corrections: []autonomy.Correction{}, Skills: []SkillRate{}, Streaks: d.Streaks, FloorMisses: []string{}, Findings: []Finding{}}
	if r.Streaks == nil {
		r.Streaks = []autonomy.LayerStreak{}
	}
	window := fmt.Sprintf("%s to %s", in.Since.Format("2006-01-02T15:04Z"), in.Until.Format("2006-01-02T15:04Z"))
	corrected := map[int]bool{}
	for _, c := range d.Corrections {
		if d.in(in, c.At) {
			r.Corrections = append(r.Corrections, c)
			corrected[c.PR] = true
		}
	}
	cost := map[string]float64{}
	for _, rec := range d.Records {
		if rec.Story != "" && d.in(in, rec.Time) {
			cost[rec.Story] += rec.CostUSD
		}
	}
	for _, s := range d.stories(in) {
		if s.PR == 0 || !d.in(in, s.Merged) {
			continue
		}
		it := d.item(s.Number)
		rs := RetroStory{Number: s.Number, Title: s.Title, PR: s.PR, CostUSD: round2(cost[fmt.Sprintf("#%d", s.Number)]), Proof: "covered"}
		if !s.Covered {
			rs.Proof = s.Reason
		}
		for _, l := range it.Labels {
			if e, ok := strings.CutPrefix(l, "effort:"); ok {
				rs.Effort, rs.Budget = e, CheckpointBudget[e]
			}
		}
		for _, p := range d.Packs {
			if p.Story != s.Number {
				continue
			}
			if _, n, ok := strings.Cut(p.Checkpoint, "/"); ok {
				if v, err := strconv.Atoi(n); err == nil && v > rs.Observed {
					rs.Observed = v
				}
			}
		}
		for _, c := range r.Corrections {
			if c.PR == s.PR {
				rs.Corrections++
			}
		}
		r.Stories = append(r.Stories, rs)
		ref := fmt.Sprintf("#%d %s (PR #%d)", s.Number, s.Title, s.PR)
		if rs.Budget > 0 && rs.Observed > rs.Budget {
			r.Findings = append(r.Findings, Finding{Title: fmt.Sprintf("#%d took %d checkpoints, over the %d of Effort %s", s.Number, rs.Observed, rs.Budget, rs.Effort),
				Detail: fmt.Sprintf("%s was estimated at Effort %s (at most %d checkpoints) and its pack records %d.", ref, rs.Effort, rs.Budget, rs.Observed)})
		}
		if perStoryUSD > 0 && rs.CostUSD > perStoryUSD {
			r.Findings = append(r.Findings, Finding{Title: fmt.Sprintf("#%d cost %.2f USD, over the %.2f USD story budget", s.Number, rs.CostUSD, perStoryUSD),
				Detail: fmt.Sprintf("%s logged %.2f USD of model spend in actions segments; [budget] per_story_usd is %.2f.", ref, rs.CostUSD, perStoryUSD)})
		}
		if !s.Covered && !s.PreGate {
			r.Findings = append(r.Findings, Finding{Title: fmt.Sprintf("#%d merged without proof after the merge gate existed", s.Number),
				Detail: fmt.Sprintf("%s merged without an authoritative pack that covers it: %s.", ref, s.Reason)})
		}
	}
	skills := map[string]*SkillRate{}
	for _, a := range d.AgentPRs {
		if !d.in(in, a.MergedAt) {
			continue
		}
		r.AgentMerges++
		if corrected[a.Number] {
			r.Corrected++
		}
		for _, sk := range a.Skills {
			if skills[sk] == nil {
				skills[sk] = &SkillRate{Skill: sk}
			}
			skills[sk].PRs++
			if corrected[a.Number] {
				skills[sk].Corrected++
			}
		}
	}
	if r.AgentMerges > 0 && float64(r.Corrected)/float64(r.AgentMerges) > CorrectionRateMax {
		r.Findings = append(r.Findings, Finding{Title: fmt.Sprintf("correction rate %d/%d above %.0f %%, %s", r.Corrected, r.AgentMerges, CorrectionRateMax*100, window),
			Detail: fmt.Sprintf("%d of %d agent PRs merged %s were corrected (WP5.7 kinds).", r.Corrected, r.AgentMerges, window)})
	}
	for _, k := range sortedKeys(skills) {
		s := skills[k]
		r.Skills = append(r.Skills, *s)
		if float64(s.Corrected)/float64(s.PRs) > CorrectionRateMax {
			r.Findings = append(r.Findings, Finding{Title: fmt.Sprintf("skill `%s` correction rate %d/%d above %.0f %%, %s", s.Skill, s.Corrected, s.PRs, CorrectionRateMax*100, window),
				Detail: fmt.Sprintf("%d of %d agent PRs that changed skills/%s/ and merged %s were corrected. `aayu evolve --skill %s` takes this Feedback once it is accepted.", s.Corrected, s.PRs, s.Skill, window, s.Skill)})
		}
	}
	for _, p := range d.Packs {
		t, err := parseTime(p.CreatedAt)
		if err != nil || !d.in(in, t) {
			continue
		}
		for _, c := range p.Checks {
			if c.Result == proof.Fail || c.Result == proof.Blocked || proof.BlocksShip(c) {
				miss := fmt.Sprintf("%s %s on #%d (%s pack at %s)", c.Floor, c.Result, p.Story, p.Kind, short(p.Head))
				r.FloorMisses = append(r.FloorMisses, miss)
				r.Findings = append(r.Findings, Finding{Title: "floor miss: " + miss, Detail: fmt.Sprintf("`%s`: %s %s (%s).", p.Path, c.Floor, c.Result, c.Detail)})
			}
		}
	}
	for i := range r.Findings {
		for _, it := range d.Items {
			if !it.PR && it.Title == FeedbackPrefix+r.Findings[i].Title {
				r.Findings[i].Issue = it.Number
			}
		}
	}
	return r
}

// Markdown renders the retro report.
func (r *Retro) Markdown() []byte {
	var b bytes.Buffer
	in := r.Inputs
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("# Retro: %s, %s to %s\n\n", in.Repo, stamp(in.Since), stamp(in.Until))
	p("Written by `aayu retro` (docs/BUILD-PLAN.md WP7.6). Findings are filed as Feedback issues on this repository, never PRs.\n\n")
	b.WriteString(in.header("retro", ""))
	p("## Estimate versus observed checkpoints\n\n")
	if len(r.Stories) == 0 {
		p("No stories delivered in the window.\n\n")
	} else {
		p("| Story | PR | Effort | Checkpoint budget | Observed | Corrections | Cost (USD) | Proof |\n|---|---|---|---|---|---|---|---|\n")
		for _, s := range r.Stories {
			obs := "n/a (no pack)"
			if s.Observed > 0 {
				obs = fmt.Sprint(s.Observed)
			}
			p("| #%d %s | #%d | %s | %d | %s | %d | %.2f | %s |\n", s.Number, cell(s.Title), s.PR, or(s.Effort, "—"), s.Budget, obs, s.Corrections, s.CostUSD, cell(s.Proof))
		}
		p("\n")
	}
	p("## Corrections and streaks\n\n")
	p("Agent PRs merged in the window: %d; corrected: %d.\n\n", r.AgentMerges, r.Corrected)
	for _, c := range r.Corrections {
		p("- #%d %s: %s (%s)\n", c.PR, c.Kind, c.Ref, strings.Join(c.Layers, ", "))
	}
	if len(r.Skills) > 0 {
		p("\n| Skill | Agent PRs | Corrected |\n|---|---|---|\n")
		for _, s := range r.Skills {
			p("| `%s` | %d | %d |\n", s.Skill, s.PRs, s.Corrected)
		}
	}
	if len(r.Streaks) > 0 {
		p("\n| Layer | Streak | Agent merges | Never-layer | Autonomous |\n|---|---|---|---|---|\n")
		for _, s := range r.Streaks {
			p("| %s | %d | %d | %t | %t |\n", s.Layer, s.Streak, s.Merges, s.Never, s.Autonomous)
		}
	}
	p("\n## Floor misses\n\n")
	if len(r.FloorMisses) == 0 {
		p("None.\n\n")
	}
	for _, m := range r.FloorMisses {
		p("- %s\n", m)
	}
	p("## Findings\n\n")
	if len(r.Findings) == 0 {
		p("None.\n")
	}
	for _, f := range r.Findings {
		p("- %s\n", f.Title)
	}
	return b.Bytes()
}

// FeedbackBody is the issue-form body of the Feedback issue for a finding.
func (r *Retro) FeedbackBody(f Finding, path string) string {
	return fmt.Sprintf("### What happened or what you need\n\n%s\n\n### Context\n\nFiled by `aayu retro` (WP7.6) from the retro report `%s` on the ledger "+
		"(window %s to %s, ledger `%s`). Accept it to have it acted on; `aayu evolve` takes accepted Feedback about a skill.\n",
		f.Detail, path, stamp(r.Inputs.Since), stamp(r.Inputs.Until), short(r.Inputs.Ledger))
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
