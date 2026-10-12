package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// ladderFixture: the shipped thin slice (story #2 under proven Feature #1,
// which sits under a Goal), rolling and main at dev's first commit, and two
// squash commits on dev whose PRs serve story #2 and a story under an
// unproven Feature, plus a chore commit with no PR.
type ladderFixture struct {
	g          *fake.GitHub
	env        map[string]string
	opts       Options
	dir, bare  string
	goal       int
	other      int // the story under the unproven Feature, status:ready like a WP story merged by hand
	pending    int // a story that never merged
	release    int // includes #2 and #other
	wide       int // includes Feature #1, whose other stories never merged
	unready    int // no Version: fails Ready
	snapshot   int // Version is a snapshot, not vX.Y.Z
	again      int // a second Release of v0.1.0
	gateFails  int // ci.yml runs that fail their first attempt
	dispatched []string
}

func newLadderFixture(t *testing.T) *ladderFixture {
	t.Helper()
	g, env, opts, dir, bare, unproven, other := shippedFixture(t)
	f := &ladderFixture{g: g, env: env, dir: dir, bare: bare, other: other}
	goal := g.AddIssue("Grow enrolment", fixtureBody(t, "goal", nil), "type:goal", "status:in-progress")
	pending := g.AddIssue("Upload progress", fixtureBody(t, "story", nil), "type:story", "status:in-progress", "effort:S")
	g.Mu.Lock()
	g.Issues[1].Parent = goal.Number
	g.Issues[other].Labels = []fake.Label{{Name: "type:story"}, {Name: "status:ready"}, {Name: "effort:S"}}
	g.Issues[unproven].Subs = append(g.Issues[unproven].Subs, pending.Number)
	pending.Parent = unproven
	g.Mu.Unlock()
	f.goal, f.pending = goal.Number, pending.Number
	rel := func(title, links string) int {
		return g.AddIssue(title, fixtureBody(t, "release", func(m map[string]string) {
			m["version"], m[workitems.FieldLinks] = "v0.1.0", links
		}), "type:release", "status:planned").Number
	}
	f.release = rel("Eligibility to early adopters", fmt.Sprintf("includes #2\nincludes #%d", other))
	f.wide = rel("Everything", fmt.Sprintf("includes #%d", unproven))
	f.again = rel("Eligibility again", "includes #2")
	f.unready = g.AddIssue("No version", fixtureBody(t, "release", func(m map[string]string) { delete(m, "version") }), "type:release").Number
	f.snapshot = g.AddIssue("Snapshot only", fixtureBody(t, "release", func(m map[string]string) { m[workitems.FieldLinks] = "includes #2" }), "type:release").Number

	first := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "push", "--quiet", "origin", first+":refs/heads/rolling", first+":refs/heads/main")
	f.commit(t, "feat: show empty list (#30)", 30, "Refs #2", "story/2-show-empty-list")
	f.commit(t, "feat: upload a CSV (#31)", 31, fmt.Sprintf("Refs #%d", other), "feat/upload")
	f.commit(t, "chore: tidy", 0, "", "")

	g.Mu.Lock()
	g.Workflows = []string{"ci.yml", "evals.yml", "release.yml"}
	g.OnDispatch = func(wf, ref string, inputs map[string]any) fake.WorkflowRun {
		f.dispatched = append(f.dispatched, fmt.Sprintf("%s@%s %v", wf, strings.SplitN(ref, "-", 2)[0], inputs))
		sha := git(t, bare, "rev-parse", "refs/heads/"+ref)
		run := fake.WorkflowRun{HeadSHA: sha, HeadBranch: ref, Status: "completed", Conclusion: "success", Attempt: 1}
		if wf == "ci.yml" && f.gateFails > 0 {
			f.gateFails--
			run.Conclusion = "failure"
		}
		if inputs["gate"] == "stable" { // the tag push of this commit will start release.yml, whose first attempt fails
			g.Runs = append(g.Runs, fake.WorkflowRun{ID: 7001, Path: ".github/workflows/release.yml", HeadSHA: sha, Event: "push", Status: "completed", Conclusion: "failure", Attempt: 1})
			g.Releases = map[string]*fake.Release{"v0.1.0": {ID: 51, TagName: "v0.1.0", Body: "goreleaser changelog",
				Assets: []map[string]any{{"name": "checksums.txt", "digest": "sha256:" + strings.Repeat("ab", 32), "size": 412}}}}
		}
		return run
	}
	g.OnRerun = func(r *fake.WorkflowRun) {
		r.Attempt++
		if f.gateFails > 0 && strings.HasSuffix(r.Path, "/ci.yml") {
			f.gateFails--
			return
		}
		r.Conclusion = "success"
	}
	g.Mu.Unlock()
	f.env = env
	f.opts = opts
	f.opts.RunPoll = time.Millisecond
	return f
}

