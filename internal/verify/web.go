package verify

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

//go:embed web/web.mjs
var webScript []byte

// Target is the running app the web, DAST, and API floors capture.
type Target struct {
	Env        string `json:"env"`
	URL        string `json:"url"`
	Production bool   `json:"production"`
}

// ErrProduction refuses a capture against production (WP4.5): proof captures
// use seeded, synthetic data only (V3).
var ErrProduction = errors.New("refused: the capture target is a production environment; proof captures run only against seeded, non-production environments (V3)")

// ResolveTarget picks the environment AAYU_ENV names (default "local") from
// [verify.environments]. A target is production when its environment says so
// or when its host is the host of any production environment.
func ResolveTarget(r *Run) (Target, error) {
	name := ""
	if r.Env != nil {
		name = r.Env("AAYU_ENV")
	}
	if name == "" {
		name = "local"
	}
	t := Target{Env: name}
	e, ok := r.Verify.Environments[name]
	if !ok {
		return t, nil
	}
	t.URL, t.Production = strings.TrimRight(e.URL, "/"), e.Production
	host := hostOf(t.URL)
	for _, other := range r.Verify.Environments {
		if other.Production && host != "" && hostOf(other.URL) == host {
			t.Production = true
		}
	}
	if t.Production {
		return t, fmt.Errorf("%w: AAYU_ENV=%s (%s)", ErrProduction, name, t.URL)
	}
	return t, nil
}

func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return strings.ToLower(p.Host)
}

// State is one Completion Goal state the web floors capture.
type State struct {
	N     int    `json:"n"`
	Kind  string `json:"kind"`
	URL   string `json:"url"`
	Video bool   `json:"video"`
}

var statePath = regexp.MustCompile("`(/[^`\\s]*)`")

// States lists the states to capture: every screenshot or recording
// criterion of the Ready snapshot, at the path written in backticks in its
// state (e.g. "state: `/settings` as the seeded admin"), else the root.
func States(r *Run, t Target) []State {
	if t.URL == "" {
		return nil
	}
	var out []State
	for _, c := range r.Criteria {
		if c.Evidence != "screenshot" && c.Evidence != "recording" {
			continue
		}
		p := "/"
		if m := statePath.FindStringSubmatch(c.State); m != nil {
			p = m[1]
		}
		out = append(out, State{N: c.N, Kind: c.Evidence, URL: t.URL + p, Video: c.Evidence == "recording"})
	}
	return out
}

var (
	piiEmail = regexp.MustCompile(`[A-Za-z0-9._%+-]+@([A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+)`)
	piiCard  = regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)
	piiSSN   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	reserved = regexp.MustCompile(`(?i)(^|\.)(example\.(com|org|net)|test|invalid|localhost|example)$`)
)

// ScanPII finds personal data in text (the same rules as the browser-side
// scan): email addresses outside the reserved example domains, Luhn-valid
// card numbers, and national ID numbers. Matches are returned masked.
func ScanPII(text string) []string {
	var out []string
	for _, m := range piiEmail.FindAllStringSubmatch(text, -1) {
		if !reserved.MatchString(m[1]) {
			out = append(out, "email address "+maskPII(m[0]))
		}
	}
	for _, m := range piiCard.FindAllString(text, -1) {
		d := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, m)
		if len(d) >= 13 && luhn(d) {
			out = append(out, "payment card number "+maskPII(d))
		}
	}
	for _, m := range piiSSN.FindAllString(text, -1) {
		out = append(out, "national ID number "+maskPII(m))
	}
	return out
}

func maskPII(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "***" + s[len(s)-2:]
}

