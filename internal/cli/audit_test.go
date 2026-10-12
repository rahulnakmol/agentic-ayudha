package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// auditFixture is a synthetic copy of this repository's own history from the
// hand-run reconciliations (ledger audit/2026-10-11/…, audit/2026-10-12/…):
// a deploy-key probe push that a hand-run report accounts for, a protected-path
// merge with a prior "Solo-mode bypass:" comment, one without, one that also
// bypassed a failing build, founder-identity ledger writes (one by deploy key),
// a rolling promotion after a green gate run, a test tag that release.yml
// refused, a story shipped with `ship --solo-bypass` and covered by an
// authoritative pack, a story merged before the merge gate existed, a Decision
// past its SLA, an agent PR with a hand commit on a never-layer, a direct `gh`
// label write that no actions segment holds, and a workflow write.
type auditFixture struct {
	g          *fake.GitHub
	env        map[string]string
	opts       Options
	dir, bare  string
	ledgerTip  string
	reportPath string
}

const (
	auditSince = "2026-10-11T00:00:00Z"
	auditUntil = "2026-10-12T04:00:00Z"
	auditCode  = "c0dec0dec0dec0dec0dec0dec0dec0dec0dec0de"
	auditR1    = "1111111111111111111111111111111111111111" // rolling promotion commit
	auditT1    = "2222222222222222222222222222222222222222" // test tag commit
	auditM8    = "8888888888888888888888888888888888888888"
	auditM9    = "9999999999999999999999999999999999999999"
	auditM86   = "8686868686868686868686868686868686868686"
	auditM92   = "9292929292929292929292929292929292929292"
	auditM95   = "9595959595959595959595959595959595959595"
	auditHead  = "44d18af847494eb1bb088fd27c568d22d270a9fb"
	auditCG    = "a6abff0b46f34e6c923c4c125cc9f7a77ae6ef25c3aeb0d4bf4e632233e76a4c"
)

// gitAt runs git with a fixed author, committer, and date and without the
// machine's git configuration, so ledger commits (and their SHAs, which the
// report names) are the same on every machine.
func gitAt(t *testing.T, dir, date string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Amp", "GIT_AUTHOR_EMAIL=amp@example.com", "GIT_COMMITTER_NAME=Amp",
		"GIT_COMMITTER_EMAIL=amp@example.com", "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date,
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1") // no hooks, templates, or signing from the machine's config
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func jsonOf(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("fixture JSON: %v\n%s", err, s)
	}
	return v
}

