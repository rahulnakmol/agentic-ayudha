package hosts

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// GitHub Copilot is one host with two front ends, Copilot CLI and VS Code
// agent mode, which runs the same Copilot runtime. The formats below are from
// GitHub's and VS Code's documentation as read on 2026-10-17; Copilot CLI has
// run them against a real subscription (CopilotProven):
//
//   - skills: .github/skills, .agents/skills, .claude/skills, first name found
//     wins (docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-skills,
//     code.visualstudio.com/docs/agent-customization/agent-skills);
//   - MCP: the project's .mcp.json, mcpServers, read once the person trusts
//     the folder; a stdio server inherits only PATH
//     (docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers);
//   - hooks: .github/hooks/*.json, version 1, camelCase events
//     (docs.github.com/en/copilot/reference/hooks-reference);
//   - the session log: <COPILOT_HOME or ~/.copilot>/session-state/<sessionId>/events.jsonl
//     (docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference),
//     whose event names are github/copilot-sdk's generated session-events schema;
const (
	CopilotHooks   = ".github/hooks/aayudha.json"
	CopilotSkills  = ".github/skills"                      // only Copilot reads it, so the command skills live here
	CopilotSession = ".aayudha/state/copilot-session.json" // the Copilot session that last ran aayu build here
)

// CopilotProven is what has run against a real Copilot subscription, for
// doctor and the install note: the rest is built from the documentation.
const CopilotProven = "live-proven with Copilot CLI 1.0.98 on 2026-10-17 (skills, the aayudha MCP tool, the hooks, each Auto tier, a HydraFusion Critique turn); VS Code agent mode is not live-proven"

// CopilotMCPEnv are the variables the aayudha MCP server needs under Copilot,
// which passes a stdio server only PATH: the home and config folders aayu and
// gh read on Unix and Windows, and the places aayu finds a GitHub token. Each
// is written as ${NAME:-}, so an unset one is empty, which aayu and gh treat as
// unset.
var CopilotMCPEnv = []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "SystemRoot", "XDG_CONFIG_HOME", "AAYU_GITHUB_TOKEN", "GH_TOKEN", "GH_CONFIG_DIR"}

func copilotMCPEnv() map[string]any {
	env := map[string]any{}
	for _, k := range CopilotMCPEnv {
		env[k] = "${" + k + ":-}"
	}
	return env
}

// CopilotHome is Copilot CLI's configuration directory: $COPILOT_HOME, else ~/.copilot.
func CopilotHome(home string, getenv func(string) string) string {
	if d := getenv("COPILOT_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".copilot")
}

// CopilotTrusted reports whether Copilot CLI trusts root: root or a folder
// above it is in trustedFolders of <copilotHome>/config.json, where Copilot
// records the answer to its first-launch question. That file starts with //
// comment lines, which are dropped before it is read as JSON.
func CopilotTrusted(root, copilotHome string) bool {
	b, err := os.ReadFile(filepath.Join(copilotHome, "config.json"))
	if err != nil {
		return false
	}
	var keep []string
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			keep = append(keep, l)
		}
	}
	var c struct {
		TrustedFolders []string `json:"trustedFolders"`
	}
	if json.Unmarshal([]byte(strings.Join(keep, "\n")), &c) != nil {
		return false
	}
	for _, r := range []string{root, resolved(root)} {
		for _, f := range c.TrustedFolders {
			if rel, err := filepath.Rel(f, r); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// copilotEvent is the camelCase preToolUse payload Copilot sends a hook. An
// MCP tool's toolName is <server>-<tool>, such as aayudha-status (seen with
// Copilot CLI 1.0.98).
type copilotEvent struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	ToolName  string `json:"toolName"`
}

// CopilotGuarded reports whether root's Copilot hooks file is one Copilot
// runs (version 1) with a preToolUse command that is aayu's pre-tool hook.
func CopilotGuarded(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(CopilotHooks)))
	var f struct {
		Version int `json:"version"`
		Hooks   struct {
			PreToolUse []struct {
				Type       string `json:"type"`
				Bash       string `json:"bash"`
				PowerShell string `json:"powershell"`
			} `json:"preToolUse"`
		} `json:"hooks"`
	}
	if err != nil || json.Unmarshal(b, &f) != nil || f.Version != 1 {
		return false
	}
	for _, h := range f.Hooks.PreToolUse {
		for _, c := range []string{h.Bash, h.PowerShell} {
			if h.Type == "command" && ourHook.MatchString(c) && strings.HasSuffix(c, " hook pre-tool --host copilot") {
				return true
			}
		}
	}
	return false
}

