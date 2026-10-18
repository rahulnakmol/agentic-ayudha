package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// The security workflow (aayudha-security.yml, Decision #480): every push to
// dev, rolling, and main, and a nightly run, runs `verify --security --ci` on
// the commit. Each failing finding is one Bug labelled SecurityLabel, found
// again by its fingerprint; later runs comment on it instead of filing again,
// and an open one seen on rolling or main blocks the next promotion.
const (
	SecurityLabel    = workitems.LabelSecurity
	SecurityWorkflow = initrepo.SecurityWorkflow
	// SecurityFiler is the workflow token's identity, which files the Bugs and writes their events. Anyone with
	// triage can open an issue with a forged marker and apply the label, so only this identity's Bugs and
	// comments count.
	SecurityFiler = "github-actions[bot]"
	// MaxSecurityBugs is how many new Bugs one run files; the rest are listed
	// in the run's output and on the last Bug it filed.
	MaxSecurityBugs = 10
)

// SecurityBug is what one run did to one Bug.
type SecurityBug struct {
	Number      int    `json:"number"`
	Fingerprint string `json:"fingerprint"`
	Action      string `json:"action"` // filed, seen (commented: found again), gone (commented: not found), unchanged
}

// SecurityResult is what `verify --security` reports.
type SecurityResult struct {
	Branch   string           `json:"branch"`
	Head     string           `json:"head"`
	Base     string           `json:"base"` // "" when the secrets floor scanned the whole history of head
	Checks   []proof.Check    `json:"checks"`
	Hold     bool             `json:"floors_hold"`
	Findings []verify.Finding `json:"findings"`
	Bugs     []SecurityBug    `json:"bugs,omitempty"`
	Unfiled  []string         `json:"unfiled,omitempty"` // fingerprints over MaxSecurityBugs
}