func newAuditFixture(t *testing.T) *auditFixture {
	t.Helper()
	f := &auditFixture{}
	root := t.TempDir()
	f.bare = filepath.Join(root, "origin.git")
	git(t, root, "init", "--quiet", "--bare", f.bare)
	seed := filepath.Join(root, "seed")
	git(t, root, "clone", "--quiet", f.bare, seed)
	gitAt(t, seed, "2026-10-11T19:00:00Z", "checkout", "--quiet", "--orphan", "ledger")
	commit := func(date, msg string, files map[string]string) string {
		writeFiles(t, seed, files)
		gitAt(t, seed, date, "add", ".")
		gitAt(t, seed, date, "commit", "--quiet", "-m", msg)
		return gitAt(t, seed, date, "rev-parse", "HEAD")
	}
	commit("2026-10-11T19:00:00Z", "docs(ledger): describe the ledger", map[string]string{"README.md": "ledger\n"})
	commit("2026-10-11T19:45:35Z", "docs(audit): hand-run solo-mode bypass reconciliation for 2026-10-11", map[string]string{
		"audit/2026-10-11/solo-mode-bypass-reconciliation.md": "# Solo-mode bypass reconciliation (hand-run)\n\n| Rule suite | Ref | Classification |\n|---|---|---|\n" +
			"| 4471059806 | `ledger-probe` | Expected: WP0.1 live test D1, deploy-key push |\n"})
	snapshot := "# o/r#91: doctor names the floor each missing tool serves\n\n| | |\n|---|---|\n| Body hash | `" + strings.Repeat("b", 64) + "` |\n" +
		"| Completion Goal hash | `" + auditCG + "` |\n\n## Body at Ready\n\n````markdown\n### Completion Goal\n\n" +
		"1. [api-trace] state: doctor without semgrep; expected: the warning names the sast floor\n````\n"
	ledgerSnap := commit("2026-10-11T23:46:18Z", "specs: Ready snapshot of #91\n\nAayudha-Identity: identity:human-fallback", map[string]string{"specs/91/ready.md": snapshot})

	evidence := []byte(`{"Action":"pass","Test":"TestOptionalToolsNamesFloors"}` + "\n")
	pack := &proof.Pack{Repo: "o/r", Story: 91, PR: 92, PRHeadSHA: auditHead, Checkpoint: "1/1", Seat: "ai-native-engineer", Host: "cli", Identity: "ci",
		Authority:          proof.Authority{Kind: proof.Authoritative, Workflow: ".github/workflows/aayudha-merge-gate.yml", RunID: 38197859955, Writer: "founder (aayu ship)"},
		CompletionGoalHash: auditCG, Criteria: []proof.Criterion{{ID: 1, Evidence: []string{"evidence/criterion-1.jsonl"}}},
		Artifacts: []proof.Artifact{proof.NewArtifact("evidence/criterion-1.jsonl", "api-trace", evidence)},
		Checks:    []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "dev"}},
		CreatedAt: "2026-10-12T03:00:00Z"}
	pack.Artifacts[0].Store = proof.StoreGit
	if err := proof.WriteLocal(filepath.Join(seed, "proof", "91", auditHead, "authoritative"), pack, map[string][]byte{"evidence/criterion-1.jsonl": evidence}); err != nil {
		t.Fatal(err)
	}
	advisory := *pack
	advisory.Story, advisory.PR, advisory.PRHeadSHA, advisory.Authority = 48, 86, auditM86, proof.Authority{Kind: proof.Advisory}
	advisory.Checks = []proof.Check{{Floor: "coverage", Result: proof.Fail, Value: 61.5, Threshold: 90, Tool: "aayudha", Version: "dev", Detail: "8/13 changed lines"}}
	advisory.CreatedAt = "2026-10-12T02:10:00Z"
	if err := proof.WriteLocal(filepath.Join(seed, "proof", "48", auditM86, "advisory"), &advisory, map[string][]byte{"evidence/criterion-1.jsonl": evidence}); err != nil {
		t.Fatal(err)
	}
	commit("2026-10-12T03:04:14Z", "proof: authoritative pack for #91 at 44d18af84749", nil)
	shipRec := `{"repo":"o/r","story":91,"pr":92,"pr_head_sha":"` + auditHead + `","merge_commit":"` + auditM92 + `",` +
		`"pack":"proof/91/` + auditHead + `/authoritative/pack.json","human_pass":{"kind":"solo-bypass","login":"founder",` +
		`"comment_url":"https://github.com/o/r/pull/92#issuecomment-6116996259","commit":"` + auditHead + `"},` +
		`"actions_segment":"actions/2026-10-12/amp-20261012T030441Z.jsonl","shipped_at":"2026-10-12T03:04:45Z","shipped_by":"founder"}` + "\n"
	ledgerShip := commit("2026-10-12T03:04:52Z", "proof: ship record of #91 (PR #92)", map[string]string{"proof/91/" + auditHead + "/authoritative/ship.json": shipRec})
	seg := strings.Join([]string{
		`{"seq":1,"time":"2026-10-12T02:33:30Z","run":"20261012T023326Z","kind":"github.pull.create","target":"o/r","story":"#91","identity":"human-fallback","request":{"method":"POST","path":"/repos/o/r/pulls"},"status":"done","result":{"number":92},"tokens_in":0,"tokens_out":0,"cost_usd":0,"wall_time_ms":1,"attempts":1,"prev":""}`,
		`{"seq":2,"time":"2026-10-12T03:04:45Z","run":"20261012T030441Z","kind":"github.pull.merge","target":"o/r#92","story":"#91","identity":"human","request":{"method":"PUT","path":"/repos/o/r/pulls/92/merge"},"payload":{"human_pass":"solo-bypass"},"status":"done","tokens_in":0,"tokens_out":0,"cost_usd":0,"wall_time_ms":1,"attempts":1,"prev":""}`,
		`{"seq":3,"time":"2026-10-12T03:04:47Z","run":"20261012T030441Z","kind":"github.issue.labels.remove","target":"o/r#91","identity":"human","request":{"method":"DELETE","path":"/repos/o/r/issues/91/labels/status:ready"},"status":"done","tokens_in":0,"tokens_out":0,"cost_usd":0,"wall_time_ms":1,"attempts":1,"prev":""}`,
		`{"seq":4,"time":"2026-10-12T03:04:49Z","run":"20261012T030441Z","kind":"github.issue.labels.add","target":"o/r#91","identity":"human","request":{"method":"POST","path":"/repos/o/r/issues/91/labels"},"payload":{"labels":["status:merged"]},"status":"done","tokens_in":0,"tokens_out":0,"cost_usd":0,"wall_time_ms":1,"attempts":1,"prev":""}`,
		`{"seq":5,"time":"2026-10-12T03:04:52Z","run":"20261012T030441Z","kind":"ledger.push","target":"o/r@ledger","identity":"human","status":"done","result":{"commit":"` + ledgerShip + `"},"tokens_in":0,"tokens_out":0,"cost_usd":0.42,"wall_time_ms":1,"attempts":1,"prev":"","story":"#91"}`,
	}, "\n") + "\n"
	f.ledgerTip = commit("2026-10-12T03:06:31Z", "actions: segment amp-20261012T030441Z", map[string]string{"actions/2026-10-12/amp-20261012T030441Z.jsonl": seg})
	gitAt(t, seed, "2026-10-12T03:06:31Z", "push", "--quiet", "origin", "ledger")
	gitAt(t, seed, "2026-10-11T19:00:00Z", "checkout", "--quiet", "--orphan", "dev")
	gitAt(t, seed, "2026-10-11T19:00:00Z", "rm", "-rf", "--quiet", ".")
	gitAt(t, seed, "2026-10-11T19:00:00Z", "commit", "--quiet", "--allow-empty", "-m", "dev")
	gitAt(t, seed, "2026-10-11T19:00:00Z", "push", "--quiet", "origin", "dev")
	f.dir = filepath.Join(root, "work")
	git(t, root, "clone", "--quiet", "--branch", "dev", f.bare, f.dir)

	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	failed := func(ruleset, typ, details string) string {
		return fmt.Sprintf(`{"rule_source":{"type":"ruleset","name":%q},"result":"fail","rule_type":%q,"details":%q},{"rule_source":{"type":"ruleset","name":"dev-integration"},"result":"pass","rule_type":"pull_request"}`, ruleset, typ, details)
	}
	pp := `Required status check "protected-paths" is failing.`
	suites := []struct {
		id                 int64
		ref, actor, after  string
		at, ruleset, rtype string
		details            string
	}{
		{4470000000, "refs/heads/dev", "founder", auditM8, "2026-10-10T10:00:00Z", "protected-paths-codeowner-review", "required_status_checks", pp},
		{4471059806, "refs/heads/ledger-probe", "", "e17ff0a9aad7209148352479bdd0520806b0b4c0", "2026-10-11T19:34:07Z", "ledger-writers", "update", "Cannot update this protected ref."},
		{4471759694, "refs/heads/dev", "founder", auditM8, "2026-10-11T21:23:37Z", "protected-paths-codeowner-review", "required_status_checks", pp},
		{4473766681, "refs/heads/dev", "founder", auditM86, "2026-10-12T02:20:04Z", "protected-paths-codeowner-review", "required_status_checks", pp},
		{4472680925, "refs/heads/ledger", "founder", ledgerSnap, "2026-10-11T23:46:18Z", "ledger-writers", "update", "Cannot update this protected ref."},
		{4472075201, "refs/heads/rolling", "founder", auditR1, "2026-10-11T22:14:02Z", "rolling-main-promotion", "update", "Cannot update this protected ref."},
		{4471739456, "refs/tags/v0.0.0-d4test.1", "founder", auditT1, "2026-10-11T21:20:30Z", "release-tags", "creation", "Cannot create this tag"},
		{4471851556, "refs/heads/dev", "founder", auditM9, "2026-10-11T21:38:34Z", "protected-paths-codeowner-review", "required_status_checks", `Required status checks "build", "protected-paths" are failing.`},
		{4474099541, "refs/heads/ledger", "", ledgerShip, "2026-10-12T03:04:52Z", "ledger-writers", "update", "Cannot update this protected ref."},
	}
	var list []string
	g.Static = map[string]any{}
	for _, s := range suites {
		actor := "null"
		if s.actor != "" {
			actor = fmt.Sprintf("%q", s.actor)
		}
		head := fmt.Sprintf(`"id":%d,"ref":%q,"actor_name":%s,"before_sha":"0000000000000000000000000000000000000001","after_sha":%q,"pushed_at":%q,"result":"bypass"`, s.id, s.ref, actor, s.after, s.at)
		list = append(list, "{"+head+"}")
		g.Static[fmt.Sprintf("/repos/o/r/rulesets/rule-suites/%d", s.id)] = jsonOf(t, "{"+head+`,"rule_evaluations":[`+failed(s.ruleset, s.rtype, s.details)+"]}")
	}
	g.Static["/repos/o/r/rulesets/rule-suites"] = jsonOf(t, "["+strings.Join(list, ",")+"]")

	user := `{"login":"founder","type":"User"}`
	pr := func(n int, title, head, body, merged, sha, labels string) string {
		return fmt.Sprintf(`{"number":%d,"title":%q,"state":"closed","body":%q,"user":%s,"head":{"ref":%q},"base":{"ref":"dev"},"merged_at":%q,"merge_commit_sha":%q,"labels":[%s]}`,
			n, title, body, user, head, merged, sha, labels)
	}
	g.Static["/repos/o/r/pulls"] = jsonOf(t, "["+strings.Join([]string{
		pr(8, "feat: CI suite (WP0.4)", "feat/ci", "PR2 of the build plan.", "2026-10-11T21:23:37Z", auditM8, ""),
		pr(9, "feat: design tokens", "feat/tokens", "Design system.", "2026-10-11T21:38:35Z", auditM9, ""),
		pr(86, "feat(inspect): merge gate (WP4.8)", "feat/gate", "Refs #48", "2026-10-12T02:20:05Z", auditM86, ""),
		pr(92, "feat: doctor names the floor each missing tool serves", "story/91-doctor-names-floors", "Refs #91", "2026-10-12T03:04:44Z", auditM92, `{"name":"identity:human-fallback"}`),
		pr(95, "fix(verify): deps floor", "fix/deps", "Refs #44\n\nFound while running M4.5 (story #91, PR #92).", "2026-10-12T02:57:33Z", auditM95, ""),
	}, ",")+"]")
	issue := func(n int, title, labels, created, updated string, isPR bool) string {
		extra := ""
		if isPR {
			extra = `,"pull_request":{"url":"x"}`
		}
		return fmt.Sprintf(`{"number":%d,"title":%q,"user":%s,"labels":[%s],"created_at":%q,"updated_at":%q,"state":"open"%s}`, n, title, user, labels, created, updated, extra)
	}
	lbl := func(names ...string) string {
		var out []string
		for _, n := range names {
			out = append(out, fmt.Sprintf(`{"name":%q}`, n))
		}
		return strings.Join(out, ",")
	}
	floorMiss := "Feedback: retro: floor miss: coverage fail on #48 (advisory pack at 8686868)"
	g.Static["/repos/o/r/issues"] = jsonOf(t, "["+strings.Join([]string{
		issue(8, "feat: CI suite (WP0.4)", "", "2026-10-11T21:17:43Z", "2026-10-11T21:23:37Z", true),
		issue(9, "feat: design tokens", "", "2026-10-11T21:24:26Z", "2026-10-11T21:38:35Z", true),
		issue(48, "WP4.8 Merge gate + `inspect`", lbl("type:story", "status:ready", "effort:M", "priority:P1", "priority:P0"), "2026-10-11T23:35:00Z", "2026-10-12T01:10:00Z", false),
		issue(86, "feat(inspect): merge gate (WP4.8)", "", "2026-10-12T01:50:00Z", "2026-10-12T02:23:33Z", true),
		issue(91, "doctor names the floor each missing tool serves", lbl("type:story", "status:merged", "effort:XS", "identity:human-fallback"), "2026-10-12T02:31:00Z", "2026-10-12T03:04:49Z", false),
		issue(92, "feat: doctor names the floor each missing tool serves", lbl("identity:human-fallback"), "2026-10-12T02:33:30Z", "2026-10-12T03:04:44Z", true),
		issue(95, "fix(verify): deps floor", "", "2026-10-12T02:50:00Z", "2026-10-12T02:57:33Z", true),
		issue(97, "Engine choice", lbl("type:decision"), "2026-10-06T09:00:00Z", "2026-10-06T09:00:00Z", false),
		issue(99, floorMiss, lbl("type:feedback", "status:backlog", "intake"), "2026-10-09T09:00:00Z", "2026-10-09T09:00:00Z", false),
		issue(120, "Created after the cursor", lbl("type:story"), "2026-10-12T05:00:00Z", "2026-10-12T05:00:00Z", false),
	}, ",")+"]")
	ev := func(kind, actor, at, extra string) string {
		who := "actor"
		if kind == "commented" {
			who = "user"
		}
		return fmt.Sprintf(`{"event":%q,%q:{"login":%q},"created_at":%q%s}`, kind, who, actor, at, extra)
	}
	label := func(name string) string { return fmt.Sprintf(`,"label":{"name":%q}`, name) }
	body := func(id int64, text string) string { return fmt.Sprintf(`,"id":%d,"body":%q`, id, text) }
	timelines := map[int][]string{
		8: {ev("commented", "founder", "2026-10-11T21:23:34Z", body(6113923044, "Solo-mode bypass: this PR changes `.github/workflows/ci.yml`; every other check is green.")),
			ev("merged", "founder", "2026-10-11T21:23:37Z", ""), ev("closed", "founder", "2026-10-11T21:23:37Z", "")},
		9: {ev("commented", "founder", "2026-10-11T21:38:30Z", body(6114062959, "Solo-mode bypass: design system.")), ev("merged", "founder", "2026-10-11T21:38:35Z", "")},
		48: {ev("labeled", "founder", "2026-10-11T23:35:00Z", label("type:story")), ev("labeled", "founder", "2026-10-11T23:35:00Z", label("effort:M")),
			ev("labeled", "founder", "2026-10-11T23:35:00Z", label("priority:P1")), ev("labeled", "founder", "2026-10-11T23:46:22Z", label("status:ready")),
			ev("labeled", "founder", "2026-10-12T01:00:00Z", label("priority:P0")),
			ev("commented", "github-actions[bot]", "2026-10-12T01:10:00Z", body(7001, "<!-- aayudha:ready-guard --> still valid")),
			ev("labeled", "founder", "2026-10-12T05:00:00Z", label("after:cursor"))},
		86: {ev("merged", "founder", "2026-10-12T02:20:05Z", ""), ev("commented", "founder", "2026-10-12T02:23:33Z", body(6116700000, "Live inspect acceptance after merge."))},
		91: {ev("labeled", "founder", "2026-10-12T02:31:00Z", label("type:story")), ev("labeled", "founder", "2026-10-12T02:31:00Z", label("effort:XS")),
			ev("labeled", "founder", "2026-10-12T02:31:25Z", label("identity:human-fallback")), ev("labeled", "founder", "2026-10-12T02:31:40Z", label("status:ready")),
			ev("unlabeled", "founder", "2026-10-12T03:04:47Z", label("status:ready")), ev("labeled", "founder", "2026-10-12T03:04:49Z", label("status:merged"))},
		92: {ev("commented", "founder", "2026-10-12T03:04:30Z", body(6116996259, "Solo-mode bypass: no second human (D2).")), ev("merged", "founder", "2026-10-12T03:04:44Z", "")},
		95: {ev("merged", "founder", "2026-10-12T02:57:33Z", "")},
		97: {ev("labeled", "founder", "2026-10-06T09:00:00Z", label("type:decision"))},
	}
	for n, es := range timelines {
		g.Static[fmt.Sprintf("/repos/o/r/issues/%d/timeline", n)] = jsonOf(t, "["+strings.Join(es, ",")+"]")
	}
	g.Static["/repos/o/r/actions/runs"] = jsonOf(t, `{"total_count":1,"workflow_runs":[{"id":38179325066,"path":".github/workflows/ci.yml","event":"workflow_dispatch","actor":`+user+
		`,"head_branch":"promote/rolling-20261011T2210Z","head_sha":"`+auditR1+`","conclusion":"success","created_at":"2026-10-11T22:10:05Z"}]}`)
	g.Static["/repos/o/r/actions/workflows/release.yml/runs"] = jsonOf(t, `{"total_count":1,"workflow_runs":[{"id":38175962428,"path":".github/workflows/release.yml","event":"push","actor":`+user+
		`,"head_branch":"v0.0.0-d4test.1","head_sha":"`+auditT1+`","conclusion":"failure","created_at":"2026-10-11T21:20:32Z"}]}`)
	g.Static["/repos/o/r/actions/workflows/aayudha-merge-gate.yml"] = jsonOf(t, `{"id":381719884,"path":".github/workflows/aayudha-merge-gate.yml","created_at":"2026-10-12T02:37:03Z"}`)
	g.Static["/repos/o/r/commits/"+auditR1] = jsonOf(t, `{"sha":"`+auditR1+`","commit":{"message":"Rolling snapshot: 20261011T2210Z\n\nSigned-off-by: Amp"}}`)
	g.Static["/repos/o/r/compare/main..."+auditT1] = jsonOf(t, `{"status":"diverged"}`)
	g.Static["/repos/o/r/commits"] = jsonOf(t, `[{"sha":"`+auditCode+`"}]`)
	g.Static["/repos/o/r/pulls/92/commits"] = jsonOf(t, `[{"sha":"aaaaaaa1","author":`+user+`,"commit":{"message":"feat: doctor\n\nAayudha-Checkpoint: 1/1"},"parents":[{"sha":"p"}]},`+
		`{"sha":"bbbbbbb2","author":`+user+`,"commit":{"message":"fix: by hand"},"parents":[{"sha":"p"}]}]`)
	g.Static["/repos/o/r/pulls/92/files"] = jsonOf(t, `[{"filename":"internal/verify/doctor.go"}]`)
	g.Static["/repos/o/r/pulls/92/reviews"] = jsonOf(t, `[]`)
	g.Static["/repos/o/r/issues/92/comments"] = jsonOf(t, `[]`)
	g.Files = map[string]string{
		"go.mod": "module example.com/x\n\ngo 1.25.0\n\nrequire (\n\tgithub.com/a/ok v1.0.0\n\tgithub.com/b/missing v0.1.0 // indirect\n\tgithub.com/c/copyleft v2.0.0\n)\n\nrequire github.com/d/old v1.2.0\n",
		"THIRD_PARTY_NOTICES.md": "| Component | Version | Licence | Use | Source |\n|---|---|---|---|---|\n| `github.com/a/ok` | v1.0.0 | MIT | x | y |\n" +
			"| `github.com/c/copyleft` | v2.0.0 | GPL-3.0 | x | y |\n| `github.com/d/old` | v1.1.0 | BSD-3-Clause | x | y |\n\n## Planned\n\n| Component | Use | Licence |\n|---|---|---|\n| `github.com/a/ok` | later | MIT |\n",
	}
	g.CheckRuns[auditCode] = []fake.CheckRun{{Name: "security", Status: "completed", Conclusion: "success"}}
	g.Issues[48] = &fake.Issue{ID: 48048, Number: 48, Title: "WP4.8 Merge gate + `inspect`", State: "open",
		Labels: []fake.Label{{Name: "type:story"}, {Name: "status:ready"}, {Name: "effort:M"}, {Name: "priority:P1"}, {Name: "priority:P0"}}}
	f.g = g
	f.env = map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	now := time.Date(2026, 10, 12, 4, 30, 0, 0, time.UTC)
	f.opts = Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: f.dir, Now: func() time.Time { return now }}
	f.reportPath = "audit/2026-10-12/audit-20261012T040000Z.md"
	return f
}

