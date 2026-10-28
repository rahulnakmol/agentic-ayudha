package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// A Feature's mission (Feature #889, Decisions #915 and #974, Story #997).
// Approve to build writes the mission plan, missions/<f>/plan.json on the
// ledger: the graph of the Feature's stories and Bugs, the milestone of each,
// the missions, the agents, and the estimate. `aayu mission start <f>`, a
// person's decision, adds who started it, the tool, and the budget, once.
// Only these two write the plan; everyone reads the tool from it, never from
// a config.

func init() { register("mission", runMission) }

// MissionPlanSchema is the schema of a mission plan.
const MissionPlanSchema = "aayudha.mission.plan/v1"

// MissionPlanPath is where a Feature's mission plan lives on the ledger.
func MissionPlanPath(f int) string { return fmt.Sprintf("missions/%d/plan.json", f) }

// missionAgents is how many stories Feature f's mission runs at once, and its
// source: the Feature's own value from Approve to build (`aayu ready <f>
// --agents N`), else [build] agents committed on the integration branch at its
// tip, which a local value may only lower (Decision #918). A prompt never sets
// it: no command of the mission takes a limit.
func (e *Env) missionAgents(ctx context.Context, c *github.Client, repo string, f, approved int) (int, string, error) {
	if approved > 0 {
		return approved, fmt.Sprintf("aayu ready %d --agents %d at Approve to build", f, approved), nil
	}
	owner, name, _ := github.SplitRepo(repo)
	pol, dev, err := e.loopPolicy(ctx, c, owner, name)
	if err != nil {
		return 0, "", err
	}
	l, err := e.Config()
	if err != nil {
		return 0, "", err
	}
	n := pol.Config.Build.Agents
	if local := buildAgents(l, n); local < n {
		return local, fmt.Sprintf("a local [build] agents, under %d on %s", n, dev), nil
	}
	return n, "[build] agents on " + dev, nil
}

// MissionPlan is missions/<f>/plan.json.
type MissionPlan struct {
	Schema       string             `json:"schema"`
	Feature      int                `json:"feature"`
	Approval     int                `json:"approval"` // the version of specs/<f>/approved.json the plan was approved with
	ApprovedBy   string             `json:"approved_by"`
	ApprovedAt   string             `json:"approved_at"`
	Release      int                `json:"release,omitempty"` // the Release of a Tatkal Feature (#877 S8)
	Graph        []MissionItem      `json:"graph"`
	Milestones   []MissionMilestone `json:"milestones"`
	FeatureDone  string             `json:"feature_done,omitempty"` // the milestone whose gate ends the Feature; a Tatkal Feature has none
	CompleteWhen string             `json:"complete_when"`
	Agents       int                `json:"agents"` // the cap Approve to build showed: a record; the list reads it at each run (Env.missionAgents, #1000)
	AgentsSource string             `json:"agents_source"`
	Estimate     MissionEstimate    `json:"estimate"`
	Missions     []Mission          `json:"missions"`
	// Set once, at Start.
	StartedBy   string  `json:"started_by,omitempty"`
	StartedAt   string  `json:"started_at,omitempty"`
	Tool        string  `json:"tool,omitempty"`
	ToolSource  string  `json:"tool_source,omitempty"`
	Budget      float64 `json:"budget,omitempty"` // USD
	BudgetBasis string  `json:"budget_basis,omitempty"`
	// StoryStarts are a Tatkal Feature's stories that a person started on their own tool (R9).
	StoryStarts []MissionStoryStart `json:"story_starts,omitempty"`
	// Pause is a stop's record (#1002); show prints its reason.
	Pause *MissionPause `json:"pause,omitempty"`
	// State is "ended (<reason>)" after the person who started the mission
	// ends it (`aayu mission stop <f> --end`, Story #1002); empty before.
	State string `json:"state,omitempty"`
	// Stopped are the stories and Bugs a person stopped alone (`aayu mission
	// stop <f> --story <n>`, Story #1002): the rest of the mission runs on.
	Stopped []MissionStopped `json:"stopped,omitempty"`
	// Lock is the mission's lock on its issues (Story #1013); --end ends it.
	Lock     *MissionLock     `json:"lock,omitempty"`
	Controls []MissionControl `json:"controls"`
	// GateFixes are the fix PRs of failed gate checks (Story #1001).
	GateFixes []GateFix `json:"gate_fixes,omitempty"`
	// EventsCursor is the newest event the Kanha has acted on (Story #1009, J4 rule 1): each wake reads every event
	// after it, not a fixed window. Unset, it is StartedAt.
	EventsCursor string `json:"events_cursor,omitempty"`
	// Wakes are the safety tick's newest wakes of the Kanha, at most keptWakes (Story #1009).
	Wakes []MissionWake `json:"wakes,omitempty"`
}

// MissionStopped is one story or Bug a person stopped, and why.
type MissionStopped struct {
	Number int    `json:"number"`
	Reason string `json:"reason"`
	By     string `json:"by"`
	At     string `json:"at"`
}

// stopped is story n's stop record in the plan, or nil.
func (p *MissionPlan) stopped(n int) *MissionStopped {
	if i := slices.IndexFunc(p.Stopped, func(s MissionStopped) bool { return s.Number == n }); i >= 0 {
		return &p.Stopped[i]
	}
	return nil
}

// MissionLock is the lock a mission holds on the bodies and the effort:,
// risk:, and priority: labels of its Feature, stories, and Bugs (Story #1013,
// Decision #974): active from the start until the mission ends.
type MissionLock struct {
	State       string        `json:"state"` // active or ended
	ConfirmedBy string        `json:"confirmed_by"`
	StartedAt   string        `json:"started_at"`
	EndedAt     *string       `json:"ended_at"`
	EndReason   *string       `json:"end_reason"`
	Issues      []LockedIssue `json:"issues"`
}

// LockedIssue is one issue the lock holds.
type LockedIssue struct {
	Number     int               `json:"number"`
	BodySHA256 string            `json:"body_sha256"`
	Labels     map[string]string `json:"labels"`
}

// MissionItem is one story or Bug of the Feature in the plan's graph.
type MissionItem struct {
	Number    int          `json:"number"`
	Kind      string       `json:"kind"` // story or bug
	Title     string       `json:"title"`
	Milestone string       `json:"milestone"`
	Effort    string       `json:"effort,omitempty"` // its effort:<size> label, which #886's working hours are per
	DependsOn []int        `json:"depends_on"`       // depends-on inside the Feature
	Outside   []MissionDep `json:"outside"`          // depends-on outside the Feature
}

// MissionDep is a depends-on target outside the Feature, with its Feature.
type MissionDep struct {
	Number  int `json:"number"`
	Feature int `json:"feature,omitempty"`
}

func (d MissionDep) String() string {
	if d.Feature != 0 && d.Feature != d.Number {
		return fmt.Sprintf("#%d (#%d)", d.Number, d.Feature)
	}
	return fmt.Sprintf("#%d", d.Number)
}