func (e *Env) verifySecurity(ctx context.Context, root string, l *config.Loaded, since string) error {
	head, err := gitIn(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	res := SecurityResult{Head: head, Findings: []verify.Finding{}}
	if res.Branch, _ = gitIn(ctx, root, "rev-parse", "--abbrev-ref", "HEAD"); res.Branch == "" || res.Branch == "HEAD" {
		res.Branch = e.Getenv("GITHUB_REF_NAME")
	}
	// A push's before: all zeros for a new branch, and possibly gone after a
	// force push. Either way the secrets floor scans the whole history.
	if strings.Trim(since, "0") != "" {
		if b, err := gitIn(ctx, root, "rev-parse", "--verify", "--quiet", since+"^{commit}"); err == nil {
			res.Base = b
		}
	}
	home, err := os.MkdirTemp("", "aayudha-verify-home-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(home) }()
	work, err := os.MkdirTemp("", "aayudha-verify-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	r := &verify.Run{Root: root, Base: res.Base, Head: head, Floors: l.Config.Verify.Floors, Verify: l.Config.Verify, Now: e.Now, Env: e.Getenv, CI: e.CI,
		Sandbox: &verify.Sandbox{Getenv: e.Getenv, Home: home}, Work: work}
	rep := verify.Execute(ctx, r, verify.SecurityFloors())
	res.Checks, res.Hold = rep.Checks, rep.Hold
	if rep.Findings != nil {
		res.Findings = rep.Findings
	}
	if e.CI {
		repo, err := e.RepoName(ctx)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		c, _, err := e.readClient(ctx)
		if err != nil {
			return err
		}
		if res.Bugs, res.Unfiled, err = securityBugs(ctx, c, repo, res); err != nil {
			return Wrap(ExitExternal, err)
		}
		if err := e.waiveSecurity(ctx, repo, &res); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error { return renderSecurity(w, res) }); err != nil {
		return err
	}
	if !res.Hold {
		return Errorf(ExitFloor, "floor miss: %s", failing(res.Checks))
	}
	return nil
}

// waiveSecurity waives a failing floor when every one of its findings is on a
// security Bug with an accepted floor waiver naming that floor, so the run is
// green and main can take the tip, as the release hold already skips waived
// Bugs. A finding over the new-Bug cap, or planned by a dry run, has no Bug,
// so it is never waived.
func (e *Env) waiveSecurity(ctx context.Context, repo string, res *SecurityResult) error {
	bug := map[string]int{}
	for _, b := range res.Bugs {
		bug[b.Fingerprint] = b.Number
	}
	// Many findings share one fingerprint, and so one Bug: each Bug's waivers are read once per floor.
	type key struct {
		floor string
		bug   int
	}
	verdicts := map[key]string{} // the accepted waiver, or "" for none
	for i, c := range res.Checks {
		if c.Result != proof.Fail {
			continue
		}
		var by, bugs []string
		waived := true
		for _, f := range res.Findings {
			if f.Floor != c.Floor {
				continue
			}
			// A dry run plans new Bugs as #0: no issue exists, so none is waived.
			n, ok := bug[f.Fingerprint()]
			if !ok || n == 0 {
				waived = false
				break
			}
			w, seen := verdicts[key{c.Floor, n}]
			if !seen {
				ws, err := e.waivers(ctx, repo, n, nil)
				if err != nil {
					return err
				}
				one := []proof.Check{{Floor: c.Floor, Result: proof.Fail}}
				if verify.ApplyWaivers(one, n, ws) != nil {
					w = one[0].Waiver
				}
				verdicts[key{c.Floor, n}] = w
			}
			if w == "" {
				waived = false
				break
			}
			if !slices.Contains(by, w) {
				by = append(by, w)
				bugs = append(bugs, "#"+strconv.Itoa(n))
			}
		}
		if waived && len(by) > 0 {
			res.Checks[i].Result, res.Checks[i].Waiver = proof.Waived, strings.Join(by, ",")
			res.Checks[i].Detail += fmt.Sprintf(" (waived by %s on Bug %s)", strings.Join(by, ", "), strings.Join(bugs, ", "))
		}
	}
	res.Hold = (&proof.Pack{Checks: res.Checks}).FloorsHold()
	return nil
}

func renderSecurity(w io.Writer, res SecurityResult) error {
	scope := "the whole history"
	if res.Base != "" {
		scope = short(res.Base) + ".." + short(res.Head)
	}
	fmt.Fprintf(w, "verify --security %s at %s (secrets: %s)\n", res.Branch, short(res.Head), scope)
	for _, c := range res.Checks {
		fmt.Fprintf(w, "  %-8s %-8s %s\n", c.Floor, c.Result, c.Detail)
	}
	for _, b := range res.Bugs {
		fmt.Fprintf(w, "  Bug #%d %s: %s\n", b.Number, b.Action, b.Fingerprint)
	}
	for _, f := range res.Unfiled {
		fmt.Fprintf(w, "  not filed (over %d new Bugs a run): %s\n", MaxSecurityBugs, f)
	}
	return nil
}

// securityEvent is one record of a fingerprint on a branch: the Bug's body
// (its first sighting) and each marked comment, oldest first. The fingerprint
// and the branch are query-escaped (securityMarker), so a path or branch with
// a quote, an @, or a > in it cannot end the marker early.
var securityEvent = regexp.MustCompile(`<!-- aayudha:security (?:fingerprint="([^"]*)" )?(seen|gone)="([^"@]+)@([0-9a-f]+)" -->`)

// securityLead is the event a comment records: the marker it starts with.
// Text after it can quote a scanner's output, which is never an event.
var securityLead = regexp.MustCompile(`^` + securityEvent.String())

// prose is scanner text (a path, a rule's message) written into a Bug or a
// comment, with marker syntax broken so it can never read as an event.
func prose(s string) string { return strings.ReplaceAll(s, "<!--", "&lt;!--") }

// securityIssue is an open aayudha:security Bug and, per branch, the last
// event on it ("seen" or "gone") and the commit it was recorded at.
type securityIssue struct {
	Number      int
	Fingerprint string
	Floor       string
	Last        map[string][2]string // branch → {event, sha}
}

// SeenOn reports whether the finding was last seen, not cleared, on branch.
func (s securityIssue) SeenOn(branch string) bool { return s.Last[branch][0] == "seen" }

func openSecurityBugs(ctx context.Context, c *github.Client, repo string) ([]securityIssue, error) {
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return nil, err
	}
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{SecurityLabel}, ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	var out []securityIssue
	for _, is := range issues {
		if is.IsPullRequest() || is.GetUser().GetLogin() != SecurityFiler {
			continue
		}
		// An edited or malformed marker (an empty or blank fingerprint) is not one of ours.
		m := unescapeEvent(securityEvent.FindStringSubmatch(is.GetBody()))
		if m == nil || len(strings.Fields(m[1])) == 0 {
			continue
		}
		s := securityIssue{Number: is.GetNumber(), Fingerprint: m[1], Floor: strings.Fields(m[1])[0], Last: map[string][2]string{m[3]: {m[2], m[4]}}}
		cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
			return c.REST.Issues.ListComments(ctx, owner, name, s.Number, &gh.IssueListCommentsOptions{ListOptions: o})
		})
		if err != nil {
			return nil, err
		}
		for _, cm := range cs {
			// Only the workflow records events: anyone who can comment could otherwise mark a finding gone and
			// release the promotion hold.
			if cm.GetUser().GetLogin() != SecurityFiler {
				continue
			}
			if m := unescapeEvent(securityLead.FindStringSubmatch(cm.GetBody())); m != nil {
				s.Last[m[3]] = [2]string{m[2], m[4]}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// securityComment appends one event to a Bug. Each change of state is a new
// comment, never an edit of an older one: events read oldest first, so an
// edited seen before a later gone would leave the gone the last word. A rerun
// writes nothing, since the Bug's last event already matches.
func securityComment(ctx context.Context, c *github.Client, repo string, n int, marker, body string) error {
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return err
	}
	_, err = c.Write(ctx, actions.Action{Kind: "github.comment.security", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}, Payload: map[string]any{"marker": marker}},
		func(ctx context.Context) (any, error) {
			cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: gh.Ptr(body)})
			return cm.GetID(), err
		})
	return err
}

