package brain

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

const today = "2026-10-18"

func skeleton(t *testing.T) map[string][]byte {
	t.Helper()
	files := Skeleton("o/r", today, true)
	if r := Check(files); !r.OK() {
		t.Fatalf("the skeleton fails its own check: %v", r.Errors)
	}
	return files
}

func TestSkeletonIsAnObsidianVault(t *testing.T) {
	files := skeleton(t)
	for _, p := range []string{"Home.md", "Decisions/Decisions.md", "Objections/Objections.md", "Capabilities/Capabilities.md",
		"Releases/Releases.md", "Timeline/Timeline.md", ".obsidian/graph.json", ".obsidian/templates.json", ".gitignore"} {
		if _, ok := files[p]; !ok {
			t.Errorf("no %s", p)
		}
	}
	for _, name := range []string{"Release", "Objective", "Objection", "Capability", "Decision", "Seat", "Workstream", "Vision", "Mission", "Principle", "Guidance", "Direction", "Persona", "Glossary term"} {
		if _, ok := files["Templates/"+name+" template.md"]; !ok {
			t.Errorf("no template for %s", name)
		}
	}
	for _, gone := range []string{"Story", "Bug", "Feedback"} {
		if _, ok := files["Templates/"+gone+" template.md"]; ok {
			t.Errorf("a %s template: issues stay the record of the work", gone)
		}
	}
	if !strings.Contains(string(files[".obsidian/graph.json"]), `"query": "tag:#type/objection"`) {
		t.Errorf("graph.json does not colour by type:\n%s", files[".obsidian/graph.json"])
	}
	if !strings.Contains(string(files["Revenue/Revenue.md"]), "This repository is public") {
		t.Error("a public repository's Revenue map does not carry the public rule")
	}
}

func note(fm, body string) []byte { return []byte("---\n" + fm + "---\n\n" + body) }

const capFM = `id: capability-proof
type: capability
title: Proof
owner: product-strategist
status: live
updated: 2026-10-01
reviewed: 2026-10-01
sources: ["issue #32"]
up: "[[Capabilities]]"
issues: [32]
tags: [type/capability]
`

