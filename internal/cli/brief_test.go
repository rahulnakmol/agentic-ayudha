package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// standInNode puts a `node` on PATH that answers the web floor script with a
// page check holding the given axe violations (the real browser run is
// TestBriefPageWithBrowser in the floors workflow).
func standInNode(t *testing.T, axe string) string {
	t.Helper()
	dir := t.TempDir()
	out := `{"versions":{"playwright":"1.64.0","axe":"4.14.0"},"states":[{"n":1,"url":"file:///brief.html","status":0,"pii":[],"axe":` + axe + `,"keyboard_unreachable":[]}]}`
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s' '" + out + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

func writeDraft(t *testing.T, md string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "brief.md")
	if err := os.WriteFile(p, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// WP7.3 through the CLI on story #2 (Ready, PR #8, an advisory pack on the
// ledger): `brief close` without an authoritative pack exits 4; the grounding
// lists the pack; a draft with a planted 30-word sentence fails the STE lint
// (exit 3) before any model call; a grounded draft passes the adversarial
// pass and is rendered with its grounding footer and accessibility check; a
// claim the pass cannot ground blocks the page (exit 4). Once the merge gate's
// authoritative pack and an Outcome exist, `brief close` renders.
func TestBriefCLI(t *testing.T) {
	fgh, dir, _, env, opts, _ := proofFixture(t)
	opts.Assets = aayudha.Assets
	model := &models.Fake{Replies: []models.Response{{Text: `{"findings":[]}`}}}
	opts.ModelClient = func(models.Route, string) models.Client { return model }
	env["AAYU_SECRET_OPENAI"] = "model-key"
	env["PATH"] = standInNode(t, "[]")
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("proof push: %s", errOut)
	}

	code, out, errOut := run(t, env, opts, "brief", "2", "--purpose", "close", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "#2 has no authoritative pack (only an advisory one") || !strings.Contains(errOut, "#2 has no Result") {
		t.Fatalf("brief close without an authoritative pack: exit %d: %s %s", code, out, errOut)
	}

	code, out, errOut = run(t, env, opts, "brief", "2", "--repo", "o/r", "--json")
	var rep briefReport
	if err := json.Unmarshal([]byte(out), &rep); code != 0 || err != nil || len(rep.Grounding.Packs) != 1 || rep.Grounding.Packs[0].ID != "P1" || rep.Grounding.Packs[0].Authority != proof.Advisory {
		t.Fatalf("grounding: exit %d: %s %s", code, out, errOut)
	}

	thirty := "# Show the empty list\n\nThe user can open the list and see all the items that the filter shows, and the team can check each item in the same list before the export starts [P1].\n"
	code, out, _ = run(t, env, opts, "brief", "2", "--register", "ste", "--draft", writeDraft(t, thirty), "--repo", "o/r")
	if code != ExitFloor || !strings.Contains(out, "sentence 1 has 30 words (at most 20)") || len(model.Requests) != 0 {
		t.Fatalf("planted 30-word sentence: exit %d, %d model call(s): %s", code, len(model.Requests), out)
	}

	draft := "# The empty list now explains itself\n\nA reviewer who opens an empty list sees a message, and the coverage floor held at 95 against 90 [P1].\n\n" +
		"## Flow\n\n```diagram\nalt: The pack proves the story only after the floors run.\nstory: Story #2\nfloors: Floors\npack: Pack P1\nstory -> floors -> pack\n```\n"
	code, out, errOut = run(t, env, opts, "brief", "2", "--draft", writeDraft(t, draft), "--repo", "o/r", "--json")
	if code != 0 {
		t.Fatalf("brief: exit %d: %s %s", code, out, errOut)
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(rep.HTML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rep.HTML, filepath.Join(dir, ".aayudha", "state", "briefs", "2")) || len(rep.SVGs) != 1 || rep.Review == nil || rep.Review.Model != "gpt-6.1-sol" {
		t.Fatalf("report %+v", rep)
	}
	for _, want := range []string{`<li id="src-P1">`, "Advisory proof pack for #2", "Adversarial pass: gpt-6.1-sol (openai family)", "axe-core 4.14.0 found 0 critical and 0 serious issues"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if r := model.Requests[0]; r.Model != "gpt-6.1-sol" || !strings.Contains(r.Messages[0].Content, `<untrusted source="grounding">`) || !strings.Contains(r.Messages[0].Content, `<untrusted source="brief-draft">`) {
		t.Errorf("adversarial request: model %s, %.300s", r.Model, r.Messages[0].Content)
	}

	model.Replies = []models.Response{{Text: `{"findings":[{"claim":"the coverage floor held at 95","problem":"P1 says 95 against 90 for coverage, but the pack is advisory"}]}`}}
	if code, out, _ := run(t, env, opts, "brief", "2", "--draft", writeDraft(t, draft), "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out, "adversarial pass (gpt-6.1-sol)") {
		t.Fatalf("an ungrounded claim: exit %d: %s", code, out)
	}

	// A serious axe finding fails the page (exit 3); the page is still written for review.
	model.Replies = []models.Response{{Text: `{"findings":[]}`}}
	env["PATH"] = standInNode(t, `[{"id":"color-contrast","impact":"serious","help":"contrast","nodes":1,"target":"p"}]`)
	if code, _, errOut := run(t, env, opts, "brief", "2", "--draft", writeDraft(t, draft), "--repo", "o/r"); code != ExitFloor || !strings.Contains(errOut, "1 serious axe findings") {
		t.Fatalf("serious axe finding: exit %d: %s", code, errOut)
	}
	env["PATH"] = standInNode(t, "[]")

	// The merge gate's authoritative pack, then an Outcome: close renders.
	log, err := actions.Open(filepath.Join(t.TempDir(), "actions.jsonl"), actions.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := &proof.Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r", Log: log, Identity: "ci"}}
	if _, err := s.Push(context.Background(), proof.LocalDir(dir, 2), false, proof.Authority{Kind: proof.Authoritative, Workflow: ".github/workflows/" + MergeGateWorkflow, RunID: 77}); err != nil {
		t.Fatal(err)
	}
	closeDraft := writeDraft(t, "# Story 2 is complete\n\nThe merge gate's pack shows the coverage floor held, and the Outcome records the merge [P1, R].\n\n## Evidence\n\n- Coverage passed at 95 against a threshold of 90 [P1].\n- The Outcome says the PR merged [R].\n")
	if code, _, errOut := run(t, env, opts, "brief", "2", "--purpose", "close", "--register", "regulatory", "--draft", closeDraft, "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "#2 has no Result") || strings.Contains(errOut, "authoritative") {
		t.Fatalf("close with the pack but no Outcome: exit %d: %s", code, errOut)
	}
	fgh.Mu.Lock()
	fgh.Issues[2].Body += "\n\n### Outcome\n\nMerged in PR #8; the authoritative pack holds.\n"
	fgh.Mu.Unlock()
	code, out, errOut = run(t, env, opts, "brief", "2", "--purpose", "close", "--register", "regulatory", "--draft", closeDraft, "--repo", "o/r", "--json")
	if code != 0 {
		t.Fatalf("brief close: exit %d: %s %s", code, out, errOut)
	}
	_ = json.Unmarshal([]byte(out), &rep)
	if p := rep.Grounding.Packs[0]; p.Authority != proof.Authoritative || p.RunID != 77 || rep.Grounding.Result == nil || rep.Grounding.Result.Field != "outcome" {
		t.Fatalf("close grounding %+v", rep.Grounding)
	}
}
