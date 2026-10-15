package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// shippedFixture runs the thin slice to the end: story #2 under Feature #1
// is merged with an authoritative pack, so Feature #1 is proven. It adds
// Feature "Bulk upload" with a merged story that has no pack (unproven).
func shippedFixture(t *testing.T) (g *fake.GitHub, env map[string]string, opts Options, dir, bare string, unproven, bareStory int) {
	t.Helper()
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"}})
	g.Mu.Lock()
	g.Pulls[prN].Reviews = []fake.Review{review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	if code, out, errOut := run(t, env, opts, "ship", strconv.Itoa(prN), "--repo", "o/r"); code != 0 {
		t.Fatalf("ship: %d %s %s", code, out, errOut)
	}
	f := g.AddIssue("Bulk upload", fixtureBody(t, "feature", nil), "type:feature", "status:in-progress")
	s := g.AddIssue("Upload a CSV", fixtureBody(t, "story", nil), "type:story", "status:merged", "effort:S")
	g.Mu.Lock()
	f.Subs, s.Parent = []int{s.Number}, f.Number
	g.Mu.Unlock()
	return g, env, opts, dir, bare, f.Number, s.Number
}

func tellRun(t *testing.T, env map[string]string, opts Options, args ...string) (int, tellReport, string) {
	t.Helper()
	code, out, errOut := run(t, env, opts, append([]string{"tell", "--repo", "o/r", "--json"}, args...)...)
	var rep tellReport
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&rep)
	return code, rep, errOut
}