// MissionMilestone is one milestone: its stories and Bugs, and its gate.
type MissionMilestone struct {
	Name    string        `json:"name"`
	Stories []int         `json:"stories"`
	Gate    MilestoneGate `json:"gate"`
}

// MilestoneGate is a milestone's gate as it stands (Story #1001): it waits
// for the milestone's last story to merge, then each gate run on the dev tip
// sets it passed or failed with the SHA it judged. After [mission] max_no_go
// failed runs it is stopped, and the mission waits for a person.
type MilestoneGate struct {
	State  string `json:"state"` // waits, passed, failed, or stopped
	DevSHA string `json:"dev_sha,omitempty"`
	At     string `json:"at,omitempty"`
}

// UnmarshalJSON reads the gate of an earlier v1 plan, a string that said it
// runs when its last story merges: a gate that waits.
func (g *MilestoneGate) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		*g = MilestoneGate{State: GateWaits}
		return nil
	}
	type plain MilestoneGate
	return json.Unmarshal(b, (*plain)(g))
}

// Gate states.
const (
	GateWaits   = "waits"
	GatePassed  = "passed"
	GateFailed  = "failed"
	GateStopped = "stopped"
)

// GateFix is one fix PR of a failed gate check (Decision #974): the gate
// files no issue, and the plan's approval pre-authorises the PR. Waits says
// why it waits for a person ("budget").
type GateFix struct {
	PR        int          `json:"pr"`
	Milestone string       `json:"milestone"`
	Check     string       `json:"check"`
	DevSHA    string       `json:"dev_sha"`
	Approval  GateApproval `json:"approval"`
	At        string       `json:"at"`
	Waits     string       `json:"waits,omitempty"`
}

// GateApproval is the approved record a fix PR rests on, by its hash.
type GateApproval struct {
	Record string `json:"record"`
	SHA256 string `json:"sha256"`
}

// MissionEstimate is the plan's estimate: one run per story or Bug, and two per milestone.
type MissionEstimate struct {
	Runs       int    `json:"runs"`
	Stories    int    `json:"stories"`
	Milestones int    `json:"milestones"`
	Basis      string `json:"basis"`
}

// Mission is one mission of the Feature (X1 A, Decision #974): the stories
// it takes, and for a later one why each waits.
type Mission struct {
	Number   int            `json:"number"`
	Stories  []MissionStory `json:"stories"`
	Estimate *MissionHours  `json:"estimate,omitempty"` // in working hours, from #886's read (Story #1015)
}

// MissionStory is a story or Bug of a mission.
type MissionStory struct {
	Number       int     `json:"number"`
	Waits        string  `json:"waits,omitempty"`         // "waits on #950 (#874)"
	WorkingHours float64 `json:"working_hours,omitempty"` // once it ended: the time its thread worked, less its waits (Story #1015)
	// Run is an ended story's run and waits, kept so the clock reads GitHub only for the stories still open.
	Run *StoryRunRecord `json:"run,omitempty"`
}

// MissionStoryStart is a person's start of a Tatkal story: it runs on their tool.
type MissionStoryStart struct {
	Number    int    `json:"number"`
	Tool      string `json:"tool"`
	StartedBy string `json:"started_by"`
	StartedAt string `json:"started_at"`
}

// MissionPause is why a mission is paused.
type MissionPause struct {
	Reason string `json:"reason"`
	By     string `json:"by,omitempty"`
	At     string `json:"at,omitempty"`
}

// MissionControl is a person's control of the mission: its start, a stop, a
// resume, its end (Story #1002), or an extension of its time limit (Story
// #1015). By ran it, on Tool, and it counts as theirs.
type MissionControl struct {
	Action   string `json:"control"`
	By       string `json:"by"`
	Tool     string `json:"tool,omitempty"`
	At       string `json:"at"`
	Presence string `json:"presence"`
	Story    int    `json:"story,omitempty"` // a control of one story or Bug
	Reason   string `json:"reason,omitempty"`
	// Story #1015: a start past [mission] max_hours records its override, and an
	// extension of a mission's time limit names the mission and the hours it adds.
	Override string  `json:"override,omitempty"`
	Mission  int     `json:"mission,omitempty"`
	Hours    float64 `json:"hours,omitempty"`
}

