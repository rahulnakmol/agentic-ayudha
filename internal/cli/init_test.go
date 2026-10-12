package cli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/nacl/box"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite internal/cli/testdata/gates")

const ciYML = "name: ci\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make test # the product's own CI\n"

// checkout makes a clone-like working tree of o/r with the product's own CI.
func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "--quiet", "-b", "dev"}, {"remote", "add", "origin", "https://github.com/o/r.git"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte(ciYML), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type initReport struct {
	Complete bool              `json:"complete"`
	Changes  []initrepo.Change `json:"changes"`
	Counts   map[string]int    `json:"counts"`
	GitHub   struct {
		REST int `json:"rest"`
	} `json:"github"`
}

func runInitCmd(t *testing.T, g *fake.GitHub, dir string, args ...string) (int, initReport, string) {
	t.Helper()
	code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}, append(args, "--json")...)
	var rep initReport
	if out != "" {
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&rep); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	return code, rep, errOut
}

// snapshot reads every file of the checkout outside .git.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "telemetry") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(dir, p)
			out[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	return out
}

func treeFiles(g *fake.GitHub, ref string) []string {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	c := g.Commits[g.Refs[ref]]
	var names []string
	for _, e := range g.Trees[c["tree"].(string)] {
		names = append(names, e["path"].(string))
	}
	return names
}

