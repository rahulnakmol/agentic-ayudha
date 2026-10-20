package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// The builder declaration aayu build writes reads back the same through every
// reader: verify's and counsel's trailer readers, the streak's agent-commit
// rule, and inspect's builder family. The model line stays one model ID per
// line and Aayudha-Effort is never read as a model, so an Amp thread on
// claude-opus-5-5 at medium is Anthropic, not an unplaceable
// "claude-amp-medium" or "anthropic/claude-opus-5-5".
func TestDeclarationsReadBack(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "hosts", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	usage, thread := read("amp-usage.txt"), read("amp-thread.txt")
	for _, c := range []struct {
		name     string
		trailers string // as build writes them, after the checkpoint lines
		host     string
		models   []string
		family   verbs.Family // the builder's family; "" when the models name several
		families []verbs.Family
		reads    string
	}{
		{
			name:     "amp",
			trailers: modelTrailers(hosts.AmpAgentModels(usage)) + "\nAayudha-Effort: " + hosts.AmpAgentMode(thread) + "\nAayudha-Host: amp",
			host:     "amp", models: []string{"claude-opus-5-5"},
			family: verbs.FamilyAnthropic, families: []verbs.Family{verbs.FamilyAnthropic},
			reads: "Claude Opus 5.5, medium effort",
		},
		{
			name:     "copilot, two families",
			trailers: "\n" + hosts.CopilotModels{Models: []string{"claude-opus-5.5", "gpt-6-luna"}}.Trailers() + "\nAayudha-Effort: high",
			host:     "copilot", models: []string{"claude-opus-5.5", "gpt-6-luna"},
			families: []verbs.Family{verbs.FamilyAnthropic, verbs.FamilyOpenAI},
			reads:    "Claude Opus 5.5 and gpt-6-luna, high effort",
		},
		{
			name:     "codex",
			trailers: modelTrailers([]string{verbs.ModelGPTSol}) + "\nAayudha-Effort: xhigh" + "\nAayudha-Host: codex",
			host:     "codex", models: []string{"gpt-6.1-sol"},
			family: verbs.FamilyOpenAI, families: []verbs.Family{verbs.FamilyOpenAI},
			reads: "GPT-6.1 Sol, xhigh effort",
		},
	} {
		// A commit outside a checkpoint, so the streak's rule reads the model lines.
		msg := "fix: a\n\n" + strings.TrimPrefix(c.trailers, "\n")

		models, host, named := declared(msg)
		if !slices.Equal(models, c.models) || host != c.host || !named {
			t.Errorf("%s: counsel reads %q on %q (%v), want %q on %q\n%s", c.name, models, host, named, c.models, c.host, msg)
		}
		if m := modelTrailer.FindStringSubmatch(msg); m == nil || m[1] != c.models[0] {
			t.Errorf("%s: verify reads the builder model %q, want %q", c.name, m, c.models[0])
		}
		if got := verbs.BuilderName(models, effortOf(msg)); got != c.reads {
			t.Errorf("%s: reads %q, want %q", c.name, got, c.reads)
		}

		p := autonomy.PR{Number: 1, Author: "founder", Labels: []string{autonomy.LabelAgentAuthored}, MergedAt: time.Now(),
			Commits: []autonomy.Commit{{SHA: "c1", Login: "founder", Message: msg, Parents: 1}}}
		if cs := (autonomy.Rules{}).Detect([]autonomy.PR{p}, nil); len(cs) != 0 {
			t.Errorf("%s: the streak counts the agent's commit as a correction: %+v", c.name, cs)
		}

		// The build record names one model only when all are of one family.
		rec := inspect.Records{Checkpoints: map[string]string{"c1": oneFamily(models)}}
		b := inspect.BuilderOf(rec, []string{"c1"}, models, nil, "")
		if b.Family != c.family || !slices.Equal(b.Candidates, c.families) {
			t.Errorf("%s: inspect places the builder in %q of %q, want %q of %q (%s)", c.name, b.Family, b.Candidates, c.family, c.families, b.Source)
		}
	}
}

// effortOf is the Aayudha-Effort value of msg, as a reader renders it.
func effortOf(msg string) string {
	for _, l := range strings.Split(msg, "\n") {
		if v, ok := strings.CutPrefix(l, "Aayudha-Effort: "); ok {
			return v
		}
	}
	return ""
}