// UnmarshalJSON reads a control of an earlier v1 plan, which said "action".
func (m *MissionControl) UnmarshalJSON(b []byte) error {
	type plain MissionControl
	var v struct {
		plain
		Old string `json:"action"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*m = MissionControl(v.plain)
	m.Action = or(m.Action, v.Old)
	return nil
}

// CostEstimate is #886's cost estimate of a Feature: the point and its range, in USD.
type CostEstimate struct {
	USD     float64 `json:"usd"`
	LowUSD  float64 `json:"low_usd"`
	HighUSD float64 `json:"high_usd"`
	Source  string  `json:"source"` // "#886"
}

// missionPlan is the mission plan Approve to build shows and writes for the
// Feature, from its stories and Bugs as they are now, the milestones the
// person named, and the approval's agents (0: [build] agents), with the lines
// the person approves, and the plan on the ledger now (nil when there is none).
func (a *approveRun) missionPlan(ctx context.Context, approved int) (MissionPlan, []string, *MissionPlan, error) {
	var p MissionPlan
	old, _, _, err := readMission(ctx, a.w, a.n)
	if err != nil {
		return p, nil, nil, err
	}
	agents, agentsSource, err := a.e.missionAgents(ctx, a.c, a.r.Repo, a.n, approved)
	if err != nil {
		return p, nil, nil, err
	}
	subs, err := featureItems(ctx, a.c, a.r.Repo, a.n, "story", "bug")
	if err != nil {
		return p, nil, nil, err
	}
	slices.SortFunc(subs, func(x, y *gh.Issue) int { return x.GetNumber() - y.GetNumber() })
	nums := make([]int, len(subs))
	for i, s := range subs {
		nums[i] = s.GetNumber()
	}
	// With no --milestones, the milestones the person named last time hold; the check below still refuses new stories.
	ms := []MissionMilestone(nil)
	if strings.TrimSpace(a.milestones) == "" && old != nil {
		ms = slices.Clone(old.Milestones)
	} else if ms, err = parseMilestones(a.milestones, nums); err != nil {
		return p, nil, nil, err
	}
	if msg := checkMilestones(a.n, ms, nums); msg != "" {
		return p, nil, nil, off("%s. Next: name each story's and Bug's milestone with aayu ready %d --milestones 'MS1=#a,#b MS2=#c'", msg, a.n)
	}
	owner, name, _ := github.SplitRepo(a.r.Repo)
	done := map[int]bool{}
	bugs := 0
	for _, si := range subs {
		labels := github.IssueLabels(si)
		kind, _ := workitems.TypeOf(si.GetType().GetName(), labels)
		if kind == "bug" {
			bugs++
		}
		it := MissionItem{Number: si.GetNumber(), Kind: kind, Title: si.GetTitle(), DependsOn: []int{}, Outside: []MissionDep{}}
		for _, l := range labels {
			if v, ok := strings.CutPrefix(l, "effort:"); ok {
				it.Effort = v
			}
		}
		for _, m := range ms {
			if slices.Contains(m.Stories, it.Number) {
				it.Milestone = m.Name
			}
		}
		done[it.Number] = si.GetState() == "closed"
		for _, d := range dependsOn(si.GetBody()) {
			if slices.Contains(nums, d) {
				it.DependsOn = append(it.DependsOn, d)
				continue
			}
			dep, err := outsideDep(ctx, a.c, owner, name, d)
			if err != nil {
				return p, nil, nil, err
			}
			it.Outside = append(it.Outside, dep.MissionDep)
			done[d] = dep.closed
		}
		p.Graph = append(p.Graph, it)
	}
	release, err := a.e.tatkalRelease(ctx, a.n)
	if err != nil {
		return p, nil, nil, Wrap(ExitExternal, err)
	}
	what := "stories"
	if bugs > 0 {
		what = "stories and Bugs"
	}
	runs := len(nums) + 2*len(ms)
	p = MissionPlan{Schema: MissionPlanSchema, Feature: a.n, Release: release, Graph: p.Graph, Milestones: ms,
		Agents: agents, AgentsSource: agentsSource,
		Estimate: MissionEstimate{Runs: runs, Stories: len(nums), Milestones: len(ms),
			Basis: fmt.Sprintf("runs ≥ %d %s + 2 × %d milestones = %d", len(nums), what, len(ms), runs)},
		Missions: splitMissions(p.Graph, func(n int) bool { return done[n] }), Controls: []MissionControl{}}
	// Each mission's estimate in working hours (Story #1015): it warns, and refuses nothing here.
	wh, err := a.e.workingHours(ctx)
	if err != nil {
		return p, nil, nil, Wrap(ExitExternal, err)
	}
	l, err := a.e.Config()
	if err != nil {
		return p, nil, nil, err
	}
	warn, maxH := missionLimits(&l.Config)
	for i := range p.Missions {
		p.Missions[i].Estimate = estimateMission(p.Graph, p.Missions[i], wh, func(n int) bool { return done[n] }, maxH)
	}
	if release != 0 { // a Tatkal Feature lives as long as its Release
		p.CompleteWhen = fmt.Sprintf("Release #%d is ready to promote and publish", release)
	} else {
		p.FeatureDone = ms[len(ms)-1].Name
		p.CompleteWhen = p.FeatureDone + " passes its gate"
	}
	parts := make([]string, len(ms))
	for i, m := range ms {
		parts[i] = m.Name + " " + issueRefs(m.Stories)
	}
	effect := []string{
		fmt.Sprintf("Mission plan: %s; every story and Bug is in one milestone; complete when %s", strings.Join(parts, " · "), p.CompleteWhen),
		missionsLine(p.Missions),
		fmt.Sprintf("Estimate: %s; up to %d stories at once (%s)", p.Estimate.Basis, p.Agents, p.AgentsSource),
	}
	effect = append(effect, estimateLines(p.Missions, warn, maxH)...)
	return p, effect, old, nil
}

// missionsLine says the missions: "Mission 1: #20, #21; #22 waits for mission 2: waits on #950 (#874)".
func missionsLine(ms []Mission) string {
	var b strings.Builder
	for i, m := range ms {
		var nums []int
		for _, s := range m.Stories {
			nums = append(nums, s.Number)
		}
		if i == 0 {
			fmt.Fprintf(&b, "Mission %d: %s", m.Number, issueRefs(nums))
			continue
		}
		for _, s := range m.Stories {
			fmt.Fprintf(&b, "; #%d waits for mission %d: %s", s.Number, m.Number, s.Waits)
		}
	}
	return b.String()
}

// splitMissions puts each story and Bug whose depends-on outside the Feature
// have merged, and whose depends-on inside it can run, in mission 1, and the
// rest in mission 2, each with why it waits (X1 A, Decision #974).
func splitMissions(graph []MissionItem, merged func(n int) bool) []Mission {
	free := map[int]bool{}
	for changed := true; changed; {
		changed = false
		for _, it := range graph {
			if free[it.Number] {
				continue
			}
			ok := !slices.ContainsFunc(it.Outside, func(d MissionDep) bool { return !merged(d.Number) }) &&
				!slices.ContainsFunc(it.DependsOn, func(n int) bool { return !free[n] && !merged(n) })
			if ok || merged(it.Number) {
				free[it.Number], changed = true, true
			}
		}
	}
	first, later := Mission{Number: 1, Stories: []MissionStory{}}, Mission{Number: 2}
	for _, it := range graph {
		if free[it.Number] {
			first.Stories = append(first.Stories, MissionStory{Number: it.Number})
			continue
		}
		var on []string
		for _, d := range it.Outside {
			if !merged(d.Number) {
				on = append(on, d.String())
			}
		}
		if len(on) == 0 {
			for _, n := range it.DependsOn {
				if !free[n] && !merged(n) {
					on = append(on, "#"+strconv.Itoa(n))
				}
			}
		}
		later.Stories = append(later.Stories, MissionStory{Number: it.Number, Waits: "waits on " + strings.Join(on, ", ")})
	}
	if len(later.Stories) == 0 {
		return []Mission{first}
	}
	return []Mission{first, later}
}

// dependsOn are the issue numbers of a body's depends-on links.
func dependsOn(body string) []int {
	_, fields := workitems.Sections(body)
	links, _ := workitems.ParseLinks(fields["Links"])
	var out []int
	for _, l := range links {
		if l.Kind == "depends-on" && !slices.Contains(out, l.Number) {
			out = append(out, l.Number)
		}
	}
	return out
}

type outside struct {
	MissionDep
	closed bool
}

// outsideDep reads a depends-on target outside the Feature: whether it has
// merged (closed), and its Feature (its parent, or itself when it is one).
func outsideDep(ctx context.Context, c *github.Client, owner, name string, n int) (outside, error) {
	d := outside{MissionDep: MissionDep{Number: n}}
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return d, Wrap(ExitExternal, err)
	}
	d.closed = is.GetState() == "closed"
	if t, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is)); t == "feature" {
		d.Feature = n
		return d, nil
	}
	parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
	if err != nil && !github.NotFound(err) {
		return d, Wrap(ExitExternal, err)
	}
	d.Feature = parent.GetNumber()
	return d, nil
}

var milestoneName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// parseMilestones reads --milestones: "MS1=#20,#21 MS2=#22,#23", in order.
// With none and no plan yet, the plan has one milestone, MS1, with every story and Bug.
func parseMilestones(spec string, all []int) ([]MissionMilestone, error) {
	if strings.TrimSpace(spec) == "" {
		return []MissionMilestone{{Name: "MS1", Stories: all, Gate: MilestoneGate{State: GateWaits}}}, nil
	}
	var ms []MissionMilestone
	for _, part := range strings.FieldsFunc(spec, func(r rune) bool { return r == ' ' || r == ';' || r == '\n' || r == '\t' }) {
		nm, refs, ok := strings.Cut(part, "=")
		if !ok || !milestoneName.MatchString(nm) || refs == "" {
			return nil, Errorf(ExitUsage, "ready --milestones: %q: want NAME=#n,#m, such as MS1=#20,#21", part)
		}
		if slices.ContainsFunc(ms, func(m MissionMilestone) bool { return m.Name == nm }) {
			return nil, Errorf(ExitUsage, "ready --milestones: %s is named twice", nm)
		}
		m := MissionMilestone{Name: nm, Gate: MilestoneGate{State: GateWaits}}
		for _, r := range strings.Split(refs, ",") {
			n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(r), "#"))
			if err != nil || n < 1 {
				return nil, Errorf(ExitUsage, "ready --milestones: %s: %q is not an issue number", nm, r)
			}
			m.Stories = append(m.Stories, n)
		}
		ms = append(ms, m)
	}
	return ms, nil
}

// checkMilestones says why the milestones do not hold the Feature's stories
// and Bugs, each in exactly one (Decision #915), or "".
func checkMilestones(f int, ms []MissionMilestone, all []int) string {
	in := map[int][]string{}
	for _, m := range ms {
		for _, n := range m.Stories {
			if !slices.Contains(all, n) {
				return fmt.Sprintf("%s names #%d, which is not a story or Bug of #%d", m.Name, n, f)
			}
			in[n] = append(in[n], m.Name)
		}
	}
	var none []int
	for _, n := range all {
		switch len(in[n]) {
		case 0:
			none = append(none, n)
		case 1:
		default:
			return fmt.Sprintf("#%d is in %s: every story and Bug belongs to one milestone", n, andList(in[n]))
		}
	}
	if len(none) == 0 {
		return ""
	}
	refs := make([]string, len(none))
	for i, n := range none {
		refs[i] = "#" + strconv.Itoa(n)
	}
	verb := "are"
	if len(none) == 1 {
		verb = "is"
	}
	return fmt.Sprintf("%s %s in no milestone: every story and Bug belongs to one", andList(refs), verb)
}

// andList joins words as "a", "a and b", or "a, b and c".
func andList(ws []string) string {
	if len(ws) < 2 {
		return strings.Join(ws, "")
	}
	return strings.Join(ws[:len(ws)-1], ", ") + " and " + ws[len(ws)-1]
}

// tatkalRelease is the Release whose Tatkal Feature f is (#877 S8), or 0.
// Until #877 S8 merges no Feature is one (tests set Options.TatkalRelease).
func (e *Env) tatkalRelease(ctx context.Context, f int) (int, error) {
	if e.TatkalRelease != nil {
		return e.TatkalRelease(ctx, f)
	}
	return 0, nil
}

// repoTool is the person's tool for repo from their user config's entry for
// it (R9). Until that entry lands it is not set (tests set Options.RepoTool).
func (e *Env) repoTool(ctx context.Context, repo string) (string, error) {
	if e.RepoTool != nil {
		return e.RepoTool(ctx, repo)
	}
	return "", nil
}

// callerTool is the tool this run is on (R9, Story #1002): the host aayu
// detects (--host, or an agent marker: doctor.DetectHost), else the person's
// tool for repo from their user config. Any person on a mission's recorded
// tool may run its next and resume.
func (e *Env) callerTool(ctx context.Context, repo string) (string, error) {
	if host, _ := doctor.DetectHost(e.Host, e.Getenv); host != "" {
		return host, nil
	}
	tool, err := e.repoTool(ctx, repo)
	return tool, Wrap(ExitExternal, err)
}

// costEstimate is #886's cost estimate of Feature f, nil until #886 merges
// (tests set Options.CostEstimate).
func (e *Env) costEstimate(ctx context.Context, f int) (*CostEstimate, error) {
	if e.CostEstimate != nil {
		return e.CostEstimate(ctx, f)
	}
	return nil, nil
}

// readMission reads Feature f's mission plan and approval from the ledger
// (nil when there is none) and the tip it read.
func readMission(ctx context.Context, w *ledger.Writer, f int) (*MissionPlan, *ready.Approval, string, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, nil, "", Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	var plan *MissionPlan
	var appr *ready.Approval
	for path, v := range map[string]any{MissionPlanPath(f): &plan, ready.ApprovalPath(f): &appr} {
		b, err := t.File(ctx, path)
		if err = ignoreMissing(err); err != nil {
			return nil, nil, "", Wrap(ExitExternal, err)
		}
		if b != nil && json.Unmarshal(b, v) != nil {
			return nil, nil, "", Errorf(ExitExternal, "ledger %s: not a record aayu wrote for #%d", path, f)
		}
	}
	if plan != nil && (plan.Schema != MissionPlanSchema || plan.Feature != f) {
		return nil, nil, "", Errorf(ExitExternal, "ledger %s: not the mission plan of #%d", MissionPlanPath(f), f)
	}
	return plan, appr, t.Tip, nil
}

// MissionStartReport is what `aayu mission start <f>` reports, or `aayu
// mission start <story>` for a story of a Tatkal Feature.
type MissionStartReport struct {
	Repo         string        `json:"repo"`
	Feature      int           `json:"feature"`
	Story        int           `json:"story,omitempty"` // a Tatkal story the person started
	Effect       []string      `json:"effect"`          // what the person saw before the device code
	Plan         *MissionPlan  `json:"plan"`
	LedgerPath   string        `json:"ledger_path"`
	LedgerCommit string        `json:"ledger_commit,omitempty"`
	Parent       *ready.Parent `json:"parent,omitempty"`     // the Kanha Start began
	Kanha        string        `json:"kanha,omitempty"`      // where it runs, in words
	Delegation   *Delegation   `json:"delegation,omitempty"` // the Tatkal story's hand-off
	DryRun       bool          `json:"dry_run,omitempty"`
}

func runMission(ctx context.Context, e *Env, in Values) error {
	f := issueNumber(in.String("feature"))
	switch action := in.String("action"); {
	case action == "tick" && f != 0:
		return Errorf(ExitUsage, "mission tick: it ticks every started mission; give no Feature")
	case action == "tick":
		return missionTick(ctx, e)
	case f == 0:
		return Errorf(ExitUsage, "mission %s: name the Feature", action)
	case action == "start":
		return missionStart(ctx, e, f, strings.TrimSpace(in.String("reason")))
	case action == "extend":
		return missionExtend(ctx, e, f, strings.TrimSpace(in.String("reason")), 0)
	case action == "next":
		return missionNext(ctx, e, f)
	case action == "stop":
		return missionStop(ctx, e, f, in)
	case action == "resume":
		return missionResume(ctx, e, f, in)
	case action == "gate":
		return missionGate(ctx, e, f, in)
	case action == "budget":
		return missionBudget(ctx, e, f, in)
	}
	return missionShow(ctx, e, f)
}

// missionStart is `aayu mission start <f>`: a person starts the Feature's
// approved mission, through GitHub's device flow. The plan records them,
// their tool for this repository, and the budget, once; then the mission's
// Kanha starts on that tool and is recorded as the approval's parent. A
// started mission with no Kanha (its start failed) starts only its Kanha, on
// the plan's tool. An agent lane is refused before anything is read. A
// mission whose learned hours pass [mission] max_hours starts only with the
// person's reason (Story #1015).
func missionStart(ctx context.Context, e *Env, f int, reason string) error {
	if err := e.laneRefusal("mission start"); err != nil {
		return err
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	if plan, _, _, err := readMission(ctx, w, f); err != nil {
		return err
	} else if plan == nil { // a story of a Tatkal Feature is started by a person, on their tool
		if tf, tp, ta, err := storyMission(ctx, c, w, repo, f); err != nil {
			return err
		} else if tp != nil {
			return storyStart(ctx, e, repo, c, w, f, tf, tp, ta)
		}
	}
	plan, appr, _, err := startable(ctx, w, f)
	if err != nil {
		return err
	}
	tool, kanhaOnly := plan.Tool, plan.StartedBy != ""
	override := ""
	for _, m := range plan.Missions {
		if h := m.Estimate; !kanhaOnly && h.over() && override == "" {
			if reason == "" {
				return Errorf(ExitBlocked, "#%d's estimate is %s working hours, %s: split this Feature, or start it with --reason", f, hrs(h.Hours), h.Mark)
			}
			override = fmt.Sprintf("mission %d's estimate, %s working hours, %s", m.Number, hrs(h.Hours), h.Mark)
		}
	}
	var budget float64
	var basis string
	var effect []string
	var locked []*gh.Issue // the issues the start locks, as the card names them
	if kanhaOnly {
		effect = []string{
			fmt.Sprintf("Starts the Kanha of #%d's mission: @%s started the mission at %s on %s, and it has no Kanha", f, plan.StartedBy, plan.StartedAt, tool),
			fmt.Sprintf("It runs in %s, the tool in %s", tool, MissionPlanPath(f)),
		}
	} else {
		if tool, err = e.repoTool(ctx, repo); err != nil {
			return Wrap(ExitExternal, err)
		}
		if tool == "" {
			return Errorf(ExitBlocked, "Start records your tool for %s from your user config, and none is set there. Nothing was started.", repo)
		}
		est, err := e.costEstimate(ctx, f)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		card := "the approved budget"
		budget, basis = appr.Budget.TotalUSD, fmt.Sprintf("the approved budget, %d x %s ([budget] per_story_usd): no cost estimate from #886 yet",
			appr.Budget.Stories, ready.USD(appr.Budget.PerStoryUSD))
		if est != nil && est.HighUSD > 0 {
			budget, card = est.HighUSD, "the top of the range"
			basis = fmt.Sprintf("the top of %s's range %s–%s (estimate %s)", est.Source, ready.USD(est.LowUSD), ready.USD(est.HighUSD), ready.USD(est.USD))
		}
		agents, _, err := e.missionAgents(ctx, c, repo, f, appr.Agents)
		if err != nil {
			return err
		}
		if locked, err = readLocked(ctx, c, repo, plan); err != nil {
			return err
		}
		effect = append([]string{fmt.Sprintf("Starts the mission of #%d, approval %d: %s", f, plan.Approval, missionsLine(plan.Missions))},
			startCard(plan, issueNumbers(locked), tool, agents)...)
		effect = append(effect,
			fmt.Sprintf("BUDGET %s · %s, locked when you start", ready.USD(budget), card),
			fmt.Sprintf("ESTIMATE %s", plan.Estimate.Basis))
		for _, m := range plan.Missions {
			if m.Estimate != nil {
				effect = append(effect, fmt.Sprintf("MISSION %d %s", m.Number, m.Estimate.Basis))
			}
		}
		if override != "" {
			effect = append(effect, fmt.Sprintf("OVERRIDE %s: you start it anyway, for the reason %q", override, reason))
		} else if reason != "" {
			effect = append(effect, fmt.Sprintf("REASON %q", reason))
		}
	}
	if why := e.kanhaCanStart(tool); why != "" {
		return Errorf(ExitBlocked, "Start begins the mission's Kanha on %s, and %s. Nothing was started.", tool, why)
	}
	effect = append(effect, fmt.Sprintf("Its Kanha starts on %s now, and starts each story on %s", tool, tool))
	p, err := e.presenceCheck(ctx, Pending{What: "mission start", Effect: effect}, AccessWrite, e.personOnly("mission start"))
	if err != nil {
		return err
	}
	// The plan or its approval can change while the code waits: start only what the person saw.
	again, appr2, tip, err := startable(ctx, w, f)
	if err != nil {
		return err
	}
	if was, now := missionJSON(plan), missionJSON(again); was != now || appr2.Version != appr.Version {
		return Errorf(ExitBlocked, "The mission plan of #%d changed while the code was waiting. Nothing was started.", f)
	}
	rep := MissionStartReport{Repo: repo, Feature: f, Effect: effect, Plan: again, LedgerPath: MissionPlanPath(f), DryRun: c.DryRun()}
	at := e.Now().UTC().Format(time.RFC3339) // the one time of this start: the plan's and its Kanha's (Bug #1099)
	if !kanhaOnly {
		// The lock holds each body and label as it is once the person confirmed, on the issues the card named.
		now, err := readLocked(ctx, c, repo, again)
		if err != nil {
			return err
		}
		if !slices.Equal(issueNumbers(now), issueNumbers(locked)) {
			return Errorf(ExitBlocked, "The issues the start of #%d locks changed while the code was waiting: now %s. Nothing was started.", f, issueRefs(issueNumbers(now)))
		}
		again.StartedBy, again.StartedAt, again.Tool, again.Budget, again.BudgetBasis = p.Login, at, tool, budget, basis
		again.Lock = newLock(p.Login, at, now)
		again.ToolSource = fmt.Sprintf("the user config of @%s for %s", p.Login, repo)
		again.Controls = append(again.Controls, MissionControl{Action: "start", By: p.Login, Tool: tool, At: at, Presence: p.Via, Override: override, Reason: reason})
		if !rep.DryRun {
			data, _ := json.MarshalIndent(again, "", "  ")
			rep.LedgerCommit, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: @%s started the mission of #%d on %s", p.Login, f, tool),
				Files: map[string][]byte{rep.LedgerPath: append(data, '\n')}, Base: tip})
			if errors.Is(err, ledger.ErrMoved) {
				return Errorf(ExitBlocked, "The ledger changed while aayu wrote the start of #%d. Nothing was started. Run it again.", f)
			}
			if err != nil {
				return Wrap(ExitExternal, err)
			}
		}
	}
	if !rep.DryRun {
		if rep.Parent, rep.Kanha, err = e.startKanha(ctx, c, w, repo, f, again, appr2, p.Login, at); err != nil {
			if k := (*kanhaUnrecorded)(nil); errors.As(err, &k) {
				return unrecordedError(f, k)
			}
			return WithNext(Errorf(ExitExternal, "The mission of #%d runs on %s, and no Kanha of it is recorded: %v", f, tool, oneLineErr(err)), fmt.Sprintf("aayu mission start %d", f))
		}
	}
	return e.Emit(rep, func(w io.Writer) error {
		if !kanhaOnly {
			fmt.Fprintf(w, "@%s started the mission of #%d on %s, with a budget of %s (%s).\n", p.Login, f, tool, ready.USD(budget), basis)
			if override != "" {
				fmt.Fprintf(w, "  override: %s; reason: %s\n", override, reason)
			}
		}
		if rep.DryRun {
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s planned, not written; no Kanha started\n", rep.LedgerPath)
			return err
		}
		if rep.LedgerCommit != "" {
			fmt.Fprintf(w, "  ledger %s (%s)\n", rep.LedgerPath, short(rep.LedgerCommit))
		}
		_, err := fmt.Fprintf(w, "  %s\n", rep.Kanha)
		return err
	})
}

// storyStart is `aayu mission start <n>` for story n of Tatkal Feature f: a
// person starts it on their own tool, through GitHub's device flow. The plan
// records the story's tool and starter, and the story is handed off on that
// tool now; f's Kanha orders and tracks it and never starts it (R9).
func storyStart(ctx context.Context, e *Env, repo string, c *github.Client, w *ledger.Writer, n, f int, plan *MissionPlan, appr *ready.Approval) error {
	switch i := slices.IndexFunc(plan.StoryStarts, func(s MissionStoryStart) bool { return s.Number == n }); {
	case plan.Release == 0:
		return Errorf(ExitBlocked, "#%d is a story of #%d, whose Kanha starts it on the tool in %s. A person starts only a story of a Tatkal Feature. Nothing was started.", n, f, MissionPlanPath(f))
	case plan.StartedBy == "":
		return WithNext(Errorf(ExitBlocked, "The Tatkal mission of #%d has not started, so #%d does not start. Nothing was started.", f, n), fmt.Sprintf("aayu mission start %d", f))
	case i >= 0:
		s := plan.StoryStarts[i]
		return Errorf(ExitBlocked, "@%s started #%d at %s on %s. Its tool does not change.", s.StartedBy, n, s.StartedAt, s.Tool)
	}
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if is.GetState() != "open" || !contains(github.IssueLabels(is), ready.LabelReady) {
		return WithNext(Errorf(ExitBlocked, "#%d is %s, not Ready. Nothing was started.", n, storyState(is)), fmt.Sprintf("aayu ready %d", n))
	}
	tool, err := e.repoTool(ctx, repo)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if tool == "" {
		return Errorf(ExitBlocked, "Start records your tool for %s from your user config, and none is set there. Nothing was started.", repo)
	}
	effect := []string{
		fmt.Sprintf("Starts #%d of Tatkal Feature #%d on %s, the tool you set for this repository", n, f, tool),
		fmt.Sprintf("The Kanha of #%d orders and tracks it, and does not start it", f),
	}
	p, err := e.presenceCheck(ctx, Pending{What: "mission start", Effect: effect}, AccessWrite, e.personOnly("mission start"))
	if err != nil {
		return err
	}
	again, appr2, tip, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if again == nil || appr2 == nil || missionJSON(plan) != missionJSON(again) || appr2.Version != appr.Version {
		return Errorf(ExitBlocked, "The mission plan of #%d changed while the code was waiting. Nothing was started.", f)
	}
	at := e.Now().UTC().Format(time.RFC3339)
	again.StoryStarts = append(again.StoryStarts, MissionStoryStart{Number: n, Tool: tool, StartedBy: p.Login, StartedAt: at})
	again.Controls = append(again.Controls, MissionControl{Action: fmt.Sprintf("start #%d", n), By: p.Login, At: at, Presence: p.Via})
	rep := MissionStartReport{Repo: repo, Feature: f, Story: n, Effect: effect, Plan: again, LedgerPath: MissionPlanPath(f), DryRun: c.DryRun()}
	var berr error
	if !rep.DryRun {
		data, _ := json.MarshalIndent(again, "", "  ")
		rep.LedgerCommit, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: @%s started #%d of #%d on %s", p.Login, n, f, tool),
			Files: map[string][]byte{rep.LedgerPath: append(data, '\n')}, Base: tip})
		if errors.Is(err, ledger.ErrMoved) {
			return Errorf(ExitBlocked, "The ledger changed while aayu wrote the start of #%d. Nothing was started. Run it again.", n)
		}
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		// The hand-off is build's, on the recorded tool (newDelegation reads it from the plan).
		sub := *e
		sub.JSON = true
		var out bytes.Buffer
		sub.Out, sub.Err = &out, io.Discard
		berr = runBuild(ctx, &sub, Values{"issue": strconv.Itoa(n), "delegate": tool})
		var res BuildResult
		if json.NewDecoder(&out).Decode(&res) == nil {
			rep.Delegation = res.Delegation
		}
	}
	if err := e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "@%s started #%d of Tatkal Feature #%d on %s; the Kanha of #%d orders and tracks it.\n", p.Login, n, f, tool, f)
		if rep.DryRun {
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s planned, not written; nothing handed off\n", rep.LedgerPath)
			return err
		}
		fmt.Fprintf(w, "  ledger %s (%s)\n", rep.LedgerPath, short(rep.LedgerCommit))
		if d := rep.Delegation; d != nil && berr == nil {
			fmt.Fprintf(w, "  handed off to %s (%s): %s\n", d.Receiver, d.Mode, orDefault(d.TaskURL, d.Task))
		}
		return nil
	}); err != nil {
		return err
	}
	if berr != nil {
		return WithNext(Errorf(ExitExternal, "#%d is started on %s, and its hand-off did not finish: %v", n, tool, oneLineErr(berr)), fmt.Sprintf("aayu build %d --delegate %s", n, tool))
	}
	return nil
}

