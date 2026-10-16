package hosts

import (
	"bytes"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rahulnakmol/agentic-ayudha/internal/skills"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

var update = flag.Bool("update", false, "rewrite the golden trees in testdata/golden")

// fixtureAssets is a two-skill tree plus the real Amp plugin template, so the
// golden trees do not change whenever a shipped skill does.
func fixtureAssets(t *testing.T) fstest.MapFS {
	t.Helper()
	tmpl, err := os.ReadFile("../../hosts/amp/aayudha.ts")
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{
		"skills/README.md":                     {Data: []byte("# skills\n")},
		"skills/alpha/SKILL.md":                {Data: []byte("---\nname: alpha\ndescription: \"First fixture skill.\"\n---\n\n# alpha\n\nRun `aayu status`.\n")},
		"skills/alpha/references/QUESTIONS.md": {Data: []byte("# Questions\n\n1. What is the goal?\n")},
		"skills/beta/SKILL.md":                 {Data: []byte("---\nname: beta\ndescription: \"Second fixture skill.\"\n---\n\n# beta\n\nRun `aayu ready 1 --check`.\n")},
		"hosts/amp/aayudha.ts":                 {Data: tmpl},
	}
}

func opts(t *testing.T, root, home string, windows bool, hosts ...verbs.HostID) Options {
	bin := "/opt/aayudha/bin/aayu"
	if windows {
		bin = `C:\Program Files\aayudha\aayu.exe`
	}
	return Options{Root: root, Home: home, Hosts: hosts, Assets: fixtureAssets(t), Binary: bin,
		Version: "1.2.3", SkillsVersion: "0.0.0", Windows: windows}
}

// snapshot reads every file under dir as slash path -> content, failing on symlinks.
func snapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			t.Errorf("%s is a symlink; install writes real copies only (V2)", p)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return out
}

