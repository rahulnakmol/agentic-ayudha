package audit

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Command is the command line that re-reads GitHub with the report's inputs.
func (in Inputs) Command(verb string) string {
	return fmt.Sprintf("aayu %s --repo %s --since %s --until %s --ledger %s", verb, in.Repo, stamp(in.Since), stamp(in.Until), in.Ledger)
}

// Replay is the command line that regenerates the report from its snapshot.
func (in Inputs) Replay(verb string) string {
	return fmt.Sprintf("aayu %s --repo %s --replay %s", verb, in.Repo, in.Snapshot)
}

// header renders the regeneration commands and the inputs table.
func (in Inputs) header(verb string, newest string) string {
	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("Regenerate this report byte for byte, with the same configuration, from the GitHub inputs snapshot written beside it\n")
	p("(GitHub's history is mutable, and rule suites are listed for a month), or re-read GitHub with the same inputs:\n\n")
	p("```sh\n%s\n%s\n```\n\n", in.Replay(verb), in.Command(verb))
	p("## Inputs\n\n| Input | Value |\n|---|---|\n")
	p("| Repository | `%s` |\n| Window | %s to %s (half-open) |\n", in.Repo, stamp(in.Since), stamp(in.Until))
	p("%s%s%s |\n", HeaderLine, stamp(in.Until), newest)
	p("| GitHub inputs snapshot | `%s` (SHA-256 `%s`) |\n", in.Snapshot, in.SnapshotSHA256)
	p("| Ledger commit | `%s` |\n", in.Ledger)
	if verb == "audit" {
		p("| Code commit (licence scan) | `%s` |\n", or(in.Code, "none"))
	}
	p("| Config hash | `%s` |\n| Actions segments | %d |\n\n", in.ConfigHash, len(in.Segments))
	if len(in.Segments) > 0 {
		p("| Segment | SHA-256 | Records |\n|---|---|---|\n")
		for _, s := range in.Segments {
			p("| `%s` | `%s` | %d |\n", s.Path, s.SHA256, s.Records)
		}
		p("\n")
	}
	return b.String()
}

// HeaderLine is the line of a report that records its cursor; the next
// `aayu audit` starts its default window there.
const HeaderLine = "| GitHub event cursor | until "

