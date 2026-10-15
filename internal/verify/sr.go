package verify

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

var (
	//go:embed sr/sr.spec.cjs
	srSpec []byte
	//go:embed sr/playwright.config.cjs
	srConfig []byte
)

// SRLabel is the PR label that runs the screen-reader floor in CI; it also
// runs nightly. macOS and Windows runners cost several times a Linux one (R3).
const SRLabel = workitems.LabelScreenReader

// srOS is the platform the floor runs on; tests replace it.
var srOS = runtime.GOOS

var quoted = regexp.MustCompile(`"([^"]+)"|“([^”]+)”`)

// Phrases are the phrases a criterion expects a screen reader to speak: each
// quoted string of its expected text, else the whole text. They come from the
// Ready snapshot, never from the agent at run time.
func Phrases(expected string) []string {
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(expected, -1) {
		out = append(out, strings.TrimSpace(m[1]+m[2]))
	}
	if len(out) == 0 && strings.TrimSpace(expected) != "" {
		out = append(out, strings.TrimSpace(expected))
	}
	return out
}

// Missing returns the expected phrases no spoken phrase contains (case- and
// space-insensitive): the transcript diff.
func Missing(expected, spoken []string) []string {
	norm := func(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
	all := norm(strings.Join(spoken, " | "))
	var out []string
	for _, p := range expected {
		if !strings.Contains(all, norm(p)) {
			out = append(out, p)
		}
	}
	return out
}

// ScreenReader is the VoiceOver/NVDA floor (WP4.7).
type ScreenReader struct{}

// Name implements Floor.
func (ScreenReader) Name() string { return "screen-reader" }

type srState struct {
	State
	Steps    int      `json:"steps"`
	Expected []string `json:"expected"`
}

func srStates(r *Run, t Target) []srState {
	var out []srState
	for _, c := range r.Criteria {
		if c.Evidence != "sr-transcript" {
			continue
		}
		p := "/"
		if m := statePath.FindStringSubmatch(c.State); m != nil {
			p = m[1]
		}
		out = append(out, srState{State: State{N: c.N, Kind: c.Evidence, URL: t.URL + p}, Steps: 15, Expected: Phrases(c.Expected)})
	}
	return out
}

// Applies implements Floor: only for stories whose footprint includes a
// screen-reader layer (L01), with sr-transcript criteria, on macOS or
// Windows, and in CI only on the label or the nightly run.
func (ScreenReader) Applies(r *Run) (bool, string) {
	layers := r.Floors.ScreenReader.Layers
	if len(layers) == 0 {
		layers = []string{"L01"}
	}
	if !slices.ContainsFunc(layers, func(l string) bool { return slices.Contains(r.Footprint, l) }) {
		return false, "no-" + strings.Join(layers, "/")
	}
	t, err := ResolveTarget(r)
	if err == nil && len(srStates(r, t)) == 0 {
		return false, "no sr-transcript criteria"
	}
	if srOS != "darwin" && srOS != "windows" {
		return false, "needs a macOS (VoiceOver) or Windows (NVDA) runner: label the PR " + SRLabel + " or wait for the nightly run"
	}
	if r.CI && !slices.Contains(r.Labels, SRLabel) && !r.Nightly {
		return false, "runs on the " + SRLabel + " label or nightly"
	}
	return true, ""
}

// Run implements Floor.
func (ScreenReader) Run(ctx context.Context, r *Run) (Result, error) {
	reader := map[string]string{"darwin": "VoiceOver", "windows": "NVDA"}[srOS]
	res := Result{Check: proof.Check{Tool: "guidepup " + reader, Version: "-", Threshold: float64(r.Floors.ScreenReader.MaxMissingPhrases), Network: NetAllowed}}
	t, err := ResolveTarget(r)
	if err != nil {
		return res, err
	}
	if err := r.need("node"); err != nil {
		return res, err
	}
	cli := ""
	for _, d := range r.nodeDirs() {
		if p := filepath.Join(d, "@playwright", "test", "cli.js"); fileExists(p) {
			cli = p
			break
		}
	}
	if cli == "" {
		return res, fmt.Errorf("@playwright/test is not installed (with @guidepup/playwright and @guidepup/guidepup) in the repository or NODE_PATH")
	}
	if err := seed(ctx, r); err != nil {
		return res, err
	}
	dir := filepath.Join(r.Work, "sr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	states := srStates(r, t)
	sj, _ := json.Marshal(states)
	for name, b := range map[string][]byte{"sr.spec.cjs": srSpec, "playwright.config.cjs": srConfig, "states.json": sj} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return res, err
		}
	}
	// The screen reader reads its settings from the user's profile, so this
	// floor keeps HOME; credential variables are still scrubbed.
	o, err := r.Sandbox.Run(ctx, Cmd{Dir: dir, Name: "node", Args: []string{cli, "test", "-c", filepath.Join(dir, "playwright.config.cjs")}, Network: true, KeepHome: true})
	if err != nil {
		return res, err
	}
	var missing, report []string
	type stateReport struct {
		N        int      `json:"n"`
		URL      string   `json:"url"`
		Expected []string `json:"expected"`
		Missing  []string `json:"missing"`
		Spoken   []string `json:"spoken"`
	}
	var reps []stateReport
	for _, st := range states {
		var spoken []string
		b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("transcript-%d.json", st.N)))
		if err != nil || json.Unmarshal(b, &spoken) != nil {
			return res, fmt.Errorf("no transcript for state %d (playwright test exited %d): %s", st.N, o.Code, tail(append(o.Stdout, o.Stderr...)))
		}
		m := Missing(st.Expected, spoken)
		for _, x := range m {
			missing = append(missing, fmt.Sprintf("state %d: %q", st.N, x))
		}
		reps = append(reps, stateReport{N: st.N, URL: st.URL, Expected: st.Expected, Missing: m, Spoken: spoken})
		res.Artifacts = append(res.Artifacts, Evidence{Path: fmt.Sprintf("transcripts/%d-%s.txt", st.N, strings.ToLower(reader)), Kind: "sr-transcript",
			Data: []byte(strings.Join(spoken, "\n") + "\n"), Criteria: []int{st.N}})
		report = append(report, fmt.Sprintf("state %d: %d of %d phrases", st.N, len(st.Expected)-len(m), len(st.Expected)))
	}
	res.Check.Value = float64(len(missing))
	res.Check.Result, res.Check.Detail = proof.Pass, reader+": "+strings.Join(report, "; ")
	if len(missing) > r.Floors.ScreenReader.MaxMissingPhrases {
		res.Check.Result, res.Check.Detail = proof.Fail, reader+": missing from the transcript: "+strings.Join(first(missing, 5), "; ")
	}
	j, _ := json.MarshalIndent(reps, "", "  ")
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/screen-reader-" + strings.ToLower(reader) + ".json", Kind: "report", Data: j})
	return res, nil
}

// nodeDirs are the node_modules directories tools resolve from: the
// repository's, then NODE_PATH.
func (r *Run) nodeDirs() []string {
	dirs := []string{filepath.Join(r.Root, "node_modules")}
	if r.Sandbox != nil && r.Sandbox.Getenv != nil {
		dirs = append(dirs, filepath.SplitList(r.Sandbox.Getenv("NODE_PATH"))...)
	}
	return dirs
}
