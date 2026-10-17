// Package journey is the home screen and `aayu next` (WP1.1b): where a
// repository is on the journey from a grilled idea to a measured release, and
// the single best next command. Gathering the facts is the CLI's job; this
// package only decides and renders, so it is pure and golden-tested.
package journey

import (
	"fmt"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Item is a work item or pull request the home screen lists.
type Item struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	Story  int    `json:"story,omitempty"` // a pull request's story
	Gate   string `json:"gate,omitempty"`  // a pull request's merge gate: passed, pending, failed, none
}

// Floor is a floor that missed in a local proof pack.
type Floor struct {
	Story  int    `json:"story"`
	Floor  string `json:"floor"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

// Due is a measurement that came due (measure/due on the ledger).
type Due struct {
	Issue int    `json:"issue"`
	Type  string `json:"type"`
	Due   string `json:"due"`
	Tag   string `json:"tag"`
}

// GitHub reachability values.
const (
	GitHubOK      = "ok"
	GitHubNoToken = "no-token"
	GitHubNoRepo  = "no-repo"
)

// State is everything the home screen knows about a repository.
type State struct {
	Repo          string   `json:"repo,omitempty"`
	InRepo        bool     `json:"in_repo"`
	Configured    bool     `json:"configured"`     // .aayudha/config.toml exists (Level 1)
	Gates         bool     `json:"gates"`          // the merge gate workflow is installed (Level 2)
	Hosts         []string `json:"hosts"`          // hosts in .aayudha/lock.toml
	Host          string   `json:"host,omitempty"` // the host aayu runs in, if detected
	OutOfSync     []string `json:"out_of_sync,omitempty"`
	Paused        bool     `json:"paused"`
	GitHub        string   `json:"github"` // GitHubOK, GitHubNoToken, GitHubNoRepo, or the error
	Ready         []Item   `json:"ready"`
	InProgress    []Item   `json:"in_progress"`
	Blocked       []Item   `json:"blocked"`
	AwaitingHuman []Item   `json:"awaiting_human"` // story PRs without a human pass
	FailingFloors []Floor  `json:"failing_floors"`
	MeasureDue    []Due    `json:"measure_due"`
	Unclassified  []Item   `json:"unclassified_feedback"` // open Feedback with no fix:<layer> label (D-7.7e)
}

// Level is the adoption level the repository has reached (docs/guide/adopt.md).
func (s State) Level() int {
	switch {
	case !s.Configured:
		return 0
	case !s.Gates:
		return 1
	}
	return 2
}

// Step is one suggested command.
type Step struct {
	Command   string        `json:"command"`
	Why       string        `json:"why"`
	Seat      verbs.SeatID  `json:"seat"`
	Stage     verbs.StageID `json:"stage"`
	HumanOnly bool          `json:"human_only"` // a person must run it; agents are refused
}

// Candidates lists every useful next step, best first.
func Candidates(s State) []Step {
	var out []Step
	add := func(st Step) { out = append(out, st) }
	host := s.Host
	if host == "" {
		host = "claude"
	}
	switch {
	case !s.InRepo:
		return []Step{{Command: "aayu tour", Why: "aayu works inside a git checkout of your project: cd into one (or run git init), then the tour sets it up in a few prompts.", Seat: verbs.Founder, Stage: verbs.StageSetup, HumanOnly: true}}
	case s.Paused:
		add(Step{Command: "aayu resume", Why: "the [budget] breaker paused every agent lane; a person checks the spend and clears it.", Seat: verbs.Founder, Stage: verbs.StageSetup, HumanOnly: true})
	}
	if !s.Configured {
		add(Step{Command: "aayu tour", Why: "this repository is not set up yet: the tour reaches Level 1 (labels, issue forms, config, the records branch, and your host's skills) in a few prompts.", Seat: verbs.Founder, Stage: verbs.StageSetup, HumanOnly: true})
	}
	if s.GitHub == GitHubNoToken {
		add(Step{Command: "gh auth login", Why: "aayu reads and writes GitHub with your own sign-in, and none was found.", Seat: verbs.DevOps, Stage: verbs.StageSetup, HumanOnly: true})
	}
	if s.Configured && len(s.Hosts) == 0 {
		add(Step{Command: "aayu install --host " + host + " --apply", Why: "no host has the aayudha skills, MCP server, and hooks yet.", Seat: verbs.DevOps, Stage: verbs.StageSetup, HumanOnly: true})
	}
	if len(s.OutOfSync) > 0 {
		add(Step{Command: "aayu doctor --fix", Why: "the installed skill copies differ from .aayudha/lock.toml (" + s.OutOfSync[0] + ").", Seat: verbs.DevOps, Stage: verbs.StageSetup, HumanOnly: true})
	}
	for _, pr := range s.AwaitingHuman {
		if pr.Gate == "passed" || pr.Gate == "none" {
			why := fmt.Sprintf("PR #%d (story #%d) passed its merge gate and waits for a person: review and approve it on GitHub, then ship it.", pr.Number, pr.Story)
			if pr.Gate == "none" {
				why = fmt.Sprintf("PR #%d (story #%d) waits for a person: review and approve it on GitHub, then ship it.", pr.Number, pr.Story)
			}
			add(Step{Command: fmt.Sprintf("aayu ship %d", pr.Number), Why: why, Seat: verbs.EngineeringManager, Stage: verbs.StageShip, HumanOnly: true})
			break
		}
	}
	for _, f := range s.FailingFloors {
		add(Step{Command: fmt.Sprintf("aayu build %d", f.Story), Why: fmt.Sprintf("#%d missed the %s floor (%s): fix it in the build worktree, then build the next checkpoint.", f.Story, f.Floor, firstOr(f.Detail, f.Result)), Seat: verbs.AINativeEngineer, Stage: verbs.StageProve})
		break
	}
	for _, it := range s.Blocked {
		add(Step{Command: fmt.Sprintf("aayu build %d", it.Number), Why: fmt.Sprintf("#%d %q is blocked: read its Build progress comment, fix the cause, then build again.", it.Number, it.Title), Seat: verbs.AINativeEngineer, Stage: verbs.StageBuild})
		break
	}
	for _, it := range s.InProgress {
		add(Step{Command: fmt.Sprintf("aayu build %d", it.Number), Why: fmt.Sprintf("#%d %q is being built: commit the next checkpoint.", it.Number, it.Title), Seat: verbs.AINativeEngineer, Stage: verbs.StageBuild})
		break
	}
	for _, it := range s.Ready {
		add(Step{Command: fmt.Sprintf("aayu build %d", it.Number), Why: fmt.Sprintf("#%d %q is Ready and nobody is building it yet.", it.Number, it.Title), Seat: verbs.AINativeEngineer, Stage: verbs.StageReady})
		break
	}
	for _, d := range s.MeasureDue {
		add(Step{Command: fmt.Sprintf("aayu measure %d", d.Issue), Why: fmt.Sprintf("%s #%d shipped in %s and its measurement was due on %s.", d.Type, d.Issue, d.Tag, d.Due), Seat: verbs.DataAIArchitect, Stage: verbs.StageLearn})
		break
	}
	if s.Configured {
		add(Step{Command: "aayu grill story", Why: "nothing is waiting: shape the next story, with numbered criteria that say what done means.", Seat: verbs.ProductStrategist, Stage: verbs.StageShape})
	}
	return out
}

// Next is the single best step. An agent gets the best step it may take
// itself; when only a person can act, the step says so (HumanOnly).
func Next(s State, agent bool) Step {
	c := Candidates(s)
	if agent && !s.Paused { // a paused agent lane may do nothing until a person resumes it
		for _, st := range c {
			if !st.HumanOnly {
				return st
			}
		}
	}
	if len(c) == 0 {
		return Step{Command: "aayu status", Why: "everything is quiet; roll up your Goals.", Seat: verbs.EngineeringManager, Stage: verbs.StageExplain}
	}
	return c[0]
}

func firstOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// path is the journey the home screen draws, with short labels.
var path = []struct {
	ID    verbs.StageID
	Label string
}{
	{verbs.StageSetup, "Set up"}, {verbs.StageShape, "Shape"}, {verbs.StageReady, "Ready"}, {verbs.StageBuild, "Build"},
	{verbs.StageProve, "Prove"}, {verbs.StageShip, "Ship"}, {verbs.StageRelease, "Release"}, {verbs.StageLearn, "Learn"},
}

// Where is the stage the repository is at: the stage of its best next step.
func Where(next Step) verbs.StageID {
	for _, p := range path {
		if p.ID == next.Stage {
			return p.ID
		}
	}
	return verbs.StageShape
}

// Render writes the home screen. Plain and Machine levels get the same words,
// so a screen reader and a script read the same thing.
func Render(p *ui.Printer, s State, next Step) {
	title := "aayudha"
	if s.Repo != "" {
		title += p.Sym(" · ", ": ") + s.Repo
	}
	p.Println(p.Style(ui.Title, title))
	if !s.InRepo {
		p.Println("You are not in a git repository.")
		p.Println("")
		renderNext(p, next)
		p.Println(p.Style(ui.Muted, "Every command: aayu --help"))
		return
	}
	level := map[int]string{0: "not set up yet", 1: "Level 1: clear work items", 2: "Level 2: proof and the merge gate"}[s.Level()]
	p.Println(p.Style(ui.Muted, level))
	p.Println("")
	p.Println(journeyLine(p, Where(next)))
	p.Println("")

	rows := [][]string{
		{"Ready to build", count(len(s.Ready)), items(s.Ready, "#")},
		{"In progress", count(len(s.InProgress)), items(s.InProgress, "#")},
		{"Waiting for a person", count(len(s.AwaitingHuman)), prs(s.AwaitingHuman)},
		{"Blocked", count(len(s.Blocked)), items(s.Blocked, "#")},
		{"Failing floors", count(len(s.FailingFloors)), floors(s.FailingFloors)},
		{"Measurements due", count(len(s.MeasureDue)), dues(s.MeasureDue)},
	}
	if p.Level == ui.Rich {
		for _, r := range rows {
			n := p.Style(ui.Muted, r[1])
			if r[1] != "0" {
				n = p.Style(ui.Accent, r[1])
			}
			p.Println(strings.TrimRight(fmt.Sprintf("  %-22s %s  %s", r[0], n, r[2]), " "))
		}
	} else {
		for _, r := range rows {
			line := fmt.Sprintf("%s: %s", r[0], r[1])
			if r[2] != "" {
				line += " (" + r[2] + ")"
			}
			p.Println(line)
		}
	}
	if note := UnclassifiedNote(s.Unclassified); note != "" {
		p.Println(p.Mark(ui.StatusWarn) + " " + note)
	}
	switch {
	case s.GitHub == GitHubNoToken:
		p.Println(p.Mark(ui.StatusWarn) + " GitHub: not signed in, so issues and pull requests are not shown.")
	case s.GitHub != GitHubOK && s.GitHub != "":
		p.Println(p.Mark(ui.StatusWarn) + " GitHub: " + s.GitHub + "; showing what this checkout knows.")
	}
	p.Println("")
	renderNext(p, next)
}

func renderNext(p *ui.Printer, next Step) {
	who := verbs.SeatName(next.Seat)
	if next.HumanOnly {
		who += ", for a person to run"
	}
	if p.Level == ui.Rich {
		p.Printf("%s %s\n", p.Style(ui.Title, "Next →"), p.Style(ui.Command, next.Command))
		p.Printf("  %s %s\n", next.Why, p.Style(ui.Muted, "("+who+")"))
		return
	}
	p.Println("Next: " + next.Command)
	p.Println("  Why: " + next.Why + " (" + who + ")")
}

// RenderNext writes `aayu next`: the command on its own first line (so a
// script can take it), then why. agent marks a step only a person can take.
func RenderNext(p *ui.Printer, next Step, agent bool) {
	cmd := next.Command
	if p.Level == ui.Rich {
		cmd = p.Style(ui.Title, "→ ") + p.Style(ui.Command, cmd)
	}
	if agent && next.HumanOnly {
		p.Println("waiting on a person: " + next.Command)
	} else {
		p.Println(cmd)
	}
	who := verbs.SeatName(next.Seat)
	if next.HumanOnly {
		who += ", for a person to run"
	}
	p.Println("  " + next.Why + " " + p.Style(ui.Muted, "("+who+")"))
}

func journeyLine(p *ui.Printer, at verbs.StageID) string {
	var parts []string
	reached := true
	for _, st := range path {
		label := st.Label
		switch {
		case st.ID == at:
			reached = false
			if p.Level == ui.Rich {
				label = p.Style(ui.Title, "● "+label)
			} else {
				label += " (you are here)"
			}
		case reached:
			if p.Level == ui.Rich {
				label = p.Style(ui.OK, "✓ "+label)
			} else {
				label += " (done)"
			}
		default:
			label = p.Style(ui.Muted, label)
		}
		parts = append(parts, label)
	}
	sep := " > "
	if p.Level == ui.Rich {
		sep = p.Style(ui.Muted, " → ")
	}
	lead := "Journey: "
	if p.Level == ui.Rich {
		lead = p.Style(ui.Muted, "Journey  ")
	}
	return lead + strings.Join(parts, sep)
}

// UnclassifiedNote lists the open Feedback issues that have no fix layer with
// the command that records one (D-7.7e); "" when there are none.
func UnclassifiedNote(xs []Item) string {
	if len(xs) == 0 {
		return ""
	}
	return fmt.Sprintf("%d open Feedback issue(s) without a fix layer (%s); classify each by the highest layer that can fix it for good: `%s`",
		len(xs), items(xs, "#"), workitems.ClassifyCommand)
}

func count(n int) string { return fmt.Sprint(n) }

func items(xs []Item, prefix string) string {
	var out []string
	for i, it := range xs {
		if i == 3 {
			out = append(out, fmt.Sprintf("and %d more", len(xs)-3))
			break
		}
		out = append(out, fmt.Sprintf("%s%d %s", prefix, it.Number, it.Title))
	}
	return strings.Join(out, "; ")
}

func prs(xs []Item) string {
	var out []string
	for i, it := range xs {
		if i == 3 {
			out = append(out, fmt.Sprintf("and %d more", len(xs)-3))
			break
		}
		out = append(out, fmt.Sprintf("PR #%d for #%d, gate %s", it.Number, it.Story, it.Gate))
	}
	return strings.Join(out, "; ")
}

func floors(xs []Floor) string {
	var out []string
	for i, f := range xs {
		if i == 3 {
			out = append(out, fmt.Sprintf("and %d more", len(xs)-3))
			break
		}
		out = append(out, fmt.Sprintf("#%d %s %s", f.Story, f.Floor, f.Result))
	}
	return strings.Join(out, "; ")
}

func dues(xs []Due) string {
	var out []string
	for _, d := range xs {
		out = append(out, fmt.Sprintf("%s #%d since %s", d.Type, d.Issue, d.Due))
	}
	return strings.Join(out, "; ")
}