func luhn(d string) bool {
	sum := 0
	for i := range len(d) {
		n := int(d[len(d)-1-i] - '0')
		if i%2 == 1 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return sum%10 == 0
}

type webState struct {
	N            int      `json:"n"`
	URL          string   `json:"url"`
	Status       int      `json:"status"`
	Error        string   `json:"error"`
	PII          []string `json:"pii"`
	Axe          []axeHit `json:"axe"`
	Motion       []string `json:"motion"`
	Unreachable  []string `json:"keyboard_unreachable"`
	SmallTargets []string `json:"small_targets"`
	Clipped      []string `json:"clipped"`
	Screenshot   string   `json:"screenshot"`
	Video        string   `json:"video"`
}

type axeHit struct {
	ID     string `json:"id"`
	Impact string `json:"impact"`
	Help   string `json:"help"`
	Nodes  int    `json:"nodes"`
	Target string `json:"target"`
}

type webOut struct {
	Versions struct {
		Playwright string `json:"playwright"`
		Axe        string `json:"axe"`
	} `json:"versions"`
	States []webState `json:"states"`
}

// runNode runs the embedded web script with input on stdin.
func runNode(ctx context.Context, r *Run, input any) (Out, error) {
	if err := r.need("node"); err != nil {
		return Out{}, err
	}
	script := filepath.Join(r.Work, "aayudha-web.mjs")
	if err := os.WriteFile(script, webScript, 0o644); err != nil {
		return Out{}, err
	}
	in, _ := json.Marshal(input)
	// The app under test is a network peer, so the web floors run with the network.
	return r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: "node", Args: []string{script}, Stdin: in, Network: true})
}

// seed runs [verify] seed once per verify run, before the first capture.
func seed(ctx context.Context, r *Run) error {
	if len(r.Verify.Seed) == 0 || r.seeded {
		return nil
	}
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: r.Verify.Seed[0], Args: r.Verify.Seed[1:], Network: true})
	if err != nil {
		return err
	}
	if o.Code != 0 {
		return fmt.Errorf("[verify] seed exited %d: %s", o.Code, tail(o.Stderr))
	}
	r.seeded = true
	return nil
}

// A11y is the web accessibility floor (WP4.5): axe at WCAG 2.2 AA on every
// Completion Goal state, keyboard-only reachability, reduced motion, 200 %
// text, 44×44 targets, and the PII redaction pass that blocks the pack.
type A11y struct{}

// Name implements Floor.
func (A11y) Name() string { return "a11y" }

// Applies implements Floor.
func (A11y) Applies(r *Run) (bool, string) { return webApplies(r) }

func webApplies(r *Run) (bool, string) {
	t, err := ResolveTarget(r)
	if err != nil {
		return true, "" // Run refuses; the CLI refuses earlier
	}
	if t.URL == "" {
		return false, "no web target ([verify.environments." + t.Env + "] url)"
	}
	if len(States(r, t)) == 0 {
		return false, "no screenshot or recording criteria"
	}
	return true, ""
}

