package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/ops"
	"github.com/rahulnakmol/agentic-ayudha/internal/production"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("runbook", runRunbook) }

// runbookReport is `aayu runbook`: the catalog, the SLOs, and the runbooks
// the Production Engineer twin writes against, and the findings on every
// runbook in the checkout or on one draft (Decision #324).
type runbookReport struct {
	Repo      string               `json:"repo"`
	Twin      string               `json:"twin"`
	Stage     string               `json:"stage"`
	StageWhy  string               `json:"stage_why"`
	Artefact  string               `json:"artefact"`
	Runbooks  []string             `json:"runbooks"`
	Uncovered []string             `json:"slos_without_runbook,omitempty"`
	Facts     []packFact           `json:"facts"`
	Sections  []production.Section `json:"sections"`
	Persona   string               `json:"persona"`
	Draft     string               `json:"draft,omitempty"`
	Findings  map[string][]string  `json:"findings,omitempty"`
	Wrote     string               `json:"wrote,omitempty"`
	// Alerts is each checked runbook's alerts and where they were found.
	Alerts map[string][]production.AlertStatus `json:"alerts,omitempty"`
}

// runRunbook is `aayu runbook`. With no flags it checks every runbook in
// ops/runbooks/ against the catalog (ops/actions.toml) and the SLOs
// (ops/slos.toml) in the checkout, and names each alerting SLO with no
// runbook; --draft checks one entry, and --create writes it to
// ops/runbooks/<name>.md for a pull request a person reviews. Exit 4 on
// findings. It never runs an action.
func runRunbook(ctx context.Context, e *Env, in Values) error {
	if in.Bool("create") && !in.Has("draft") {
		return Errorf(ExitUsage, "runbook --create needs --draft <entry>: only a checked runbook is written")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "runbook needs a checkout of the repository: runbooks live in %s/", production.RunbookDir)
	}
	defs, _, err := readSLOs(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	var cat *ops.Catalog
	if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ops.Path))); err == nil {
		if cat, err = ops.Parse(b); err != nil {
			return Wrap(ExitUsage, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	personas, err := seats.LoadPersonas(e.Assets)
	if err != nil {
		return err
	}
	p := personas[verbs.ProductionEngineer]
	pr, _ := p.Practice(production.Runbook.Practice)
	rep := runbookReport{Repo: repo, Twin: p.Name, Sections: production.Runbook.Sections, Runbooks: []string{}, Facts: []packFact{}}
	rep.Stage, rep.StageWhy = l.Config.TeamStage()
	rep.Artefact = pr.At(rep.Stage).Artefact
	rep.Persona = p.Prompt(rep.Stage, rep.StageWhy)

	if cat == nil || len(cat.Actions) == 0 {
		rep.Facts = append(rep.Facts, packFact{Text: "The ops catalog has no actions: a runbook cannot mitigate anything until a person adds one", Cite: ops.Path})
	} else {
		for _, a := range cat.Actions {
			rep.Facts = append(rep.Facts, packFact{Text: actionFact(a), Cite: ops.Path})
		}
		rep.Facts = append(rep.Facts, packFact{Text: "Release gate: each catalog action named as a rollback needs one successful aayudha-ops run before `aayu release` ships", Cite: ops.Path})
	}
	if defs != nil {
		for _, o := range defs.SLOs {
			rep.Facts = append(rep.Facts, packFact{Text: sloFact(o), Cite: production.SLOPath})
		}
	}
	files, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(production.RunbookDir), "*.md"))
	for _, f := range files {
		rel := path.Join(production.RunbookDir, filepath.Base(f))
		if filepath.Base(f) != "README.md" {
			rep.Runbooks = append(rep.Runbooks, rel)
			rep.Facts = append(rep.Facts, packFact{Text: "Runbook " + rel, Cite: rel})
		}
	}
	if defs != nil {
		rep.Uncovered = defs.Uncovered(rep.Runbooks)
	}
	found, unread, facts := e.providerAlerts(ctx, repo, l.Config.Integrations)
	rep.Facts = append(rep.Facts, facts...)
	grounding := rep.markdown()

	known := func(cite string) bool { return strings.Contains(grounding, cite) || fileIn(root, cite) }
	check := func(action, params string) string {
		if cat == nil {
			return "there is no ops catalog in the checkout"
		}
		a, ok := cat.Find(action)
		if !ok {
			return "not in " + ops.Path
		}
		ps, err := ops.ParseParams(params)
		if err == nil {
			_, err = a.Environment(ps)
		}
		if err == nil && params != "" && !strings.ContainsAny(params, "<>") {
			_, err = a.Render(ps)
		}
		if err != nil {
			return err.Error()
		}
		return ""
	}
	rollback := func(name string) string {
		if a, ok := cat.Find(name); ok {
			return a.Rollback
		}
		return ""
	}
	rep.Alerts = map[string][]production.AlertStatus{}
	checkOne := func(file, md string) []string {
		fs := production.CheckRunbook(md, rep.Stage, pr, known, check, rollback, defs)
		alerts := production.RunbookAlerts(md, defs, found)
		for i, a := range alerts {
			if a.State == production.AlertMissing && unread {
				// A configured provider could not be read: the alert may be there.
				alerts[i].State = production.AlertNotChecked
			} else if a.State == production.AlertMissing {
				fs = append(fs, fmt.Sprintf("the Symptom names alert `%s`, which is not an alert or monitor in %s", a.Name, strings.Join(slices.Sorted(maps.Keys(found)), " or ")))
			}
		}
		if len(alerts) > 0 {
			rep.Alerts[file] = alerts
		}
		return fs
	}

	rep.Findings = map[string][]string{}
	if in.Has("draft") {
		draft := in.String("draft")
		b, err := os.ReadFile(draft)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		rep.Draft = draft
		if fs := checkOne(draft, string(b)); len(fs) > 0 {
			rep.Findings[draft] = fs
		} else if in.Bool("create") {
			name := production.RunbookName(string(b))
			if name == "" {
				rep.Findings[draft] = []string{"--create names the file after the title's subject: give it `# <artefact>: <failure>`"}
			} else {
				rep.Wrote = path.Join(production.RunbookDir, name+".md")
				if err := writeIn(root, rep.Wrote, b); err != nil {
					return err
				}
			}
		}
	} else {
		for _, rel := range rep.Runbooks {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			if fs := checkOne(rel, string(b)); len(fs) > 0 {
				rep.Findings[rel] = fs
			}
		}
		for _, name := range rep.Uncovered {
			rep.Findings[production.SLOPath] = append(rep.Findings[production.SLOPath], fmt.Sprintf("SLO %q alerts but has no runbook in %s/", name, production.RunbookDir))
		}
	}
	if err := e.Emit(rep, func(w io.Writer) error {
		if !in.Has("draft") {
			if _, err := io.WriteString(w, grounding); err != nil {
				return err
			}
		}
		for _, k := range slices.Sorted(maps.Keys(rep.Alerts)) {
			var parts []string
			for _, a := range rep.Alerts[k] {
				parts = append(parts, "`"+a.Name+"` "+string(a.State)+strings.TrimSuffix(" in "+a.Where, " in "))
			}
			fmt.Fprintf(w, "\n%s alerts: %s.\n", k, strings.Join(parts, "; "))
		}
		keys := make([]string, 0, len(rep.Findings))
		for k := range rep.Findings {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "\n%s: not a complete runbook yet:\n", k)
			for _, f := range rep.Findings[k] {
				fmt.Fprintf(w, "- %s\n", f)
			}
		}
		if len(keys) > 0 {
			return nil
		}
		var err error
		switch {
		case rep.Draft != "":
			_, err = fmt.Fprintf(w, "%s: complete: every part is there, every claim is cited, the confirm query is bounded, and the mitigation is a catalog action with its rollback.\n", rep.Draft)
			if err == nil && rep.Wrote != "" {
				_, err = fmt.Fprintf(w, "Wrote %s; open a pull request for a person to review it.\n", rep.Wrote)
			}
		default:
			_, err = fmt.Fprintf(w, "\n%d runbook(s) checked; every alerting SLO has one.\n", len(rep.Runbooks))
		}
		return err
	}); err != nil {
		return err
	}
	if len(rep.Findings) > 0 {
		return Errorf(ExitBlocked, "runbook: findings in %d file(s)", len(rep.Findings))
	}
	return nil
}

