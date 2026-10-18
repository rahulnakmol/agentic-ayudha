package hosts

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// copilotLog is a Copilot session log in the shape of github/copilot-sdk's
// generated session-events schema (nodejs/src/generated/session-events.ts,
// read 2026-10-17); copilotCritique below is the shape Copilot CLI 1.0.98
// wrote. Auto answers with two families, then a HydraFusion Critique turn
// drafts with Claude, critiques with Gemini, and revises with Claude.
const copilotLog = `{"id":"1","parentId":null,"timestamp":"2026-10-17T10:00:00Z","type":"session.start","data":{"sessionId":"s1","copilotVersion":"1.0.98","producer":"copilot-agent","selectedModel":"auto","autoTier":"balance","startTime":"2026-10-17T10:00:00Z","version":1,"context":{"cwd":"/r"}}}
{"id":"2","parentId":"1","timestamp":"2026-10-17T10:00:05Z","type":"session.auto_mode_resolved","data":{"chosenModel":"claude-sonnet-4.5"}}
{"id":"3","parentId":"2","timestamp":"2026-10-17T10:00:09Z","type":"assistant.message","data":{"messageId":"m1","content":"plan","model":"Claude-Sonnet-4.5"}}
{"id":"4","parentId":"3","timestamp":"2026-10-17T10:01:00Z","type":"assistant.message","data":{"messageId":"m2","content":"code","model":"gpt-5.4"}}
{"id":"5","parentId":"4","timestamp":"2026-10-17T10:02:00Z","type":"session.fusion_resolved","data":{"contractVersion":1,"fusionId":"f1","pattern":"critique","policy":"default","primaryModel":"claude-opus-4.5","secondaryModel":"gemini-3-pro","critics":[{"model":"gemini-3-pro","phaseId":"p2"}],"fallbackModel":"gpt-5.4","followUpModel":"claude-opus-4.5","syntheticModel":"hydrafusion","turnId":"t3"}}
not json
{"id":"6","parentId":"5","timestamp":"2026-10-17T10:02:30Z","type":"assistant.message","data":{"messageId":"m3","content":"critique","model":"hydrafusion","fusion":{"fusionId":"f1","pattern":"critique","policy":"default","syntheticModel":"hydrafusion","phaseKind":"critic","role":"critic","sourceModel":"gemini-3-pro"}}}
{"id":"7","parentId":"6","timestamp":"2026-10-17T10:03:00Z","type":"assistant.message","data":{"messageId":"m4","content":"revised","model":"hydrafusion","fusion":{"fusionId":"f1","pattern":"critique","policy":"default","syntheticModel":"hydrafusion","phaseKind":"revision","sourceModel":"claude-opus-4.5"}}}
{"id":"8","parentId":"7","timestamp":"2026-10-17T10:03:01Z","type":"assistant.message","data":{"messageId":"m5","content":"","model":"claude-sonnet-4.5"}}`

// copilotCritique is a HydraFusion Critique turn as Copilot CLI 1.0.98 logged
// it on 2026-10-17 (contents dropped): the critic appears only in its phase
// and the plan's secondaryModel, never in an assistant.message.
const copilotCritique = `{"type":"session.start","data":{"selectedModel":"hydrafusion"}}
{"type":"session.fusion_resolved","data":{"pattern":"critique","primaryModel":"claude-opus-5.5","secondaryModel":"gpt-6-luna","critics":null}}
{"type":"assistant.fusion_phase_completed","data":{"model":"claude-opus-5.5","phaseKind":"draft","role":"solver"}}
{"type":"assistant.fusion_phase_completed","data":{"model":"gpt-6-luna","phaseKind":"critic","role":"critic"}}
{"type":"assistant.message","data":{"model":"claude-opus-5.5","fusion":{"phaseKind":"draft","role":"solver","sourceModel":"claude-opus-5.5"}}}
{"type":"session.fusion_completed","data":{"pattern":"critique"}}
`