func (f *auditFixture) audit(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return run(t, f.env, f.opts, append([]string{"audit", "--repo", "o/r"}, args...)...)
}

// WP7.6 accept: the audit of the dogfood fixture matches the reviewed golden
// report, which classifies what the hand-run reconciliations did.
func TestAuditDogfoodFixture(t *testing.T) {
	f := newAuditFixture(t)
	code, out, errOut := f.audit(t, "--since", "2026-10-11", "--until", auditUntil)
	if code != 0 {
		t.Fatalf("audit: %d %s %s", code, out, errOut)
	}
	got := git(t, f.bare, "show", "ledger:"+f.reportPath) + "\n"
	golden := filepath.Join("testdata", "audit", "report.golden")
	if *update {
		_ = os.MkdirAll(filepath.Dir(golden), 0o755)
		_ = os.WriteFile(golden, []byte(got), 0o644)
	}
	want, _ := os.ReadFile(golden)
	// The config hash covers the embedded defaults, which other WPs extend; the
	// re-run test checks it is stable, so the golden comparison leaves it out.
	confHash := regexp.MustCompile("sha256:[0-9a-f]{64}")
	if confHash.ReplaceAllString(got, "sha256:…") != confHash.ReplaceAllString(string(want), "sha256:…") {
		t.Fatalf("audit report differs from %s (go test ./internal/cli -run TestAuditDogfoodFixture -update):\n%s", golden, got)
	}
	for _, line := range []string{
		"| 4473766681 | `refs/heads/dev` | founder | `8686868` | 2026-10-12T02:20:04Z | `required_status_checks` (protected-paths-codeowner-review) | **no** |",
		"#9 merged past failing rules other than the protected-paths check",
		"| 4471059806 | `refs/heads/ledger-probe` | (none) |",
		"accounted for by audit/2026-10-11/solo-mode-bypass-reconciliation.md",
		"deploy-key push (no actor listed, D1), identified by ledger commit",
		"agent-run promotion \"Rolling snapshot: 20261011T2210Z\" after gate run 38179325066",
		"release run 38175962428 refused it (failure)",
		"| #48 WP4.8 Merge gate + `inspect` | ready | #86 | 2026-10-12T02:20:05Z | **no** | merged without proof (pre-gate) |",
		"- Proof coverage: 1/2 delivered stories and bugs; merged without proof (pre-gate) 1; merged without proof 0",
		"| #97 Engine choice | yes | 35.0 | yes | no |",
		"- correction of #92: non-bot-commit, commit bbbbbbb by founder",
		"- agent PR #92 merged on never-layers L04",
		"`github.com/b/missing` v0.1.0 is not in THIRD_PARTY_NOTICES.md",
		"`github.com/c/copyleft` v2.0.0: licence \"GPL-3.0\" is not on the allow list",
		"`github.com/d/old` is v1.2.0 in go.mod but v1.1.0 in THIRD_PARTY_NOTICES.md",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("report lacks %q", line)
		}
	}
	if strings.Contains(got, "4470000000 |") || strings.Contains(got, "after:cursor") || strings.Contains(got, "#120") {
		t.Errorf("report holds events outside the window or after the cursor")
	}
}

