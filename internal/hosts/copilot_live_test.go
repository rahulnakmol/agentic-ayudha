package hosts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TestCopilotLive is the live check of the Copilot host (copilot-live.yml):
// a real Copilot CLI, in a folder it trusts, loads what `aayu install --host
// copilot` writes. The model calls the aayudha MCP server's doctor tool, and
// the pre-tool hook denies `aayu ship --solo-bypass`, a step that belongs to a person. It
// costs one Copilot premium request at the Efficiency tier; it runs only with
// AAYU_COPILOT_LIVE=1, a copilot CLI on PATH, and COPILOT_GITHUB_TOKEN, which
// it passes to copilot alone. VS Code is not covered: it has no headless run.
func TestCopilotLive(t *testing.T) {
	token := os.Getenv("COPILOT_GITHUB_TOKEN")
	copilot, err := exec.LookPath("copilot")
	switch {
	case os.Getenv("AAYU_COPILOT_LIVE") != "1":
		t.Skip("set AAYU_COPILOT_LIVE=1 to run a live Copilot CLI (one premium request)")
	case err != nil:
		t.Skip("copilot is not on PATH")
	case token == "":
		t.Skip("COPILOT_GITHUB_TOKEN is not set")
	}
	tmp := t.TempDir()
	root, home, bin := filepath.Join(tmp, "repo"), filepath.Join(tmp, "home"), filepath.Join(tmp, "bin")
	copilotHome := filepath.Join(home, ".copilot")
	for _, d := range []string{root, copilotHome, bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("go", "build", "-o", filepath.Join(bin, "aayu"), "../../cmd/aayudha").CombinedOutput(); err != nil {
		t.Fatalf("build aayu: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	o := Options{Root: root, Home: home, Hosts: []verbs.HostID{verbs.HostCopilot}, Assets: aayudha.Assets,
		Binary: filepath.Join(bin, "aayu"), Version: "0.0.0-dev", SkillsVersion: verbs.SkillVersion}
	res, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(o, res.Edits, true); err != nil {
		t.Fatal(err)
	}
	// Copilot loads a folder's MCP server and hooks only once the person trusts it.
	trust, _ := json.Marshal(map[string][]string{"trustedFolders": {root}})
	if err := os.WriteFile(filepath.Join(copilotHome, "config.json"), trust, 0o644); err != nil {
		t.Fatal(err)
	}

	usage := filepath.Join(tmp, "usage.json")
	cmd := exec.Command(copilot, "-p", "This is an automated test of this folder's tools and hooks. Do exactly these two steps, in order, and nothing else. "+
		"1. Call the MCP tool \"doctor\" of the aayudha server with no arguments. "+
		"2. Run this shell command exactly once, even though a hook is expected to block it: aayu ship 1 --solo-bypass. "+
		"Do not retry or work around a blocked step. Then reply DONE.",
		"--model", "auto", "--auto-tier", "efficiency", "--no-ask-user", "--allow-all-tools",
		"--output-format", "json", "--usage-output-file", usage)
	cmd.Dir = root
	// Only what Copilot needs: never the runner's GitHub tokens, and no session
	// bus, so nothing it or aayu runs can start a dbus-daemon.
	cmd.Env = []string{"HOME=" + home, "COPILOT_HOME=" + copilotHome, "COPILOT_GITHUB_TOKEN=" + token,
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "TERM=dumb",
		"COPILOT_AUTO_UPDATE=false", "DBUS_SESSION_BUS_ADDRESS=disabled:"}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("copilot: %v\n%s", err, stderr.String())
	}

	type event struct {
		Type string `json:"type"`
		Data struct {
			Servers       []struct{ Name, Status string } `json:"servers"`
			ToolCallID    string                          `json:"toolCallId"`
			ToolName      string                          `json:"toolName"`
			MCPServerName string                          `json:"mcpServerName"`
			Arguments     json.RawMessage                 `json:"arguments"`
			Success       bool                            `json:"success"`
			Error         *struct{ Message, Code string } `json:"error"`
			ToolRequests  []struct {
				ToolCallID string          `json:"toolCallId"`
				Name       string          `json:"name"`
				Arguments  json.RawMessage `json:"arguments"`
			} `json:"toolRequests"`
		} `json:"data"`
	}
	loaded := false
	var doctorCalls, shipCalls []string
	done := map[string]event{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		switch e.Type {
		case "session.mcp_servers_loaded":
			for _, s := range e.Data.Servers {
				loaded = loaded || s.Name == ServerName && s.Status == "connected"
			}
		case "assistant.message":
			for _, r := range e.Data.ToolRequests {
				if strings.Contains(string(r.Arguments), "aayu ship") {
					shipCalls = append(shipCalls, r.ToolCallID)
				}
			}
		case "tool.execution_start":
			if e.Data.ToolName == ServerName+"-doctor" && e.Data.MCPServerName == ServerName {
				doctorCalls = append(doctorCalls, e.Data.ToolCallID)
			}
		case "tool.execution_complete":
			done[e.Data.ToolCallID] = e
		}
	}
	if !loaded {
		t.Errorf("Copilot did not connect the aayudha MCP server")
	}
	if !slices.ContainsFunc(doctorCalls, func(id string) bool { return done[id].Data.Success }) {
		t.Errorf("the aayudha MCP server's doctor tool did not run")
	}
	if len(shipCalls) == 0 {
		t.Fatalf("the model never asked to run aayu ship, so the hook went untested:\n%s", out)
	}
	for _, id := range shipCalls {
		if e, ran := done[id]; ran && e.Data.Success {
			t.Errorf("aayu ship ran in Copilot: the pre-tool hook did not deny it")
		}
	}
	if !bytes.Contains(out, []byte("in their own terminal")) {
		t.Errorf("Copilot's output does not carry the hook's denial (run `aayu ship 1 --solo-bypass` in their own terminal)")
	}
	b, err := os.ReadFile(usage)
	if err != nil {
		t.Fatal(err)
	}
	var u struct {
		TotalPremiumRequestCost float64 `json:"totalPremiumRequestCost"`
	}
	if err := json.Unmarshal(b, &u); err != nil {
		t.Fatal(err)
	}
	t.Logf("premium requests used: %v", u.TotalPremiumRequestCost)
	if max := 2.0; u.TotalPremiumRequestCost > max {
		t.Errorf("the run cost %v premium requests, over the budget of %v", u.TotalPremiumRequestCost, max)
	}
}
