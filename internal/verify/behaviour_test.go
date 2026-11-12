package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// fakeControl is a control tool in shell: it answers each check by its ID,
// writes what the check would capture, and records every call in calls.log.
const fakeControl = `in=$(cat)
out=$(printf '%s' "$in" | sed -n 's/.*"out":"\([^"]*\)".*/\1/p')
id=$(printf '%s' "$in" | sed -n 's/.*"check":{"id":"\([^"]*\)".*/\1/p')
echo "$id" >> calls.log
case "$id" in
pass) printf '{"argv":["list"],"exit":0}\n' > "$out/trace.jsonl"; echo '{"status":"passed","artifacts":[{"path":"trace.jsonl","kind":"api-trace"}]}' ;;
shot) printf 'PNG' > "$out/page.png"; echo '{"status":"passed","artifacts":[{"path":"page.png","kind":"screenshot"}]}' ;;
away) echo '{"status":"unreachable","reason":"the seed launch has no events"}' ;;
fail) echo '{"status":"failed","reason":"step 1: exit 2, want 0"}'; exit 1 ;;
escape) echo '{"status":"passed","artifacts":[{"path":"../../secret","kind":"report"}]}' ;;
esac
`

func check(id, kind, evidence string) string {
	return "```check\n{\"id\": \"" + id + "\", \"kind\": \"" + kind + "\", \"evidence\": \"" + evidence + "\", \"steps\": [{\"run\": [\"" + id + "\"]}]}\n```\n\n"
}

func feature(id, source string, checks ...string) string {
	return "# " + id + "\n\nWhat it does.\n\n```feature\n{\"id\": \"" + id + "\", \"sources\": [\"" + source + "\"], \"verified_at\": \"\"}\n```\n\n" +
		"## Sub-features\n\n## How to reach it\n\n## Behaviour checks\n\n" + strings.Join(checks, "") + "## Gotchas\n"
}

// behaviourRepo has two features: tasks (app/main.go; a passing cli check, a
// passing ui check, an unreachable api check) and export (app/export.go; a
// failing cli check). It returns the root and the first commit.
func behaviourRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := ".agents/skills/verify-tasks/"
	root, base, _ := repo(t, map[string]string{"app/main.go": "package main\n", "app/export.go": "package main\n"}, map[string]string{
		dir + "control.sh":         fakeControl,
		dir + "features/README.md": "# map\n\n```map\n{\"product\": \"tasks\", \"control\": [\"sh\", \"" + dir + "control.sh\"]}\n```\n\n1. [Tasks](tasks.md)\n2. [Export](export.md)\n",
		dir + "features/tasks.md":  feature("tasks", "app/main.go", check("pass", "cli", "api-trace"), check("shot", "ui", "screenshot"), check("away", "api", "api-trace")),
		dir + "features/export.md": feature("export", "app/export.go", check("fail", "cli", "api-trace")),
		".aayudha/evidence/7.toml": "[[criterion]]\nid = 1\nchecks = [\"tasks.pass\"]\n\n[[criterion]]\nid = 2\nchecks = [\"tasks.pass\"]\n\n[[criterion]]\nid = 3\nchecks = [\"tasks.shot\"]\n",
	})
	return root, base
}

func behaviourRun(t *testing.T, root string, changed ...string) *Run {
	t.Helper()
	return &Run{Root: root, Changed: changed, Work: t.TempDir(), Story: 7,
		Criteria: []workitems.Criterion{{N: 1, Evidence: "api-trace"}, {N: 2, Evidence: "screenshot"}, {N: 3, Evidence: "screenshot"}},
		Verify:   config.Verify{Environments: map[string]config.Environment{"local": {URL: "http://127.0.0.1:9999"}}},
		Sandbox:  &Sandbox{Home: t.TempDir()}}
}

func calls(t *testing.T, root string) []string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(root, "calls.log"))
	_ = os.Remove(filepath.Join(root, "calls.log"))
	return strings.Fields(strings.TrimSpace(string(b)))
}

