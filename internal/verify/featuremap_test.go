package verify

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

const tasksApp = `package main

func routes(mux *Mux) {
	mux.HandleFunc("GET /{$}", page)
	mux.HandleFunc("POST /api/add", add)
}

const form = "<form><label for=t>Task</label><input id=t><button>Add task</button></form>"

func commands() { cmd("list"); cmd("add") }
`

const fixtureReadme = "# tasks feature map\n\n```map\n" + `{"product": "tasks", "control": ["node", "control.mjs"],
 "surfaces": [{"kind": "route", "files": ["app/**/*.go"], "exclude": ["**/*_test.go"], "pattern": "HandleFunc\\(\"([^\"]+)\""},
              {"kind": "command", "files": ["app/*.go"], "pattern": "cmd\\(\"([a-z-]+)\"\\)"}]}` + "\n```\n\n## Features\n\n1. [Tasks](tasks.md): add and list tasks.\n"

func fixtureFeature(verifiedAt string) string {
	return "# Tasks\n\nPeople add tasks and list them.\n\n```feature\n" + `{"id": "tasks", "layers": ["L01"], "sources": ["app/main.go"],
 "surfaces": {"route": ["GET /{$}", "POST /api/add"], "command": ["list", "add"]}, "verified_at": "` + verifiedAt + `"}` +
		"\n```\n\n## Sub-features\n\n- add\n\n## How to reach it\n\n- Open `/`.\n\n## Behaviour checks\n\n```check\n" +
		`{"id": "add", "kind": "ui", "evidence": "screenshot", "steps": [{"goto": "/"}, {"fill": {"label": "Task", "value": "milk"}}, {"click": {"role": "button", "name": "Add task"}}, {"api": {"path": "/api/add", "json": {"name": "Ada"}}}]}` +
		"\n```\n\n```check\n" + `{"id": "list", "kind": "cli", "evidence": "api-trace", "steps": [{"run": ["list"], "exit": 0}]}` +
		"\n```\n\n## Gotchas\n\n- None.\n"
}