func (r runbookReport) markdown() string {
	var b strings.Builder
	b.WriteString("# Runbook grounding\n\n")
	fmt.Fprintf(&b, "Twin: %s, the Production Engineer. Team stage: %s (%s), so a runbook's title is `# %s: <failure>`.\n\n", r.Twin, r.Stage, r.StageWhy, r.Artefact)
	b.WriteString("A runbook confirms a known failure with a bounded query, mitigates it only with a catalog action proposed through\n" +
		"`aayu respond <incident> --action <name>` (a person approves it through the Environment), verifies it, and rolls it back.\n" +
		"It never gives a shell command against production.\n\n## Facts\n\n")
	for _, f := range r.Facts {
		fmt.Fprintf(&b, "- %s (%s)\n", f.Text, f.Cite)
	}
	if len(r.Uncovered) > 0 {
		fmt.Fprintf(&b, "\nAlerting SLOs with no runbook yet: %s.\n", strings.Join(r.Uncovered, ", "))
	}
	b.WriteString("\n## Runbook outline\n\nWrite each part as a `## ` heading, in this order, and check it with `aayu runbook --draft <file>`;\n" +
		"`--create` writes it to `ops/runbooks/<name>.md` for a reviewed pull request:\n\n")
	b.WriteString(production.Runbook.Template(r.Artefact, "<the failure>"))
	b.WriteString("## Persona\n\n" + r.Persona)
	return b.String()
}

