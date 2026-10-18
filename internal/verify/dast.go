package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// ZAPImage is the pinned OWASP ZAP image (stable 2.17.0) the DAST floor runs,
// fully qualified: rootless Podman refuses a short name with no search registry.
const ZAPImage = "docker.io/zaproxy/zap-stable@sha256:7aaa659b0d43078febd82e29bad112285c370727e86ab8340444220e17d9f0d2"

// ContainerRuntimes are the runtimes the DAST floor runs ZAP under, in order
// of preference (Decision #485, G4).
var ContainerRuntimes = []string{"docker", "podman"}

// DASTRules are the active rules the plan runs against an app verify started:
// cross-site scripting (reflected, persistent) and SQL and command injection.
var DASTRules = []string{"40012", "40014", "40018", "90020"}

// goos is the host OS; a test sets it to check the macOS and Windows layout,
// where containers run in a VM and reach the host by name, not by loopback.
var goos = runtime.GOOS

// zapPlan is a ZAP Automation Framework plan: spider, the passive baseline,
// then ({{ACTIVE}}) an active scan limited to the DASTRules, so a reflected
// XSS (which the passive baseline cannot see) is found in a minute or two
// rather than a full active scan's hour.
const zapPlan = `env:
  contexts:
    - name: target
      urls: ["{{URL}}/"]
      includePaths: ["{{URLRE}}/.*"]
  parameters:
    failOnError: true
    progressToStdout: false
jobs:
  - type: spider
    parameters: {maxDuration: 1, maxDepth: 5}
  - type: passiveScan-wait
    parameters: {maxDuration: 2}
{{ACTIVE}}  - type: report
    parameters: {template: traditional-json, reportDir: /zap/wrk, reportFile: zap.json}
`

const zapActive = `  - type: activeScan
    parameters: {maxRuleDurationInMins: 2, maxScanDurationInMins: 5}
    policyDefinition:
      defaultStrength: medium
      defaultThreshold: "off"
      rules:
{{RULES}}`

// SkippedDAST starts the detail of a web or api story's dast floor that had
// nothing to scan; it blocks ship unless a floor-waiver Decision names dast.
const SkippedDAST = proof.SkippedDAST + " no runnable target"

// DASTFix says how to give the DAST floor a target (verify's skip and doctor).
const DASTFix = "Give verify a start command ([verify] seed, e.g. [\"./scripts/serve.sh\"]) and its URL ([verify.environments.local] url = \"http://127.0.0.1:8080\"), or waive: aayu decide <story> --waiver dast"

// DAST is the ZAP floor (Decision #485): on by default for the web and api
// platforms, active rules only against an app verify started on a loopback
// host, under Docker or Podman.
type DAST struct{}

// Name implements Floor.
func (DAST) Name() string { return "dast" }

// DASTPlatforms are the platforms the DAST floor covers (G2).
var DASTPlatforms = []string{"web", "api"}

// Applies implements Floor: Run records why a web or api story has nothing
// to scan, so the skip can block ship.
func (DAST) Applies(*Run) (bool, string) { return true, "" }

// ContainerRuntime returns the first container runtime on the floor's PATH,
// or "" with none.
func ContainerRuntime(env []string) string {
	for _, rt := range ContainerRuntimes {
		if _, err := lookPath(rt, env); err == nil {
			return rt
		}
	}
	return ""
}