// Run implements Floor.
func (A11y) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "playwright + axe-core", Version: "-", Network: NetAllowed}}
	t, err := ResolveTarget(r)
	if err != nil {
		return res, err
	}
	if err := seed(ctx, r); err != nil {
		return res, err
	}
	a := r.Floors.A11y
	out := filepath.Join(r.Work, "web")
	o, err := runNode(ctx, r, map[string]any{"mode": "capture", "states": States(r, t), "out": out, "checks": map[string]any{
		"axe_tags": []string{"wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"}, "keyboard": a.KeyboardOnly, "reduced_motion": a.ReducedMotion,
		"text_scale_percent": max(a.TextScalePercent, 100), "min_target_px": a.MinTargetPx}})
	if err != nil {
		return res, err
	}
	var w webOut
	if err := json.Unmarshal(o.Stdout, &w); err != nil {
		return res, fmt.Errorf("web floor script exited %d: %s", o.Code, tail(o.Stderr))
	}
	res.Check.Version = "playwright " + w.Versions.Playwright + ", axe-core " + w.Versions.Axe

	// Redaction first: any personal data blocks the pack, and no capture of
	// this run is kept.
	var pii []string
	for i, s := range w.States {
		if s.Screenshot != "" {
			if txt, ok := ocr(ctx, r, s.Screenshot); ok {
				w.States[i].PII = append(w.States[i].PII, ScanPII(txt)...)
			}
		}
		for _, p := range w.States[i].PII {
			pii = append(pii, fmt.Sprintf("state %d: %s", s.N, p))
		}
	}
	ocrNote := "DOM scan; OCR with tesseract"
	if _, err := exec.LookPath("tesseract"); err != nil {
		ocrNote = "DOM scan; OCR unavailable (tesseract not installed)"
	}
	redaction := proof.Check{Floor: "redaction", Tool: "aayudha PII scan", Version: "1", Result: proof.Pass, Detail: ocrNote, Value: float64(len(pii))}
	if len(pii) > 0 {
		redaction.Result, redaction.Detail = proof.Blocked, "personal data in a capture; the pack is blocked and no capture is kept: "+strings.Join(pii, "; ")
		res.Check.Result, res.Check.Detail = proof.Blocked, "captures discarded by the redaction pass"
		res.More = []proof.Check{redaction}
		return res, nil
	}

	var crit, serious int
	var axeIDs, unreachable, motion, clipped, small, errs []string
	for _, s := range w.States {
		if s.Error != "" || s.Status >= 400 {
			errs = append(errs, fmt.Sprintf("state %d (%s): %s HTTP %d", s.N, s.URL, s.Error, s.Status))
		}
		for _, v := range s.Axe {
			switch v.Impact {
			case "critical":
				crit++
			case "serious":
				serious++
			}
			if v.Impact == "critical" || v.Impact == "serious" {
				axeIDs = append(axeIDs, fmt.Sprintf("state %d: %s (%s, %s)", s.N, v.ID, v.Impact, v.Target))
			}
		}
		for _, x := range s.Unreachable {
			unreachable = append(unreachable, fmt.Sprintf("state %d: %s", s.N, x))
		}
		for _, x := range s.Motion {
			motion = append(motion, fmt.Sprintf("state %d: %s", s.N, x))
		}
		for _, x := range s.Clipped {
			clipped = append(clipped, fmt.Sprintf("state %d: %q", s.N, x))
		}
		for _, x := range s.SmallTargets {
			small = append(small, fmt.Sprintf("state %d: %s", s.N, x))
		}
	}
	if len(errs) > 0 {
		return res, fmt.Errorf("could not capture: %s", strings.Join(errs, "; "))
	}
	res.Check.Value, res.Check.Threshold = float64(crit+serious), float64(a.MaxCritical+a.MaxSerious)
	res.Check.Result = proof.Pass
	res.Check.Detail = fmt.Sprintf("%s: %d critical, %d serious", a.Standard, crit, serious)
	if len(axeIDs) > 0 {
		res.Check.Detail += ": " + strings.Join(first(axeIDs, 5), "; ")
	}
	if crit > a.MaxCritical || serious > a.MaxSerious {
		res.Check.Result = proof.Fail
	}
	sub := func(floor string, on bool, found []string, what string, threshold float64) proof.Check {
		c := proof.Check{Floor: floor, Tool: "playwright", Version: w.Versions.Playwright, Value: float64(len(found)), Threshold: threshold, Network: NetAllowed}
		switch {
		case !on:
			c.Result, c.Detail = proof.Skipped, "skipped: off in [verify.floors.a11y]"
		case len(found) > 0:
			c.Result, c.Detail = proof.Fail, what+": "+strings.Join(first(found, 5), "; ")
		default:
			c.Result, c.Detail = proof.Pass, what+": none"
		}
		return c
	}
	res.More = []proof.Check{
		sub("keyboard", a.KeyboardOnly, unreachable, "controls not reachable by Tab", 0),
		sub("reduced-motion", a.ReducedMotion, motion, "animations running with prefers-reduced-motion", 0),
		sub("text-scale", a.TextScalePercent > 100, clipped, fmt.Sprintf("text clipped at %d %%", a.TextScalePercent), 0),
		sub("targets", a.MinTargetPx > 0, small, fmt.Sprintf("targets under %dx%d px", a.MinTargetPx, a.MinTargetPx), float64(a.MinTargetPx)),
		redaction,
	}
	for _, s := range w.States {
		if s.Screenshot != "" {
			if b, err := os.ReadFile(s.Screenshot); err == nil {
				res.Artifacts = append(res.Artifacts, Evidence{Path: fmt.Sprintf("screens/%d.png", s.N), Kind: "screenshot", Data: b, Criteria: []int{s.N}})
			}
		}
		if s.Video != "" {
			if b, err := os.ReadFile(s.Video); err == nil {
				res.Artifacts = append(res.Artifacts, Evidence{Path: fmt.Sprintf("recordings/%d.webm", s.N), Kind: "recording", Data: b, Criteria: []int{s.N}})
			}
		}
	}
	rep, _ := json.MarshalIndent(w, "", "  ")
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/a11y.json", Kind: "report", Data: rep})
	return res, nil
}