// TestGoldenTrees is the WP3.2 acceptance: one golden tree per host, on Unix and
// on Windows (CRLF, no symlinks), and a second apply changes nothing.
func TestGoldenTrees(t *testing.T) {
	for _, h := range AllHosts() {
		for _, osName := range []string{"unix", "windows"} {
			t.Run(string(h)+"-"+osName, func(t *testing.T) {
				tmp := t.TempDir()
				root, home := filepath.Join(tmp, "repo"), filepath.Join(tmp, "home")
				for _, d := range []string{root, home} {
					if err := os.MkdirAll(d, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				o := opts(t, root, home, osName == "windows", h)
				res, err := Plan(o)
				if err != nil {
					t.Fatal(err)
				}
				if err := Apply(o, res.Edits, true); err != nil {
					t.Fatal(err)
				}
				got := map[string][]byte{}
				for p, b := range snapshot(t, root) {
					got["repo/"+p] = b
				}
				for p, b := range snapshot(t, home) {
					got["home/"+p] = b
				}
				for p, b := range got {
					crlf, lfs := bytes.Count(b, []byte("\r\n")), bytes.Count(b, []byte("\n"))
					if osName == "windows" && crlf != lfs {
						t.Errorf("%s: %d of %d line endings are CRLF, want all", p, crlf, lfs)
					}
					if osName == "unix" && crlf != 0 {
						t.Errorf("%s: has CRLF line endings", p)
					}
				}

				golden := filepath.Join("testdata", "golden", string(h)+"-"+osName)
				if *update {
					_ = os.RemoveAll(golden)
					for p, b := range got {
						fp := filepath.Join(golden, filepath.FromSlash(p))
						if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(fp, b, 0o644); err != nil {
							t.Fatal(err)
						}
					}
				}
				want := snapshot(t, golden)
				if len(want) == 0 {
					t.Fatalf("no golden tree at %s; run go test ./internal/hosts -update", golden)
				}
				for p, b := range want {
					if !bytes.Equal(got[p], b) {
						t.Errorf("%s differs from %s:\n got: %q\nwant: %q", p, golden, got[p], b)
					}
				}
				for p := range got {
					if _, ok := want[p]; !ok {
						t.Errorf("%s is not in the golden tree", p)
					}
				}

				again, err := Plan(o)
				if err != nil {
					t.Fatal(err)
				}
				if len(again.Edits) != 0 {
					t.Errorf("re-apply is not a no-op: %s", Render(again.Edits, func(a, b string) string { return "" }))
				}
				if problems := CheckCopies(root); len(problems) != 0 {
					t.Errorf("copies out of sync right after install: %v", problems)
				}
			})
		}
	}
}

func write(t *testing.T, root, rel, s string) {
	t.Helper()
	fp := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestInstallIntoExistingRepo: the product's own files, hooks, and skills
// survive; a symlinked .claude/skills (the old V2 setup) becomes real copies; a
// skill aayudha no longer ships is removed; drift is detected.
func TestInstallIntoExistingRepo(t *testing.T) {
	tmp := t.TempDir()
	root, home := filepath.Join(tmp, "repo"), filepath.Join(tmp, "home")
	write(t, root, "AGENTS.md", "# Product\n\nOur rules.\n")
	write(t, root, "CLAUDE.md", "# Claude notes\n")
	write(t, root, ".claude/settings.json", `{"model": "opus", "hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "./lint.sh"}]}]}}`)
	write(t, root, ".agents/skills/ours/SKILL.md", "product's own skill\n")
	write(t, root, ".agents/skills/gamma/SKILL.md", "installed by an older aayu\n")
	write(t, root, LockPath, "version = 1\nskills_version = \"0.0.0\"\nhosts = [\"amp\"]\n[skills]\ngamma = { version = \"0.0.0\", sha256 = \"x\" }\n")
	write(t, home, AmpSettings, `{"amp.dangerouslyAllowAll": true}`)
	symlinked := os.Symlink(filepath.Join(root, ".agents", "skills"), filepath.Join(root, ".claude", "skills")) == nil

	o := opts(t, root, home, false, verbs.HostClaude)
	res, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(o, res.Edits, true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, "AGENTS.md"); !strings.HasPrefix(got, "# Product\n\nOur rules.\n\n<!-- BEGIN aayudha:skills -->") {
		t.Errorf("AGENTS.md lost the product's text:\n%s", got)
	}
	if got := read(t, root, "CLAUDE.md"); got != "# Claude notes\n\n@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q", got)
	}
	settings := read(t, root, ".claude/settings.json")
	for _, want := range []string{`"model": "opus"`, `"command": "./lint.sh"`, `"command": "\"/opt/aayudha/bin/aayu\" hook pre-tool --host claude"`, `"enabledMcpjsonServers": [`} {
		if !strings.Contains(settings, want) {
			t.Errorf(".claude/settings.json lacks %s:\n%s", want, settings)
		}
	}
	if got := read(t, root, ".agents/skills/ours/SKILL.md"); got != "product's own skill\n" {
		t.Errorf("product skill changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "gamma")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale aayudha skill gamma was not removed: %v", err)
	}
	if symlinked {
		if fi, err := os.Lstat(filepath.Join(root, ".claude", "skills")); err != nil || fi.Mode()&fs.ModeSymlink != 0 || !fi.IsDir() {
			t.Errorf(".claude/skills is still a symlink or missing: %v", err)
		}
	}
	// Hosts accumulate: amp from the old lock, claude from this run. Amp's home settings kept their key.
	if lock, _, _ := ReadLock(root); strings.Join(lock.Hosts, ",") != "amp,claude" {
		t.Errorf("lock hosts = %v, want amp,claude", lock.Hosts)
	}
	if got := read(t, home, AmpSettings); !strings.Contains(got, `"amp.dangerouslyAllowAll": true`) || !strings.Contains(got, `"aayudha": {`) {
		t.Errorf("Amp settings:\n%s", got)
	}
	if problems := CheckCopies(root); len(problems) != 0 {
		t.Fatalf("copies out of sync after install: %v", problems)
	}

	// A re-run keeps the product's hook once and ours once.
	res, _ = Plan(o)
	if len(res.Edits) != 0 {
		t.Fatalf("second plan: %s", Render(res.Edits, func(a, b string) string { return b }))
	}
	write(t, root, ".claude/skills/beta/SKILL.md", "edited by hand\n")
	if problems := CheckCopies(root); len(problems) != 1 || !strings.Contains(problems[0], ".claude/skills/beta") {
		t.Fatalf("hand edit not detected: %v", problems)
	}
	fix, _ := Plan(Options{Root: root, Home: home, Assets: o.Assets, Binary: o.Binary, Version: o.Version, SkillsVersion: o.SkillsVersion, SkillsOnly: true})
	if len(fix.Edits) != 1 || fix.Edits[0].Path != ".claude/skills/beta/SKILL.md" {
		t.Fatalf("skills-only re-sync plan: %+v", fix.Edits)
	}
}

func TestPin(t *testing.T) {
	root := t.TempDir()
	o := opts(t, root, t.TempDir(), false)
	o.Pin = "0.0.1"
	if _, err := Plan(o); !errors.Is(err, ErrPinned) {
		t.Fatalf("--pin to a version this binary does not carry: %v", err)
	}
	write(t, root, LockPath, "version = 1\npin = \"0.0.1\"\nhosts = [\"amp\"]\n")
	o.Pin = ""
	if _, err := Plan(o); !errors.Is(err, ErrPinned) {
		t.Fatalf("lock pinned to another version: %v", err)
	}
	o.Pin = "0.0.0"
	res, err := Plan(o)
	if err != nil || res.Lock.Pin != "0.0.0" {
		t.Fatalf("moving the pin: %v, %+v", err, res)
	}
}

func TestHooks(t *testing.T) {
	dir := t.TempDir()
	pause := filepath.Join(dir, "paused")
	for _, h := range AllHosts() {
		r, err := RunHook("pre-tool", h, "", pause, nil, "")
		if err != nil || r.Code != 0 {
			t.Errorf("%s pre-tool, not paused: %+v %v", h, r, err)
		}
	}
	write(t, dir, "paused", "spend reached 100.00 USD")
	for _, h := range AllHosts() {
		r, _ := RunHook("pre-tool", h, "", pause, nil, "")
		if r.Code != 2 || !strings.Contains(r.Stderr, "spend reached 100.00 USD") {
			t.Errorf("%s pre-tool, paused: want exit 2 naming the breach, got %+v", h, r)
		}
	}
	if r, _ := RunHook("pre-tool", verbs.HostCursor, "", pause, nil, ""); !strings.Contains(r.Stdout, `"permission":"deny"`) {
		t.Errorf("cursor deny JSON: %q", r.Stdout)
	}
	if _, err := RunHook("post-tool", verbs.HostClaude, "", pause, nil, ""); err == nil {
		t.Error("unknown hook accepted")
	}
	for _, h := range Hooks {
		cmd := HookCommand("/x/aayu", h.Name, verbs.HostClaude)
		if !ourHook.MatchString(cmd) || !skills.HookCommand.MatchString(strings.Replace(cmd, `"/x/aayu"`, "aayu", 1)) {
			t.Errorf("hook command %q is not `aayu hook <name>`", cmd)
		}
	}
}
