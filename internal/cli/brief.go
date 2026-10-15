package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("brief", runBrief) }

// STEWordsPath is the STE approved-word list inside the embedded skills.
const STEWordsPath = "skills/brief/references/ste-words.md"

type briefReport struct {
	Subject   int               `json:"subject"`
	Register  string            `json:"register"`
	Purpose   string            `json:"purpose"`
	Grounding *brief.Grounding  `json:"grounding"`
	Findings  []brief.Finding   `json:"findings,omitempty"`
	Reading   *brief.Reading    `json:"reading,omitempty"`
	Review    *brief.Review     `json:"review,omitempty"`
	A11y      *verify.PageCheck `json:"a11y,omitempty"`
	AltText   []string          `json:"alt_text,omitempty"`
	Figures   []*brief.Figure   `json:"figures,omitempty"`
	HTML      string            `json:"html,omitempty"`
	SVGs      []string          `json:"svgs,omitempty"`
	Manifest  string            `json:"manifest,omitempty"`
	Draft     string            `json:"draft_sha256,omitempty"`
	Generated time.Time         `json:"generated_at"`
	Refused   []string          `json:"refused,omitempty"`
	stories   []int             // stories and bugs whose packs ground the brief
	tree      *ledger.Tree      // the ledger read, for screens
	store     *proof.Store
	entries   map[string]proof.Entry
}