// WP7.5 accept: a guide references only proven Features. The brief offers
// only Features whose shipped stories all have authoritative packs; a draft
// linking an unproven Feature, a story, or an avoided word is refused with
// exit 4; a passing draft has every outbound link UTM-tagged.
func TestTellGuideCitesOnlyProvenFeatures(t *testing.T) {
	_, env, opts, dir, _, unproven, story := shippedFixture(t)

	code, rep, errOut := tellRun(t, env, opts, "guide")
	if code != 0 || len(rep.Proven) != 1 || rep.Proven[0].Number != 1 || rep.Brand.Source != "default" {
		t.Fatalf("brief: %d %+v %s", code, rep, errOut)
	}
	if len(rep.Unproven) != 1 || rep.Unproven[0].Number != unproven {
		t.Fatalf("brief lists unproven %+v, want only #%d", rep.Unproven, unproven)
	}

	draft := func(name, text string) string {
		p := filepath.Join(t.TempDir(), name)
		_ = os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	good := draft("good.md", "# Getting started\n\nAnswer the [eligibility questionnaire](https://github.com/o/r/issues/1) (#1).\n"+
		"Read [the docs](https://docs.example.com/start?lang=en), or see https://example.com/pricing.\n\n```sh\ncurl https://example.com/raw # #"+strconv.Itoa(unproven)+"\n```\n")
	code, rep, errOut = tellRun(t, env, opts, "guide", "--from", good, "--campaign", "onboarding")
	if code != 0 || rep.Draft == nil || len(rep.Draft.Cited) != 1 || rep.Draft.Cited[0] != 1 || rep.Draft.Tagged != 3 {
		t.Fatalf("good draft: %d %+v %s", code, rep, errOut)
	}
	cp, _ := os.ReadFile(rep.CopyPath)
	for _, want := range []string{
		"https://docs.example.com/start?lang=en&utm_campaign=onboarding&utm_medium=guide&utm_source=r)",
		"https://example.com/pricing?utm_campaign=onboarding&utm_medium=guide&utm_source=r.",
		"curl https://example.com/raw #", // code blocks are left alone
	} {
		if !strings.Contains(string(cp), want) {
			t.Fatalf("copy lacks %q:\n%s", want, cp)
		}
	}
	if !strings.HasPrefix(rep.CopyPath, filepath.Join(dir, ".aayudha", "tell", "guide")) {
		t.Fatalf("copy path %s", rep.CopyPath)
	}

	// The same, as the host reads it.
	code, out, _ := run(t, env, opts, "tell", "guide", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "Voice (default): Plain and specific") || !strings.Contains(out, "link them as #N:\n  #1 Eligibility questionnaire\n") ||
		!strings.Contains(out, "Not yet provable (do not claim or link): 1 Feature(s)") || !strings.Contains(out, "utm_source=r&utm_medium=guide&utm_campaign=guide-") {
		t.Fatalf("text brief: %d\n%s", code, out)
	}
	code, out, _ = run(t, env, opts, "tell", "guide", "--repo", "o/r", "--from", good, "--campaign", "onboarding")
	if code != 0 || !strings.Contains(out, "tell guide: draft passes; cites #1; 3 link(s) tagged; copy ") {
		t.Fatalf("text check: %d\n%s", code, out)
	}
	code, out, _ = run(t, env, opts, "tell", "guide", "--repo", "o/r", "--from", draft("bad.md", "\nSee #"+strconv.Itoa(story)+".\n"))
	if code != ExitBlocked || !strings.Contains(out, "refused: line 2: #"+strconv.Itoa(story)+" is not a Feature with authoritative packs") {
		t.Fatalf("text refusal: %d\n%s", code, out)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"publish without a draft", []string{"--publish"}, "tell --publish needs --from"},
		{"campaign with a space", []string{"--campaign", "launch day"}, `campaign "launch day"`},
		{"missing draft", []string{"--from", filepath.Join(dir, "nope.md")}, "tell --from"},
		{"draft is a directory", []string{"--from", dir}, "want a Markdown file"},
	} {
		if code, _, errOut := run(t, env, opts, append([]string{"tell", "guide", "--repo", "o/r"}, tt.args...)...); code != ExitUsage || !strings.Contains(errOut, tt.want) {
			t.Errorf("%s: exit %d %s, want 2 and %q", tt.name, code, errOut, tt.want)
		}
	}
	// Outside a checkout the ledger's packs cannot be read: nothing is proven, so #1 cannot be claimed.
	outside := opts
	outside.Dir = t.TempDir()
	if code, rep, _ := tellRun(t, env, outside, "guide", "--from", good); code != ExitBlocked || len(rep.Proven) != 0 || !strings.Contains(rep.ProofNote, "No checkout") {
		t.Fatalf("outside a checkout: %d %+v", code, rep)
	}
	if code, out, _ := run(t, env, outside, "tell", "guide", "--repo", "o/r"); code != 0 || !strings.Contains(out, "link them as #N:\n  none\n") || !strings.Contains(out, "No checkout") {
		t.Fatalf("text brief outside a checkout: %d\n%s", code, out)
	}
	if code, out, _ := run(t, env, outside, "tell", "launch", "--repo", "o/r", "--from", draft("plain.md", "Sign up at https://example.com.\n")); code != 0 || !strings.Contains(out, "cites none; 1 link(s) tagged") {
		t.Fatalf("a draft that claims nothing: %d\n%s", code, out)
	}
	_ = os.MkdirAll(filepath.Join(dir, "design"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "design", "brand.yaml"), []byte("voice: [\n"), 0o644)
	if code, _, errOut := run(t, env, opts, "tell", "guide", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "design/brand.yaml") {
		t.Fatalf("broken brand.yaml: %d %s", code, errOut)
	}
	_ = os.Remove(filepath.Join(dir, "design", "brand.yaml"))

	for _, tt := range []struct {
		name, text, want string
	}{
		{"unproven feature", "Upload in bulk (#" + strconv.Itoa(unproven) + ").\n", "#" + strconv.Itoa(unproven) + " is not a Feature with authoritative packs"},
		{"unproven feature by URL", "See https://github.com/o/r/issues/" + strconv.Itoa(unproven) + ".\n", "#" + strconv.Itoa(unproven) + " is not a Feature"},
		{"a story, not a Feature", "Fixed in o/r#" + strconv.Itoa(story) + ".\n", "#" + strconv.Itoa(story) + " is not a Feature"},
		{"unproven feature by a root-relative link", "[Bulk upload](/o/r/issues/" + strconv.Itoa(unproven) + ")\n", "#" + strconv.Itoa(unproven) + " is not a Feature"},
		{"avoided word", "A seamless questionnaire (#1).\n", `"seamless" is on the brand's avoid list (default)`},
	} {
		code, rep, _ := tellRun(t, env, opts, "guide", "--from", draft("bad.md", tt.text))
		if code != ExitBlocked || len(rep.Problems) != 1 || !strings.Contains(rep.Problems[0].Reason, tt.want) {
			t.Errorf("%s: exit %d %+v, want 4 and %q", tt.name, code, rep.Problems, tt.want)
		}
	}
}

