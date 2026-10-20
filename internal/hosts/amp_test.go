package hosts

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Only Amp's Agent rows name the builder, without the provider prefix, each
// once in table order, and only from the Model Routing table. A table aayu
// cannot read names no model rather than a guess.
func TestAmpAgentModels(t *testing.T) {
	for _, c := range []struct {
		name, usage string
		want        []string
	}{
		{"captured", readFixture(t, "amp-usage.txt"), []string{"claude-opus-5-5"}},
		{"several agent models, columns reordered", readFixture(t, "amp-usage-multi.txt"), []string{"claude-sonnet-5-5", "gpt-6.1-sol"}},
		{"no routing table", readFixture(t, "amp-usage-notable.txt"), nil},
		{"no Purpose column", "## Model Routing\n\n| Model ID | Routing |\n| --- | --- |\n| anthropic/claude-opus-5-5 | Agent |\n", nil},
		{"a cell that is not one token", "## Model Routing\n\n| Model ID | Purpose |\n| --- | --- |\n| claude amp medium | Agent |\n", nil},
		{"empty", "", nil},
	} {
		if got := AmpAgentModels(c.usage); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// The effort is agentMode in the frontmatter only, never a line of the
// conversation below it.
func TestAmpAgentMode(t *testing.T) {
	for _, c := range []struct{ name, md, want string }{
		{"captured", readFixture(t, "amp-thread.txt"), "medium"},
		{"no frontmatter", "# Title\n\nagentMode: high\n", ""},
		{"no agentMode", "---\ntitle: x\n---\n\nagentMode: high\n", ""},
		{"quoted", "---\nagentMode: \"xhigh\"\n---\n", "xhigh"},
		{"not one token", "---\nagentMode: very high\n---\n", ""},
	} {
		if got := AmpAgentMode(c.md); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