var sessionID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// recordCopilotSession notes, in root's local state, the Copilot session whose
// agent is about to run aayu build, in shell or through the aayudha MCP
// server's build tool: build reads that session's log for the models Copilot
// reported (ReadCopilotSession).
func recordCopilotSession(root string, event []byte, shell string) {
	var e copilotEvent
	if root == "" || json.Unmarshal(event, &e) != nil || !sessionID.MatchString(e.SessionID) || e.SessionID == "." || e.SessionID == ".." {
		return
	}
	runsBuild := e.ToolName == ServerName+"-build" // the aayudha MCP server's build tool
	for _, m := range aayuWord.FindAllStringSubmatch(shell, -1) {
		runsBuild = runsBuild || m[1] == "build"
	}
	if !runsBuild {
		return
	}
	b, _ := json.Marshal(map[string]string{"session_id": e.SessionID, "cwd": e.Cwd})
	path := filepath.Join(root, filepath.FromSlash(CopilotSession))
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, append(b, '\n'), 0o644)
	}
}

// CopilotSessionLog returns the log of the Copilot session whose pre-tool hook
// noted, within the last maxAge, that it was about to run aayu build in root,
// or "" when there is none. It takes the note: the hook writes one before each
// aayu build its agent runs, so a later build by a person or another host is
// never credited to that session.
func CopilotSessionLog(root, copilotHome string, maxAge time.Duration, now time.Time) string {
	path := filepath.Join(root, filepath.FromSlash(CopilotSession))
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	_ = os.Remove(path)
	if now.Sub(fi.ModTime()) > maxAge {
		return ""
	}
	var rec struct {
		SessionID string `json:"session_id"`
	}
	if err != nil || json.Unmarshal(b, &rec) != nil || !sessionID.MatchString(rec.SessionID) || rec.SessionID == "." || rec.SessionID == ".." {
		return ""
	}
	return filepath.Join(copilotHome, "session-state", rec.SessionID, "events.jsonl")
}

// CopilotModels is what a Copilot session log says about who answered.
type CopilotModels struct {
	// Models are the concrete models Copilot reported for the session's
	// responses, lower-cased, each once, the most recent first. A HydraFusion
	// response counts its source model, and its critic is not among them.
	Models []string `json:"models,omitempty"`
	// Critics are the models of HydraFusion Critique's read-only critics.
	Critics []string `json:"critics,omitempty"`
}

// ReadCopilotSession reads a Copilot session log (events.jsonl, or the
// --output-format json stream of copilot -p): assistant.message's model, or
// for HydraFusion its fusion.sourceModel, and a Critique turn's critics, from
// the critic phase that ran and the plan in session.fusion_resolved (critics,
// else secondaryModel). Lines it cannot parse are skipped.
func ReadCopilotSession(r io.Reader) (CopilotModels, error) {
	var models, critics []string
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Model  string `json:"model"`
				Fusion *struct {
					SourceModel string `json:"sourceModel"`
					PhaseKind   string `json:"phaseKind"`
					Role        string `json:"role"`
				} `json:"fusion"`
				Pattern        string `json:"pattern"`
				PhaseKind      string `json:"phaseKind"`
				SecondaryModel string `json:"secondaryModel"`
				Critics        []struct {
					Model string `json:"model"`
				} `json:"critics"`
			} `json:"data"`
		}
		if len(strings.TrimSpace(string(line))) > 0 && json.Unmarshal(line, &ev) == nil {
			switch d := ev.Data; ev.Type {
			case "assistant.message":
				switch {
				case d.Fusion == nil:
					models = append(models, d.Model)
				case d.Fusion.PhaseKind == "critic" || d.Fusion.Role == "critic":
					critics = append(critics, d.Fusion.SourceModel)
				case d.Fusion.SourceModel != "":
					models = append(models, d.Fusion.SourceModel)
				}
			case "assistant.fusion_phase_completed":
				if d.PhaseKind == "critic" {
					critics = append(critics, d.Model)
				}
			case "session.fusion_resolved":
				if d.Pattern == "critique" {
					for _, c := range d.Critics {
						critics = append(critics, c.Model)
					}
					if len(d.Critics) == 0 {
						critics = append(critics, d.SecondaryModel)
					}
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return CopilotModels{}, err
		}
	}
	return CopilotModels{Models: recentFirst(models), Critics: recentFirst(critics)}, nil
}

// recentFirst lower-cases ids and keeps each once, the last seen first,
// dropping empty ones and the selectors that name no concrete model.
func recentFirst(ids []string) []string {
	var out []string
	for i := len(ids) - 1; i >= 0; i-- {
		id := strings.ToLower(strings.TrimSpace(ids[i]))
		if id == "" || verbs.CopilotSelector(id) || slices.Contains(out, id) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// Trailers are the commit trailers that record m for inspect's
// different-family rule: Aayudha-Host: copilot, one Aayudha-Model line per
// model the session reported, and one Aayudha-Critic-Model line per
// HydraFusion Critique critic. Without a model the builder stays unknown.
func (m CopilotModels) Trailers() string {
	var b strings.Builder
	b.WriteString("Aayudha-Host: copilot")
	for _, id := range m.Models {
		b.WriteString("\nAayudha-Model: " + id)
	}
	for _, id := range m.Critics {
		b.WriteString("\nAayudha-Critic-Model: " + id)
	}
	return b.String()
}
