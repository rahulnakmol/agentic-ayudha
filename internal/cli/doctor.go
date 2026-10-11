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
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
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
	results := doctor.Run(ctx, p)
	fix := ""
	if in.Bool("fix") {
		// --fix re-syncs skill copies and host adapters after showing a diff;
		// both arrive with install (WP3.2), so there is nothing to change yet.
		fix = "--fix: nothing to re-sync; skill copies and host adapters are not yet available (WP3.2)"
	}
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
