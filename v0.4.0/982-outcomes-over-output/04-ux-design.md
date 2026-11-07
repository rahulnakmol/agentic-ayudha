# UX design: Feature #982

## Status of the design

There is no UX design baseline for #982 on the ledger (`git ls-tree -r --name-only origin/ledger specs/982` lists
intent, spec, tech, and `design.json` only). The Feature says "Its screens join the 0.4.0 UX baseline before it is
approved (prototype v44)" ([#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope). Prototype v44 is
not in the repository, and no 0.4.0 UX baseline exists on the ledger. So the screens below are from the Completion
Goals only. One surface is built: the `aayu measure` metric of #989.

## Delivered: `aayu measure` for founder minutes

Command, from the `measure` verb in `internal/verbs/registry.go`:

```text
aayu measure [issue] --metric aayu:founder_minutes_per_pr [--from YYYY-MM-DD] [--to YYYY-MM-DD]
```

- `issue`: the Goal or Initiative to measure.
- `--metric`: "acquisition, activation, retention, referral, revenue, north_star, aayu:founder_minutes_per_pr (from
  the actions log on the ledger), or `<source>:<metric> [filter]`; default the issue's Metric."
- `--from`: window start, UTC; default 28 days before `--to`. `--to`: window end, exclusive; default today.

The Result detail text, from `measure.Founder` in `internal/measure/founder.go`:

```text
Founder minutes per merged PR: <n> founder minutes in <s> sessions of <a> human-identity actions in the actions log,
over <m> pull requests merged into <dev> (a gap of more than [outcomes] session_gap_minutes = <g> starts a session;
each session adds [outcomes] action_minutes = <x>)
```

The query line names the ledger tip and the GitHub list call (`m.Query` in `founderMinutes`,
`internal/cli/measure.go`). Errors a person can see:

- No merged PR: "no merged pull request in the window; founder minutes per merged PR is undefined" (`measure.Founder`).
- Another `aayu:` metric: "measure: metric …: aayu measures only aayu:founder_minutes_per_pr" (`founderMinutes`).
- A broken actions-log line: `measure: ledger <tip>: <path> line <n>: …`, exit Blocked (`founderMinutes`).

## Planned: grill and the issue forms ([#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984), [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988))

The Feature form gets an Outcome field after Problem, with four prompts: who it helps, the metric, the expected change,
and core competency (#984 c5). The section in the body has four lines (#984 Assumptions):

```text
### Outcome

Who: founders
Metric: aayu:founder_minutes_per_pr
Change: founder minutes per merged PR fall
Core: yes
```

The exact line format above is inferred from "four lines: Who, Metric, Change, Core (yes or no)"; the Story does not
show one.

Planned `aayu grill feature` output:

| State | Output |
|---|---|
| No Outcome, no `outcome:none` | "not ready: the Feature names no outcome: who it helps, the metric it serves, and the expected change. Add ### Outcome, or the label outcome:none with a reason." Exit 4 (#984 c1) |
| `outcome:none` with no Reason | "outcome:none needs a Reason line" (#984 c3) |
| Metric not in the parent chain | "the Outcome metric is not a metric of #870 or its Goal; name one of: github:prs_merged, aayu:founder_minutes_per_pr" (#984 c4) |
| "Core: no" under a Goal with "What we do best" | Passes, and asks `#<n> does not serve what we do best (“…”). Keep it, or say no?`, naming the Product Strategist (#988 c2) |
| Goal with no "What we do best" | `the Goal states no core competency; add “What we do best” to #<goal>` (#988 c4) |

The Goal form gets a "What we do best" field after Summary (#988 c1). `aayu triage <feedback> --decide story` quotes
that line and asks whether the work serves it (#988 c3).

## Planned: Approve to build ([#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985))

- Terminal text of `aayu ready <f>`: "Outcome: founders · founder minutes per merged PR · falls · core", before the
  line with "budget up to $…" (c1).
- With `outcome:none`: "Outcome: none (CI only; no customer sees it)" (c3).
- With neither: `Approve to build is off: #<n> names no outcome. Next: aayu grill feature <n>`, exit 4 (c2).
- Web inbox card at 1280 and 390 px wide: the same Outcome line, in amber when the Feature has none (c4). The web
  inbox does not exist (Feature [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976), not built).
- The terminal Approve to build card exists today (`approveItem` in `internal/cli/inboxapprove.go`). It shows the
  stories and Bugs, the budget, the agents, the milestones, and the high-risk entries, and no Outcome line.

## Planned: the "Did it move?" card ([#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986))

An inbox card in the Decide area, 14 days (by default) after the Feature's last story merges:

```text
Did it move? #982 Outcomes over output
<metric> before and after (from aayu measure)
<Feedback issues that link #982>
Moved · No change · Unknown   (+ one line)
```

The card layout above is inferred from c1 and c2; the Story gives the title and the content, not the layout. With no
metric source the card says "Measure: no source for aayu:founder_minutes_per_pr; record what you saw" (c5).

## Planned: releases and retros ([#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987))

- `aayu release 906`: "20 shipped · 4 moved a metric · 2 no change · 3 unknown · 11 not yet read · 1 no customer
  outcome" (c1).
- `aayu retro`: a table of each shipped Feature with its outcome, its line, and its estimate accuracy (c2).
- Web Releases page and release Tree at 1280 px: the same counts in the header; each Feature node marked moved, no
  change, unknown, not yet read, or none (c3). These screens are #976's ([#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964),
  [#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983)), not built.
- `aayu retro --week` and the web History view: recommendations taken or overridden, estimate accuracy, escaped P0
  and P1 Bugs, outcomes read, and one next step, such as "Next: aayu grill feature 1301 (2 overrides of Drona on its
  stories)" (c4). `retro` has no `--week` flag today (`internal/verbs/registry.go`).

## The now bar of the web inbox

Feature #976 plans "founder minutes per merged PR: not measured yet" in its now bar until #982 adds the measure's
source ([#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c8, c11). The source exists on dev since
[2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53), but the web inbox does not.

## Agent-facing surface

- `aayu measure` is bot identity and exposed as a write (`internal/verbs/registry.go`, `Name: "measure"`), so an agent
  can run the built metric through the CLI, skill, or MCP.
- `aayu grill` is the Product Strategist's verb; the planned checks run there for agents and people alike.
- Recording an outcome (#986) is a person's choice on an inbox card; the inbox is refused for agents except `--list`
  (`plumb("inbox", …)` in `internal/verbs/registry.go`).