// runBrief: without --draft it prints the grounding to write from; with a
// draft it runs the deterministic checks, the adversarial pass, renders the
// page, and checks the page's accessibility. `close` first needs an
// authoritative pack for every story it covers and a Result.
func runBrief(ctx context.Context, e *Env, in Values) error {
	n := issueNumber(in.String("subject"))
	rep := &briefReport{Subject: n, Register: orDefault(in.String("register"), "engineer"), Purpose: orDefault(in.String("purpose"), "explain"), Generated: e.Now().UTC()}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	if err := e.briefGrounding(ctx, c, repo, rep, func() (*proof.Store, error) { return e.proofStore(ctx, c, tok, "human", "") }); err != nil {
		return err
	}
	if rep.tree != nil {
		defer rep.tree.Close()
	}
	g := rep.Grounding
	if rep.Purpose == "close" {
		rep.Refused = closeGate(g, rep.stories)
		if len(rep.Refused) > 0 {
			_ = e.Emit(rep, func(w io.Writer) error {
				for _, r := range rep.Refused {
					fmt.Fprintf(w, "refused: %s\n", r)
				}
				return nil
			})
			return Errorf(ExitBlocked, "brief close: #%d: %s", n, strings.Join(rep.Refused, "; "))
		}
	}
	if !in.Has("draft") {
		return e.Emit(rep, func(w io.Writer) error { return groundingText(w, rep) })
	}

	draftPath := in.String("draft")
	src, err := os.ReadFile(draftPath)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	h := sha256.Sum256(src)
	rep.Draft = hex.EncodeToString(h[:])
	d, err := brief.Parse(src, filepath.Dir(draftPath))
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	words, err := e.steWords()
	if err != nil {
		return err
	}
	rep.Findings = brief.Check(d, g, brief.Options{Register: rep.Register, Purpose: rep.Purpose, Words: words})
	if rep.Register == "eli5" {
		r := brief.Readability(d.Units)
		rep.Reading = &r
	}
	figs, more := brief.Prepare(ctx, d, g, brief.Sources{Artifact: rep.artifact})
	rep.Findings = append(rep.Findings, more...)
	rep.Figures = figs
	if len(rep.Findings) > 0 {
		_ = e.Emit(rep, func(w io.Writer) error { return findingsText(w, rep) })
		return Errorf(ExitFloor, "brief: %d check(s) failed; nothing rendered", len(rep.Findings))
	}

	// The adversarial pass: a model from another family than the writer looks
	// for claims the sources do not support (ground rule 5). It can block a
	// brief, never pass one the checks above failed (rule 7).
	review, err := e.briefReview(ctx, g, src)
	if err != nil {
		return err
	}
	rep.Review = &review
	if len(review.Findings) > 0 {
		_ = e.Emit(rep, func(w io.Writer) error { return findingsText(w, rep) })
		return Errorf(ExitBlocked, "brief: the adversarial pass found %d claim(s) the sources do not support; nothing rendered", len(review.Findings))
	}

	checks := []string{fmt.Sprintf("Structure, citations, and visuals: pass (%d figure(s)).", len(figs))}
	switch rep.Register {
	case "eli5":
		checks = append(checks, fmt.Sprintf("Reading grade (Flesch-Kincaid): %.1f, at most %.0f.", rep.Reading.Grade, brief.MaxGrade))
	case "ste":
		checks = append(checks, fmt.Sprintf("STE lint: pass (sentences of at most %d words, %d approved words, one instruction per sentence).", brief.MaxSTEWords, words.Len()))
	case "regulatory":
		checks = append(checks, "Every paragraph and list item cites a source.")
	}
	page, err := brief.Render(d, g, figs, brief.Page{Register: rep.Register, Purpose: rep.Purpose, Version: e.Build.Version, Generated: rep.Generated, Draft: rep.Draft, Checks: checks, Review: review})
	if err != nil {
		return err
	}
	dir := filepath.Join(e.stateDir(), "state", "briefs", fmt.Sprint(n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Join(dir, rep.Register+"-"+rep.Purpose)
	for _, f := range figs {
		if f.SVG == "" {
			continue
		}
		p := fmt.Sprintf("%s-fig-%d.svg", base, f.N)
		if err := os.WriteFile(p, []byte(f.SVG), 0o644); err != nil {
			return err
		}
		rep.SVGs = append(rep.SVGs, p)
	}
	rep.HTML, rep.Manifest = base+".html", base+".json"
	if err := os.WriteFile(rep.HTML, page, 0o644); err != nil {
		return err
	}

	// Accessibility of the page itself: alt text here, axe in a browser.
	rep.AltText = brief.AltText(page)
	a11yText, a11yErr := "", error(nil)
	pc, err := e.checkPage(ctx, rep.HTML)
	switch {
	case err != nil:
		a11yText, a11yErr = "not checked: "+err.Error(), Errorf(ExitExternal, "brief: the page is written to %s, but its accessibility could not be checked: %v", rep.HTML, err)
	default:
		rep.A11y = &pc
		a11yText = fmt.Sprintf("%s found %d critical and %d serious issues (WCAG 2.2 AA); every image has alt text.", pc.Tool, pc.Critical, pc.Serious)
		if len(rep.AltText) > 0 {
			a11yText = fmt.Sprintf("%s found %d critical and %d serious issues (WCAG 2.2 AA); %s.", pc.Tool, pc.Critical, pc.Serious, strings.Join(rep.AltText, "; "))
		}
		if pc.Critical+pc.Serious > 0 || len(rep.AltText) > 0 {
			a11yErr = Errorf(ExitFloor, "brief: the page has %d critical and %d serious axe findings and %d alt-text problem(s): %s", pc.Critical, pc.Serious, len(rep.AltText), rep.HTML)
		}
	}
	page = brief.SetA11y(page, a11yText)
	if err := os.WriteFile(rep.HTML, page, 0o644); err != nil {
		return err
	}
	m, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(rep.Manifest, append(m, '\n'), 0o644); err != nil {
		return err
	}
	if err := e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "#%d %s/%s brief: %s\n", n, rep.Register, rep.Purpose, rep.HTML)
		for _, s := range rep.SVGs {
			fmt.Fprintf(w, "  static diagram or chart: %s\n", s)
		}
		fmt.Fprintf(w, "  accessibility: %s\n", a11yText)
		return nil
	}); err != nil {
		return err
	}
	return a11yErr
}