// TestBehaviourFloor (WP4.5b.2 accept): the checks a change touches and the
// criteria map run through the control tool; passing checks give hashed
// artifacts mapped only to criteria of their own kind; an unreachable check
// proves nothing and fails nothing; a failing check fails the floor; cli
// checks run without network and ui and api checks with it.
func TestBehaviourFloor(t *testing.T) {
	ctx := context.Background()
	root, _ := behaviourRepo(t)

	r := behaviourRun(t, root, "app/main.go")
	rep := Execute(ctx, r, []Floor{Behaviour{}})
	c := rep.Checks[0]
	if c.Result != proof.Pass || c.Value != 0 || !rep.Hold {
		t.Fatalf("touched tasks: %+v", c)
	}
	for _, want := range []string{"2 of 3 checks passed", "unreachable, proving nothing: tasks.away (the seed launch has no events)", "criterion 2: tasks.pass produced no screenshot"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail %q lacks %q", c.Detail, want)
		}
	}
	var got []string
	for _, a := range rep.Artifacts {
		got = append(got, fmt.Sprintf("%s %s %v", a.Path, a.Kind, a.Criteria))
	}
	want := []string{"behaviour/tasks.pass/trace.jsonl api-trace [1]", "behaviour/tasks.shot/page.png screenshot [3]", "reports/behaviour.json report []"}
	if !slices.Equal(got, want) {
		t.Fatalf("artifacts %q, want %q", got, want)
	}
	pack, _ := Pack(r, rep, proof.Pack{})
	if u := Unmapped(r, pack); !slices.Equal(u, []int{2}) {
		t.Fatalf("unmapped %v, want [2] (a recording or screenshot criterion is not proved by an api-trace)", u)
	}
	// export was not touched, so its failing check did not run.
	if log := calls(t, root); !slices.Equal(log, []string{"pass", "shot", "away"}) {
		t.Fatalf("calls %q, want the three tasks checks", log)
	}
	var report struct {
		Checks []CheckResult `json:"checks"`
	}
	_ = json.Unmarshal(rep.Artifacts[2].Data, &report)
	for _, x := range report.Checks {
		if wantNet := map[string]bool{"cli": false, "ui": true, "api": true}[x.Kind]; (x.Network == NetAllowed) != wantNet {
			t.Errorf("%s (%s) ran with network %q", x.Ref, x.Kind, x.Network)
		}
	}

	// Touching export runs its failing check: the floor fails.
	r = behaviourRun(t, root, "app/export.go")
	r.Story, r.Criteria = 0, nil
	rep = Execute(ctx, r, []Floor{Behaviour{}})
	if c := rep.Checks[0]; c.Result != proof.Fail || c.Value != 1 || rep.Hold || !strings.Contains(c.Detail, "failed: export.fail (step 1: exit 2, want 0)") {
		t.Fatalf("failing check: %+v", c)
	}
	if log := calls(t, root); len(log) != 1 || log[0] != "fail" {
		t.Fatalf("calls %q, want only export.fail", log)
	}

	// Nothing touched and no story: the floor is skipped.
	r = behaviourRun(t, root, "README.md")
	r.Story = 0
	if ok, why := (Behaviour{}).Applies(r); ok || !strings.Contains(why, "no behaviour check for this change") {
		t.Fatalf("untouched: applies %v (%s)", ok, why)
	}
	// --check selects by feature or check, whatever the change touched.
	r.Checks = "export"
	if rep := Execute(ctx, r, []Floor{Behaviour{}}); rep.Checks[0].Result != proof.Fail {
		t.Fatalf("--check export: %+v", rep.Checks[0])
	}
	r.Checks = "tasks.nope"
	if rep := Execute(ctx, r, []Floor{Behaviour{}}); rep.Checks[0].Result != proof.Blocked || !strings.Contains(rep.Checks[0].Detail, `no behaviour check "tasks.nope"`) {
		t.Fatalf("--check tasks.nope: %+v", rep.Checks[0])
	}
	_ = calls(t, root)
}

// A control tool that names an artifact outside its output directory is an
// error (blocked), not a pass.
func TestBehaviourArtifactEscape(t *testing.T) {
	root, _ := behaviourRepo(t)
	write(t, root, map[string]string{".agents/skills/verify-tasks/features/export.md": feature("export", "app/export.go", check("escape", "cli", "api-trace"))})
	r := behaviourRun(t, root, "app/export.go")
	rep := Execute(context.Background(), r, []Floor{Behaviour{}})
	if c := rep.Checks[0]; c.Result != proof.Blocked || !strings.Contains(c.Detail, "outside the output directory") {
		t.Fatalf("escape: %+v", c)
	}
}

// A ui or api check never runs against production: the floor refuses before
// the seed or the control tool runs.
func TestBehaviourRefusesProduction(t *testing.T) {
	root, _ := behaviourRepo(t)
	r := behaviourRun(t, root, "app/main.go")
	r.Verify.Environments["local"] = config.Environment{URL: "https://app.example.com", Production: true}
	marker := filepath.Join(t.TempDir(), "seeded")
	r.Verify.Seed = []string{"touch", marker}
	rep := Execute(context.Background(), r, []Floor{Behaviour{}})
	if c := rep.Checks[0]; c.Result != proof.Blocked || !strings.Contains(c.Detail, "production environment") {
		t.Fatalf("production: %+v", c)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("the seed ran against production")
	}
	if log := calls(t, root); len(log) != 0 {
		t.Fatalf("the control tool ran: %q", log)
	}
}