// Markdown renders the report. It holds no clock reading other than the
// inputs, so the same inputs render the same bytes.
func (r *Report) Markdown() []byte {
	var b bytes.Buffer
	in, c := r.Inputs, r.Counts()
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	p("# Audit report: %s, %s to %s\n\n", in.Repo, stamp(in.Since), stamp(in.Until))
	p("Written by `aayu audit` (docs/BUILD-PLAN.md WP7.6). GitHub's own event history is the source of record (ground rule 6);\n")
	p("the actions segments on the ledger are reconciled against it.\n\n")
	newest := "; newest rule suite none"
	if r.NewestSuite != 0 {
		newest = fmt.Sprintf("; newest rule suite %d", r.NewestSuite)
	}
	b.WriteString(in.header("audit", newest))
	p("## Summary\n\n")
	p("- Rule-suite bypasses: %d; accounted for %d; **unexplained %d**\n", c.Bypasses, c.Bypasses-c.Unexplained, c.Unexplained)
	p("  - solo-mode bypasses (protected-path merges): %d\n  - ledger writes under the founder identity (D1 day-one rule): %d\n  - promotions to rolling or main: %d\n  - tag ruleset bypasses: %d\n",
		c.Solo, c.LedgerWrites, c.Promotions, c.Tags)
	p("- Solo-mode human passes (`ship --solo-bypass`): %d\n", c.SoloShips)
	p("- Human-fallback writes: %d\n", c.Fallback)
	p("- GitHub writes in the window: %d; in an actions segment %d; by workflows and Apps %d; **not in any actions segment %d**\n",
		c.Writes, c.Writes-c.CI-c.Unlogged, c.CI, c.Unlogged)
	p("- Proof coverage: %d/%d delivered stories and bugs; merged without proof (pre-gate) %d; merged without proof %d\n",
		c.Covered, c.Delivered, c.PreGate, c.PostGate)
	p("- Floor misses in authoritative packs: %d; waived floors: %d\n", c.FloorMisses, c.Waivers)
	p("- Decisions over their SLA: %d\n", c.OverSLA)
	p("- Autonomy exceptions: %d corrections; %d agent merges on never-layers\n", c.Corrections, c.NeverMerges)
	p("- Licence findings: %d (%d go.mod modules); CI `security` job on the code commit: %s\n\n", c.Licence, r.Modules, or(strings.Join(r.Security, ", "), "no completed run"))

	p("## Rule-suite bypasses\n\n")
	p("GitHub records every push that a ruleset would have refused but a bypass actor made (`rulesets/rule-suites`, result `bypass`).\n")
	p("A protected-path merge is a solo-mode bypass when only the `protected-paths` check failed and a PR comment starting\n")
	p("\"Solo-mode bypass:\" came first (WP0.1, Appendix B D2). A deploy-key push lists no actor and is identified from the\n")
	p("ledger commit (D1).\n\n")
	for _, class := range []string{ClassSolo, ClassLedger, ClassPromotion, ClassTag, ClassOther} {
		var rows []Bypass
		for _, x := range r.Bypasses {
			if x.Class == class {
				rows = append(rows, x)
			}
		}
		p("### %s (%d)\n\n", title(class), len(rows))
		if len(rows) == 0 {
			p("None.\n\n")
			continue
		}
		p("| Rule suite | Ref | Actor | After | Pushed at | Bypassed rules | Accounted | Explanation |\n|---|---|---|---|---|---|---|---|\n")
		for _, x := range rows {
			note := x.Note
			if x.Comment != "" {
				note += "; " + x.Comment
			}
			if class == ClassLedger {
				note += map[bool]string{true: "; in an actions segment", false: "; not in an actions segment"}[x.Logged]
			}
			p("| %d | `%s` | %s | `%s` | %s | %s | %s | %s |\n", x.ID, x.Ref, or(x.Actor, "(none)"), short(x.After), stamp(x.PushedAt),
				cell(rules(x.Failed)), flag(x.Accounted), cell(note))
		}
		p("\n")
	}

	p("## Solo-mode human passes (`ship --solo-bypass`)\n\n")
	if len(r.SoloShips) == 0 {
		p("None.\n\n")
	} else {
		p("| Story | PR | Merge commit | Shipped at | By | Ship record | Comment |\n|---|---|---|---|---|---|---|\n")
		for _, s := range r.SoloShips {
			p("| #%d | #%d | `%s` | %s | %s | `%s` | %s |\n", s.Story, s.PR, short(s.Merge), stamp(s.ShippedAt), s.Login, s.Path, or(link(s.Comment), "—"))
		}
		p("\n")
	}

	p("## Human-fallback writes\n\n")
	p("Writes made under the human identity because the repository has no bot identity (WP1.4 day-one path).\n\n")
	if len(r.Fallback)+len(r.FallbackLdg) == 0 {
		p("None.\n\n")
	} else {
		p("| At | Target | Write | Actor | Record |\n|---|---|---|---|---|\n")
		for _, w := range r.Fallback {
			p("| %s | %s | %s | %s | %s |\n", stamp(w.At), cell(w.Target), cell(w.Kind), w.Actor, or(codeOr(w.Logged), "issue timeline"))
		}
		for _, e := range r.FallbackLdg {
			p("| %s | ledger `%s` | %s | %s | ledger commit trailer |\n", stamp(e.At), short(e.SHA), cell(e.Subject), cell(e.Author))
		}
		p("\n")
	}

	r.renderWrites(p)
	r.renderProof(p)

	p("## Decision SLA\n\n")
	if len(r.Decisions) == 0 {
		p("No Decisions open or closed in the window.\n\n")
	} else {
		p("| Decision | Open | Business hours | Over SLA | Provisional |\n|---|---|---|---|---|\n")
		for _, d := range r.Decisions {
			p("| #%d %s | %s | %.1f | %s | %s |\n", d.Number, cell(d.Title), yes(d.Open), d.Hours, yes(d.Over), yes(d.Provisional))
		}
		p("\n")
	}

	p("## Autonomy exceptions\n\n")
	if len(r.Corrections)+len(r.NeverMerges) == 0 {
		p("No corrections and no agent merges on never-layers in the window.\n\n")
	} else {
		for _, x := range r.Corrections {
			p("- correction of #%d: %s, %s at %s (%s)\n", x.PR, x.Kind, x.Ref, stamp(x.At), strings.Join(x.Layers, ", "))
		}
		for _, m := range r.NeverMerges {
			p("- agent PR #%d merged on never-layers %s at %s\n", m.PR, strings.Join(m.Layers, ", "), stamp(m.At))
		}
		p("\n")
	}

	p("## Licence scan\n\n")
	p("Every go.mod requirement of the code commit needs a THIRD_PARTY_NOTICES.md row with the same version and an allowed\n")
	p("licence; the CI `security` job runs go-licenses and the npm licence check on the same commit.\n\n")
	if len(r.Licence) == 0 {
		p("No findings.\n\n")
	} else {
		for _, l := range r.Licence {
			p("- %s\n", l)
		}
		p("\n")
	}

	p("## Sources and retention\n\n")
	p("- Rule suites: `GET /repos/{owner}/{repo}/rulesets/rule-suites` (repository admin), listed for at most the last month;\n")
	p("  the GitHub inputs snapshot keeps them, so `--replay` reproduces this report after GitHub stops listing them.\n")
	p("- Issue and pull request timelines, merged pull requests, and workflow runs: GitHub REST, read up to the cursor.\n")
	p("- Ledger: proof packs, ship records, Ready snapshots, actions segments, and earlier audit reports at the ledger commit.\n")
	p("- Earlier reports (hand-run or `aayu audit`) on the ledger account for bypasses they list by rule-suite ID.\n")
	return b.Bytes()
}

