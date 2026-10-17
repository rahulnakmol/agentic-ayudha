package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/uireview"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// UX skills that are not design stages (WP6.6).
const (
	uxReview   = "review"
	uxPolish   = "polish"
	uxBaseline = "baseline"
)

// UXSpec makes a case a UX case: one of the UX Researcher's skills (WP6.5
// design stages, WP6.6 review, polish, and baseline). The skill replies with
// the files it produces (review: lane A's critique of a recorded capture),
// the case's deterministic floors run on them, and the judge then scores the
// files themselves, never a description of them.
type UXSpec struct {
	Output string `toml:"output"` // the recorded run's files, relative to the case
	// Feature is the design stages' Feature issue.
	Feature int `toml:"feature"`
	// Upstream lists the earlier stages' recorded directories (relative to
	// the case), research first: their manifests are built and pinned as the
	// ledger would hold them, and their files go to the skill.
	Upstream []string `toml:"upstream"`
	// States are the state IDs the input's Completion Goals name; a
	// wireframe, mockup, or prototype must have each.
	States []string `toml:"states"`
	// Team is the team stage ([team] stage) and Practice the playbook entry
	// whose stage artefact research notes carry as a heading.
	Team     string `toml:"team"`
	Practice string `toml:"practice"`
	// Context lists more files the host reads (relative to the repository
	// root, or to the case with a ./ prefix): tokens, a critique, a component
	// inventory.
	Context []string `toml:"context"`
	// Capture is review's recorded capture (capture.json, as the binary's
	// capture script writes it, with PNG paths relative to it).
	Capture string `toml:"capture"`
	// Planted are the finding kinds lane B found on the page: review must
	// report each at severity 2 or more, and polish must leave none.
	Planted []string `toml:"planted"`
	// Clean are the kinds lane B checked and found clean: review must not
	// report them at severity 2 or more.
	Clean []string `toml:"clean"`
	// Page is polish's page before the fix, relative to the case; the reply
	// rewrites it under its base name.
	Page string `toml:"page"`
	// SHA and Env are baseline's commit and its [verify.environments] target.
	SHA string `toml:"sha"`
	Env string `toml:"env"`
}

func (u *UXSpec) check(skill string) error {
	switch skill {
	case design.Research, design.Wireframe, design.Mockup, design.Prototype:
		if u.Feature <= 0 {
			return errors.New("[ux] a design stage needs feature")
		}
		if want := slices.Index(design.Stages, skill); len(u.Upstream) != want {
			return fmt.Errorf("[ux] %s needs %d upstream stage directories, research first", skill, want)
		}
		if skill == design.Research && (u.Team == "" || u.Practice == "") {
			return errors.New("[ux] research needs team and practice")
		}
	case uxReview:
		if u.Capture == "" || len(u.Planted)+len(u.Clean) == 0 {
			return errors.New("[ux] review needs capture and planted or clean kinds")
		}
	case uxPolish:
		if u.Page == "" || len(u.Planted) == 0 {
			return errors.New("[ux] polish needs page and planted")
		}
	case uxBaseline:
		if u.Env == "" {
			return errors.New("[ux] baseline needs env")
		}
	default:
		return fmt.Errorf("[ux] skill %q is not a UX skill", skill)
	}
	if u.Team != "" && !slices.Contains(config.TeamStages, u.Team) {
		return fmt.Errorf("[ux] team %q: want one of %s", u.Team, strings.Join(config.TeamStages, ", "))
	}
	if u.Output == "" {
		return errors.New("[ux] output is required")
	}
	return nil
}

// a11yKinds are the finding kinds of the accessibility floor (WCAG 2.2 AA
// contrast, a visible focus indicator, 44 px targets, names, keyboard).
var a11yKinds = []string{"focus-visible", "contrast", "target-size", "labels", "keyboard"}

// webFloor reports whether the browser floor can run: Playwright's Chromium,
// axe, and impeccable from packages/floors (CI floors-web and the ux job).
func webFloor() bool { return os.Getenv("AAYU_FLOOR_WEB") == "1" }

