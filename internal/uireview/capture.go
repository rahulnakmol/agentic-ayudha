package uireview

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

//go:embed capture/review.mjs
var reviewScript []byte

// Capture is one captured state.
type Capture struct {
	ID           string   `json:"id"`
	URL          string   `json:"url"`
	Status       int      `json:"status"`
	Error        string   `json:"error,omitempty"`
	Rest         string   `json:"rest,omitempty"`  // screenshot at rest (path)
	Focus        string   `json:"focus,omitempty"` // screenshot with keyboard focus on the first control (path)
	HTML         string   `json:"html,omitempty"`
	Aria         string   `json:"aria,omitempty"`
	Axe          []AxeHit `json:"axe"`
	FocusMissing []string `json:"focus_missing"`
	SmallTargets []string `json:"small_targets"`
}

// AxeHit is one axe violation.
type AxeHit struct {
	ID     string `json:"id"`
	Impact string `json:"impact"`
	Help   string `json:"help"`
	Nodes  int    `json:"nodes"`
	Target string `json:"target"`
}

// Browser runs the embedded capture script with node in the verify sandbox.
// Playwright, axe-core, and impeccable resolve from Root, then NODE_PATH.
type Browser struct {
	Sandbox     *verify.Sandbox
	Root, Work  string
	MinTargetPx int
	Getenv      func(string) string
}

func (b *Browser) node(ctx context.Context, input any) ([]byte, error) {
	script := filepath.Join(b.Work, "aayudha-review.mjs")
	if err := os.WriteFile(script, reviewScript, 0o644); err != nil {
		return nil, err
	}
	in, _ := json.Marshal(input)
	o, err := b.Sandbox.Run(ctx, verify.Cmd{Dir: b.Root, Name: "node", Args: []string{script}, Stdin: in, Network: true})
	if err != nil {
		return nil, err
	}
	if o.Code != 0 || len(o.Stdout) == 0 {
		return nil, fmt.Errorf("review capture exited %d: %s", o.Code, tail(o.Stderr))
	}
	return o.Stdout, nil
}

// Capture renders and probes each state.
func (b *Browser) Capture(ctx context.Context, states []State) ([]Capture, error) {
	out := filepath.Join(b.Work, "captures")
	raw, err := b.node(ctx, map[string]any{"states": states, "out": out, "min_target_px": b.MinTargetPx,
		"axe_tags": []string{"wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"}})
	if err != nil {
		return nil, err
	}
	var caps []Capture
	if err := json.Unmarshal(raw, &caps); err != nil {
		return nil, fmt.Errorf("review capture: %w", err)
	}
	return caps, nil
}

// ImpeccableHit is one finding of `impeccable detect --json`.
type ImpeccableHit struct {
	Antipattern string `json:"antipattern"`
	Name        string `json:"name"`
	Severity    string `json:"severity"`
	Snippet     string `json:"snippet"`
	Selector    string `json:"selector"`
	Advisory    bool   `json:"advisory"`
}

// Impeccable shells out to `impeccable detect --json` against a URL with
// Playwright's Chromium, project config and advisories off (waivers are
// Decisions, not tool config), and telemetry and update checks off.
func (b *Browser) Impeccable(ctx context.Context, url string) ([]ImpeccableHit, string, error) {
	bin := ""
	for _, d := range nodeDirs(b.Root, b.Getenv("NODE_PATH")) {
		if p, err := exec.LookPath(filepath.Join(d, ".bin", "impeccable")); err == nil { // impeccable.cmd on Windows
			bin = p
			break
		}
	}
	if bin == "" {
		p, err := exec.LookPath("impeccable")
		if err != nil {
			return nil, "", errors.New("impeccable is not installed (npm i -D impeccable@4.5.2) in the repository or NODE_PATH")
		}
		bin = p
	}
	raw, err := b.node(ctx, map[string]any{"mode": "chrome-path"})
	if err != nil {
		return nil, "", err
	}
	var cp struct {
		Chrome string `json:"chrome"`
	}
	if err := json.Unmarshal(raw, &cp); err != nil || cp.Chrome == "" {
		return nil, "", fmt.Errorf("no Chromium from playwright: %s", raw)
	}
	env := map[string]string{"CHROME_PATH": cp.Chrome, "IMPECCABLE_NO_TELEMETRY": "1", "IMPECCABLE_NO_UPDATE_CHECK": "1", "DO_NOT_TRACK": "1"}
	v, _ := b.Sandbox.Run(ctx, verify.Cmd{Dir: b.Root, Name: bin, Args: []string{"--version"}, Env: env})
	version := strings.TrimSpace(string(v.Stdout))
	o, err := b.Sandbox.Run(ctx, verify.Cmd{Dir: b.Root, Name: bin, Network: true, Env: env,
		Args: []string{"detect", "--json", "--no-config", "--no-advisory", "--viewport", "1280x800", url}})
	if err != nil {
		return nil, version, err
	}
	// Exit 0: no primary findings; 2: findings; 1: a target could not be scanned.
	if o.Code != 0 && o.Code != 2 {
		return nil, version, fmt.Errorf("impeccable detect exited %d: %s", o.Code, tail(o.Stderr))
	}
	var hits []ImpeccableHit
	if err := json.Unmarshal(o.Stdout, &hits); err != nil {
		return nil, version, fmt.Errorf("impeccable detect: %w: %s", err, tail(o.Stdout))
	}
	return hits, version, nil
}

func nodeDirs(root, nodePath string) []string {
	dirs := []string{filepath.Join(root, "node_modules")}
	for _, d := range filepath.SplitList(nodePath) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

func tail(b []byte) string {
	if len(b) > 400 {
		b = b[len(b)-400:]
	}
	return strings.TrimSpace(string(b))
}