// commit adds a commit to dev on the remote; with a PR number, GitHub reports
// that merged PR (into dev, with body) for it.
func (f *ladderFixture) commit(t *testing.T, msg string, pr int, body, head string) string {
	t.Helper()
	git(t, f.dir, "commit", "--quiet", "--allow-empty", "-m", msg)
	git(t, f.dir, "push", "--quiet", "origin", "HEAD:refs/heads/dev")
	sha := git(t, f.dir, "rev-parse", "HEAD")
	pulls := []any{}
	if pr != 0 {
		pulls = append(pulls, map[string]any{"number": pr, "body": body, "merged_at": "2026-10-12T08:00:00Z",
			"base": map[string]any{"ref": "dev"}, "head": map[string]any{"ref": head}})
	}
	f.g.Mu.Lock()
	if f.g.Static == nil {
		f.g.Static = map[string]any{}
	}
	f.g.Static["/repos/o/r/commits/"+sha+"/pulls"] = pulls
	f.g.Mu.Unlock()
	return sha
}

func (f *ladderFixture) run(t *testing.T, at time.Time, env map[string]string, args ...string) (int, Promotion, string) {
	t.Helper()
	opts := f.opts
	opts.Now = func() time.Time { return at }
	code, out, errOut := run(t, env, opts, append([]string{"release", "--repo", "o/r", "--json"}, args...)...)
	var p Promotion
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&p)
	return code, p, errOut
}

