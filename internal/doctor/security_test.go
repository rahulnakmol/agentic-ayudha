package doctor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Story #487 criterion 5: the security-workflow row fails when the workflow
// is missing, names each mapped tip's latest push run, and warns on a red tip
// and on a nightly run more than 48 hours old.
func TestSecurityWorkflowRow(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	run := func(conclusion string, at time.Time) map[string]any {
		return map[string]any{"head_sha": "abcdef1234567", "status": "completed", "conclusion": conclusion, "created_at": at}
	}
	probe := func(installed bool, sets []string, runs map[string]map[string]any) (*Probe, *[]string) {
		files := map[string]string{"go.mod": "module m\n"}
		if installed {
			files[".github/workflows/"+SecurityWorkflow] = "name: aayudha-security\n"
		}
		var asked []string
		p := &Probe{Root: repoWith(t, files), Config: load(t, sets...), Repo: "o/r", Now: func() time.Time { return now },
			API: func(_ context.Context, path string, out any) error {
				asked = append(asked, path)
				key := path[strings.Index(path, "event="):]
				var rs []map[string]any
				if r, ok := runs[key]; ok {
					rs = append(rs, r)
				}
				b, _ := json.Marshal(map[string]any{"workflow_runs": rs})
				return json.Unmarshal(b, out)
			}}
		return p, &asked
	}
	green := map[string]map[string]any{
		"event=push&branch=dev":     run("success", now.Add(-time.Hour)),
		"event=push&branch=rolling": run("success", now.Add(-time.Hour)),
		"event=push&branch=main":    run("success", now.Add(-time.Hour)),
		"event=schedule":            run("success", now.Add(-9*time.Hour)),
	}
	ctx := context.Background()

	p, _ := probe(false, nil, green)
	if r := checkSecurityWorkflow(ctx, p); r.Status != Fail || !strings.Contains(r.Detail, "aayu init --gates") {
		t.Fatalf("missing: %+v", r)
	}
	p, _ = probe(true, nil, green)
	if r := checkSecurityWorkflow(ctx, p); r.Status != Pass || len(r.Lines) != 4 || !strings.Contains(r.Lines[2], "main: success at abcdef1") {
		t.Fatalf("all green: %+v", r)
	}
	red := map[string]map[string]any{}
	for k, v := range green {
		red[k] = v
	}
	red["event=push&branch=main"] = run("failure", now.Add(-time.Hour))
	p, _ = probe(true, nil, red)
	if r := checkSecurityWorkflow(ctx, p); r.Status != Warn || !strings.Contains(r.Detail, "1 branch tip(s) failed") || !strings.Contains(r.Lines[2], "main: failure") {
		t.Fatalf("red main: %+v", r)
	}
	stale := map[string]map[string]any{}
	for k, v := range green {
		stale[k] = v
	}
	stale["event=schedule"] = run("success", now.Add(-49*time.Hour))
	p, _ = probe(true, nil, stale)
	if r := checkSecurityWorkflow(ctx, p); r.Status != Warn || !strings.Contains(r.Lines[3], "more than 48 hours ago") {
		t.Fatalf("stale nightly: %+v", r)
	}
	// Mapped branches (coexistence, no rolling): the rows follow the map.
	p, asked := probe(true, []string{"branches.dev=trunk", "branches.main=prod"}, green)
	r := checkSecurityWorkflow(ctx, p)
	if got := strings.Join(*asked, " "); !strings.Contains(got, "branch=trunk") || !strings.Contains(got, "branch=prod") || strings.Contains(got, "branch=rolling") || strings.Contains(got, "branch=dev") {
		t.Fatalf("mapped branches asked: %s (%+v)", got, r)
	}
}
