package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// CopilotRoute is the route of a call through GitHub Copilot's own client: a
// [models.<lane>.copilot] row that names a concrete Copilot model (not an
// Auto selector) opts that lane's direct calls into it. The key is the
// "copilot" secret, else COPILOT_GITHUB_TOKEN, and goes to that process only.
var CopilotRoute = Route{Via: "copilot", Style: "copilot-cli", Secret: "copilot"}

// ErrCopilot is a call the Copilot route could not make or read: copilot is
// not installed, not signed in, or returned no reply. Callers fall back to
// the lane's normal route.
var ErrCopilot = errors.New("copilot route")

// CopilotCLI calls a model with Copilot CLI in non-interactive mode
// (copilot -p): no tools, no questions, no custom instructions, in an empty
// working directory and a fresh COPILOT_HOME, so no repository hooks, MCP
// configuration, or person's settings load. The reply and its usage come from
// the JSONL output and the usage file; the AI credits Copilot reports are the
// call's cost.
type CopilotCLI struct {
	Bin   string // the copilot executable; "" looks it up on Path
	Token string // COPILOT_GITHUB_TOKEN for the process
	Path  string // PATH for the process
}

// Complete implements Client.
func (c CopilotCLI) Complete(ctx context.Context, r Request) (Response, error) {
	bin := c.Bin
	if bin == "" {
		name := "copilot"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		for _, d := range filepath.SplitList(c.Path) {
			if p := filepath.Join(d, name); isExec(p) {
				bin = p
				break
			}
		}
		if bin == "" {
			return Response{}, fmt.Errorf("%w: copilot (GitHub Copilot CLI) is not installed", ErrCopilot)
		}
	}
	if c.Token == "" {
		return Response{}, fmt.Errorf("%w: not signed in: set the copilot secret or COPILOT_GITHUB_TOKEN", ErrCopilot)
	}
	dir, err := os.MkdirTemp("", "aayu-copilot-")
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	home, work, usage := filepath.Join(dir, "home"), filepath.Join(dir, "work"), filepath.Join(dir, "usage.json")
	for _, d := range []string{home, work} {
		if err := os.Mkdir(d, 0o700); err != nil {
			return Response{}, err
		}
	}
	args := []string{"-p", copilotPrompt(r), "-s", "--model", r.Model, "--available-tools=", "--no-ask-user",
		"--no-custom-instructions", "--no-auto-update", "--output-format", "json", "--usage-output-file", usage}
	if r.Effort != "" {
		args = append(args, "--reasoning-effort", r.Effort)
	}
	n := 0
	for _, m := range r.Messages {
		for _, im := range m.Images {
			n++
			ext := strings.TrimPrefix(im.MediaType, "image/")
			f := filepath.Join(dir, fmt.Sprintf("image-%d.%s", n, ext))
			if err := os.WriteFile(f, im.Data, 0o600); err != nil {
				return Response{}, err
			}
			args = append(args, "--attachment", f)
		}
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = work
	cmd.Env = []string{"PATH=" + c.Path, "HOME=" + home, "COPILOT_HOME=" + home, "TERM=dumb",
		"COPILOT_AUTO_UPDATE=false", "DBUS_SESSION_BUS_ADDRESS=disabled:", "COPILOT_GITHUB_TOKEN=" + c.Token}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	u, _ := os.ReadFile(usage)
	resp, err := parseCopilot(stdout.Bytes(), u)
	if err == nil && resp.TokensIn == 0 {
		// A reply with no usable usage report is not accounted: charge an upper
		// bound so the breaker still sees the spend, and fail so the caller
		// falls back rather than trusting a route it cannot meter.
		resp.CostUSD = copilotUpperBound(r, n)
		err = fmt.Errorf("%w: copilot -p returned no usage report; charged an upper bound of %.4f USD", ErrCopilot, resp.CostUSD)
	}
	if runErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return resp, fmt.Errorf("%w: copilot -p: %w: %s", ErrCopilot, runErr, msg)
	}
	return resp, err
}

