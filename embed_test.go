package aayudha

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/skills"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TestPrivateRecordsStayOut keeps the founder's decision of 2026-10-17: this repository is public, so the research
// notes and the Appendix B decision log are archived in the private repository rahulnakmol/agentic-ayudha-research,
// and the recorded Bug #173 halt runs and the Appendix B generator are deleted. None of them may be tracked here again.
func TestPrivateRecordsStayOut(t *testing.T) {
	out, err := exec.Command("git", "ls-files", "--", "docs/research", "docs/appendix-b", "testdata/workflow-halt", "internal/appendixb").Output()
	if err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	for _, f := range strings.Fields(string(out)) {
		t.Errorf("%s is tracked: research notes and the Appendix B log are archived in the private repository "+
			"rahulnakmol/agentic-ayudha-research, and testdata/workflow-halt and internal/appendixb were deleted. "+
			"Cite a note as \"private research note <file>\"; record a deviation in the PR description under "+
			"\"Deviation from the plan:\"", f)
	}
}

// TestSkillLint runs the WP3.1 skill lint (internal/skills) over the embedded skills/ tree, which is what
// ships in the binary and what `aayu install` copies. CI runs it in the `build` job. It replaces the WP0.4
// subset (Appendix B D-0.4e).
func TestSkillLint(t *testing.T) {
	if _, err := fs.Stat(Assets, "skills/ready/SKILL.md"); err != nil {
		t.Fatalf("embedded skills tree: %v", err)
	}
	for _, f := range skills.Lint(Assets, "skills") {
		t.Error(f)
	}
}

// TestMissionSkillProtocolSection (Story #1005): the mission skill holds Kanha's protocol, which every tool's Kanha
// and #911's skill follow. On each event it runs recover (Story #1003), then the list, builds each start once on the mission's tool, reports one
// line, and stops; with the list's error it starts nothing. It names ship, merge, and approval only as never-steps.
func TestMissionSkillProtocolSection(t *testing.T) {
	b, err := fs.ReadFile(Assets, "skills/mission/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(b), "\n## Kanha's protocol\n")
	if !ok {
		t.Fatal(`skills/mission/SKILL.md has no "## Kanha's protocol" section`)
	}
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	steps := []string{"1. Run `aayu mission recover <f>`.", "2. Run `aayu mission next <f>`.", "run `aayu build <n> --delegate <tool>` once for each line `start #<n>`",
		"4. Report one line, then stop.", "If `recover` or `next` exits with an error, start nothing.", "Exit 4: the repository is halted",
		"In a Tatkal Feature", "Start only what the list marks `start`, each once."}
	for i, s := range steps {
		at := strings.Index(section, s)
		if at < 0 || i > 0 && at < strings.Index(section, steps[i-1]) {
			t.Errorf("Kanha's protocol lacks %q, or has it out of order", s)
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(section, "\n  ", " "), "\n") {
		if strings.Contains(line, "aayu ship") || strings.Contains(line, "--approve") || strings.Contains(line, "pr merge") {
			if !strings.HasPrefix(line, "- Never merge, approve, ship, or skip a gate.") {
				t.Errorf("Kanha's protocol names ship, merge, or approval outside its never rule: %q", line)
			}
		}
	}
}

// TestNoMobbinOriginImages runs the Mobbin-origin image rule over the whole repository, not only the
// skills tree (WP6.4: lint blocks Mobbin-origin images anywhere). CI runs it in the `build` job.
func TestNoMobbinOriginImages(t *testing.T) {
	for _, f := range skills.MobbinImages(os.DirFS("."), ".", ".git", "node_modules") {
		t.Error(f)
	}
}

// TestPersonaEverywhere (WP5.1b): every skill body carries its Accountable seat's persona block exactly as
// rendered from seats/<seat>/persona.toml, `aayu install` copies it byte for byte to every host, and AGENTS.md
// carries the same seat section the generator renders.
func TestPersonaEverywhere(t *testing.T) {
	personas, err := seats.LoadPersonas(Assets)
	if err != nil {
		t.Fatal(err)
	}
	src := map[string][]byte{}
	for _, c := range verbs.Verbs {
		if !c.Has(verbs.SurfaceSkill) {
			continue
		}
		b, err := fs.ReadFile(Assets, "skills/"+c.Name+"/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		src[c.Name] = b
		region := "<!-- BEGIN GENERATED: persona -->\n" + personas[c.Seat].SkillBlock(c, personas[verbs.PolicyOwner(c)]) + "<!-- END GENERATED: persona -->"
		if n := strings.Count(string(b), region); n != 1 {
			t.Errorf("skills/%s/SKILL.md carries the %s persona block %d times, want 1", c.Name, c.Seat, n)
		}
	}

	tmp := t.TempDir()
	root, home := filepath.Join(tmp, "repo"), filepath.Join(tmp, "home")
	for _, d := range []string{root, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	res, err := hosts.Plan(hosts.Options{Root: root, Home: home, Assets: Assets, Binary: "/bin/aayu", Version: "t", SkillsVersion: verbs.SkillVersion})
	if err != nil {
		t.Fatal(err)
	}
	copies := map[string]int{}
	for _, e := range res.Edits {
		name, ok := strings.CutSuffix(e.Path, "/SKILL.md")
		if e.Kind != hosts.Write || !ok {
			continue
		}
		name = path.Base(name)
		if want, ok := src[name]; ok {
			copies[name]++
			if !bytes.Equal(e.New, want) {
				t.Errorf("%s differs from skills/%s/SKILL.md", e.Path, name)
			}
		}
	}
	dirs := map[string]bool{} // hosts that read the same skill directory share one copy
	for _, h := range hosts.AllHosts() {
		dirs[hosts.SkillDir(h)] = true
	}
	for name := range src {
		if copies[name] != len(dirs) {
			t.Errorf("skills/%s/SKILL.md was installed %d times, want once per skill directory (%d)", name, copies[name], len(dirs))
		}
	}

	agents, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if body := strings.Trim(seats.AgentsSection(personas).Body, "\n"); !strings.Contains(string(agents), body) {
		t.Error("AGENTS.md does not carry the rendered seat section (go generate ./...)")
	}
}
