# UX design: #897 metered, risk-routed merge

The person sees the risk line on the proof comment, the factory meter on Initiative #870, and Bugs filed after round 3.
Agents see the tier and the review path. Nothing of this Feature is built.

## Today (delivered building blocks)

- The proof comment opens with a risk line: a tier, its reasons, and up to five files to read first
  (`internal/inspect/risk.go`, `Risk`, `riskLook = 5`; caller `internal/cli/proof.go:728`).
- `aayu measure --metric aayu:founder_minutes_per_pr` reads a person's minutes per merged PR from the actions log
  (`aayu measure --help` on [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b)).
- `aayu status --streak` computes the autonomy streak of every layer from merged-PR history (`aayu status --help`).
- `aayu ship <pr>` squash-merges after a green gate and writes a ship record (`aayu ship --help`;
  `internal/cli/ship.go`, `ShipRecord`).

## Planned (planned, not built)

| Moment | Who | What they see (from #897, #902, #910) |
|---|---|---|
| A PR opens | Builder and person | The risk line shows the new tier and each rule that matched, with its paths. |
| Review | Builder | One review from the configured reviewer, at every tier. At most 3 rounds. |
| After round 3 | Person | A P2 or P3 is filed as one Bug; the PR can merge. A P0 or P1 follows the proof rule (#899). |
| A PR merges | Nobody acts | A workflow writes one record line on the ledger. |
| Each night | Nobody acts | A sweep back-fills missing records; empty fields show as gaps. |
| The founder checks progress | Person | Initiative #870 shows the meter: merged per 28 days, fix-round hours, a person's wait, rework and reverts in 7 days, human touches per PR, cost or tokens per PR and per story. Each number shows its source, unit, and age. |
| Rollout | Person | 2 weeks silent, then visible, then enforced. A config kill switch turns it off. |

The issue gives no screen layout or text for the meter.

## Agent-facing surface

- The merge record is the data source for #900's routing evidence and #898's meter read tool.
- Agent PRs must carry `Aayudha-Host`, `Aayudha-Model`, and `Aayudha-Effort` lines (`AGENTS.md`), or the record has
  gaps (#897 rule: a missing field stays empty).

## Accessibility

No new terminal screen. The meter is on a GitHub issue; its Markdown must keep tables readable. Inferred.
