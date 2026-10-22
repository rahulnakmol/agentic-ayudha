package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode"

	gh "github.com/google/go-github/v90/github"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// Merge on proof (Decision #593): a PR into the integration branch merges
// with no human pass when every required check is green, the merge gate's
// pack verifies with its floors holding, inspect's disposition at the head is
// merge with an accepted rung, and the PR touches nothing in the narrow set
// ([autonomy] always_person): who may approve or bypass, the gate itself, and
// releases and secrets. The streak, the never layers and the plan review stop
// blocking it; they are left to the sampled audit after merge.

// ProofPass records what a merge on proof rested on.
type ProofPass struct {
	Rung      string   `json:"rung"`                // the review rung at the head (inspect.Rung*)
	Checks    []string `json:"required_checks"`     // the base branch's required checks, all green
	Protected []string `json:"protected,omitempty"` // CODEOWNERS paths it merged with no code owner's approval
	Policy    string   `json:"policy"`              // the base branch's tip whose config and CODEOWNERS judged it
}

// branchTip is the commit a branch points at now.
func branchTip(ctx context.Context, c *github.Client, owner, name, branch string) (string, error) {
	ref, _, err := c.REST.Git.GetRef(ctx, owner, name, "heads/"+branch)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	return ref.GetObject().GetSHA(), nil
}

// shipPolicy is the policy ship judges a PR by (#645): the embedded defaults
// and the base branch's committed .aayudha/config.toml and LAYERS.md at its
// tip when ship read it, never the invoking checkout, the user config, AAYU_*
// variables or --set, which the agent running ship controls.
type shipPolicy struct {
	Tip    string // the base branch's tip it was read at
	Config *config.Config
	Layers verify.LayerGlobs
}

// basePolicy reads the shipPolicy at the tip of branch.
func basePolicy(ctx context.Context, c *github.Client, owner, name, branch string) (*shipPolicy, error) {
	tip, err := branchTip(ctx, c, owner, name, branch)
	if err != nil {
		return nil, err
	}
	read := func(p string) (string, error) { // "" when absent at the tip
		fc, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, p, &gh.RepositoryContentGetOptions{Ref: tip})
		if github.NotFound(err) {
			return "", nil
		}
		if err != nil {
			return "", Wrap(ExitExternal, err)
		}
		return fc.GetContent()
	}
	// The loader merges config.json and config.local.* into the same layers
	// as config.toml; ship reads only config.toml, so it refuses a base whose
	// policy could be split across them rather than judge a part of it.
	for _, f := range []string{".aayudha/config.json", ".aayudha/config.local.toml", ".aayudha/config.local.json"} {
		if text, err := read(f); err != nil {
			return nil, err
		} else if text != "" {
			return nil, Errorf(ExitBlocked, "%s holds %s at %s, and ship judges the pass only from .aayudha/config.toml; move its settings there", branch, f, short(tip))
		}
	}
	conf, err := read(".aayudha/config.toml")
	if err != nil {
		return nil, err
	}
	layers, err := read("LAYERS.md")
	if err != nil {
		return nil, err
	}
	l, err := config.Load(config.Sources{RepoTOML: []byte(conf), RepoSource: ".aayudha/config.toml at " + short(tip)})
	if err != nil {
		return nil, Errorf(ExitBlocked, "%s's config at %s does not load, so ship cannot judge the pass: %v", branch, short(tip), err)
	}
	return &shipPolicy{Tip: tip, Config: &l.Config, Layers: autonomy.LayersFrom(layers)}, nil
}

// shipFiles lists the paths PR pr changes, with each rename's old path: by
// GitHub's comparison of its captured base and head, which a push while ship
// runs cannot swap, or by the PR's own listing (every page) when the
// comparison is cut short (it lists at most 300 files).
func shipFiles(ctx context.Context, c *github.Client, owner, name string, pr *gh.PullRequest) ([]string, error) {
	cmp, _, err := c.REST.Repositories.CompareCommits(ctx, owner, name, pr.GetBase().GetSHA(), pr.GetHead().GetSHA(), &gh.ListOptions{PerPage: 1})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	files := cmp.Files
	if len(files) != pr.GetChangedFiles() {
		if files, err = github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CommitFile, *gh.Response, error) {
			return c.REST.PullRequests.ListFiles(ctx, owner, name, pr.GetNumber(), &o)
		}); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		// GitHub lists at most 3,000 of a PR's files: a list still short
		// could hide a never-layer path, so ship does not judge it.
		if len(files) != pr.GetChangedFiles() {
			return nil, Errorf(ExitBlocked, "ship #%d: GitHub lists %d of the PR's %d changed files, so ship cannot judge its footprint; split the PR", pr.GetNumber(), len(files), pr.GetChangedFiles())
		}
	}
	var out []string
	for _, f := range files {
		out = append(out, f.GetFilename())
		if old := f.GetPreviousFilename(); old != "" {
			out = append(out, old)
		}
	}
	return out, nil
}

