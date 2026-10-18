package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("doctor", runDoctor) }

var originRepo = regexp.MustCompile(`github\.com[:/]([A-Za-z0-9-]+/[A-Za-z0-9._-]+?)(\.git)?$`)

// probe builds the doctor probe for this invocation.
func (e *Env) probe(ctx context.Context) *doctor.Probe {
	p := &doctor.Probe{
		Root: config.RepoRoot(e.Dir), Getenv: e.Getenv, LookPath: e.lookPath(), Exec: e.exec(), Combined: e.Exec,
		Secrets: e.SecretStore(), Version: e.Build.Version, Host: e.Host, TracePath: e.TracePath(), Repo: e.Repo, Verbose: e.Verbose,
	}
	if p.Combined == nil {
		p.Combined = agentenv.CombinedExec
	}
	p.Config, p.ConfigErr = e.Config()
	r := e.Resolver()
	p.Human = r.Human
	p.CursorMCPHuman = e.cursorMCPResolver().Human
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
	p.Capabilities = func(ctx context.Context, name string) ([]integrations.Check, error) {
		return providers.Capabilities(ctx, e.integrationDeps(p.Repo), name)
	}
	if tok, err := r.Human(ctx); err == nil {
		p.API = func(ctx context.Context, path string, out any) error { return e.githubGet(ctx, tok, path, out) }
	}
	if p.Root != "" {
		_, ok, err := hosts.ReadLock(p.Root)
		switch found := hosts.AayudhaConfig(p.Root); {
		case err == nil && ok:
			p.HostConfig = e.hostConfigDrift
		case len(found) > 0:
			// The lock says what install wrote; without it, aayudha's entries cannot be checked.
			if err == nil {
				err = errors.New(hosts.LockPath + " is missing")
			}
			p.HostConfig = func(context.Context) ([]string, error) {
				return nil, fmt.Errorf("%s carry aayudha's hooks or MCP server, but %w: run `aayu install --apply` to rewrite them", strings.Join(found, ", "), err)
			}
		}
	}
	p.Reachable = func(ctx context.Context) []identity.Reach { return e.reachable(ctx, p.Repo) }
	p.Agent = e.agentSignal()
	// Copilot's stored login is not readable, and a data-residency tenant has
	// its own endpoint: then aayu does not ask, rather than send its token to
	// the public one.
	if hosts.CopilotToken(e.Getenv) != "" && hosts.CopilotOnGitHubCom(e.Getenv) {
		p.CopilotModels = e.copilotEnabledModels
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
		if msg, err := e.fixDelegateEnv(); err != nil {
			return err
		} else if msg != "" {
			fix += "\n" + msg
		}
	}
	pr := e.progress("Checking tools, tokens, identity, config, and hosts")
	results := doctor.Run(ctx, p)
	pr.Done(ui.StatusInfo, fmt.Sprintf("%d checks", len(results)))
	type report struct {
		Results  []doctor.Result `json:"results"`
		Blocking bool            `json:"blocking"`
		Fix      string          `json:"fix,omitempty"`
	}
	blocking := doctor.Blocking(results)
	if err := e.Emit(report{results, blocking, fix}, func(w io.Writer) error {
		if look := e.Look(); look.Human() {
			renderDoctorHuman(look, results)
		} else if err := doctor.Render(w, results); err != nil {
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

// renderDoctorHuman is doctor for a person: a table (Rich) or aligned columns
// (Plain) whose first column is always the status word, then a one-line tally.
func renderDoctorHuman(p *ui.Printer, rs []doctor.Result) {
	rows := make([][]string, 0, len(rs))
	tally := map[doctor.Status]int{}
	for _, r := range rs {
		tally[r.Status]++
		detail := r.Detail
		if r.Blocking && r.Status == doctor.Fail {
			detail += " [blocking]"
		}
		for i, l := range r.Lines {
			if i == 3 { // a person gets the gist; --json and a pipe keep every line
				detail += fmt.Sprintf("\n  %s and %d more (aayu doctor --json)", p.Sym("…", "..."), len(r.Lines)-3)
				break
			}
			detail += "\n  " + l
		}
		rows = append(rows, []string{p.Mark(strings.ToUpper(string(r.Status))), r.ID, detail})
	}
	p.Printf("%s", p.Table([]string{"Status", "Check", "Detail"}, rows))
	p.Println(fmt.Sprintf("%d pass, %d warn, %d fail, %d not yet available", tally[doctor.Pass], tally[doctor.Warn], tally[doctor.Fail], tally[doctor.NA]))
}

// hostConfigDrift plans aayu install as this binary would run it and names
// the executable host config files that would change.
func (e *Env) hostConfigDrift(ctx context.Context) ([]string, error) {
	o, err := e.installOptions()
	if err != nil {
		return nil, err
	}
	if o.Binary, err = e.binaryPath(ctx); err != nil {
		return nil, err
	}
	res, err := hosts.Plan(o)
	if err != nil {
		return nil, err
	}
	return append(hosts.ConfigDrift(res.Edits), res.Unmanaged...), nil
}

// copilotEnabledModels asks Copilot which models the account it signs in with may
// pick. The token goes only to Copilot's own endpoint.
func (e *Env) copilotEnabledModels(ctx context.Context) (map[verbs.Family][]string, error) {
	tok := hosts.CopilotToken(e.Getenv)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hosts.CopilotModelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", hosts.CopilotModelsURL, resp.Status)
	}
	return hosts.CopilotFamilies(resp.Body)
}
