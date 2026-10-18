package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

// under reports whether p lies below dir, and its path relative to dir: a
// first component of .. leaves it, ..cache does not.
func under(dir, p string) (string, bool) {
	rel, err := filepath.Rel(absPath(dir), absPath(p))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// surveyOutSafe refuses an output directory a symbolic link could redirect:
// the default one sits in the surveyed checkout, which the survey does not
// trust, so a link committed at .aayudha/survey/index.json or under
// manuals/ would make a write land on any file the user can write. Every
// directory from the checkout down to out, and everything already under out,
// must be a real file or directory.
func surveyOutSafe(root, out string) error {
	refuse := func(p string) error {
		return fmt.Errorf("survey: %s is a symbolic link; the survey writes only real files under its output directory %s (remove the link, or give --out a real directory)", p, out)
	}
	if rel, ok := under(root, out); ok {
		p := absPath(root)
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			p = filepath.Join(p, part)
			if fi, err := os.Lstat(p); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
				return refuse(p)
			}
		}
	}
	if fi, err := os.Lstat(out); err != nil || !fi.IsDir() {
		if err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			return refuse(out)
		}
		return nil
	}
	return filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type()&fs.ModeSymlink != 0 {
			return refuse(p)
		}
		return err
	})
}

// runSurvey (WP5.9, Decision #287): with no mode flag it indexes the checkout
// deterministically and writes index.json; --module prints one module's fact
// pack from that index. The host's model reads the packs and the code and
// writes the survey draft; the binary never calls a model.
func runSurvey(ctx context.Context, e *Env, in Values) error {
	root, out := surveyDirs(e, in)
	if err := surveyOutSafe(root, out); err != nil {
		return Wrap(ExitUsage, err)
	}
	indexFile := filepath.Join(out, "index.json")
	if in.Has("golden") || in.Has("parity") || in.Has("compat") {
		return e.surveyGolden(ctx, in, root, out, indexFile)
	}
	if in.Has("bin") || in.Has("run") || in.Has("shadow") || in.Has("probe") || in.Has("check-bin") || in.Has("coverage") {
		return Errorf(ExitUsage, "survey --bin, --run, --shadow, --probe, --check-bin, and --coverage go with --golden, --parity, or --compat <cases file>")
	}
	if in.Has("backlog") {
		return e.surveyBacklog(ctx, in, indexFile, out)
	}
	if in.Has("create") || in.Has("reviewed") || in.Has("limit") {
		return Errorf(ExitUsage, "survey --create, --reviewed, and --limit go with --backlog <file> --draft <file>")
	}
	if in.Has("draft") || in.Bool("sections") {
		return e.surveyDraft(indexFile, out, in.String("draft"))
	}
	if in.Has("docs") && in.Has("module") {
		return Errorf(ExitUsage, "survey --docs goes with indexing (no mode flag): it adds manuals and notes to the index")
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
	manuals, err := surveyManuals(ctx, in.String("docs"), filepath.Join(out, "manuals"))
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	opt := survey.Options{Budget: in.Int("budget"), Manuals: manuals}
	if rel, ok := under(root, out); ok {
		opt.Out = filepath.ToSlash(rel)
	}
	ix, err := survey.Build(ctx, root, opt)
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

// SurveyDraftResult is the check of a survey draft, whole or in sections,
// and where its report went.
type SurveyDraftResult struct {
	survey.SectionReport
	// Saved lists the sections this run wrote under Dir, each with its
	// findings, before the next was checked; Resumed the saved sections it
	// merged without rewriting; Stale the saved sections against another
	// index (another commit or other content), which must be written again.
	Dir     string   `json:"sections_dir"`
	Saved   []string `json:"saved,omitempty"`
	Resumed []string `json:"resumed,omitempty"`
	Stale   []string `json:"stale,omitempty"`
	Merged  string   `json:"survey_json,omitempty"`
	Survey  string   `json:"survey_md,omitempty"`
	Next    string   `json:"next"`
}

// surveyDraft checks a survey draft, whole or some of its sections, against
// the index. Each section in the file is saved with its findings under
// <out>/sections as it is checked; sections saved earlier are resumed, so a
// survey can be written and fixed one section at a time and an error never
// loses the sections before it. With every section present it writes the
// merged survey.json and renders SURVEY.md, as for a whole draft. With no
// file (--sections) it reports what is saved.
func (e *Env) surveyDraft(indexFile, out, file string) error {
	ix, err := survey.Load(indexFile)
	if err != nil {
		return Errorf(ExitUsage, "survey --draft: %v; run `aayu survey` first", err)
	}
	if err := ix.LoadProbes(out); err != nil {
		return Wrap(ExitUsage, err)
	}
	res := SurveyDraftResult{Dir: filepath.Join(out, "sections")}
	// A section is fresh while the index and the golden probes it may cite
	// are unchanged.
	pd, err := survey.ProbesDigest(out)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	digest := ix.Digest
	if pd != "" {
		digest += "+probes:" + pd
	}
	saved, stale, err := survey.LoadSections(res.Dir, ix.Commit, digest)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	res.Stale = stale
	parts := map[string]json.RawMessage{}
	for name, s := range saved {
		parts[name] = s.Content
	}
	fromFile := map[string]json.RawMessage{}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		if fromFile, err = survey.SplitDraft(b); err != nil {
			return Errorf(ExitUsage, "%s: %v", file, err)
		}
	}
	for name, b := range fromFile {
		parts[name] = b
	}
	save := func(parts map[string]json.RawMessage, rep survey.SectionReport) error {
		for _, name := range survey.SectionNames() {
			b, ok := fromFile[name]
			if _, in := parts[name]; !ok || !in {
				if in {
					res.Resumed = append(res.Resumed, name)
				}
				continue
			}
			ps := rep.Sections[name]
			if err := survey.SaveSection(res.Dir, survey.SavedSection{Section: name, Commit: ix.Commit, Digest: digest, Pass: len(ps) == 0, Problems: ps, Content: b}); err != nil {
				return Wrap(ExitInternal, err)
			}
			res.Saved = append(res.Saved, name)
		}
		return nil
	}
	d, rep, err := survey.CheckSections(ix, parts)
	if err != nil {
		// A section that does not parse is reported; the sections that do are
		// still checked and saved, so the error never loses them.
		good, bad := survey.Parseable(parts)
		if len(bad) == 0 {
			return Errorf(ExitUsage, "%s: %v", cmp.Or(file, res.Dir), err)
		}
		var msgs []string
		for _, name := range survey.SectionNames() {
			if e, ok := bad[name]; ok {
				msgs = append(msgs, e.Error())
			}
		}
		if _, rep, err := survey.CheckSections(ix, good); err == nil {
			if err := save(good, rep); err != nil {
				return err
			}
		}
		saved := "none saved"
		if len(res.Saved) > 0 {
			saved = "saved " + strings.Join(res.Saved, ", ")
		}
		return Errorf(ExitUsage, "%s: %s (%s; fix the section and run again)", cmp.Or(file, res.Dir), strings.Join(msgs, "; "), saved)
	}
	res.SectionReport = rep
	if err := save(parts, rep); err != nil {
		return err
	}
	switch {
	case len(rep.Missing) > 0:
		// A merged survey from an earlier run no longer holds: its sections are
		// missing or stale, and a backlog must never be drafted from it.
		for _, f := range []string{filepath.Join(out, "survey.json"), filepath.Join(out, "SURVEY.md")} {
			if same, _ := sameFile(f, file); !same {
				if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
					return Wrap(ExitInternal, err)
				}
			}
		}
		res.Next = "write the next section (" + rep.Missing[0] + ", skills/survey), then run `aayu survey --draft <file>` with it; saved sections are kept"
		if len(rep.Problems) > 0 {
			res.Next = "fix every problem in the sections you wrote, then run again; saved sections are kept"
		}
	default:
		res.Merged, res.Survey = filepath.Join(out, "survey.json"), filepath.Join(out, "SURVEY.md")
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return Wrap(ExitInternal, err)
		}
		if err := os.WriteFile(res.Merged, append(b, '\n'), 0o644); err != nil {
			return Wrap(ExitInternal, err)
		}
		if err := os.WriteFile(res.Survey, []byte(survey.Render(ix, d, rep.Report)), 0o644); err != nil {
			return Wrap(ExitInternal, err)
		}
		res.Next = "fix every problem, then run again"
		if res.Pass {
			res.Next = "interview the person on the open questions (skills/survey), record their answers in the draft, then draft the backlog"
			if res.Coverage.OpenQuestions == 0 {
				res.Next = "draft the backlog from the survey (skills/survey), then `aayu survey --backlog <file> --draft " + res.Merged + "`"
			}
		}
	}
	if err := e.Emit(res, func(w io.Writer) error { return writeSurveyDraft(w, file, res) }); err != nil {
		return err
	}
	if len(res.Problems) > 0 {
		return Errorf(ExitBlocked, "the survey draft has %d problem(s); revise it", len(res.Problems))
	}
	return nil
}