// WP7.8 accept (fake GitHub, local remote): the promotion ladder of a Release
// issue drives CONTRIBUTING's procedure and refuses what it must; a rolling
// snapshot stamps the stories the range shipped `released`, publishes notes
// that cite only proven Features, records the artifact identity and shipping
// evidence, and makes the Goal above them due for measure; a second snapshot
// the same day is rolling-<date>.2; main gets `Release: vX.Y.Z` and an
// annotated tag and closes the Release.
func TestReleaseLadder(t *testing.T) {
	f := newLadderFixture(t)
	day := time.Date(2026, 10, 12, 9, 41, 0, 0, time.UTC)
	rel := strconv.Itoa(f.release)
	rolling0 := git(t, f.bare, "rev-parse", "refs/heads/rolling")

	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "PATH": f.env["PATH"]}
	for _, tt := range []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{"agent lane", agent, []string{rel, "--to", "rolling"}, "refused for agents"},
		{"no confirmation off a terminal", f.env, []string{rel, "--to", "rolling"}, "release not confirmed"},
		{"main before rolling", f.env, []string{rel, "--to", "main", "--yes"}, "has not been promoted to rolling yet"},
		{"includes unmerged work", f.env, []string{strconv.Itoa(f.wide), "--to", "rolling", "--yes"}, fmt.Sprintf("#%d (status in-progress) is not in rolling..dev", f.pending)},
		{"not a Release", f.env, []string{"2", "--to", "rolling", "--yes"}, `is a "story", not a Release`},
		{"fails Ready", f.env, []string{strconv.Itoa(f.unready), "--to", "rolling", "--yes"}, "does not pass Ready: version"},
		{"main needs vX.Y.Z", f.env, []string{strconv.Itoa(f.snapshot), "--to", "main", "--yes"}, `Version "rolling-2026-10-20": promoting to main needs vX.Y.Z`},
		{"no gate workflow", f.env, []string{rel, "--to", "rolling", "--yes"}, "no ci.yml workflow to gate the candidate"},
		{"outside a checkout", f.env, []string{rel, "--to", "rolling", "--yes"}, "release needs a checkout"},
	} {
		saved := f.opts
		switch tt.name {
		case "no gate workflow":
			f.g.Mu.Lock()
			f.g.Workflows = []string{"evals.yml"}
			f.g.Mu.Unlock()
		case "outside a checkout":
			f.opts.Dir = t.TempDir()
		}
		code, _, errOut := f.run(t, day, tt.env, tt.args...)
		f.opts = saved
		f.g.Mu.Lock()
		f.g.Workflows = []string{"ci.yml", "evals.yml", "release.yml"}
		f.g.Mu.Unlock()
		if code == 0 || !strings.Contains(errOut, tt.want) {
			t.Fatalf("%s: exit %d %s, want %q", tt.name, code, errOut, tt.want)
		}
	}
	code, plan, errOut := f.run(t, day, f.env, rel, "--to", "rolling", "--dry-run")
	if code != 0 || !plan.DryRun || plan.Tag != "rolling-2026-10-12" || fmt.Sprint(plan.Stories) != fmt.Sprintf("[2 %d]", f.other) || len(f.dispatched) != 0 {
		t.Fatalf("dry run: %d %+v %s", code, plan, errOut)
	}
	if got := git(t, f.bare, "rev-parse", "refs/heads/rolling"); got != rolling0 {
		t.Fatal("the dry run moved rolling")
	}

	// A gate that still fails after one re-run of its failed jobs stops the promotion: nothing lands.
	f.gateFails = 2
	if code, _, errOut := f.run(t, day, f.env, rel, "--to", "rolling", "--yes"); code != ExitBlocked || !strings.Contains(errOut, "the rolling gate failed") {
		t.Fatalf("failing gate: %d %s", code, errOut)
	}
	if git(t, f.bare, "rev-parse", "refs/heads/rolling") != rolling0 || git(t, f.bare, "tag", "--list") != "" || git(t, f.bare, "branch", "--list", "promote/*") != "" {
		t.Fatal("a failed gate left rolling moved, a tag, or the candidate branch")
	}
	if l := f.g.LabelNames(2); !contains(l, "status:merged") {
		t.Fatalf("a failed gate stamped story #2: %v", l)
	}
	f.dispatched = nil

	f.gateFails = 1 // the first gate attempt fails (a transient signing timeout); release re-runs its failed jobs once
	code, p, errOut := f.run(t, day, f.env, rel, "--to", "rolling", "--yes")
	if code != 0 {
		t.Fatalf("release to rolling: %d %s", code, errOut)
	}
	tip := git(t, f.bare, "rev-parse", "refs/heads/rolling")
	if p.Commit != tip || git(t, f.bare, "rev-parse", "rolling-2026-10-12") != tip || git(t, f.bare, "cat-file", "-t", "rolling-2026-10-12") != "commit" {
		t.Fatalf("rolling %s, tag %s, promotion %+v", tip, git(t, f.bare, "rev-parse", "rolling-2026-10-12"), p)
	}
	if msg := git(t, f.bare, "log", "-1", "--format=%s%n%b", tip); !strings.HasPrefix(msg, "Rolling snapshot: 20261012T0941Z\n") || !strings.Contains(msg, "Signed-off-by:") {
		t.Fatalf("snapshot commit message:\n%s", msg)
	}
	if parents := git(t, f.bare, "log", "-1", "--format=%P", tip); parents != rolling0+" "+git(t, f.bare, "rev-parse", "refs/heads/dev") {
		t.Fatalf("snapshot parents %s", parents)
	}
	if out := git(t, f.bare, "branch", "--list", "promote/*"); out != "" {
		t.Fatalf("candidate branch left on the remote: %s", out)
	}
	if fmt.Sprint(f.dispatched) != "[evals.yml@dev map[] ci.yml@promote/rolling map[gate:rolling]]" || p.Gate.Attempts != 2 || p.Gate.Conclusion != "success" {
		t.Fatalf("dispatches %v, gate %+v", f.dispatched, p.Gate)
	}
	for _, s := range []int{2, f.other} {
		if l := f.g.LabelNames(s); !contains(l, "status:released") || contains(l, "status:merged") || contains(l, "status:ready") {
			t.Fatalf("story #%d labels %v", s, l)
		}
	}
	if !contains(f.g.LabelNames(f.pending), "status:in-progress") {
		t.Fatalf("an unshipped story was stamped: %v", f.g.LabelNames(f.pending))
	}
	notes := git(t, f.bare, "show", "ledger:tell/release/rolling-2026-10-12.md")
	if !strings.Contains(notes, "### Eligibility questionnaire ([#1](https://github.com/o/r/issues/1?utm_campaign=rolling-2026-10-12&utm_medium=release&utm_source=r))") ||
		!strings.Contains(notes, "- Show empty list") || !strings.Contains(notes, "1 more change(s) shipped under Features without authoritative packs") || strings.Contains(notes, "Upload a CSV") {
		t.Fatalf("release notes:\n%s", notes)
	}
	var rec Promotion
	_ = json.Unmarshal([]byte(git(t, f.bare, "show", fmt.Sprintf("ledger:releases/%d/rolling.json", f.release))), &rec)
	if rec.Commit != tip || rec.Tree == "" || rec.Gate == nil || rec.Notes != "tell/release/rolling-2026-10-12.md" || rec.PromotedBy != "founder" {
		t.Fatalf("promotion record: %+v", rec)
	}
	var due MeasureDue
	_ = json.Unmarshal([]byte(git(t, f.bare, "show", fmt.Sprintf("ledger:measure/due/%d.json", f.goal))), &due)
	if due.Type != "goal" || due.Due != "2026-11-09" || fmt.Sprint(due.Stories) != "[2]" || due.Tag != "rolling-2026-10-12" {
		t.Fatalf("measure due: %+v", due)
	}
	f.g.Mu.Lock()
	body := f.g.Issues[f.release].Body
	f.g.Mu.Unlock()
	for _, want := range []string{"### Shipping evidence", "**dev → rolling** on 2026-10-12T09:41:00Z by @founder", "commit `" + tip + "`",
		"ci.yml run", "success (attempts 2)", fmt.Sprintf("Stories released: #2, #%d", f.other), fmt.Sprintf("Measure due: #%d (goal) after 2026-11-09", f.goal)} {
		if !strings.Contains(body, want) {
			t.Fatalf("Shipping evidence lacks %q:\n%s", want, body)
		}
	}
	if !contains(f.g.LabelNames(f.release), "status:released") {
		t.Fatalf("Release labels %v", f.g.LabelNames(f.release))
	}
	if code, out, _ := run(t, f.env, Options{Dir: f.dir}, "actions", "grep", "git.push"); code != 0 || strings.Count(out, `"refspec"`) != 6 {
		t.Fatalf("git pushes in the actions log (the failed gate: candidate and its deletion; then candidate, rolling, tag, deletion): %s", out)
	}

	// A second snapshot the same day.
	if code, _, errOut := f.run(t, day.Add(time.Hour), f.env, rel, "--to", "rolling", "--yes"); code != ExitBlocked || !strings.Contains(errOut, "nothing to promote") {
		t.Fatalf("empty range: %d %s", code, errOut)
	}
	f.commit(t, "fix: empty list copy (#32)", 32, "Refs #2", "story/2-copy")
	opts := f.opts
	opts.Now = func() time.Time { return day.Add(time.Hour) }
	code, out, errOut := run(t, f.env, opts, "release", rel, "--to", "rolling", "--yes", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, `to rolling as "Rolling snapshot: 20261012T1041Z" = `) || !strings.Contains(out, ", tag rolling-2026-10-12.2 (candidate promote/rolling-20261012T1041Z)") ||
		!strings.Contains(out, "  stories released: none\n") || !strings.Contains(out, fmt.Sprintf("  already released: #%d\n", f.other)) {
		t.Fatalf("second snapshot: %d\n%s%s", code, out, errOut)
	}

	// Main: Release: v0.1.0, an annotated tag, and the Release closed.
	code, p, errOut = f.run(t, day.Add(2*time.Hour), f.env, rel, "--to", "main", "--yes")
	if code != 0 || p.Message != "Release: v0.1.0" || p.Gate == nil || !strings.Contains(fmt.Sprint(f.dispatched), "ci.yml@promote/main map[gate:stable]") {
		t.Fatalf("release to main: %d %+v %s", code, p, errOut)
	}
	if p.ReleaseRun == nil || p.ReleaseRun.Conclusion != "success" || p.ReleaseRun.Attempts != 2 ||
		fmt.Sprint(p.Artifacts) != "[{checksums.txt sha256:"+strings.Repeat("ab", 32)+" 412}]" {
		t.Fatalf("release run and artifact identity: %+v %+v", p.ReleaseRun, p.Artifacts)
	}
	f.g.Mu.Lock()
	ghNotes := f.g.Releases["v0.1.0"].Body
	f.g.Mu.Unlock()
	if !strings.HasPrefix(ghNotes, "goreleaser changelog\n\n"+tellMarker+"\n## v0.1.0\n") {
		t.Fatalf("GitHub Release notes:\n%s", ghNotes)
	}
	if mainTip := git(t, f.bare, "rev-parse", "refs/heads/main"); mainTip != p.Commit || git(t, f.bare, "cat-file", "-t", "v0.1.0") != "tag" || git(t, f.bare, "rev-parse", "v0.1.0^{commit}") != mainTip {
		t.Fatalf("main %s, v0.1.0 %s", mainTip, git(t, f.bare, "rev-parse", "v0.1.0^{commit}"))
	}
	f.g.Mu.Lock()
	state, body := f.g.Issues[f.release].State, f.g.Issues[f.release].Body
	f.g.Mu.Unlock()
	if state != "closed" || !strings.Contains(body, "**dev → rolling**") || !strings.Contains(body, "**rolling → main**") {
		t.Fatalf("Release after main: %s\n%s", state, body)
	}
	if code, _, errOut := f.run(t, day.Add(3*time.Hour), f.env, rel, "--to", "main", "--yes"); code == 0 || !strings.Contains(errOut, "closed") {
		t.Fatalf("a second main rung: %d %s", code, errOut)
	}
	if code, _, errOut := f.run(t, day.Add(3*time.Hour), f.env, strconv.Itoa(f.again), "--to", "main", "--yes"); code != ExitBlocked || !strings.Contains(errOut, "tag v0.1.0 already exists") {
		t.Fatalf("a version that already shipped: %d %s", code, errOut)
	}
}