func TestCheckpointKeepsHumanTextAndRefreshesGeneratedBlocks(t *testing.T) {
	files := skeleton(t)
	human := "# Proof\n\n## What it does\n\nIt keeps the evidence for each change.\n\n```text\n<!-- BEGIN aayudha:brain:history -->\n```\n\n````md\n```\n<!-- BEGIN aayudha:brain:record -->\n````\n\n```text\n    ```\n<!-- BEGIN aayudha:brain:history -->\nan example\n<!-- END aayudha:brain:history -->\n```\n\n```text\n\t```\n<!-- BEGIN aayudha:brain:history -->\na tab example\n<!-- END aayudha:brain:history -->\n```\n\n## Why it exists\n\nA claim without evidence does not count.\n"
	files["Capabilities/Proof.md"] = note(capFM, human+"\n"+Block("history", "- stale line")+"\n\nA closing line a person wrote.\n")
	files["Decisions/Decisions.md"] = []byte("---\n# A person's comment: keep this map.\n" + strings.TrimPrefix(string(files["Decisions/Decisions.md"]), "---\n"))
	r := Records{
		Repo: "o/r", Inception: "2026-09-01",
		Issues: map[int]Issue{32: {Number: 32, Type: "feature", Title: "M4 Proof"}, 40: {Number: 40, Type: "story", Title: "Push packs --> [[x]] <!-- END aayudha:brain:delivered -->", Parent: 32},
			42: {Number: 42, Type: "story", Title: "Track ![x](https://example.test/t) <b> %%", Parent: 32},
			41: {Number: 41, Type: "story", Title: "Other", Parent: 99}, 268: {Number: 268, Type: "release"}},
		Tags:  []Tag{{Name: "v0.2.0", Date: "2026-10-17", URL: "https://github.com/o/r/releases/tag/v0.2.0"}},
		Rungs: []Rung{{Path: "releases/268/main.json", Release: 268, Version: "v0.2.0", Target: "main", Tag: "v0.2.0", Stories: []int{41, 40, 42}}},
	}
	cp, err := Run(files, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	merged := maps.Clone(files)
	maps.Copy(merged, cp.Files)
	got := string(merged["Capabilities/Proof.md"])
	for _, want := range []string{human, "A closing line a person wrote.", "- Delivered in [[v0.2.0]] by #40, #42.", "- Recorded in #32 Feature: M4 Proof."} {
		if !strings.Contains(got, want) {
			t.Errorf("the capability lost or lacks %q:\n%s", want, got)
		}
	}
	if _, ok := cp.Files["Decisions/Decisions.md"]; ok {
		t.Errorf("a checkpoint with no new fact rewrote a map (and dropped its comment)")
	}
	if rel := string(merged["Releases/v0.2.0.md"]); !strings.Contains(rel, `#42 Story: Track \!\[x\]\(https://example.test/t\) \<b\> \%\%`) {
		t.Errorf("an issue title's Markdown is not escaped:\n%s", rel)
	}
	if strings.Contains(got, "stale line") || strings.Contains(got, "#41") {
		t.Errorf("the generated block was not refreshed, or names a story outside the capability:\n%s", got)
	}
	n, _ := Parse("", merged["Capabilities/Proof.md"])
	if n.Str("updated") != today || n.Str("reviewed") != "2026-10-01" || n.Str("owner") != "product-strategist" {
		t.Errorf("frontmatter: updated %q reviewed %q owner %q", n.Str("updated"), n.Str("reviewed"), n.Str("owner"))
	}
	if len(cp.Review) != 1 || cp.Review[0] != "Capabilities/Proof.md" {
		t.Errorf("needs review = %v, want the capability whose facts changed after its review", cp.Review)
	}
	if !strings.Contains(string(merged["Home.md"]), "- [[Proof]]") {
		t.Error("Home does not list the note that needs review")
	}
	rel := string(merged["Releases/v0.2.0.md"])
	if !strings.Contains(rel, "#32 Feature: M4 Proof [[Proof]]") || !strings.Contains(rel, "  - #40 Story: Push packs --\\> \\[\\[x\\]\\] \\<\\!-- END aayudha:brain:delivered --\\>") || !strings.Contains(rel, "Tagged v0.2.0 on 2026-10-17") {
		t.Errorf("the release note does not group what shipped under its feature and capability:\n%s", rel)
	}
	if rep := Check(merged); !rep.OK() {
		t.Errorf("the checkpointed vault fails check: %v", rep.Errors)
	}
	again, err := Run(merged, r, today, true)
	if err != nil || len(again.Paths) != 0 {
		t.Errorf("a second checkpoint with the same records changed %v (%v)", again.Paths, err)
	}
	withIssue := maps.Clone(merged) // a person lists the Release issue on its note
	withIssue["Releases/v0.2.0.md"] = []byte(strings.Replace(rel, "\nup:", "\nissues: [268]\nup:", 1))
	if listed, err := Run(withIssue, r, today, true); err != nil || len(listed.Paths) != 0 {
		t.Errorf("a release note that lists its issue was rewritten: %v (%v):\n%s", listed.Paths, err, listed.Files["Releases/v0.2.0.md"])
	}

	// A main rung releases a version that has no GitHub Release; a person who
	// lost every role keeps a note that no longer claims one.
	r.Rungs = append(r.Rungs, Rung{Path: "releases/300/main.json", Release: 300, Version: "v0.3.0", Target: "main", Tag: "v0.3.0", Date: "2026-10-20"})
	r.Approver = "@Alice" // her note is People/Alice.md: a login ignores case and its optional @
	merged["People/Alice.md"] = note("id: person-alice\ntype: person\ntitle: Alice\nowner: founder\nstatus: current\nupdated: 2026-10-01\nsources: [\"file .aayudha/config.toml\"]\nup: \"[[People]]\"\ntags: [type/person]\n",
		"# Alice\n\n## Roles\n\n"+Block("roles", "")+"\n")
	merged["People/olduser.md"] = note("id: person-olduser\ntype: person\ntitle: olduser\nowner: founder\nstatus: alumni\nupdated: 2026-10-01\nsources: [\"file .aayudha/config.toml\"]\nup: \"[[People]]\"\ntags: [type/person]\n",
		"# olduser\n\n## Roles\n\n"+Block("roles", "- [[Founder]]")+"\n")
	third, err := Run(merged, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	if tl := string(third.Files["Timeline/Timeline.md"]); !strings.Contains(tl, "[[v0.3.0]] is released (promoted to main by Release #300).") || !strings.Contains(tl, "ledger releases/300/main.json") {
		t.Errorf("the timeline lacks a version released on main without a GitHub Release:\n%s", third.Files["Timeline/Timeline.md"])
	}
	if n, _ := Parse("", third.Files["Releases/v0.3.0.md"]); n.Str("status") != "released" {
		t.Errorf("a version on main is %q, want released", n.Str("status"))
	}
	if got := string(third.Files["People/olduser.md"]); !strings.Contains(got, "- No role in the current records.") || strings.Contains(got, "[[Founder]]") {
		t.Errorf("a person who lost every role still claims one:\n%s", got)
	}
	if _, dup := third.Files["People/@alice.md"]; dup || !strings.Contains(string(third.Files["People/Alice.md"]), "the approver") {
		t.Errorf("the checkpoint did not reuse People/Alice.md for alice: wrote %v", third.Paths)
	}
	hostile := maps.Clone(merged) // a person's title is text in a map, never a marker or an image
	hostile["Capabilities/Proof.md"] = []byte(strings.Replace(string(merged["Capabilities/Proof.md"]), "title: Proof", `title: "Proof <!-- END aayudha:brain:notes --> ![x](https://example.test/t)"`, 1))
	fifth, err := Run(hostile, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(hostile, fifth.Files)
	if m := string(hostile["Capabilities/Capabilities.md"]); strings.Count(m, "<!-- END aayudha:brain:notes -->") != 1 || strings.Contains(m, "![x]") {
		t.Errorf("a note title broke out of its map's generated block:\n%s", m)
	}
	persona := maps.Clone(merged) // a note whose type does not list issues loses an issue's record too
	persona["Personas/Founder.md"] = note("id: persona-founder\ntype: persona\ntitle: Founder\nowner: product-strategist\nstatus: current\nupdated: 2026-10-01\nsources: [\"issue #32\"]\nup: \"[[Personas]]\"\nissues: [32]\ntags: [type/persona]\n", "# Founder\n")
	pcp, err := Run(persona, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(persona, pcp.Files)
	persona["Personas/Founder.md"] = []byte(strings.Replace(string(persona["Personas/Founder.md"]), "issues: [32]", "issues: []", 1))
	if pcp, err = Run(persona, r, today, true); err != nil {
		t.Fatal(err)
	}
	maps.Copy(persona, pcp.Files)
	if got := string(persona["Personas/Founder.md"]); !strings.Contains(got, "BEGIN aayudha:brain:record") || strings.Contains(got, "Recorded in #32") {
		t.Errorf("a persona with no issues keeps the removed issue's record:\n%s", got)
	}
	gone := maps.Clone(merged) // a person removes the last issue: its generated facts go too
	gone["Capabilities/Proof.md"] = []byte(strings.Replace(string(merged["Capabilities/Proof.md"]), "issues: [32]", "issues: []", 1))
	fourth, err := Run(gone, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(gone, fourth.Files)
	if n, _ := Parse("", gone["Capabilities/Proof.md"]); strings.TrimSpace(Blocks(n.Body)["record"]+Blocks(n.Body)["history"]) != "" {
		t.Errorf("a capability with no issues keeps the removed issue's facts:\n%s", gone["Capabilities/Proof.md"])
	}
	unshipped := r // the records no longer name a story for v0.2.0: its Delivered block empties
	unshipped.Rungs = nil
	sixth, err := Run(merged, unshipped, today, true)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := Parse("", sixth.Files["Releases/v0.2.0.md"]); strings.TrimSpace(Blocks(n.Body)["delivered"]) != "" {
		t.Errorf("a release keeps stories the records no longer hold:\n%s", sixth.Files["Releases/v0.2.0.md"])
	}
	merged["Capabilities/Proof.md"] = []byte(strings.Replace(string(merged["Capabilities/Proof.md"]), "issues: [32]", "issues: [32, not-an-issue]", 1))
	if _, err := Run(merged, r, today, true); err == nil || !strings.Contains(err.Error(), `"not-an-issue" is not an issue number`) {
		t.Errorf("a checkpoint over a malformed issues entry: err %v", err)
	}
}

func TestCheckpointWithoutHomeKeepsTheVault(t *testing.T) {
	files := skeleton(t)
	delete(files, HomePath)
	if !IsVault(files) {
		t.Fatal("a vault without Home but with the aayudha marker is not a vault")
	}
	other := maps.Clone(files)
	delete(other, MarkerPath)
	if IsVault(other) {
		t.Fatal("a vault with only .obsidian/types.json counts as an aayudha brain")
	}
	mine := []byte(strings.Replace(string(files["Decisions/Decisions.md"]), "The durable reasons", "Our own words on the durable reasons", 1))
	files["Decisions/Decisions.md"] = mine
	cp, err := Run(files, Records{Repo: "o/r"}, today, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cp.Files[HomePath]; !ok {
		t.Error("the checkpoint did not restore Home")
	}
	if b, ok := cp.Files["Decisions/Decisions.md"]; ok && !strings.Contains(string(b), "Our own words") {
		t.Errorf("restoring Home replaced a map a person wrote:\n%s", b)
	}
}

func TestCheckpointRendersThePlanAndItsDrift(t *testing.T) {
	files := skeleton(t)
	files["Capabilities/Proof.md"] = note(capFM, "# Proof\n")
	r := Records{
		Repo:   "o/r",
		Issues: map[int]Issue{30: {Type: "initiative", Title: "Dogfood"}, 32: {Type: "feature", Title: "M4 Proof", Parent: 30}, 657: {Type: "initiative", Title: "Review in CI", Parent: 32}}, // renamed and retyped since the baseline
		Baseline: []Baseline{{Path: "releases/v0.3.0/baseline-2.json", N: 2, Version: "v0.3.0", ReleaseIssue: 700, BaselinedAt: "2026-10-18", BaselinedBy: "rahulnakmol", BudgetPct: 10,
			Items: []PlanItem{{Number: 657, Type: "story", Title: "CI review", Effort: "S", Parent: 32}}}},
		Drift: []Drift{{Path: "releases/v0.3.0/drift.json", Version: "v0.3.0", Baseline: 2, BudgetPct: 10, DriftPct: 15, Decision: 701,
			Added:   []PlanItem{{Number: 702, Type: "bug", Title: "Fix it"}},
			Resized: []Resize{{PlanItem: PlanItem{Number: 657, Type: "initiative", Title: "CI review"}, From: "S", To: "L", FromType: "story"}}}},
	}
	cp, err := Run(files, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	got := string(cp.Files["Releases/v0.3.0.md"])
	for _, want := range []string{"Baseline 2 of Release #700, taken 2026-10-18 by @rahulnakmol", "- #30 Initiative: Dogfood / #32 Feature: M4 Proof [[Proof]]",
		"  - #657 Story: CI review (effort S)", "15% of a 10% budget", "Over the budget: Decision #701.", "  - #702 Bug: Fix it", "- #657 Initiative: CI review: effort S to L; type Story to Initiative", "status: planned"} {
		if !strings.Contains(got, want) {
			t.Errorf("the release plan lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Review in CI") {
		t.Errorf("the plan shows the issue's title today, not the one its baseline captured:\n%s", got)
	}
	merged := maps.Clone(files)
	maps.Copy(merged, cp.Files)
	if rep := Check(merged); !rep.OK() {
		t.Errorf("a plan's issue titles and types are link lines, not copies, yet check fails: %v", rep.Errors)
	}
}

func rules(r Report) string {
	var s []string
	for _, f := range r.Errors {
		s = append(s, f.Rule+" "+f.Message)
	}
	return strings.Join(s, "\n")
}

func TestCheckFailsWhatBreaksTheBrain(t *testing.T) {
	good := `id: principle-evidence
type: principle
title: Evidence first
owner: founder
status: current
updated: 2026-10-18
sources: ["file ETHOS.md"]
up: "[[Principles]]"
tags: [type/principle]
`
	long := "This sentence has far too many words in it for a reader to follow, because it keeps adding one clause after another clause until it ends."
	if n := len(strings.Fields(long)); n <= MaxSentenceWords {
		t.Fatalf("the long sentence has %d words", n)
	}
	cases := []struct {
		name, fm, body, rule string
	}{
		{"clean", good, "# Evidence first\n\nEvery claim cites its source. See [[Decisions]].\n", ""},
		{"broken link", good, "See [[No such note]].\n", "link [[No such note]] does not resolve"},
		{"missing source", strings.Replace(good, `sources: ["file ETHOS.md"]`, "sources: []", 1), "Short.\n", "sources no sources"},
		{"not a citation", strings.Replace(good, `"file ETHOS.md"`, `"the founder said so"`, 1), "Short.\n", `sources source "the founder said so" is not a citation`},
		{"long sentence", good, long + "\n", "ste a sentence has 26 words"},
		{"long sentence in a generated block is not prose", good, Block("record", "- "+long) + "\n", ""},
		{"orphan", strings.Replace(good, "up: \"[[Principles]]\"\n", "", 1), "Short.\n", "orphan no up link"},
		{"a relationship to the note itself", strings.Replace(good, "owner: founder", "owner: founder\nobjectives: [\"[[Evidence first]]\"]", 1), "Short.\n", "link objectives: [[Evidence first]] does not resolve to another note"},
		{"an impossible date", strings.Replace(good, "updated: 2026-10-18", "updated: 2026-99-42", 1), "Short.\n", `updated "2026-99-42" is not a YYYY-MM-DD date`},
		{"a link in an indented code block is text", good, "Short.\n\n    See [[No such note]].\n", ""},
		{"a link in a fence inside a quote is text", good, "> ~~~text\n> See [[No such note]].\n> ~~~\n", ""},
		{"a backtick in a fence's info string opens no fence", good, "````md`example`\nSee [[No such note]].\n", "link [[No such note]] does not resolve"},
		{"an open comment in a fence hides no link", good, "```text\n<!-- BEGIN aayudha:brain:history\n```\n\nSee [[No such note]].\n\n<!-- a person's note -->\n", "link [[No such note]] does not resolve"},
		{"a comment marker in code hides no link", good, "Write `<!--` to open a comment. See [[No such note]]. Close it with `-->`.\n", "link [[No such note]] does not resolve"},
		{"a key twice", strings.Replace(good, "status: current", "status: current\nstatus: done", 1), "Short.\n", `frontmatter frontmatter: key "status" appears twice`},
		{"up is a template", strings.Replace(good, "[[Principles]]", "[[Principle template]]", 1), "Short.\n", "orphan up link [[Principle template]] does not resolve to another note"},
		{"a link in double-backtick code is text", good, "Write ``[[No such note]]`` to link a note.\n", ""},
		{"up is the note itself", strings.Replace(good, "[[Principles]]", "[[Evidence first#Context]]", 1), "Short.\n", "orphan up link [[Evidence first#Context]] does not resolve to another note"},
		{"up is a same-note anchor", strings.Replace(good, "[[Principles]]", "[[#Context]]", 1), "Short.\n", "orphan up link [[#Context]] does not resolve to another note"},
		{"issues: not a number", strings.Replace(good, "owner: founder", "owner: founder\nissues: [not-an-issue]", 1), "Short.\n", `issues: "not-an-issue" is not an issue number`},
		{"people: not a login", strings.Replace(good, "owner: founder", "owner: founder\npeople: [alice-]", 1), "Short.\n", `people: "alice-" is not a GitHub login`},
		{"owner is a person", strings.Replace(good, "owner: founder", "owner: rahulnakmol", 1), "Short.\n", `owner "rahulnakmol" is not a seat slug`},
		{"bad status", strings.Replace(good, "status: current", "status: done", 1), "Short.\n", `status "done" is not one of`},
		{"a link alias that opens a fence hides no pasted criterion", good, "See [[Decisions|\n```\n]].\n1. [screenshot] state: home; expected: a list\n", "copy a Completion Goal criterion"},
		{"a link alias that opens a fence hides no long sentence", good, "See [[Decisions|\n```\n]].\n" + long + "\n", "ste a sentence has 26 words"},
		{"a tilde alias hides no pasted criterion", good, "[[Decisions|~~~]]\n1. [screenshot] state: home; expected: a list\n", "copy a Completion Goal criterion"},
		{"an HTML alias hides no long sentence", good, "[[Decisions|<pre>]]\n" + long + "\n", "ste a sentence has 27 words"},
		{"an indented alias hides no long sentence", good, "Text.\n\n[[Decisions|    " + long + "]]\n", "ste a sentence has 26 words"},
		{"a generated block twice", good, Block("record", "- a") + "\n\n" + Block("record", "- b") + "\n", "block two generated record blocks"},
		{"issues as a mapping", strings.Replace(good, "owner: founder", "owner: founder\nissues: {1: 2}", 1), "Short.\n", `frontmatter: "issues" holds a mapping`},
		{"objectives links a note of another type", strings.Replace(good, "owner: founder", "owner: founder\nobjectives: [\"[[Decisions]]\"]", 1), "Short.\n", "objectives: [[Decisions]] has type moc; objectives links a note of type objective"},
		{"pasted criterion", good, "1. [screenshot] state: home; expected: a list\n", "copy a Completion Goal criterion"},
		{"pasted label", good, "It is status:ready now.\n", "copy an issue label (status:ready)"},
		{"label in code is fine", good, "The label is `status:ready`.\n", ""},
		{"label in a URL is fine", good, "See [the open bugs](https://github.com/o/r/issues?q=label:type:bug).\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := skeleton(t)
			files["Principles/Evidence first.md"] = note(tc.fm, tc.body)
			got := rules(Check(files))
			if tc.rule == "" && got != "" {
				t.Fatalf("want no error, got:\n%s", got)
			}
			if !strings.Contains(got, tc.rule) {
				t.Fatalf("want %q, got:\n%s", tc.rule, got)
			}
		})
	}
	if rt, _ := FindType("release"); strings.Contains(string(Template(rt)), "objections") { // check accepts no objection against a release
		t.Error("the Release template offers an objections key that check can never pass")
	}
	files := skeleton(t) // a file name that closes a wikilink fails check, and no generated map links it
	files["Principles/x]] ![t](t.png).md"] = note(good, "Short.\n")
	files["Principles/ Evidence first.md"] = note(strings.Replace(good, "principle-evidence", "principle-spaced", 1), "Short.\n") // [[ Evidence first]] would reach the other note
	files["Principles/x\n---\ny.md"] = note(strings.Replace(good, "principle-evidence", "principle-broken", 1), "Short.\n")       // a line break would split a generated link
	got := rules(Check(files))
	for _, name := range []string{"x]] ![t](t.png)", " Evidence first", "x\n---\ny"} {
		if !strings.Contains(got, fmt.Sprintf("name the file name %q breaks a wikilink", name)) {
			t.Errorf("check passed the file name %q:\n%s", name, got)
		}
	}
	if cp, err := Run(files, Records{Repo: "o/r"}, today, true); err != nil || strings.Contains(string(cp.Files["Principles/Principles.md"]), "![t]") {
		t.Errorf("a checkpoint linked a file name that breaks a wikilink (%v):\n%s", err, cp.Files["Principles/Principles.md"])
	}
	files = skeleton(t) // a closing line of ---suffix is not the delimiter, so the note has no frontmatter
	files["Principles/Evidence first.md"] = []byte("---\n" + good + "---suffix\nShort.\n")
	if got := rules(Check(files)); !strings.Contains(got, "frontmatter no YAML frontmatter") {
		t.Errorf("a ---suffix line closed the frontmatter:\n%s", got)
	}
}

func TestCheckObjectionsAndSupersedesLinkBothWays(t *testing.T) {
	files := skeleton(t)
	dec := func(id, extra string) string {
		return "id: " + id + "\ntype: decision\ntitle: " + id + "\nowner: founder\nstatus: accepted\nupdated: 2026-10-18\nsources: [\"Decision #1\"]\nup: \"[[Decisions]]\"\ntags: [type/decision]\n" + extra
	}
	files["Decisions/A.md"] = note(dec("a", "supersedes: [\"[[B]]\"]\nobjections: [\"[[Too slow]]\"]\n"), "# A\n")
	files["Decisions/B.md"] = note(dec("b", ""), "# B\n")
	files["Objections/Too slow.md"] = note("id: objection-slow\ntype: objection\ntitle: Too slow\nowner: product-strategist\nstatus: answered\nupdated: 2026-10-18\nsources: [\"issue #2\"]\nup: \"[[Objections]]\"\nraised_by: the founder\nobjects_to: \"[[B]]\"\ntags: [type/objection]\n", "# Too slow\n")
	got := rules(Check(files))
	for _, want := range []string{
		"two-way supersedes: [[B]] does not link back under superseded_by",
		"two-way objections: [[Too slow]] does not link back under objects_to",
		"two-way objects_to: [[B]] does not link back under objections",
		"objection status answered needs a resolution",
		"objection raised_by \"the founder\" is neither a seat slug nor a GitHub login",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q, got:\n%s", want, got)
		}
	}
	files["Decisions/B.md"] = note(dec("b", "superseded_by: [\"[[A]]\"]\n"), "# B\n")
	files["Objections/Too slow.md"] = []byte(strings.Replace(string(files["Objections/Too slow.md"]), "raised_by: the founder", "raised_by: founder", 1))
	files["Objections/Too slow.md"] = []byte(strings.Replace(strings.Replace(string(files["Objections/Too slow.md"]), `"[[B]]"`, `"[[A]]"`, 1), "raised_by:", "resolution: The cache answers it.\nraised_by:", 1))
	if got := rules(Check(files)); got != "" {
		t.Errorf("two-way links in place, yet:\n%s", got)
	}
	files["Capabilities/Proof.md"] = note(strings.Replace(capFM, "issues: [32]\n", "objections: [\"[[Too slow]]\"]\n", 1), "# Proof\n")
	files["Objections/Too slow.md"] = []byte(strings.Replace(string(files["Objections/Too slow.md"]), `objects_to: "[[A]]"`, `objects_to: "[[Proof]]"`, 1))
	if got := rules(Check(files)); !strings.Contains(got, "two-way objects_to: [[Proof]] has type capability, and this note has type objection") {
		t.Errorf("an objection against a capability passes:\n%s", got)
	}
}

func TestWorkstreamsLinkIntoOneGraph(t *testing.T) {
	files := skeleton(t)
	maps.Copy(files, Workstream("o/r", "payments", today))
	files["Workstreams/payments/Capabilities/Refunds.md"] = note(strings.NewReplacer("capability-proof", "capability-refunds", "title: Proof", "title: Refunds", `"[[Capabilities]]"`, `"[[payments MOC]]"`).Replace(capFM),
		"# Refunds\n\nIt serves the root vault's [[Decisions]] map.\n")
	if r := Check(files); !r.OK() {
		t.Fatalf("a workstream with its index fails: %v", r.Errors)
	}
	cp, err := Run(files, Records{Repo: "o/r", Inception: "2026-09-01"}, today, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cp.Files["Workstreams/payments/payments MOC.md"]), "- [[Refunds]]") ||
		!strings.Contains(string(cp.Files["Workstreams/Workstreams.md"]), "- [[payments Index]]") ||
		!strings.Contains(string(cp.Files["Home.md"]), "- [[payments Index]]") {
		t.Errorf("the workstream is not linked into the root graph")
	}
	delete(files, WorkstreamIndex("payments"))
	if got := rules(Check(files)); !strings.Contains(got, "workstream no index note Workstreams/payments/payments Index.md") {
		t.Errorf("a workstream without its index passes:\n%s", got)
	}
	files["Capabilities/Refunds.md"] = files["Workstreams/payments/Capabilities/Refunds.md"]
	if got := rules(Check(files)); !strings.Contains(got, `unique-name 2 notes share the name "refunds"`) {
		t.Errorf("two notes with one name pass:\n%s", got)
	}
}

// A checkpoint finds a note a person moved or re-cased instead of making a
// second one; it empties the generated blocks of a release or seat the records
// no longer name; and text that is not a login names no person note.
func TestCheckpointFindsMovedNotesAndClearsGoneRecords(t *testing.T) {
	seat := func(id, name, human string) Seat {
		return Seat{ID: id, Name: name, Twin: "Atri", Purpose: "Counsels.", Verbs: []string{"decide"}, Human: human}
	}
	r := Records{Repo: "o/r", Issues: map[int]Issue{},
		Tags:  []Tag{{Name: "v0.1.0", Date: "2026-10-01", URL: "https://github.com/o/r/releases/tag/v0.1.0"}, {Name: "v0.2.0", Date: "2026-10-17", URL: "https://github.com/o/r/releases/tag/v0.2.0"}},
		Seats: []Seat{seat("founder", "Founder", ""), seat("old-seat", "Old Seat", "")},
	}
	first, err := Run(skeleton(t), r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	files := maps.Clone(skeleton(t))
	maps.Copy(files, first.Files)
	// A person moves the note, changes its case, and removes its empty Plan block.
	files["Releases/2026/V0.2.0.md"] = []byte(strings.Replace(string(files["Releases/v0.2.0.md"]), Block("plan", ""), "", 1))
	delete(files, "Releases/v0.2.0.md")

	// A decision note lists its Decision and a related Story: only the Decision dates the choice.
	files["Decisions/Use goldmark.md"] = note("id: decision-goldmark\ntype: decision\ntitle: Use goldmark\nowner: founder\nstatus: accepted\nupdated: 2026-10-18\nsources: [\"issue #5\"]\nup: \"[[Decisions]]\"\nissues: [5, 6]\ntags: [type/decision]\n", "# Use goldmark\n")
	r.Issues = map[int]Issue{5: {Number: 5, Type: "decision", Title: "Use goldmark", Closed: "2026-10-10"}, 6: {Number: 6, Type: "story", Title: "Mask code", Closed: "2026-10-11"}}
	r.Tags = r.Tags[1:] // v0.1.0 and Old Seat leave the records
	r.Seats = []Seat{seat("founder", "Founder", "../not a login")}
	second, err := Run(files, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(files, second.Files)
	if _, dup := files["Releases/v0.2.0.md"]; dup {
		t.Errorf("the checkpoint made a second v0.2.0 note beside the moved one: wrote %v", second.Paths)
	}
	blocks := func(p string) map[string]string {
		n, err := Parse(p, files[p])
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		return Blocks(n.Body)
	}
	if b := blocks("Releases/2026/V0.2.0.md"); !strings.Contains(b["record"], "Tagged v0.2.0") {
		t.Errorf("the moved release note was not refreshed: %v", b)
	} else if _, ok := b["plan"]; ok {
		t.Error("an empty generated block came back after a person removed it")
	}
	if b := blocks("Releases/v0.1.0.md"); strings.TrimSpace(b["record"]) != "" {
		t.Errorf("a release the records no longer name keeps its record: %q", b["record"])
	}
	if tl := string(files["Timeline/Timeline.md"]); !strings.Contains(tl, "[[Use goldmark]] is decided in #5.") || strings.Contains(tl, "decided in #6") {
		t.Errorf("the timeline dates a decision by a related Story, or not by its Decision:\n%s", tl)
	}
	twice := maps.Clone(files) // a copied generated block is refused, not left stale
	twice["Seats/Founder.md"] = append(slices.Clone(files["Seats/Founder.md"]), []byte("\n"+Block("facts", "- old")+"\n")...)
	if _, err := Run(twice, r, today, true); err == nil || !strings.Contains(err.Error(), "two generated facts blocks") {
		t.Errorf("a checkpoint over a note with two facts blocks: err %v", err)
	}
	if b := blocks("Seats/Old Seat.md"); strings.TrimSpace(b["facts"]) != "" {
		t.Errorf("a seat the records no longer name keeps its facts: %q", b["facts"])
	}
	if b := blocks("Seats/Founder.md"); !strings.Contains(b["facts"], "human is not a GitHub login") {
		t.Errorf("a paired seat with a bad login: %q", b["facts"])
	}
	for _, p := range second.Paths {
		if strings.HasPrefix(p, "People/") && p != "People/People.md" {
			t.Errorf("text that is not a login made a person note: %s", p)
		}
	}
	if rep := Check(files); !rep.OK() {
		t.Errorf("the vault fails check: %v", rep.Errors)
	}
}