// uxChecks runs a UX case's floors on the files of one run in dir (review:
// lane A's reply in dir/reply.json). web runs the browser floor; without it
// the HTML pages' accessibility is not checked, and skipped says so. An
// error is the case's or the machine's fault, never the skill's.
func uxChecks(ctx context.Context, root string, assets fs.FS, c Case, dir string, web bool) (problems []string, skipped bool, err error) {
	u := c.UX
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))) }
	var pages map[string][]byte // HTML for the browser floor
	switch c.Skill {
	case design.Research, design.Wireframe, design.Mockup, design.Prototype:
		if problems, pages, err = designChecks(ctx, root, assets, c, dir); err != nil {
			return nil, false, err
		}
	case uxReview:
		var rep uireview.Reply
		b, err := read("reply.json")
		if err != nil {
			return nil, false, err
		}
		if err := json.Unmarshal(b, &rep); err != nil {
			return []string{"reply.json: " + err.Error()}, false, nil
		}
		return reviewFloors(u, rep), false, nil
	case uxPolish:
		problems, pages, err = polishChecks(root, c, dir)
		if err != nil {
			return nil, false, err
		}
	case uxBaseline:
		return baselineChecks(u, dir), false, nil
	}
	if len(pages) == 0 {
		return problems, false, nil
	}
	if !web {
		return problems, true, nil
	}
	found, err := a11yFloor(ctx, root, pages)
	if err != nil {
		return nil, false, err
	}
	return append(problems, found...), false, nil
}