// fakePlaywright is a playwright package whose page's accessibility tree is
// its install's name, so aria.txt says which Playwright control.mjs loaded.
func fakePlaywright(name string) map[string]string {
	return map[string]string{
		"playwright/package.json": `{"name": "playwright", "main": "index.js"}`,
		"playwright/index.js": `const page = {
  goto: async () => ({ status: () => 200 }),
  url: () => "http://127.0.0.1:9999/",
  locator: () => ({ ariaSnapshot: async () => "` + name + `" }),
  screenshot: async () => {},
  video: () => null,
};
exports.chromium = { launch: async () => ({ newContext: async () => ({ newPage: async () => page, close: async () => {} }), close: async () => {} }) };
`,
	}
}

// Bug #1226: the Node control tool (hosts/verify-skill/node/control.mjs) finds
// the Playwright that pnpm keeps only in its store, NODE_PATH's Playwright
// comes first as before, and with none installed each browser check is
// unreachable with the command to install it, which fails nothing.
func TestControlFindsPlaywright(t *testing.T) {
	control, err := os.ReadFile(filepath.Join("..", "..", "hosts", "verify-skill", "node", "control.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	dir := ".agents/skills/verify-tasks/"
	ui := "```check\n{\"id\": \"home\", \"kind\": \"ui\", \"evidence\": \"screenshot\", \"steps\": [{\"goto\": \"/\"}]}\n```\n\n"
	root, _, _ := repo(t, map[string]string{"app/main.go": "package main\n"}, map[string]string{
		dir + "control.mjs":        string(control),
		dir + "features/README.md": "# map\n\n```map\n{\"product\": \"tasks\", \"control\": [\"node\", \"" + dir + "control.mjs\"]}\n```\n\n1. [Tasks](tasks.md)\n",
		dir + "features/tasks.md":  feature("tasks", "app/main.go", ui),
	})
	store := filepath.Join(root, "node_modules", ".pnpm", "playwright@1.64.0", "node_modules")
	nodePath := t.TempDir()
	for _, tc := range []struct {
		name, nodePath, status, reason, aria string
		store, inNodePath                    bool
		broken                               map[string]string // files over the store's Playwright that break it
	}{
		{name: "only in pnpm's store, no NODE_PATH", store: true, status: CheckPassed, aria: "store"},
		{name: "NODE_PATH and pnpm's store", store: true, inNodePath: true, nodePath: nodePath, status: CheckPassed, aria: "node-path"},
		{name: "none installed", nodePath: t.TempDir(), status: CheckUnreachable,
			reason: `Playwright is not installed: run "npm install --save-dev playwright && npx playwright install chromium"`},
		{name: "installed but fails to load", store: true, status: checkError, reason: "no result on stdout",
			broken: map[string]string{"playwright/index.js": "throw new Error(\"playwright: broken install\");\n"}},
		{name: "installed with an invalid package.json", store: true, status: checkError, reason: "no result on stdout",
			broken: map[string]string{"playwright/package.json": "{\"name\": \"playwright\",\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.RemoveAll(filepath.Join(root, "node_modules"))
			_ = os.RemoveAll(filepath.Join(nodePath, "playwright"))
			if tc.store {
				write(t, store, fakePlaywright("store"))
			}
			if tc.inNodePath {
				write(t, nodePath, fakePlaywright("node-path"))
			}
			if tc.broken != nil { // a Playwright that is there but broken is an error, never "not installed"
				write(t, store, tc.broken)
			}
			r := behaviourRun(t, root)
			r.Story, r.Criteria, r.Checks = 0, nil, SelectAll
			r.Sandbox.Getenv = func(k string) string {
				return map[string]string{"PATH": os.Getenv("PATH"), "NODE_PATH": tc.nodePath}[k]
			}
			rep := Execute(context.Background(), r, []Floor{Behaviour{}})
			var got struct {
				Checks []CheckResult `json:"checks"`
			}
			aria := ""
			for _, a := range rep.Artifacts {
				switch a.Path {
				case "reports/behaviour.json":
					_ = json.Unmarshal(a.Data, &got)
				case "behaviour/tasks.home/aria.txt":
					aria = string(a.Data)
				}
			}
			if len(got.Checks) != 1 || got.Checks[0].Status != tc.status || !strings.HasPrefix(got.Checks[0].Reason, tc.reason) || aria != tc.aria {
				t.Fatalf("checks %+v, aria %q; want %s (%q), aria %q: %+v", got.Checks, aria, tc.status, tc.reason, tc.aria, rep.Checks[0])
			}
			if c := rep.Checks[0]; tc.broken == nil && c.Result != proof.Pass {
				t.Fatalf("floor %+v, want passed: an unreachable check fails nothing", c)
			}
		})
	}
}