// WP7.6 accept: a direct `gh` write outside `actions` is flagged, and a write
// an actions segment holds is not.
func TestAuditFlagsUnloggedWrite(t *testing.T) {
	f := newAuditFixture(t)
	if code, _, errOut := f.audit(t, "--since", auditSince, "--until", auditUntil); code != 0 {
		t.Fatalf("audit: %s", errOut)
	}
	got := git(t, f.bare, "show", "ledger:"+f.reportPath)
	writes := section(got, "## Writes not in any actions segment")
	if !strings.Contains(writes, "| #48 | founder | 2026-10-11T23:35:00Z | 2026-10-12T01:00:00Z | labeled effort:M ×1, labeled priority:P0 ×1, labeled priority:P1 ×1, labeled status:ready ×1, labeled type:story ×1, opened issue ×1 |") {
		t.Errorf("the direct label write on #48 is not flagged:\n%s", writes)
	}
	for _, logged := range []string{"labeled status:merged", "unlabeled status:ready", "| #92 | founder | 2026-10-12T02:33:30Z", "| github-actions[bot] |"} {
		if strings.Contains(writes, logged) {
			t.Errorf("unlogged writes list %q, which a segment or a workflow run records:\n%s", logged, writes)
		}
	}
	if !strings.Contains(writes, "Writes matched to a record: (the ledger commit itself) 1, `actions/2026-10-12/amp-20261012T030441Z.jsonl` 5.") ||
		!strings.Contains(writes, "Workflow and App writes: `github-actions[bot]` 1.") {
		t.Errorf("reconciled writes not counted:\n%s", writes)
	}
}

