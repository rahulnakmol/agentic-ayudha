package doctor

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// SecurityWorkflow is the security workflow `aayu init --gates` installs
// (Decision #480): verify's security floors on every push to dev, rolling,
// and main, and nightly.
const SecurityWorkflow = "aayudha-security.yml"

// SecurityNightlyMaxAge is how old the latest nightly run may be before
// doctor warns that new advisories are not being looked for.
const SecurityNightlyMaxAge = 48 * time.Hour

type workflowRuns struct {
	Runs []struct {
		HeadSHA    string    `json:"head_sha"`
		Status     string    `json:"status"`
		Conclusion string    `json:"conclusion"`
		CreatedAt  time.Time `json:"created_at"`
	} `json:"workflow_runs"`
}

// checkSecurityWorkflow reports whether the security workflow is installed,
// the latest push run's conclusion on each branch tip, and whether the
// nightly run is current.
func checkSecurityWorkflow(ctx context.Context, p *Probe) Result {
	if p.Root == "" || p.Config == nil {
		return Result{Status: Pass, Detail: "no checkout"}
	}
	if _, err := os.Stat(filepath.Join(p.Root, ".github", "workflows", SecurityWorkflow)); err != nil {
		return Result{Status: Fail, Detail: "not installed: no scan of pushes to dev, rolling, and main, and none nightly; run `aayu init --gates` and commit .github/workflows/" + SecurityWorkflow + " through a pull request"}
	}
	if p.API == nil || p.Repo == "" {
		return Result{Status: Warn, Detail: "installed; its runs are unknown without a GitHub token"}
	}
	b := p.Config.Config.Branches
	var branches []string
	for _, x := range [][2]string{{b.Dev, "dev"}, {b.Rolling, "rolling"}, {b.Main, "main"}} {
		name := x[0]
		if name == "" && (x[1] != "rolling" || b.Dev == "") {
			name = x[1] // unmapped: the default name; a mapped repository without rolling has none
		}
		if name != "" {
			branches = append(branches, name)
		}
	}
	base := "/repos/" + p.Repo + "/actions/workflows/" + SecurityWorkflow + "/runs?per_page=1"
	status, problems := Pass, 0
	var lines []string
	for _, br := range branches {
		var runs workflowRuns
		if err := p.API(ctx, base+"&event=push&branch="+url.QueryEscape(br), &runs); err != nil {
			return Result{Status: Warn, Detail: "installed; its runs could not be read: " + err.Error()}
		}
		switch {
		case len(runs.Runs) == 0:
			lines = append(lines, br+": no push run yet")
		case runs.Runs[0].Status != "completed":
			lines = append(lines, fmt.Sprintf("%s: %s at %s", br, runs.Runs[0].Status, short(runs.Runs[0].HeadSHA)))
		default:
			r := runs.Runs[0]
			lines = append(lines, fmt.Sprintf("%s: %s at %s", br, r.Conclusion, short(r.HeadSHA)))
			if r.Conclusion != "success" {
				status, problems = Warn, problems+1
			}
		}
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	var nightly workflowRuns
	if err := p.API(ctx, base+"&event=schedule", &nightly); err != nil {
		return Result{Status: Warn, Detail: "installed; its nightly runs could not be read: " + err.Error(), Lines: lines}
	}
	switch {
	case len(nightly.Runs) == 0:
		status = Warn
		lines = append(lines, "nightly: no run yet, so new advisories against unchanged dependencies are not looked for")
	case now().Sub(nightly.Runs[0].CreatedAt) > SecurityNightlyMaxAge:
		status = Warn
		lines = append(lines, fmt.Sprintf("nightly: last run %s, more than %d hours ago (a schedule stops after 60 days without activity; re-enable it in Actions)", nightly.Runs[0].CreatedAt.UTC().Format(time.RFC3339), int(SecurityNightlyMaxAge.Hours())))
	default:
		lines = append(lines, fmt.Sprintf("nightly: %s, %s", nightly.Runs[0].Conclusion, nightly.Runs[0].CreatedAt.UTC().Format(time.RFC3339)))
	}
	detail := "installed; every scanned tip is green"
	switch {
	case problems > 0:
		detail = fmt.Sprintf("installed; %d branch tip(s) failed their last scan: see the aayudha:security Bugs", problems)
	case status == Warn:
		detail = "installed; the nightly scan is not current"
	}
	return Result{Status: status, Detail: detail, Lines: lines}
}

// checkDAST reports what the dast floor will do here (Decision #485): n/a
// off the web and api platforms, the fix for a missing target, and the
// container runtime it runs ZAP under, rootless Podman named.
func checkDAST(ctx context.Context, p *Probe) Result {
	if p.Config == nil {
		return Result{Status: Pass, Detail: "no config"}
	}
	v := p.Config.Config.Verify
	if len(v.Platforms) == 0 {
		return Result{Status: Warn, Detail: "[verify] platforms is unset, so the dast floor is n/a: set it (e.g. [\"web\"], [\"api\"], or [\"cli\"]) so a web or api app is scanned"}
	}
	if !slices.ContainsFunc(v.Platforms, func(x string) bool { return slices.Contains(verify.DASTPlatforms, strings.ToLower(x)) }) {
		return Result{Status: Pass, Detail: "n/a: no web or api in [verify] platforms"}
	}
	var lines []string
	status := Pass
	env := "local"
	if p.Getenv != nil && p.Getenv("AAYU_ENV") != "" {
		env = p.Getenv("AAYU_ENV")
	}
	target := strings.TrimSpace(v.Environments[env].URL)
	switch {
	case !v.DAST:
		status = Warn
		lines = append(lines, "target: none ([verify] dast = false), so a web or api story records a blocking skipped: dast. "+verify.DASTFix)
	case target == "":
		status = Warn
		lines = append(lines, "target: none ([verify.environments."+env+"] url is unset), so a web or api story records a blocking skipped: dast. "+verify.DASTFix)
	case v.Environments[env].ActiveScan:
		lines = append(lines, "target: "+target+", active scan ([verify.environments."+env+"] active_scan = true)")
	case len(v.Seed) > 0 && verify.IsLoopback(hostname(target)):
		lines = append(lines, "target: "+target+", started by [verify] seed: active scan")
	default:
		lines = append(lines, "target: "+target+", not started by [verify] seed on a loopback host: passive scan only")
	}
	rt := ""
	for _, name := range verify.ContainerRuntimes {
		if _, err := p.LookPath(name); err == nil {
			rt = name
			break
		}
	}
	if rt == "" {
		status = Warn
		lines = append(lines, "container runtime: none; install Docker or Podman to scan locally (the merge gate and "+SecurityWorkflow+" run it in CI)")
	} else {
		ver, _ := p.Exec(ctx, p.Root, rt, "--version")
		desc := rt
		if f := strings.Fields(strings.ReplaceAll(ver, ",", " ")); len(f) >= 3 {
			desc += " " + f[2] // "Docker version 29.9.0, build x" and "podman version 4.3.1"
		}
		if rt == "podman" {
			if out, err := p.Exec(ctx, p.Root, rt, "info", "--format", "{{.Host.Security.Rootless}}"); err == nil && strings.TrimSpace(out) == "true" {
				desc += " (rootless)"
			}
		}
		lines = append(lines, "container runtime: "+desc)
	}
	detail := "the dast floor runs here"
	if status == Warn {
		detail = "the dast floor cannot run here as configured"
	}
	return Result{Status: status, Detail: detail, Lines: lines}
}

func hostname(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