// A fingerprint is set only on the Bug's first sighting.
func securityMarker(fingerprint, event, branch, sha string) string {
	fp := ""
	if fingerprint != "" {
		fp = `fingerprint="` + url.QueryEscape(fingerprint) + `" `
	}
	return fmt.Sprintf(`<!-- aayudha:security %s%s="%s@%s" -->`, fp, event, url.QueryEscape(branch), sha)
}

// unescapeEvent decodes a securityEvent match's fingerprint and branch.
func unescapeEvent(m []string) []string {
	if m == nil {
		return nil
	}
	for _, i := range []int{1, 3} {
		if v, err := url.QueryUnescape(m[i]); err == nil {
			m[i] = v
		}
	}
	return m
}

// securityBugs files a Bug for each new failing finding, comments on the Bug
// of each finding seen again on a new branch or commit, and comments on each
// Bug last seen on this branch whose finding this run's floor no longer
// reports. A range scan (base set) never clears a secret: the secret may sit
// in older history it did not read.
func securityBugs(ctx context.Context, c *github.Client, repo string, res SecurityResult) ([]SecurityBug, []string, error) {
	owner, name, _ := github.SplitRepo(repo)
	open, err := openSecurityBugs(ctx, c, repo)
	if err != nil {
		return nil, nil, err
	}
	var findings []verify.Finding
	seen := map[string]bool{}
	for _, f := range res.Findings {
		if fp := f.Fingerprint(); !seen[fp] {
			seen[fp] = true
			findings = append(findings, f)
		}
	}
	var out []SecurityBug
	var unfiled []string
	filed, lastFiled := 0, 0
	for _, f := range findings {
		fp := f.Fingerprint()
		if i := slices.IndexFunc(open, func(s securityIssue) bool { return s.Fingerprint == fp }); i >= 0 {
			s := open[i]
			if s.Last[res.Branch] == [2]string{"seen", res.Head} {
				out = append(out, SecurityBug{Number: s.Number, Fingerprint: fp, Action: "unchanged"})
				continue
			}
			marker := securityMarker("", "seen", res.Branch, res.Head)
			body := fmt.Sprintf("%s\nFound again on `%s` at %s: %s (%s).\n", marker, res.Branch, res.Head, prose(f.Where), f.Level)
			if err := securityComment(ctx, c, repo, s.Number, marker, body); err != nil {
				return nil, nil, err
			}
			out = append(out, SecurityBug{Number: s.Number, Fingerprint: fp, Action: "seen"})
			continue
		}
		if filed == MaxSecurityBugs {
			unfiled = append(unfiled, fp)
			continue
		}
		t, _ := workitems.FindType("bug")
		title := fmt.Sprintf("Security: %s finding %s in %s", f.Floor, f.ID, f.Path)
		marker := securityMarker(fp, "seen", res.Branch, res.Head)
		body := workitems.Render(t, map[string]string{
			workitems.FieldSummary: fmt.Sprintf("`aayudha verify --security` (aayudha-security.yml, Decision #480) found a %s %s finding on `%s` at %s. Later runs comment here instead of filing again; an open security Bug seen on rolling or main blocks the next promotion until it is fixed or a person accepts a floor waiver (`aayu decide <this Bug> --waiver %s`).\n\n%s",
				f.Level, f.Floor, res.Branch, res.Head, f.Floor, marker),
			"reproduce": fmt.Sprintf("Check out %s and run `aayu verify --security`.", res.Head),
			"expected":  fmt.Sprintf("The %s floor reports no finding at this level ([verify.floors.%s] in CONSTRAINTS.md).", f.Floor, f.Floor),
			"actual":    prose(fmt.Sprintf("%s: %s", f.ID, f.Where)),
		})
		var n int
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo,
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues", repo)}, Payload: map[string]any{"title": title, "fingerprint": fp}},
			func(ctx context.Context) (any, error) {
				b, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: []string{"type:bug", "status:backlog", SecurityLabel}})
				n = b.GetNumber()
				return map[string]any{"number": n}, err
			}); err != nil {
			return nil, nil, err
		}
		filed, lastFiled = filed+1, n
		out = append(out, SecurityBug{Number: n, Fingerprint: fp, Action: "filed"})
	}
	if len(unfiled) > 0 && lastFiled > 0 {
		marker := fmt.Sprintf("<!-- aayudha:security-unfiled %s@%s -->", res.Branch, res.Head)
		body := fmt.Sprintf("%s\nThis run on `%s` at %s filed %d Bugs, its limit; these findings have no Bug yet and are filed by a later run once the open ones close:\n\n- %s\n",
			marker, res.Branch, res.Head, MaxSecurityBugs, prose(strings.Join(unfiled, "\n- ")))
		if _, err := c.UpsertComment(ctx, repo, lastFiled, marker, body, actions.Action{Kind: "github.comment.security"}); err != nil {
			return nil, nil, err
		}
	}
	ran := map[string]bool{}
	for _, ch := range res.Checks {
		ran[ch.Floor] = (ch.Result == proof.Pass || ch.Result == proof.Fail)
	}
	for _, s := range open {
		if seen[s.Fingerprint] || !ran[s.Floor] || !s.SeenOn(res.Branch) || (s.Floor == "secrets" && res.Base != "") {
			continue
		}
		marker := securityMarker("", "gone", res.Branch, res.Head)
		body := fmt.Sprintf("%s\nNot found on `%s` at %s. Close this Bug once it is gone from every branch it was seen on.\n", marker, res.Branch, res.Head)
		if err := securityComment(ctx, c, repo, s.Number, marker, body); err != nil {
			return nil, nil, err
		}
		out = append(out, SecurityBug{Number: s.Number, Fingerprint: s.Fingerprint, Action: "gone"})
	}
	return out, unfiled, nil
}

