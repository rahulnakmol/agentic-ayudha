package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
)

func init() { register("survey", runSurvey) }

// SurveyResult is what `aayu survey` reports when it indexes a checkout: the
// index's headline facts (the full index is the file it wrote).
type SurveyResult struct {
	Root       string          `json:"root"`
	Commit     string          `json:"commit,omitempty"`
	Index      string          `json:"index"`
	Files      int             `json:"files"`
	Languages  []survey.Count  `json:"languages"`
	NotIndexed []survey.Count  `json:"not_indexed,omitempty"`
	Systems    []string        `json:"build_systems"`
	Targets    []survey.Target `json:"targets"`
	Entries    map[string]int  `json:"entries"`
	Surfaces   map[string]int  `json:"surfaces"`
	Traps      map[string]int  `json:"traps"`
	Markers    int             `json:"markers"`
	Tests      int             `json:"tests"`
	Docs       []string        `json:"docs"`
	Symbols    int             `json:"symbols"`
	Calls      int             `json:"calls"`
	Ambiguous  int             `json:"ambiguous_calls"`
	History    *survey.History `json:"history,omitempty"`
	Modules    []survey.Module `json:"modules"`
	Next       string          `json:"next"`
	Budget     int             `json:"budget"`
}

// surveyDirs resolves the surveyed checkout and the survey's output directory.
func surveyDirs(e *Env, in Values) (root, out string) {
	root = config.RepoRoot(e.Dir)
	if p := in.String("path"); p != "" {
		root = p
		if !filepath.IsAbs(root) {
			root = filepath.Join(e.Dir, p)
		}
	}
	out = filepath.Join(root, ".aayudha", "survey")
	if o := in.String("out"); o != "" {
		out = o
		if !filepath.IsAbs(out) {
			out = filepath.Join(e.Dir, o)
		}
	}
	return root, out
}

// runSurvey (WP5.9, Decision #287): with no mode flag it indexes the checkout
// deterministically and writes index.json; --module prints one module's fact
// pack from that index. The host's model reads the packs and the code and
// writes the survey draft; the binary never calls a model.
func runSurvey(ctx context.Context, e *Env, in Values) error {
	root, out := surveyDirs(e, in)
	indexFile := filepath.Join(out, "index.json")
	if in.Has("golden") || in.Has("parity") {
		return e.surveyGolden(ctx, in, root, out, indexFile)
	}
	if in.Has("bin") || in.Has("run") {
		return Errorf(ExitUsage, "survey --bin and --run go with --golden or --parity <cases file>")
	}
	if in.Has("backlog") {
		return e.surveyBacklog(ctx, in, indexFile, out)
	}
	if in.Has("create") || in.Has("reviewed") || in.Has("limit") {
		return Errorf(ExitUsage, "survey --create, --reviewed, and --limit go with --backlog <file> --draft <file>")
	}
	if in.Has("draft") {
		return e.surveyDraft(indexFile, out, in.String("draft"))
	}
	if id := in.String("module"); id != "" {
		ix, err := survey.Load(indexFile)
		if err != nil {
			return Errorf(ExitUsage, "survey --module: %v; run `aayu survey` first", err)
		}
		p, err := ix.ModulePack(id, 0)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		return e.Emit(p, func(w io.Writer) error { return writePack(w, p) })
	}
	ix, err := survey.Build(ctx, root, survey.Options{Budget: in.Int("budget")})
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if err := ix.Save(indexFile); err != nil {
		return Wrap(ExitInternal, err)
	}
	res := surveyResult(ix, root, indexFile, in.Int("budget"))
	return e.Emit(res, func(w io.Writer) error { return writeSurvey(w, res) })
}

