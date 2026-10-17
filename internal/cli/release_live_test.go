package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Stand-ins for aayudha's workflows on the scratch repository: the gate takes
// the gate input, and the tag workflow publishes a GitHub Release with one
// asset, as goreleaser does.
var liveWorkflows = map[string]string{
	"ci.yml": `name: ci
on:
  workflow_dispatch:
    inputs:
      gate:
        type: choice
        options: [none, rolling, stable]
        default: none
permissions:
  contents: read
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - env:
          GATE: ${{ inputs.gate }}
        run: echo "promotion gate $GATE on $GITHUB_SHA"
`,
	"evals.yml": `name: evals
on:
  workflow_dispatch:
permissions:
  contents: read
jobs:
  tier2:
    runs-on: ubuntu-latest
    steps:
      - run: echo "tier-2 evals on $GITHUB_SHA"
`,
	"release.yml": `name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - env:
          GH_TOKEN: ${{ github.token }}
        run: |
          echo "$GITHUB_SHA  scratch" > checksums.txt
          gh release create "$GITHUB_REF_NAME" checksums.txt --repo "$GITHUB_REPOSITORY" --notes "Built by the scratch release workflow."
`,
}

// liveAPI is test scaffolding (not a verb write), so it calls the API
// directly. It retries for a minute while GitHub reports the repository
// locked by a visibility change or a PR not yet mergeable.
func liveAPI(t *testing.T, token, method, path string, body any) []byte {
	t.Helper()
	for i := 0; ; i++ {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, "https://api.github.com"+path, rd)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		transient := resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusMethodNotAllowed ||
			(resp.StatusCode == http.StatusForbidden && bytes.Contains(b, []byte("locked")))
		if transient && i < 12 {
			time.Sleep(5 * time.Second)
			continue
		}
		if resp.StatusCode >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
		}
		return b
	}
}