// A GitHub failure before the gated commit lands promotes nothing: rolling and
// the tags stay as they were, and the candidate branch is deleted. One after
// it lands (stamping, notes, records) still exits 5, so the promotion is never
// reported done with its bookkeeping missing.
func TestReleaseFailsClosed(t *testing.T) {
	for _, tt := range []struct {
		fail   string
		landed bool
	}{
		{"GET /repos/o/r/issues/{release}", false},       // the Release
		{"GET /repos/o/r/issues/2", false},               // a listed story
		{"GET /repos/o/r/commits/{commit}/pulls", false}, // what the range ships
		{"GET /repos/o/r/actions/workflows", false},      // which workflows exist
		{"POST /repos/o/r/actions/workflows/evals.yml/dispatches", false},
		{"GET /repos/o/r/actions/runs/901", false}, // waiting for the evals
		{"POST /repos/o/r/actions/workflows/ci.yml/dispatches", false},
		{"GET /repos/o/r/actions/runs/902", false},        // waiting for the gate
		{"POST /repos/o/r/issues/2/labels", true},         // stamping
		{"GET /repos/o/r/issues", true},                   // the Features, for the notes
		{"GET /repos/o/r/issues/2/parent", true},          // a story's Feature
		{"GET /repos/o/r/issues/1/parent", true},          // the Goal above it
		{"GET /repos/o/r/releases/tags/{tag}", true},      // the tag's GitHub Release
		{"POST /repos/o/r/issues/{release}/labels", true}, // the Release's status
		{"PATCH /repos/o/r/issues/{release}", true},       // its Shipping evidence
	} {
		f := newLadderFixture(t)
		rolling0 := git(t, f.bare, "rev-parse", "refs/heads/rolling")
		fail := strings.NewReplacer("{release}", strconv.Itoa(f.release), "{commit}", git(t, f.dir, "rev-parse", "HEAD~2"), "{tag}", "rolling-2026-10-12").Replace(tt.fail)
		f.g.Mu.Lock()
		f.g.Fail = fail
		f.g.Mu.Unlock()
		code, _, errOut := f.run(t, time.Date(2026, 10, 12, 9, 41, 0, 0, time.UTC), f.env, strconv.Itoa(f.release), "--to", "rolling", "--yes")
		moved := git(t, f.bare, "rev-parse", "refs/heads/rolling") != rolling0
		if code != ExitExternal || moved != tt.landed || git(t, f.bare, "branch", "--list", "promote/*") != "" {
			t.Errorf("%s: exit %d, rolling moved %t, want exit 5 and moved %t: %s", fail, code, moved, tt.landed, errOut)
		}
		if !tt.landed && (git(t, f.bare, "tag", "--list") != "" || !contains(f.g.LabelNames(2), "status:merged")) {
			t.Errorf("%s: a failure before landing tagged or stamped", fail)
		}
		if tt.landed && !strings.Contains(errOut, "tagged rolling-2026-10-12, but the bookkeeping after landing failed") {
			t.Errorf("%s: a failure after landing must say what landed: %s", fail, errOut)
		}
	}
}