// WP7.5 accept: publish refuses a bot identity (an agent lane, a GitHub bot
// account, a configured bot login) and writes nothing; a human's publish lands
// the copy and its record on the ledger, sets the tag's release notes, and is
// one tell.publish action in the log.
func TestTellPublishHumanOnly(t *testing.T) {
	g, env, opts, dir, bare, _, _ := shippedFixture(t)
	g.Mu.Lock()
	g.Releases = map[string]*fake.Release{"v0.1.0": {ID: 41, TagName: "v0.1.0", Body: "goreleaser changelog", HTMLURL: "https://github.com/o/r/releases/tag/v0.1.0"}}
	g.Mu.Unlock()
	notes := filepath.Join(t.TempDir(), "notes.md")
	_ = os.WriteFile(notes, []byte("## v0.1.0\n\n### Eligibility questionnaire ([#1](https://github.com/o/r/issues/1))\n\n- Show empty list\n"), 0o644)
	publish := func(env map[string]string, extra ...string) (int, tellReport, string) {
		return tellRun(t, env, opts, append([]string{"release", "--from", notes, "--tag", "v0.1.0", "--publish"}, extra...)...)
	}
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "PATH": env["PATH"]}
	g.Reset()
	for _, tt := range []struct {
		name  string
		env   map[string]string
		setup func()
		extra []string
		want  string
	}{
		{"agent lane", agent, nil, nil, "refused for an agent lane"},
		{"CI token", env, nil, []string{"--ci"}, "refused in CI"},
		{"GitHub bot account", env, func() { g.ViewerType = "Bot" }, nil, "is a bot identity"},
		{"configured bot login", env, func() { g.ViewerType = "User" }, []string{"--set", "identity.bot_logins=founder"}, "founder is a bot identity"},
	} {
		if tt.setup != nil {
			g.Mu.Lock()
			tt.setup()
			g.Mu.Unlock()
		}
		code, _, errOut := publish(tt.env, tt.extra...)
		if code != ExitBlocked || !strings.Contains(errOut, tt.want) {
			t.Fatalf("%s: exit %d %s, want 4 and %q", tt.name, code, errOut, tt.want)
		}
	}
	if w := g.Writes(); len(w) != 0 {
		t.Fatalf("refused publishes wrote to GitHub: %+v", w)
	}
	if out := git(t, bare, "ls-tree", "-r", "--name-only", "ledger"); strings.Contains(out, "tell/") {
		t.Fatalf("refused publishes wrote the ledger:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".aayudha", "tell")); !os.IsNotExist(err) {
		t.Fatalf("refused publishes wrote a local copy: %v", err)
	}

	if code, out, errOut := run(t, env, opts, "tell", "release", "--repo", "o/r", "--from", notes, "--tag", "v0.1.0", "--publish", "--dry-run"); code != 0 ||
		!strings.Contains(out, "would publish as tell/release/v0.1.0.md by founder") {
		t.Fatalf("dry-run publish: %d %s %s", code, out, errOut)
	}
	if w := g.Writes(); len(w) != 0 {
		t.Fatalf("a dry-run publish wrote to GitHub: %+v", w)
	}
	code, out, errOut := run(t, env, opts, "tell", "release", "--repo", "o/r", "--from", notes, "--tag", "v0.1.0", "--publish")
	if code != 0 || !strings.Contains(out, "published as tell/release/v0.1.0.md by founder; release notes set on https://github.com/o/r/releases/tag/v0.1.0") {
		t.Fatalf("publish: %d %s %s", code, out, errOut)
	}
	code, rep, errOut := publish(env)
	if code != 0 || rep.Published == nil || rep.Published.PublishedBy != "founder" || rep.Published.Release == "" {
		t.Fatalf("publish: %d %+v %s", code, rep.Published, errOut)
	}
	if got := git(t, bare, "show", "ledger:tell/release/v0.1.0.md"); !strings.Contains(got, "utm_campaign=v0.1.0&utm_medium=release&utm_source=r") {
		t.Fatalf("published copy:\n%s", got)
	}
	var pub Publication
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:tell/release/v0.1.0.json")), &pub)
	if pub.PublishedBy != "founder" || len(pub.Cited) != 1 || pub.Cited[0] != 1 || len(pub.SHA256) != 64 {
		t.Fatalf("publication record: %+v", pub)
	}
	g.Mu.Lock()
	body := g.Releases["v0.1.0"].Body
	g.Mu.Unlock()
	if !strings.HasPrefix(body, "goreleaser changelog\n\n"+tellMarker+"\n## v0.1.0") || strings.Count(body, tellMarker) != 2 {
		t.Fatalf("release body:\n%s", body)
	}
	if code, _, _ := publish(env); code != 0 {
		t.Fatalf("second publish: %d", code)
	}
	// A tag without a GitHub Release (rolling-*, which goreleaser ignores) publishes to the ledger only.
	code, rep, errOut = tellRun(t, env, opts, "release", "--from", notes, "--tag", "rolling-2026-10-12", "--publish")
	if code != 0 || rep.Published.Release != "" || !strings.Contains(git(t, bare, "show", "ledger:tell/release/rolling-2026-10-12.json"), `"tag": "rolling-2026-10-12"`) {
		t.Fatalf("publish for a rolling tag: %d %+v %s", code, rep.Published, errOut)
	}
	g.Mu.Lock()
	again := g.Releases["v0.1.0"].Body
	g.Mu.Unlock()
	if again != body {
		t.Fatalf("republishing changed the notes instead of replacing them in place:\n%s", again)
	}
	if code, out, _ := run(t, env, Options{Dir: dir}, "actions", "grep", "tell.publish"); code != 0 || !strings.Contains(out, `"login":"founder"`) {
		t.Fatalf("actions log: %d %s", code, out)
	}
}

