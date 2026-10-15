package delegate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// LocalBins are the receivers' CLIs, in the order aayu looks for them.
var LocalBins = map[string][]string{
	Codex:  {"codex"},
	Cursor: {"cursor-agent", "agent"},
	Claude: {"claude"},
	Amp:    {"amp"},
}

// LocalArgs is the headless command that runs a receiver's CLI on the brief
// in the worktree. Each may edit files there; none commits, pushes, or opens a
// PR (aayu does, after verify). The flags are the CLIs' documented ones
// (research §5): Codex's workspace-write sandbox, Cursor's print mode with
// --force, Claude Code's print mode allowed to edit, and Amp's execute mode.
func LocalArgs(receiver, bin, wt, prompt string) []string {
	switch receiver {
	case Codex:
		return []string{"exec", "--cd", wt, "--sandbox", "workspace-write", prompt}
	case Cursor:
		return []string{"-p", "--force", "--output-format", "text", "--workspace", wt, prompt}
	case Claude:
		return []string{"-p", "--permission-mode", "acceptEdits", "--output-format", "text", prompt}
	case Amp:
		return []string{"-x", prompt}
	}
	return nil
}

// receiverKeys are the variables each receiver's own CLI authenticates with;
// the scrubbed environment passes only the receiver's own.
var receiverKeys = map[string][]string{
	Codex:  {"CODEX_API_KEY", "OPENAI_API_KEY"},
	Cursor: {"CURSOR_API_KEY"},
	Claude: {"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"},
	Amp:    {"AMP_API_KEY"},
}

var keepEnv = regexp.MustCompile(`^(PATH|HOME|USER|LOGNAME|SHELL|TERM|LANG|LC_[A-Z_]+|TZ|TMPDIR|XDG_[A-Z_]+|GOPATH|GOCACHE|GOMODCACHE|GOTOOLCHAIN|GOFLAGS|NODE_PATH|npm_config_cache|PNPM_HOME|COREPACK_HOME)$`)

var secretName = regexp.MustCompile(`(?i)token|secret|passw|credential|api_?key|private`)

// ScrubEnv is the local receiver's environment: the shell and toolchain
// variables, the receiver's own key, and nothing of GitHub's or aayu's. Git
// and gh lose their credentials (no credential helper, no SSH command, an
// empty gh config), so the receiver cannot push or open a PR; aayu does both.
// ghConfig is an empty directory for GH_CONFIG_DIR.
func ScrubEnv(environ []string, receiver, ghConfig string) []string {
	var out []string
	for _, kv := range environ {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case slices.Contains(receiverKeys[receiver], k):
			out = append(out, kv)
		case keepEnv.MatchString(k) && !secretName.MatchString(k):
			out = append(out, kv)
		}
	}
	return append(out,
		"GH_CONFIG_DIR="+ghConfig,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=false",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=credential.helper", "GIT_CONFIG_VALUE_0=",
		"AAYU_DELEGATED="+receiver,
	)
}

// LookLocal finds a receiver's CLI on PATH.
func LookLocal(receiver string, look func(string) (string, error)) (string, bool) {
	for _, b := range LocalBins[receiver] {
		if p, err := look(b); err == nil {
			return p, true
		}
	}
	return "", false
}

// Worktree is where a story delegated to a local receiver is built:
// .aayudha/worktrees/<n>-<receiver>.
func Worktree(root string, story int, receiver string) string {
	return filepath.Join(root, ".aayudha", "worktrees", fmt.Sprintf("%d-%s", story, receiver))
}

// EmptyDir makes an empty temporary directory (the local receiver's gh config).
func EmptyDir() (string, error) { return os.MkdirTemp("", "aayu-delegate-gh-") }
