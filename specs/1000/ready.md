# rahulnakmol/agentic-ayudha#1000: [build] agents = 5 per Kanha, and the mission limits are config keys

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `2025090bf2f91e4fa27617e371746ff9925450c67c0dd339d31a3c50a1850ce7` |
| Completion Goal hash | `fd1cc14431dfa892ae2c7924b4eed4f59809f6144d8321e51f87d1ead6164f04` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1000/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: no `[build] agents` in any config; expected: the list for #889 starts at most 5 stories at a time
2. [api-trace] state: the committed config on dev says `agents = 5`, and `AAYU_BUILD__AGENTS=10` is set locally; expected: the list uses 5; a local value only lowers it
3. [api-trace] state: @rahulnakmol runs `aayu ready 889 --agents 3` as part of Approve to build; expected: specs/889/approved.json has `agents: 3`, and the list for #889 uses 3 while other Features use the key
4. [api-trace] state: a wake's prompt text says `agents 10` and `budget 2000`; expected: the list still uses the committed `[build] agents` (5) and the approved budget; a prompt never sets a limit
5. [api-trace] state: the committed config still sets `[build] in_flight = 2`; expected: `aayu config validate` says `[build] in_flight is replaced by [build] agents (Y1)`
6. [api-trace] state: `go generate ./...` after the change; expected: CONSTRAINTS.md lists `[build] agents = 5`, `[mission] max_no_go = 3`, `retry_max = 2`, and `retry_backoff_minutes = 10`, `check_minutes = 5`, `warn_hours = 24`, `max_hours = 48`, and none of in_flight, target_prs_month, in_flight_max, auto_default_hours, or review_hold_ratio

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want one number for how many stories each Kanha runs at once, so that I set the factory's pace with one key.

### Completion Goal

1. [api-trace] state: no `[build] agents` in any config; expected: the list for #889 starts at most 5 stories at a time
2. [api-trace] state: the committed config on dev says `agents = 5`, and `AAYU_BUILD__AGENTS=10` is set locally; expected: the list uses 5; a local value only lowers it
3. [api-trace] state: @rahulnakmol runs `aayu ready 889 --agents 3` as part of Approve to build; expected: specs/889/approved.json has `agents: 3`, and the list for #889 uses 3 while other Features use the key
4. [api-trace] state: a wake's prompt text says `agents 10` and `budget 2000`; expected: the list still uses the committed `[build] agents` (5) and the approved budget; a prompt never sets a limit
5. [api-trace] state: the committed config still sets `[build] in_flight = 2`; expected: `aayu config validate` says `[build] in_flight is replaced by [build] agents (Y1)`
6. [api-trace] state: `go generate ./...` after the change; expected: CONSTRAINTS.md lists `[build] agents = 5`, `[mission] max_no_go = 3`, `retry_max = 2`, and `retry_backoff_minutes = 10`, `check_minutes = 5`, `warn_hours = 24`, `max_hours = 48`, and none of in_flight, target_prs_month, in_flight_max, auto_default_hours, or review_hold_ratio

### Measure

Kanhas that ran more story subthreads at once than their agents value.

### Threshold

0

### Scenario

@rahulnakmol keeps agents at 5 and starts two Features. Ten story subthreads run, and the plan view shows 2 × 5.

### Environment

go test of internal/config and internal/cli with committed and local configs; never production.

### Links

depends-on #998

### Assumptions

#### Notes

Depends on, outside this slice: #918.

internal/config/ is in `[autonomy] always_person`, so @rahulnakmol ships this PR. `aayu init` asks the value (F2); #892 owns init's questions. @rahulnakmol set the default to 5 (M3). #918 is accepted.

#### Review guidance

check: aayu never writes the config. The committed value caps a local one, as in_flight does today (buildnext.go:382-402). `[build] agents` is the only cap on parallel work (#918, accepted, supersedes #868).
ignore: Docs wording.
risk: internal/config/ is in the narrow set (.aayudha/config.toml:83). A high default starts many paid threads.

#### Behaviours

1. [limit] says: A Kanha never runs more story subthreads than its agents value; source: Decision #915 (M1), F2; criteria: 1, 3
2. [authority] says: A local setting can only lower the committed agents value; source: buildnext.go:382-402; config.go:73; criteria: 2
3. [authority] says: Every wake reads its limits from config, never from its prompt; source: J4 rule 5 (mission-ux page, section 5); criteria: 4
````