// WP2.5 accept: run twice → zero changes; --dry-run writes nothing; a public
// repository ends with no .lfsconfig on ledger and proof push --media exits 4;
// the product's existing ci.yml stays byte-identical.
func TestInitTwiceDryRunPublicCoexistence(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Labels = []*fake.Label{{ID: 7, Name: "bug", Color: "d73a4a"}, {ID: 8, Name: "status:ready", Color: "ffffff"}}
	dir := checkout(t)

	before := snapshot(t, dir)
	code, rep, errOut := runInitCmd(t, g, dir, "init", "--dry-run")
	if code != 0 {
		t.Fatalf("dry run: exit %d: %s", code, errOut)
	}
	if w := g.Writes(); len(w) != 0 || rep.Counts["planned"] == 0 {
		t.Fatalf("dry run sent %d writes, planned %d", len(w), rep.Counts["planned"])
	}
	if after := snapshot(t, dir); len(after) != len(before) {
		t.Fatalf("dry run wrote files: %d -> %d", len(before), len(after))
	}

	code, rep, errOut = runInitCmd(t, g, dir, "init", "--yes")
	if code != 0 || !rep.Complete {
		t.Fatalf("first run: exit %d complete %t: %s", code, rep.Complete, errOut)
	}
	nLabels := len(workitems.Labels())
	if rep.Counts["created"] != nLabels-1+2 || rep.Counts["adopted"] != 1 { // status:ready adopted; ledger and ruleset created
		t.Fatalf("first run counts %v, want %d labels created, 1 adopted, plus ledger and ruleset", rep.Counts, nLabels-1)
	}
	first := snapshot(t, dir)
	if first[".github/workflows/ci.yml"] != ciYML {
		t.Fatalf("ci.yml changed:\n%s", first[".github/workflows/ci.yml"])
	}
	for _, typ := range workitems.Types {
		if first[initrepo.FormPath(typ)] != string(workitems.Form(typ, workitems.FormOptions{})) {
			t.Errorf("form %s missing or wrong", typ.ID)
		}
	}
	if !strings.Contains(first[".aayudha/config.toml"], `media = "" # required: private bucket`) {
		t.Errorf("public repository config:\n%s", first[".aayudha/config.toml"])
	}
	if !strings.Contains(first["AGENTS.md"], "<!-- BEGIN aayudha:work-items -->") {
		t.Errorf("AGENTS.md:\n%s", first["AGENTS.md"])
	}
	if files := treeFiles(g, "heads/ledger"); slices.Contains(files, ".lfsconfig") || slices.Contains(files, ".gitattributes") {
		t.Errorf("public ledger has LFS config: %v", files)
	}

	g.Reset()
	code, rep, errOut = runInitCmd(t, g, dir, "init", "--yes")
	if code != 0 || len(g.Writes()) != 0 || rep.Counts["created"]+rep.Counts["written"]+rep.Counts["planned"] != 0 {
		t.Fatalf("second run: exit %d, %d writes, counts %v: %s", code, len(g.Writes()), rep.Counts, errOut)
	}
	if second := snapshot(t, dir); !mapsEqual(first, second) {
		t.Fatal("second run changed files")
	}

	if code, _, errOut := run(t, nil, Options{Dir: dir}, "proof", "push", "--media"); code != ExitBlocked || !strings.Contains(errOut, "V3") {
		t.Errorf("proof push --media on a public repo: exit %d %s", code, errOut)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// WP2.5 accept: a partial run is resumed from state without duplicates.
func TestInitResume(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Private = true
	g.Fail, g.FailAfter = "POST /repos/o/r/labels", 5
	dir := checkout(t)
	if code, _, _ := runInitCmd(t, g, dir, "init", "--yes"); code == 0 {
		t.Fatal("injected failure did not stop init")
	}
	st, err := initrepo.LoadState(dir)
	if err != nil || st == nil || st.Complete || len(st.Objects) != 5 || !st.IsDone("identity") || st.IsDone("labels") {
		t.Fatalf("state after the failure: %+v, %v", st, err)
	}
	if code, _, errOut := runInitCmd(t, g, dir, "init", "--yes"); code != ExitUsage || !strings.Contains(errOut, "--resume") {
		t.Fatalf("rerun without --resume: exit %d %s", code, errOut)
	}
	code, rep, errOut := runInitCmd(t, g, dir, "init", "--yes", "--resume")
	if code != 0 || !rep.Complete {
		t.Fatalf("resume: exit %d: %s", code, errOut)
	}
	creates := 0
	for _, w := range g.Writes() {
		if w.Method == "POST" && w.Path == "/repos/o/r/labels" {
			creates++
		}
	}
	if len(g.Labels) != len(workitems.Labels()) || creates != len(workitems.Labels())+1 { // +1: the failed attempt
		t.Fatalf("%d labels, %d create requests; want %d labels and no duplicate creates", len(g.Labels), creates, len(workitems.Labels()))
	}
	// Private repository: the ledger tracks media in LFS.
	if files := treeFiles(g, "heads/ledger"); !slices.Contains(files, ".lfsconfig") || !slices.Contains(files, ".gitattributes") {
		t.Errorf("private ledger files: %v", files)
	}
}

// The ledger branch and a ruleset protecting it already exist (aayudha's own
// repository): init adopts both and adds nothing alongside; uninit removes
// only what init created.
func TestInitAdoptsLedgerAndUninit(t *testing.T) {
	g := fake.New(t, "o/r", "Organization")
	g.Refs["heads/ledger"] = "abc123"
	g.Rulesets = []map[string]any{{"id": 99, "name": "ledger-writers", "target": "branch", "enforcement": "active",
		"conditions": map[string]any{"ref_name": map[string]any{"include": []any{"refs/heads/ledger"}}},
		"rules":      []any{map[string]any{"type": "deletion"}, map[string]any{"type": "non_fast_forward"}, map[string]any{"type": "update"}}}}
	g.IssueTypes = []*fake.IssueType{{ID: 3, Name: "Bug"}}
	dir := checkout(t)
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Product\n\nOur rules.\n"), 0o644)
	code, rep, errOut := runInitCmd(t, g, dir, "init", "--yes")
	if code != 0 {
		t.Fatalf("init: exit %d: %s", code, errOut)
	}
	for _, w := range g.Writes() {
		if strings.Contains(w.Path, "/git/") || strings.Contains(w.Path, "/rulesets") {
			t.Errorf("init wrote %s %s; the ledger and its ruleset already exist", w.Method, w.Path)
		}
	}
	if len(g.IssueTypes) != 10 || !strings.Contains(snapshot(t, dir)[initrepo.FormPath(workitems.Types[3])], "\ntype: \"Story\"\n") {
		t.Errorf("organization: %d issue types; forms must set the type", len(g.IssueTypes))
	}
	_ = rep
	if code, out, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "x"}, Options{GitHubAPI: g.URL(), Dir: dir}, "uninit", "--external"); code != 0 || !strings.Contains(out, "no external resources") {
		t.Errorf("uninit --external: %d %s", code, out)
	}
	if code, _, errOut := runInitCmd(t, g, dir, "uninit", "--yes"); code != 0 {
		t.Fatalf("uninit: exit %d: %s", code, errOut)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if len(g.IssueTypes) != 1 || g.Refs["heads/ledger"] != "abc123" || len(g.Rulesets) != 1 || len(g.Labels) != 0 {
		t.Errorf("after uninit: %d types, ledger %q, %d rulesets, %d labels", len(g.IssueTypes), g.Refs["heads/ledger"], len(g.Rulesets), len(g.Labels))
	}
	after := snapshot(t, dir)
	if after["AGENTS.md"] != "# Product\n\nOur rules.\n" || after[".github/workflows/ci.yml"] != ciYML || len(after) != 2 {
		t.Errorf("files after uninit: %v", slices.Sorted(func(y func(string) bool) {
			for k := range after {
				if !y(k) {
					return
				}
			}
		}))
	}
}

// WP2.6 accept: --gates installs exactly the workflows whose WP the binary has
// (golden): two on the WP2.6 binary, three since WP2.9 added triage, four
// since WP4.8 added the merge gate, five with the WP5.7 streak, six with the
// WP5.6 ops executor, and the WP7.1 loops workflow when the checkout has loops;
// every extra is idempotent; uninit --external lists what init created outside
// the repository.
func TestInitExtras(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	pub, _, _ := box.GenerateKey(rand.Reader)
	g.PublicKey = base64.StdEncoding.EncodeToString(pub[:])
	g.App = map[string]any{"id": 4242, "slug": "aayudha-r", "client_id": "Iv1.abc", "pem": "-----BEGIN RSA PRIVATE KEY-----\nMIIEsecret\n-----END RSA PRIVATE KEY-----\n"}
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 9 * * 1-5\"\n"), 0o644)
	var manifest string
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Build: BuildInfo{Version: "0.4.0"},
		AppManifestCode: func(_ context.Context, post string, m func(string) string) (string, error) {
			manifest = m("http://127.0.0.1:1/callback")
			return "code-1", nil
		}}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	args := []string{"init", "--yes", "--json", "--gates", "--projects", "--identity", "--integrations"}
	code, out, errOut := run(t, env, opts, args...)
	if code != 0 {
		t.Fatalf("init with extras: exit %d: %s\n%s", code, errOut, out)
	}
	files := snapshot(t, dir)
	var workflows []string
	for p := range files {
		if strings.HasPrefix(p, ".github/workflows/") {
			workflows = append(workflows, strings.TrimPrefix(p, ".github/workflows/"))
		}
	}
	slices.Sort(workflows)
	if !slices.Equal(workflows, []string{"aayudha-loops.yml", "aayudha-merge-gate.yml", "aayudha-ops.yml", "aayudha-ready-guard.yml", "aayudha-streak.yml", "aayudha-token.yml", "aayudha-triage.yml", "ci.yml"}) || files[".github/workflows/ci.yml"] != ciYML {
		t.Fatalf("workflows after --gates: %v", workflows)
	}
	for _, wf := range []string{"aayudha-merge-gate.yml", "aayudha-ops.yml", "aayudha-ready-guard.yml", "aayudha-streak.yml", "aayudha-token.yml", "aayudha-triage.yml"} {
		golden := filepath.Join("testdata", "gates", wf)
		if *updateGolden {
			_ = os.MkdirAll(filepath.Dir(golden), 0o755)
			_ = os.WriteFile(golden, []byte(files[".github/workflows/"+wf]), 0o644)
		}
		want, err := os.ReadFile(golden)
		if err != nil || string(want) != files[".github/workflows/"+wf] {
			t.Errorf("%s differs from %s (go test ./internal/cli -run TestInitExtras -update-golden)", wf, golden)
		}
	}
	// aayudha runs its own streak workflow: the template as a development build installs it.
	if own, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "aayudha-streak.yml")); err != nil || string(own) != string(initrepo.GateWorkflows("dev")[initrepo.StreakWorkflow]) {
		t.Errorf(".github/workflows/aayudha-streak.yml is not the WP5.7 template rendered for dev")
	}
	if !strings.Contains(files[".github/workflows/aayudha-loops.yml"], "aayudha@v0.4.0") || files["loops/amp/digest.json"] == "" || !strings.Contains(files["loops/cursor/aayudha-loops.tf"], `"5 8 * * 1-5"`) {
		t.Errorf("loops rendered by --gates: %v", slices.Collect(maps.Keys(files)))
	}
	cfg := files[".aayudha/config.toml"]
	for _, want := range []string{"[identity]\napp_id = 4242\napp_slug = \"aayudha-r\"\nbroker = \"aayudha-token.yml\"", "[integrations.posthog]", "[integrations.sentry]", "[integrations.growthbook]",
		"[integrations.azure]\nscope = \"read\"\nauth = \"oidc\"", "[integrations.aws]\nscope = \"read\"\nauth = \"oidc\"", "[integrations.gcp]\nscope = \"read\"\nauth = \"oidc\""} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config lacks %q:\n%s", want, cfg)
		}
	}
	// WP7.2b.3: each cloud's least-privilege setup is written under .aayudha/setup/<cloud>/
	// (golden), its steps are printed, and the config it writes loads (no stored key).
	var setup []string
	for p := range files {
		if strings.HasPrefix(p, ".aayudha/setup/") {
			setup = append(setup, p)
			golden := filepath.Join("testdata", "setup", strings.TrimPrefix(p, ".aayudha/setup/"))
			if *updateGolden {
				_ = os.MkdirAll(filepath.Dir(golden), 0o755)
				_ = os.WriteFile(golden, []byte(files[p]), 0o644)
			}
			if want, err := os.ReadFile(golden); err != nil || string(want) != files[p] {
				t.Errorf("%s differs from %s (go test ./internal/cli -run TestInitExtras -update-golden)", p, golden)
			}
			if strings.HasSuffix(p, ".json") && !json.Valid([]byte(strings.NewReplacer("<", "", ">", "").Replace(files[p]))) {
				t.Errorf("%s is not JSON", p)
			}
		}
	}
	slices.Sort(setup)
	if want := []string{".aayudha/setup/aws/README.md", ".aayudha/setup/aws/media-policy.json", ".aayudha/setup/aws/read-policy.json", ".aayudha/setup/aws/trust-policy.json", ".aayudha/setup/aws/workflow.yml",
		".aayudha/setup/azure/README.md", ".aayudha/setup/azure/federated-credential.json", ".aayudha/setup/azure/lifecycle-policy.json", ".aayudha/setup/azure/workflow.yml",
		".aayudha/setup/gcp/README.md", ".aayudha/setup/gcp/workflow.yml"}; !slices.Equal(setup, want) {
		t.Errorf("setup files %v, want %v", setup, want)
	}
	for _, want := range []string{"# Azure for o/r (aayudha)", "# AWS for o/r (aayudha)", "# Google Cloud for o/r (aayudha)", `"repo:o/r:ref:refs/heads/`} {
		if !strings.Contains(errOut+files[".aayudha/setup/azure/federated-credential.json"], want) {
			t.Errorf("init did not print or write %q", want)
		}
	}
	if code, _, errOut := run(t, env, opts, "config", "validate"); code != 0 {
		t.Errorf("config with the cloud sections does not validate: %s", errOut)
	}
	g.Mu.Lock()
	if len(g.Projects) != 3 || g.Variables["AAYUDHA_APP_CLIENT_ID"] != "Iv1.abc" || g.Secrets["AAYUDHA_APP_PRIVATE_KEY"] == "" || strings.Contains(g.Secrets["AAYUDHA_APP_PRIVATE_KEY"], "MIIE") {
		t.Errorf("projects %d, variables %v, secret sealed %t", len(g.Projects), g.Variables, !strings.Contains(g.Secrets["AAYUDHA_APP_PRIVATE_KEY"], "MIIE"))
	}
	bypass := fmt.Sprint(g.Rulesets[0]["bypass_actors"])
	g.Mu.Unlock()
	if !strings.Contains(bypass, "Integration") || !strings.Contains(bypass, "4242") || strings.Contains(bypass, "RepositoryRole") {
		t.Errorf("ledger ruleset bypass after --identity: %s", bypass)
	}
	if !strings.Contains(manifest, `"contents":"write"`) || !strings.Contains(manifest, "127.0.0.1:1/callback") {
		t.Errorf("manifest: %s", manifest)
	}
	for _, leak := range []string{"MIIEsecret"} {
		for p, body := range snapshot(t, dir) {
			if strings.Contains(body, leak) {
				t.Errorf("private key in %s", p)
			}
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".aayudha", "telemetry", "actions.jsonl")); strings.Contains(string(b), leak) {
			t.Error("private key in the actions log")
		}
	}

	// Idempotent: the same flags again write nothing.
	g.Reset()
	if code, _, errOut := run(t, env, opts, args...); code != 0 || len(g.Writes()) != 0 {
		t.Fatalf("second run with extras: exit %d, writes %v: %s", code, g.Writes(), errOut)
	}
	if again := snapshot(t, dir); !mapsEqual(files, again) {
		for k := range again {
			if files[k] != again[k] {
				t.Errorf("second run changed %s:\n%s\n---\n%s", k, files[k], again[k])
			}
		}
		t.Fatal("second run with extras changed files")
	}

	// uninit removes what lives in the repository and lists the rest.
	if code, _, errOut := run(t, env, opts, "uninit", "--yes"); code != 0 {
		t.Fatalf("uninit: %d %s", code, errOut)
	}
	code, out, _ = run(t, env, opts, "uninit", "--external", "--json")
	var ext struct {
		External []initrepo.Object `json:"external"`
	}
	_ = json.Unmarshal([]byte(out), &ext)
	var kinds []string
	for _, o := range ext.External {
		kinds = append(kinds, o.Kind+":"+o.Name)
	}
	slices.Sort(kinds)
	want := []string{"app:bot", "integration:aws", "integration:azure", "integration:gcp", "integration:growthbook", "integration:posthog", "integration:sentry", "project:r Delivery", "project:r Ops", "project:r Portfolio"}
	if code != 0 || !slices.Equal(kinds, want) {
		t.Errorf("uninit --external: exit %d, %v, want %v", code, kinds, want)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if len(g.Secrets) != 0 || len(g.Variables) != 0 {
		t.Errorf("uninit left secrets %v, variables %v", g.Secrets, g.Variables)
	}
}