func writeSurveyDraft(w io.Writer, file string, res SurveyDraftResult) error {
	verdict := "passes"
	switch {
	case len(res.Problems) > 0:
		verdict = "does not pass yet"
	case len(res.Missing) > 0:
		verdict = "passes so far; sections still to write: " + strings.Join(res.Missing, ", ")
	}
	c := res.Coverage
	fmt.Fprintf(w, "survey draft %s: %s; modules cited %d of %d, entry points covered %d of %d, markers %d of %d; %d open questions, %d unknown claims\n",
		cmp.Or(file, res.Dir), verdict, c.ModulesCited, c.Modules, c.EntriesCovered, c.Entries, c.MarkersCited, c.Markers, c.OpenQuestions, c.Unknowns)
	for _, name := range survey.SectionNames() {
		ps, ok := res.Sections[name]
		state := "missing"
		switch {
		case !ok:
		case len(ps) == 0:
			state = "passes"
		default:
			state = fmt.Sprintf("%d problem(s)", len(ps))
		}
		how := ""
		if slices.Contains(res.Saved, name) {
			how = ", saved"
		} else if slices.Contains(res.Resumed, name) {
			how = ", resumed"
		}
		fmt.Fprintf(w, "  section %s: %s%s\n", name, state, how)
		for _, p := range ps {
			fmt.Fprintf(w, "    problem: %s\n", p)
		}
	}
	for _, s := range res.Stale {
		fmt.Fprintf(w, "  section %s: saved against another index commit; write it again\n", s)
	}
	if res.Survey != "" {
		fmt.Fprintf(w, "  wrote %s and %s\n", res.Merged, res.Survey)
	}
	fmt.Fprintf(w, "  sections in %s\n  next: %s\n", res.Dir, res.Next)
	return nil
}

// sameFile reports whether a and b name one existing file (the draft given
// may itself be <out>/survey.json).
func sameFile(a, b string) (bool, error) {
	if b == "" {
		return false, nil
	}
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
