package hosts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Hooks are the host hooks `aayu hook <name>` implements (WP3.2). install
// registers each of them with every host it installs for.
var Hooks = []struct{ Name, Summary string }{
	{"session-start", "Report skill copies that differ from .aayudha/lock.toml."},
	{"pre-tool", "Block agent tool calls while the repository is halted or the [budget] breaker has paused agent lanes."},
	{"status-line", "Print one line of journey state for a host's status line, from the last home screen (no network)."},
}

// HookResult is what a hook prints and its exit code. Exit code 2 blocks the
// tool call in Claude Code, Cursor, and the Amp plugin.
type HookResult struct {
	Stdout, Stderr string
	Code           int
	Next           string // the step that clears a refusal, for the error's "next:" line
}

// RunHook runs a hook for host in the repository at root ("" outside one).
// pauseFile is the [budget] breaker's pause file; event is the JSON the host
// sent on stdin (nil when it sent none). The CLI checks halt (WP5.7) before it
// runs pre-tool and answers with Deny itself when the repository is halted.
func RunHook(name string, host verbs.HostID, root, pauseFile string, event []byte) (HookResult, error) {
	switch name {
	case "pre-tool":
		msg, next := "", ""
		if b, err := os.ReadFile(pauseFile); err == nil {
			msg = fmt.Sprintf("aayudha: agent lanes are paused by the [budget] breaker (%s): %s. A human must clear it.", pauseFile, strings.TrimSpace(string(b)))
			next = "a person clears the breaker with `aayu resume`"
		} else if cmd, why := HumanOnlyCommand(shellCommand(event)); why != "" {
			msg = fmt.Sprintf("aayudha: %s is for a person, not an agent (%s); ask them to run `%s` in their own terminal.", firstWords(cmd), why, cmd)
			next = "a person runs `" + cmd + "` in their own terminal"
		}
		r := Deny(host, msg)
		r.Next = next
		return r, nil
	case "session-start":
		msg := ""
		if root != "" {
			if problems := CheckCopies(root); len(problems) > 0 {
				msg = "aayudha: skill copies differ from " + LockPath + " (" + strings.Join(problems, "; ") + "); run `aayu doctor --fix`."
			}
			// Codex does not route a request through the grill gate on its own
			// (Appendix B D-5.5c.12), so its session starts with the rule.
			if _, err := os.Stat(filepath.Join(root, ".aayudha", "config.toml")); err == nil && host == verbs.HostCodex {
				msg = strings.TrimSpace(Routing + "\n" + msg)
			}
		}
		if host == verbs.HostCursor {
			if msg == "" {
				return HookResult{Stdout: "{}\n"}, nil
			}
			out, _ := json.Marshal(map[string]string{"additional_context": msg})
			return HookResult{Stdout: string(out) + "\n"}, nil
		}
		if msg == "" {
			return HookResult{}, nil
		}
		return HookResult{Stdout: msg + "\n"}, nil
	}
	return HookResult{}, fmt.Errorf("unknown hook %q", name)
}

// Routing is how an agent takes a request in a repository aayudha runs
// (WP3.2b, D-3.2b.b): the grill gate first, then only the work the person asked
// for. The AGENTS.md hosts section and Codex's session-start hook carry it.
const Routing = "aayudha: before you change anything for a request, run `aayu grill story --assess --idea \"<the person's words>\" --json` (the grill skill). " +
	"On `unpack`, ask the person its questions, riskiest first, and wait for their answers before you file, ready, or build anything. " +
	"Work only on the issue the person asked for: never pick up an open or Ready story on your own."

// Deny is the pre-tool hook's answer for host: allow when msg is empty, else
// block the tool call with msg (exit 2, and Cursor's JSON permission reply).
func Deny(host verbs.HostID, msg string) HookResult {
	if host == verbs.HostCursor {
		if msg == "" {
			return HookResult{Stdout: `{"permission":"allow"}` + "\n"}
		}
		out, _ := json.Marshal(map[string]string{"permission": "deny", "user_message": msg, "agent_message": msg})
		return HookResult{Stdout: string(out) + "\n", Stderr: msg + "\n", Code: 2}
	}
	if msg == "" {
		return HookResult{}
	}
	return HookResult{Stderr: msg + "\n", Code: 2}
}

