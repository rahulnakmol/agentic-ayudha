package delegate

import (
	"cmp"
	"crypto/rand"
	"encoding/json"
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
	// OpenCode's CLI runs its stories and its Kanha; Copilot's only its
	// Kanha (Story #1012).
	OpenCode: {"opencode"},
	Copilot:  {"copilot"},
}

// LocalArgs is the headless command that runs a receiver's CLI on the brief
// in the worktree. Each may edit files there; none commits, pushes, or opens a
// PR (aayu does, after verify). The flags are the CLIs' documented ones
// (research §5): Codex's workspace-write sandbox, Cursor's print mode with
// --force, Claude Code's print mode allowed to edit and to run shell commands
// (acceptEdits alone denies Bash in print mode, so the receiver could not run
// the tests the brief asks it to verify with; D-5.5c.13), and Amp's execute
// mode. Claude Code's --allowedTools takes several values, so it comes before
// another flag, never just before the prompt.
func LocalArgs(receiver, bin, wt, prompt string) []string {
	switch receiver {
	case Codex:
		return []string{"exec", "--cd", wt, "--sandbox", "workspace-write", prompt}
	case Cursor:
		return []string{"-p", "--force", "--output-format", "text", "--workspace", wt, prompt}
	case Claude:
		return []string{"-p", "--allowedTools", "Bash", "--permission-mode", "acceptEdits", "--output-format", "text", prompt}
	case Amp:
		return []string{"-x", prompt}
	case OpenCode:
		return []string{"run", prompt}
	}
	return nil
}

// LocalKanha are the tools whose mission Kanha runs as a local session of
// their CLI on the starter's machine (Story #1012, Y9 and K1 A): a cloud task
// of Codex's or Copilot's takes no follow-up, and OpenCode has no cloud
// agent. A Copilot cloud task never coordinates a mission.
var LocalKanha = []string{Codex, Copilot, OpenCode}

// KanhaStart is the headless command that starts a local Kanha session on
// the brief. Copilot takes id, the session ID aayu picks (a UUID); Codex and
// OpenCode print theirs as JSON (KanhaSession); Cursor's is a new empty chat,
// which KanhaResume then gives the brief. The flags let each run aayu and
// reach GitHub without a prompt; aayu's own lane marker refuses it every
// human-only step.
func KanhaStart(tool, id, brief string) []string {
	switch tool {
	case Codex:
		return append([]string{"exec", "--json"}, append(codexKanha, brief)...)
	case Copilot:
		return []string{"-p", brief, "--session-id=" + id, "--allow-all-tools"}
	case OpenCode:
		return []string{"run", "--format", "json", brief}
	case Cursor:
		return []string{"create-chat"}
	}
	return nil
}

// KanhaResume is the headless command that resumes local Kanha session id
// with one event's text. Only the event text reaches it, never issue text.
// The forms are the CLIs' own (checked against their sources, 2026-10-27):
// Codex takes its sandbox on each run, before `resume`; Copilot's -p takes the
// prompt as its value, so `-p --resume <id> "<event>"` does not parse; Cursor
// runs commands headless only with --force.
func KanhaResume(tool, id, event string) []string {
	switch tool {
	case Codex:
		return append(append([]string{"exec"}, codexKanha...), "resume", id, event)
	case Copilot:
		return []string{"-p", event, "--resume=" + id, "--allow-all-tools"}
	case OpenCode:
		return []string{"run", "--session", id, event}
	case Cursor:
		return []string{"-p", "--resume=" + id, "--force", "--trust", event}
	}
	return nil
}

// NewSessionID is a random UUID (version 4): the session ID aayu picks for a
// Copilot Kanha, which Copilot takes only as a UUID.
func NewSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// codexKanha lets a Codex Kanha write in the checkout and reach the network
// (aayu talks to GitHub); exec never asks for approval.
var codexKanha = []string{"--sandbox", "workspace-write", "-c", "sandbox_workspace_write.network_access=true"}

