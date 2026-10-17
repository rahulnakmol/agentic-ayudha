package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// seatFixture is a checkout of a repository shaped like aayudha in solo mode:
// its CODEOWNERS, config, and an AGENTS.md of the product's own, a ledger
// branch on a local bare origin, and a fake GitHub holding the solo-mode
// ruleset as it is live on aayudha (Appendix B D2) and one human, rahulnakmol.
func seatFixture(t *testing.T) (dir, bare string, g *fake.GitHub, files map[string]string) {
	t.Helper()
	dir, bare = ledgerCheckout(t)
	files = map[string]string{}
	for src, dst := range map[string]string{"CODEOWNERS": ".github/CODEOWNERS", "config.toml": ".aayudha/config.toml", "AGENTS.md": "AGENTS.md"} {
		b, err := os.ReadFile(filepath.Join("testdata", "seat", src))
		if err != nil {
			t.Fatal(err)
		}
		files[dst] = string(b)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, dst)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, dst), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g = fake.New(t, "o/r", "User")
	g.Viewer = "rahulnakmol"
	g.Files = map[string]string{".github/CODEOWNERS": files[".github/CODEOWNERS"]}
	var rs map[string]any
	b, _ := os.ReadFile(filepath.Join("testdata", "seat", "ruleset.json"))
	if err := json.Unmarshal(b, &rs); err != nil {
		t.Fatal(err)
	}
	rs["id"] = int64(rs["id"].(float64)) // the fake matches IDs as printed
	g.Rulesets = []map[string]any{rs}
	return dir, bare, g, files
}

func seatRun(t *testing.T, dir string, g *fake.GitHub, args ...string) (int, seats.Result, string) {
	t.Helper()
	opts := Options{IsTerminal: stdinTTY, Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets}
	code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, append([]string{"seat", "--repo", "o/r", "--json", "--yes"}, args...)...)
	var res seats.Result
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	return code, res, errOut
}