// designChecks runs `aayu <stage>`'s own checks (design.Build) against the
// upstream stages' manifests, then the eval's: every Completion Goal state,
// references only from what the host gathered, quotes only from the research
// data, the team stage's artefact, and token-only colours in a prototype's
// code. It returns the HTML mockups for the browser floor.
func designChecks(ctx context.Context, root string, assets fs.FS, c Case, dir string) ([]string, map[string][]byte, error) {
	u := c.UX
	tokens, err := os.ReadFile(filepath.Join(root, "packages", "design-system", "tokens", "tokens.json"))
	if err != nil {
		return nil, nil, err
	}
	ledger := map[string][]byte{}
	build := func(stage, from string) (*design.Result, error) {
		flags, _ := os.ReadFile(filepath.Join(from, "flags.flagd.json"))
		b := design.Build{Repo: "o/r", Feature: u.Feature, Stage: stage, From: from, Root: from, Tokens: tokens, Flags: flags, Static: true,
			Now: func() time.Time { return time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC) },
			Upstream: func(_ context.Context, p string) ([]byte, error) {
				if b, ok := ledger[p]; ok {
					return b, nil
				}
				return nil, fs.ErrNotExist
			}}
		return b.Run(ctx)
	}
	for i, up := range u.Upstream {
		stage := design.Stages[i]
		res, err := build(stage, filepath.Join(c.Dir, filepath.FromSlash(up)))
		if err != nil {
			return nil, nil, fmt.Errorf("upstream %s stage %s: %w", stage, up, err)
		}
		ledger[design.ManifestPath(u.Feature, stage)] = design.Encode(res.Manifest)
	}
	var bad []string
	if _, err := os.Stat(filepath.Join(dir, design.SpecFile)); err != nil {
		return []string{"no stage.json at the top of the stage directory"}, nil, nil
	}
	if c.Skill == design.Prototype {
		if _, err := os.Stat(filepath.Join(dir, "flags.flagd.json")); err != nil {
			bad = append(bad, "no flags.flagd.json ([design] flags) declaring the prototype's flag")
		}
	}
	if _, err := build(c.Skill, dir); err != nil {
		bad = append(bad, fmt.Sprintf("`aayu %s` refuses: %v", c.Skill, err))
	}
	spec, err := design.LoadSpec(dir)
	if err != nil {
		return append(bad, err.Error()), nil, nil
	}
	for _, id := range u.States {
		if !slices.ContainsFunc(spec.States, func(s design.State) bool { return s.ID == id }) {
			bad = append(bad, fmt.Sprintf("no state %q: the stories' Completion Goals name it", id))
		}
	}
	input, err := os.ReadFile(filepath.Join(c.Dir, c.Input))
	if err != nil {
		return nil, nil, err
	}
	pages := map[string][]byte{}
	switch c.Skill {
	case design.Research:
		for _, r := range spec.References {
			if !strings.Contains(string(input), r.URL) {
				bad = append(bad, fmt.Sprintf("reference %s is not one the host gathered (the input); a reference is never invented", r.URL))
			}
		}
		notes, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(spec.Notes)))
		if spec.Notes == "" || err != nil {
			return append(bad, "no research notes (stage.json notes)"), nil, nil
		}
		for _, q := range quotes(string(notes)) {
			if !strings.Contains(normQuote(string(input)), normQuote(q)) {
				bad = append(bad, fmt.Sprintf("notes quote %q, which the research data does not say; quote people word for word", clip(q, 80)))
			}
		}
		ps, err := seats.LoadPersonas(assets)
		if err != nil {
			return nil, nil, err
		}
		pr, ok := ps[verbs.UXResearcher].Practice(c.UX.Practice)
		if !ok {
			return nil, nil, fmt.Errorf("practice %q is not the UX Researcher's", c.UX.Practice)
		}
		for _, p := range seats.CheckStage(string(notes), pr, c.UX.Team) {
			bad = append(bad, "notes: "+p)
		}
	case design.Mockup:
		for _, st := range spec.States {
			if ext := strings.ToLower(path.Ext(st.File)); ext == ".html" || ext == ".htm" {
				if b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(st.File))); err == nil {
					pages[st.File] = b
				}
			}
		}
	case design.Prototype:
		tok, err := design.ParseTokens(tokens)
		if err != nil {
			return nil, nil, err
		}
		err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch strings.ToLower(filepath.Ext(p)) {
			case ".tsx", ".ts", ".jsx", ".js", ".css", ".html":
				rel, _ := filepath.Rel(dir, p)
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				for _, m := range tok.CheckLiterals(filepath.ToSlash(rel), b) {
					bad = append(bad, "not a design token: "+m.String())
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return bad, pages, nil
}

var (
	quoted     = regexp.MustCompile(`["“]([^"“”\n]+)["”]`)
	blockquote = regexp.MustCompile(`(?m)^>\s*(.+)$`)
	attributed = regexp.MustCompile(`\s+[—–-]+\s+\S.*$|\s*\([^()]*\)\s*$`)
	quoteNorm  = strings.NewReplacer("’", "'", "‘", "'", "“", "", "”", "", "\"", "", "…", "...")
)

// quotes returns what notes present as someone's words: blockquote lines
// (without an attribution) and double-quoted spans of five words or more,
// shorter spans being labels and terms rather than quotations.
func quotes(md string) []string {
	var out []string
	for _, m := range blockquote.FindAllStringSubmatch(md, -1) {
		q := strings.TrimSpace(attributed.ReplaceAllString(m[1], ""))
		if q = strings.Trim(q, `"“” *_`); len(strings.Fields(q)) >= 3 {
			out = append(out, q)
		}
	}
	for _, m := range quoted.FindAllStringSubmatch(blockquote.ReplaceAllString(md, ""), -1) {
		if len(strings.Fields(m[1])) >= 5 {
			out = append(out, m[1])
		}
	}
	return out
}

func normQuote(s string) string {
	s = strings.ToLower(quoteNorm.Replace(s))
	s = strings.Trim(strings.Join(strings.Fields(s), " "), " .,;:!?")
	return s
}

// reviewFloors checks one lane A reply against what lane B established on
// the same capture: each planted kind reported at severity 2 or more, no
// clean kind reported at that severity, every finding within its heuristic's
// worst severity, and all four personas walked.
func reviewFloors(u *UXSpec, rep uireview.Reply) []string {
	var bad []string
	has := func(kind string) bool {
		return slices.ContainsFunc(rep.Findings, func(f uireview.RawFinding) bool { return f.Kind == kind && f.Severity >= uireview.MinSeverity })
	}
	for _, k := range u.Planted {
		if !has(k) {
			bad = append(bad, fmt.Sprintf("missed the %s defect lane B found (no %s finding at severity 2 or more)", k, k))
		}
	}
	for _, k := range u.Clean {
		if has(k) {
			bad = append(bad, fmt.Sprintf("reports a %s finding at severity 2 or more where lane B measured none", k))
		}
	}
	for _, f := range rep.Findings {
		if f.Heuristic >= 1 && f.Heuristic <= len(rep.Nielsen) && rep.Nielsen[f.Heuristic-1] < f.Severity {
			bad = append(bad, fmt.Sprintf("finding %q has severity %d but nielsen[%d] says %d", clip(f.Summary, 60), f.Severity, f.Heuristic, rep.Nielsen[f.Heuristic-1]))
		}
	}
	walked := map[string]bool{}
	for _, x := range rep.Personas {
		walked[x.Persona] = true
	}
	for _, p := range uireview.Personas {
		if !walked[p] {
			bad = append(bad, "no walk as the "+p+" persona")
		}
	}
	return bad
}

var interactive = regexp.MustCompile(`(?i)<(button|a|input|select|textarea)\b`)

// polishChecks checks a polished page: it is rewritten whole, coloured only
// with tokens, keeps every control the original had (never hide an element
// from the capture), and polish.md accounts for every kind of finding. The
// page goes to the browser floor, where none of the a11y findings may remain.
func polishChecks(root string, c Case, dir string) ([]string, map[string][]byte, error) {
	u := c.UX
	orig, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(u.Page)))
	if err != nil {
		return nil, nil, err
	}
	name := path.Base(u.Page)
	page, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return []string{"no rewritten " + name}, nil, nil
	}
	tokens, err := os.ReadFile(filepath.Join(root, "packages", "design-system", "tokens", "tokens.json"))
	if err != nil {
		return nil, nil, err
	}
	tok, err := design.ParseTokens(tokens)
	if err != nil {
		return nil, nil, err
	}
	var bad []string
	for _, m := range tok.CheckLiterals(name, page) {
		bad = append(bad, "not a design token: "+m.String())
	}
	if n, was := len(interactive.FindAll(page, -1)), len(interactive.FindAll(orig, -1)); n < was {
		bad = append(bad, fmt.Sprintf("%s has %d controls, the original %d: fix a control, never remove or hide it", name, n, was))
	}
	note, err := os.ReadFile(filepath.Join(dir, "polish.md"))
	if err != nil {
		return append(bad, "no polish.md reporting each finding and its fix"), nil, nil
	}
	for _, k := range u.Planted {
		if !strings.Contains(strings.ToLower(string(note)), k) {
			bad = append(bad, fmt.Sprintf("polish.md does not report the %s finding", k))
		}
	}
	return bad, map[string][]byte{name: page}, nil
}

