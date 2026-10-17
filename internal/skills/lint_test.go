package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// skillMD builds a SKILL.md with the generated keys, extra frontmatter lines, and a body.
func skillMD(name, desc, extra, body string) string {
	return "---\nname: " + name + "\ndescription: \"" + desc + "\"\ndisable-model-invocation: false\nuser-invocable: true\nversion: \"0.0.0\"\naayu:\n  seat: product-strategist\n" +
		extra + "---\n" + body
}

func lines(n int) string { return strings.Repeat("line\n", n) }

// good is a fixture that passes every rule; each bad case changes one thing.
func good() fstest.MapFS {
	return fstest.MapFS{
		"skills/README.md":                    {Data: []byte("# skills\n\nNot a skill; `aayu nonsense` here is ignored.\n")},
		"skills/ready/SKILL.md":               {Data: []byte(skillMD("ready", strings.Repeat("d", MaxDescription), "", "# ready\n\nRun `aayu ready 12 --check`, then:\n\n```sh\naayu --json proof list\naayu hook session-start\n```\n\nThe aayudha binary is not a call.\n"+lines(MaxBodyLines-10)))},
		"skills/ready/references/notes.md":    {Data: []byte("See `aayu status --all` and the [Mobbin page](https://mobbin.com/screens/123).\n")},
		"skills/ready/scripts/check.sh":       {Data: []byte("#!/bin/sh\n# Validate only.\nset -eu\nexec aayu ready \"$1\" --check\n")},
		"skills/ready/assets/diagram.png":     {Data: []byte("\x89PNG\r\n\x1a\nplain pixels")},
		"skills/status/SKILL.md":              {Data: []byte(skillMD("status", "Roll up status.", "hooks:\n  PreToolUse:\n    - matcher: \"Bash\"\n      hooks:\n        - type: command\n          command: \"aayu hook pre-tool --host claude\"\n", "# status\n"))},
		"skills/status/references/example.md": {Data: []byte("![chart](charts/rollup.svg)\n")},
	}
}

func TestLintGood(t *testing.T) {
	if fs := Lint(good(), "skills"); len(fs) != 0 {
		t.Fatalf("good fixture: %v", fs)
	}
}