// copilotPrompt is the request as one prompt: copilot -p has no system or
// assistant turns, so they are labelled in the text.
func copilotPrompt(r Request) string {
	var b strings.Builder
	b.WriteString(r.System)
	for _, m := range r.Messages {
		b.WriteString("\n\n")
		if len(r.Messages) > 1 {
			b.WriteString(strings.ToUpper(m.Role[:1]) + m.Role[1:] + ":\n")
		}
		b.WriteString(m.Content)
	}
	return strings.TrimSpace(b.String())
}

// parseCopilot reads copilot's JSONL output (the last assistant.message) and
// its usage file. AI credits: 1 AI unit = 1 credit = 0.01 USD, reported in
// nano-units; Copilot bills input and visible output, not reasoning.
func parseCopilot(jsonl, usage []byte) (Response, error) {
	var resp Response
	found := false
	sc := bufio.NewScanner(bytes.NewReader(jsonl))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Content string `json:"content"`
				Model   string `json:"model"`
			} `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Type != "assistant.message" {
			continue
		}
		resp.Text, found = ev.Data.Content, true
		if ev.Data.Model != "" {
			resp.Model = ev.Data.Model
		}
	}
	var u struct {
		TotalNanoAiu float64 `json:"totalNanoAiu"`
		TokenDetails map[string]struct {
			TokenCount int `json:"tokenCount"`
		} `json:"tokenDetails"`
		ModelMetrics map[string]struct {
			Usage struct {
				ReasoningTokens int `json:"reasoningTokens"`
			} `json:"usage"`
		} `json:"modelMetrics"`
	}
	if json.Unmarshal(usage, &u) == nil {
		resp.TokensIn = u.TokenDetails["input"].TokenCount
		resp.CachedIn = u.TokenDetails["cache_read"].TokenCount
		resp.CacheWriteIn = u.TokenDetails["cache_write"].TokenCount
		resp.TokensOut = u.TokenDetails["output"].TokenCount
		for _, m := range u.ModelMetrics {
			resp.TokensOut += m.Usage.ReasoningTokens
		}
		resp.CostUSD = u.TotalNanoAiu / 1e9 * 0.01
	}
	if !found {
		return resp, fmt.Errorf("%w: copilot -p returned no reply", ErrCopilot)
	}
	return resp, nil
}

// copilotUpperBound is the most a call can have cost when Copilot reports no
// usage: the prompt at one token per two characters, Copilot's own prompt
// (about 15k tokens in the probes) rounded up to 25k, 2k per image, and the
// whole output cap (32k when the request sets none), at the model's price or
// the fallback rate.
func copilotUpperBound(r Request, images int) float64 {
	p, _, ok := PriceOf(nil, r.Model)
	if !ok {
		p = ListPrice{Short: Rates{Input: FallbackInputPerMTok, Output: FallbackOutputPerMTok}}
	}
	out := r.MaxTokens
	if out == 0 {
		out = 32000
	}
	in := len(copilotPrompt(r))/2 + 25000 + 2000*images
	return (float64(in)*p.Short.Input + float64(out)*p.Short.Output) / 1e6
}

// RouteOf is the route a resolved model's direct call takes: the opt-in
// Copilot route for a copilot row that names a concrete model, else RouteFor.
// Gateway.Do and `aayu models` both use it.
func RouteOf(m config.Models, res Resolved) (Route, error) {
	if CopilotOptIn(res) {
		return CopilotRoute, nil
	}
	return RouteFor(m, res.Model, res.Family)
}

// CopilotOptIn reports whether host's row for lane routes direct calls
// through Copilot: a [models.<lane>.copilot] row that names a concrete model.
func CopilotOptIn(m Resolved) bool {
	return m.Host == verbs.HostCopilot && m.Source == SourceConfig && m.Family != verbs.FamilyHostManaged
}

func isExec(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && (runtime.GOOS == "windows" || fi.Mode()&0o111 != 0)
}