// IsLoopback reports whether a URL host name is a loopback host: the
// active rules run only against an app verify started on one (G1).
func IsLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Run implements Floor.
func (DAST) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "zap", Version: ZAPImage[strings.Index(ZAPImage, "@")+1:][:19], Threshold: float64(r.Floors.DAST.MaxHigh), Network: NetAllowed}}
	skip := func(detail string) (Result, error) {
		res.Check.Result, res.Check.Detail = proof.Skipped, detail
		return res, nil
	}
	if !slices.ContainsFunc(r.Verify.Platforms, func(p string) bool { return slices.Contains(DASTPlatforms, strings.ToLower(p)) }) {
		return skip("n/a: dast (no web or api in [verify] platforms)")
	}
	t, err := ResolveTarget(r)
	if err != nil {
		return res, err
	}
	if !r.Verify.DAST || t.URL == "" {
		why := SkippedDAST + " ([verify.environments." + t.Env + "] url is unset)"
		if !r.Verify.DAST {
			why = SkippedDAST + " ([verify] dast = false)"
		}
		if !r.Floors.DAST.SkippedBlocksShip {
			return skip("skipped: dast, nothing to scan (not blocking: [verify.floors.dast] skipped_blocks_ship = false)")
		}
		return skip(why + ". " + DASTFix)
	}
	rt := ContainerRuntime(r.Sandbox.Env(Cmd{}, NetAllowed))
	if rt == "" {
		if r.CI {
			return res, fmt.Errorf("no container runtime: the dast floor needs Docker or Podman on the runner, so it does not pass")
		}
		return skip("skipped: dast (no container runtime: install Docker or Podman; the merge gate and aayudha-security.yml run it)")
	}
	if err := seed(ctx, r); err != nil {
		return res, err
	}
	u, err := url.Parse(t.URL)
	if err != nil || u.Host == "" {
		return res, fmt.Errorf("[verify.environments.%s] url %q is not an absolute URL", t.Env, t.URL)
	}
	loopback := IsLoopback(u.Hostname())
	// G1: the active rules attack the app; run them only against one verify
	// started itself, or a URL the protected repository config opts in.
	active := (r.seeded && loopback) || r.Verify.Environments[t.Env].ActiveScan
	args := []string{"run", "--rm"}
	if goos == "linux" {
		args = append(args, "--network", "host")
	} else if loopback {
		// Docker Desktop and podman machine run containers in a VM, whose
		// host network is the VM's: reach the host by its runtime name.
		gw := map[string]string{"docker": "host.docker.internal", "podman": "host.containers.internal"}[rt]
		if p := u.Port(); p != "" {
			u.Host = gw + ":" + p
		} else {
			u.Host = gw
		}
	}
	target := strings.TrimRight(u.String(), "/")
	wrk := filepath.Join(r.Work, "zap")
	if err := os.MkdirAll(wrk, 0o777); err != nil {
		return res, err
	}
	_ = os.Chmod(wrk, 0o777) // the image runs as its own zap user (a subordinate UID under rootless Podman)
	activeJob := ""
	if active {
		var rules strings.Builder
		for _, id := range DASTRules {
			fmt.Fprintf(&rules, "        - {id: %s, strength: medium, threshold: medium}\n", id)
		}
		activeJob = strings.Replace(zapActive, "{{RULES}}", rules.String(), 1)
	}
	plan := strings.NewReplacer("{{URLRE}}", strings.ReplaceAll(regexp.QuoteMeta(target), `\`, `\\`), "{{URL}}", target, "{{ACTIVE}}", activeJob).Replace(zapPlan)
	if err := os.WriteFile(filepath.Join(wrk, "plan.yaml"), []byte(plan), 0o644); err != nil {
		return res, err
	}
	args = append(args, "-v", wrk+":/zap/wrk:rw", ZAPImage, "zap.sh", "-cmd", "-autorun", "/zap/wrk/plan.yaml")
	// The runtime keeps the person's HOME, so rootless Podman uses their image
	// store (a throwaway HOME is a fresh store and a 3.7 GB pull every run),
	// and the variables that say which daemon or machine to talk to.
	get := r.Sandbox.Getenv
	if get == nil {
		get = os.Getenv
	}
	conn := map[string]string{}
	for _, k := range []string{"XDG_RUNTIME_DIR", "XDG_DATA_HOME", "DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG", "CONTAINER_HOST", "CONTAINER_CONNECTION", "CONTAINERS_CONF", "CONTAINERS_STORAGE_CONF"} {
		if v := get(k); v != "" {
			conn[k] = v
		}
	}
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Work, Name: rt, Network: true, Args: args, KeepHome: true, Env: conn})
	if err != nil {
		return res, err
	}
	b := readFile(filepath.Join(wrk, "zap.json"))
	var rep struct {
		Version string `json:"@version"`
		Site    []struct {
			Alerts []struct {
				PluginID string `json:"pluginid"`
				Name     string `json:"name"`
				RiskCode string `json:"riskcode"`
				Count    string `json:"count"`
			} `json:"alerts"`
		} `json:"site"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		return res, fmt.Errorf("zap exited %d without a JSON report: %s", o.Code, tail(append(o.Stdout, o.Stderr...)))
	}
	if rep.Version != "" {
		res.Check.Version = rep.Version
	}
	var s Severity
	for _, site := range rep.Site {
		for _, a := range site.Alerts {
			level := map[string]string{"3": "high", "2": "medium", "1": "low"}[a.RiskCode]
			s.found(level, fmt.Sprintf("%s (%s, %s)", a.Name, a.PluginID, a.Count), Finding{Floor: "dast", ID: a.PluginID, Path: t.Env, Where: a.Name, Level: level})
		}
	}
	c := severityCheck(s, 0, r.Floors.DAST.MaxHigh)
	mode := "active scan (rules " + strings.Join(DASTRules, ", ") + ") of the app verify started"
	if !active {
		mode = "passive scan only (verify did not start the app on a loopback host; a passive scan cannot see a reflected XSS)"
	} else if !r.seeded || !loopback {
		mode = "active scan (rules " + strings.Join(DASTRules, ", ") + ", [verify.environments." + t.Env + "] active_scan = true)"
	}
	c.Detail = mode + " under " + rt + ": " + c.Detail
	if c.Result == proof.Fail {
		if why, advisory := newFloorAdvisory(r); advisory {
			c.Result, c.Detail = proof.Pass, why+": "+c.Detail
		} else {
			res.Findings = s.Found
		}
	}
	c.Tool, c.Version, c.Network, c.Threshold = res.Check.Tool, res.Check.Version, NetAllowed, res.Check.Threshold
	res.Check = c
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/zap.json", Kind: "report", Data: b})
	return res, nil
}

