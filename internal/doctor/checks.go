package doctor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
)

func checkBinary(ctx context.Context, p *Probe) Result {
	if strings.HasPrefix(p.Version, "0.0.0-dev") {
		return Result{Status: Warn, Detail: fmt.Sprintf("development build %s; install a signed release with install.sh", p.Version)}
	}
	if p.Latest == nil {
		return Result{Status: Warn, Detail: "could not check for a newer release (offline)"}
	}
	latest, err := p.Latest(ctx)
	if err != nil {
		return Result{Status: Warn, Detail: "could not check for a newer release: " + err.Error()}
	}
	if strings.TrimPrefix(latest, "v") != strings.TrimPrefix(p.Version, "v") {
		return Result{Status: Warn, Detail: fmt.Sprintf("aayu %s is installed; %s is available (aayu self-update)", p.Version, latest)}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("aayu %s is the latest release", p.Version)}
}

var gitVersion = regexp.MustCompile(`git version (\d+)\.(\d+)`)

func checkGit(ctx context.Context, p *Probe) Result {
	out, err := p.Exec(ctx, "", "git", "--version")
	if err != nil {
		return Result{Status: Fail, Blocking: true, Detail: "git not found; git >= 2.40 is required (ground rule 2)"}
	}
	m := gitVersion.FindStringSubmatch(out)
	if m == nil {
		return Result{Status: Fail, Blocking: true, Detail: fmt.Sprintf("cannot read the git version from %q", strings.TrimSpace(out))}
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 2 || (major == 2 && minor < 40) {
		return Result{Status: Fail, Blocking: true, Detail: fmt.Sprintf("git %d.%d is too old; git >= 2.40 is required for linked worktrees", major, minor)}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("git %d.%d (>= 2.40 required)", major, minor)}
}

func checkGitLFS(ctx context.Context, p *Probe) Result {
	out, err := p.Exec(ctx, "", "git", "lfs", "version")
	if err != nil || !strings.HasPrefix(out, "git-lfs/") {
		return Result{Status: Fail, Blocking: true, Detail: "git-lfs not found; it is required for ledger operations (https://git-lfs.com)"}
	}
	return Result{Status: Pass, Detail: strings.Fields(out)[0]}
}

func checkGH(_ context.Context, p *Probe) Result {
	if _, err := p.LookPath("gh"); err != nil {
		return Result{Status: Warn, Detail: "gh not found (optional: token source via `gh auth token`, and release provenance checks in self-update)"}
	}
	return Result{Status: Pass, Detail: "gh found (optional)"}
}

func checkToken(ctx context.Context, p *Probe) Result {
	tok, err := p.Human(ctx)
	if err != nil {
		return Result{Status: Warn, Detail: "no GitHub token: commands that call GitHub will fail (set AAYU_GITHUB_TOKEN or run gh auth login)"}
	}
	u, err := p.WhoAmI(ctx, tok)
	if err != nil {
		return Result{Status: Warn, Detail: fmt.Sprintf("token from %s did not authenticate: %v", tok.Source, err)}
	}
	r := Result{Status: Pass, Detail: fmt.Sprintf("token from %s -> %s", tok.Source, u.Login)}
	if len(u.Scopes) > 0 {
		r.Detail += " (scopes: " + strings.Join(u.Scopes, ", ") + ")"
	}
	if ws := identity.ScopeWarnings(u); len(ws) > 0 {
		r.Lines = ws
		if slices.ContainsFunc(u.Scopes, func(s string) bool {
			return s == "workflow" || s == "delete_repo" || strings.HasPrefix(s, "admin:")
		}) {
			r.Status = Warn
		}
	}
	return r
}

func checkBot(_ context.Context, p *Probe) Result {
	if p.Config == nil {
		return Result{Status: Warn, Detail: "bot identity: unknown (config does not load)"}
	}
	id := p.Config.Config.Identity
	switch identity.Configured(id) {
	case identity.KindApp:
		return Result{Status: Pass, Detail: fmt.Sprintf("bot identity: GitHub App %s[bot] (laptops mint tokens through the %s broker)", id.AppSlug, orDefault(id.Broker, "aayudha-token.yml"))}
	case identity.KindMachineUser:
		return Result{Status: Pass, Detail: "bot identity: machine user " + id.MachineUser}
	}
	return Result{Status: Warn, Detail: "bot identity: not configured; agent writes run under the human identity, tagged " + identity.HumanFallbackTag}
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func checkConfig(_ context.Context, p *Probe) Result {
	if p.ConfigErr != nil {
		return Result{Status: Fail, Blocking: true, Detail: "config is invalid", Lines: strings.Split(p.ConfigErr.Error(), "\n")}
	}
	if len(p.Config.Notices) > 0 {
		return Result{Status: Warn, Detail: "config loads, with notices", Lines: p.Config.Notices}
	}
	return Result{Status: Pass, Detail: "config is valid"}
}

func checkSchema(_ context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	changes, err := upgrade.Plan(p.Root, upgrade.Current, upgrade.Default)
	if errors.Is(err, upgrade.ErrNewer) {
		return Result{Status: Fail, Blocking: true, Detail: err.Error()}
	}
	if err != nil {
		return Result{Status: Fail, Blocking: true, Detail: err.Error()}
	}
	if len(changes) > 0 {
		var ls []string
		for _, c := range changes {
			ls = append(ls, fmt.Sprintf("%s: %s schema v%d -> v%d", c.Path, c.Kind, c.From, c.To))
		}
		return Result{Status: Warn, Detail: "repository files are at an older schema; run aayu upgrade", Lines: ls}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("repository schema current (config v%d)", upgrade.Current[upgrade.KindConfig])}
}

// hostEnv maps an environment variable each host sets to the host.
var hostEnv = []struct{ env, host string }{
	{"AMP_THREAD_ID", "amp"},
	{"CLAUDECODE", "claude"},
	{"CURSOR_AGENT", "cursor"},
	{"CURSOR_TRACE_ID", "cursor"},
}

// DetectHost returns the host from --host or the environment, and how it was found.
func DetectHost(flag string, getenv func(string) string) (host, how string) {
	if flag != "" {
		return flag, "--host"
	}
	for _, h := range hostEnv {
		if getenv(h.env) != "" {
			return h.host, h.env
		}
	}
	return "", ""
}

func checkHost(_ context.Context, p *Probe) Result {
	host, how := DetectHost(p.Host, p.Getenv)
	if host == "" {
		return Result{Status: Warn, Detail: "no host detected (Amp, Cursor, or Claude Code); pass --host when running outside one"}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("host %s (from %s)", host, how)}
}

func checkSecrets(_ context.Context, p *Probe) Result {
	var avail, unavail []string
	writable := false
	for _, s := range p.Secrets.Status() {
		if s.Available {
			avail = append(avail, s.Backend)
			writable = writable || s.Backend != "env"
		} else {
			unavail = append(unavail, s.Backend+" ("+s.Reason+")")
		}
	}
	r := Result{Status: Pass, Detail: "secrets backends: " + strings.Join(avail, ", ")}
	if len(unavail) > 0 {
		r.Lines = []string{"unavailable: " + strings.Join(unavail, "; ")}
	}
	if !writable {
		r.Status = Warn
		r.Detail += "; no writable backend (secrets can come only from AAYU_SECRET_* variables)"
	}
	return r
}

func checkIssueTypes(ctx context.Context, p *Probe) Result {
	if p.API == nil || p.Repo == "" {
		return Result{Status: Warn, Detail: "could not check issue-type support (no GitHub token or repository)"}
	}
	var repo struct {
		Owner struct {
			Type string `json:"type"`
		} `json:"owner"`
	}
	if err := p.API(ctx, "/repos/"+p.Repo, &repo); err != nil {
		return Result{Status: Warn, Detail: "could not read " + p.Repo + ": " + err.Error()}
	}
	if repo.Owner.Type == "Organization" {
		return Result{Status: Pass, Detail: "organization repository: GitHub issue types are available"}
	}
	return Result{Status: Pass, Detail: "personal repository: issue types are organization-only, so work-item types use the type:* label fallback (R1)"}
}

func checkIntegrations(_ context.Context, p *Probe) Result {
	if p.Config == nil || len(p.Config.Config.Integrations) == 0 {
		return Result{Status: Pass, Detail: "no integrations configured"}
	}
	names := make([]string, 0, len(p.Config.Config.Integrations))
	for n := range p.Config.Config.Integrations {
		names = append(names, n)
	}
	sort.Strings(names)
	r := Result{Status: Pass, Detail: fmt.Sprintf("%d integration(s); provider reachability and real key scope: not yet available (WP5.2)", len(names))}
	for _, n := range names {
		in := p.Config.Config.Integrations[n]
		if _, backend, err := p.Secrets.Get(in.Secret); err != nil {
			r.Status = Warn
			r.Lines = append(r.Lines, fmt.Sprintf("%s: secret %q is not set (aayu auth set-secret %s, or %s)", n, in.Secret, in.Secret, secrets.EnvVar(in.Secret)))
		} else {
			r.Lines = append(r.Lines, fmt.Sprintf("%s: secret %q in %s, configured scope %s", n, in.Secret, backend, in.Scope))
		}
	}
	return r
}

// OptionalTools are reported when missing; none is required.
var OptionalTools = []string{"node", "playwright", "axe", "semgrep", "gitleaks", "osv-scanner", "zap.sh", "k6", "lighthouse", "guidepup", "impeccable"}

func checkOptionalTools(_ context.Context, p *Probe) Result {
	var missing []string
	for _, t := range OptionalTools {
		if _, err := p.LookPath(t); err != nil {
			missing = append(missing, t)
		}
	}
	if len(missing) == 0 {
		return Result{Status: Pass, Detail: "all optional tools found"}
	}
	return Result{Status: Warn, Detail: fmt.Sprintf("%d optional tool(s) missing; floors that need them are skipped: %s", len(missing), strings.Join(missing, ", "))}
}

func checkWorktrees(ctx context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	out, err := p.Exec(ctx, p.Root, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return Result{Status: Warn, Detail: "could not list worktrees: " + err.Error()}
	}
	var leaks []string
	path := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "worktree ") {
			path = strings.TrimPrefix(l, "worktree ")
		}
		if strings.HasPrefix(l, "prunable") {
			leaks = append(leaks, path+" ("+strings.TrimSpace(strings.TrimPrefix(l, "prunable"))+")")
		}
	}
	if len(leaks) > 0 {
		return Result{Status: Warn, Detail: fmt.Sprintf("%d leaked worktree(s); remove them with git worktree prune (aayu gc, WP5.5)", len(leaks)), Lines: leaks}
	}
	return Result{Status: Pass, Detail: "no leaked worktrees"}
}

func checkTelemetry(_ context.Context, p *Probe) Result {
	if p.Config != nil && p.Config.Config.Telemetry.Disabled {
		return Result{Status: Pass, Detail: "traces disabled"}
	}
	d := "traces to " + p.TracePath + "; nothing leaves this machine"
	if p.Config != nil && p.Config.Config.Telemetry.OTLPEndpoint != "" {
		d = "traces to " + p.TracePath + " and, opted in, OTLP " + p.Config.Config.Telemetry.OTLPEndpoint
	}
	return Result{Status: Pass, Detail: d}
}