// requiredChecks checks that every required status check of the base
// branch's rules (every page of them), other than the CODEOWNERS check
// (seats.GateCheck), is green on the head: its check run, from the app the
// rule names if it names one, and a commit status of the same name, as GitHub
// requires both. A rule it cannot read stops ship. It returns the checks it
// read, and the CODEOWNERS check's state ("" when it is not required; the
// worst of its states when several rules require it), which only proofPass
// may accept red.
func requiredChecks(ctx context.Context, c *github.Client, owner, name string, n int, base, head string, runs []*gh.CheckRun) (want []string, gate string, err error) {
	var checks []*gh.RuleStatusCheck
	for rule, err := range c.REST.Repositories.ListRulesForBranchIter(ctx, owner, name, base, &gh.ListOptions{PerPage: 100}) {
		if err != nil {
			return nil, "", Wrap(ExitExternal, err)
		}
		if r, ok := rule.(*gh.RequiredStatusChecksBranchRule); ok {
			checks = append(checks, r.Parameters.RequiredStatusChecks...)
		}
	}
	// Classic branch protection's required checks bind too, and the rules
	// endpoint does not list them; the branch read needs no admin right.
	br, _, err := c.REST.Repositories.GetBranch(ctx, owner, name, base, 1)
	if err != nil {
		return nil, "", Wrap(ExitExternal, err)
	}
	for _, rc := range br.GetProtection().GetRequiredStatusChecks().GetChecks() {
		checks = append(checks, &gh.RuleStatusCheck{Context: rc.Context, IntegrationID: rc.AppID})
	}
	statuses, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepoStatus, *gh.Response, error) {
		return c.REST.Repositories.ListStatuses(ctx, owner, name, head, &o)
	})
	if err != nil {
		return nil, "", Wrap(ExitExternal, err)
	}
	for _, sc := range checks {
		state := checkState(runs, statuses, sc)
		switch {
		case sc.Context == seats.GateCheck: // every rule that requires it counts: the worst state wins
			if gate == "" || gate == "success" || state != "success" && state != "failure" {
				gate = state
			}
		case state != "success":
			return nil, "", Errorf(ExitBlocked, "ship #%d: required check %s is %s on %s; never merge with a red required check", n, sc.Context, state, short(head))
		case !slices.Contains(want, sc.Context):
			want = append(want, sc.Context)
		}
	}
	return want, gate, nil
}

// checkState is the state of a required check: its newest check run from
// its app (any app when the rule names none), with skipped and neutral
// counted as success as GitHub counts them, and, for a rule that names no app, the newest
// commit status of the same name, which must pass too; "missing" when there
// is neither.
func checkState(runs []*gh.CheckRun, statuses []*gh.RepoStatus, sc *gh.RuleStatusCheck) string {
	// The newest run of the name in each check suite counts (a rerun's
	// success replaces a failure in its suite; the API lists the newest
	// first), and every suite's must pass: a run of another workflow or
	// event, a dispatch that skips the job say, cannot stand in for a
	// failing one.
	state, seen := "missing", map[int64]bool{}
	for _, r := range runs {
		if r.GetName() != sc.Context || sc.IntegrationID != nil && r.GetApp().GetID() != *sc.IntegrationID || seen[r.GetCheckSuite().GetID()] {
			continue
		}
		seen[r.GetCheckSuite().GetID()] = true
		st := r.GetConclusion()
		switch {
		case r.GetStatus() != "completed":
			st = r.GetStatus()
		case st == "skipped" || st == "neutral":
			st = "success"
		}
		if state == "missing" || state == "success" {
			state = st
		}
	}
	for _, st := range statuses { // newest first; a failing one fails the check even beside a pinned run
		if st.GetContext() == sc.Context {
			if st.GetState() != "success" || state == "missing" && sc.IntegrationID == nil {
				return st.GetState()
			}
			break
		}
	}
	return state
}