// PageCheck is the accessibility of one page checked outside a verify run
// (the rendered brief of WP7.3).
type PageCheck struct {
	Tool        string   `json:"tool"`
	Critical    int      `json:"critical"`
	Serious     int      `json:"serious"`
	Violations  []string `json:"violations,omitempty"`  // serious and critical axe rules
	Unreachable []string `json:"unreachable,omitempty"` // controls Tab does not reach
	Screenshot  string   `json:"-"`                     // full-page capture, in work
}

// CheckPage runs the web floor script on one page URL (a file:// page is
// fine): axe-core with the WCAG 2.2 AA tags and Tab reachability, under
// reduced motion. root is where Playwright and axe-core resolve from.
func CheckPage(ctx context.Context, sb *Sandbox, root, work, url string) (PageCheck, error) {
	r := &Run{Root: root, Work: work, Sandbox: sb}
	o, err := runNode(ctx, r, map[string]any{"mode": "capture", "states": []State{{N: 1, Kind: "screenshot", URL: url}}, "out": work, "checks": map[string]any{
		"axe_tags": []string{"wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"}, "keyboard": true, "reduced_motion": true,
		"text_scale_percent": 100, "min_target_px": 0}})
	if err != nil {
		return PageCheck{}, err
	}
	var w webOut
	if err := json.Unmarshal(o.Stdout, &w); err != nil || len(w.States) != 1 {
		return PageCheck{}, fmt.Errorf("web floor script exited %d: %s", o.Code, tail(o.Stderr))
	}
	s := w.States[0]
	if s.Error != "" {
		return PageCheck{}, fmt.Errorf("could not load %s: %s", url, s.Error)
	}
	pc := PageCheck{Tool: "playwright " + w.Versions.Playwright + ", axe-core " + w.Versions.Axe, Unreachable: s.Unreachable, Screenshot: s.Screenshot}
	for _, v := range s.Axe {
		switch v.Impact {
		case "critical":
			pc.Critical++
		case "serious":
			pc.Serious++
		default:
			continue
		}
		pc.Violations = append(pc.Violations, fmt.Sprintf("%s (%s, %d node(s), %s): %s", v.ID, v.Impact, v.Nodes, v.Target, v.Help))
	}
	return pc, nil
}

// ocr reads a screenshot's text with tesseract when it is installed.
func ocr(ctx context.Context, r *Run, png string) (string, bool) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", false
	}
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Work, Name: "tesseract", Args: []string{png, "stdout"}})
	if err != nil || o.Code != 0 {
		return "", false
	}
	return string(o.Stdout), true
}

func first(xs []string, n int) []string {
	if len(xs) > n {
		return append(slices.Clone(xs[:n]), fmt.Sprintf("and %d more", len(xs)-n))
	}
	return xs
}

// Perf is the Lighthouse floor (WP4.5): budgets from [verify.floors.perf]
// on every distinct state URL.
type Perf struct{}

// Name implements Floor.
func (Perf) Name() string { return "perf" }

// Applies implements Floor: budgets are product-specific and have no default
// (CONSTRAINTS.md), so the floor runs once the product sets one.
func (Perf) Applies(r *Run) (bool, string) {
	b := r.Floors.Perf
	if b.MinScore == 0 && b.MaxLCPMs == 0 && b.MaxCLS == 0 && b.MaxTBTMs == 0 {
		return false, "no budgets in [verify.floors.perf]"
	}
	return webApplies(r)
}

type lhSummary struct {
	URL   string  `json:"url"`
	Score float64 `json:"score"`
	LCP   float64 `json:"lcp_ms"`
	CLS   float64 `json:"cls"`
	TBT   float64 `json:"tbt_ms"`
}