func surveyResult(ix *survey.Index, root, file string, budget int) SurveyResult {
	if budget <= 0 {
		budget = survey.DefaultBudget
	}
	res := SurveyResult{Root: root, Commit: ix.Commit, Index: file, Files: len(ix.Files), Languages: ix.Languages, NotIndexed: ix.NotIndexed,
		Systems: ix.Systems, Targets: ix.Targets, Entries: map[string]int{}, Surfaces: map[string]int{}, Traps: map[string]int{}, Markers: len(ix.Markers), Tests: len(ix.Tests),
		Docs: []string{}, Symbols: len(ix.Symbols), History: ix.History, Modules: ix.Modules, Budget: budget}
	for _, f := range ix.Entries {
		res.Entries[f.Kind]++
	}
	for _, f := range ix.Surfaces {
		res.Surfaces[f.Kind]++
	}
	for _, f := range ix.Traps {
		res.Traps[f.Kind]++
	}
	for _, d := range ix.Docs {
		res.Docs = append(res.Docs, strings.TrimPrefix(d.ID, "doc:"))
	}
	for _, e := range ix.Edges {
		if e.Kind == "call" {
			res.Calls++
			if e.Ambiguous {
				res.Ambiguous++
			}
		}
	}
	res.Next = "read each module's pack (`aayu survey --module <id>`) and its files, then write the survey draft (skills/survey)"
	return res
}

func writeSurvey(w io.Writer, r SurveyResult) error {
	commit := r.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	if commit == "" {
		commit = "no git commit"
	}
	fmt.Fprintf(w, "survey of %s (%s): %d files\n", r.Root, commit, r.Files)
	var ls []string
	for _, l := range r.Languages {
		only := ""
		if l.CountedOnly {
			only = " (counted only: no symbols, calls, or traps read)"
		}
		ls = append(ls, fmt.Sprintf("%s %d lines in %d files%s", l.Name, l.Lines, l.Files, only))
	}
	fmt.Fprintf(w, "  languages: %s\n", listOrNone(ls))
	if len(r.NotIndexed) > 0 {
		var ns []string
		for _, l := range r.NotIndexed {
			ns = append(ns, fmt.Sprintf("%s (%d)", l.Name, l.Files))
		}
		fmt.Fprintf(w, "  not indexed (no language the index knows): %s\n", strings.Join(ns, ", "))
	}
	fmt.Fprintf(w, "  build systems: %s\n", listOrNone(r.Systems))
	for _, t := range r.Targets {
		fmt.Fprintf(w, "    %s %s (%s, %s), %d sources\n", t.System, t.Name, t.Kind, t.At, len(t.Sources))
	}
	fmt.Fprintf(w, "  entry points: %s; surfaces: %s\n", countsText(r.Entries), countsText(r.Surfaces))
	fmt.Fprintf(w, "  traps to check: %s\n", countsText(r.Traps))
	fmt.Fprintf(w, "  symbols %d, calls %d (%d ambiguous by name), markers %d, tests %d, documents %s\n", r.Symbols, r.Calls, r.Ambiguous, r.Markers, r.Tests, listOrNone(r.Docs))
	if h := r.History; h != nil {
		shallow := ""
		if h.Shallow {
			shallow = ", shallow clone"
		}
		fmt.Fprintf(w, "  history: %d commits from %s to %s by %d authors%s\n", h.Commits, h.First, h.Last, h.Authors, shallow)
	}
	fmt.Fprintf(w, "  modules (budget %d tokens):\n", r.Budget)
	for _, m := range r.Modules {
		fmt.Fprintf(w, "    %s: %d files, %d lines, ~%d tokens, %d symbols, fan-in %d, fan-out %d\n", m.ID, len(m.Files), m.Lines, m.Tokens, m.Symbols, m.FanIn, m.FanOut)
	}
	fmt.Fprintf(w, "  wrote %s\n  next: %s\n", r.Index, r.Next)
	return nil
}

