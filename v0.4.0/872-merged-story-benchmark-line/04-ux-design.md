# UX design: one benchmark line per merged story

This Feature has no new command, flag, or screen (#872 Scope). The delivery plan view is out of scope (S17, #872
Scope, Out).

## Agent-facing surface

- `aayu ship <pr>` and `aayu release` keep their flags (`aayu ship --help`). Planned: each writes one more ledger
  line. The terminal output of either is not specified to change.
- The lines are data for other verbs: `audit`, `retro` (inferred from #872 Expected paths), and Feature #886's
  estimates (#886 Scope).

## Person-facing effect (later)

- Approve to build shows learned time and cost once #886 reads the lines (#886 Summary). Today it says "no cost
  estimate from #886 yet" (`internal/cli/mission.go:895`).
