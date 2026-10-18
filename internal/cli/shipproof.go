package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
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
	state := "missing"
	for _, r := range runs { // the API lists the newest run of a name first; a rerun's success replaces a failure
		if r.GetName() != sc.Context || sc.IntegrationID != nil && r.GetApp().GetID() != *sc.IntegrationID {
			continue
		}
		switch {
		case r.GetStatus() != "completed":
			state = r.GetStatus()
		case r.GetConclusion() == "skipped" || r.GetConclusion() == "neutral":
			state = "success"
		default:
			state = r.GetConclusion()
		}
		break
	}
	if sc.IntegrationID != nil {
		return state
	}
	for _, st := range statuses { // newest first
		if st.GetContext() == sc.Context {
			if st.GetState() != "success" || state == "missing" {
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
// integration branch are the base revision's config, never the PR's own or
// the checkout's: a PR could otherwise empty the narrow set it is judged by,
// or call main the integration branch.
func (e *Env) proofPass(ctx context.Context, c *github.Client, owner, name string, pr *gh.PullRequest, rung, gate string) (pp *ProofPass, why string, person bool, err error) {
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
	l, err := config.Load(config.Sources{RepoTOML: []byte(read(".aayudha/config.toml", base)), RepoSource: ".aayudha/config.toml at " + short(base)})
	switch {
	case readErr != nil:
		return nil, "", false, Wrap(ExitExternal, readErr)
	case err != nil:
		return nil, "", false, Errorf(ExitBlocked, "the base revision's config does not load, so ship cannot judge the narrow set: %v", err)
	}
	a, dev := l.Config.Autonomy, or(l.Config.Branches.Dev, "dev")
	if !a.Proof {
		return nil, "", false, nil
	}
	files, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CommitFile, *gh.Response, error) {
		return c.REST.PullRequests.ListFiles(ctx, owner, name, pr.GetNumber(), &o)
	})
	if err != nil {
		return nil, "", false, Wrap(ExitExternal, err)
	}
	if len(files) != pr.GetChangedFiles() {
		return nil, fmt.Sprintf("GitHub listed %d of its %d changed files (it lists at most 3,000), so the narrow set cannot be judged", len(files), pr.GetChangedFiles()), true, nil
	}
	pp = &ProofPass{Rung: rung}
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
					return nil, fmt.Sprintf("it changes [%s] in %s, which decides who may approve or bypass", strings.Join(s, "], ["), p), true, nil
				}
			case strings.HasPrefix(p, ".github/workflows/") && (holdsCredentials(read(p, base)) || holdsCredentials(read(p, head))):
				return nil, fmt.Sprintf("it touches %s, a workflow that holds a credential (a secret, the workflow token, an environment, or a write permission)", p), true, nil
			}
			changed = append(changed, p)
		}
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
	pp.Protected = identity.ProtectedPathsGate(identity.GateInput{Codeowners: read(".github/CODEOWNERS", base), Changed: changed,
		Author: pr.GetUser().GetLogin(), HeadSHA: head, Reviews: rv, BotLogins: l.Config.Identity.BotLogins}).Unapproved
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

// credentialRef is, in a workflow's canonical JSON, any mention of the
// secrets context (secrets.X, secrets['X'], toJSON(secrets), secrets: inherit;
// the expression language ignores case), the workflow token (github.token,
// github['token'], GITHUB_TOKEN), an environment
// key, or a write permission.
var credentialRef = regexp.MustCompile(`(?i)\bsecrets\b|github\.token|github\s*\[\s*['"]token['"]\s*\]|\bgithub_token\b|"environment":|"write(-all)?"`)

// holdsCredentials reports whether a workflow holds a credential: it reads a
// secret or the workflow token, names an environment (which holds secrets
// and deployment rights), grants a write permission, or leaves a job's
// permissions to the repository's default, which may be write. It judges the
// parsed YAML, so quoting, escapes, anchors and merge keys change nothing,
// and a workflow it cannot parse counts as holding one. "" is no workflow.
func holdsCredentials(workflow string) bool {
	if workflow == "" {
		return false
	}
	var v map[string]any
	if err := yaml.Unmarshal([]byte(workflow), &v); err != nil {
		return true
	}
	b, err := json.Marshal(v)
	if err != nil || credentialRef.Match(b) {
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
	var out []string
	for _, n := range names {
		if errB != nil || errA != nil || !reflect.DeepEqual(b[n], a[n]) {
			out = append(out, n)
		}
	}
	return out
}