type printer func(format string, a ...any)

func (r *Report) renderWrites(p printer) {
	p("## Writes not in any actions segment\n\n")
	p("Each write in GitHub's event history (issue and PR timelines, creations, workflow dispatches, ledger commits) is\n")
	p("matched to a record in an actions segment on the ledger (same request and target, within %s). A write with no\n", Tolerance)
	p("record is a direct `gh` or web write, or an `aayu` run whose segment never reached the ledger. Workflow and App\n")
	p("writes are recorded by their runs.\n\n")
	type group struct {
		target, actor string
		first, last   time.Time
		kinds         map[string]int
	}
	var keys []string
	groups := map[string]*group{}
	for _, w := range r.Writes {
		if w.CI || w.Logged != "" {
			continue
		}
		k := w.Target + "\x00" + w.Actor
		g, ok := groups[k]
		if !ok {
			g = &group{target: w.Target, actor: w.Actor, first: w.At, kinds: map[string]int{}}
			groups[k] = g
			keys = append(keys, k)
		}
		g.last = w.At
		g.kinds[w.Kind]++
	}
	if len(keys) == 0 {
		p("None.\n\n")
	} else {
		p("| Target | Actor | First | Last | Writes |\n|---|---|---|---|---|\n")
		for _, k := range keys {
			g := groups[k]
			var ks []string
			for kind, n := range g.kinds {
				ks = append(ks, fmt.Sprintf("%s ×%d", kind, n))
			}
			sort.Strings(ks)
			p("| %s | %s | %s | %s | %s |\n", cell(g.target), cell(g.actor), stamp(g.first), stamp(g.last), cell(strings.Join(ks, ", ")))
		}
		p("\n")
	}
	logged := map[string]int{}
	ci := map[string]int{}
	for _, w := range r.Writes {
		switch {
		case w.CI:
			ci[w.Actor]++
		case w.Logged != "":
			logged[w.Logged]++
		}
	}
	if len(logged) > 0 {
		p("Writes matched to a record: %s.\n\n", counts(logged))
	}
	if len(ci) > 0 {
		p("Workflow and App writes: %s.\n\n", counts(ci))
	}
}