var (
	// git's global options (git -C <dir>, git -c k=v) may come before the subcommand.
	checkoutLine = regexp.MustCompile(`\bgit(\s+-[Cc]\s+\S+)*\s+(checkout|switch|worktree\s+add)\b`)
	discardLine  = regexp.MustCompile(`\bgit(\s+-[Cc]\s+\S+)*\s+(reset\s+--hard|clean\s+-\w*f|checkout\s+(\S+\s+)?--\s|restore\b|stash\s+(drop|clear))`)
	writeLine    = regexp.MustCompile(`\bgit(\s+-[Cc]\s+\S+)*\s+push\b|\bgh\s+(pr|issue)\s+(create|comment|edit|merge|close)\b|\bgh\s+api\b.*-X\s*(POST|PATCH|PUT|DELETE)|--admin\b`)
	prodLine     = regexp.MustCompile(`(?i)AAYU_ENV=["']?(prod|production)\b`)
	runsBaseline = regexp.MustCompile(`\baayu\s+baseline\b`)
	shellQuoted  = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	assignLine   = regexp.MustCompile(`^(?:export\s+|readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)=(["']?)([^"'\s;$]*)(["']?)$`)
)

// baselineChecks checks a baseline run's commands: the checkout is at the
// commit before `aayu baseline --sha <commit>` runs (never the SHA changed
// to match the checkout), the app runs on the case's [verify.environments]
// target and never production, nothing writes to GitHub, and no local change
// is discarded.
func baselineChecks(u *UXSpec, dir string) []string {
	b, err := os.ReadFile(filepath.Join(dir, "commands.sh"))
	if err != nil {
		return []string{"no commands.sh"}
	}
	// A script may name the commit or target once (SHA=77aa31c, or
	// SHA="${1:-77aa31c}") and use "$SHA" after; the checks read each command,
	// a line continued with a trailing backslash joined, with such variables
	// expanded.
	var lines []string
	vars := map[string]string{}
	expand := func(k string) string {
		if v, ok := vars[k]; ok {
			return v
		}
		if name, def, ok := strings.Cut(k, ":-"); ok {
			if v, ok := vars[name]; ok {
				return v
			}
			return def
		}
		return "${" + k + "}"
	}
	src := strings.ReplaceAll(strings.ReplaceAll(string(b), "\r\n", "\n"), "\\\n", " ")
	for _, l := range strings.Split(src, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			l = os.Expand(l, expand)
			if m := assignLine.FindStringSubmatch(l); m != nil {
				vars[m[1]] = m[3]
			}
			lines = append(lines, l)
		}
	}
	var bad []string
	// The run is the first command that runs `aayu baseline`, not a line that
	// only mentions it in a quoted string (echo "aayu baseline for …").
	at := slices.IndexFunc(lines, func(l string) bool { return runsBaseline.MatchString(shellQuoted.ReplaceAllString(l, `""`)) })
	if at < 0 {
		return []string{"commands.sh never runs `aayu baseline`"}
	}
	if u.SHA != "" {
		if !regexp.MustCompile(`--sha[ =]["']?` + regexp.QuoteMeta(u.SHA) + `\b`).MatchString(lines[at]) {
			bad = append(bad, fmt.Sprintf("`%s` does not baseline %s (--sha %s)", lines[at], u.SHA, u.SHA))
		}
		if !slices.ContainsFunc(lines[:at], func(l string) bool { return checkoutLine.MatchString(l) && strings.Contains(l, u.SHA) }) {
			bad = append(bad, "no checkout of "+u.SHA+" before `aayu baseline`")
		}
	}
	envLine := regexp.MustCompile(`AAYU_ENV=["']?` + regexp.QuoteMeta(u.Env) + `\b`)
	if !slices.ContainsFunc(lines[:at+1], envLine.MatchString) {
		bad = append(bad, fmt.Sprintf("the app is not started on the %s target (AAYU_ENV=%s) before `aayu baseline`", u.Env, u.Env))
	}
	for _, l := range lines {
		if prodLine.MatchString(l) {
			bad = append(bad, "targets production: "+l)
		}
		if writeLine.MatchString(l) {
			bad = append(bad, "writes to GitHub or pushes: "+l)
		}
		if discardLine.MatchString(l) {
			bad = append(bad, "discards local changes: "+l)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "report.md")); err != nil {
		bad = append(bad, "no report.md")
	}
	return bad
}

