# rahulnakmol/agentic-ayudha#1009: An hourly safety tick wakes each Kanha that missed an event, and syncs cloud PRs

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `01fdc6e20bc108b54348d7d1143dc2e7afce0285601842f60e944905bb6257bb` |
| Completion Goal hash | `57e93346388bfe1ba701b23257fd83feb0bbca1485c284c65679a068a023f7d5` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1009/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: loops/build-ready.toml after the change; expected: `cron = "0 * * * *"`, and `loops.UTCCrons` gives one UTC cron for Europe/London in summer and in winter
2. [api-trace] state: a tick; the Kanha of #889 has a free agent and a runnable story and was not told since the last tick; the Kanha of #870 was told 20 minutes ago; expected: the tick tells the Kanha of #889 once and does not tell #870's
3. [api-trace] state: the Kanha of #889 wakes 3 hours late; expected: it reads every event since `events_cursor` in missions/889/plan.json, not a fixed window, misses none, and moves the cursor only after it has acted
4. [api-trace] state: a tick, and story #51's PR (a Codex receiver) has a new inspect verdict on its head; expected: the tick runs `aayu build 51 --delegate codex` once: it sends the findings, or ships at merge-ready
5. [api-trace] state: `aayu install --loops` after the change; expected: each host's loop file has the hourly schedule and the tick prompt, and `aayu doctor` reports no drift

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want one hourly tick to catch what events missed, so that a lost event costs at most one hour.

### Completion Goal

1. [api-trace] state: loops/build-ready.toml after the change; expected: `cron = "0 * * * *"`, and `loops.UTCCrons` gives one UTC cron for Europe/London in summer and in winter
2. [api-trace] state: a tick; the Kanha of #889 has a free agent and a runnable story and was not told since the last tick; the Kanha of #870 was told 20 minutes ago; expected: the tick tells the Kanha of #889 once and does not tell #870's
3. [api-trace] state: the Kanha of #889 wakes 3 hours late; expected: it reads every event since `events_cursor` in missions/889/plan.json, not a fixed window, misses none, and moves the cursor only after it has acted
4. [api-trace] state: a tick, and story #51's PR (a Codex receiver) has a new inspect verdict on its head; expected: the tick runs `aayu build 51 --delegate codex` once: it sends the findings, or ships at merge-ready
5. [api-trace] state: `aayu install --loops` after the change; expected: each host's loop file has the hourly schedule and the tick prompt, and `aayu doctor` reports no drift

### Measure

Ticks that built a story in their own session, per 28 days.

### Threshold

0

### Scenario

A person merges a story in GitHub's web page at 09:10. The 10:00 tick wakes its Kanha.

### Environment

go test of internal/loops and internal/cli with the repository's loop files; never production.

### Links

depends-on #998
depends-on #1004

### Assumptions

#### Notes

loops/build-ready.toml is in `[autonomy] always_person`, so @rahulnakmol ships this PR.

#### Review guidance

check: The tick never builds and never passes --solo-bypass or --admin. It wakes a Kanha at most once per tick.
ignore: The loop's description text.
risk: loops/build-ready.toml is in the narrow set (.aayudha/config.toml:100). Each tick is a paid agent turn: 24 a day.

#### Behaviours

1. [never] says: The tick only wakes Kanhas and syncs cloud PRs; it never builds a story; source: core design G18; Decision #915 (M1), F6; criteria: 2, 4
2. [order] says: A wake reads events from its cursor, so a late wake misses nothing; source: J4 rule 1 (mission-ux page, section 5); criteria: 3
````