func (r *Report) renderProof(p printer) {
	p("## Proof coverage\n\n")
	p("Delivered stories and bugs (a done status at the cursor, or a merged PR into the dev branch that references them).\n")
	p("A story is covered by an authoritative pack that maps every Ready criterion, matches its hashes, and whose floors\n")
	p("hold. A story merged before the merge gate existed is \"merged without proof (pre-gate)\": done but unproven, and\n")
	p("still uncovered. The merge gate (`%s`) was registered at %s.\n\n", MergeGateWorkflow, or(stampOr(r.GateSince), "no time before the cursor"))
	if len(r.Stories) == 0 {
		p("No delivered stories or bugs.\n\n")
	} else {
		p("| Story | Status | Merged by | Merged at | Covered | Pack or reason |\n|---|---|---|---|---|---|\n")
		for _, s := range r.Stories {
			pr, at := "—", "—"
			if s.PR != 0 {
				pr, at = fmt.Sprintf("#%d", s.PR), stamp(s.Merged)
			}
			why := s.Reason
			if s.Covered {
				why = "`" + s.Pack + "`"
			}
			p("| #%d %s | %s | %s | %s | %s | %s |\n", s.Number, cell(s.Title), or(s.Status, "—"), pr, at, flag(s.Covered), cell(why))
		}
		p("\n")
	}
	p("## Floors and waivers\n\n")
	if len(r.Packs) == 0 {
		p("No proof packs written in the window.\n\n")
		return
	}
	p("| Pack | Kind | Floors held | Misses | Waived |\n|---|---|---|---|---|\n")
	for _, pk := range r.Packs {
		var held, miss, waived []string
		for _, c := range pk.Checks {
			switch {
			case c.Result == proof.Waived:
				waived = append(waived, c.Floor+" ("+c.Waiver+")")
			case c.Result == proof.Fail || c.Result == proof.Blocked || proof.BlocksShip(c):
				miss = append(miss, c.Floor+": "+c.Result)
			default:
				held = append(held, c.Floor)
			}
		}
		p("| `%s` | %s | %s | %s | %s |\n", pk.Path, pk.Kind, or(strings.Join(held, ", "), "—"), or(strings.Join(miss, ", "), "—"), or(strings.Join(waived, ", "), "—"))
	}
	p("\n")
}

func stampOr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return stamp(t)
}

func counts(m map[string]int) string {
	var ks []string
	for k, n := range m {
		ks = append(ks, fmt.Sprintf("%s %d", codeOr(k), n))
	}
	sort.Strings(ks)
	return strings.Join(ks, ", ")
}

func rules(rs []Rule) string {
	var out []string
	for _, x := range rs {
		s := "`" + x.Type + "` (" + x.Ruleset + ")"
		out = append(out, s)
	}
	return strings.Join(out, ", ")
}

func title(class string) string {
	switch class {
	case ClassSolo:
		return "Solo-mode bypasses (protected-path merges)"
	case ClassLedger:
		return "Ledger writes under the founder identity (D1 day-one rule)"
	case ClassPromotion:
		return "Promotions to rolling or main"
	case ClassTag:
		return "Tag ruleset bypasses"
	}
	return "Other bypasses"
}

func link(u string) string {
	if strings.HasPrefix(u, "https://") {
		return "<" + u + ">"
	}
	return u
}

func cell(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", `\|`), "\n", " ") }

func yes(b bool) string { return map[bool]string{true: "yes", false: "no"}[b] }

// flag is yes/no with a "no" that stands out: a bypass not accounted for, a story not covered.
func flag(b bool) string { return map[bool]string{true: "yes", false: "**no**"}[b] }

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func codeOr(s string) string {
	if s == "" || strings.HasPrefix(s, "(") {
		return s
	}
	return "`" + s + "`"
}