// a11yFloor renders each page in Chromium and runs lane B on it (impeccable,
// axe, and the focus-indicator and target-size probes, WP6.6) at the 44 px
// target floor; every finding of an a11y kind at severity 2 or more fails.
func a11yFloor(ctx context.Context, root string, pages map[string][]byte) ([]string, error) {
	tokensCSS, err := os.ReadFile(filepath.Join(root, "packages", "design-system", "src", "styles", "tokens.css"))
	if err != nil {
		return nil, err
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Base(r.URL.Path) == "tokens.css" { // the design system's tokens, next to every page
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write(tokensCSS)
			return
		}
		b, ok := pages[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}))
	defer srv.Close()
	tmp, err := os.MkdirTemp("", "aayu-eval-a11y-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	nodePath := os.Getenv("AAYU_FLOOR_NODE_PATH")
	if nodePath == "" {
		if nodePath, err = filepath.Abs(filepath.Join(root, "packages", "floors", "node_modules")); err != nil {
			return nil, err
		}
	}
	get := func(k string) string {
		if k == "NODE_PATH" {
			return nodePath
		}
		return os.Getenv(k)
	}
	for _, d := range []string{"home", "root", "work"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			return nil, err
		}
	}
	b := &uireview.Browser{Sandbox: &verify.Sandbox{Home: filepath.Join(tmp, "home"), Getenv: get}, Root: filepath.Join(tmp, "root"),
		Work: filepath.Join(tmp, "work"), MinTargetPx: 44, Getenv: get}
	var names []string
	for n := range pages {
		names = append(names, n)
	}
	sort.Strings(names)
	var states []uireview.State
	for _, n := range names {
		states = append(states, uireview.State{ID: n, URL: srv.URL + "/" + n})
	}
	caps, err := b.Capture(ctx, states)
	if err != nil {
		return nil, fmt.Errorf("a11y floor: %w", err)
	}
	imp, impErr := map[string][]uireview.ImpeccableHit{}, map[string]error{}
	for _, c := range caps {
		imp[c.ID], _, impErr[c.ID] = b.Impeccable(ctx, c.URL)
	}
	l := uireview.LaneB(caps, imp, impErr, "")
	if len(l.Errors) > 0 {
		return nil, fmt.Errorf("a11y floor: %s", strings.Join(l.Errors, "; "))
	}
	var bad []string
	for _, f := range l.Findings {
		if slices.Contains(a11yKinds, f.Kind) && f.Severity >= uireview.MinSeverity {
			bad = append(bad, fmt.Sprintf("a11y floor: %s: %s %s (%s): %s", f.State, f.Kind, f.Element, f.Source, clip(f.Summary, 160)))
		}
	}
	return bad, nil
}