// proofPass says whether PR pr may merge on proof at rung, or why not ("" when
// proof is off). person reports a PR that needs a person per PR whatever else
// would pass it (the narrow set, or a file list GitHub truncated), so ship
// gives it no autonomous pass either. The gate, the pack, and the required
// checks are checked before it; gate is the CODEOWNERS check's state, which
// may be red only for the CODEOWNERS paths the PR touches (Decision #593:
// those merge under the founder's recorded bypass). The policy and the
// integration branch are the config, and CODEOWNERS, at the base branch's
// tip now (pp.Policy, which ship checks again just before it merges), never
// the PR's own or the checkout's: a PR could otherwise empty the narrow set
// it is judged by, or call main the integration branch, and a policy that
// tightened on the base while ship ran still applies.
func (e *Env) proofPass(ctx context.Context, c *github.Client, owner, name string, pr *gh.PullRequest, pol *shipPolicy, rung, gate string) (pp *ProofPass, why string, person bool, err error) {
	base, head := pr.GetBase().GetSHA(), pr.GetHead().GetSHA()
	var readErr error
	read := func(p, ref string) string { // "" when absent at ref
		fc, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, p, &gh.RepositoryContentGetOptions{Ref: ref})
		if err != nil {
			if !github.NotFound(err) && readErr == nil {
				readErr = err
			}
			return ""
		}
		s, _ := fc.GetContent()
		return s
	}
	policy, cfg := pol.Tip, pol.Config
	a, dev := cfg.Autonomy, or(cfg.Branches.Dev, "dev")
	// The narrow set needs a person on every path, the streak's included, so
	// it is judged before the proof flag: with proof off, person still says
	// whether the streak may merge the PR (#680).
	// The files are the comparison of the captured base and head, never the
	// live PR's listing, which a push or a base change during ship could
	// swap for another diff and back. Its first page lists every file, up
	// to 300.
	cmp, _, err := c.REST.Repositories.CompareCommits(ctx, owner, name, base, head, &gh.ListOptions{PerPage: 1})
	if err != nil {
		return nil, "", false, Wrap(ExitExternal, err)
	}
	files := cmp.Files
	if len(files) != pr.GetChangedFiles() {
		return nil, fmt.Sprintf("GitHub's comparison of %s...%s lists %d of the PR's %d changed files (it lists at most 300), so the narrow set cannot be judged", short(base), short(head), len(files), pr.GetChangedFiles()), true, nil
	}
	pp = &ProofPass{Rung: rung, Policy: policy}
	var changed []string
	for _, f := range files {
		for _, p := range []string{f.GetFilename(), f.GetPreviousFilename()} {
			switch {
			case p == "":
				continue
			case alwaysPerson(a.AlwaysPerson, p) != "":
				return nil, fmt.Sprintf("it touches %s, in the narrow set that always needs a person ([autonomy] always_person: %s)", p, alwaysPerson(a.AlwaysPerson, p)), true, nil
			case slices.Contains(a.AlwaysPersonConfig, p):
				if s := changedSections(read(p, base), read(p, head), a.AlwaysPersonSections); len(s) > 0 {
					return nil, fmt.Sprintf("it changes [%s] in %s, which decides who may approve or bypass, how a PR is verified, reviewed and proven, or where a credential is sent", strings.Join(s, "], ["), p), true, nil
				}
			case strings.HasPrefix(p, ".github/workflows/") && (holdsCredentials(read(p, base)) || holdsCredentials(read(p, head))):
				return nil, fmt.Sprintf("it touches %s, a workflow that holds a credential (a secret, the workflow token, an action it calls, an environment, or a write permission)", p), true, nil
			case path.Base(p) == "action.yml" || path.Base(p) == "action.yaml":
				return nil, fmt.Sprintf("it touches %s, an action's manifest, whose steps run with the token of the workflow that calls it", p), true, nil
			}
			changed = append(changed, p)
		}
	}
	// A symbolic link can point a policy file at a path the narrow set does
	// not judge, so a PR that adds, changes, removes, or retypes one needs a
	// person: the link is looked for in both trees.
	for _, rev := range []string{base, head} {
		tree, _, err := c.REST.Git.GetTree(ctx, owner, name, rev, true)
		if err != nil {
			return nil, "", false, Wrap(ExitExternal, err)
		}
		if tree.GetTruncated() {
			return nil, fmt.Sprintf("GitHub truncated the tree of %s, so its symbolic links cannot be judged", short(rev)), true, nil
		}
		for _, en := range tree.Entries {
			if en.GetMode() == "120000" && slices.Contains(changed, en.GetPath()) {
				return nil, fmt.Sprintf("it changes %s, a symbolic link, which can point a policy file at a path the narrow set does not judge", en.GetPath()), true, nil
			}
		}
	}
	if !a.Proof {
		if readErr != nil { // a file the narrow set judges could not be read
			return nil, "", false, Wrap(ExitExternal, readErr)
		}
		return nil, "", false, nil
	}
	// The merge gate judged the diff against the base the PR had when it ran;
	// a PR whose base was changed could have been judged against another
	// branch and restored, so its review proves nothing about this diff.
	events, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Timeline, *gh.Response, error) {
		return c.REST.Issues.ListIssueTimeline(ctx, owner, name, pr.GetNumber(), &o)
	})
	if err != nil {
		return nil, "", false, Wrap(ExitExternal, err)
	}
	if slices.ContainsFunc(events, func(t *gh.Timeline) bool { return t.GetEvent() == "base_ref_changed" }) {
		return nil, "its base branch was changed after it was opened, so the merge gate's review may have judged another diff", true, nil
	}
	// The CODEOWNERS check's failure is excused only for the reason the gate
	// itself, rerun here on the base's CODEOWNERS, gives: protected paths
	// with no CODEOWNER's approval. A pattern it cannot judge, or a failure
	// the gate would not give (the paths are approved), excuses nothing.
	reviews, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequestReview, *gh.Response, error) {
		return c.REST.PullRequests.ListReviews(ctx, owner, name, pr.GetNumber(), &o)
	})
	if err != nil {
		return nil, "", false, Wrap(ExitExternal, err)
	}
	var rv []identity.Review
	for _, r := range reviews {
		rv = append(rv, identity.Review{User: identity.Account{Login: r.GetUser().GetLogin(), Type: r.GetUser().GetType()}, State: r.GetState(), CommitID: r.GetCommitID()})
	}
	pp.Protected = identity.ProtectedPathsGate(identity.GateInput{Codeowners: read(".github/CODEOWNERS", policy), Changed: changed,
		Author: pr.GetUser().GetLogin(), HeadSHA: head, Reviews: rv, BotLogins: cfg.Identity.BotLogins}).Unapproved
	switch {
	case readErr != nil: // a file the narrow set judges could not be read
		return nil, "", false, Wrap(ExitExternal, readErr)
	case pr.GetBase().GetRef() != dev:
		return nil, fmt.Sprintf("it merges into %s, not the integration branch %s", pr.GetBase().GetRef(), dev), false, nil
	case !slices.Contains(a.ProofRungs, rung):
		return nil, fmt.Sprintf("the review rung at the head is %s, not one of %s ([autonomy] proof_rungs)", or(rung, "none recorded"), strings.Join(a.ProofRungs, ", ")), false, nil
	case gate != "" && gate != "success" && gate != "failure":
		return nil, fmt.Sprintf("required check %s is %s; only its failure for the CODEOWNERS paths a PR touches is accepted", seats.GateCheck, gate), false, nil
	case gate == "failure" && len(pp.Protected) == 0:
		return nil, fmt.Sprintf("required check %s is failure, and the gate, rerun on the base's CODEOWNERS, finds no CODEOWNERS path without an approval that would explain it", seats.GateCheck), false, nil
	}
	return pp, "", false, nil
}

