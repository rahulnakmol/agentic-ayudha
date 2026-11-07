# UX design: Approve to build learns time and cost

Status: planned, not built. No UX baseline exists. The surfaces exist today; #886 changes their numbers.

## The person today (884d81b)

- Approve to build: `aayu ready <feature>`, human-only, with `--agents` and `--milestones` (`aayu ready --help`). It
  writes the mission plan (Story #997).
- `aayu mission show`: "print the plan, each number with its source" (`internal/verbs/registry.go:873`, the
  `mission` verb's action input).
- The budget line says the approved budget and "no cost estimate from #886 yet" (`internal/cli/mission.go:895`).
- With no working hours, "the plan has no estimate in hours, and nothing warns or refuses at Start"
  (`internal/cli/missiontime.go:49-51`).
- A mission ask for a fix PR over budget names "the per-story cap, until #872 measures spend"
  (`internal/cli/missionask.go:72`), with options "Raise the budget to … with a reason" and "Keep #n waiting".

## Planned

- The plan shows a cost estimate with a range (`CostEstimate.LowUSD`, `HighUSD`, `internal/cli/mission.go:309-315`)
  and a time estimate in working hours with its basis: learned, default, and the sizes with no history
  (`MissionHours`, `internal/cli/missiontime.go:58-73`). Planned, not built.
- Over `[mission] max_hours` on learned hours alone, Start needs a person's reason; on default hours alone it says
  "no history" (`MissionHours.Mark` comment, `internal/cli/missiontime.go:68-72`). The code exists; it acts only once
  #886 supplies hours.
- Slice and grill may show the estimate (inferred from Expected paths). Not specified.
