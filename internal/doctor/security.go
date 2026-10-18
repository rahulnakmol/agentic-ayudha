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
	Runs []workflowRun `json:"workflow_runs"`
}

type workflowRun struct {
	HeadSHA    string    `json:"head_sha"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"created_at"`
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
	var branches []string
	for _, name := range p.Config.Config.Branches.Names() {
		if name != "" { // a mapped repository without rolling has none
			branches = append(branches, name)
		}
	}
	base := "/repos/" + p.Repo + "/actions/workflows/" + SecurityWorkflow + "/runs?per_page=1"
	status, problems, unscanned := Pass, 0, 0
	var lines []string
	for _, br := range branches {
		var runs workflowRuns
		if err := p.API(ctx, base+"&event=push&branch="+url.QueryEscape(br), &runs); err != nil {
			return Result{Status: Warn, Detail: "installed; its runs could not be read: " + err.Error()}
		}
		// The latest run is of the tip only if its commit is the branch's: a branch can advance with no run (the
		// workflow disabled, or its trigger broken on that branch).
		var tip struct {
			Commit struct {
				SHA string `json:"sha"`
			} `json:"commit"`
		}
		// One path parameter: a slash in the name is escaped too, as go-github does.
		if err := p.API(ctx, "/repos/"+p.Repo+"/branches/"+url.PathEscape(br), &tip); err != nil {
			return Result{Status: Warn, Detail: "installed; the tip of " + br + " could not be read: " + err.Error()}
		}
		switch {
		case len(runs.Runs) > 0 && runs.Runs[0].HeadSHA != tip.Commit.SHA:
			status, unscanned = Warn, unscanned+1
			lines = append(lines, fmt.Sprintf("%s: tip %s has no scan; the last run was %s at %s", br, short(tip.Commit.SHA), runOutcome(runs.Runs[0]), short(runs.Runs[0].HeadSHA)))
		case len(runs.Runs) == 0:
			status, unscanned = Warn, unscanned+1
			lines = append(lines, br+": no push run yet")
		case runs.Runs[0].Status != "completed":
			status, unscanned = Warn, unscanned+1
			lines = append(lines, fmt.Sprintf("%s: %s at %s, no completed scan", br, runs.Runs[0].Status, short(runs.Runs[0].HeadSHA)))
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
	case nightly.Runs[0].Status != "completed" || nightly.Runs[0].Conclusion != "success":
		status = Warn
		state := nightly.Runs[0].Conclusion
		if nightly.Runs[0].Status != "completed" {
			state = nightly.Runs[0].Status
		}
		lines = append(lines, fmt.Sprintf("nightly: %s, %s, so the last refresh of advisories did not complete green", state, nightly.Runs[0].CreatedAt.UTC().Format(time.RFC3339)))
	default:
		lines = append(lines, fmt.Sprintf("nightly: %s, %s", nightly.Runs[0].Conclusion, nightly.Runs[0].CreatedAt.UTC().Format(time.RFC3339)))
	}
	detail := "installed; every scanned tip is green"
	switch {
	case problems > 0:
		detail = fmt.Sprintf("installed; %d branch tip(s) failed their last scan: see the aayudha:security Bugs", problems)
	case unscanned > 0:
		detail = fmt.Sprintf("installed; %d branch tip(s) have no completed scan", unscanned)
	case status == Warn:
		detail = "installed; the nightly scan is not current"
	}
	return Result{Status: status, Detail: detail, Lines: lines}
}

func runOutcome(r workflowRun) string {
	if r.Status != "completed" {
		return r.Status
	}
	return r.Conclusion
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
		lines = append(lines, "target: none ([verify] dast = false), so a web or api story records skipped: dast, which blocks its ship from [verify] new_floors_blocking_from unless waived. "+verify.DASTFix)
	case target == "":
		status = Warn
		lines = append(lines, "target: none ([verify.environments."+env+"] url is unset), so a web or api story records skipped: dast, which blocks its ship from [verify] new_floors_blocking_from unless waived. "+verify.DASTFix)
	case !httpURL(target):
		status = Fail
		lines = append(lines, "target: "+target+" is not an absolute http or https URL ([verify.environments."+env+"] url), so verify blocks the dast floor: write it as http://host:port")
	case v.Environments[env].ActiveScan:
		lines = append(lines, "target: "+target+", active scan ([verify.environments."+env+"] active_scan = true)")
	case len(v.Seed) > 0 && verify.IsLoopback(hostname(target)):
		lines = append(lines, "target: "+target+", active scan if [verify] seed starts it (an app already answering there gets the passive scan only)")
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
		if status != Fail {
			status = Warn
		}
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
	switch status {
	case Warn:
		detail = "the dast floor cannot run here as configured"
	case Fail:
		detail = "the dast floor is blocked here: its target is not a URL it can scan"
	}
	return Result{Status: status, Detail: detail, Lines: lines}
}

func httpURL(raw string) bool {
	_, ok := verify.HTTPURL(raw)
	return ok
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
