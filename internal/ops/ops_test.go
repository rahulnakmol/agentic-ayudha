package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const restart = `
[[action]]
name = "restart"
argv_template = ["scripts/restart.sh", "--service={service}", "{replicas}"]
approval = "human"
environments = ["production"]
max_per_hour = 2
rollback = "none"
blast_radius = "service"
[action.params.service]
type = "enum"
values = ["api", "worker"]
[action.params.replicas]
type = "int"
min = 1
max = 5
[action.params.note]
type = "string"
pattern = "^[a-z ]{1,40}$"
`

func TestValidate(t *testing.T) {
	base := "version = 1\n" + restart
	tests := []struct {
		name, toml, want string
	}{
		{"valid", base, ""},
		{"inline shell", strings.Replace(base, `["scripts/restart.sh", "--service={service}", "{replicas}"]`, `["bash", "-c", "kubectl rollout restart {service}"]`, 1), "inline shell"},
		{"program is a parameter", strings.Replace(base, `"scripts/restart.sh"`, `"{service}"`, 1), "cannot be a parameter"},
		{"undeclared placeholder", strings.Replace(base, `"{replicas}"`, `"{region}"`, 1), "{region}, which is not a declared parameter"},
		{"unanchored string pattern", strings.Replace(base, `"^[a-z ]{1,40}$"`, `"[a-z]+"`, 1), "anchored pattern"},
		{"global is never auto", strings.Replace(strings.Replace(base, `"service"`+"\n", `"global"`+"\n", 1), `approval = "human"`, `approval = "auto"`, 1), "never auto"},
		{"rollback names nothing", strings.Replace(base, `rollback = "none"`, `rollback = "undo"`, 1), `rollback = "undo"`},
		{"reserved parameter", base + "[action.params.environment]\ntype = \"bool\"\n", "reserved"},
		{"unknown key", strings.Replace(base, `max_per_hour = 2`, "max_per_hour = 2\nshell = true", 1), "shell"},
		{"no environment", strings.Replace(base, `environments = ["production"]`, `environments = []`, 1), "at least one GitHub Environment"},
		{"environment name", strings.Replace(base, `["production"]`, `["prod env"]`, 1), "not a GitHub Environment name"},
		{"no hourly cap", strings.Replace(base, `max_per_hour = 2`, `max_per_hour = 0`, 1), "max_per_hour"},
		{"rollback to itself", strings.Replace(base, `rollback = "none"`, `rollback = "restart"`, 1), "cannot be the action itself"},
		{"approval", strings.Replace(base, `approval = "human"`, `approval = "maybe"`, 1), "want auto or human"},
		{"blast radius", strings.Replace(base, `blast_radius = "service"`, `blast_radius = "planet"`, 1), "blast_radius"},
		{"enum without values", strings.Replace(base, `values = ["api", "worker"]`, `values = []`, 1), "needs values"},
		{"int bounds", strings.Replace(base, "min = 1\nmax = 5", "min = 6\nmax = 5", 1), "min > max"},
		{"parameter type", strings.Replace(base, `type = "int"`, `type = "float"`, 1), "want string, int, enum, or bool"},
		{"parameter name", base + "[action.params.Bad]\ntype = \"bool\"\n", "want lowercase"},
		{"bad pattern", strings.Replace(base, `"^[a-z ]{1,40}$"`, `"^[a-z$"`, 1), "pattern"},
		{"version", strings.Replace(base, "version = 1", "version = 2", 1), "version = 2"},
		{"duplicate action", base + restart, "defined twice"},
		{"bad name", strings.Replace(base, `name = "restart"`, `name = "Restart!"`, 1), "name must match"},
		{"not TOML", "version = ", "ops/actions.toml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.toml))
			switch {
			case tt.want == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Fatalf("error %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	c, err := Parse([]byte("version = 1\n" + restart))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.Find("restart")
	argv, err := a.Render(map[string]string{"service": "api", "replicas": "3"})
	if err != nil || !slices.Equal(argv, []string{"scripts/restart.sh", "--service=api", "3"}) {
		t.Fatalf("render: %v %v", argv, err)
	}
	if _, err := ParseParams("service=api,service=worker"); !errors.Is(err, ErrParams) {
		t.Errorf("duplicate parameter: %v", err)
	}
	if _, err := ParseParams("service"); !errors.Is(err, ErrParams) {
		t.Errorf("parameter without a value: %v", err)
	}
	if p, err := ParseParams(" "); err != nil || len(p) != 0 {
		t.Errorf("no parameters: %v %v", p, err)
	}
	multi := a
	multi.Environments = []string{"staging", "production"}
	if _, err := multi.Environment(map[string]string{}); !errors.Is(err, ErrParams) {
		t.Errorf("two environments, none picked: %v", err)
	}
	if e, err := multi.Environment(map[string]string{"environment": "staging"}); err != nil || e != "staging" {
		t.Errorf("picked environment: %s %v", e, err)
	}
	if _, err := multi.Environment(map[string]string{"environment": "dev"}); !errors.Is(err, ErrParams) {
		t.Errorf("environment the action does not list: %v", err)
	}
	for name, params := range map[string]map[string]string{
		"enum outside values":            {"service": "api; rm -rf /", "replicas": "3"},
		"int out of range":               {"service": "api", "replicas": "9"},
		"int with a sign trick":          {"service": "api", "replicas": "+3"},
		"placeholder parameter missing":  {"service": "api"},
		"undeclared parameter":           {"service": "api", "replicas": "3", "cmd": "x"},
		"string starting with a dash":    {"service": "api", "replicas": "3", "note": "-rf"},
		"string not matching its schema": {"service": "api", "replicas": "3", "note": "Robert'); DROP"},
		"int that is not a number":       {"service": "api", "replicas": "three"},
	} {
		if _, err := a.Render(params); !errors.Is(err, ErrParams) {
			t.Errorf("%s: %v, want ErrParams", name, err)
		}
	}
}

// fakeSource is a default branch whose catalog history is versions, newest first.
type fakeSource struct {
	files   map[string]string // sha -> catalog
	history []string
	onDef   map[string]bool
}

func (f fakeSource) File(_ context.Context, ref, _ string) ([]byte, bool, error) {
	s, ok := f.files[ref]
	return []byte(s), ok, nil
}
func (f fakeSource) OnDefault(_ context.Context, sha string) (bool, error) { return f.onDef[sha], nil }
func (f fakeSource) History(_ context.Context, sha, _ string, max int) ([]string, error) {
	i := slices.Index(f.history, sha)
	if i < 0 {
		return nil, nil
	}
	return f.history[i:min(len(f.history), i+max)], nil
}

func action(name, approval, argv string) string {
	return "[[action]]\nname = \"" + name + "\"\nargv_template = [\"" + argv + "\"]\napproval = \"" + approval + "\"\nenvironments = [\"staging\"]\nmax_per_hour = 1\nrollback = \"none\"\nblast_radius = \"single\"\n"
}

// An action can be auto only once its definition merged as human first.
func TestAutoNeverOnFirstMerge(t *testing.T) {
	v := func(actions ...string) string { return "version = 1\n" + strings.Join(actions, "") }
	src := fakeSource{
		files: map[string]string{
			"c3": v(action("flush", "auto", "flush.sh"), action("fresh", "auto", "fresh.sh"), action("changed", "auto", "changed-v2.sh")),
			"c2": v(action("flush", "human", "flush.sh"), action("changed", "human", "changed-v1.sh")),
			"c1": v(action("flush", "human", "flush.sh")),
			"pr": v(action("flush", "auto", "flush.sh"), action("sneaky", "auto", "sneaky.sh")),
		},
		history: []string{"c3", "c2", "c1"},
		onDef:   map[string]bool{"c3": true, "c2": true, "c1": true},
	}
	c, err := Load(context.Background(), src, "c3")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"flush": Auto, "fresh": Human, "changed": Human} {
		if a, _ := c.Find(name); a.Effective != want {
			t.Errorf("%s: effective %s (%s), want %s", name, a.Effective, a.Note, want)
		}
	}
	// A pull request's catalog is never read: its commit is not on the default branch.
	if _, err := Load(context.Background(), src, "pr"); !errors.Is(err, ErrNotPinned) {
		t.Errorf("PR catalog: %v, want ErrNotPinned", err)
	}
}

// The repository's own catalog parses.
func TestRepositoryCatalog(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", Path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(b); err != nil {
		t.Fatal(err)
	}
}