// briefGrounding reads the subject, the work items under it, the best proof
// pack of every story and bug among them, the Result, and the measure
// snapshots the Result names.
func (e *Env) briefGrounding(ctx context.Context, c *github.Client, repo string, rep *briefReport, store func() (*proof.Store, error)) error {
	n := rep.Subject
	is, it, err := issueItem(ctx, c, repo, n)
	if err != nil {
		return err
	}
	g := &brief.Grounding{Repo: repo, Subject: issueSource(is, it)}
	rep.Grounding = g
	if it.Type == "story" || it.Type == "bug" {
		rep.stories = []int{n}
	} else {
		if err := e.briefItems(ctx, c, repo, n, 0, g); err != nil {
			return err
		}
		for _, x := range g.Items {
			if x.Type == "story" || x.Type == "bug" {
				rep.stories = append(rep.stories, x.Number)
			}
		}
	}
	field := workitems.FieldResult
	if t, ok := workitems.FindType(it.Type); ok {
		if _, has := t.Field(field); !has {
			field = "outcome"
		}
	}
	if v := strings.TrimSpace(it.Fields[field]); !workitems.Empty(v) {
		h := sha256.Sum256([]byte(v))
		g.Result = &brief.Result{Issue: n, Field: field, Text: v, SHA256: hex.EncodeToString(h[:])}
		for i, m := range snapshotRE.FindAllStringSubmatch(v, -1) {
			s := brief.Snapshot{ID: fmt.Sprintf("D%d", i+1), File: m[1], SHA256: m[2]}
			p := filepath.Join(e.stateDir(), "state", "measure", filepath.Base(m[1]))
			if _, err := os.Stat(p); err == nil {
				s.Path = p
			} else {
				g.Notes = append(g.Notes, fmt.Sprintf("Snapshot %s is named by the Result but is not on this machine; charts cannot query it here.", m[1]))
			}
			g.Snapshots = append(g.Snapshots, s)
		}
	}
	if len(rep.stories) == 0 {
		return nil
	}
	if config.RepoRoot(e.Dir) == "" {
		g.Notes = append(g.Notes, "Proof packs were not read: run aayu brief in a checkout of "+repo+".")
		return nil
	}
	s, err := store()
	if err != nil {
		return err
	}
	t, err := s.Ledger.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		g.Notes = append(g.Notes, "The repository has no ledger branch, so there are no proof packs.")
		return nil
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	rep.tree, rep.store, rep.entries, g.Ledger = t, s, map[string]proof.Entry{}, t.Tip
	for _, story := range rep.stories {
		es, err := s.Packs(ctx, t, story)
		if err != nil {
			return err
		}
		if len(es) == 0 {
			continue
		}
		x := es[0] // authoritative first, then newest
		raw, err := t.File(ctx, x.Path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(raw)
		p := brief.Pack{ID: fmt.Sprintf("P%d", len(g.Packs)+1), Story: story, Authority: x.Pack.Authority.Kind, RunID: x.Pack.Authority.RunID,
			HeadSHA: x.Pack.PRHeadSHA, PR: x.Pack.PR, LedgerPath: x.Path, SHA256: hex.EncodeToString(h[:]), FloorsHold: x.Pack.FloorsHold(), CreatedAt: x.Pack.CreatedAt}
		if p.Authority == proof.Authoritative && (!strings.Contains(x.Path, "/"+proof.Authoritative+"/") || x.Pack.Authority.Workflow != ".github/workflows/"+MergeGateWorkflow) {
			p.Authority = proof.Advisory // only the merge gate writes authoritative packs (WP4.8)
		}
		for _, ck := range x.Pack.Checks {
			p.Checks = append(p.Checks, brief.Floor{Floor: ck.Floor, Result: ck.Result, Value: ck.Value, Threshold: ck.Threshold})
		}
		for _, a := range x.Pack.Artifacts {
			p.Artifacts = append(p.Artifacts, brief.Artifact{Path: a.Path, Kind: a.Kind, SHA256: a.SHA256})
		}
		if p.Criteria, err = packCriteria(ctx, t, story, x.Pack); err != nil {
			return err
		}
		if err := inspection(ctx, c, repo, x.Pack, &p); err != nil {
			return err
		}
		g.Packs = append(g.Packs, p)
		rep.entries[p.ID] = x
	}
	return nil
}

// packCriteria is a pack's criterion -> evidence map: every criterion of the
// story's Ready snapshot on the ledger (the pack's own criteria when there is
// no snapshot), each with the artifacts the pack maps to it.
func packCriteria(ctx context.Context, t *ledger.Tree, story int, pk *proof.Pack) ([]brief.PackCriterion, error) {
	var crit []workitems.Criterion
	b, err := t.File(ctx, ready.LedgerPath(story))
	switch {
	case err == nil:
		snap, err := ready.ParseSnapshot(string(b))
		if err != nil {
			return nil, err
		}
		crit = snap.Criteria
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	default:
		for _, c := range pk.Criteria {
			crit = append(crit, workitems.Criterion{N: c.ID})
		}
	}
	var out []brief.PackCriterion
	for _, c := range crit {
		pc := brief.PackCriterion{ID: c.N, Kind: c.Evidence, Evidence: []brief.Artifact{}}
		if c.State != "" || c.Expected != "" {
			pc.Text = fmt.Sprintf("state: %s; expected: %s", c.State, c.Expected)
		}
		for _, m := range pk.Criteria {
			if m.ID != c.N {
				continue
			}
			for _, ref := range m.Evidence {
				if a, ok := pk.Artifact(ref); ok {
					pc.Evidence = append(pc.Evidence, brief.Artifact{Path: a.Path, Kind: a.Kind, SHA256: a.SHA256})
				}
			}
		}
		out = append(out, pc)
	}
	return out, nil
}

// inspection adds the inspect result (WP4.8) for the pack's head: the
// disposition and its per-criterion verdicts, from inspect's replace-in-place
// comment on the pack's pull request. A result for another head is not this
// pack's and is left out.
func inspection(ctx context.Context, c *github.Client, repo string, pk *proof.Pack, p *brief.Pack) error {
	if pk.PR <= 0 {
		return nil
	}
	owner, name, _ := github.SplitRepo(repo)
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, pk.PR, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	for _, cm := range comments {
		if !strings.Contains(cm.GetBody(), inspect.Marker) {
			continue
		}
		st := inspect.ParseState(cm.GetBody())
		if st.Result == nil || st.Result.Head != pk.PRHeadSHA {
			continue
		}
		r := st.Result
		p.Inspect = &brief.Inspection{Disposition: r.Disposition, Summary: r.Summary, Model: r.Model, Head: r.Head, By: cm.GetUser().GetLogin()}
		for i := range p.Criteria {
			for _, v := range r.Criteria {
				if v.ID == p.Criteria[i].ID {
					p.Criteria[i].Verdict, p.Criteria[i].Judgment = v.Verdict, v.Evidence
				}
			}
		}
	}
	return nil
}

// snapshotRE finds the snapshot a measure Result names (WP7.2 ResultText).
var snapshotRE = regexp.MustCompile(`Snapshot: (\S+\.sqlite) \(SHA-256 ([0-9a-f]{64})`)

func issueSource(is *gh.Issue, it workitems.Item) brief.Issue {
	fields := map[string]string{}
	for k, v := range it.Fields {
		if !workitems.Empty(v) {
			fields[k] = v
		}
	}
	return brief.Issue{Number: is.GetNumber(), Type: it.Type, Title: is.GetTitle(), State: is.GetState(), Status: it.Label("status"),
		URL: is.GetHTMLURL(), UpdatedAt: is.GetUpdatedAt().UTC().Format(time.RFC3339), BodySHA256: workitems.BodyHash(is.GetBody()), Fields: fields}
}

// briefItems adds every work item under issue n, depth first.
func (e *Env) briefItems(ctx context.Context, c *github.Client, repo string, n, depth int, g *brief.Grounding) error {
	if depth > 8 {
		return nil
	}
	owner, name, _ := github.SplitRepo(repo)
	subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(n), &o)
	})
	if err != nil {
		return err
	}
	for _, s := range subs {
		is := (*gh.Issue)(s)
		labels := github.IssueLabels(is)
		typ, _ := workitems.TypeOf(is.GetType().GetName(), labels)
		it := workitems.Item{Labels: labels}
		g.Items = append(g.Items, brief.Issue{Number: is.GetNumber(), Type: typ, Title: is.GetTitle(), State: is.GetState(), Status: it.Label("status"), URL: is.GetHTMLURL()})
		if err := e.briefItems(ctx, c, repo, is.GetNumber(), depth+1, g); err != nil {
			return err
		}
	}
	return nil
}