// TestReleaseLive runs the ladder against GitHub (AAYU_LIVE=1): an
// aayu-scratch-* repository (created, made public, given aayudha's promotion
// rulesets, and deleted with AAYU_LIVE_ADMIN_TOKEN when set) with dev, rolling, main, and
// ledger, stand-in workflows, a Goal > Feature > Story tree, the story's PR
// squash-merged into dev, and a Release that includes it. `release --to
// rolling` under the human token runs the evals and the gate on the
// candidate, lands the snapshot and its tag through the admin-only rulesets,
// and stamps the story released; `release --to main` tags v0.1.0, waits for
// the tag's release run, sets tell's notes on the GitHub Release, and closes
// the Release.
func TestReleaseLive(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("live test: set AAYU_LIVE=1")
	}
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin)
	// Runs before scratch.Repo's delete: wait out a visibility change still in progress, which makes a delete 409.
	t.Cleanup(func() {
		liveAPI(t, admin, "PATCH", "/repos/"+repo, map[string]any{"description": "aayudha live test scratch repository; deleted by the test"})
	})
	api := func(token, method, path string, body any) map[string]any {
		var out map[string]any
		_ = json.Unmarshal(liveAPI(t, token, method, path, body), &out)
		return out
	}

	// Branches, workflows, and the ledger, pushed with the admin token.
	dir := filepath.Join(t.TempDir(), "clone")
	g := &ledger.Writer{Dir: dir, Token: admin}
	gitDo := func(args ...string) string {
		t.Helper()
		out, err := g.Output(t.Context(), dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	if err := g.Exec(t.Context(), filepath.Dir(dir), "clone", "--quiet", "https://github.com/"+repo+".git", dir); err != nil {
		t.Fatal(err)
	}
	gitDo("config", "user.name", "aayudha live test")
	gitDo("config", "user.email", "aayudha-live@users.noreply.github.com")
	for name, body := range liveWorkflows {
		p := filepath.Join(dir, ".github", "workflows", name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	gitDo("add", ".")
	gitDo("commit", "--quiet", "-m", "ci: stand-in promotion workflows")
	gitDo("push", "--quiet", "origin", "HEAD:refs/heads/main", "HEAD:refs/heads/rolling", "HEAD:refs/heads/dev")
	gitDo("checkout", "--quiet", "--orphan", "ledger")
	gitDo("rm", "-r", "--quiet", "--cached", ".")
	_ = os.WriteFile(filepath.Join(dir, "LEDGER.md"), []byte("ledger\n"), 0o644)
	gitDo("add", "LEDGER.md")
	gitDo("commit", "--quiet", "-m", "ledger")
	gitDo("push", "--quiet", "origin", "ledger")
	gitDo("checkout", "--quiet", "-f", "-B", "dev", "origin/dev")
	gitDo("clean", "-fdq")
	// Rulesets on a personal account's private repository need GitHub Pro, so the scratch repository (synthetic
	// fixtures only) is public for its few minutes.
	api(admin, "PATCH", "/repos/"+repo, map[string]any{"default_branch": "dev", "private": false})
	// aayudha's promotion rulesets (Appendix B D3, D4): only the admin role moves rolling and main or makes v* tags.
	admins := []map[string]any{{"actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always"}}
	restrict := []map[string]any{{"type": "deletion"}, {"type": "non_fast_forward"}, {"type": "update"}}
	api(admin, "POST", "/repos/"+repo+"/rulesets", map[string]any{"name": "rolling-main-promotion", "target": "branch", "enforcement": "active",
		"conditions": map[string]any{"ref_name": map[string]any{"include": []string{"refs/heads/rolling", "refs/heads/main"}, "exclude": []string{}}},
		"rules":      restrict, "bypass_actors": admins})
	api(admin, "POST", "/repos/"+repo+"/rulesets", map[string]any{"name": "release-tags", "target": "tag", "enforcement": "active",
		"conditions": map[string]any{"ref_name": map[string]any{"include": []string{"refs/tags/v*"}, "exclude": []string{}}},
		"rules":      append([]map[string]any{{"type": "creation"}}, restrict...), "bypass_actors": admins})

	// Goal > Feature > Story, and the Release.
	issue := func(title, typ string, labels []string, f func(map[string]string)) (int, int64) {
		is := api(tok, "POST", "/repos/"+repo+"/issues", map[string]any{"title": title, "body": fixtureBody(t, typ, f), "labels": append([]string{"type:" + typ}, labels...)})
		return int(is["number"].(float64)), int64(is["id"].(float64))
	}
	goal, _ := issue("Grow enrolment", "goal", []string{"status:in-progress"}, nil)
	feature, featureID := issue("Eligibility questionnaire", "feature", []string{"status:in-progress"}, nil)
	story, storyID := issue("Show empty list", "story", []string{"status:ready", "effort:S"}, nil)
	api(tok, "POST", fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, goal), map[string]any{"sub_issue_id": featureID})
	api(tok, "POST", fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, feature), map[string]any{"sub_issue_id": storyID})
	rel, _ := issue("Eligibility to early adopters", "release", []string{"status:planned"}, func(m map[string]string) {
		m["version"], m[workitems.FieldLinks] = "v0.1.0", fmt.Sprintf("includes #%d", story)
	})

	// The story's PR, squash-merged into dev.
	gitDo("checkout", "--quiet", "-b", fmt.Sprintf("story/%d-show-empty-list", story))
	_ = os.WriteFile(filepath.Join(dir, "empty.txt"), []byte("Your list is empty\n"), 0o644)
	gitDo("add", "empty.txt")
	gitDo("commit", "--quiet", "-m", "feat: show empty list")
	gitDo("push", "--quiet", "origin", "HEAD")
	pr := api(tok, "POST", "/repos/"+repo+"/pulls", map[string]any{"title": "feat: show empty list", "head": fmt.Sprintf("story/%d-show-empty-list", story),
		"base": "dev", "body": fmt.Sprintf("Refs #%d", story)})
	prN := int(pr["number"].(float64))
	time.Sleep(3 * time.Second)
	merged := api(tok, "PUT", fmt.Sprintf("/repos/%s/pulls/%d/merge", repo, prN), map[string]any{"merge_method": "squash"})
	gitDo("checkout", "--quiet", "dev")
	// GitHub links a fresh squash commit to its merged PR after a short delay, and its git side can serve dev
	// without the commit for a while after the API merged it; release reads both.
	for i := 0; ; i++ {
		var pulls []struct {
			MergedAt *string `json:"merged_at"`
		}
		linked := json.Unmarshal(liveAPI(t, tok, "GET", fmt.Sprintf("/repos/%s/commits/%s/pulls", repo, merged["sha"]), nil), &pulls) == nil && len(pulls) > 0 && pulls[0].MergedAt != nil
		if linked && strings.HasPrefix(gitDo("ls-remote", "origin", "refs/heads/dev"), merged["sha"].(string)) {
			break
		}
		if i == 30 {
			t.Fatalf("the squash merge %v of PR #%d is not on dev or not linked to the PR after a minute", merged["sha"], prN)
		}
		time.Sleep(2 * time.Second)
	}

	env := map[string]string{"AAYU_GITHUB_TOKEN": tok, "PATH": os.Getenv("PATH")}
	opts := Options{Dir: dir, RunPoll: 5 * time.Second, IsTerminal: stdinTTY} // a person at a terminal: --to main needs no request comment
	release := func(to string) (Promotion, string) {
		t.Helper()
		var (
			code     int
			out      string
			errOut   string
			notInDev = fmt.Sprintf("#%d (status ready) is not in rolling..dev", story)
		)
		// Even after the wait above, CI has twice seen release's own reads of the range miss the squash merge
		// (runs 38761858117 and 38763856598) while the test's reads had it. That refusal comes before release
		// writes anything, so retry it for a minute, logging what GitHub serves each time.
		for i := 0; ; i++ {
			start := time.Now()
			code, out, errOut = run(t, env, opts, "release", strconv.Itoa(rel), "--to", to, "--yes", "--repo", repo, "--json")
			t.Logf("release --to %s: exit %d in %v\n%s", to, code, time.Since(start).Round(time.Second), errOut)
			if code != ExitBlocked || !strings.Contains(errOut, notInDev) || i == 6 {
				break
			}
			var pulls []struct {
				Number   int     `json:"number"`
				MergedAt *string `json:"merged_at"`
				Base     struct {
					Ref string `json:"ref"`
				} `json:"base"`
			}
			_ = json.Unmarshal(liveAPI(t, tok, "GET", fmt.Sprintf("/repos/%s/commits/%s/pulls", repo, merged["sha"]), nil), &pulls)
			t.Logf("squash merge %v; release fetched origin/dev %s; rolling..dev %q; pulls with the merge %+v", merged["sha"],
				gitDo("rev-parse", "origin/dev"), gitDo("rev-list", "--no-merges", "origin/rolling..origin/dev"), pulls)
			time.Sleep(10 * time.Second)
		}
		var p Promotion
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&p)
		if code != 0 {
			t.Fatalf("release --to %s: exit %d: %s", to, code, errOut)
		}
		return p, errOut
	}
	issueOf := func(n int) map[string]any { return api(tok, "GET", fmt.Sprintf("/repos/%s/issues/%d", repo, n), nil) }
	labels := func(n int) string {
		var ls []string
		for _, l := range issueOf(n)["labels"].([]any) {
			ls = append(ls, l.(map[string]any)["name"].(string))
		}
		return strings.Join(ls, ",")
	}

	p, _ := release("rolling")
	gitDo("fetch", "--quiet", "--tags", "--force", "origin")
	tag := "rolling-" + time.Now().UTC().Format(time.DateOnly)
	if p.Tag != tag || gitDo("rev-parse", "origin/rolling") != p.Commit || gitDo("rev-parse", tag) != p.Commit ||
		!strings.HasPrefix(gitDo("log", "-1", "--format=%s", p.Commit), "Rolling snapshot: ") {
		t.Fatalf("rolling: %+v", p)
	}
	if p.Evals == nil || p.Evals.Conclusion != "success" || p.Gate == nil || p.Gate.Conclusion != "success" || fmt.Sprint(p.Stories) != fmt.Sprintf("[%d]", story) {
		t.Fatalf("rolling rung: %+v", p)
	}
	if l := labels(story); !strings.Contains(l, "status:released") || strings.Contains(l, "status:ready") {
		t.Fatalf("story labels %s", l)
	}
	if b := issueOf(rel)["body"].(string); !strings.Contains(b, "**dev → rolling**") || !strings.Contains(b, p.Gate.URL) {
		t.Fatalf("Shipping evidence:\n%s", b)
	}
	if out := gitDo("ls-tree", "-r", "--name-only", "origin/ledger"); !strings.Contains(out, fmt.Sprintf("releases/%d/rolling.json", rel)) ||
		!strings.Contains(out, fmt.Sprintf("measure/due/%d/%s.json", goal, tag)) || !strings.Contains(out, "tell/release/"+tag+".md") {
		t.Fatalf("ledger:\n%s", out)
	}
	t.Logf("rolling: %s %s gate %s evals %s", p.Tag, short(p.Commit), p.Gate.URL, p.Evals.URL)

	p, _ = release("main")
	gitDo("fetch", "--quiet", "--tags", "--force", "origin")
	if gitDo("rev-parse", "origin/main") != p.Commit || gitDo("cat-file", "-t", "v0.1.0") != "tag" || p.ReleaseRun == nil || p.ReleaseRun.Conclusion != "success" {
		t.Fatalf("main rung: %+v", p)
	}
	if len(p.Artifacts) == 0 || p.Artifacts[len(p.Artifacts)-1].Name != "checksums.txt" {
		t.Fatalf("artifact identity: %+v", p.Artifacts)
	}
	ghRel := api(tok, "GET", "/repos/"+repo+"/releases/tags/v0.1.0", nil)
	if b, _ := ghRel["body"].(string); !strings.Contains(b, tellMarker) || !strings.Contains(b, "## v0.1.0") {
		t.Fatalf("GitHub Release notes:\n%s", b)
	}
	if is := issueOf(rel); is["state"] != "closed" || !strings.Contains(is["body"].(string), "**rolling → main**") {
		t.Fatalf("Release after main: %v", is["state"])
	}
	t.Logf("main: %s %s gate %s release %s artifacts %+v", p.Tag, short(p.Commit), p.Gate.URL, p.ReleaseRun.URL, p.Artifacts)
}
