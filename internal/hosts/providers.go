package hosts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ProviderServer is a PostHog or Sentry MCP server a host has registered
// (Decision #449): where it is registered and how it is reached, so doctor
// can say whether it is set up to read only.
type ProviderServer struct {
	Host      string // claude, cursor, codex, amp
	File      string // as shown to a person: .mcp.json, ~/.cursor/mcp.json
	Committed bool   // the file is in the repository, so whatever it holds is shared
	Name      string
	URL       string
	Command   string
	Args      []string
	Env       map[string]string
	Headers   map[string]string
}

// mcpServer is one entry of a host's mcpServers (JSON) or mcp_servers (TOML).
type mcpServer struct {
	URL         string            `json:"url" toml:"url"`
	ServerURL   string            `json:"serverUrl" toml:"-"`
	Command     string            `json:"command" toml:"command"`
	Args        []string          `json:"args" toml:"args"`
	Env         map[string]string `json:"env" toml:"env"`
	Headers     map[string]string `json:"headers" toml:"-"`
	HTTPHeaders map[string]string `json:"-" toml:"http_headers"`
}

// ProviderServers lists the PostHog and Sentry MCP servers in every host
// config aayudha knows: the repository's .mcp.json, .cursor/mcp.json, and
// .codex/config.toml, and the person's ~/.claude.json (this project's and
// global servers), ~/.cursor/mcp.json, Amp settings, and Codex config.
func ProviderServers(root, home, codexHome string) []ProviderServer {
	var out []ProviderServer
	add := func(host, file string, committed bool, servers map[string]mcpServer) {
		names := make([]string, 0, len(servers))
		for n := range servers {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			s := servers[n]
			p := ProviderServer{Host: host, File: file, Committed: committed, Name: n, URL: s.URL, Command: s.Command, Args: s.Args, Env: s.Env, Headers: s.Headers}
			if p.URL == "" {
				p.URL = s.ServerURL
			}
			if p.Headers == nil {
				p.Headers = s.HTTPHeaders
			}
			hay := strings.ToLower(strings.Join(append([]string{n, p.URL, p.Command}, p.Args...), " "))
			if strings.Contains(hay, "posthog") || strings.Contains(hay, "sentry") {
				out = append(out, p)
			}
		}
	}
	readJSON := func(path string, keys ...string) map[string]mcpServer {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var v any
		if json.Unmarshal(b, &v) != nil {
			return nil
		}
		for _, k := range keys {
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[k]
		}
		raw, _ := json.Marshal(v)
		var servers map[string]mcpServer
		_ = json.Unmarshal(raw, &servers)
		return servers
	}
	readTOML := func(path string) map[string]mcpServer {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var cfg struct {
			Servers map[string]mcpServer `toml:"mcp_servers"`
		}
		_ = toml.Unmarshal(b, &cfg)
		return cfg.Servers
	}
	if root != "" {
		add("claude", ClaudeMCP, true, readJSON(filepath.Join(root, ClaudeMCP), "mcpServers"))
		add("cursor", CursorMCP, true, readJSON(filepath.Join(root, filepath.FromSlash(CursorMCP)), "mcpServers"))
		add("codex", CodexConfig, true, readTOML(filepath.Join(root, filepath.FromSlash(CodexConfig))))
	}
	if home != "" {
		add("claude", "~/.claude.json", false, readJSON(filepath.Join(home, ".claude.json"), "mcpServers"))
		if root != "" {
			add("claude", "~/.claude.json (this project)", false, readJSON(filepath.Join(home, ".claude.json"), "projects", root, "mcpServers"))
		}
		add("cursor", "~/.cursor/mcp.json", false, readJSON(filepath.Join(home, ".cursor", "mcp.json"), "mcpServers"))
		add("amp", "~/"+AmpSettings, false, readJSON(filepath.Join(home, filepath.FromSlash(AmpSettings)), "amp.mcpServers"))
	}
	if codexHome != "" {
		add("codex", filepath.Join(codexHome, "config.toml"), false, readTOML(filepath.Join(codexHome, "config.toml")))
	}
	return out
}
