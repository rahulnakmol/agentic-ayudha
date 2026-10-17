package seats

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func repoPersonas(t *testing.T) map[verbs.SeatID]Persona {
	t.Helper()
	ps, err := LoadPersonas(os.DirFS(filepath.Join("..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

// TestPersonas: all fourteen personas are complete, every playbook entry has a
// public source, and no persona text uses a word on the brand's avoid list or
// an exclamation mark (design/brand.yaml).
func TestPersonas(t *testing.T) {
	ps := repoPersonas(t)
	if len(ps) != len(verbs.Seats) {
		t.Fatalf("%d personas, want %d", len(ps), len(verbs.Seats))
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "design", "brand.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var brand struct {
		Voice struct {
			Avoid []string `yaml:"avoid"`
		} `yaml:"voice"`
	}
	if err := yaml.Unmarshal(b, &brand); err != nil {
		t.Fatal(err)
	}
	var avoid []*regexp.Regexp
	for _, w := range brand.Voice.Avoid {
		if strings.Contains(w, " ") {
			continue // phrases such as "exclamation marks" are rules, checked below
		}
		avoid = append(avoid, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(w)+`\b`))
	}
	for id, p := range ps {
		// Founder (2026-10-15): help the human decide quickly; never let a blocker sit. Behaviour, not clocks.
		if !slices.Contains(p.Values, "helping a human decide quickly, and never letting a blocker sit") {
			t.Errorf("%s: values lack the decide-quickly value", id)
		}
		for _, pr := range p.Practices {
			if len(pr.Sources) == 0 {
				t.Errorf("%s %s: no source", id, pr.ID)
			}
		}
		for _, text := range p.Texts() {
			if strings.Contains(text, "!") {
				t.Errorf("%s: exclamation mark in %q", id, text)
			}
			for _, re := range avoid {
				if m := re.FindString(text); m != "" {
					t.Errorf("%s: brand avoid-list word %q in %q", id, m, text)
				}
			}
		}
	}
}

// TestPersonaValidateRejects: each kind of incomplete or unsafe persona is
// refused with a message naming the problem.
func TestPersonaValidateRejects(t *testing.T) {
	good := repoPersonas(t)[verbs.EngineeringManager]
	if v := good.Validate(); len(v) > 0 {
		t.Fatalf("the EM persona should validate: %v", v)
	}
	clone := func(edit func(*Persona)) Persona {
		p := good
		p.Practices = append([]Practice(nil), good.Practices...)
		p.Practices[0].Sources = append([]Source(nil), good.Practices[0].Sources...)
		edit(&p)
		return p
	}
	tests := []struct {
		name string
		p    Persona
		want string
	}{
		{"empty voice", clone(func(p *Persona) { p.Voice = "" }), "voice is empty"},
		{"one value", clone(func(p *Persona) { p.Values = p.Values[:1] }), "values: want at least two"},
		{"not a question", clone(func(p *Persona) { p.Questions = []string{"Who owns it?", "Tell me more."} }), "is not a question"},
		{"source without a year", clone(func(p *Persona) { p.Practices[0].Sources = []Source{{Cite: "Someone, Some Book"}} }), "want author, title, venue, and year"},
		{"no source", clone(func(p *Persona) { p.Practices[0].Sources = nil }), "cite at least one public source"},
		{"http source", clone(func(p *Persona) { p.Practices[0].Sources = []Source{{Cite: "A, B (C, 2020)", URL: "http://x"}} }), "want https://"},
		{"company résumé", clone(func(p *Persona) { p.Temperament = "Calm, like the managers at Netflix." }), `names "Netflix"`},
		{"employment claim", clone(func(p *Persona) { p.Praise = "Praises as someone who worked at a large lab." }), "never claims employment"},
		{"stage artefacts overlap", clone(func(p *Persona) {
			p.Practices[0].Enterprise = Depth{Artefact: p.Practices[0].Startup.Artefact + " in full", Shape: "x"}
		}), "stage artefacts must be distinct"},
		{"Olympian name", clone(func(p *Persona) { p.Name = "Hermes" }), "an Olympian god"},
		{"title with article", clone(func(p *Persona) { p.Title = "the Founder's Sage" }), "without a leading"},
		{"two-word name", clone(func(p *Persona) { p.Name = "Nestor Pylos" }), "want one capitalised word"},
	}
	for _, tt := range tests {
		got := strings.Join(tt.p.Validate(), "\n")
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: got %q, want it to mention %q", tt.name, got, tt.want)
		}
	}
}

// TestSkillBlockVoiceAndStandard: verify is spoken by the AI Native Engineer
// twin against the Quality Architect's standard; inspect is the EM's voice and
// standard, executed in the review lane.
// A title replaces the seat beside the twin's name everywhere it is introduced;
// without one the seat name stays (D-5.1v).
func TestPersonaTitle(t *testing.T) {
	sage := Persona{Seat: verbs.Founder, Name: "Prometheus", Title: "Founder's Sage"}
	if got := sage.Heading(); got != "Prometheus, the Founder's Sage" {
		t.Errorf("Heading with a title = %q", got)
	}
	if got := agentsVoices(map[verbs.SeatID]Persona{verbs.Founder: sage}); !strings.Contains(got, "- **Prometheus**, Founder's Sage: ") {
		t.Errorf("AGENTS.md line with a title: %q", got)
	}
	em := Persona{Seat: verbs.EngineeringManager, Name: "Nestor"}
	if got, role := em.Heading(), em.Role(); got != "Nestor, the Engineering Manager twin" || role != "Engineering Manager" {
		t.Errorf("without a title: Heading %q, Role %q", got, role)
	}
}

func TestSkillBlockVoiceAndStandard(t *testing.T) {
	ps := repoPersonas(t)
	v, _ := verbs.Find("verify")
	b := ps[v.Seat].SkillBlock(v, ps[verbs.PolicyOwner(v)])
	if !strings.Contains(b, "## Voice: Talos, the AI Native Engineer twin") ||
		!strings.Contains(b, "Talos speaks and runs `verify` in the chore lane; the standard is Themis's, the Quality Architect's") {
		t.Errorf("verify block does not split voice and standard:\n%s", b)
	}
	in, _ := verbs.Find("inspect")
	b = ps[in.Seat].SkillBlock(in, ps[verbs.PolicyOwner(in)])
	if !strings.Contains(b, "`inspect` runs in the review lane; Nestor owns both the voice and the standard.") {
		t.Errorf("inspect block:\n%s", b)
	}
	for _, must := range []string{EvidenceRule, StageRule, "**When paired with a human:**", "| Practice | Startup | Scaleup | Enterprise |"} {
		if !strings.Contains(b, must) {
			t.Errorf("inspect block lacks %q", must)
		}
	}
}

// TestCheckGrounding: the evidence rule on a twin's output. Each case differs
// from a passing one by the single thing the rule must catch.
func TestCheckGrounding(t *testing.T) {
	grounding := "Tombstone in internal/ledger/tombstone.go:12-16; LAYERS.md L07; #212; PR #219; commit `a41c9e2`; run 38259017355; infra/appinsights.bicep:12"
	tests := []struct {
		name, md string
		want     string // "" passes
	}{
		{"cited claim", "The `Tombstone` struct has no kind (`internal/ledger/tombstone.go:12-16`).", ""},
		{"planted uncited architectural claim", "The `Tombstone` struct has no kind (`internal/ledger/tombstone.go:12-16`).\n\nThe ledger package already stores the kind elsewhere, so the migration is small.", "uncited claim"},
		{"invented path", "The migration lives in `internal/ledger/migrate.go:3`.", "cites internal/ledger/migrate.go, which the grounding does not contain"},
		{"invented issue", "The estimate miss is tracked in #300.", "cites #300"},
		{"commit and run", "The non-bot commit `a41c9e2` counted as a correction; see run 38259017355.", ""},
		{"question is no claim", "Which layer owns the record of a purge?", ""},
		{"header row is no claim", "| Layer | Risk |\n|---|---|\n| L07 | the migration (`LAYERS.md`) |", ""},
		{"labelled analogy beside its evidence", "Analogy: a receipt without the items.\n\nThe tombstone lacks the kind (#212).", ""},
		{"analogy alone", "Analogy: a receipt without the items.\n\nWe should talk soon.", "illustration without the cited point"},
		{"cites nothing", "Nice work, team.", "cites nothing at all"},
		// D-5.1d: the false positives of tier-2 runs 38682959916 and 38685472745, each with its other side.
		{"template placeholder is no claim", "File the retro note under `retro/<date>/` after each plan review.", ""},
		{"placeholder is no evidence for a real claim", "File it under `retro/<date>/`.\n\nThe migration is small, so the plan holds.", "uncited claim"},
		{"a real but ungrounded path still fails", "The plan has no rollback (`ledger/plans/212/plan.json`).", "cites ledger/plans/212/plan.json"},
		{"the twin's own charter", "My habits come from `seats/engineering-manager/CHARTER.md`, and the plan for #212 follows them.", ""},
		{"the twin's own persona.toml", "The stage rule is in `seats/quality-architect/persona.toml`; the floor still holds (#212).", ""},
		{"not a seat's charter", "See `seats/janitor/CHARTER.md` for the plan.", "cites seats/janitor/CHARTER.md"},
		{"another file under a seat", "See `seats/engineering-manager/notes.md` for the plan.", "cites seats/engineering-manager/notes.md"},
		{"bold option heading", "**A. Add `Kind` to `Tombstone`.**\n\nThe struct has no kind (`internal/ledger/tombstone.go:12-16`).", ""},
		{"bold label then a claim", "**A. Add `Kind` to `Tombstone`.** Every tombstone needs a migration.", "uncited claim"},
		{"short lead-in", "When a criterion changes stored state in L07, the plan includes:\n\n- an L07 checkpoint (#212)", ""},
		{"long claim ending with a colon", "The ledger package already stores the kind of every purged object in a separate migration file, as follows:", "uncited claim"},
		// Decision #283: a point marked as not evidenced is not stated as fact; its citations are still checked.
		{"marked assumption", "Assumption: the latency budget is two seconds until the human sets one.\n\nThe tombstone lacks the kind (#212).", ""},
		{"marked in code or bold", "`Assumption:` the latency budget is two seconds.\n\n**Unverified:** the cost is low.\n\nThe tombstone lacks the kind (#212).", ""},
		{"marked unverified", "Unverified: the ledger may hold 40 000 tombstones.\n\nThe tombstone lacks the kind (#212).", ""},
		{"unmarked assumption", "We assume the latency budget is two seconds.\n\nThe tombstone lacks the kind (#212).", "uncited claim"},
		{"a marked point still cites only real sources", "Assumption: the migration in #300 is small.\n\nThe tombstone lacks the kind (#212).", "cites #300"},
		// D-5.1g.j: infrastructure files are sources too, resolved like any other path.
		{"an infrastructure file", "Events are kept for 90 days, with no per-person delete shown (`infra/appinsights.bicep:12`).", ""},
		{"an invented infrastructure file", "Events are kept for 90 days (`infra/logs.bicep:3`).", "cites infra/logs.bicep"},
		{"code fences are evidence quotes", "The struct (#212):\n\n```go\ntype Tombstone struct{ SHA string } // the migration\n```", ""},
	}
	for _, tt := range tests {
		got := strings.Join(CheckGrounding(tt.md, grounding), "\n")
		switch {
		case tt.want == "" && got != "":
			t.Errorf("%s: unexpected findings: %s", tt.name, got)
		case tt.want != "" && !strings.Contains(got, tt.want):
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestCheckStage: the stage's artefact heading is required, another stage's
// is refused, and a startup answer stays under the one-page bound.
func TestCheckStage(t *testing.T) {
	pr, _ := repoPersonas(t)[verbs.EngineeringManager].Practice("delivery-review")
	long := "# Retro note\n\n" + strings.Repeat("word ", StartupMaxWords+20) + "\n"
	tests := []struct {
		name, md, stage, want string
	}{
		{"startup artefact", "# Retro note: #212\n\nShort.", config.StageStartup, ""},
		{"enterprise artefact", "# Retrospective memo: #212\n\nLong.", config.StageEnterprise, ""},
		{"missing artefact", "# Notes\n\nShort.", config.StageStartup, `no heading for the startup artefact "Retro note"`},
		{"ceremony at a startup", "# Retro note\n\n## Retrospective memo\n\nx", config.StageStartup, `carries the enterprise artefact "Retrospective memo"`},
		{"over one page", long, config.StageStartup, "a startup gets the one-page version"},
		{"headings and tables are not body prose", "# Retro note\n\n| a | b |\n|---|---|\n| " + strings.Repeat("cell ", 50) + "|\n\n" + strings.Repeat("word ", StartupMaxWords) + "\n", config.StageStartup, ""},
		{"body prose over the bound", "# Retro note\n\n" + strings.Repeat("word ", StartupMaxWords+1) + "\n", config.StageStartup, "401 words"},
		{"long is fine at enterprise", strings.Replace(long, "Retro note", "Retrospective memo", 1), config.StageEnterprise, ""},
	}
	for _, tt := range tests {
		got := strings.Join(CheckStage(tt.md, pr, tt.stage), "\n")
		switch {
		case tt.want == "" && got != "":
			t.Errorf("%s: unexpected findings: %s", tt.name, got)
		case tt.want != "" && !strings.Contains(got, tt.want):
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