// newFloorAdvisory reports whether a miss of a floor Decision #480 added is
// still advisory, and says so: before [verify] new_floors_blocking_from, or
// with no date.
func newFloorAdvisory(r *Run) (string, bool) {
	from, err := time.Parse(time.DateOnly, r.Verify.NewFloorsBlockingFrom)
	if err != nil {
		return "advisory (no [verify] new_floors_blocking_from date)", true
	}
	return "advisory until " + r.Verify.NewFloorsBlockingFrom + " ([verify] new_floors_blocking_from)", r.Now == nil || r.Now().UTC().Before(from)
}

// API is the k6 floor (WP4.6): the product's k6 script holds the API and CLI
// contract as thresholds; k6 exits 99 when one is crossed.
type API struct{}

// Name implements Floor.
func (API) Name() string { return "api" }

// Applies implements Floor.
func (API) Applies(r *Run) (bool, string) {
	if r.Floors.API.Script == "" {
		return false, "no k6 script ([verify.floors.api] script)"
	}
	return true, ""
}

// Run implements Floor.
func (API) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "k6", Network: NetAllowed}}
	t, err := ResolveTarget(r)
	if err != nil {
		return res, err
	}
	if err := r.need("k6"); err != nil {
		return res, err
	}
	res.Check.Version = strings.Fields(version(ctx, r, "k6", "version") + " x x")[1]
	if err := seed(ctx, r); err != nil {
		return res, err
	}
	out := r.tempFile("k6-summary.json")
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: "k6", Network: true, Env: map[string]string{"BASE_URL": t.URL, "K6_NO_USAGE_REPORT": "true"},
		Args: []string{"run", "--quiet", "--no-color", "--summary-export", out, r.Floors.API.Script}})
	if err != nil {
		return res, err
	}
	b := readFile(out)
	var sum struct {
		Metrics map[string]struct {
			Thresholds map[string]bool `json:"thresholds"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(b, &sum); err != nil {
		return res, fmt.Errorf("k6 exited %d without a summary: %s", o.Code, tail(o.Stderr))
	}
	var crossed, total []string
	for m, v := range sum.Metrics {
		for th, failed := range v.Thresholds {
			total = append(total, m)
			if failed { // --summary-export marks a crossed threshold true
				crossed = append(crossed, m+" "+th)
			}
		}
	}
	res.Check.Value = float64(len(crossed))
	res.Check.Result, res.Check.Detail = proof.Pass, strconv.Itoa(len(total))+" threshold(s) held"
	if len(crossed) > 0 || o.Code == 99 {
		res.Check.Result, res.Check.Detail = proof.Fail, "thresholds crossed: "+strings.Join(first(crossed, 5), "; ")
	} else if o.Code != 0 {
		return res, fmt.Errorf("k6 exited %d: %s", o.Code, tail(o.Stderr))
	}
	res.Artifacts = append(res.Artifacts, Evidence{Path: "traces/k6-summary.json", Kind: "api-trace", Data: b})
	return res, nil
}