// WP7.6 accept: a solo-mode self-approval (WP0.1) and a human-fallback write
// (WP1.4) each appear in the report.
func TestAuditSoloBypassAndHumanFallback(t *testing.T) {
	f := newAuditFixture(t)
	if code, _, errOut := f.audit(t, "--since", auditSince, "--until", auditUntil); code != 0 {
		t.Fatalf("audit: %s", errOut)
	}
	got := git(t, f.bare, "show", "ledger:"+f.reportPath)
	for _, want := range []string{
		"| 4471759694 | `refs/heads/dev` | founder | `8888888` | 2026-10-11T21:23:37Z | `required_status_checks` (protected-paths-codeowner-review) | yes | #8 merged into dev past only the protected-paths check, after a \"Solo-mode bypass:\" comment; #8 (comment 6113923044, 2026-10-11T21:23:34Z) |",
		"| #91 | #92 | `9292929` | 2026-10-12T03:04:45Z | founder | `proof/91/" + auditHead + "/authoritative/ship.json` | <https://github.com/o/r/pull/92#issuecomment-6116996259> |",
		"| 2026-10-12T02:31:25Z | #91 | work item tagged identity:human-fallback | founder | issue timeline |",
		"| 2026-10-12T02:33:30Z | o/r | github.pull.create | human-fallback | `actions/2026-10-12/amp-20261012T030441Z.jsonl` |",
		"| ledger `", "| specs: Ready snapshot of #91 | Amp | ledger commit trailer |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	log := git(t, f.bare, "log", "-1", "--format=%B", "ledger", "--", f.reportPath)
	if !strings.Contains(log, "Aayudha-Identity: identity:human-fallback") {
		t.Errorf("report not written under the founder's identity tagged human-fallback (D1):\n%s", log)
	}
}

// WP7.6 accept: re-running audit with the inputs recorded in the report header
// produces a byte-identical report, after the ledger has moved on.
func TestAuditRerunByteIdentical(t *testing.T) {
	f := newAuditFixture(t)
	if code, _, errOut := f.audit(t, "--since", auditSince, "--until", auditUntil); code != 0 {
		t.Fatalf("audit: %s", errOut)
	}
	first := git(t, f.bare, "show", "ledger:"+f.reportPath) + "\n"
	if tip := git(t, f.bare, "rev-parse", "ledger"); tip == f.ledgerTip {
		t.Fatal("the report did not move the ledger")
	}
	var cmdline, replay string
	for _, l := range strings.Split(first, "\n") {
		switch {
		case strings.HasPrefix(l, "aayu audit --repo o/r --since "):
			cmdline = l
		case strings.HasPrefix(l, "aayu audit --repo o/r --replay "):
			replay = l
		}
	}
	snapshot := strings.TrimSuffix(f.reportPath, ".md") + ".inputs.json"
	if cmdline != "aayu audit --repo o/r --since "+auditSince+" --until "+auditUntil+" --ledger "+f.ledgerTip || replay != "aayu audit --repo o/r --replay "+snapshot {
		t.Fatalf("recorded commands %q, %q", cmdline, replay)
	}
	if !strings.Contains(first, "| GitHub inputs snapshot | `"+snapshot+"` (SHA-256 `") {
		t.Fatalf("report header lacks the snapshot:\n%s", first)
	}
	// Re-reading GitHub with the recorded inputs, after the ledger moved on, gives the same bytes.
	out := filepath.Join(t.TempDir(), "again.md")
	if code, _, errOut := run(t, f.env, f.opts, append([]string{"audit"}, append(strings.Fields(cmdline)[2:], "--dry-run", "--out", out)...)...); code != 0 {
		t.Fatalf("re-run: %s", errOut)
	}
	if again, _ := os.ReadFile(out); string(again) != first {
		t.Fatalf("re-run differs:\n--- first\n%s\n--- again\n%s", first, again)
	}
	code, stdout, _ := run(t, f.env, f.opts, append([]string{"audit"}, strings.Fields(cmdline)[2:]...)...)
	if code != 0 || !strings.Contains(stdout, "report unchanged on the ledger") {
		t.Fatalf("re-run without --dry-run: %d %s", code, stdout)
	}
	// GitHub changes afterwards: the solo-mode comment is edited and the rule suites age out. The
	// snapshot still regenerates the report byte for byte, without asking GitHub.
	f.g.Mu.Lock()
	f.g.Static["/repos/o/r/issues/8/timeline"] = jsonOf(t, `[{"event":"commented","user":{"login":"founder"},"created_at":"2026-10-11T21:23:34Z","id":6113923044,"body":"edited later"}]`)
	f.g.Static["/repos/o/r/rulesets/rule-suites"] = jsonOf(t, `[]`)
	f.g.Mu.Unlock()
	f.g.Reset()
	code, stdout, errOut := run(t, f.env, f.opts, append([]string{"audit"}, strings.Fields(replay)[2:]...)...)

	if code != 0 || stdout != first {
		t.Fatalf("replay: %d %s\n--- replayed\n%s", code, errOut, stdout)
	}
	for _, r := range f.g.Requests {
		if strings.Contains(r.Path, "rulesets") || strings.Contains(r.Path, "timeline") {
			t.Fatalf("replay asked GitHub: %s %s", r.Method, r.Path)
		}
	}
	if code, _, errOut := f.audit(t, "--replay", snapshot, "--since", auditSince); code != ExitUsage {
		t.Fatalf("replay with a window: %d %s", code, errOut)
	}
	if code, _, errOut := f.audit(t, "--replay", "audit/none.inputs.json"); code != ExitUsage {
		t.Fatalf("replay of a missing snapshot: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, f.env, f.opts, "audit", "--repo", "o/r", "--replay", snapshot, "--set", "decisions.sla_hours=8"); code != ExitBlocked || !strings.Contains(errOut, "configuration changed") {
		t.Fatalf("replay under another configuration: %d %s", code, errOut)
	}
	if code, _, errOut := f.audit(t, "--ledger", "0123456789abcdef0123456789abcdef01234567"); code != ExitUsage || !strings.Contains(errOut, "not on the ledger branch") {
		t.Fatalf("unknown ledger commit: %d %s", code, errOut)
	}
	// Without --since the window starts at the newest report's cursor.
	code, stdout, errOut = f.audit(t, "--until", "2026-10-12T04:20:00Z", "--dry-run")
	if code != 0 || !strings.Contains(stdout, "--since "+auditUntil+" --until 2026-10-12T04:20:00Z") {
		t.Fatalf("default window: %d %s %s", code, stdout, errOut)
	}
	for _, bad := range [][]string{{"--until", "yesterday"}, {"--since", "2026-10-13", "--until", auditUntil}, {"--since", "10/11"}} {
		if code, _, errOut := f.audit(t, bad...); code != ExitUsage {
			t.Errorf("audit %v: %d %s", bad, code, errOut)
		}
	}
	notCheckout := f.opts
	notCheckout.Dir = t.TempDir()
	if code, _, errOut := run(t, f.env, notCheckout, "audit", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "checkout") {
		t.Errorf("audit outside a checkout: %d %s", code, errOut)
	}
}

// audit --mark-merged moves a pre-gate story to status:merged with a comment
// naming its merge PR, through internal/actions, and refuses anything else
// before writing.
func TestAuditMarkMerged(t *testing.T) {
	f := newAuditFixture(t)
	code, _, errOut := f.audit(t, "--mark-merged", "48,91")
	if code != ExitBlocked || !strings.Contains(errOut, "#91: covered by proof/91/") || len(f.g.Writes()) != 0 {
		t.Fatalf("mark-merged with a covered story: %d %s %v", code, errOut, f.g.Writes())
	}
	code, out, errOut := f.audit(t, "--mark-merged", "48")
	if code != 0 || !strings.Contains(out, "#48: status:merged (merged without proof, pre-gate, in #86)") {
		t.Fatalf("mark-merged: %d %s %s", code, out, errOut)
	}
	var paths []string
	for _, w := range f.g.Writes() {
		paths = append(paths, w.Method+" "+w.Path)
	}
	want := []string{"DELETE /repos/o/r/issues/48/labels/status:ready", "POST /repos/o/r/issues/48/labels", "POST /repos/o/r/issues/48/comments"}
	if strings.Join(paths, "\n") != strings.Join(want, "\n") {
		t.Fatalf("writes:\n%s", strings.Join(paths, "\n"))
	}
	cm := f.g.Issues[48].Comments
	if len(cm) != 1 || !strings.Contains(cm[0].Body, PreGateMarker) || !strings.Contains(cm[0].Body, "merged in #86 (`8686868`, 2026-10-12T02:20:05Z)") ||
		!strings.Contains(cm[0].Body, "No authoritative pack exists") {
		t.Fatalf("comment: %+v", cm)
	}
	seg := git(t, f.bare, "log", "-1", "--format=%s", "ledger")
	if !strings.HasPrefix(seg, "actions: segment ") {
		t.Fatalf("the moves' actions segment is not on the ledger: %s", seg)
	}
	if code, _, errOut := f.audit(t, "--mark-merged", "4x"); code != ExitUsage {
		t.Fatalf("bad list: %d %s", code, errOut)
	}
}

// WP7.6 retro: findings become Feedback issues on this repository, never PRs,
// once each; the report records its inputs.
func TestRetroFilesFeedback(t *testing.T) {
	f := newAuditFixture(t)
	code, out, errOut := run(t, f.env, f.opts, "retro", "--repo", "o/r", "--since", auditSince, "--until", auditUntil)
	if code != 0 {
		t.Fatalf("retro: %d %s %s", code, out, errOut)
	}
	var creates []map[string]any
	for _, w := range f.g.Writes() {
		if strings.HasSuffix(w.Path, "/pulls") {
			t.Fatalf("retro opened a PR: %+v", w)
		}
		if w.Method == "POST" && w.Path == "/repos/o/r/issues" {
			creates = append(creates, w.Body)
		}
	}
	if len(creates) != 1 || creates[0]["title"] != "Feedback: retro: correction rate 1/1 above 20 %, 2026-10-11T00:00Z to 2026-10-12T04:00Z" {
		t.Fatalf("feedback issues filed: %+v\n%s", creates, out)
	}
	if !strings.Contains(fmt.Sprint(creates[0]["labels"]), "type:feedback") || !strings.Contains(fmt.Sprint(creates[0]["body"]), "### What happened or what you need") {
		t.Fatalf("feedback issue: %+v", creates[0])
	}
	if !strings.Contains(out, "#99: floor miss: coverage fail on #48") {
		t.Fatalf("already-filed finding not matched to #99:\n%s", out)
	}
	rep := git(t, f.bare, "show", "ledger:retro/2026-10-12/retro-20261012T040000Z.md") + "\n"
	f.g.Reset()
	code, replayed, errOut := run(t, f.env, f.opts, "retro", "--repo", "o/r", "--replay", "retro/2026-10-12/retro-20261012T040000Z.inputs.json")
	if code != 0 || replayed != rep || len(f.g.Writes()) != 0 {
		t.Fatalf("retro replay: %d %s %v\n%s", code, errOut, f.g.Writes(), replayed)
	}
	for _, want := range []string{"| #91 doctor names the floor each missing tool serves | #92 | XS | 1 | 1 | 1 | 0.42 | covered |",
		"| GitHub event cursor | until 2026-10-12T04:00:00Z |", "| Ledger commit | `" + f.ledgerTip + "` |",
		"| `actions/2026-10-12/amp-20261012T030441Z.jsonl` | `"} {
		if !strings.Contains(rep, want) {
			t.Errorf("retro report lacks %q:\n%s", want, rep)
		}
	}
}

// A GitHub error on any source fails the audit with exit 5 and writes no
// report: a partial reconciliation would read as a clean one.
func TestAuditGitHubErrors(t *testing.T) {
	f := newAuditFixture(t)
	for _, p := range []string{"/rulesets/rule-suites", "/rulesets/rule-suites/4471059806", "/pulls", "/issues", "/issues/48/timeline",
		"/actions/runs", "/actions/workflows/release.yml/runs", "/actions/workflows/aayudha-merge-gate.yml", "/commits/" + auditR1,
		"/compare/main..." + auditT1, "/commits", "/contents/go.mod", "/contents/THIRD_PARTY_NOTICES.md", "/commits/" + auditCode + "/check-runs",
		"/pulls/92/files"} {
		f.g.Mu.Lock()
		f.g.Fail, f.g.FailAfter = "GET /repos/o/r"+p, 0
		f.g.Mu.Unlock()
		if code, _, errOut := f.audit(t, "--since", auditSince, "--until", auditUntil); code != ExitExternal {
			t.Errorf("GET %s failing: exit %d %s", p, code, errOut)
		}
	}
	f.g.Mu.Lock()
	f.g.Fail = "GET /repos/o/r/pulls"
	f.g.Mu.Unlock()
	if code, _, errOut := run(t, f.env, f.opts, "retro", "--repo", "o/r", "--since", auditSince, "--until", auditUntil); code != ExitExternal {
		t.Errorf("retro with GitHub failing: exit %d %s", code, errOut)
	}
	f.g.Mu.Lock()
	f.g.Fail = "GET /repos/o/r/actions/workflows/aayudha-merge-gate.yml"
	f.g.Mu.Unlock()
	if code, _, errOut := f.audit(t, "--mark-merged", "48"); code != ExitExternal {
		t.Errorf("mark-merged with GitHub failing: exit %d %s", code, errOut)
	}
	if tip := git(t, f.bare, "rev-parse", "ledger"); tip != f.ledgerTip || len(f.g.Writes()) != 0 {
		t.Fatalf("a failed audit wrote: ledger %s, GitHub %v", tip, f.g.Writes())
	}
}

func section(doc, heading string) string {
	_, rest, _ := strings.Cut(doc, heading)
	if i := strings.Index(rest, "\n## "); i >= 0 {
		rest = rest[:i]
	}
	return rest
}