// closeGate is what `brief close` needs (WP7.3): a Result, and for every
// story and bug the brief covers an authoritative pack whose floors hold.
func closeGate(g *brief.Grounding, stories []int) []string {
	var out []string
	if g.Result == nil {
		out = append(out, fmt.Sprintf("#%d has no Result", g.Subject.Number))
	}
	if len(stories) == 0 {
		out = append(out, fmt.Sprintf("#%d covers no story or bug, so no authoritative pack proves it", g.Subject.Number))
	}
	for _, s := range stories {
		i := slices.IndexFunc(g.Packs, func(p brief.Pack) bool { return p.Story == s })
		switch {
		case i < 0:
			out = append(out, fmt.Sprintf("#%d has no proof pack on the ledger", s))
		case g.Packs[i].Authority != proof.Authoritative:
			out = append(out, fmt.Sprintf("#%d has no authoritative pack (only an advisory one, %s); the merge gate writes authoritative packs", s, g.Packs[i].LedgerPath))
		case !g.Packs[i].FloorsHold:
			out = append(out, fmt.Sprintf("#%d's authoritative pack has floors that do not hold", s))
		}
	}
	return out
}

func (rep *briefReport) artifact(ctx context.Context, p brief.Pack, rel string) ([]byte, error) {
	x, ok := rep.entries[p.ID]
	if !ok || rep.store == nil {
		return nil, fmt.Errorf("pack %s was not read from the ledger", p.ID)
	}
	a, ok := x.Pack.Artifact(rel)
	if !ok {
		return nil, fs.ErrNotExist
	}
	return rep.store.Opener(ctx, rep.tree, x)(*a)
}