// alwaysPerson is the narrow-set pattern p matches ("" for none): a pattern
// ending in "/" covers a directory, any other is a path.Match glob.
func alwaysPerson(patterns []string, p string) string {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, p); ok || strings.HasSuffix(pat, "/") && strings.HasPrefix(p, pat) {
			return pat
		}
	}
	return ""
}

// credentialText is any mention, in a decoded string of a workflow, of the
// secrets context (secrets.X, secrets['X'], toJSON(secrets), secrets:
// inherit) or GITHUB_TOKEN; the expression language ignores case.
var (
	credentialText = regexp.MustCompile(`(?i)\bsecrets\b|\bgithub_token\b`)
	expression     = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)
	githubContext  = regexp.MustCompile(`(?i)\bgithub\b`)
)

// readsToken reports whether an expression reads the workflow token: any
// use of the github context other than a named property that is not token
// (github.token, github . token, github['token'], github.*, toJSON(github),
// github || x, github as an argument all hold it).
func readsToken(expr string) bool {
	for _, m := range githubContext.FindAllStringIndex(expr, -1) {
		rest := strings.TrimLeft(expr[m[1]:], " \t\r\n")
		if !strings.HasPrefix(rest, ".") {
			return true
		}
		rest = strings.TrimLeft(rest[1:], " \t\r\n")
		n := strings.IndexFunc(rest, func(r rune) bool { return r != '_' && r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if n < 0 {
			n = len(rest)
		}
		if n == 0 || strings.EqualFold(rest[:n], "token") {
			return true
		}
	}
	return false
}

// holdsCredentials reports whether a workflow holds a credential: it reads a
// secret or the workflow token, calls an action or a reusable workflow (an
// action gets the token by default, as checkout and github-script do, and
// its own steps are not judged here), names an environment (which holds
// secrets and deployment rights), grants a write permission, or leaves a
// job's permissions to the repository's default, which may be write. It
// judges every decoded key and string of the parsed YAML, so quoting,
// escapes, anchors and merge keys change nothing, and a workflow it cannot
// parse, or one of more than one document, counts as holding one. "" is no
// workflow.
func holdsCredentials(workflow string) bool {
	if workflow == "" {
		return false
	}
	dec := yaml.NewDecoder(strings.NewReader(workflow))
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		return true
	}
	var more any
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return true
	}
	held := false
	var walk func(key string, x any)
	text := func(key, s string) {
		if credentialText.MatchString(s) || strings.EqualFold(s, "write") || strings.EqualFold(s, "write-all") {
			held = true
		}
		for _, m := range expression.FindAllStringSubmatch(s, -1) {
			held = held || readsToken(m[1])
		}
		if key == "if" { // an if is an expression without ${{ }}
			held = held || readsToken(s)
		}
	}
	walk = func(key string, x any) {
		switch x := x.(type) {
		case map[string]any:
			for k, e := range x {
				if strings.EqualFold(k, "environment") || strings.EqualFold(k, "uses") {
					held = true
				}
				text("", k)
				walk(strings.ToLower(k), e)
			}
		case []any:
			for _, e := range x {
				walk(key, e)
			}
		case string:
			text(key, x)
		case nil, bool, int, int64, uint64, float64:
		default:
			held = true // a shape it cannot judge
		}
	}
	walk("", v)
	if held {
		return true
	}
	if v["permissions"] != nil {
		return false
	}
	jobs, ok := v["jobs"].(map[string]any)
	if !ok {
		return true
	}
	for _, j := range jobs {
		if job, ok := j.(map[string]any); !ok || job["permissions"] == nil {
			return true
		}
	}
	return false
}

// changedSections lists the TOML tables in names whose parsed values differ
// between two versions of a file, however they are written (a [table], a
// dotted key, or an inline table). A version that does not parse counts as
// changing every one of them.
func changedSections(before, after string, names []string) []string {
	var b, a map[string]any
	errB, errA := toml.Unmarshal([]byte(before), &b), toml.Unmarshal([]byte(after), &a)
	// The loader splits a quoted key on its dots ("approvals.approvers" is
	// approvals.approvers) and matches tables whatever their case, so a
	// table is every top-level key whose first segment names it.
	section := func(m map[string]any, n string) map[string]any {
		out := map[string]any{}
		for k, v := range m {
			if seg, _, _ := strings.Cut(k, "."); strings.EqualFold(seg, n) {
				out[k] = v
			}
		}
		return out
	}
	var out []string
	for _, n := range names {
		if errB != nil || errA != nil || !reflect.DeepEqual(section(b, n), section(a, n)) {
			out = append(out, n)
		}
	}
	return out
}
