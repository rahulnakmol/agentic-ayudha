package verbs

import "testing"

// A known ID reads as its display name in either spelling; an unknown one
// shows as written, never a guess from its shape.
func TestModelName(t *testing.T) {
	for id, want := range map[string]string{
		ModelClaudeOpus:     "Claude Opus 5.5",
		"Claude-Opus-5-5":   "Claude Opus 5.5",
		CopilotClaudeSonnet: "Claude Sonnet 5.5",
		ModelGPTSol:         "GPT-6.1 Sol",
		"claude-amp-medium": "claude-amp-medium",
		"claude-opus-9-9":   "claude-opus-9-9",
	} {
		if got := ModelName(id); got != want {
			t.Errorf("ModelName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestBuilderName(t *testing.T) {
	for _, c := range []struct {
		models       []string
		effort, want string
	}{
		{[]string{ModelClaudeOpus}, "medium", "Claude Opus 5.5, medium effort"},
		{[]string{CopilotClaudeOpus, "gpt-6-luna"}, "", "Claude Opus 5.5 and gpt-6-luna"},
		{nil, "medium", "model not reported, medium effort"},
		{nil, "", "model not reported"},
	} {
		if got := BuilderName(c.models, c.effort); got != c.want {
			t.Errorf("BuilderName(%q, %q) = %q, want %q", c.models, c.effort, got, c.want)
		}
	}
}