// CheckCopies compares each installed skill copy with lock.toml. It returns one
// line per problem; none means the copies are in sync. Without a lock it
// returns nil (nothing was installed).
func CheckCopies(root string) []string {
	lock, ok, err := ReadLock(root)
	if err != nil {
		return []string{err.Error()}
	}
	if !ok {
		return nil
	}
	var problems []string
	seen := map[string]bool{}
	for _, h := range lock.Hosts {
		dir := SkillDir(verbs.HostID(h))
		if seen[dir] {
			continue
		}
		seen[dir] = true
		if fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir))); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			problems = append(problems, dir+" is a symlink, not real copies (V2)")
			continue
		}
		names := make([]string, 0, len(lock.Skills))
		for n := range lock.Skills {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			files, err := readCopy(filepath.Join(root, filepath.FromSlash(dir), n))
			switch {
			case errors.Is(err, fs.ErrNotExist):
				problems = append(problems, dir+"/"+n+" is missing")
			case err != nil:
				problems = append(problems, dir+"/"+n+": "+err.Error())
			case HashSkill(files) != lock.Skills[n].SHA256:
				problems = append(problems, dir+"/"+n+" was edited or is stale")
			}
		}
	}
	for _, n := range slices.Sorted(maps.Keys(lock.ProductSkills)) {
		ps, mirror := lock.ProductSkills[n], ClaudeSkills+"/"+n
		files, err := readCopy(filepath.Join(root, filepath.FromSlash(mirror)))
		src, serr := readCopy(filepath.Join(root, filepath.FromSlash(ps.Source)))
		switch {
		case err != nil:
			problems = append(problems, mirror+" (the copy of "+ps.Source+"): "+err.Error())
		case HashSkill(files) != ps.SHA256:
			problems = append(problems, mirror+" was edited; its source is "+ps.Source)
		case serr != nil || HashSkill(src) != ps.SHA256:
			problems = append(problems, mirror+" is behind its source "+ps.Source)
		}
	}
	return problems
}

// readCopy reads every file of an installed skill; a symlink is an error.
func readCopy(dir string) (map[string][]byte, error) {
	fi, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return nil, errors.New("is a symlink (V2)")
	}
	files := map[string][]byte{}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink (V2)", filepath.Base(p))
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	return files, err
}

// humanPlumbing are the plumbing commands a person runs: they refuse agents or
// act with authority an agent must not take.
var humanPlumbing = []string{"resume", "self-update", "tour", "seat", "uninit"}

// aayuWord finds aayu run as a command: at the start, after a separator, or
// as the last element of a path (quoted or not), but not inside a quoted
// argument such as a commit message.
var aayuWord = regexp.MustCompile(`(?:^|[\s;&|(/\\])aayu(?:dha)?(?:\.exe)?"?\s+([a-z][a-z0-9-]*)([^;&|\n]*)`)

// HumanOnlyCommand reports a shell command that runs a human-only aayudha
// verb (WP3.2b): a verb the MCP server never exposes, a human-only flag, or
// a person's plumbing command. It returns the aayu command and why, or "".
// Hosts that send the shell command to their pre-tool hook (Claude Code,
// Codex, Cursor) block it, so an agent is never the one that ships or releases.
func HumanOnlyCommand(shell string) (cmd, why string) {
	for _, m := range aayuWord.FindAllStringSubmatch(shell, -1) {
		name, rest := m[1], m[2]
		c, ok := verbs.Find(name)
		if !ok {
			continue
		}
		cmd = strings.TrimSpace("aayu " + name + rest)
		if slices.Contains(humanPlumbing, name) {
			return cmd, "a person's plumbing command"
		}
		if c.Exposure == verbs.ExposeNever && c.Has(verbs.SurfaceSkill) || name == "init" || name == "install" && strings.Contains(rest, "--apply") {
			return cmd, "a human-only verb"
		}
		for _, in := range c.Inputs {
			if in.HumanOnly && regexp.MustCompile(`(^|\s)--`+regexp.QuoteMeta(in.Name)+`(\s|=|$)`).MatchString(rest) {
				return cmd, "--" + in.Name + " is human-only"
			}
		}
	}
	return "", ""
}

// shellCommand is the shell command in a pre-tool event: tool_input.command
// (Claude Code, Codex) or command (Cursor's beforeShellExecution).
func shellCommand(event []byte) string {
	var e struct {
		Command   string `json:"command"`
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(event, &e) != nil {
		return ""
	}
	if e.ToolInput.Command != "" {
		return e.ToolInput.Command
	}
	return e.Command
}

func firstWords(cmd string) string {
	f := strings.Fields(cmd)
	if len(f) > 2 {
		f = f[:2]
	}
	return strings.Join(f, " ")
}
