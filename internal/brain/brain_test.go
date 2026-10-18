package brain

import (
	"maps"
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

	// A main rung releases a version that has no GitHub Release; a person who
	// lost every role keeps a note that no longer claims one.
	r.Rungs = append(r.Rungs, Rung{Path: "releases/300/main.json", Release: 300, Version: "v0.3.0", Target: "main", Tag: "v0.3.0", Date: "2026-10-20"})
	r.Approver = "alice" // her note is People/Alice.md: a login ignores case
	merged["People/Alice.md"] = note("id: person-alice\ntype: person\ntitle: Alice\nowner: founder\nstatus: current\nupdated: 2026-10-01\nsources: [\"file .aayudha/config.toml\"]\nup: \"[[People]]\"\ntags: [type/person]\n",
		"# Alice\n\n## Roles\n\n"+Block("roles", "")+"\n")
	merged["People/olduser.md"] = note("id: person-olduser\ntype: person\ntitle: olduser\nowner: founder\nstatus: alumni\nupdated: 2026-10-01\nsources: [\"file .aayudha/config.toml\"]\nup: \"[[People]]\"\ntags: [type/person]\n",
		"# olduser\n\n## Roles\n\n"+Block("roles", "- [[Founder]]")+"\n")
	third, err := Run(merged, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(third.Files["Timeline/Timeline.md"]), "[[v0.3.0]] is released (promoted to main by Release #300).") {
		t.Errorf("the timeline lacks a version released on main without a GitHub Release:\n%s", third.Files["Timeline/Timeline.md"])
	}
	if n, _ := Parse("", third.Files["Releases/v0.3.0.md"]); n.Str("status") != "released" {
		t.Errorf("a version on main is %q, want released", n.Str("status"))
	}
	if got := string(third.Files["People/olduser.md"]); !strings.Contains(got, "- No role in the current records.") || strings.Contains(got, "[[Founder]]") {
		t.Errorf("a person who lost every role still claims one:\n%s", got)
	}
	if _, dup := third.Files["People/alice.md"]; dup || !strings.Contains(string(third.Files["People/Alice.md"]), "the approver") {
		t.Errorf("the checkpoint did not reuse People/Alice.md for alice: wrote %v", third.Paths)
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
		Issues: map[int]Issue{30: {Type: "initiative", Title: "Dogfood"}, 32: {Type: "feature", Title: "M4 Proof", Parent: 30}, 657: {Type: "story", Title: "CI review", Parent: 32}},
		Baseline: []Baseline{{Path: "releases/v0.3.0/baseline-2.json", N: 2, Version: "v0.3.0", ReleaseIssue: 700, BaselinedAt: "2026-10-18", BaselinedBy: "rahulnakmol", BudgetPct: 10,
			Items: []PlanItem{{Number: 657, Type: "story", Title: "CI review", Effort: "S", Parent: 32}}}},
		Drift: []Drift{{Path: "releases/v0.3.0/drift.json", Version: "v0.3.0", Baseline: 2, BudgetPct: 10, DriftPct: 15, Decision: 701,
			Added: []PlanItem{{Number: 702, Type: "bug", Title: "Fix it"}}}},
	}
	cp, err := Run(files, r, today, true)
	if err != nil {
		t.Fatal(err)
	}
	got := string(cp.Files["Releases/v0.3.0.md"])
	for _, want := range []string{"Baseline 2 of Release #700, taken 2026-10-18 by @rahulnakmol", "- #30 Initiative: Dogfood / #32 Feature: M4 Proof [[Proof]]",
		"  - #657 Story: CI review (effort S)", "15% of a 10% budget", "Over the budget: Decision #701.", "  - #702 Bug: Fix it", "status: planned"} {
		if !strings.Contains(got, want) {
			t.Errorf("the release plan lacks %q:\n%s", want, got)
		}
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
		{"up is the note itself", strings.Replace(good, "[[Principles]]", "[[Evidence first#Context]]", 1), "Short.\n", "orphan up link [[Evidence first#Context]] does not resolve to another note"},
		{"up is a same-note anchor", strings.Replace(good, "[[Principles]]", "[[#Context]]", 1), "Short.\n", "orphan up link [[#Context]] does not resolve to another note"},
		{"issues: not a number", strings.Replace(good, "owner: founder", "owner: founder\nissues: [not-an-issue]", 1), "Short.\n", `issues: "not-an-issue" is not an issue number`},
		{"people: not a login", strings.Replace(good, "owner: founder", "owner: founder\npeople: [alice-]", 1), "Short.\n", `people: "alice-" is not a GitHub login`},
		{"owner is a person", strings.Replace(good, "owner: founder", "owner: rahulnakmol", 1), "Short.\n", `owner "rahulnakmol" is not a seat slug`},
		{"bad status", strings.Replace(good, "status: current", "status: done", 1), "Short.\n", `status "done" is not one of`},
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
