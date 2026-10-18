package cli

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"
	"github.com/pelletier/go-toml/v2"

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
	Protected []string `json:"protected,omitempty"` // CODEOWNERS paths it merged with no code owner's review
}

// requiredChecks checks that every required status check of the base
// branch's rules, other than the CODEOWNERS check (seats.GateCheck), is green
// on the head. It returns the checks it read, and the CODEOWNERS check's state
// ("" when it is not required), which only proofPass may accept red.
func requiredChecks(ctx context.Context, c *github.Client, owner, name string, n int, base, head string, runs []*gh.CheckRun) (want []string, gate string, err error) {
	rules, _, err := c.REST.Repositories.ListRulesForBranch(ctx, owner, name, base, nil)
	if err != nil && !github.NotFound(err) {
		return nil, "", Wrap(ExitExternal, err)
	}
	for _, r := range rules.GetRequiredStatusChecks() {
		for _, sc := range r.Parameters.RequiredStatusChecks {
			switch ctxName := sc.Context; {
			case ctxName == seats.GateCheck:
				gate = checkState(runs, ctxName)
			case !slices.Contains(want, ctxName):
				want = append(want, ctxName)
			}
		}
	}
	for _, w := range want {
		if state := checkState(runs, w); state != "success" {
			return nil, "", Errorf(ExitBlocked, "ship #%d: required check %s is %s on %s; never merge with a red required check", n, w, state, short(head))
		}
	}
	return want, gate, nil
}

// checkState is the state of the newest run named name ("missing" when none),
// with skipped counted as success, as GitHub counts it for a required check.
func checkState(runs []*gh.CheckRun, name string) string {
	for _, r := range runs { // the API lists the newest run of a name first; a rerun's success replaces a failure
		if r.GetName() != name {
			continue
		}
		switch {
		case r.GetStatus() != "completed":
			return r.GetStatus()
		case r.GetConclusion() == "skipped":
			return "success"
		default:
			return r.GetConclusion()
		}
	}
	return "missing"
}

// proofPass says whether PR pr may merge on proof at rung, or why not ("" when
// proof is off). The gate, the pack, and the required checks are checked
// before it; gate is the CODEOWNERS check's state, which may be red only for
// the CODEOWNERS paths the PR touches (Decision #593: those merge under the
// founder's recorded bypass). The policy and the integration branch are the
// base revision's config, never the PR's own or the checkout's: a PR could
// otherwise empty the narrow set it is judged by, or call main the
// integration branch.
func (e *Env) proofPass(ctx context.Context, c *github.Client, owner, name string, pr *gh.PullRequest, rung, gate string) (*ProofPass, string, error) {
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
	baseConfig := read(".aayudha/config.toml", base)
	if readErr != nil {
		return nil, "", Wrap(ExitExternal, readErr)
	}
	cfg, err := protectedConfig(baseConfig)
	if err != nil {
		return nil, "", err
	}
	a, dev := cfg.Autonomy, or(cfg.Branches.Dev, "dev")
	switch {
	case !a.Proof:
		return nil, "", nil
	case pr.GetBase().GetRef() != dev:
		return nil, fmt.Sprintf("it merges into %s, not the integration branch %s", pr.GetBase().GetRef(), dev), nil
	case !slices.Contains(a.ProofRungs, rung):
		return nil, fmt.Sprintf("the review rung at the head is %s, not one of %s ([autonomy] proof_rungs)", or(rung, "none recorded"), strings.Join(a.ProofRungs, ", ")), nil
	}
	files, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CommitFile, *gh.Response, error) {
		return c.REST.PullRequests.ListFiles(ctx, owner, name, pr.GetNumber(), &o)
	})
	if err != nil {
		return nil, "", Wrap(ExitExternal, err)
	}
	codeowners := identity.ParseCodeowners(read(".github/CODEOWNERS", base))
	pp := &ProofPass{Rung: rung}
	for _, f := range files {
		for _, p := range []string{f.GetFilename(), f.GetPreviousFilename()} {
			if p == "" {
				continue
			}
			if pat, ok := alwaysPerson(a.AlwaysPerson, p); ok {
				return nil, fmt.Sprintf("it touches %s, in the narrow set that always needs a person ([autonomy] always_person: %s)", p, pat), nil
			}
			if slices.Contains(a.AlwaysPersonConfig, p) {
				if s := changedSections(read(p, base), read(p, head), a.AlwaysPersonSections); len(s) > 0 {
					return nil, fmt.Sprintf("it changes [%s] in %s, which decides who may approve or bypass", strings.Join(s, "], ["), p), nil
				}
			}
			if strings.HasPrefix(p, ".github/workflows/") && (holdsSecrets(read(p, base)) || holdsSecrets(read(p, head))) {
				return nil, fmt.Sprintf("it touches %s, a workflow that reads a secret or names an environment", p), nil
			}
			if owners, _, _ := identity.Owners(codeowners, p); len(owners) > 0 && !slices.Contains(pp.Protected, p) {
				pp.Protected = append(pp.Protected, p)
			}
		}
	}
	if readErr != nil { // a file the narrow set judges could not be read
		return nil, "", Wrap(ExitExternal, readErr)
	}
	if gate != "" && gate != "success" && len(pp.Protected) == 0 {
		return nil, fmt.Sprintf("required check %s is %s, and the PR touches no CODEOWNERS path that would explain it", seats.GateCheck, gate), nil
	}
	return pp, "", nil
}

// protectedConfig is the config a revision's .aayudha/config.toml (text, ""
// when absent) sets over the defaults, with no user, local, env, or flag layer.
func protectedConfig(text string) (*config.Config, error) {
	dir, err := os.MkdirTemp("", "aayu-base-config-")
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if text != "" {
		if err := os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(text), 0o644); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
	}
	l, err := config.Load(config.Sources{RepoDir: dir})
	if err != nil {
		return nil, Errorf(ExitBlocked, "the base revision's .aayudha/config.toml does not load, so no merge on proof: %v", err)
	}
	return &l.Config, nil
}

// alwaysPerson matches p against the narrow set: a pattern ending in "/"
// covers a directory, any other is a path.Match glob.
func alwaysPerson(patterns []string, p string) (string, bool) {
	for _, pat := range patterns {
		if strings.HasSuffix(pat, "/") && strings.HasPrefix(p, pat) {
			return pat, true
		}
		if ok, _ := path.Match(pat, p); ok {
			return pat, true
		}
	}
	return "", false
}

// secretRef is any mention of the secrets context (secrets.X, secrets['X'],
// toJSON(secrets), secrets: inherit) or an environment key. It errs towards a
// person: a comment that says "secrets" counts too.
var secretRef = regexp.MustCompile(`\bsecrets\b|\benvironment\s*:`)

// holdsSecrets reports whether a workflow reads a secret or names an
// environment (an environment holds secrets and deployment rights).
func holdsSecrets(workflow string) bool { return secretRef.MatchString(workflow) }

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
