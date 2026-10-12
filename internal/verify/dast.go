package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// ZAPImage is the pinned OWASP ZAP image (stable 2.17.0) the DAST floor runs.
const ZAPImage = "zaproxy/zap-stable@sha256:7aaa659b0d43078febd82e29bad112285c370727e86ab8340444220e17d9f0d2"

// zapPlan is a ZAP Automation Framework plan: spider, the passive baseline,
// then an active scan limited to the cross-site scripting and injection rules,
// so a reflected XSS (which the passive baseline cannot see) is found in a
// minute or two rather than a full active scan's hour.
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
  - type: activeScan
    parameters: {maxRuleDurationInMins: 2, maxScanDurationInMins: 5}
    policyDefinition:
      defaultStrength: medium
      defaultThreshold: "off"
      rules:
        - {id: 40012, strength: medium, threshold: medium}
        - {id: 40014, strength: medium, threshold: medium}
        - {id: 40018, strength: medium, threshold: medium}
        - {id: 90020, strength: medium, threshold: medium}
  - type: report
    parameters: {template: traditional-json, reportDir: /zap/wrk, reportFile: zap.json}
`

// DAST is the ZAP floor (WP4.6), opt-in with [verify] dast = true.
type DAST struct{}

// Name implements Floor.
func (DAST) Name() string { return "dast" }

// Applies implements Floor.
func (DAST) Applies(r *Run) (bool, string) {
	if !r.Verify.DAST {
		return false, "[verify] dast is off"
	}
	t, err := ResolveTarget(r)
	if err == nil && t.URL == "" {
		return false, "no target ([verify.environments." + t.Env + "] url)"
	}
	return true, ""
}

// Run implements Floor.
func (DAST) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "zap", Version: ZAPImage[strings.Index(ZAPImage, "@")+1:][:19], Threshold: float64(r.Floors.DAST.MaxHigh), Network: NetAllowed}}
	t, err := ResolveTarget(r)
	if err != nil {
		return res, err
	}
	if err := r.need("docker"); err != nil {
		return res, err
	}
	if err := seed(ctx, r); err != nil {
		return res, err
	}
	wrk := filepath.Join(r.Work, "zap")
	if err := os.MkdirAll(wrk, 0o777); err != nil {
		return res, err
	}
	_ = os.Chmod(wrk, 0o777) // the image runs as its own zap user
	plan := strings.NewReplacer("{{URLRE}}", strings.ReplaceAll(regexp.QuoteMeta(t.URL), `\`, `\\`), "{{URL}}", t.URL).Replace(zapPlan)
	if err := os.WriteFile(filepath.Join(wrk, "plan.yaml"), []byte(plan), 0o644); err != nil {
		return res, err
	}
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Work, Name: "docker", Network: true,
		Args: []string{"run", "--rm", "--network", "host", "-v", wrk + ":/zap/wrk:rw", ZAPImage, "zap.sh", "-cmd", "-autorun", "/zap/wrk/plan.yaml"}})
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
			s.add(level, fmt.Sprintf("%s (%s, %s)", a.Name, a.PluginID, a.Count))
		}
	}
	c := severityCheck(s, 0, r.Floors.DAST.MaxHigh)
	c.Tool, c.Version, c.Network, c.Threshold = res.Check.Tool, res.Check.Version, NetAllowed, res.Check.Threshold
	res.Check = c
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/zap.json", Kind: "report", Data: b})
	return res, nil
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
