package delegate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The environment source every receiver shares (research §6.2): the
// repository's .agents/setup, a pinned, idempotent, checksum-verified setup
// script (Amp runs it natively). Each receiver's environment calls it, so a
// pin changes in one place.
const (
	SetupPath     = ".agents/setup"
	SetupCommand  = "bash .agents/setup"
	CursorEnvPath = ".cursor/environment.json"
)

// EnvCheck is one receiver's environment against the source.
type EnvCheck struct {
	Receiver string   `json:"receiver"`
	Status   string   `json:"status"` // ok, drift, manual, missing
	Detail   string   `json:"detail"`
	Lines    []string `json:"lines,omitempty"`
}

var (
	setupGo = regexp.MustCompile(`(?m)^GO_VERSION="?([0-9.]+)"?`)
	modGo   = regexp.MustCompile(`(?m)^go ([0-9.]+)`)
)

// CursorEnvironment is .cursor/environment.json with aayu's two keys set:
// name and install (the source script). Every other key the owner set is
// kept as it is.
func CursorEnvironment(existing []byte) ([]byte, error) {
	m := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", CursorEnvPath, err)
		}
	}
	m["name"] = "aayudha (.agents/setup)"
	m["install"] = SetupCommand
	b, err := json.MarshalIndent(m, "", "  ")
	return append(b, '\n'), err
}

// CheckEnvironments compares every receiver's environment with the source.
func CheckEnvironments(root string) []EnvCheck {
	read := func(p string) ([]byte, error) { return os.ReadFile(filepath.Join(root, filepath.FromSlash(p))) }
	var out []EnvCheck
	src := EnvCheck{Receiver: "source", Status: "ok", Detail: SetupPath + " is the toolchain every receiver installs"}
	setup, err := read(SetupPath)
	switch {
	case err != nil:
		src.Status, src.Detail = "missing", "no "+SetupPath+": write a pinned, idempotent setup script there (every receiver's environment runs it)"
	default:
		if fi, _ := os.Stat(filepath.Join(root, SetupPath)); fi != nil && fi.Mode()&0o111 == 0 {
			src.Status, src.Detail = "drift", SetupPath+" is not executable (chmod +x; Amp runs it directly)"
		}
		mod, _ := read("go.mod")
		if s, m := setupGo.FindSubmatch(setup), modGo.FindSubmatch(mod); s != nil && m != nil && olderVersion(string(s[1]), string(m[1])) {
			src.Status = "drift"
			src.Lines = append(src.Lines, fmt.Sprintf("%s installs Go %s, older than go.mod's go %s", SetupPath, s[1], m[1]))
		}
	}
	out = append(out, src)
	missing := src.Status == "missing"
	amp := EnvCheck{Receiver: Amp, Status: "ok", Detail: "orbs run " + SetupPath + " natively when a project snapshot is prepared"}
	if missing {
		amp.Status, amp.Detail = "missing", "orbs have no "+SetupPath+" to run"
	}
	out = append(out, amp)

	cur := EnvCheck{Receiver: Cursor, Status: "ok", Detail: CursorEnvPath + " installs with " + SetupCommand + " (it takes effect on the default branch)"}
	b, err := read(CursorEnvPath)
	want, _ := CursorEnvironment(b)
	switch {
	case err != nil:
		cur.Status, cur.Detail = "drift", "no "+CursorEnvPath+"; `aayu doctor --fix` writes it, then merge it to the default branch"
	case !bytes.Equal(bytes.TrimSpace(b), bytes.TrimSpace(want)):
		cur.Status, cur.Detail = "drift", CursorEnvPath+" does not install with "+SetupCommand+"; `aayu doctor --fix` sets name and install and keeps the other keys"
	}
	out = append(out, cur)

	out = append(out, EnvCheck{Receiver: Codex, Status: "manual", Detail: "Codex cloud environments are set in the Codex UI only; aayu cannot read them",
		Lines: []string{"Install script: " + SetupCommand, "Internet access: the package-managers preset (or custom domains), GET/HEAD/OPTIONS only",
			"Secrets: none; put the environment's ID in [delegate] codex_env"}})

	cl := EnvCheck{Receiver: Claude, Status: "ok"}
	files, _ := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	found := ""
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil && strings.Contains(string(b), "anthropics/claude-code-action@") {
			found = f
			if !strings.Contains(string(b), SetupPath) {
				cl.Status, cl.Detail = "drift", filepath.Base(f)+" runs claude-code-action without "+SetupPath
				cl.Lines = []string{"add a step before the action: `- run: " + SetupCommand + "`"}
			}
		}
	}
	switch {
	case found == "":
		cl.Status, cl.Detail = "missing", "no workflow runs anthropics/claude-code-action; `/install-github-app` in Claude Code writes one"
	case cl.Status == "ok":
		cl.Detail = filepath.Base(found) + " runs " + SetupPath + " before claude-code-action"
	}
	return append(out, cl)
}

// olderVersion reports whether dotted version a is older than b.
func olderVersion(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(as), len(bs)); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}