// TestLintBad: each fixture breaks exactly one rule and fails with exactly that rule.
func TestLintBad(t *testing.T) {
	cases := []struct {
		name, rule string
		edit       func(fstest.MapFS)
	}{
		{"no frontmatter", RuleFrontmatter, func(m fstest.MapFS) { m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte("# status\n")} }},
		{"missing generated key", RuleFrontmatter, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(strings.Replace(skillMD("status", "Roll up.", "", "# status\n"), "version: \"0.0.0\"\n", "", 1))}
		}},
		{"missing SKILL.md", RuleFrontmatter, func(m fstest.MapFS) { m["skills/empty/references/x.md"] = &fstest.MapFile{Data: []byte("x\n")} }},
		{"two-word name", RuleName, func(m fstest.MapFS) {
			m["skills/self-check/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("self-check", "Check.", "", "# x\n"))}
		}},
		{"name differs from directory", RuleName, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("ready", "Roll up.", "", "# status\n"))}
		}},
		{"description 201 characters", RuleDescription, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("status", strings.Repeat("d", MaxDescription+1), "", "# status\n"))}
		}},
		{"body 501 lines", RuleBody, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("status", "Roll up.", "", lines(MaxBodyLines+1)))}
		}},
		{"unknown command in body", RuleCall, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("status", "Roll up.", "", "Run `aayu statuz`.\n"))}
		}},
		{"bad enum value in a reference", RuleCall, func(m fstest.MapFS) {
			m["skills/ready/references/notes.md"] = &fstest.MapFile{Data: []byte("```sh\naayu proof publish 12\n```\n")}
		}},
		{"unknown flag in a reference", RuleCall, func(m fstest.MapFS) {
			m["skills/ready/references/notes.md"] = &fstest.MapFile{Data: []byte("Use `aayu ready 12 --force`.\n")}
		}},
		{"script runs another program", RuleScript, func(m fstest.MapFS) {
			m["skills/ready/scripts/check.sh"] = &fstest.MapFile{Data: []byte("#!/bin/sh\ncurl https://example.com\n")}
		}},
		{"script pipes aayu output", RuleScript, func(m fstest.MapFS) {
			m["skills/ready/scripts/check.sh"] = &fstest.MapFile{Data: []byte("#!/bin/sh\naayu status --json | jq .\n")}
		}},
		{"script calls an unknown command", RuleCall, func(m fstest.MapFS) {
			m["skills/ready/scripts/check.sh"] = &fstest.MapFile{Data: []byte("#!/bin/sh\naayu deploy\n")}
		}},
		{"hook with inline shell", RuleHook, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("status", "Roll up.", "hooks:\n  PreToolUse:\n    - hooks:\n        - type: command\n          command: \"aayu hook pre-tool && rm -rf /tmp/x\"\n", "# status\n"))}
		}},
		{"prompt hook", RuleHook, func(m fstest.MapFS) {
			m["skills/status/SKILL.md"] = &fstest.MapFile{Data: []byte(skillMD("status", "Roll up.", "hooks:\n  Stop:\n    - hooks:\n        - type: prompt\n", "# status\n"))}
		}},
		{"Mobbin image file", RuleMobbin, func(m fstest.MapFS) {
			m["skills/ready/assets/ref.png"] = &fstest.MapFile{Data: []byte("\x89PNG\r\n\x1a\ntEXtSource\x00https://mobbin.com/screens/9")}
		}},
		{"Mobbin image embed", RuleMobbin, func(m fstest.MapFS) {
			m["skills/status/references/example.md"] = &fstest.MapFile{Data: []byte("![login](https://cdn.mobbin.com/screens/9.png)\n")}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := good()
			c.edit(m)
			got := Lint(m, "skills")
			if len(got) != 1 || got[0].Rule != c.rule {
				t.Fatalf("want exactly one %s finding, got %v", c.rule, got)
			}
		})
	}
}

// TestMobbinImagesAnywhere: outside skills the Mobbin rule still finds an
// image file of Mobbin origin and an image embed of a Mobbin URL, allows a
// plain link, and does not enter skipped directories.
func TestMobbinImagesAnywhere(t *testing.T) {
	m := fstest.MapFS{
		"docs/references.md":            {Data: []byte("Reference: [Monzo login](https://mobbin.com/screens/9).\n\n<img src=\"https://bytescale.mobbin.com/p/9.png\">\n")},
		"ledger/design/f1/ref.webp":     {Data: []byte("RIFF....WEBPVP8 mobbin.com/screens/9")},
		"design/own.png":                {Data: []byte("\x89PNG\r\n\x1a\n")},
		"node_modules/x/mobbin-cat.png": {Data: []byte("\x89PNG")},
	}
	got := MobbinImages(m, ".", "node_modules")
	if len(got) != 2 || got[0].Path != "docs/references.md" || got[1].Path != "ledger/design/f1/ref.webp" || got[0].Rule != RuleMobbin {
		t.Fatalf("findings = %v", got)
	}
}

func TestLintSymlink(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/status/SKILL.md", skillMD("status", "Roll up.", "", "# status\n"))
	if err := os.Symlink(filepath.Join(dir, "skills", "status"), filepath.Join(dir, "skills", "ready")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "skills", "status", "SKILL.md"), filepath.Join(dir, "skills", "status", "COPY.md")); err != nil {
		t.Fatal(err)
	}
	got := Lint(os.DirFS(dir), "skills")
	if len(got) != 2 || got[0].Rule != RuleSymlink || got[1].Rule != RuleSymlink {
		t.Fatalf("want two symlink findings (a symlinked skill and a symlinked file), got %v", got)
	}
}