// securityGate holds a promotion (Decision #480) while an open aayudha:security
// Bug was last seen on a guarded branch (rolling or main) and no accepted
// floor waiver on it names its floor; and while the source tip being promoted
// has no green push run of the security workflow, when it is installed. A
// Bug seen only on dev holds nothing.
//
// Any workflow's token comments as github-actions[bot], so a write
// collaborator can forge a Bug's events; the run list cannot be forged (a run
// on the source branch at its tip runs that tip's own workflow file). The
// green run on the tip is what lets a promotion through, never an event alone.
func (e *Env) securityGate(ctx context.Context, c *github.Client, repo string, guarded []string, src, srcSHA string) error {
	open, err := openSecurityBugs(ctx, c, repo)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	var held []string
	for _, s := range open {
		i := slices.IndexFunc(guarded, s.SeenOn)
		if i < 0 {
			continue
		}
		// The fix travels the normal path: the source tip being promoted was scanned and no longer has the finding.
		if s.Last[src] == [2]string{"gone", srcSHA} {
			continue
		}
		ws, err := e.waivers(ctx, repo, s.Number, nil)
		if err != nil {
			return err
		}
		if len(verify.ApplyWaivers([]proof.Check{{Floor: s.Floor, Result: proof.Fail}}, s.Number, ws)) > 0 {
			continue
		}
		held = append(held, fmt.Sprintf("#%d (%s, last seen on %s at %s)", s.Number, s.Fingerprint, guarded[i], short(s.Last[guarded[i]][1])))
	}
	if len(held) > 0 {
		return Errorf(ExitBlocked, "release: open security Bug(s) %s; nothing was promoted. Fix each on %s and let %s scan its tip (a Bug marked gone at the tip being promoted no longer holds), or a person accepts a floor waiver on it (`aayu decide <bug> --waiver <floor>`)",
			strings.Join(held, ", "), src, SecurityWorkflow)
	}
	owner, name, _ := github.SplitRepo(repo)
	// By its file name, not a page of the workflow list: a repository can have more than 100 workflows.
	if _, resp, err := c.REST.Actions.GetWorkflowByFileName(ctx, owner, name, SecurityWorkflow); err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil // not installed: doctor reports it
		}
		return Wrap(ExitExternal, err)
	}
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, SecurityWorkflow, &gh.ListWorkflowRunsOptions{Branch: src, Event: "push", HeadSHA: srcSHA, ListOptions: gh.ListOptions{PerPage: 20}})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	for _, r := range runs.WorkflowRuns {
		if r.GetStatus() != "completed" {
			continue
		}
		if r.GetConclusion() == "success" {
			return nil
		}
		return Errorf(ExitBlocked, "release: %s ended %s on the %s tip %s (%s); nothing was promoted", SecurityWorkflow, r.GetConclusion(), src, short(srcSHA), r.GetHTMLURL())
	}
	return Errorf(ExitBlocked, "release: no completed %s push run on the %s tip %s, so it was never scanned; nothing was promoted (it runs on every push to %s)", SecurityWorkflow, src, short(srcSHA), src)
}
