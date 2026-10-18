package initrepo

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// Story #487 criterion 1: the security workflow runs on push to exactly the
// mapped dev, rolling, and main, nightly, and on dispatch; never on a pull
// request; with the least permissions; and scans every mapped tip at night.
func TestSecurityWorkflowTriggers(t *testing.T) {
	type workflow struct {
		On struct {
			Push struct {
				Branches []string `yaml:"branches"`
			} `yaml:"push"`
			Schedule []map[string]string `yaml:"schedule"`
		} `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Concurrency map[string]any `yaml:"concurrency"`
			Strategy    struct {
				Matrix struct {
					Branch string `yaml:"branch"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	for _, c := range []struct {
		name     string
		branches []string
		want     []string
	}{
		{"default", nil, []string{"dev", "rolling", "main"}},
		{"mapped, no rolling", []string{"trunk", "", "prod"}, []string{"trunk", "prod"}},
		{"mapped", []string{"trunk", "canary", "prod"}, []string{"trunk", "canary", "prod"}},
		// A valid ref with YAML flow syntax stays one branch pattern.
		{"mapped, a comma in a name", []string{"release,2026", "canary", "prod"}, []string{"release,2026", "canary", "prod"}},
		// An apostrophe ends an expression's string literal unless doubled; a quote ends a JSON string unless escaped.
		{"mapped, an apostrophe and a quote", []string{"it's", `a"b`, "prod"}, []string{"it's", `a"b`, "prod"}},
	} {
		raw := string(GateWorkflows("v1.2.3", c.branches...)[SecurityWorkflow])
		var wf workflow
		if err := yaml.Unmarshal([]byte(raw), &wf); err != nil {
			t.Fatalf("%s: %v\n%s", c.name, err, raw)
		}
		if !slices.Equal(wf.On.Push.Branches, c.want) {
			t.Errorf("%s: push branches %v, want %v", c.name, wf.On.Push.Branches, c.want)
		}
		var on struct {
			On map[string]any `yaml:"on"`
		}
		_ = yaml.Unmarshal([]byte(raw), &on)
		keys := slices.Sorted(maps.Keys(on.On))
		if !slices.Equal(keys, []string{"push", "schedule", "workflow_dispatch"}) || len(wf.On.Schedule) != 1 || wf.On.Schedule[0]["cron"] == "" {
			t.Errorf("%s: triggers %v %+v", c.name, keys, wf.On)
		}
		want := map[string]string{"contents": "read", "issues": "write", "actions": "read"}
		if len(wf.Permissions) != len(want) {
			t.Errorf("%s: permissions %v", c.name, wf.Permissions)
		}
		for k, v := range want {
			if wf.Permissions[k] != v {
				t.Errorf("%s: permission %s = %q, want %q", c.name, k, wf.Permissions[k], v)
			}
		}
		// Every pushed range is scanned: a third push to a branch never replaces the second's pending scan.
		if cc := wf.Jobs["security"].Concurrency; cc["queue"] != "max" || cc["cancel-in-progress"] != false {
			t.Errorf("%s: concurrency %v drops a pending push scan", c.name, cc)
		}
		// The nightly list, read as GitHub reads it: the single-quoted literal (an apostrophe written twice), then JSON.
		m := wf.Jobs["security"].Strategy.Matrix.Branch
		lit := regexp.MustCompile(`\|\| fromJSON\('((?:[^']|'')*)'\) \}\}$`).FindStringSubmatch(m)
		var nightly []string
		if lit == nil || json.Unmarshal([]byte(strings.ReplaceAll(lit[1], "''", "'")), &nightly) != nil || !slices.Equal(nightly, c.want) {
			t.Errorf("%s: nightly matrix %q is not the branches %q", c.name, m, c.want)
		}
		// The pushed branch's name reaches JSON escaped, never pasted into a literal.
		if !strings.Contains(m, "fromJSON(format('[{0}]', toJSON(github.ref_name)))") {
			t.Errorf("%s: push matrix %q", c.name, m)
		}
		if !strings.Contains(raw, "aayudha@$AAYU_VERSION") || !strings.Contains(raw, `AAYU_VERSION: "v1.2.3"`) || regexp.MustCompile(`[^$]\{\{`).MatchString(raw) {
			t.Errorf("%s: unrendered or unpinned install:\n%s", c.name, raw)
		}
	}
}
