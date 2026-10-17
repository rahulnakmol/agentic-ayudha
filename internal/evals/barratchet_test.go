package evals

import (
	"errors"
	"maps"
	"strings"
	"testing"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/ratchet"
)

// barTree is a small evals/ tree: a gating group and an advisory one, a case
// with a scope rule and a rubric, and a session case with its rules.
func barTree() map[string]string {
	return map[string]string{
		"evals/eval.toml": `judge = "gpt-6.1-sol"
k = 5
pass_score = 4
min_pass_k = 0.8
min_mean = 3.5
max_drop = 0.3

[[groups]]
name = "plain"
prefixes = ["stub/"]
max_usd = 3.0

[[groups]]
name = "quiet"
prefixes = ["other/"]
max_usd = 3.0
gate = "advisory"

[[groups]]
name = "skills"
max_usd = 4.0
`,
		"evals/cases/stub/one/case.toml": `skill = "stub"
input = "input.txt"
scope = [
  { pattern = '(?i)\bcsv\b', why = "a format the request does not state" },
]

[tier2]
rubric = "rubric.md"
`,
		"evals/cases/stub/one/input.txt": "Export the list.\n",
		"evals/cases/stub/one/rubric.md": `# Rubric: export

Score the Story from 1 to 5. Every requirement has an observable criterion
with fitting evidence.

1. Faithful: nothing beyond the request is required.
2. Observable: each expected result can be seen.

- 5: Both hold.
- 1: Not a usable Story.
`,
		"evals/cases/build/two/case.toml": `skill = "build"
input = "input.txt"

[tier2]
rubric = "rubric.md"
session = "session.toml"
`,
		"evals/cases/build/two/input.txt": "Build #41.\n",
		"evals/cases/build/two/rubric.md": "- 5: It builds the story.\n",
		"evals/cases/build/two/session.toml": `max_turns = 4

[[forbid]]
pattern = '^gh\b'
why = "works around the binary"

[[say_forbid]]
pattern = '(?i)\bmerged\b'
allow = '(?i)\bnot merged\b'
why = "claims the work merged"
`,
	}
}

// barRoot writes files under a new temporary root.
func barRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	return root
}