// WP8.2: tell docs --site builds the docs site from the checkout, the
// registry, and the embedded skills; a page that claims an unproven Feature
// refuses the site and writes nothing.
func TestTellDocsSite(t *testing.T) {
	_, env, opts, dir, _, unproven, _ := shippedFixture(t)
	opts.Assets = aayudha.Assets
	readme := func(text string) { _ = os.WriteFile(filepath.Join(dir, "README.md"), []byte(text), 0o644) }
	readme("# r\n\nThe [eligibility questionnaire](https://github.com/o/r/issues/1) ships with proof. See [CONSTRAINTS.md](CONSTRAINTS.md).\n")
	_ = os.WriteFile(filepath.Join(dir, "CONSTRAINTS.md"), []byte("# Constraints\n"), 0o644)

	site := filepath.Join(t.TempDir(), "site")
	code, rep, errOut := tellRun(t, env, opts, "docs", "--site", site)
	if code != 0 || len(rep.Cited) != 1 || rep.Cited[0] != 1 || len(rep.Problems) != 0 {
		t.Fatalf("site: %d %+v %s", code, rep, errOut)
	}
	for file, want := range map[string]string{
		"index.html":        `<a href="constraints.html">CONSTRAINTS.md</a>`,
		"constraints.html":  `<h1 id="constraints">Constraints</h1>`,
		"verbs.html":        `<h2 id="tell"><code>tell</code></h2>`,
		"skills/tell.html":  `aayu tell docs --site _site`,
		"skills/grill.html": `<a href="../verbs.html">Verbs</a>`,
	} {
		if b, err := os.ReadFile(filepath.Join(site, filepath.FromSlash(file))); err != nil || !strings.Contains(string(b), want) {
			t.Errorf("%s lacks %s (%v)", file, want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(site, "getting-started.html")); !os.IsNotExist(err) {
		t.Errorf("a page without a source in the checkout was built: %v", err)
	}

	// With a guide, its README is the home page, README.md the overview, and
	// the guide's images are served from the site.
	_ = os.MkdirAll(filepath.Join(dir, "docs", "guide", "img"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "docs", "guide", "README.md"), []byte("# Welcome\n\n![Nine steps](img/journey.svg)\n\nRead the [overview](../../README.md).\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "docs", "guide", "img", "journey.svg"), []byte("<svg/>"), 0o644)
	guided := filepath.Join(t.TempDir(), "guided")
	if code, _, errOut := tellRun(t, env, opts, "docs", "--site", guided); code != 0 {
		t.Fatalf("guided site: %d %s", code, errOut)
	}
	for file, want := range map[string]string{
		"index.html":                 `<img src="docs/guide/img/journey.svg" alt="Nine steps">`,
		"overview.html":              `<a href="constraints.html">CONSTRAINTS.md</a>`,
		"docs/guide/img/journey.svg": `<svg/>`,
		"skills/grill.html":          `<a href="../overview.html">Project overview</a>`,
	} {
		if b, err := os.ReadFile(filepath.Join(guided, filepath.FromSlash(file))); err != nil || !strings.Contains(string(b), want) {
			t.Errorf("guided %s lacks %s (%v)", file, want, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(guided, "index.html")); !strings.Contains(string(b), `<a href="overview.html">overview</a>`) {
		t.Errorf("the guide home does not link the overview:\n%s", b)
	}

	readme("# r\n\nBulk upload is next (#" + strconv.Itoa(unproven) + ").\n")
	refused := filepath.Join(t.TempDir(), "refused")
	code, out, _ := run(t, env, opts, "tell", "docs", "--repo", "o/r", "--site", refused)
	if code != ExitBlocked || !strings.Contains(out, "refused: README.md line 3: #"+strconv.Itoa(unproven)+" is not a Feature") {
		t.Fatalf("unproven claim: %d %s", code, out)
	}
	if _, err := os.Stat(refused); !os.IsNotExist(err) {
		t.Fatalf("a refused site was written: %v", err)
	}
	for _, args := range [][]string{{"guide", "--site", refused}, {"docs", "--site", refused, "--from", "README.md"}, {"docs", "--site", site}} {
		if code, _, errOut := run(t, env, opts, append([]string{"tell", "--repo", "o/r"}, args...)...); code != ExitUsage {
			t.Errorf("%v: exit %d %s, want 2", args, code, errOut)
		}
	}
}