func (e *Env) steWords() (*brief.Words, error) {
	if e.Assets == nil {
		return nil, Errorf(ExitInternal, "brief: the embedded skills are not available")
	}
	b, err := fs.ReadFile(e.Assets, path.Clean(STEWordsPath))
	if err != nil {
		return nil, Wrap(ExitInternal, err)
	}
	return brief.ParseWords(b)
}

// briefReview runs the adversarial pass on the review lane: a model from
// another family than the spec lane that wrote the draft.
func (e *Env) briefReview(ctx context.Context, g *brief.Grounding, draft []byte) (brief.Review, error) {
	if e.DryRun {
		return brief.Review{Findings: []brief.ReviewFinding{}, Skipped: "--dry-run"}, nil
	}
	gw, err := e.Gateway()
	if err != nil {
		return brief.Review{}, err
	}
	host, _ := doctor.DetectHost(e.Host, e.Getenv)
	if host == "" {
		host = string(verbs.DirectHost)
	}
	writer, err := gw.Resolver.Resolve(verbs.LaneSpec, verbs.HostID(host))
	if err != nil {
		return brief.Review{}, Wrap(ExitUsage, err)
	}
	reviewer, err := gw.Resolver.ResolveDirect(verbs.LaneReview, verbs.HostID(host), false)
	if err != nil {
		return brief.Review{}, Wrap(ExitUsage, err)
	}
	story := fmt.Sprintf("#%d", g.Subject.Number)
	if err := models.CheckFamilies(story, writer.Family, reviewer.Family); err != nil {
		return brief.Review{}, Wrap(ExitBlocked, fmt.Errorf("brief adversarial pass: %w", err))
	}
	prompt, err := brief.ReviewPrompt(g, draft)
	if err != nil {
		return brief.Review{}, err
	}
	var out struct {
		Findings []brief.ReviewFinding `json:"findings"`
	}
	req := models.Request{System: brief.ReviewSystem, MaxTokens: 2000, Messages: []models.Message{{Role: "user", Content: prompt}},
		Format: &models.Format{Name: "brief_review", Schema: brief.ReviewSchema, Strict: true}}
	err = models.Structured(ctx, func(ctx context.Context, r models.Request) (models.Response, error) {
		resp, _, err := gw.Do(ctx, models.Call{Story: story, Seat: string(verbs.Mentalist), Lane: verbs.LaneReview, Host: verbs.HostID(host), Req: r})
		return resp, err
	}, req, brief.ReviewSchema, &out)
	if err != nil {
		if errors.Is(err, models.ErrBudget) {
			return brief.Review{}, err
		}
		return brief.Review{}, Wrap(ExitExternal, fmt.Errorf("brief adversarial pass: %w", err))
	}
	if out.Findings == nil {
		out.Findings = []brief.ReviewFinding{}
	}
	return brief.Review{Model: reviewer.Model, Family: string(reviewer.Family), Findings: out.Findings}, nil
}

