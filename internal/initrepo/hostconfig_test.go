package initrepo

import (
	"strings"
	"testing"
)

// The host-config block keeps every hand-written line, adds the owners a
// hand line already gives a path (the block's line is the last match there),
// sits before aayu seat's block, is stable on a re-run, and uninit removes
// only it.
func TestRenderHostConfigOwners(t *testing.T) {
	existing := "# owners\n/docs/  @writer\n/.claude/  @lead\n\n# BEGIN aayudha:seats (written by `aayu seat`)\n/internal/ui/  @ux\n# END aayudha:seats\n"
	got := RenderHostConfigOwners(existing, "founder")
	for _, want := range []string{"/docs/  @writer\n/.claude/  @lead\n", "/.mcp.json  @founder\n", "/.claude/  @founder @lead\n", "/.github/hooks/  @founder\n", "/.codex/  @founder\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, got)
		}
	}
	if strings.Index(got, HostConfigEnd) > strings.Index(got, "# BEGIN aayudha:seats") {
		t.Errorf("the host-config block is after the seats block:\n%s", got)
	}
	if again := RenderHostConfigOwners(got, "founder"); again != got {
		t.Errorf("a re-run changed it:\n%s", again)
	}
	if removed := RenderHostConfigOwners(got, ""); strings.Contains(removed, "host-config") || !strings.Contains(removed, "/.claude/  @lead") || !strings.Contains(removed, "/internal/ui/  @ux") {
		t.Errorf("removal:\n%s", removed)
	}
}
