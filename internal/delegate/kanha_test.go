package delegate

import (
	"regexp"
	"slices"
	"testing"
)

// Story #1012: the local Kanha's start and resume commands, as each CLI
// takes them (checked against the CLIs' sources and help, 2026-10-27).

// Criterion 2: a local Codex Kanha s-123 is resumed with the merge event. The
// sandbox flags come before `resume`: Codex takes them from each run, and a
// read-only sandbox could not reach GitHub.
func TestCodexExecResumeArgs(t *testing.T) {
	event := "merged #40 (PR #41): run aayu mission next 889"
	want := []string{"exec", "--sandbox", "workspace-write", "-c", "sandbox_workspace_write.network_access=true", "resume", "s-123", event}
	if got := KanhaResume(Codex, "s-123", event); !slices.Equal(got, want) {
		t.Fatalf("codex resume: %q, want %q", got, want)
	}
	if got := KanhaStart(Codex, "", "brief"); !slices.Equal(got[:2], []string{"exec", "--json"}) || got[len(got)-1] != "brief" || slices.Contains(got, "resume") {
		t.Fatalf("codex start: %q", got)
	}
	// `codex exec --json` prints thread.started first; its thread_id is the session.
	out := `{"type":"thread.started","thread_id":"s-123"}` + "\n" + `{"type":"turn.started"}` + "\n" + `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"thread_id x"}}` + "\n"
	if got := KanhaSession(Codex, out); got != "s-123" {
		t.Fatalf("codex session %q from %q", got, out)
	}
	if got := KanhaSession(Codex, "Submitted https://chatgpt.com/codex/tasks/task_e_1\n"); got != "" {
		t.Fatalf("codex session from text output: %q, want none", got)
	}
}

// Criterion 3: a local Copilot Kanha p-456 is resumed with the block event.
// Copilot's -p takes the prompt as its value, so the event follows -p, and
// --resume names the session.
func TestCopilotResumeArgs(t *testing.T) {
	event := "blocked #63: spec unclear: run aayu mission next 889"
	want := []string{"-p", event, "--resume=p-456", "--allow-all-tools"}
	if got := KanhaResume(Copilot, "p-456", event); !slices.Equal(got, want) {
		t.Fatalf("copilot resume: %q, want %q", got, want)
	}
}

// Criterion 4: a Copilot mission's Kanha is a local `copilot -p` session,
// never a Copilot cloud task, which takes no follow-up: aayu picks its
// session ID (a UUID, the only kind Copilot takes for a new session), and
// Copilot is not a story receiver with a cloud adapter here (#917 adds those).
func TestCopilotCloudNeverCoordinates(t *testing.T) {
	if !slices.Contains(LocalKanha, Copilot) || slices.Contains(Receivers, Copilot) {
		t.Fatalf("LocalKanha %q, Receivers %q: want Copilot's Kanha local and no Copilot receiver", LocalKanha, Receivers)
	}
	id := NewSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) || id == NewSessionID() {
		t.Fatalf("session ID %q: want a fresh UUID v4", id)
	}
	want := []string{"-p", "brief", "--session-id=" + id, "--allow-all-tools"}
	if got := KanhaStart(Copilot, id, "brief"); !slices.Equal(got, want) {
		t.Fatalf("copilot start: %q, want %q", got, want)
	}
	if bins := LocalBins[Copilot]; !slices.Equal(bins, []string{"copilot"}) {
		t.Fatalf("copilot's CLI: %q", bins)
	}
}

// Criterion 5: a local OpenCode Kanha o-789 is resumed with the answer
// event, and OpenCode's stories run as local `opencode run` sessions.
func TestOpenCodeRunSessionArgs(t *testing.T) {
	event := "answered: #33 resumes"
	if got, want := KanhaResume(OpenCode, "o-789", event), []string{"run", "--session", "o-789", event}; !slices.Equal(got, want) {
		t.Fatalf("opencode resume: %q, want %q", got, want)
	}
	if got, want := KanhaStart(OpenCode, "", "brief"), []string{"run", "--format", "json", "brief"}; !slices.Equal(got, want) {
		t.Fatalf("opencode start: %q, want %q", got, want)
	}
	out := `{"type":"step_start","timestamp":1,"sessionID":"o-789","part":{}}` + "\n" + `{"type":"text","sessionID":"o-789"}` + "\n"
	if got := KanhaSession(OpenCode, out); got != "o-789" {
		t.Fatalf("opencode session %q", got)
	}
	if !slices.Contains(Receivers, OpenCode) || !slices.Contains(LocalKanha, OpenCode) {
		t.Fatalf("Receivers %q, LocalKanha %q: want OpenCode in both", Receivers, LocalKanha)
	}
	if got, want := LocalArgs(OpenCode, "opencode", "/wt", "the brief"), []string{"run", "the brief"}; !slices.Equal(got, want) {
		t.Fatalf("opencode story run: %q, want %q", got, want)
	}
}

// Criterion 8: with no Cursor API key, a Cursor Kanha is a local chat: aayu
// creates it (create-chat prints only its ID), gives it the brief, and
// resumes it with each event.
func TestCursorLocalFallback(t *testing.T) {
	if got := KanhaStart(Cursor, "", "brief"); !slices.Equal(got, []string{"create-chat"}) {
		t.Fatalf("cursor start: %q", got)
	}
	if got := KanhaSession(Cursor, "c-456\n"); got != "c-456" {
		t.Fatalf("cursor chat %q", got)
	}
	if got := KanhaSession(Cursor, "Error: Authentication required\n"); got != "" {
		t.Fatalf("cursor chat from an error: %q, want none", got)
	}
	event := "merged #40 (PR #41): run aayu mission next 889"
	got := KanhaResume(Cursor, "c-456", event)
	if want := []string{"-p", "--resume=c-456", "--force", "--trust", event}; !slices.Equal(got, want) {
		t.Fatalf("cursor resume: %q, want %q", got, want)
	}
}