// missionLedger is the repository, the person's own GitHub client, and their
// ledger writer.
func (e *Env) missionLedger(ctx context.Context) (string, *github.Client, *ledger.Writer, error) {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	return repo, c, w, err
}

// startable reads the plan Start may start: one that Approve to build wrote
// at the Feature's current approval, and whose mission has no Kanha yet.
func startable(ctx context.Context, w *ledger.Writer, f int) (*MissionPlan, *ready.Approval, string, error) {
	plan, appr, tip, err := readMission(ctx, w, f)
	switch {
	case err != nil:
		return nil, nil, "", err
	case plan == nil || appr == nil:
		return nil, nil, "", WithNext(Errorf(ExitBlocked, "#%d has no mission plan: a person approves it to build first. Nothing was started.", f), fmt.Sprintf("aayu ready %d", f))
	case appr.Version != plan.Approval:
		return nil, nil, "", WithNext(Errorf(ExitBlocked, "The mission plan of #%d is from approval %d, and #%d is at approval %d now. Nothing was started.", f, plan.Approval, f, appr.Version), fmt.Sprintf("aayu ready %d", f))
	case plan.StartedBy != "" && appr.Parent != nil:
		return nil, nil, "", Errorf(ExitBlocked, "The mission of #%d started at %s: @%s started it on %s. Its tool and budget do not change while it runs.", f, plan.StartedAt, plan.StartedBy, plan.Tool)
	}
	return plan, appr, tip, nil
}

