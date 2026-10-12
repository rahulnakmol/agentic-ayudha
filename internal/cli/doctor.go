package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
)

func init() { register("doctor", runDoctor) }

var originRepo = regexp.MustCompile(`github\.com[:/]([A-Za-z0-9-]+/[A-Za-z0-9._-]+?)(\.git)?$`)

// probe builds the doctor probe for this invocation.
func (e *Env) probe(ctx context.Context) *doctor.Probe {
	p := &doctor.Probe{
		Root: config.RepoRoot(e.Dir), Getenv: e.Getenv, LookPath: e.lookPath(), Exec: e.exec(),
		Secrets: e.SecretStore(), Version: e.Build.Version, Host: e.Host, TracePath: e.TracePath(), Repo: e.Repo,
	}
	p.Config, p.ConfigErr = e.Config()
	r := e.Resolver()
	p.Human = r.Human
	p.WhoAmI = func(ctx context.Context, t identity.Token) (identity.User, error) {
		return identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), t)
	}
	if p.Repo == "" && p.Root != "" {
		if out, err := p.Exec(ctx, p.Root, "git", "remote", "get-url", "origin"); err == nil {
			if m := originRepo.FindStringSubmatch(strings.TrimSpace(out)); m != nil {
				p.Repo = m[1]
			}
		}
	}
	p.Integration = func(ctx context.Context, name string) (integrations.Scope, error) {
		return providers.Check(ctx, e.integrationDeps(p.Repo), name)
	}
	if tok, err := r.Human(ctx); err == nil {
		p.API = func(ctx context.Context, path string, out any) error { return e.githubGet(ctx, tok, path, out) }
	}
	p.Latest = func(ctx context.Context) (string, error) {
		var rel struct {
			TagName string `json:"tag_name"`
		}
		err := getJSON(ctx, e.HTTP, e.githubAPI()+"/repos/"+releaseRepo+"/releases/latest", &rel)
		return rel.TagName, err
	}
	return p
}

// integrationDeps is what the integration adapters need from this run; repo
// is the current repository, the GitHub adapter's default.
func (e *Env) integrationDeps(repo string) providers.Deps {
	d := providers.Deps{Secrets: e.SecretStore(), Transport: e.HTTP.Transport, Repo: repo, Now: e.Now, Getenv: e.Getenv}
	if l, err := e.Config(); err == nil {
		d.Config = &l.Config
	}
	return d
}

func (e *Env) lookPath() func(string) (string, error) {
	if e.LookPath != nil {
		return e.LookPath
	}
	return exec.LookPath
}

func (e *Env) exec() func(ctx context.Context, dir, name string, args ...string) (string, error) {
	if e.Exec != nil {
		return e.Exec
	}
	return func(ctx context.Context, dir, name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		return string(out), err
	}
}

func (e *Env) githubGet(ctx context.Context, tok identity.Token, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.githubAPI()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok.Value)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func runDoctor(ctx context.Context, e *Env, in Values) error {
	p := e.probe(ctx)
	if in.Bool("all") {
		var repos []string
		if p.Config != nil {
			repos = p.Config.Config.Fleet.Repos
		}
		if len(repos) == 0 {
			return Errorf(ExitUsage, "doctor --all: no [fleet] repos in the user config (~/.config/aayudha/config.toml)")
		}
		rows := doctor.Fleet(ctx, repos, p.API)
		return e.Emit(rows, func(w io.Writer) error {
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", strings.ToUpper(string(r.Status)), r.Repo, r.Detail)
			}
			return tw.Flush()
		})
	}
	fix := ""
	if in.Bool("fix") {
		var err error
		if fix, err = e.fixCopies(); err != nil {
			return err
		}
	}
	results := doctor.Run(ctx, p)
	type report struct {
		Results  []doctor.Result `json:"results"`
		Blocking bool            `json:"blocking"`
		Fix      string          `json:"fix,omitempty"`
	}
	blocking := doctor.Blocking(results)
	if err := e.Emit(report{results, blocking, fix}, func(w io.Writer) error {
		if err := doctor.Render(w, results); err != nil {
			return err
		}
		if fix != "" {
			fmt.Fprintln(w, fix)
		}
		return nil
	}); err != nil {
		return err
	}
	if blocking {
		return Errorf(ExitFloor, "doctor: floor-blocking failures (see [blocking] above)")
	}
	return nil
}

// fixCopies re-syncs the skill copies and lock.toml with this binary's skills
// after showing the diff (doctor --fix). Host adapters (hooks, MCP
// registrations) are rewritten by aayu install, which a human runs.
func (e *Env) fixCopies() (string, error) {
	o, err := e.installOptions()
	if err != nil {
		return "--fix: nothing to re-sync (not in a repository)", nil
	}
	if _, ok, err := hosts.ReadLock(o.Root); err != nil || !ok {
		return "--fix: nothing to re-sync (no " + hosts.LockPath + "; aayu install installs skills)", err
	}
	o.SkillsOnly = true
	res, err := hosts.Plan(o)
	if err != nil {
		return "", Wrap(ExitBlocked, fmt.Errorf("doctor --fix: %w", err))
	}
	if len(res.Edits) == 0 {
		return "--fix: nothing to re-sync; skill copies match " + hosts.LockPath, nil
	}
	if !e.JSON {
		fmt.Fprint(e.Out, hosts.Render(res.Edits, upgrade.Unified))
	}
	if err := hosts.Apply(o, res.Edits, false); err != nil {
		return "", err
	}
	return fmt.Sprintf("--fix: re-synced %d file(s) with %s", len(res.Edits), hosts.LockPath), nil
}