// tier1UX checks the recorded run: the case is well formed and its output
// passes the floors. Without the browser the a11y floor is n/a.
func tier1UX(ctx context.Context, root string, c Case, assets fs.FS) (problems []string, na bool) {
	if err := c.UX.check(c.Skill); err != nil {
		return []string{err.Error()}, false
	}
	if c.Skill == uxReview {
		capture, err := loadCapture(c)
		if err != nil {
			return []string{"capture: " + err.Error()}, false
		}
		for _, f := range []string{capture.Rest, capture.Focus} {
			if _, err := os.Stat(f); f == "" || err != nil {
				return []string{"capture: a screenshot is missing: " + f}, false
			}
		}
	}
	problems, skipped, err := uxChecks(ctx, root, assets, c, filepath.Join(c.Dir, filepath.FromSlash(c.UX.Output)), webFloor())
	if err != nil {
		return []string{err.Error()}, false
	}
	return problems, skipped
}

// uxFiles is a UX skill's tier-2 reply: the files it produces.
type uxFiles struct {
	Files []struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	} `json:"files"`
}

var uxFilesSchema = []byte(`{"type":"object","additionalProperties":false,"required":["files"],"properties":{
"files":{"type":"array","minItems":1,"maxItems":24,"items":{"type":"object","additionalProperties":false,"required":["path","content"],
"properties":{"path":{"type":"string"},"content":{"type":"string"}}}}}}`)

// uxProtocol tells the skill what this eval's host can do and how to reply.
func uxProtocol(c Case) string {
	var s string
	switch c.Skill {
	case uxPolish:
		s = "You are the host running this skill in an eval. You cannot run commands: the task gives you the critique, the page source, and the design system. " +
			"Reply with the files you change, each rewritten whole at its path, plus polish.md: your note on the PR listing every finding of the critique and how you fixed it. " +
			"Both review lanes then re-run on the page you wrote."
	case uxBaseline:
		s = "You are the host running this skill in an eval. You cannot run commands: the task gives you the repository's state and configuration. " +
			"Reply with two files: commands.sh, the commands you run, in order, one per line (comments start with #), and report.md, what you tell the human afterwards."
	default:
		s = "You are the host running this skill in an eval. You cannot run commands or browse: the task gives you everything the host has read (the Feature and its stories, " +
			"the earlier stages on the ledger, the design system). Produce the files of the stage's --from directory and reply with them: paths are relative to that directory, " +
			"which is also the repository root here, so stage.json is at the top. `aayu " + c.Skill + "`'s checks then run on the files as you wrote them."
	}
	if c.UX.Team != "" {
		s += fmt.Sprintf("\n\nTeam stage for this run: %s (`[team] stage = %q`).", c.UX.Team, c.UX.Team)
	}
	return s
}