func readRepo(t *testing.T, dir, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// WP5.1 accept: `seat add` produces the expected CODEOWNERS diff and onboarding
// issue, switches the solo-mode policy once the second human is a code owner on
// the default branch, and `seat remove` reverses it and leaves no story with a
// dangling human owner.
func TestSeatAddRemove(t *testing.T) {
	dir, bare, g, orig := seatFixture(t)
	// An open story owned by the human about to leave, with its Ready snapshot,
	// and one owned by someone else that must not change.
	story := g.AddIssue("Export claims", "### Summary\n\nAs a member, I want CSV.\n\n### Technical Owner\n\n@alice\n\n### Links\n\ndepends-on #9\n", "type:story", "status:ready")
	story.Comments = append(story.Comments, &fake.Comment{ID: 501, Body: ready.SnapshotMarker + "\n| Field | Value |\n|---|---|\n| Technical Owner | @alice |\n| Body hash | `abc` |\n"})
	other := g.AddIssue("Totals", "### Technical Owner\n\n@alicea\n", "type:bug")

	// 1. Add the second human. The CODEOWNERS change is local; the default
	// branch still names one human, so the policy stays solo (pending).
	code, res, errOut := seatRun(t, dir, g, "add", "ux-researcher-evolutionist", "--human", "@alice")
	if code != 0 {
		t.Fatalf("seat add: exit %d: %s", code, errOut)
	}
	wantBlock := seats.BlockBegin + `
/packages/design-system/  @alice @rahulnakmol
/examples/benefits-qa/web/  @alice @rahulnakmol
/.github/  @rahulnakmol @alice
/skills/  @rahulnakmol @alice
/ops/  @rahulnakmol @alice
/seats/  @rahulnakmol @alice
/CONSTRAINTS.md  @rahulnakmol @alice
/internal/config/defaults.toml  @rahulnakmol @alice
/.aayudha/config.toml  @rahulnakmol @alice
/.agents/  @rahulnakmol @alice
/.claude/  @rahulnakmol @alice
` + seats.BlockEnd + "\n"
	if got, want := readRepo(t, dir, ".github/CODEOWNERS"), orig[".github/CODEOWNERS"]+"\n"+wantBlock; got != want {
		t.Errorf("CODEOWNERS after add:\n%s\nwant:\n%s", got, want)
	}
	if got := readRepo(t, dir, ".aayudha/config.toml"); got != orig[".aayudha/config.toml"]+"\n[seats.ux-researcher-evolutionist]\nmode = \"paired\"\nhuman = \"alice\"\n" {
		t.Errorf("config after add:\n%s", got)
	}
	if agents := readRepo(t, dir, "AGENTS.md"); !strings.HasPrefix(agents, orig["AGENTS.md"]) || !strings.Contains(agents, "<!-- BEGIN aayudha:raci -->") {
		t.Errorf("AGENTS.md lost its own content or lacks the RACI section:\n%s", agents)
	}
	if res.Policy != seats.PolicyPending || res.Ruleset != "unchanged" || !reflect.DeepEqual(res.Humans, []string{"rahulnakmol", "alice"}) {
		t.Errorf("first add: policy %q, ruleset %q, humans %v", res.Policy, res.Ruleset, res.Humans)
	}
	is := g.Issues[res.Issue]
	if is == nil || is.Title != "Seat onboarding: UX Researcher & Evolutionist (@alice)" || g.LabelNames(res.Issue)[0] != seats.OnboardingLabel {
		t.Fatalf("onboarding issue #%d: %+v", res.Issue, is)
	}
	for _, want := range []string{"@alice, you are now the paired human", "## How we work", "| L01 | Experience |", "`review` | Accountable", "### Purpose", "Owns the experience", "**Iris, the UX Researcher & Evolutionist twin**", "**When paired with a human:**"} {
		if !strings.Contains(is.Body, want) {
			t.Errorf("onboarding issue body lacks %q:\n%s", want, is.Body)
		}
	}
	if strings.Contains(is.Body, "BEGIN GENERATED") {
		t.Errorf("onboarding issue repeats the charter's generated region")
	}

	// 2. The CODEOWNERS change merged (through the solo-mode bypass). Rerunning
	// switches the ruleset: one approval with native CODEOWNER review, no gate
	// check, no bypass. The onboarding issue is not posted twice.
	g.Files[".github/CODEOWNERS"] = readRepo(t, dir, ".github/CODEOWNERS")
	issues := len(g.Issues)
	code, res, errOut = seatRun(t, dir, g, "add", "ux-researcher-evolutionist", "--human", "alice")
	if code != 0 || res.Policy != seats.PolicyTwoHuman || res.Ruleset != "updated" || len(g.Issues) != issues {
		t.Fatalf("second add: exit %d %+v, %d issues: %s", code, res, len(g.Issues), errOut)
	}
	rs := g.Rulesets[0]
	pr := rs["rules"].([]any)[0].(map[string]any)["parameters"].(map[string]any)
	if pr["required_approving_review_count"] != float64(1) || pr["require_code_owner_review"] != true ||
		len(rs["rules"].([]any)) != 1 || len(rs["bypass_actors"].([]any)) != 0 {
		t.Errorf("ruleset after the switch: %v", rs)
	}

	// 3. Remove: the ruleset returns to solo mode exactly as it was, the files
	// return byte for byte, and the open story goes back to the twin.
	code, res, errOut = seatRun(t, dir, g, "remove", "ux-researcher-evolutionist")
	if code != 0 || res.Policy != seats.PolicySolo || res.Ruleset != "updated" {
		t.Fatalf("remove: exit %d %+v: %s", code, res, errOut)
	}
	var want map[string]any
	b, _ := os.ReadFile(filepath.Join("testdata", "seat", "ruleset.json"))
	_ = json.Unmarshal(b, &want)
	for _, k := range []string{"rules", "bypass_actors"} {
		if !reflect.DeepEqual(norm(t, g.Rulesets[0][k]), norm(t, want[k])) {
			t.Errorf("ruleset %s after remove:\n%v\nwant:\n%v", k, g.Rulesets[0][k], want[k])
		}
	}
	for _, p := range []string{".github/CODEOWNERS", ".aayudha/config.toml"} {
		if got := readRepo(t, dir, p); got != orig[p] {
			t.Errorf("%s after remove:\n%s\nwant:\n%s", p, got, orig[p])
		}
	}
	twin := "the UX Researcher & Evolutionist twin"
	if !reflect.DeepEqual(res.Reassigned, []int{story.Number}) || !strings.Contains(story.Body, "### Technical Owner\n\n"+twin+"\n\n### Links") ||
		!strings.Contains(story.Comments[0].Body, "| Technical Owner | "+twin+" |") || other.Body != "### Technical Owner\n\n@alicea\n" {
		t.Errorf("reassigned %v; story body:\n%s\nsnapshot:\n%s\nother: %q", res.Reassigned, story.Body, story.Comments[0].Body, other.Body)
	}
	for _, is := range g.Issues {
		if is.State == "open" && strings.Contains(is.Body+commentBodies(is.Comments), "@alice\n") {
			t.Errorf("#%d still names @alice as owner", is.Number)
		}
	}
	if log := git(t, bare, "show", "ledger:"+seats.LedgerLog); strings.Count(log, "\n") != 1 ||
		!strings.Contains(log, `"action":"add"`) || !strings.Contains(log, `"action":"remove"`) || !strings.Contains(log, `"reassigned":[1]`) {
		t.Errorf("ledger %s:\n%s", seats.LedgerLog, log)
	}

	// seat list shows the seats from config; agents may list but never change seats.
	code, out, _ := run(t, map[string]string{"AAYU_AGENT": "1"}, Options{Dir: dir}, "seat", "list", "--json")
	if code != 0 || !strings.Contains(out, `"policy": "solo"`) || strings.Count(out, `"mode": "solo"`) != 14 {
		t.Errorf("seat list: exit %d\n%s", code, out)
	}
	if code, _, _ := run(t, map[string]string{"AAYU_AGENT": "1"}, Options{Dir: dir}, "seat", "add", "auditor", "--human", "bob"); code != ExitBlocked {
		t.Errorf("seat add by an agent: exit %d, want %d", code, ExitBlocked)
	}
}

// norm round-trips v through JSON so ints and float64s compare equal.
func norm(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
