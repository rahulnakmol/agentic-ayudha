# rahulnakmol/agentic-ayudha#989: Measure founder minutes per merged PR

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `8e27bc680c58b3d2599ff9ba4855063c551f2a4d654ee26730777a35d06bb9a2` |
| Completion Goal hash | `0c8ff6dc657e01105cbb06184b920382b0eb3c7dca36707487edc76ec34dbdf8` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/989/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the actions log with human-identity records at 09:00, 09:04, 09:09 and 11:30, and 12 pull requests merged into dev that day; `[outcomes] session_gap_minutes = 10` and `[outcomes] action_minutes = 2` at their defaults; expected: two sessions (09:00–09:09 and 11:30), 9 + 2 + 2 = 13 founder minutes, and 1.1 minutes per merged PR
2. [api-trace] state: `aayu measure <goal> --metric aayu:founder_minutes_per_pr --from 2026-10-01 --to 2026-10-29`; expected: the Result names the population (human-identity actions in the actions log and pull requests merged into dev), the window, the two keys, and the value
3. [api-trace] state: a window with no merged pull request; expected: “no merged pull request in the window; founder minutes per merged PR is undefined”, and no Result written
4. [api-trace] state: bot and human-fallback records in the same log; expected: only human-identity records count

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want aayu to measure how many of my minutes each merged pull request costs, so that 0.4.0 can show it spends less of my time per useful change.

Feature #982 (outcomes over output, Decision #981).

### Completion Goal

1. [api-trace] state: the actions log with human-identity records at 09:00, 09:04, 09:09 and 11:30, and 12 pull requests merged into dev that day; `[outcomes] session_gap_minutes = 10` and `[outcomes] action_minutes = 2` at their defaults; expected: two sessions (09:00–09:09 and 11:30), 9 + 2 + 2 = 13 founder minutes, and 1.1 minutes per merged PR
2. [api-trace] state: `aayu measure <goal> --metric aayu:founder_minutes_per_pr --from 2026-10-01 --to 2026-10-29`; expected: the Result names the population (human-identity actions in the actions log and pull requests merged into dev), the window, the two keys, and the value
3. [api-trace] state: a window with no merged pull request; expected: “no merged pull request in the window; founder minutes per merged PR is undefined”, and no Result written
4. [api-trace] state: bot and human-fallback records in the same log; expected: only human-identity records count

### Measure

Windows in which `aayu measure` cannot report founder minutes per merged PR from the actions log.

### Threshold

0

### Scenario

The founder measures the new 0.4.0 Goal and sees founder minutes per merged PR for the last 28 days.

### Environment

`internal/measure` tests with a seeded actions log and a seeded list of merged pull requests; seeded data only.

### Links



### Checkpoints

1. Sessions from the actions log (criterion 1)
2. The Result with its population (criterion 2)
3. Empty windows and identities (criteria 3, 4)

### Assumptions

The measure is a proxy: it counts time in aayu's recorded human actions, not time spent reading GitHub elsewhere. Each session counts its span plus `[outcomes] action_minutes` for its last action; a gap over `[outcomes] session_gap_minutes` starts a new session. Both are new keys with rows in `internal/config/gen`.
This story is made Ready only after Decision #981 is accepted and Feature #982 passes Approve to build (#877). A story added after that, or a change to its Completion Goal, effort, or title, needs the founder's new acknowledgment (Decision #903).

### Open questions

answered: none open; the founder chose O1–O4 A and J2 A on 2026-10-24

### Parked ideas

Time a person spends reading GitHub outside aayu

### Provenance

Request: "will those 300 things actually make a difference? Are they going to actually make happier customers … what really is our core competency? How can we deliver on that better?"
1. stated (request/J2): "J2 A": founder minutes per merged PR
2. stated (request/J2): "J2 A": founder minutes per merged PR
3. default (outcomes): a measure with no merged pull request has no value
4. default (identity): only a person's actions are a person's minutes
````