// providerAlerts lists the alerts and monitors of each configured Sentry and
// PostHog project, for the check of the alerts a runbook names (Decision
// #449, phase P1). A provider that is not configured, or cannot be read with
// a read-only key, is left out, so its alerts are not checked rather than
// missing; the facts say why, and where each list came from. unread reports
// a configured provider that could not be read: an alert the others lack may
// be in it, so no alert is missing until every configured provider answers.
func (e *Env) providerAlerts(ctx context.Context, repo string, configured map[string]config.Integration) (found map[string][]string, unread bool, facts []packFact) {
	found = map[string][]string{}
	d := e.integrationDeps(repo)
	for _, name := range []string{"sentry", "posthog"} {
		if _, ok := configured[name]; !ok {
			facts = append(facts, packFact{Text: fmt.Sprintf("[integrations.%s] is not set: the alerts runbooks name are not checked in %s", name, name), Cite: production.RunbookDir})
			continue
		}
		names, pv, err := listAlerts(ctx, d, name, configured[name].Project)
		if err != nil {
			unread = true
			facts = append(facts, packFact{Text: fmt.Sprintf("The alerts runbooks name are not checked in %s: %v", name, err), Cite: production.RunbookDir})
			continue
		}
		found[name] = names
		facts = append(facts, packFact{Text: fmt.Sprintf("%d alerts and monitors in %s (%s; sha256 %s)", len(names), pv.Source, strings.Join(pv.Requests, ", "), pv.Digest), Cite: pv.Source})
	}
	return found, unread, facts
}

// listAlerts reads one provider's alert list after checking its key reads
// only: a key the provider reports as able to write is refused before the list.
func listAlerts(ctx context.Context, d providers.Deps, name, project string) ([]string, production.Provenance, error) {
	refuse := func(sc integrations.Scope, err error) error {
		if err == nil && len(sc.Write) > 0 {
			err = fmt.Errorf("the key can write (%s); aayudha reads with read-only keys only", strings.Join(sc.Write, ", "))
		}
		return err
	}
	if name == "sentry" {
		c, err := providers.Sentry(d)
		if err == nil {
			err = refuse(c.Scope(ctx))
		}
		if err != nil {
			return nil, production.Provenance{}, err
		}
		return production.Sentry{API: c}.Alerts(ctx)
	}
	c, err := providers.PostHog(d)
	if err == nil {
		err = refuse(c.Scope(ctx))
	}
	if err != nil {
		return nil, production.Provenance{}, err
	}
	return production.PostHogAlerts(ctx, c, project)
}