// Story #400 (Feedback #327): a change that lowers a skill-eval bar fails the
// check unless it names a new floor waiver for eval-bar that a human accepted;
// a change that only raises or adds passes with no waiver.
func TestEvalBarRatchet(t *testing.T) {
	base := barRoot(t, barTree())
	edit := func(path, old, new string) func(map[string]string) {
		return func(f map[string]string) {
			if !strings.Contains(f[path], old) {
				t.Fatalf("%s lacks %q", path, old)
			}
			f[path] = strings.Replace(f[path], old, new, 1)
		}
	}
	accepted := func(int) error { return nil }
	for _, tc := range []struct {
		name   string
		change func(map[string]string)
		bar    string // the lowered bar; empty: nothing lowered
	}{
		{"min_mean lowered", edit("evals/eval.toml", "min_mean = 3.5", "min_mean = 3.4"), "min_mean"},
		{"min_pass_k lowered", edit("evals/eval.toml", "min_pass_k = 0.8", "min_pass_k = 0.6"), "min_pass_k"},
		{"pass_score lowered", edit("evals/eval.toml", "pass_score = 4", "pass_score = 3"), "pass_score"},
		{"max_drop raised", edit("evals/eval.toml", "max_drop = 0.3", "max_drop = 0.5"), "max_drop"},
		{"a gating group set to advisory", edit("evals/eval.toml", "max_usd = 3.0\n\n[[groups]]\nname = \"quiet\"", "max_usd = 3.0\ngate = \"advisory\"\n\n[[groups]]\nname = \"quiet\""), "stub/one"},
		{"pass^k made advisory", edit("evals/eval.toml", "max_usd = 3.0\n\n[[groups]]\nname = \"quiet\"", "max_usd = 3.0\npass_k = \"advisory\"\n\n[[groups]]\nname = \"quiet\""), "stub/one"},
		{"a case moved into a new advisory group", edit("evals/eval.toml", "[[groups]]\nname = \"plain\"", "[[groups]]\nname = \"later\"\nprefixes = [\"stub/one\"]\nmax_usd = 2.0\ngate = \"advisory\"\n\n[[groups]]\nname = \"plain\""), "stub/one"},
		{"a rubric line deleted", edit("evals/cases/stub/one/rubric.md", "2. Observable: each expected result can be seen.\n", ""), "evals/cases/stub/one/rubric.md"},
		{"a rubric anchor loosened by a word", edit("evals/cases/stub/one/rubric.md", "- 5: Both hold.", "- 5: Most hold."), "evals/cases/stub/one/rubric.md"},
		{"a wrapped rubric paragraph loses words", edit("evals/cases/stub/one/rubric.md", "Every requirement has an observable criterion\nwith fitting evidence.", "Every requirement has a criterion."), "evals/cases/stub/one/rubric.md"},
		{"a rubric deleted", func(f map[string]string) { delete(f, "evals/cases/build/two/rubric.md") }, "evals/cases/build/two/rubric.md"},
		{"a scope rule removed", edit("evals/cases/stub/one/case.toml", "  { pattern = '(?i)\\bcsv\\b', why = \"a format the request does not state\" },\n", ""), "stub/one"},
		{"a session rule loosened with an allow", edit("evals/cases/build/two/session.toml", "pattern = '^gh\\b'\n", "pattern = '^gh\\b'\nallow = '^gh pr view'\n"), "build/two"},
		{"a negation allow widened", edit("evals/cases/build/two/session.toml", `\bnot merged\b`, `\bnot\b`), "build/two"},
		{"a case removed", func(f map[string]string) {
			for p := range maps.Clone(f) {
				if strings.HasPrefix(p, "evals/cases/stub/one/") {
					delete(f, p)
				}
			}
		}, "stub/one"},
		// Raising a bar, adding to a rubric, adding a rule, or a new case in a new advisory group lowers nothing.
		{"min_mean raised", edit("evals/eval.toml", "min_mean = 3.5", "min_mean = 4.0"), ""},
		{"a rubric line tightened", edit("evals/cases/stub/one/rubric.md", "- 5: Both hold.", "- 5: Both hold, with no gap."), ""},
		{"a rubric paragraph rewrapped", edit("evals/cases/stub/one/rubric.md", "criterion\nwith fitting", "criterion with\nfitting"), ""},
		{"a rule added", edit("evals/cases/build/two/session.toml", "max_turns = 4\n", "max_turns = 4\n\n[[forbid]]\npattern = '^git push'\nwhy = \"pushes\"\n"), ""},
		{"a new case in a new advisory group", func(f map[string]string) {
			f["evals/eval.toml"] = strings.Replace(f["evals/eval.toml"], "[[groups]]\nname = \"skills\"", "[[groups]]\nname = \"fresh\"\nprefixes = [\"new/\"]\nmax_usd = 1.0\ngate = \"advisory\"\n\n[[groups]]\nname = \"skills\"", 1)
			f["evals/cases/new/three/case.toml"] = "skill = \"new\"\ninput = \"input.txt\"\n"
			f["evals/cases/new/three/input.txt"] = "x\n"
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := barTree()
			tc.change(files)
			head := barRoot(t, files)
			lowered, problems, err := CheckBars(base, head, accepted)
			if err != nil {
				t.Fatal(err)
			}
			if tc.bar == "" {
				if len(lowered) > 0 || len(problems) > 0 {
					t.Fatalf("nothing is lowered, got %v, problems %q", lowered, problems)
				}
				return
			}
			if len(lowered) == 0 || lowered[0].Bar != tc.bar {
				t.Fatalf("lowered %v; want %s first", lowered, tc.bar)
			}
			if len(problems) == 0 || !strings.Contains(problems[0], tc.bar+" lowers a skill-eval bar without a new waiver") ||
				!strings.Contains(problems[len(problems)-1], "aayu decide --waiver eval-bar") {
				t.Fatalf("no waiver named: problems %q", problems)
			}

			// The PR names a new waiver a human accepted: it passes.
			named := maps.Clone(files)
			named["evals/eval.toml"] = "bar_waiver = 412\n" + named["evals/eval.toml"]
			var asked int
			if _, problems, _ := CheckBars(base, barRoot(t, named), func(n int) error { asked = n; return nil }); len(problems) > 0 || asked != 412 {
				t.Fatalf("an accepted waiver: problems %q, asked about #%d", problems, asked)
			}
			// A waiver that is not accepted fails, naming why.
			if _, problems, _ := CheckBars(base, barRoot(t, named), func(int) error { return errors.New("#412 is not accepted") }); len(problems) != 1 || !strings.Contains(problems[0], "bar_waiver = 412: #412 is not accepted") {
				t.Fatalf("an unaccepted waiver: problems %q", problems)
			}
		})
	}

	// A waiver the base already names does not count again.
	waived := barTree()
	waived["evals/eval.toml"] = "bar_waiver = 7\n" + waived["evals/eval.toml"]
	again := maps.Clone(waived)
	edit("evals/eval.toml", "min_mean = 3.5", "min_mean = 3.0")(again)
	if _, problems, _ := CheckBars(barRoot(t, waived), barRoot(t, again), accepted); len(problems) == 0 {
		t.Error("a lowering passed on the base's own waiver")
	}

	// The waiver is a floor-waiver Decision for eval-bar, closed by the person who accepted it.
	is := &gh.Issue{Number: gh.Ptr(412), Title: gh.Ptr("Waive the " + BarFloor + " floor on #400"), State: gh.Ptr("closed"), StateReason: gh.Ptr("completed"),
		Labels:   []*gh.Label{{Name: "type:decision"}, {Name: "decision:waiver"}},
		Body:     gh.Ptr("### Decision\n\nwaive, accepted by @founder on 2026-10-17.\n"),
		ClosedBy: &gh.User{Login: gh.Ptr("founder"), Type: gh.Ptr("User")}}
	if err := ratchet.AcceptedWaiver(is, BarFloor, nil); err != nil {
		t.Errorf("an accepted eval-bar waiver was refused: %v", err)
	}
	is.Title = gh.Ptr("Waive the coverage floor on #400")
	if err := ratchet.AcceptedWaiver(is, BarFloor, nil); err == nil {
		t.Error("a coverage waiver was taken for an eval-bar waiver")
	}

	// The repository's own evals load in both roles and lower nothing against themselves.
	if lowered, problems, err := CheckBars(root, root, accepted); err != nil || len(lowered) > 0 || len(problems) > 0 {
		t.Fatalf("the repository against itself: %v, %v, %q", err, lowered, problems)
	}
}
