package hosts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Hooks are the host hooks `aayu hook <name>` implements (WP3.2). install
// registers each of them with every host it installs for.
var Hooks = []struct{ Name, Summary string }{
	{"session-start", "Report skill copies that differ from .aayudha/lock.toml."},
	{"pre-tool", "Block agent tool calls while the [budget] breaker has paused agent lanes."},
}

// HookResult is what a hook prints and its exit code. Exit code 2 blocks the
// tool call in Claude Code, Cursor, and the Amp plugin.
type HookResult struct {
	Stdout, Stderr string
	Code           int
}

// RunHook runs a hook for host in the repository at root ("" outside one).
// pauseFile is the [budget] breaker's pause file. Halt (WP5.7) is not yet
// available.
func RunHook(name string, host verbs.HostID, root, pauseFile string) (HookResult, error) {
	switch name {
	case "pre-tool":
		msg := ""
		if b, err := os.ReadFile(pauseFile); err == nil {
			msg = fmt.Sprintf("aayudha: agent lanes are paused by the [budget] breaker (%s): %s. A human must clear it.", pauseFile, strings.TrimSpace(string(b)))
		}
		if host == verbs.HostCursor {
			if msg == "" {
				return HookResult{Stdout: `{"permission":"allow"}` + "\n"}, nil
			}
			out, _ := json.Marshal(map[string]string{"permission": "deny", "user_message": msg, "agent_message": msg})
			return HookResult{Stdout: string(out) + "\n", Stderr: msg + "\n", Code: 2}, nil
		}
		if msg == "" {
			return HookResult{}, nil
		}
		return HookResult{Stderr: msg + "\n", Code: 2}, nil
	case "session-start":
		msg := ""
		if root != "" {
			if problems := CheckCopies(root); len(problems) > 0 {
				msg = "aayudha: skill copies differ from " + LockPath + " (" + strings.Join(problems, "; ") + "); run `aayu doctor --fix`."
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
