package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

func uxCase(t *testing.T, id string) Case {
	t.Helper()
	all, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range all {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no case %s", id)
	return Case{}
}

// recorded copies a case's recorded output to a scratch directory.
func recorded(t *testing.T, c Case) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join(c.Dir, c.UX.Output))); err != nil {
		t.Fatal(err)
	}
	return dir
}

func edit(t *testing.T, path, old, new string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), old) {
		t.Fatalf("%s: %v, or no %q", path, err, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The UX floors (ux.go) catch each lapse they exist for, on a recorded run
// with one planted change; the recorded runs themselves pass (TestTier1).
func TestUXFloorsCatchLapses(t *testing.T) {
	for _, tc := range []struct {
		id, file, old, new, want string
	}{
		// research: references are the host's pages, never images or inventions; quotes are the data's words.
		{"research/orders-status", "stage.json", `"references": [`, `"references": [{"source": "mobbin", "kind": "screen", "id": "7c1d", "url": "https://cdn.bytescale.com/mobbin/7c1d2e3f-preview.png"},`, "points at an image"},
		{"research/orders-status", "stage.json", `"references": [`, `"references": [{"source": "mobbin", "kind": "screen", "id": "0000", "url": "https://mobbin.com/screens/00000000-0000-4000-8000-000000000000"},`, "not one the host gathered"},
		{"research/orders-status", "notes.md", "## What people did", "> I would pay extra for live tracking on a map. (P2)\n\n## What people did", "which the research data does not say"},
		{"research/orders-status", "notes.md", "## Job story", "## Job map", `no heading for the startup artefact "Job story"`},
		{"research/delivery-window", "notes.md", "**Questions.**", "### Research questions\n\n**Questions.**", `carries the startup artefact "Research questions"`},
		// mockup: token colours only, every state.
		{"mockup/orders-status", "empty.html", "color: var(--ay-color-text-label)", "color: #777777", "not design tokens"},
		{"mockup/delivery-window", "stage.json", `,
    {"id": "saved", "title": "Delivery window saved", "file": "saved.html"}`, "", `no state "saved"`},
		// prototype: off by default, every component mapped, token colours in code.
		{"prototype/orders-status", "flags.flagd.json", `"defaultVariant": "off"`, `"defaultVariant": "on"`, "on by default"},
		{"prototype/delivery-window", "src/delivery/WindowOption.figma.tsx", "node-id=41-3", "node-id=41-4", "no Code Connect mapping for Figma component(s) 41:3"},
		{"prototype/orders-status", "src/orders/OrdersMessages.tsx", `className="mb-section"`, `style={{ color: "#ff00aa" }} className="mb-section"`, "not a design token: colour #ff00aa"},
		// review: lane A against lane B's measurements.
		{"review/planted-preferences", "reply.json", `{"kind": "contrast", "heuristic": 8, "severity": 3`, `{"kind": "layout", "heuristic": 8, "severity": 3`, "missed the contrast defect"},
		{"review/clean-preferences", "reply.json", `{"kind": "copy", "heuristic": 10, "severity": 1`, `{"kind": "target-size", "heuristic": 10, "severity": 1`, ""},
		{"review/clean-preferences", "reply.json", `{"kind": "feedback", "heuristic": 1, "severity": 1`, `{"kind": "contrast", "heuristic": 1, "severity": 2`, "where lane B measured none"},
		{"review/planted-preferences", "reply.json", `"nielsen": [3, 0, 0, 0, 0, 0, 2, 3, 0, 1]`, `"nielsen": [3, 0, 0, 0, 0, 0, 1, 3, 0, 1]`, "severity 2 but nielsen[7] says 1"},
		{"review/planted-preferences", "reply.json", `{"persona": "screen-reader"`, `{"persona": "low-vision"`, "no walk as the screen-reader persona"},
		// polish: never remove a control; report every finding; tokens only.
		{"polish/orders-toolbar", "orders.html", `<button type="button" aria-label="Filter by status">`, `<span aria-label="Filter by status">`, "fix a control, never remove or hide it"},
		{"polish/orders-toolbar", "polish.md", "| labels (A, B: axe button-name)", "| names (A, B: axe button-name)", "does not report the labels finding"},
		{"polish/planted-preferences", "preferences.html", "p { color: #4f5a6b;", "p { color: #6b6b6b;", "not a design token: colour #6b6b6b"},
		// baseline: the named commit, checked out, on the target, nothing discarded or written.
		{"baseline/wrong-checkout", "commands.sh", "AAYU_ENV=staging aayu baseline --sha 77aa31c", "AAYU_ENV=staging aayu baseline", "does not baseline 77aa31c"},
		{"baseline/wrong-checkout", "commands.sh", "git worktree add --detach ../shop-v0.3.0 77aa31c", "git worktree add --detach ../shop-v0.3.0 HEAD", "no checkout of 77aa31c"},
		{"baseline/wrong-checkout", "commands.sh", "git fetch origin --tags", "git reset --hard", "discards local changes"},
		// A commit or target named once in a variable counts where the variable is used, and only with its value.
		{"baseline/wrong-checkout", "commands.sh", "git worktree add --detach ../shop-v0.3.0 77aa31c", "SHA=\"77aa31c\"\ngit worktree add --detach ../shop-v0.3.0 \"${SHA}\"", ""},
		{"baseline/wrong-checkout", "commands.sh", "AAYU_ENV=staging aayu baseline --sha 77aa31c", "SHA=77aa31c\nTARGET=staging\nAAYU_ENV=\"$TARGET\" aayu baseline --sha \"$SHA\"", ""},
		{"baseline/wrong-checkout", "commands.sh", "git worktree add --detach ../shop-v0.3.0 77aa31c", "SHA=5d1e0b2\ngit worktree add --detach ../shop-v0.3.0 \"$SHA\"", "no checkout of 77aa31c"},
		{"baseline/release-candidate", "commands.sh", "AAYU_ENV=preview aayu baseline", "TARGET=production\nAAYU_ENV=$TARGET aayu baseline", "targets production"},
		// Valid shell the floor reads as written (D-6.5h): git's -C option, a continued line, a defaulted
		// variable, and a mention of the command in a message before it runs.
		{"baseline/wrong-checkout", "commands.sh", "git worktree add --detach ../shop-v0.3.0 77aa31c", "git -C \"$PWD\" worktree add --detach \\\n  ../shop-v0.3.0 \\\n  77aa31c", ""},
		{"baseline/wrong-checkout", "commands.sh", "git worktree add --detach ../shop-v0.3.0 77aa31c", "SHA=\"${1:-77aa31c}\"\ngit worktree add --detach ../shop-v0.3.0 \"$SHA\"", ""},
		{"baseline/wrong-checkout", "commands.sh", "git fetch origin --tags", "echo \"aayu baseline will run for 77aa31c once it is checked out\"\ngit fetch origin --tags", ""},
		{"baseline/wrong-checkout", "commands.sh", "git fetch origin --tags", "git -C ../shop reset --hard", "discards local changes"},
		{"baseline/wrong-checkout", "commands.sh", "git worktree add --detach ../shop-v0.3.0 77aa31c", "SHA=\"${1:-5d1e0b2}\"\ngit worktree add --detach ../shop-v0.3.0 \"$SHA\"", "no checkout of 77aa31c"},
		{"baseline/release-candidate", "commands.sh", "AAYU_ENV=preview aayu baseline", "AAYU_ENV=production aayu baseline", "targets production"},
		{"baseline/release-candidate", "commands.sh", "kill %1", "gh pr comment 57 --body baselined", "writes to GitHub"},
	} {
		c := uxCase(t, tc.id)
		dir := recorded(t, c)
		edit(t, filepath.Join(dir, filepath.FromSlash(tc.file)), tc.old, tc.new)
		problems, _, err := uxChecks(context.Background(), root, aayudha.Assets, c, dir, false)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		got := strings.Join(problems, "\n")
		if tc.want == "" { // a cosmetic finding of a clean kind is allowed
			if got != "" {
				t.Errorf("%s %s: unexpected problems %s", tc.id, tc.file, got)
			}
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s %s: want %q in\n%s", tc.id, tc.file, tc.want, got)
		}
	}
}

// The a11y floor renders the polished page in Chromium and runs lane B: with
// the planted defects put back, it finds each of them (CI floors-web).
func TestUXA11yFloor(t *testing.T) {
	if !webFloor() {
		t.Skip("set AAYU_FLOOR_WEB=1 with packages/floors and Playwright's Chromium")
	}
	c := uxCase(t, "polish/planted-preferences")
	dir := recorded(t, c)
	planted, err := os.ReadFile(filepath.Join(c.Dir, "preferences.html"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "preferences.html"), planted, 0o644); err != nil {
		t.Fatal(err)
	}
	problems, skipped, err := uxChecks(context.Background(), root, aayudha.Assets, c, dir, true)
	if err != nil || skipped {
		t.Fatalf("%v, skipped %v", err, skipped)
	}
	got := strings.Join(problems, "\n")
	for _, k := range []string{"focus-visible", "contrast", "target-size", "not a design token: colour #838383"} {
		if !strings.Contains(got, k) {
			t.Errorf("a11y floor missed %s:\n%s", k, got)
		}
	}
}

func filesReply(t *testing.T, dir string, drop string) string {
	t.Helper()
	var r uxFiles
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if filepath.ToSlash(rel) == drop {
			return nil
		}
		b, err := os.ReadFile(p)
		r.Files = append(r.Files, struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}{filepath.ToSlash(rel), string(b)})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(r)
	return string(b)
}

// A design skill's run (tier 2): the skill replies with its stage files; a
// reply the floors refuse is sent back once with the findings, as `aayu
// wireframe` refusing sends a host back; the judge then sees the files and the
// floors that passed. A review run is lane A's own prompt on the recorded
// capture, with no revision.
func TestTier2UXRuns(t *testing.T) {
	c := uxCase(t, "wireframe/orders-status")
	c.Tier2 = &Tier2Spec{Rubric: "rubric.md"}
	dir := filepath.Join(c.Dir, c.UX.Output)
	noError := strings.Replace(filesReply(t, dir, ""), `{\"id\": \"error\", \"title\": \"Orders could not load\", \"file\": \"error.md\"}`, `{\"id\": \"failed\", \"title\": \"Orders could not load\", \"file\": \"error.md\"}`, 1)
	judge := models.Response{Text: `{"score": 5, "reason": "follows the research"}`}
	fake := &models.Fake{Replies: []models.Response{{Text: noError}, {Text: filesReply(t, dir, "")}, judge}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 1
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	run := res.Cases[0].Runs[0]
	if len(fake.Requests) != 3 || !run.Revised || run.Score != 5 || res.Cases[0].Model != "claude-opus-5-5" {
		t.Fatalf("%d calls, %s, run %+v", len(fake.Requests), res.Cases[0].Model, run)
	}
	first, revise, judged := fake.Requests[0], fake.Requests[1], fake.Requests[2]
	if !strings.Contains(first.System, "Team stage for this run: startup") || !strings.Contains(first.Messages[0].Content, `<untrusted source="ledger:design/420/research/files/notes.md">`) {
		t.Error("the skill prompt lacks the team stage or the research stage")
	}
	if !strings.Contains(revise.Messages[2].Content, `no state "error"`) {
		t.Errorf("revision request: %.400s", revise.Messages[2].Content)
	}
	if !strings.Contains(judged.Messages[0].Content, "### list.md") || !strings.Contains(judged.Messages[0].Content, "Passed: `aayu wireframe`'s checks") {
		t.Errorf("judge request: %.400s", judged.Messages[0].Content)
	}

	rc := uxCase(t, "review/planted-preferences")
	rc.Tier2 = &Tier2Spec{Rubric: "rubric.md"}
	reply, err := os.ReadFile(filepath.Join(rc.Dir, "output", "reply.json"))
	if err != nil {
		t.Fatal(err)
	}
	missed := strings.Replace(string(reply), `{"kind": "target-size", "heuristic": 7, "severity": 2`, `{"kind": "target-size", "heuristic": 7, "severity": 1`, 1)
	fake = &models.Fake{Replies: []models.Response{{Text: missed}, {Text: string(reply)}, judge}}
	h = harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 2
	res, err = h.Tier2(context.Background(), []Case{rc}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if len(fake.Requests) != 3 || runs[0].Score != 1 || !strings.Contains(runs[0].Floor, "missed the target-size defect") || runs[1].Score != 5 ||
		res.Cases[0].Model != "gemini-3.1-pro-low" {
		t.Fatalf("%d calls, %s, runs %+v", len(fake.Requests), res.Cases[0].Model, runs)
	}
	if lane := fake.Requests[0]; len(lane.Messages[0].Images) != 2 || !strings.Contains(lane.System, "You are lane A of a UI review") {
		t.Errorf("lane A request: %d images, %.80s", len(lane.Messages[0].Images), lane.System)
	}
	if j := fake.Requests[2].Messages[0].Content; !strings.Contains(j, "Lane B on the same capture") || !strings.Contains(j, `<untrusted source="markup">`) {
		t.Errorf("judge request: %.300s", j)
	}
}