// missionJSON is the plan as a person saw it: the Kanha's events cursor and
// the tick's wakes move while a mission runs, and change nothing they decide.
func missionJSON(p *MissionPlan) string {
	if p != nil {
		c := *p
		c.EventsCursor, c.Wakes = "", nil
		p = &c
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// MissionShowReport is what `aayu mission show <f>` reports. Each number
// carries its source.
type MissionShowReport struct {
	Repo       string             `json:"repo"`
	Feature    int                `json:"feature"`
	LedgerPath string             `json:"ledger_path"`
	State      string             `json:"state"` // not started, running, or paused: <reason>
	Milestones []MilestoneState   `json:"milestones"`
	Missions   []MissionState     `json:"missions"`
	Agents     SourcedNumber      `json:"agents"`
	Budget     *MissionBudgetView `json:"budget,omitempty"` // nil until Start
	Estimate   SourcedNumber      `json:"estimate"`
	Clock      []MissionClock     `json:"clock,omitempty"` // each started mission's clock (Story #1015)
	Plan       *MissionPlan       `json:"plan"`
}

// SourcedNumber is a number and where it comes from.
type SourcedNumber struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

// MissionBudgetView is the budget, what the mission used, and what is left.
type MissionBudgetView struct {
	Budget SourcedNumber `json:"budget"`
	Used   SourcedNumber `json:"used"`
	Left   SourcedNumber `json:"left"`
}

// MilestoneState is a milestone with each story's state.
type MilestoneState struct {
	Name    string            `json:"name"`
	State   string            `json:"state"`
	Stories []StoryStateEntry `json:"stories"`
}

// StoryStateEntry is a story or Bug and its state now.
type StoryStateEntry struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

// MissionState is a mission as it stands now.
type MissionState struct {
	Number  int            `json:"number"`
	State   string         `json:"state"`
	Stories []int          `json:"stories"`
	Waits   []MissionStory `json:"waits,omitempty"`
}

// missionShow is `aayu mission show <f>`: the plan as it stands. It writes
// nothing, and anyone may run it.
func missionShow(ctx context.Context, e *Env, f int) error {
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if plan == nil {
		return WithNext(Errorf(ExitBlocked, "#%d has no mission plan: a person approves it to build first.", f), fmt.Sprintf("aayu ready %d", f))
	}
	// Each story's state now, and whether each depends-on target merged.
	owner, name, _ := github.SplitRepo(repo)
	state, merged := map[int]string{}, map[int]bool{}
	for _, it := range plan.Graph {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Number)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		state[it.Number], merged[it.Number] = storyState(is), is.GetState() == "closed"
		for _, d := range it.Outside {
			dep, _, err := c.REST.Issues.Get(ctx, owner, name, d.Number)
			if err != nil {
				return Wrap(ExitExternal, err)
			}
			merged[d.Number] = dep.GetState() == "closed"
		}
	}
	approved := 0
	if appr != nil {
		approved = appr.Agents
	}
	agents, agentsSource, err := e.missionAgents(ctx, c, repo, f, approved)
	if err != nil {
		return err
	}
	rep := MissionShowReport{Repo: repo, Feature: f, LedgerPath: MissionPlanPath(f), State: "not started: a person runs aayu mission start " + strconv.Itoa(f), Plan: plan,
		Agents:   SourcedNumber{Value: float64(agents), Source: agentsSource},
		Estimate: SourcedNumber{Value: float64(plan.Estimate.Runs), Source: plan.Estimate.Basis + ", at Approve to build"}}
	switch {
	case plan.State != "":
		rep.State = plan.State
	case plan.Pause != nil:
		rep.State = "paused: " + plan.Pause.Reason
	case plan.StartedBy != "":
		rep.State = fmt.Sprintf("running: started by @%s at %s", plan.StartedBy, plan.StartedAt)
	}
	mergedN, before := 0, ""
	for _, m := range plan.Milestones {
		ms := MilestoneState{Name: m.Name}
		k := 0
		for _, n := range m.Stories {
			st := state[n]
			if s := plan.stopped(n); s != nil && !merged[n] {
				st = fmt.Sprintf("stopped (%s)", s.Reason)
			}
			ms.Stories = append(ms.Stories, StoryStateEntry{Number: n, State: st})
			if merged[n] {
				k++
			}
		}
		mergedN += k
		ms.State = fmt.Sprintf("building: %d of %d merged; the gate runs when its last story merges", k, len(m.Stories))
		switch g := m.Gate; {
		case g.State == GatePassed:
			ms.State = fmt.Sprintf("passed its gate on dev %s at %s", short(g.DevSHA), g.At)
		case g.State == GateFailed:
			ms.State = fmt.Sprintf("failed its gate on dev %s: its fix PRs run, and the gate runs again when they merge", short(g.DevSHA))
		case g.State == GateStopped:
			ms.State = fmt.Sprintf("stopped: its gate failed on dev %s, and the mission asks a person", short(g.DevSHA))
		case k == len(m.Stories):
			ms.State = "gate: its last story merged, so its gate runs"
		}
		if before != "" && m.Gate.State != GatePassed { // ST16: a milestone's PRs merge in order
			ms.State += fmt.Sprintf("; its PRs merge after %s's gate passes", before)
		}
		if before == "" && m.Gate.State != GatePassed {
			before = m.Name
		}
		rep.Milestones = append(rep.Milestones, ms)
	}
	rep.Missions = missionsNow(plan, func(n int) bool { return merged[n] })
	l, err := e.Config()
	if err != nil {
		return err
	}
	_, maxH := missionLimits(&l.Config)
	if rep.Clock, err = e.missionClocks(ctx, c, w, repo, f, plan, maxH, l.Config.Identity); err != nil {
		return err
	}
	if plan.StartedBy != "" && appr != nil {
		open, err := openFixSet(ctx, c, repo, plan)
		if err != nil {
			return err
		}
		used := missionUsed(plan, appr, mergedN, e.costs(repo), open)
		rep.Budget = &MissionBudgetView{
			Budget: SourcedNumber{Value: plan.Budget, Source: plan.BudgetBasis + ", locked at Start"},
			Used:   used,
			Left:   SourcedNumber{Value: plan.Budget - used.Value, Source: "budget less used"},
		}
	}
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "Mission of #%d · %s · approval %d by @%s\n", f, rep.LedgerPath, plan.Approval, plan.ApprovedBy)
		fmt.Fprintln(w, rep.State)
		if plan.Tool != "" {
			fmt.Fprintf(w, "tool      %s (%s: the tool of @%s, who started it)\n", plan.Tool, rep.LedgerPath, plan.StartedBy)
		}
		for _, m := range rep.Milestones {
			parts := make([]string, len(m.Stories))
			for i, s := range m.Stories {
				parts[i] = fmt.Sprintf("#%d %s", s.Number, s.State)
			}
			fmt.Fprintf(w, "%-9s %s · %s\n", m.Name, m.State, strings.Join(parts, ", "))
		}
		for _, m := range rep.Missions {
			fmt.Fprintf(w, "mission %d %s", m.Number, m.State)
			if len(m.Stories) > 0 {
				fmt.Fprintf(w, " · %s", issueRefs(m.Stories))
			}
			fmt.Fprintln(w)
			for _, s := range m.Waits {
				fmt.Fprintf(w, "          #%d %s\n", s.Number, s.Waits)
			}
		}
		fmt.Fprintf(w, "agents    %d (%s)\n", int(rep.Agents.Value), rep.Agents.Source)
		if b := rep.Budget; b != nil {
			fmt.Fprintf(w, "budget    %s (%s)\n", ready.USD(b.Budget.Value), b.Budget.Source)
			fmt.Fprintf(w, "used      %s (%s)\n", ready.USD(b.Used.Value), b.Used.Source)
			fmt.Fprintf(w, "left      %s (%s)\n", ready.USD(b.Left.Value), b.Left.Source)
		} else {
			fmt.Fprintln(w, "budget    not set: Start sets it")
		}
		fmt.Fprintf(w, "estimate  %d runs (%s)\n", plan.Estimate.Runs, rep.Estimate.Source)
		for _, m := range plan.Missions {
			if h := m.Estimate; h != nil {
				mark := ""
				if h.Mark != "" {
					mark = "; marked " + h.Mark
				}
				fmt.Fprintf(w, "          mission %d: %s%s\n", m.Number, h.Basis, mark)
			}
		}
		for _, mc := range rep.Clock {
			fmt.Fprintf(w, "clock     mission %d: %s\n", mc.Mission, mc)
		}
		_, err := fmt.Fprintf(w, "complete  when %s\n", plan.CompleteWhen)
		return err
	})
}

