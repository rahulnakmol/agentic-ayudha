package main

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// Story #1000, criterion 6: CONSTRAINTS.md, rendered from defaults.toml,
// lists [build] agents and the [mission] keys with their defaults, and none
// of the caps Decision #918 retired.
func TestConstraintsRows(t *testing.T) {
	var doc map[string]any
	if err := toml.Unmarshal(config.DefaultsTOML, &doc); err != nil {
		t.Fatal(err)
	}
	values := map[string]any{}
	flatten("", doc, values)
	out, err := render(values)
	if err != nil {
		t.Fatal(err)
	}
	for key, v := range map[string]string{"build.agents": "5", "mission.max_no_go": "3", "mission.retry_max": "2", "mission.retry_backoff_minutes": "10",
		"mission.check_minutes": "5", "mission.warn_hours": "24", "mission.max_hours": "48"} {
		if row := "| `" + key + "` | `" + v + "` |"; !strings.Contains(out, row) {
			t.Errorf("CONSTRAINTS.md has no row %q", row)
		}
	}
	for _, gone := range []string{"in_flight", "target_prs_month", "in_flight_max", "auto_default_hours", "review_hold_ratio"} {
		if strings.Contains(out, gone) {
			t.Errorf("CONSTRAINTS.md still names %s", gone)
		}
	}
}
