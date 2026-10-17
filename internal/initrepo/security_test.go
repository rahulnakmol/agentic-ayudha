package initrepo

import (
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
			Strategy struct {
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
		nightly := `fromJSON('["` + strings.Join(c.want, `","`) + `"]')`
		if m := wf.Jobs["security"].Strategy.Matrix.Branch; !strings.Contains(m, nightly) {
			t.Errorf("%s: nightly matrix %q lacks %s", c.name, m, nightly)
		}
		if !strings.Contains(raw, "aayudha@$AAYU_VERSION") || !strings.Contains(raw, `AAYU_VERSION: "v1.2.3"`) || regexp.MustCompile(`[^$]\{\{`).MatchString(raw) {
			t.Errorf("%s: unrendered or unpinned install:\n%s", c.name, raw)
		}
	}
}