// Run implements Floor.
func (Perf) Run(ctx context.Context, r *Run) (Result, error) {
	b := r.Floors.Perf
	res := Result{Check: proof.Check{Tool: "lighthouse", Version: "-", Threshold: b.MinScore * 100, Network: NetAllowed}}
	t, err := ResolveTarget(r)
	if err != nil {
		return res, err
	}
	if err := seed(ctx, r); err != nil {
		return res, err
	}
	lh := ""
	for _, d := range r.nodeDirs() {
		if p, err := exec.LookPath(filepath.Join(d, ".bin", "lighthouse")); err == nil { // lighthouse.cmd on Windows
			lh = p
			break
		}
	}
	if lh == "" {
		if err := r.need("lighthouse"); err != nil {
			return res, err
		}
		lh = "lighthouse"
	}
	o, err := runNode(ctx, r, map[string]any{"mode": "chrome-path"})
	if err != nil {
		return res, err
	}
	var cp struct {
		Chrome string `json:"chrome"`
	}
	if json.Unmarshal(o.Stdout, &cp) != nil || cp.Chrome == "" {
		return res, fmt.Errorf("no Chromium from playwright: %s", tail(o.Stderr))
	}
	res.Check.Version = version(ctx, r, lh, "--version")
	var urls []string
	for _, s := range States(r, t) {
		if !slices.Contains(urls, s.URL) {
			urls = append(urls, s.URL)
		}
	}
	var sums []lhSummary
	var over []string
	minScore := 1.0
	for i, u := range urls {
		out := filepath.Join(r.Work, fmt.Sprintf("lh-%d.json", i))
		lo, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: lh, Network: true, Env: map[string]string{"CHROME_PATH": cp.Chrome},
			Args: []string{u, "--output=json", "--output-path=" + out, "--only-categories=performance", "--quiet", "--chrome-flags=--headless=new --no-sandbox"}})
		if err != nil {
			return res, err
		}
		var lhr struct {
			Categories struct {
				Performance struct {
					Score *float64 `json:"score"`
				} `json:"performance"`
			} `json:"categories"`
			Audits map[string]struct {
				NumericValue float64 `json:"numericValue"`
			} `json:"audits"`
		}
		if err := json.Unmarshal(readFile(out), &lhr); err != nil || lhr.Categories.Performance.Score == nil {
			return res, fmt.Errorf("lighthouse on %s exited %d without a score: %s", u, lo.Code, tail(lo.Stderr))
		}
		s := lhSummary{URL: u, Score: *lhr.Categories.Performance.Score, LCP: lhr.Audits["largest-contentful-paint"].NumericValue,
			CLS: lhr.Audits["cumulative-layout-shift"].NumericValue, TBT: lhr.Audits["total-blocking-time"].NumericValue}
		sums = append(sums, s)
		minScore = min(minScore, s.Score)
		if s.Score < b.MinScore {
			over = append(over, fmt.Sprintf("%s score %.2f < %.2f", u, s.Score, b.MinScore))
		}
		if b.MaxLCPMs > 0 && s.LCP > b.MaxLCPMs {
			over = append(over, fmt.Sprintf("%s LCP %.0f ms > %.0f", u, s.LCP, b.MaxLCPMs))
		}
		if b.MaxCLS > 0 && s.CLS > b.MaxCLS {
			over = append(over, fmt.Sprintf("%s CLS %.3f > %.3f", u, s.CLS, b.MaxCLS))
		}
		if b.MaxTBTMs > 0 && s.TBT > b.MaxTBTMs {
			over = append(over, fmt.Sprintf("%s TBT %.0f ms > %.0f", u, s.TBT, b.MaxTBTMs))
		}
	}
	res.Check.Value = float64(int(minScore * 100))
	res.Check.Result, res.Check.Detail = proof.Pass, fmt.Sprintf("%d URL(s) within budget", len(urls))
	if len(over) > 0 {
		res.Check.Result, res.Check.Detail = proof.Fail, strings.Join(first(over, 5), "; ")
	}
	// Only the numbers are kept: a Lighthouse report embeds screenshots of the page.
	j, _ := json.MarshalIndent(sums, "", "  ")
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/perf.json", Kind: "metric", Data: j})
	return res, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
