package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/hostcall"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
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

// checkApprovals reports whether a recorded approver's reply can approve a
// batch (Decision #488): only when agents write as a bot and no process an
// agent runs can reach an approver's GitHub credential. It fails when this
// process can, naming where the credential was found and whose it is, never
// its value. The failure does not block: comment approvals are refused from
// such a session anyway (commentApprovalsOff), and the device code still works.
func checkApprovals(ctx context.Context, p *Probe) Result {
	if p.Config == nil {
		return Result{Status: Warn, Detail: "approvals: unknown (config does not load)"}
	}
	cfg := p.Config.Config
	ap := cfg.Approvals
	var reach []identity.Reach
	if p.Reachable != nil {
		reach = p.Reachable(ctx)
	}
	var lines []string
	for _, r := range reach {
		who := "@" + r.Login
		if r.Login == "" {
			who = "an account GitHub could not confirm just now"
		}
		lines = append(lines, fmt.Sprintf("%s -> %s", r.Source, who))
	}
	device := "every batch needs the device code"
	if slices.Contains(ap.DeviceFor, "all") { // no reply approves anything, so a reachable credential cannot forge one
		return Result{Status: Pass, Lines: lines, Detail: "approvals: replies are off (`[approvals] device_for = [\"all\"]`), so " + device}
	}
	// With no approvers recorded, no reply approves anything either, so an
	// unconfirmed credential (a git credential helper with nothing stored on a
	// clean macOS or Windows machine) cannot forge one: the row only warns.
	if ex := identity.Exposed(reach, ap.Approvers); len(ap.Approvers) > 0 && len(ex) > 0 {
		var who []string
		for _, r := range ex {
			if r.Login == "" {
				who = append(who, fmt.Sprintf("an unconfirmed account (%s)", r.Source))
				continue
			}
			who = append(who, fmt.Sprintf("@%s (%s)", r.Login, r.Source))
		}
		where := "this process"
		if p.Agent != "" {
			where = "this agent session (" + p.Agent + ")"
		}
		return Result{Status: Fail, Lines: lines, Detail: fmt.Sprintf("approvals: %s can reach an approver's GitHub credential: %s. An agent here could write an `/aayu approve` reply in their name, so batches from here need the device code. Remove it from every agent's environment (docs/guide/how-trust.md, Approving by reply)", where, strings.Join(who, ", "))}
	}
	if p.Agent != "" {
		for _, r := range reach {
			if r.Login != "" && !identity.IsBot(identity.Account{Login: r.Login}, cfg.Identity.BotLogins) {
				lines = append(lines, fmt.Sprintf("this agent session can act as @%s, which is not a bot identity", r.Login))
			}
		}
	}
	switch {
	case len(ap.Approvers) == 0:
		return Result{Status: Warn, Lines: lines, Detail: "approvals: no `[approvals] approvers`, so " + device}
	case identity.Configured(cfg.Identity) == "":
		return Result{Status: Warn, Lines: lines, Detail: "approvals: no bot identity, so agents write as a person and " + device}
	case identity.Configured(cfg.Identity) != identity.KindApp:
		return Result{Status: Warn, Lines: lines, Detail: "approvals: the bot is a machine user, and the aayudha-approve workflow acts only as the aayudha GitHub App, so " + device}
	}
	keep := "one-way doors need the device code"
	if !slices.Contains(ap.DeviceFor, "one-way") {
		keep = "no door needs the device code"
	}
	return Result{Status: Pass, Lines: lines, Detail: fmt.Sprintf("approvals: a reply from %s approves a batch; %s", strings.Join(ap.Approvers, ", "), keep)}
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
	changes, err := upgrade.Plan(p.Root, upgrade.Current, upgrade.Default, p.CI)
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

// DetectHost returns the host from --host or the environment, and how it was
// found: the first agent marker that names a host (hosts.AgentMarkers), else
// CURSOR_TRACE_ID, which Cursor sets in its terminal for a person too, so it
// names the host without marking an agent.
func DetectHost(flag string, getenv func(string) string) (host, how string) {
	if flag != "" {
		return flag, "--host"
	}
	for _, m := range hosts.AgentMarkers {
		if m.Host != "" && getenv(m.Env) != "" {
			return string(m.Host), m.Env
		}
	}
	if getenv("CURSOR_TRACE_ID") != "" {
		return string(verbs.HostCursor), "CURSOR_TRACE_ID"
	}
	return "", ""
}

func checkHost(_ context.Context, p *Probe) Result {
	host, how := DetectHost(p.Host, p.Getenv)
	if host == "" {
		return Result{Status: Warn, Detail: "no host detected (Amp, Claude Code, OpenAI Codex, GitHub Copilot, or Cursor); pass --host when running outside one"}
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

// checkIntegrations reports every adapter this binary carries and every
// configured integration (WP1.3, WP5.2): an unconfigured adapter is "not
// configured", never a failure; a configured one is resolved (a write-scoped
// key is refused before any call) and its provider is asked who the key
// belongs to and what it may do.
func checkIntegrations(ctx context.Context, p *Probe) Result {
	var configured map[string]config.Integration
	if p.Config != nil {
		configured = p.Config.Config.Integrations
	}
	var names, extra []string
	for _, k := range providers.Known {
		names = append(names, k.Name)
	}
	for n := range configured {
		if !slices.Contains(names, n) {
			extra = append(extra, n)
		}
	}
	sort.Strings(extra)
	r := Result{Status: Pass}
	worse := func(s Status) {
		if s == Fail || (s == Warn && r.Status == Pass) {
			r.Status = s
		}
	}
	reachable := 0
	for _, n := range append(names, extra...) {
		in, ok := configured[n]
		if !ok {
			k, _ := providers.Find(n)
			r.Lines = append(r.Lines, fmt.Sprintf("%s: not configured; set [integrations.%s] for %s", n, n, k.Purpose))
			continue
		}
		if p.Integration == nil {
			worse(Warn)
			r.Lines = append(r.Lines, n+": not checked")
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		sc, err := p.Integration(cctx, n)
		cancel()
		switch {
		case errors.Is(err, integrations.ErrNoAdapter):
			r.Lines = append(r.Lines, fmt.Sprintf("%s: configured; no adapter in this binary yet, so not checked", n))
		case errors.Is(err, secrets.ErrWriteScoped):
			worse(Fail)
			r.Lines = append(r.Lines, fmt.Sprintf("%s: scope = %q: read verbs refuse this key, so it was not sent; configure a read-only key", n, in.Scope))
		case errors.Is(err, integrations.ErrNotConfigured):
			worse(Warn)
			r.Lines = append(r.Lines, fmt.Sprintf("%s: %v", n, err))
		case err != nil:
			worse(Fail)
			// An adapter that can explain the failure says what to fix.
			if checks := capabilities(ctx, p, n); len(checks) > 0 && !checks[0].OK && checks[0].Fix != "" {
				r.Lines = append(r.Lines, fmt.Sprintf("%s: %s. Fix: %s", n, checks[0].Detail, checks[0].Fix))
				continue
			}
			r.Lines = append(r.Lines, fmt.Sprintf("%s: unreachable: %v", n, err))
		case len(sc.Write) > 0:
			worse(Fail)
			r.Lines = append(r.Lines, fmt.Sprintf("%s: reachable, but the key is write-capable (%s per %s); read verbs refuse it", n, strings.Join(sc.Write, ", "), sc.Endpoint))
		case sc.Verified:
			reachable++
			r.Lines = append(r.Lines, fmt.Sprintf("%s: reachable as %s; read-only scope verified (%s per %s)", n, sc.Account, strings.Join(sc.Scopes, ", "), sc.Endpoint))
			for _, ch := range capabilities(ctx, p, n) {
				what := fmt.Sprintf("  %s, %s (%s", n, ch.Capability, strings.Join(ch.Scopes, ", "))
				if len(ch.Verbs) > 0 {
					what += "; used by " + strings.Join(ch.Verbs, ", ")
				}
				if ch.OK {
					r.Lines = append(r.Lines, what+"): works: "+ch.Detail)
					continue
				}
				worse(Fail)
				r.Lines = append(r.Lines, what+"): does not work: "+ch.Detail+". Fix: "+ch.Fix)
			}
		default:
			reachable++
			r.Lines = append(r.Lines, fmt.Sprintf("%s: reachable as %s; the provider does not report this key's scope, so configured scope %s is trusted", n, sc.Account, in.Scope))
		}
	}
	r.Detail = fmt.Sprintf("%d of %d configured integration(s) reachable with a read-only key", reachable, len(configured))
	if len(configured) == 0 {
		r.Detail = "no integrations configured"
	}
	return r
}

// capabilities asks the integration's adapter to try each capability with a
// cheap read; nil when doctor cannot or the adapter has no checks.
func capabilities(ctx context.Context, p *Probe, name string) []integrations.Check {
	if p.Capabilities == nil {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	checks, err := p.Capabilities(cctx, name)
	if err != nil {
		return nil
	}
	return checks
}

// checkProviderMCP reads every PostHog and Sentry MCP server a host has
// registered (Decision #449) and says what to change so it reads only, is
// pinned to one project, and keeps no key in a file the repository shares.
// aayudha's pre-tool hook refuses their writes either way, so a loose setting
// is a warning; a shared key fails.
func checkProviderMCP(_ context.Context, p *Probe) Result {
	home := p.Getenv("HOME")
	servers := hosts.ProviderServers(p.Root, home, hosts.CodexHome(home, p.Getenv), hosts.CopilotHome(home, p.Getenv))
	if len(servers) == 0 {
		return Result{Status: Pass, Detail: "no PostHog or Sentry MCP server registered with a host"}
	}
	r := Result{Status: Pass}
	var ok []string
	for _, s := range servers {
		provider, findings := hostcall.Audit(hostcall.Server{URL: s.URL, Command: s.Command, Args: s.Args, Env: s.Env, Headers: s.Headers, Committed: s.Committed})
		where := fmt.Sprintf("%s %q (%s, %s)", provider, s.Name, s.Host, s.File)
		if len(findings) == 0 {
			ok = append(ok, where)
			continue
		}
		for _, f := range findings {
			if f.Secret {
				r.Status = Fail
			} else if r.Status == Pass {
				r.Status = Warn
			}
			r.Lines = append(r.Lines, fmt.Sprintf("%s: %s. Fix: %s", where, f.Problem, f.Fix))
		}
	}
	for _, w := range ok {
		r.Lines = append(r.Lines, w+": set up to read only")
	}
	r.Detail = fmt.Sprintf("%d PostHog or Sentry MCP server(s); aayudha's pre-tool hook lets them read and refuses their writes", len(servers))
	if r.Status != Pass {
		r.Detail += "; the lines say what to tighten"
	}
	return r
}

// OptionalTool is a program doctor looks for on PATH and what needs it.
type OptionalTool struct {
	Name  string
	Use   string
	Floor bool // a verify floor runs it: without it the floor blocks, it never passes (WP4.3)
}

// OptionalTools are reported when missing; none is required to run aayudha.
// Playwright, axe-core, Lighthouse's runtime, and Guidepup are npm packages
// the web and screen-reader floors load through node. The dast floor's
// container runtime, Docker or Podman, is the dast check's.
var OptionalTools = []OptionalTool{
	{"node", "a11y, perf, and screen-reader floors: Playwright, axe-core, Guidepup", true},
	{"semgrep", "sast floor", true},
	{"gitleaks", "secrets floor", true},
	{"osv-scanner", "deps floor", true},
	{"k6", "api floor", true},
	{"lighthouse", "perf floor", true},
	{"tesseract", "OCR in the redaction pass; without it the pass scans the DOM only", false},
	{"impeccable", "review lane B", false},
	{"duckdb", "measure, optional", false},
}

func checkOptionalTools(_ context.Context, p *Probe) Result {
	var missing []string
	floors := false
	for _, t := range OptionalTools {
		if _, err := p.LookPath(t.Name); err != nil {
			missing = append(missing, t.Name+" ("+t.Use+")")
			floors = floors || t.Floor
		}
	}
	if len(missing) == 0 {
		return Result{Status: Pass, Detail: "all optional tools found"}
	}
	d := fmt.Sprintf("%d optional tool(s) missing: %s", len(missing), strings.Join(missing, ", "))
	if floors {
		d += "; a floor whose tool is missing blocks verify, it never passes"
	}
	return Result{Status: Warn, Detail: d}
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
		return Result{Status: Warn, Detail: fmt.Sprintf("%d leaked worktree(s); `aayu gc` prunes them", len(leaks)), Lines: leaks}
	}
	return Result{Status: Pass, Detail: "no leaked worktrees"}
}

// leaseTag is the lock comment `aayu build` keeps on a story it holds (WP5.5).
var leaseTag = regexp.MustCompile(`<!-- aayudha:lease holder=(\S+) expires=(\S+) -->`)

// checkLeases lists the open stories labelled aayudha:leased and warns about
// leases whose lock comment has expired, which `aayu gc` reclaims.
func checkLeases(ctx context.Context, p *Probe) Result {
	if p.API == nil || p.Repo == "" {
		return Result{Status: Warn, Detail: "could not check build leases (no GitHub token or repository)"}
	}
	var issues []struct {
		Number int `json:"number"`
	}
	if err := p.API(ctx, "/repos/"+p.Repo+"/issues?state=open&labels=aayudha:leased&per_page=100", &issues); err != nil {
		return Result{Status: Warn, Detail: "could not list leased stories: " + err.Error()}
	}
	var stale []string
	for _, is := range issues {
		var cs []struct {
			Body string `json:"body"`
		}
		if err := p.API(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", p.Repo, is.Number), &cs); err != nil {
			return Result{Status: Warn, Detail: fmt.Sprintf("could not read #%d's lease: %v", is.Number, err)}
		}
		lease := ""
		for _, c := range cs {
			if m := leaseTag.FindStringSubmatch(c.Body); m != nil {
				if exp, err := time.Parse(time.RFC3339, m[2]); err == nil && exp.Before(time.Now()) {
					lease = fmt.Sprintf("#%d: %s, expired %s", is.Number, m[1], m[2])
				} else if err == nil {
					lease = "live"
				}
			}
		}
		switch lease {
		case "live":
		case "":
			stale = append(stale, fmt.Sprintf("#%d: labelled leased without a lock comment", is.Number))
		default:
			stale = append(stale, lease)
		}
	}
	if len(stale) > 0 {
		return Result{Status: Warn, Detail: fmt.Sprintf("%d stale build lease(s); `aayu gc` reclaims them", len(stale)), Lines: stale}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("%d live build lease(s)", len(issues))}
}

// checkFeedbackLayers lists the open Feedback issues with no fix:<layer> label
// (D-7.7e): evolve refuses them and nothing else acts on them until someone
// records the highest layer that can fix them for good.
func checkFeedbackLayers(ctx context.Context, p *Probe) Result {
	if p.API == nil || p.Repo == "" {
		return Result{Status: Warn, Detail: "could not check Feedback fix layers (no GitHub token or repository)"}
	}
	var issues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		PR any `json:"pull_request"`
	}
	if err := p.API(ctx, "/repos/"+p.Repo+"/issues?state=open&labels=type:feedback&per_page=100", &issues); err != nil {
		return Result{Status: Warn, Detail: "could not list open Feedback: " + err.Error()}
	}
	var open []string
	for _, is := range issues {
		var names []string
		for _, l := range is.Labels {
			names = append(names, l.Name)
		}
		if is.PR == nil && workitems.FixLayerOf(names) == "" {
			open = append(open, fmt.Sprintf("#%d %s", is.Number, is.Title))
		}
	}
	if len(open) > 0 {
		return Result{Status: Warn, Detail: fmt.Sprintf("%d open Feedback issue(s) without a fix layer; evolve refuses them until each is classified: `%s`", len(open), workitems.ClassifyCommand), Lines: open}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("%d open Feedback issue(s), each with a fix layer", len(issues))}
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

// checkSkillCopies compares the installed skill copies with lock.toml (WP3.2).
func checkSkillCopies(ctx context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	lock, ok, err := hosts.ReadLock(p.Root)
	switch {
	case err != nil:
		return Result{Status: Fail, Detail: err.Error()}
	case !ok:
		return Result{Status: Warn, Detail: "skills are not installed here (aayu install shows the plan, --apply writes it)"}
	}
	if problems := hosts.CheckCopies(p.Root); len(problems) > 0 {
		return Result{Status: Fail, Detail: fmt.Sprintf("skill copies differ from %s; aayu doctor --fix re-syncs them", hosts.LockPath), Lines: problems}
	}
	// A Cursor plugin installed from a marketplace lives with the Cursor
	// account, where aayu cannot see it (D-3.2b.i), so doctor says so.
	var cursor []string
	if slices.Contains(lock.Hosts, "cursor") {
		cursor = []string{"Cursor: aayu sees an aayudha plugin only in ~/.cursor/plugins/local, not one installed from a marketplace; if you installed one, disable it for this workspace in Cursor, or Cursor lists each aayudha skill twice"}
	}
	// Each person installs for their own tool, and install keeps the lock's
	// other tools (Story #1005), so a tool missing from the lock is only added.
	missing := toolNotInLock(ctx, p, lock)
	home := p.Getenv("HOME")
	if doubles := hosts.SkillDoubles(p.Root, home, hosts.CodexHome(home, p.Getenv), lock); len(doubles) > 0 {
		if missing != "" {
			doubles = append(doubles, missing)
		}
		return Result{Status: Fail, Detail: "a host lists each aayudha skill twice", Lines: append(doubles, cursor...)}
	}
	if hosts.CursorReadsBoth(p.Root, lock) {
		// Claude Code reads only .claude/skills and Codex only .agents/skills,
		// so with both installed the copies are in both, and Cursor reads both.
		lines := []string{"turn off Third-Party Imports in Cursor's settings (Agents) on this machine, or install for Claude Code or Codex, not both"}
		if missing != "" {
			lines = append(lines, missing)
		}
		return Result{Status: Warn, Detail: "Cursor lists each aayudha skill twice: Claude Code needs the copies in .claude/skills and Codex those in .agents/skills, and Cursor reads both",
			Lines: append(lines, cursor...)}
	}
	if missing != "" {
		return Result{Status: Warn, Detail: missing, Lines: cursor}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("%d skill(s) for %s in sync with %s (skills %s)", len(lock.Skills), strings.Join(lock.Hosts, ", "), hosts.LockPath, lock.SkillsVersion), Lines: cursor}
}

// toolNotInLock says when the person's tool for this repository (R9) is not
// one of lock.toml's hosts, so it has no skill copies to read.
func toolNotInLock(ctx context.Context, p *Probe, lock hosts.Lock) string {
	if p.RepoTool == nil {
		return ""
	}
	tool, err := p.RepoTool(ctx)
	if err != nil {
		return "cannot read your tool for this repository: " + err.Error()
	}
	if tool == "" || slices.Contains(lock.Hosts, tool) {
		return ""
	}
	who := "your tool"
	if p.Human != nil && p.WhoAmI != nil {
		if tok, err := p.Human(ctx); err == nil {
			if u, err := p.WhoAmI(ctx, tok); err == nil && u.Login != "" {
				who = "@" + u.Login + "'s tool"
			}
		}
	}
	return fmt.Sprintf("%s is %s but not in lock.toml: run aayu install --host %s", tool, who, tool)
}

// checkMCP checks where each host gets the aayudha MCP server: an
// installed host's registration (present, and its absolute binary path runs
// and reports this aayu's version, WP3.3), or an enabled aayudha plugin. It
// warns when no host has the server at all.
func checkMCP(ctx context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	var lock hosts.Lock
	if l, ok, err := hosts.ReadLock(p.Root); err == nil && ok {
		lock = l
	}
	var problems, steps, fine []string
	home := p.Getenv("HOME")
	codexHome := hosts.CodexHome(home, p.Getenv)
	// A host this repository is not installed for still has the server when
	// its aayudha plugin is on.
	for _, h := range []verbs.HostID{verbs.HostClaude, verbs.HostCodex, verbs.HostCursor} {
		if !slices.Contains(lock.Hosts, string(h)) && hosts.PluginEnabled(h, p.Root, home, codexHome) {
			fine = append(fine, string(h)+" from the aayudha plugin")
		}
	}
	for _, r := range hosts.Registrations(p.Root, home, lock.Hosts) {
		plugin := hosts.PluginEnabled(r.Host, p.Root, home, codexHome)
		switch {
		case r.Host == "codex" && r.Err == nil && r.Command != "" && !hosts.CodexTrusted(p.Root, codexHome):
			// Codex reads .codex/config.toml only once the person trusts the
			// project: a setup step, not a fault (D-3.2b.m). Until then any
			// plugin server is the one Codex lists, once.
			steps = append(steps, "codex: "+hosts.CodexTrustStep(p.Root, codexHome))
		case plugin && r.Command != "":
			problems = append(problems, fmt.Sprintf("%s: the aayudha plugin's server and %s both register %q, so the host lists it twice", r.Host, r.File, hosts.ServerName))
		case plugin:
			fine = append(fine, string(r.Host)+" from the aayudha plugin")
		case r.Err != nil:
			problems = append(problems, fmt.Sprintf("%s: %s: %v", r.Host, r.File, r.Err))
		case r.Command == "":
			problems = append(problems, fmt.Sprintf("%s: no %q server in %s", r.Host, hosts.ServerName, r.File))
		case repoLocal(p.Root, r.Command):
			// A registration is repository content anyone can change; doctor
			// runs only an absolute binary outside the checkout.
			problems = append(problems, fmt.Sprintf("%s: %s points the %q server at %s, which is relative or inside this repository, so doctor does not run it; install writes aayu's absolute path", r.Host, r.File, hosts.ServerName, r.Command))
		default:
			out, err := p.Exec(ctx, "", r.Command, "version")
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: %s does not run (%v)", r.Host, r.Command, err))
			} else if !strings.Contains(out, " "+p.Version) {
				problems = append(problems, fmt.Sprintf("%s: %s reports %q; this aayu is %s", r.Host, r.Command, strings.TrimSpace(out), p.Version))
			} else {
				fine = append(fine, string(r.Host)+" from "+r.File+" at this aayu's version")
			}
		}
	}
	// Cursor starts the server with only a few of the person's variables, so
	// a token the shell finds through another may not reach it (D-3.2b.p).
	if slices.Contains(lock.Hosts, "cursor") && p.Human != nil && p.CursorMCPHuman != nil {
		if tok, err := p.Human(ctx); err == nil {
			if _, err := p.CursorMCPHuman(ctx); err != nil {
				steps = append(steps, "cursor: Cursor starts the aayudha MCP server with only HOME, LOGNAME, PATH, SHELL, TERM, USER, and PWD, so it finds no GitHub token there (this shell's comes from "+tok.Source+"): run `gh auth login` with gh's default config folder, or `aayu auth set-secret "+identity.HumanTokenSecret+"`")
			}
		}
	}
	if len(problems) > 0 {
		return Result{Status: Fail, Detail: "MCP registrations need aayu install --apply", Lines: append(problems, steps...)}
	}
	if len(steps) > 0 {
		detail := "trust this project in Codex, so it loads the aayudha MCP server from " + hosts.CodexConfig
		if !strings.HasPrefix(steps[len(steps)-1], "codex: ") {
			detail = "the aayudha MCP server in Cursor finds no GitHub token; the lines say what a person does"
			if len(steps) > 1 {
				detail = "the aayudha MCP server needs a person's step in Codex and in Cursor; the lines say which"
			}
		}
		return Result{Status: Warn, Detail: detail, Lines: steps}
	}
	if len(fine) == 0 {
		return Result{Status: Warn, Detail: "no host has the aayudha MCP server: this repository is not installed for one, and no aayudha plugin is on; run `aayu install --host <claude|codex|copilot|cursor|amp> --apply`, or add the aayudha plugin to your host"}
	}
	return Result{Status: Pass, Detail: "aayudha MCP server: " + strings.Join(fine, "; ")}
}

// repoLocal reports whether command is relative or lies inside root, after
// resolving symbolic links: a file the repository itself can supply.
func repoLocal(root, command string) bool {
	if !filepath.IsAbs(command) {
		return true
	}
	for _, r := range []string{root, evalOr(root)} {
		for _, c := range []string{command, evalOr(command)} {
			if rel, err := filepath.Rel(r, c); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

func evalOr(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// checkCopilot reports what GitHub Copilot needs from a person here: the
// hooks file, trust in the folder (Copilot CLI loads the project's MCP server
// and hooks only then), and a sign-in kept apart from aayu's GitHub identity.
// Copilot CLI signs in with COPILOT_GITHUB_TOKEN, then GH_TOKEN, then
// GITHUB_TOKEN, before a stored copilot login; aayu sends COPILOT_GITHUB_TOKEN
// only to Copilot's own endpoint, never for its GitHub writes, so with it set
// each keeps its own account.
func checkCopilot(ctx context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	if l, ok, err := hosts.ReadLock(p.Root); err != nil || !ok || !slices.Contains(l.Hosts, string(verbs.HostCopilot)) {
		return Result{Status: Pass, Detail: "not installed for GitHub Copilot here"}
	}
	var problems, steps, lines []string
	if !hosts.CopilotGuarded(p.Root) {
		problems = append(problems, "no aayudha hooks in "+hosts.CopilotHooks+", so nothing stops an agent in Copilot from running a human-only verb: run `aayu install --host copilot --apply`")
	}
	if other := hosts.CopilotOtherHooks(p.Root); len(other) > 0 {
		// A trusted folder runs every hook file in it, from whatever branch is
		// checked out, with the person's environment.
		steps = append(steps, "Copilot runs these hook commands from this repository with your environment; check you expect each one, especially after checking out someone else's branch:")
		steps = append(steps, other...)
	}
	home := p.Getenv("HOME")
	if home == "" {
		home = p.Getenv("USERPROFILE") // Windows
	}
	if path, err := p.LookPath("copilot"); err == nil {
		if out, err := p.Exec(ctx, "", path, "--version"); err == nil {
			lines = append(lines, strings.TrimSpace(strings.SplitN(out, "\n", 2)[0]))
		} else {
			steps = append(steps, fmt.Sprintf("%s --version fails (%v): reinstall Copilot CLI or fix PATH", path, err))
		}
		if !hosts.CopilotTrusted(p.Root, hosts.CopilotHome(home, p.Getenv)) {
			steps = append(steps, "trust this folder in Copilot CLI: start `copilot` here once and answer yes; until then it loads neither the aayudha MCP server nor the hooks")
		}
		if p.Getenv("TERM_PROGRAM") == "vscode" {
			// VS Code keeps its own Workspace Trust, which aayu cannot read.
			steps = append(steps, "this is VS Code's terminal: VS Code keeps its own Workspace Trust, which aayu cannot read; trust this folder there, or VS Code's agent mode loads neither the aayudha MCP server nor the hooks")
		}
	} else {
		// VS Code keeps Workspace Trust where aayu cannot read it.
		steps = append(steps, "Copilot CLI is not on PATH, so aayu cannot check trust: in VS Code, trust this folder (Workspace Trust), or it loads neither the aayudha MCP server nor the hooks")
	}
	switch ct := p.Getenv("COPILOT_GITHUB_TOKEN"); {
	case ct != "" && p.Human != nil:
		if tok, err := p.Human(ctx); err == nil && tok.Value == ct {
			problems = append(problems, "aayu's GitHub token ("+tok.Source+") is Copilot's COPILOT_GITHUB_TOKEN, so aayu would write as Copilot's account: give aayu its own token")
		} else {
			lines = append(lines, "Copilot signs in with COPILOT_GITHUB_TOKEN; aayu's GitHub writes never use it")
		}
	case ct != "":
		lines = append(lines, "Copilot signs in with COPILOT_GITHUB_TOKEN; aayu's GitHub writes never use it")
	case p.Getenv("GH_TOKEN") != "" || p.Getenv("GITHUB_TOKEN") != "":
		lines = append(lines, "Copilot CLI signs in with GH_TOKEN or GITHUB_TOKEN here, ahead of a stored copilot login; to run Copilot on another account, set COPILOT_GITHUB_TOKEN for it")
	default:
		lines = append(lines, "Copilot signs in with its own login (copilot login, or VS Code's GitHub account)")
	}
	famLines, famSteps := copilotFamilies(ctx, p)
	lines, steps = append(lines, famLines...), append(steps, famSteps...)
	lines = append(lines, hosts.CopilotProven)
	switch {
	case len(problems) > 0:
		return Result{Status: Fail, Detail: "GitHub Copilot is not set up safely here; the lines say what to fix", Lines: append(append(problems, steps...), lines...)}
	case len(steps) > 0:
		return Result{Status: Warn, Detail: "GitHub Copilot needs a person's step; the lines say which", Lines: append(steps, lines...)}
	}
	return Result{Status: Pass, Detail: "GitHub Copilot has the aayudha hooks and trusts this folder", Lines: lines}
}

// checkHostConfig fails when a hook, MCP server entry, permission, or plugin
// file a host runs from this repository differs from what aayu install
// writes. A host reads them from whatever branch is checked out, before any
// review, so a changed one is treated as a fault, never a note.
func checkHostConfig(ctx context.Context, p *Probe) Result {
	if p.HostConfig == nil {
		return Result{Status: Pass, Detail: "not installed for a host here"}
	}
	drift, err := p.HostConfig(ctx)
	switch {
	case err != nil:
		// A check that cannot finish is no proof the config is safe.
		return Result{Status: Fail, Blocking: true, Detail: "cannot compare the hosts' hooks and MCP entries with what aayu install writes: " + err.Error()}
	case len(drift) > 0:
		return Result{Status: Fail, Blocking: true, Detail: "a hook or MCP entry differs from what aayu install writes, so a host may run a command aayu did not install: read the diff with `aayu install`, then run `aayu install --apply`",
			Lines: drift}
	}
	return Result{Status: Pass, Detail: "the hosts' hooks and MCP entries are what aayu install writes"}
}

// copilotFamilies says which model families Copilot enables for the account,
// and so which builds its review lane can review in another family.
func copilotFamilies(ctx context.Context, p *Probe) (lines, steps []string) {
	if p.CopilotModels == nil {
		return nil, nil
	}
	fams, err := p.CopilotModels(ctx)
	if err != nil {
		// No list is no proof a reviewer from another family is there.
		return nil, []string{"cannot list Copilot's models, so aayu cannot say whether its review lane can review in another family: " + err.Error()}
	}
	var names []string
	for f, ids := range fams {
		names = append(names, fmt.Sprintf("%s (%s)", f, strings.Join(ids, ", ")))
	}
	slices.Sort(names)
	lines = []string{"Copilot enables these model families for its account: " + strings.Join(names, "; ")}
	// Copilot's builder is always from an enabled family, so another family is
	// there to review it whenever at least two are enabled.
	if len(fams) < 2 {
		return lines, []string{"Copilot cannot review in another family: it enables fewer than two model families for this account; ask your Copilot admin to enable another family's models"}
	}
	lines = append(lines, "Copilot can review a build in another family when the build left one of these families unused; a build that used all of them is labelled same-family")
	var off []string
	for f, id := range map[verbs.Family]string{verbs.FamilyAnthropic: verbs.CopilotClaudeOpus, verbs.FamilyOpenAI: verbs.CopilotGPTSol, verbs.FamilyGoogle: verbs.CopilotGeminiArgon} {
		if !slices.Contains(fams[f], id) {
			off = append(off, id)
		}
	}
	if len(off) > 0 {
		slices.Sort(off)
		lines = append(lines, "the review lane's default reviewers "+strings.Join(off, ", ")+" are not enabled here: pin an enabled model from another family with [models.review.copilot] model")
	}
	return lines, nil
}

// checkCodexHooks reports aayudha's Codex hooks that Codex would skip (WP3.2b,
// D-3.2b.a): Codex runs a plugin's or a project's hooks only after the person
// trusts them in /hooks, so until then nothing stops an agent in Codex from
// running a human-only verb.
func checkCodexHooks(_ context.Context, p *Probe) Result {
	found, problems, err := hosts.CodexHookTrust(p.Root, hosts.CodexHome(p.Getenv("HOME"), p.Getenv))
	switch {
	case err != nil:
		return Result{Status: Fail, Detail: err.Error()}
	case found == 0:
		return Result{Status: Pass, Detail: "no aayudha hooks for OpenAI Codex here"}
	case len(problems) > 0:
		return Result{Status: Fail, Detail: "Codex skips aayudha hooks it does not trust, so no guard rail stops an agent from running a human-only verb there: open /hooks in Codex and trust them", Lines: problems}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("Codex trusts aayudha's %d hook(s)", found)}
}

// checkLedgerMedia catches a visibility change after init (V3, WP2.5): a public
// repository whose ledger tracks media in Git LFS is a floor-blocking
// failure, because LFS objects cannot be purged from GitHub.
// checkLFSQuota: GitHub publishes no LFS quota API for personal accounts
// (Appendix B D-1.7a), so the check says where the media go and where to look.
func checkLFSQuota(_ context.Context, p *Probe) Result {
	if p.Config != nil && p.Config.Config.Proof.Media != "" {
		return Result{Status: Pass, Detail: "design and UI baseline media go to the bucket " + p.Config.Config.Proof.Media + "; proof media never use Git LFS"}
	}
	warn := 80
	if p.Config != nil && p.Config.Config.LFS.QuotaWarnPercent > 0 {
		warn = p.Config.Config.LFS.QuotaWarnPercent
	}
	return Result{Status: NA, Detail: fmt.Sprintf("LFS quota: GitHub has no quota API for this account; watch Settings > Billing > Git LFS (warn at %d %%)", warn)}
}

func checkLedgerMedia(ctx context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	out, err := p.Exec(ctx, p.Root, "git", "ls-tree", "--name-only", "origin/ledger")
	if err != nil {
		return Result{Status: Pass, Detail: "no ledger branch fetched (origin/ledger)"}
	}
	names := strings.Fields(out)
	if !slices.Contains(names, ".lfsconfig") && !slices.Contains(names, ".gitattributes") {
		return Result{Status: Pass, Detail: "ledger stores no proof media in Git LFS"}
	}
	if p.API == nil || p.Repo == "" {
		return Result{Status: Warn, Detail: "ledger tracks proof media in Git LFS; repository visibility unknown (no GitHub token or repository)"}
	}
	var repo struct {
		Private bool `json:"private"`
	}
	if err := p.API(ctx, "/repos/"+p.Repo, &repo); err != nil {
		return Result{Status: Warn, Detail: "ledger tracks proof media in Git LFS; repository visibility unknown: " + err.Error()}
	}
	if repo.Private {
		return Result{Status: Pass, Detail: "private repository: ledger proof media in Git LFS"}
	}
	return Result{Status: Fail, Blocking: true,
		Detail: "repository is public but its ledger tracks media in Git LFS (V3): LFS objects cannot be purged from GitHub",
		Lines:  []string{"remove .lfsconfig and the LFS lines of .gitattributes from ledger and delete media already pushed there; proof media stay in run artifacts, and design media need [proof] media"}}
}

// checkLedgerRulesets reports whether `aayu init` could protect the ledger
// branch with its rulesets: on a private repository on GitHub Free it cannot,
// and init records GitHub's answer in state.json (Appendix B D-2.5b).
func checkLedgerRulesets(_ context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: NA, Detail: "not in a repository"}
	}
	b, err := os.ReadFile(filepath.Join(p.Root, ".aayudha", "state.json"))
	if err != nil {
		return Result{Status: NA, Detail: "aayu init has not run here (no .aayudha/state.json)"}
	}
	var st struct {
		RulesetsUnapplied string `json:"rulesets_unapplied"`
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return Result{Status: Warn, Detail: ".aayudha/state.json: " + err.Error()}
	}
	if st.RulesetsUnapplied != "" {
		return Result{Status: Warn, Detail: "ledger rulesets not applied: " + st.RulesetsUnapplied,
			Lines: []string{"the ledger branch is unprotected: anyone with write access can force-push or delete it",
				"make the repository public or upgrade to GitHub Pro, then run `aayu init` again"}}
	}
	return Result{Status: Pass, Detail: "aayu init applied or adopted the ledger rulesets"}
}

// checkProof says where proof lives (S-4.2b): the merge gate keeps each pack as
// its run artifact for [proof] retention_days within GitHub's limit, and `aayu
// ship` lands pack.json on the ledger, so proof needs no bot App, bucket,
// secret, or purge job. It warns about what a 0.1 setup still carries.
func checkProof(_ context.Context, p *Probe) Result {
	days, media := 90, ""
	if p.Config != nil {
		days, media = p.Config.Config.Proof.RetentionDays, p.Config.Config.Proof.Media
	}
	detail := fmt.Sprintf("the merge gate keeps each pack as its aayudha-pack run artifact for %d days (GitHub allows at most 90 on a public repository, 400 on a private one), and `aayu ship` lands pack.json on the ledger; no bot App, bucket, or secret needed", proof.ArtifactRetention(days, "400"))
	if media != "" {
		detail += "; ship also copies media to " + media
	}
	var lines []string
	if p.Root != "" {
		if _, err := os.Stat(filepath.Join(p.Root, "loops", "proof-purge.toml")); err == nil {
			lines = append(lines, "loops/proof-purge.toml runs `aayu proof purge`, which 0.2 removed (media expire with their run artifact): delete it, then run `aayu init --gates` to re-render the loops")
		}
		if b, err := os.ReadFile(filepath.Join(p.Root, ".github", "workflows", "aayudha-merge-gate.yml")); err == nil && !strings.Contains(string(b), "aayudha proof comment") {
			lines = append(lines, ".github/workflows/aayudha-merge-gate.yml is a 0.1 gate (its aayudha/pack job writes the ledger with a bot App instead of checking the pack and commenting): run `aayu init --gates`, which replaces it unless you edited it")
		}
	}
	if len(lines) > 0 {
		return Result{Status: Warn, Detail: "proof: " + detail, Lines: lines}
	}
	return Result{Status: Pass, Detail: "proof: " + detail}
}