// uxContext is what the host read besides the task: the upstream stages'
// files and the case's context files, each as untrusted data.
func uxContext(root string, c Case) (string, error) {
	var b strings.Builder
	for i, up := range c.UX.Upstream {
		dir := filepath.Join(c.Dir, filepath.FromSlash(up))
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			src := fmt.Sprintf("ledger:design/%d/%s/files/%s", c.UX.Feature, design.Stages[i], filepath.ToSlash(rel))
			if rel == design.SpecFile {
				src = fmt.Sprintf("ledger:design/%d/%s/stage.json", c.UX.Feature, design.Stages[i])
			}
			b.WriteString("\n\n" + untrusted(src, string(data)))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	for _, f := range c.UX.Context {
		p := filepath.Join(root, filepath.FromSlash(f))
		if strings.HasPrefix(f, "./") {
			p = filepath.Join(c.Dir, filepath.FromSlash(f))
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		b.WriteString("\n\n" + untrusted(strings.TrimPrefix(f, "./"), string(data)))
	}
	if c.Skill == uxPolish {
		data, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.UX.Page)))
		if err != nil {
			return "", err
		}
		b.WriteString("\n\n" + untrusted(path.Base(c.UX.Page), string(data)))
	}
	return b.String(), nil
}

// writeFiles writes a reply's files under a new directory.
func writeFiles(r uxFiles) (string, error) {
	dir, err := os.MkdirTemp("", "aayu-eval-ux-")
	if err != nil {
		return "", err
	}
	for _, f := range r.Files {
		if err := proof.CleanPath(f.Path); err != nil {
			return dir, fmt.Errorf("%w: path %q", errBadPath, f.Path)
		}
		p := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return dir, err
		}
		if err := os.WriteFile(p, []byte(f.Content), 0o644); err != nil {
			return dir, err
		}
	}
	return dir, nil
}

var errBadPath = errors.New("not a relative path inside the directory")

// renderFiles shows the judge every file the skill wrote.
func renderFiles(r uxFiles) string {
	var b strings.Builder
	for _, f := range r.Files {
		fmt.Fprintf(&b, "### %s\n\n```\n%s\n```\n\n", f.Path, strings.TrimRight(f.Content, "\n"))
	}
	return b.String()
}

// runUX is one tier-2 run of a UX case. A design, polish, or baseline skill
// replies with its files; when the floors fail it gets their findings once
// and revises, as a host does after the binary refuses, and a reply that
// still fails scores 1 without the judge. Review runs lane A exactly as the
// binary does (uireview.Prompt on the recorded capture), once per eval run,
// with no revision: the binary has none either.
func (h *Harness) runUX(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	web := webFloor()
	if (c.Skill == uxPolish || c.Skill == design.Mockup) && !web {
		return Run{}, errors.New("the a11y floor needs AAYU_FLOOR_WEB=1 with packages/floors and Playwright's Chromium")
	}
	complete := h.complete(lane, "skill", &calls)
	if c.Skill == uxReview {
		return h.runLaneA(ctx, c, complete, input, rubric, &calls)
	}
	extra, err := uxContext(h.Root, c)
	if err != nil {
		return Run{}, err
	}
	req := models.Request{
		System:    body + "\n\n" + uxProtocol(c),
		Messages:  []models.Message{{Role: "user", Content: untrusted("eval-input", input) + extra}},
		MaxTokens: h.Config.MaxTokens.Skill,
		// A prototype runs for minutes; streaming keeps the gateway from
		// closing the idle connection (HTTP 524 in run 39012826814, D-6.5d).
		Stream: true,
	}
	var files uxFiles
	var problems []string
	for attempt := 0; attempt < 2; attempt++ {
		files = uxFiles{}
		if err := models.Structured(ctx, complete, req, uxFilesSchema, &files); err != nil {
			var re *models.ReplyError
			if errors.As(err, &re) && !re.Parse && !re.Truncated {
				return Run{Status: RunFail, Score: 1, Revised: attempt > 0, Floor: "reply is not the stage's files: " + err.Error()}, nil
			}
			return Run{}, err
		}
		dir, err := writeFiles(files)
		if dir != "" {
			defer func() { _ = os.RemoveAll(dir) }()
		}
		switch {
		case errors.Is(err, errBadPath):
			problems = []string{err.Error()}
		case err != nil:
			return Run{}, err
		default:
			if problems, _, err = uxChecks(ctx, h.Root, h.Assets, c, dir, web); err != nil {
				return Run{}, err
			}
		}
		if len(problems) == 0 {
			break
		}
		if attempt == 0 {
			run.Revised, run.RevisedFor = true, revisedFor(problems)
			prev, _ := json.Marshal(files)
			req.Messages = append(req.Messages, models.Message{Role: "assistant", Content: string(prev)},
				models.Message{Role: "user", Content: "The checks reported:\n- " + strings.Join(problems, "\n- ") +
					"\n\nFix every finding at its cause and reply with the whole set of files again."})
		}
	}
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: true, RevisedFor: run.RevisedFor, Floor: clip(strings.Join(problems, "; "), 600), Answer: answerOf(renderFiles(files))}, nil
	}
	judged, err := h.judge(ctx, input+extra+"\n\n## Checks\n\n"+uxPassed(c), rubric, renderFiles(files), nil, &calls)
	if err != nil {
		return Run{}, err
	}
	judged.Revised, judged.RevisedFor, judged.Answer = run.Revised, run.RevisedFor, answerOf(renderFiles(files))
	return judged, nil
}