// featureRepo writes the app, then the map verified at the app's commit, and
// returns the root and that commit.
func featureRepo(t *testing.T) (string, string) {
	t.Helper()
	root, _, app := repo(t, nil, map[string]string{"app/main.go": tasksApp, "app/main_test.go": `x := HandleFunc("GET /only-in-a-test")`})
	dir := ".agents/skills/verify-tasks/features/"
	write(t, root, map[string]string{dir + "README.md": fixtureReadme, dir + "tasks.md": fixtureFeature(app)})
	git(t, root, "add", "-A")
	git(t, root, "commit", "--quiet", "-m", "map")
	return root, app
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, s := range files {
		f := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestMapDrift: a map in sync is clean (a request body's "name" is data, not a
// handle); then a renamed button, a new route, a
// removed command, a dead index entry, a feature file outside the contract and
// the index, an unknown check in an evidence map, and an edited source are each
// reported, by kind, in that one run.
func TestMapDrift(t *testing.T) {
	ctx := context.Background()
	root, _ := featureRepo(t)
	d, maps, err := CheckMaps(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != DriftClean || len(d.Items()) != 0 {
		t.Fatalf("clean map: verdict %s, items %q", d.Verdict, d.Items())
	}
	if len(maps) != 1 || maps[0].Product != "tasks" || len(maps[0].Features) != 1 || len(maps[0].Features[0].Checks) != 2 {
		t.Fatalf("maps: %+v", maps)
	}

	dir := ".agents/skills/verify-tasks/features/"
	app := strings.NewReplacer("<button>Add task</button>", "<button>Create task</button>",
		`mux.HandleFunc("POST /api/add", add)`, "mux.HandleFunc(\"POST /api/add\", add)\n\tmux.HandleFunc(\"GET /api/export\", export)",
		`cmd("add")`, `cmd("new")`).Replace(tasksApp)
	write(t, root, map[string]string{
		"app/main.go":              app,
		dir + "README.md":          fixtureReadme + "2. [Reports](reports.md): weekly reports.\n",
		dir + "notes.md":           "# Notes\n\n```feature\n{\"id\": \"notes\", \"sources\": [\"app/notes.go\"], \"verified_at\": \"\"}\n```\n\n## Checks\n",
		".aayudha/evidence/7.toml": "[[criterion]]\nid = 1\nchecks = [\"tasks.add\", \"tasks.nope\"]\n",
	})
	git(t, root, "add", "-A")
	git(t, root, "commit", "--quiet", "-m", "drift")

	d, _, err = CheckMaps(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	p := ".agents/skills/verify-tasks: "
	want := []string{
		p + `contract: .agents/skills/verify-tasks/features/notes.md: the H2s are ["Checks"]; want ["Sub-features" "How to reach it" "Behaviour checks" "Gotchas"] in this order`,
		p + `contract: .agents/skills/verify-tasks/features/notes.md: feature "notes" has no behaviour check`,
		p + "not in the index: .agents/skills/verify-tasks/features/notes.md",
		p + "index entry without a feature file: reports",
		p + `unmapped command "new" (app/main.go)`,
		p + `unmapped route "GET /api/export" (app/main.go)`,
		p + `command "add" of feature tasks is no longer in the source`,
		p + "evidence map names an unknown check: .aayudha/evidence/7.toml: tasks.nope",
		p + "feature tasks: handle not in its sources: add: Add task",
		p + "feature tasks: stale, to re-check: app/main.go",
		p + "feature notes: source app/notes.go matches no file",
		p + "feature notes: stale, to re-check: never verified (no verified_at)",
	}
	if d.Verdict != DriftFound || !slices.Equal(d.Items(), want) {
		t.Fatalf("verdict %s, items:\n%s\nwant:\n%s", d.Verdict, strings.Join(d.Items(), "\n"), strings.Join(want, "\n"))
	}
}

// TestMapStaleOnly: an edited source with every handle and surface intact is
// stale, not drift.
func TestMapStaleOnly(t *testing.T) {
	root, _ := featureRepo(t)
	write(t, root, map[string]string{"app/main.go": tasksApp + "\n// a comment\n"})
	d, _, err := CheckMaps(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if d.Verdict != DriftStale || len(d.Items()) != 1 || !strings.HasSuffix(d.Items()[0], "feature tasks: stale, to re-check: app/main.go") {
		t.Fatalf("verdict %s, items %q", d.Verdict, d.Items())
	}
}

// TestMapFloor: the feature-map floor never blocks; it reports drift and keeps
// the report, and it is skipped where there is no map.
func TestMapFloor(t *testing.T) {
	ctx := context.Background()
	none, _, _ := repo(t, nil, map[string]string{"main.go": "package main\n"})
	if ok, why := (MapFloor{}).Applies(&Run{Root: none}); ok || !strings.Contains(why, "no feature map") {
		t.Fatalf("no map: applies %v (%s)", ok, why)
	}
	root, _ := featureRepo(t)
	write(t, root, map[string]string{"app/main.go": strings.Replace(tasksApp, "Add task", "Create task", 1)})
	r := &Run{Root: root}
	if ok, _ := (MapFloor{}).Applies(r); !ok {
		t.Fatal("a map: the floor does not apply")
	}
	rep := Execute(ctx, r, []Floor{MapFloor{}})
	c := rep.Checks[0]
	if !rep.Hold || c.Result != proof.Pass || c.Value != 1 || !strings.Contains(c.Detail, "advisory: 1 drift item") || !strings.Contains(c.Detail, "Add task") {
		t.Fatalf("check %+v hold %v", c, rep.Hold)
	}
	if len(rep.Artifacts) != 1 || rep.Artifacts[0].Path != "reports/feature-map.json" || !strings.Contains(string(rep.Artifacts[0].Data), `"verdict": "drift"`) {
		t.Fatalf("artifacts %+v", rep.Artifacts)
	}
}