// The builder's models are every concrete model Copilot reported for a
// response, most recent first, each once and lower-cased; a Critique
// critic is the reviewer, never a builder, and the synthetic HydraFusion
// model names no family.
func TestReadCopilotSession(t *testing.T) {
	m, err := ReadCopilotSession(strings.NewReader(copilotLog))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"claude-sonnet-4.5", "claude-opus-4.5", "gpt-5.4"}; !slices.Equal(m.Models, want) {
		t.Errorf("models %v, want %v", m.Models, want)
	}
	if want := []string{"gemini-3-pro"}; !slices.Equal(m.Critics, want) {
		t.Errorf("critics %v, want %v", m.Critics, want)
	}
	if got, want := m.Trailers(), "Aayudha-Host: copilot\nAayudha-Model: claude-sonnet-4.5\nAayudha-Model: claude-opus-4.5\nAayudha-Model: gpt-5.4\nAayudha-Critic-Model: gemini-3-pro"; got != want {
		t.Errorf("trailers:\n%s\nwant:\n%s", got, want)
	}
	if m, _ := ReadCopilotSession(strings.NewReader(copilotCritique)); !slices.Equal(m.Models, []string{"claude-opus-5.5"}) || !slices.Equal(m.Critics, []string{"gpt-6-luna"}) {
		t.Errorf("live critique: %+v", m)
	}
	// An older log whose Critique turn names only secondaryModel.
	old, _ := ReadCopilotSession(strings.NewReader(`{"type":"session.fusion_resolved","data":{"pattern":"critique","secondaryModel":"GPT-5.4"}}` + "\n" + `{"type":"session.fusion_resolved","data":{"pattern":"cascade","secondaryModel":"gemini-3-pro"}}`))
	if !slices.Equal(old.Critics, []string{"gpt-5.4"}) || old.Models != nil {
		t.Errorf("older critique: %+v", old)
	}
}

// build finds the session log of the Copilot session that ran it, only a
// recent one, and only once.
func TestCopilotSessionLog(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	now := time.Now()
	if got := CopilotSessionLog(root, home, time.Hour, now); got != "" {
		t.Errorf("no record: %q", got)
	}
	recordCopilotSession(root, []byte(`{"sessionId":"abc-1","cwd":"/r"}`), "aayu build 4")
	if got, want := CopilotSessionLog(root, home, time.Hour, now), filepath.Join(home, "session-state", "abc-1", "events.jsonl"); got != want {
		t.Errorf("log %q, want %q", got, want)
	}
	if got := CopilotSessionLog(root, home, time.Hour, now); got != "" {
		t.Errorf("a second build took the same note: %q", got)
	}
	recordCopilotSession(root, []byte(`{"sessionId":"abc-1","cwd":"/r"}`), "aayu build 4")
	if got := CopilotSessionLog(root, home, time.Hour, now.Add(2*time.Hour)); got != "" {
		t.Errorf("stale record: %q", got)
	}
	recordCopilotSession(root, []byte(`{"sessionId":"abc-2","toolName":"aayudha-build","toolArgs":{"issue":4}}`), "")
	if got := CopilotSessionLog(root, home, time.Hour, now); !strings.HasSuffix(got, filepath.Join("abc-2", "events.jsonl")) {
		t.Errorf("the MCP build tool's session: %q", got)
	}
	recordCopilotSession(root, []byte(`{"sessionId":"abc-3","toolName":"aayudha-status"}`), "")
	if got := CopilotSessionLog(root, home, time.Hour, now); got != "" {
		t.Errorf("a status call noted a build: %q", got)
	}
	if got := CopilotHome("/h", func(k string) string { return map[string]string{"COPILOT_HOME": "/c"}[k] }); got != "/c" {
		t.Errorf("COPILOT_HOME: %q", got)
	}
}

// Copilot CLI trusts a folder and everything under it, from a config.json
// that starts with comment lines; a sibling with the same prefix is not
// trusted.
func TestCopilotTrusted(t *testing.T) {
	home, base := t.TempDir(), t.TempDir()
	repo := filepath.Join(base, "repo")
	cfg := "// User settings belong in settings.json.\n// This file is managed automatically.\n{\n  \"trustedFolders\": [" + strconv.Quote(repo+"2") + "]\n}\n"
	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if CopilotTrusted(repo, home) {
		t.Error("a sibling folder's trust counted")
	}
	cfg = strings.Replace(cfg, strconv.Quote(repo+"2"), strconv.Quote(base), 1)
	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !CopilotTrusted(repo, home) {
		t.Error("the parent folder's trust did not count")
	}
}

// Install removes a Copilot command skill it wrote and no longer ships, and
// leaves a person's own .github/skills/aayudha-* skill alone.
func TestCopilotInstallKeepsPersonsSkills(t *testing.T) {
	tmp := t.TempDir()
	root, home := filepath.Join(tmp, "repo"), filepath.Join(tmp, "home")
	write(t, root, ".github/skills/aayudha-ours/SKILL.md", "---\nname: aayudha-ours\n---\nour own skill\n")
	write(t, root, ".github/skills/aayudha-retired/SKILL.md", "<!-- The aayudha command `retired`, "+copilotCommandMark+". -->\n")
	o := opts(t, root, home, false, verbs.HostCopilot)
	res, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(o, res.Edits, true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, ".github/skills/aayudha-ours/SKILL.md"); !strings.Contains(got, "our own skill") {
		t.Errorf("a person's skill changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "skills", "aayudha-retired", "SKILL.md")); err == nil {
		t.Error("a command install wrote and no longer ships was kept")
	}
}