// checkPage runs axe on the rendered page in a browser (the WP4.5 web floor
// script, with Playwright and axe-core resolved from the checkout or NODE_PATH).
func (e *Env) checkPage(ctx context.Context, page string) (verify.PageCheck, error) {
	home, err := os.MkdirTemp("", "aayudha-brief-home-")
	if err != nil {
		return verify.PageCheck{}, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	work, err := os.MkdirTemp("", "aayudha-brief-")
	if err != nil {
		return verify.PageCheck{}, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	root := config.RepoRoot(e.Dir)
	if root == "" {
		root = filepath.Dir(page)
	}
	abs, err := filepath.Abs(page)
	if err != nil {
		return verify.PageCheck{}, err
	}
	return verify.CheckPage(ctx, &verify.Sandbox{Getenv: e.Getenv, Home: home}, root, work, "file://"+filepath.ToSlash(abs))
}

func groundingText(w io.Writer, rep *briefReport) error {
	g := rep.Grounding
	fmt.Fprintf(w, "Grounding for a %s brief (%s) on #%d. Cite sources by ID; say nothing they do not.\n\n", rep.Register, rep.Purpose, g.Subject.Number)
	subj, _ := json.MarshalIndent(g.Subject, "", "  ")
	fmt.Fprintf(w, "[#%d] %s %q (%s)\n%s\n\n", g.Subject.Number, g.Subject.Type, g.Subject.Title, g.Subject.State, brief.Untrusted(fmt.Sprintf("issue #%d", g.Subject.Number), string(subj)))
	for _, it := range g.Items {
		fmt.Fprintf(w, "[#%d] %s %q (%s, status:%s)\n", it.Number, it.Type, it.Title, it.State, it.Status)
	}
	for _, p := range g.Packs {
		var floors []string
		for _, c := range p.Checks {
			floors = append(floors, fmt.Sprintf("%s %s (%g against %g)", c.Floor, c.Result, c.Value, c.Threshold))
		}
		fmt.Fprintf(w, "[%s] %s pack for #%d at %s, floors hold: %t: %s\n", p.ID, p.Authority, p.Story, short(p.HeadSHA), p.FloorsHold, strings.Join(floors, "; "))
		if p.Inspect != nil {
			fmt.Fprintf(w, "      inspect: %s (%s) on %s\n", p.Inspect.Disposition, p.Inspect.Model, short(p.Inspect.Head))
		}
		for _, c := range p.Criteria {
			ev := "no evidence in the pack: not proved"
			if len(c.Evidence) > 0 {
				var refs []string
				for _, a := range c.Evidence {
					refs = append(refs, a.Kind+" "+a.Path)
				}
				ev = "evidence " + strings.Join(refs, ", ")
			}
			v := ""
			if c.Verdict != "" {
				v = "; inspect: " + c.Verdict
			}
			fmt.Fprintf(w, "  [%s] criterion %d [%s] %s: %s%s\n", c.Cite(p.ID), c.ID, c.Kind, c.Text, ev, v)
		}
	}
	if r := g.Result; r != nil {
		fmt.Fprintf(w, "[R] %s on #%d\n%s\n", r.Field, r.Issue, brief.Untrusted(fmt.Sprintf("%s of #%d", r.Field, r.Issue), r.Text))
	}
	for _, s := range g.Snapshots {
		fmt.Fprintf(w, "[%s] measure snapshot %s (sha256 %s)\n", s.ID, s.File, short(s.SHA256))
	}
	for _, n := range g.Notes {
		fmt.Fprintf(w, "note: %s\n", n)
	}
	return nil
}

func findingsText(w io.Writer, rep *briefReport) error {
	for _, f := range rep.Findings {
		fmt.Fprintf(w, "%s\n", f)
	}
	if rep.Review != nil {
		for _, f := range rep.Review.Findings {
			fmt.Fprintf(w, "adversarial pass (%s): %q: %s\n", rep.Review.Model, f.Claim, f.Problem)
		}
	}
	return nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