// missionUsed is what the mission used of its budget: each merged story or
// Bug at the per-story cap of the approval, until #872 measures spend, and
// each gate fix PR that does not wait (Decision #974: fix PRs count against
// the Feature's budget): a closed one at its measured spend, and an open one
// at least at the per-story cap, which it may still spend.
func missionUsed(plan *MissionPlan, appr *ready.Approval, merged int, costs map[int]float64, open map[int]bool) SourcedNumber {
	per := appr.Budget.PerStoryUSD
	used := SourcedNumber{Value: per * float64(merged),
		Source: fmt.Sprintf("%d merged x %s (the per-story cap of approval %d), until #872 measures spend", merged, ready.USD(per), appr.Version)}
	v, fixes := fixSpend(plan, per, costs, open)
	used.Value += v
	if len(fixes) > 0 {
		used.Source += "; gate fix PRs: " + strings.Join(fixes, ", ")
	}
	return used
}

// fixSpend is what the plan's gate fix PRs count against the budget, as
// missionUsed says, with a note for each.
func fixSpend(plan *MissionPlan, per float64, costs map[int]float64, open map[int]bool) (float64, []string) {
	total, notes := 0.0, []string(nil)
	for _, fx := range plan.GateFixes {
		switch v := costs[fx.PR]; {
		case fx.Waits != "": // a waiting fix spends nothing until a person lets it go on
		case open[fx.PR]:
			v = max(v, per)
			total += v
			notes = append(notes, fmt.Sprintf("#%d %s (open: at least the per-story cap)", fx.PR, ready.USD(v)))
		case v > 0:
			total += v
			notes = append(notes, fmt.Sprintf("#%d %s measured", fx.PR, ready.USD(v)))
		}
	}
	return total, notes
}

