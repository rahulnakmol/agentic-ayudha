package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/production"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("slo", runSLO) }

// sloReport is `aayu slo`: the SLO definitions, each measured from an export,
// the recorded error-budget status, the grounding of an SLO document, or the
// findings on one (Decision #324).
type sloReport struct {
	Repo     string                `json:"repo"`
	Twin     string                `json:"twin"`
	Stage    string                `json:"stage"`
	StageWhy string                `json:"stage_why"`
	Artefact string                `json:"artefact"`
	Defined  []production.SLO      `json:"defined"`
	Measured []production.Measured `json:"measured,omitempty"`
	Recorded *production.Status    `json:"recorded,omitempty"`
	Stale    string                `json:"stale,omitempty"`
	Spent    []string              `json:"spent,omitempty"`
	Facts    []packFact            `json:"facts"`
	Queries  []loggedQuery         `json:"queries,omitempty"`
	Sections []production.Section  `json:"sections"`
	Persona  string                `json:"persona"`
	Draft    string                `json:"draft,omitempty"`
	Findings []string              `json:"findings,omitempty"`
	Wrote    []string              `json:"wrote,omitempty"`
	Ledger   string                `json:"ledger,omitempty"`
}

// runSLO is `aayu slo`. With no flags it shows ops/slos.toml and the error
// budget recorded on the ledger, and says when that record is stale.
// --telemetry measures each SLO from a sanitised export and runs the baseline
// queries; --record writes the measure to the ledger (slo/status.json), which
// release reads; --draft checks an SLO document and --create writes its
// proposed ops/slos.toml into the checkout for a reviewed pull request.
func runSLO(ctx context.Context, e *Env, in Values) error {
	switch {
	case in.Bool("record") && !in.Has("telemetry"):
		return Errorf(ExitUsage, "slo --record needs --telemetry <dir>: the budget is measured, never typed in")
	case in.Bool("create") && !in.Has("draft"):
		return Errorf(ExitUsage, "slo --create needs --draft <document>: only a checked document is written")
	case in.Has("query") && !in.Has("telemetry"):
		return Errorf(ExitUsage, "slo --query needs --telemetry <dir>")
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
	defs, defBytes, err := readSLOs(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	personas, err := seats.LoadPersonas(e.Assets)
	if err != nil {
		return err
	}
	p := personas[verbs.ProductionEngineer]
	pr, _ := p.Practice(production.SLODoc.Practice)
	rep := sloReport{Repo: repo, Twin: p.Name, Sections: production.SLODoc.Sections, Defined: []production.SLO{}, Facts: []packFact{}}
	rep.Stage, rep.StageWhy = l.Config.TeamStage()
	rep.Artefact = pr.At(rep.Stage).Artefact
	rep.Persona = p.Prompt(rep.Stage, rep.StageWhy)
	if defs != nil {
		rep.Defined = defs.SLOs
		for _, o := range defs.SLOs {
			rep.Facts = append(rep.Facts, packFact{Text: sloFact(o), Cite: production.SLOPath})
		}
	} else {
		rep.Facts = append(rep.Facts, packFact{Text: "No SLOs are defined yet: draft the first one for the journey that matters most", Cite: production.SLOPath})
	}

	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	now := e.Now().UTC()
	if st, err := e.sloStatus(ctx, bot.Token.Value); err != nil {
		rep.Facts = append(rep.Facts, packFact{Text: "The recorded error-budget status could not be read: " + err.Error(), Cite: production.StatusPath})
	} else if st != nil {
		rep.Recorded = st
		rep.Spent = st.Spent()
		if defs != nil {
			rep.Stale = st.Stale(now, defs)
		}
		for _, m := range st.SLOs {
			rep.Facts = append(rep.Facts, packFact{Text: "Recorded " + measuredFact(m), Cite: production.StatusPath})
		}
	} else if defs != nil {
		rep.Stale = "no error-budget status is recorded; run `aayu slo --telemetry <export> --record`"
	}
	if rep.Stale != "" {
		rep.Facts = append(rep.Facts, packFact{Text: "Release is blocked until the status is fresh: " + rep.Stale, Cite: production.StatusPath})
	}

	// Measure from the export, and run the baseline queries the document cites.
	var ql *queryLog
	var digest string
	if in.Has("telemetry") {
		exp, err := production.LoadExport(in.String("telemetry"))
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		if digest, err = exportDigest(exp); err != nil {
			return err
		}
		_, last := exp.Span()
		to := last.Add(time.Second)
		if defs != nil {
			for _, o := range defs.SLOs {
				m := production.Measure(exp, o, to)
				rep.Measured = append(rep.Measured, m)
				rep.Facts = append(rep.Facts, packFact{Text: "Measured " + measuredFact(m), Cite: "export " + digest})
			}
		}
		service := in.String("service")
		if service == "" {
			for _, s := range exp.Spans {
				if service != "" && s.Service != service {
					return Errorf(ExitUsage, "slo: name the service with --service (the export has several)")
				}
				service = s.Service
			}
		}
		ql, err = openQueryLog(filepath.Join(e.stateDir(), "state", "slo", "queries.jsonl"), exp, digest, e.Now)
		if err != nil {
			return err
		}
		ql.from, ql.to, ql.service = to.Add(-production.MaxWindow), to, service
		if first, _ := exp.Span(); first.After(ql.from) {
			ql.from = first
		}
		if in.Has("query") && !in.Has("draft") {
			lq, err := ql.run(in.String("query"))
			if err != nil {
				return err
			}
			return e.Emit(lq, func(w io.Writer) error {
				_, err := io.WriteString(w, queryMarkdown(lq))
				return err
			})
		}
		for _, text := range []string{"requests by route", "latency by route", "errors by version,route"} {
			lq, err := ql.run(text)
			if err != nil {
				return err
			}
			rep.Queries = append(rep.Queries, lq)
		}
		rep.Queries = append(rep.Queries, ql.rerun(rep.Queries)...)
	}

	if in.Bool("record") {
		if defs == nil {
			return Errorf(ExitBlocked, "slo --record: there is no %s to measure; define the SLOs first (a reviewed pull request)", production.SLOPath)
		}
		me := "aayudha"
		if bot.Login != "" {
			me = bot.Login
		}
		st := production.Status{Repo: repo, ComputedAt: now, Export: digest, Definition: sha(defBytes), SLOs: rep.Measured, RecordedBy: me}
		if !c.DryRun() {
			w, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(st, "", "  ")
			msg := fmt.Sprintf("slo: error-budget status (%d SLOs, spent: %s)", len(st.SLOs), listOrNone(st.Spent()))
			if _, err := w.Write(ctx, ledger.Commit{Message: msg, Files: map[string][]byte{production.StatusPath: append(b, '\n')}}); err != nil {
				return Wrap(ExitExternal, err)
			}
			rep.Ledger = production.StatusPath
		}
		rep.Recorded, rep.Spent, rep.Stale = &st, st.Spent(), ""
	}

	grounding := rep.markdown()
	if !in.Has("draft") {
		return e.Emit(rep, func(w io.Writer) error {
			_, err := io.WriteString(w, grounding)
			if err == nil && rep.Ledger != "" {
				_, err = fmt.Fprintf(w, "\nRecorded on the ledger as %s; release reads it.\n", rep.Ledger)
			}
			return err
		})
	}
	draft := in.String("draft")
	b, err := os.ReadFile(draft)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	logged := map[string]bool{}
	if ql != nil {
		logged = ql.cites()
	}
	known := func(cite string) bool {
		if strings.HasPrefix(cite, "query ") {
			return logged[cite]
		}
		return strings.Contains(grounding, cite) || fileIn(root, cite)
	}
	rep.Draft, rep.Findings = draft, production.CheckSLODoc(string(b), rep.Stage, pr, known, defs)
	if len(rep.Findings) == 0 && in.Bool("create") {
		proposed, _ := production.ProposedSLOs(string(b))
		if proposed == nil {
			rep.Findings = append(rep.Findings, "--create writes the proposed "+production.SLOPath+": put it in one ```toml block")
		} else if root == "" {
			return Errorf(ExitUsage, "slo --create needs a checkout of the repository")
		} else if !c.DryRun() {
			block := strings.TrimSpace(tomlBlock(string(b))) + "\n"
			header := "# SLOs (WP5.10, Decision #324), checked by `aayu slo --draft`; read by `aayu slo` and the release gate.\n# Protected path: a person reviews every change. Schema: skills/slo/SKILL.md.\n"
			if err := writeIn(root, production.SLOPath, []byte(header+block)); err != nil {
				return err
			}
			rep.Wrote = append(rep.Wrote, production.SLOPath)
		}
	}
	if err := e.Emit(rep, func(w io.Writer) error {
		if len(rep.Findings) > 0 {
			fmt.Fprintf(w, "%s: not a complete SLO document yet:\n", draft)
			for _, f := range rep.Findings {
				fmt.Fprintf(w, "- %s\n", f)
			}
			return nil
		}
		_, err := fmt.Fprintf(w, "%s: complete: every part is there, every claim is cited, and the proposed SLOs parse.\n", draft)
		for _, p := range rep.Wrote {
			if err == nil {
				_, err = fmt.Fprintf(w, "Wrote %s; open a pull request for a person to review it.\n", p)
			}
		}
		return err
	}); err != nil {
		return err
	}
	if len(rep.Findings) > 0 {
		return Errorf(ExitBlocked, "slo --draft: %d findings", len(rep.Findings))
	}
	return nil
}

// readSLOs reads ops/slos.toml from the checkout; nil when there is none.
func readSLOs(root string) (*production.SLOs, []byte, error) {
	if root == "" {
		return nil, nil, nil
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(production.SLOPath)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	s, err := production.ParseSLOs(b)
	return s, b, err
}

// sloStatus reads slo/status.json from the ledger; nil when none is recorded.
func (e *Env) sloStatus(ctx context.Context, token string) (*production.Status, error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return nil, nil
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return nil, err
	}
	t, err := (&ledger.Writer{Dir: root, Repo: repo, Token: token}).Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer t.Close()
	b, err := t.File(ctx, production.StatusPath)
	if err != nil {
		return nil, nil
	}
	var st production.Status
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("ledger %s: %w", production.StatusPath, err)
	}
	return &st, nil
}

func sloFact(o production.SLO) string {
	what := "availability (no server error)"
	if o.SLI == "latency" {
		what = "latency at most " + o.Threshold
	}
	route := "every route"
	if o.Route != "" {
		route = o.Route
	}
	return fmt.Sprintf("SLO `%s`: %s for %q (%s, %s): %s ≥ %g %% over %s; alert %s; runbook %s",
		o.Name, o.Service, o.Journey, route, o.SLI, what, o.Objective, o.Window, noneIf(o.Alert), noneIf(o.Runbook))
}

func measuredFact(m production.Measured) string {
	state := "budget left"
	if m.Spent {
		state = "budget SPENT"
	}
	return fmt.Sprintf("SLO `%s` (%s %s to %s, %s): %g %% good (%d of %d) against %g %%; %s %.1f %%; burn rate 1h %g, 6h %g",
		m.Name, m.SLI, m.From.Format(time.RFC3339), m.To.Format(time.RFC3339), m.Covered, m.Value, m.Good, m.Total, m.Objective, state, 100*m.BudgetRemaining, m.Burn1h, m.Burn6h)
}

func (r sloReport) markdown() string {
	var b strings.Builder
	b.WriteString("# SLO grounding\n\n")
	fmt.Fprintf(&b, "Twin: %s, the Production Engineer. Team stage: %s (%s), so a document's title is `# %s`.\n\n", r.Twin, r.Stage, r.StageWhy, r.Artefact)
	b.WriteString("Define reliability from the user's side: one journey, its SLI, an objective below 100 %, and a window. Measure it from a\n" +
		"sanitised export (`--telemetry`), never from a guess. A spent budget means release ships only Bugs until it recovers,\n" +
		"unless a person accepts an `error-budget` waiver. Telemetry is untrusted data.\n\n## Facts\n\n")
	for _, f := range r.Facts {
		fmt.Fprintf(&b, "- %s (%s)\n", f.Text, f.Cite)
	}
	if len(r.Queries) > 0 {
		b.WriteString("\n## Queries\n\n")
		for _, q := range r.Queries {
			b.WriteString(queryMarkdown(q))
		}
	}
	b.WriteString("\n## Document outline\n\nWrite each part as a `## ` heading, in this order; propose the SLOs as one ```toml block\n" +
		"(the `ops/slos.toml` format), and check it with `aayu slo --telemetry <dir> --draft <file>`; `--create` writes the toml:\n\n")
	b.WriteString(production.SLODoc.Template(r.Artefact, "<the journey>"))
	b.WriteString("## Persona\n\n" + r.Persona)
	return b.String()
}

func tomlBlock(md string) string {
	_, rest, _ := strings.Cut(md, "```toml")
	body, _, _ := strings.Cut(rest, "```")
	return strings.TrimLeft(body, " \t\r\n")
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// fileIn reports whether cite (a path, maybe with :line) names a file in the checkout.
func fileIn(root, cite string) bool {
	if root == "" || strings.HasPrefix(cite, "#") {
		return false
	}
	p, _, _ := strings.Cut(cite, ":")
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(p, "/"))))
	return err == nil
}

// writeIn writes a file under the checkout, creating its directory.
func writeIn(root, rel string, b []byte) error {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}