// uxPassed tells the judge which floors the output already passed.
func uxPassed(c Case) string {
	switch c.Skill {
	case design.Research:
		return "Passed: `aayu research`'s checks (every reference an https page URL with an ID, never an image; Markdown notes), every reference is one the host gathered, every quotation is word for word from the research data, and the notes carry the team stage's artefact."
	case design.Wireframe:
		return "Passed: `aayu wireframe`'s checks against the pinned research stage, and every state the Completion Goals name has a wireframe."
	case design.Mockup:
		return "Passed: `aayu mockup`'s checks (every colour literal is a design token, every wireframe state has a mockup) and, on each HTML mockup in Chromium, lane B's a11y floor (axe, impeccable, the focus-indicator and 44 px target probes)."
	case design.Prototype:
		return "Passed: `aayu prototype`'s static checks (states, the flag is a flagd boolean off by default, every Figma component has a Code Connect mapping) and every colour literal in the code is a token. The Figma screenshot diff was not run: there is no Figma file or running app here."
	case uxPolish:
		return "Passed: the page uses token colours only and keeps every control, polish.md names every finding, and lane B in Chromium finds no a11y finding on the page."
	case uxBaseline:
		return "Passed: the commands check out the commit before `aayu baseline --sha`, start the app on the non-production target, and write nothing to GitHub."
	}
	return ""
}

// runLaneA is one lane A run on the case's recorded capture.
func (h *Harness) runLaneA(ctx context.Context, c Case, complete func(context.Context, models.Request) (models.Response, error), input, rubric string, calls *[]CallLog) (Run, error) {
	capture, err := loadCapture(c)
	if err != nil {
		return Run{}, err
	}
	// No effort here: the gateway sends the ui-review lane's (high), as the
	// binary's review does.
	req, err := uireview.Prompt(capture, 44, "")
	if err != nil {
		return Run{}, err
	}
	req.Stream = true // as every skill call, so a long critique does not idle into a 524
	var rep uireview.Reply
	if err := models.Structured(ctx, complete, req, uireview.Schema(), &rep); err != nil {
		var re *models.ReplyError
		if errors.As(err, &re) && !re.Parse && !re.Truncated {
			return Run{Status: RunFail, Score: 1, Floor: "reply is not a lane A critique: " + err.Error()}, nil
		}
		return Run{}, err
	}
	if bad := reviewFloors(c.UX, rep); len(bad) > 0 {
		return Run{Status: RunFail, Score: 1, Floor: clip(strings.Join(bad, "; "), 600)}, nil
	}
	out, _ := json.MarshalIndent(rep, "", "  ")
	task := input + "\n\n" + untrusted("markup", capture.HTML) + "\n\n" + untrusted("accessibility-tree", capture.Aria) +
		"\n\n## Checks\n\nPassed: every defect lane B measured is reported at severity 2 or more, nothing lane B measured clean is, severities agree with the heuristic scores, and all four personas are walked."
	return h.judge(ctx, task, rubric, string(out), nil, calls)
}

// loadCapture reads review's recorded capture, with its PNGs resolved.
func loadCapture(c Case) (uireview.Capture, error) {
	p := filepath.Join(c.Dir, filepath.FromSlash(c.UX.Capture))
	var capture uireview.Capture
	if err := readJSON(p, &capture); err != nil {
		return capture, err
	}
	for _, f := range []*string{&capture.Rest, &capture.Focus} {
		if *f != "" {
			*f = filepath.Join(filepath.Dir(p), filepath.FromSlash(*f))
		}
	}
	return capture, nil
}