// openFixSet are the plan's fix PRs that are still open, by number.
func openFixSet(ctx context.Context, c *github.Client, repo string, plan *MissionPlan) (map[int]bool, error) {
	owner, name, _ := github.SplitRepo(repo)
	fixes, err := openFixPRs(ctx, c, owner, name, plan)
	out := map[int]bool{}
	for _, fx := range fixes {
		out[fx.PR] = true
	}
	return out, err
}

// missionsNow is each mission as it stands: mission 1 as approved, and the
// next one started, under the plan's approval and on its tool, once a waiting
// story's depends-on outside the Feature have merged; the rest wait for the
// one after (X1 A, Decision #974).
func missionsNow(plan *MissionPlan, merged func(int) bool) []MissionState {
	started := plan.StartedBy != "" && plan.Pause == nil && plan.State == ""
	// A started mission that is paused is held; the one paused row is the header's.
	held := map[bool]string{true: "running", false: "not started"}[started]
	switch {
	case plan.State != "":
		held = plan.State
	case plan.StartedBy != "" && plan.Pause != nil:
		held = "held until the mission resumes"
	}
	var out []MissionState
	for _, m := range plan.Missions {
		ms := MissionState{Number: m.Number, Stories: []int{}}
		if m.Number == 1 {
			for _, s := range m.Stories {
				ms.Stories = append(ms.Stories, s.Number)
			}
			ms.State = held
			out = append(out, ms)
			continue
		}
		var waiting []MissionItem
		for _, s := range m.Stories {
			for _, it := range plan.Graph {
				if it.Number == s.Number {
					waiting = append(waiting, it)
				}
			}
		}
		// The stories of earlier missions can run; only the waiting ones are split again.
		earlier := func(n int) bool {
			return merged(n) || slices.ContainsFunc(plan.Graph, func(it MissionItem) bool { return it.Number == n }) &&
				!slices.ContainsFunc(waiting, func(it MissionItem) bool { return it.Number == n })
		}
		split := splitMissions(waiting, earlier)
		for _, s := range split[0].Stories {
			ms.Stories = append(ms.Stories, s.Number)
		}
		switch {
		case len(ms.Stories) == 0:
			ms.State, ms.Waits = "waits", split[1].Stories
		case started:
			ms.State = fmt.Sprintf("running: started under approval %d on %s, with no new Approve to build", plan.Approval, plan.Tool)
		case plan.StartedBy != "":
			ms.State = held
		default:
			ms.State = "can start once the mission starts"
		}
		out = append(out, ms)
		if len(split) > 1 && len(ms.Stories) > 0 {
			out = append(out, MissionState{Number: m.Number + 1, State: "waits", Stories: []int{}, Waits: split[1].Stories})
		}
	}
	return out
}

// storyState is a story's or Bug's state: merged once closed, else its status label.
func storyState(is *gh.Issue) string {
	if is.GetState() == "closed" {
		return "merged"
	}
	for _, l := range github.IssueLabels(is) {
		if s, ok := strings.CutPrefix(l, "status:"); ok {
			return s
		}
	}
	return "open"
}