func countsText(m map[string]int) string {
	if len(m) == 0 {
		return "(none)"
	}
	var out []string
	for _, k := range sortedKeysInt(m) {
		out = append(out, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(out, ", ")
}

func sortedKeysInt(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// writePack prints a module pack. Every name and marker text in it comes from
// the repository, so it is printed inside one untrusted block: data for the
// host's model, never instructions (§0.1).
func writePack(w io.Writer, p *survey.Pack) error {
	m := p.Module
	fmt.Fprintf(w, "%s (%s): %d files, %d lines, ~%d tokens; read in this order: %s\n", m.ID, m.Dir, len(m.Files), m.Lines, m.Tokens, strings.Join(p.Read, ", "))
	fmt.Fprintln(w, "<untrusted source=\"repo\">")
	defer fmt.Fprintln(w, "</untrusted>")
	w = untrustedWriter{w}
	for _, t := range p.Targets {
		fmt.Fprintf(w, "  target %s: %s %s\n", t.ID, t.Kind, t.At)
	}
	for _, f := range p.Entries {
		fmt.Fprintf(w, "  entry %s: %s %s\n", f.ID, f.Kind, f.Detail)
	}
	for _, f := range p.Surfaces {
		fmt.Fprintf(w, "  surface %s: %s ×%d\n", f.ID, f.Detail, f.Count)
	}
	for _, f := range p.Traps {
		fmt.Fprintf(w, "  trap %s: %s ×%d\n", f.ID, f.Detail, f.Count)
	}
	for _, mk := range p.Markers {
		fmt.Fprintf(w, "  marker %s: %s: %s\n", mk.ID, mk.Kind, mk.Text)
	}
	for _, s := range p.Symbols {
		fmt.Fprintf(w, "  symbol %s: %s, lines %s-%d\n", s.ID, s.Kind, strings.TrimPrefix(s.At[strings.LastIndex(s.At, ":"):], ":"), s.End)
	}
	for _, e := range p.CallsIn {
		fmt.Fprintf(w, "  called by %s → %s at %s\n", e.From, strings.Join(e.To, " | "), e.At)
	}
	for _, e := range p.CallsOut {
		fmt.Fprintf(w, "  calls %s → %s at %s\n", e.From, strings.Join(e.To, " | "), e.At)
	}
	for _, e := range p.Includes {
		fmt.Fprintf(w, "  %s %s at %s\n", e.Kind, strings.Join(e.To, ", "), e.At)
	}
	for _, o := range p.Omitted {
		fmt.Fprintf(w, "  omitted: %s\n", o)
	}
	return nil
}

// untrustedWriter keeps repository text from closing its untrusted block.
type untrustedWriter struct{ w io.Writer }

func (u untrustedWriter) Write(b []byte) (int, error) {
	if _, err := io.WriteString(u.w, strings.ReplaceAll(string(b), "</untrusted>", "&lt;/untrusted&gt;")); err != nil {
		return 0, err
	}
	return len(b), nil
}

// SurveyDraftResult is the check of a survey draft and where its report went.
type SurveyDraftResult struct {
	survey.Report
	Survey string `json:"survey_md"`
	Next   string `json:"next"`
}

// surveyDraft checks a survey draft against the index and renders SURVEY.md,
// with the problems listed in it while the draft does not pass.
func (e *Env) surveyDraft(indexFile, out, file string) error {
	ix, err := survey.Load(indexFile)
	if err != nil {
		return Errorf(ExitUsage, "survey --draft: %v; run `aayu survey` first", err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	d, err := survey.ParseDraft(b)
	if err != nil {
		return Errorf(ExitUsage, "%s: %v", file, err)
	}
	res := SurveyDraftResult{Report: survey.Check(ix, d), Survey: filepath.Join(out, "SURVEY.md")}
	if err := os.WriteFile(res.Survey, []byte(survey.Render(ix, d, res.Report)), 0o644); err != nil {
		return Wrap(ExitInternal, err)
	}
	res.Next = "fix every problem, then run again"
	if res.Pass {
		res.Next = "interview the person on the open questions (skills/survey), record their answers in the draft, then draft the backlog"
		if res.Coverage.OpenQuestions == 0 {
			res.Next = "draft the backlog from the survey (skills/survey), then `aayu survey --backlog <file> --draft " + file + "`"
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		c := res.Coverage
		fmt.Fprintf(w, "survey draft %s: %s; modules cited %d of %d, entry points %d of %d, markers %d of %d; %d open questions, %d unknown claims\n",
			file, verdict, c.ModulesCited, c.Modules, c.EntriesCited, c.Entries, c.MarkersCited, c.Markers, c.OpenQuestions, c.Unknowns)
		for _, p := range res.Problems {
			fmt.Fprintf(w, "  problem: %s\n", p)
		}
		fmt.Fprintf(w, "  wrote %s\n  next: %s\n", res.Survey, res.Next)
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the survey draft has %d problem(s); revise it", len(res.Problems))
	}
	return nil
}