// KanhaSession is the session ID in a KanhaStart command's output: Codex's
// thread.started thread_id, OpenCode's first sessionID, and Cursor's chat ID
// (create-chat prints only it). "" when there is none.
func KanhaSession(tool, out string) string {
	if tool == Cursor {
		if id := strings.TrimSpace(out); id != "" && !strings.ContainsAny(id, " \t\n") {
			return id
		}
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		var ev struct {
			ThreadID  string `json:"thread_id"`
			SessionID string `json:"sessionID"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &ev) != nil {
			continue
		}
		if id := cmp.Or(ev.ThreadID, ev.SessionID); id != "" {
			return id
		}
	}
	return ""
}

// OwnKey reports whether the receiver's own CLI variable is set, which lets
// its CLI run without a saved sign-in.
func OwnKey(receiver string, getenv func(string) string) bool {
	for _, k := range receiverKeys[receiver] {
		if getenv(k) != "" {
			return true
		}
	}
	return false
}

// receiverKeys are the variables only the receiver's own CLI reads; the
// scrubbed environment passes the receiver's own and nothing else. A model
// provider's key (OPENAI_API_KEY, ANTHROPIC_API_KEY) or base URL is never
// passed: it may be a model gateway's credential, and a receiver runs on its
// user's own sign-in (founder rule, 2026-10-16; Appendix B D-5.5c.5).
var receiverKeys = map[string][]string{
	Codex:  {"CODEX_API_KEY"},
	Cursor: {"CURSOR_API_KEY"},
	Claude: {"CLAUDE_CODE_OAUTH_TOKEN"},
	Amp:    {"AMP_API_KEY"},
}

// receiverPaths are the paths only the receiver's own CLI reads, kept like
// HOME: CODEX_HOME is where Codex keeps its sign-in and configuration, so the
// receiver reads the same Codex home that agentenv.Detect probed with this
// process's environment (Appendix B D-5.5c.25). It is a path, not a secret.
var receiverPaths = map[string][]string{
	Codex: {"CODEX_HOME"},
}

var keepEnv = regexp.MustCompile(`^(PATH|HOME|USER|LOGNAME|SHELL|TERM|LANG|LC_[A-Z_]+|TZ|TMPDIR|XDG_[A-Z_]+|GOPATH|GOCACHE|GOMODCACHE|GOTOOLCHAIN|GOFLAGS|NODE_PATH|npm_config_cache|PNPM_HOME|COREPACK_HOME)$`)

var secretName = regexp.MustCompile(`(?i)token|secret|passw|credential|api_?key|private`)

// ScrubEnv is the local receiver's environment: the shell and toolchain
// variables, the receiver's own key and home (CODEX_HOME), and nothing of GitHub's or aayu's. Git
// and gh lose their credentials (no credential helper, no SSH command, an
// empty gh config), so the receiver cannot push or open a PR; aayu does both.
// ghConfig is an empty directory for GH_CONFIG_DIR.
func ScrubEnv(environ []string, receiver, ghConfig string) []string {
	var out []string
	for _, kv := range environ {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case slices.Contains(receiverKeys[receiver], k), slices.Contains(receiverPaths[receiver], k):
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

// Worktree is where hand-off k (from 1) of a story to a local receiver, or of
// a receiver's returned diff, is built: .aayudha/worktrees/<n>-<receiver>, and
// <n>-<receiver>-<k> from the second hand-off on, so each starts fresh.
func Worktree(root string, story int, receiver string, k int) string {
	name := fmt.Sprintf("%d-%s", story, receiver)
	if k > 1 {
		name += fmt.Sprintf("-%d", k)
	}
	return filepath.Join(root, ".aayudha", "worktrees", name)
}

// Branch is hand-off k's branch: story/<n>-<slug>, and story/<n>-<slug>-<k>
// from the second hand-off on.
func Branch(story int, slug string, k int) string {
	b := fmt.Sprintf("story/%d-%s", story, slug)
	if k > 1 {
		b += fmt.Sprintf("-%d", k)
	}
	return b
}

// EmptyDir makes an empty temporary directory (the local receiver's gh config).
func EmptyDir() (string, error) { return os.MkdirTemp("", "aayu-delegate-gh-") }
