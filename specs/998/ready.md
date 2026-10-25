# rahulnakmol/agentic-ayudha#998: aayu mission next <f> lists the mission's startable stories in order, with no model call

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `c5d356dc4ca68117444eb8bdc3ad2c6c70a04d1957954a66cfbfa3626556bbc4` |
| Completion Goal hash | `991e8804ceaf51f22ef0bad7b155dacfdd821eef11985e115e71c177ccfbe06e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/998/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: mission #889 with agents 2 and none running; Ready stories #20 (P1, slice 1), #21 (P1, slice 2), and #22 (P2); expected: `aayu mission next 889` prints `start #20`, `start #21`, then `waits #22: no free agent (2 of 2 busy after these starts)`
2. [api-trace] state: #23 is status:blocked, and #24's host has a usage pause until 20:10; expected: the list shows `waits #23: blocked` and `waits #24: amp paused until 20:10`, and neither is a start
3. [api-trace] state: a Ready story #30 of another Feature; expected: the list for #889 never names #30
4. [api-trace] state: two Kanha runs for #889 ask at the same time, with 1 free agent; expected: one run says `start #20`, the other says `waits #20: claimed by another run`, and #20 starts once
5. [api-trace] state: the repository is halted, or this machine's breaker is paused; expected: the command exits 4 or 6 and lists nothing to start
6. [api-trace] state: the list marked #20 start, and #20 became status:blocked before the Kanha acted; expected: `aayu build 20 --delegate <tool>` reads the live state first and prints `#20 changed since the list: now blocked; not started`
7. [api-trace] state: Feature #889 has no approval at its current hashes; expected: exit 4 with `#889 is not approved to build`
8. [api-trace] state: #889's approved budget is 660 USD, its measured spend is 620 USD, and Ready story #25 has a per-story cap of 60 USD; expected: the list shows `waits #25: would pass #889's approved budget (620 + 60 > 660 USD)`, and #25 is not a start
9. [api-trace] state: the Tatkal Feature #920 of Release #906 was approved once with its ad hoc budget, and story #930 joined it and became Ready an hour ago, with no new Approve to build and no #903 ack; expected: the next list for #920 marks #930 start
10. [api-trace] state: Ready Bug #26 is a sub-issue of #889 in milestone MS1, next to Ready story #27; expected: `aayu mission next 889` lists both, by the same order, as `start #26 (bug)` and `start #27`
11. [api-trace] state: Feature #874's mission is done, and Ready Bug #940 has parent #874 in Release #906; expected: `aayu mission next 874` lists nothing, and the list for #906's Tatkal Feature #920 marks #940 start
12. [api-trace] state: `aayu mission next 889 --json` with #20 to start and #22 waiting; expected: `{"schema": "aayudha.work/v1", "scope": {"kind": "feature", "id": "889"}, "items": [{"key": "#20", "action": "start"}, {"key": "#22", "action": "waits", "reason": "no free agent"}]}`, with `subthread` set on an item that already has one
13. [api-trace] state: `aayu mission next 874` runs on claude, and #874's recorded tool is codex; expected: it is refused with `#874's mission runs on codex, the tool of @bob, who started it: run this there`, and it lists nothing to start

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want aayu to say which stories of a mission can start now, in order, so that Kanha starts only those.

### Completion Goal

1. [api-trace] state: mission #889 with agents 2 and none running; Ready stories #20 (P1, slice 1), #21 (P1, slice 2), and #22 (P2); expected: `aayu mission next 889` prints `start #20`, `start #21`, then `waits #22: no free agent (2 of 2 busy after these starts)`
2. [api-trace] state: #23 is status:blocked, and #24's host has a usage pause until 20:10; expected: the list shows `waits #23: blocked` and `waits #24: amp paused until 20:10`, and neither is a start
3. [api-trace] state: a Ready story #30 of another Feature; expected: the list for #889 never names #30
4. [api-trace] state: two Kanha runs for #889 ask at the same time, with 1 free agent; expected: one run says `start #20`, the other says `waits #20: claimed by another run`, and #20 starts once
5. [api-trace] state: the repository is halted, or this machine's breaker is paused; expected: the command exits 4 or 6 and lists nothing to start
6. [api-trace] state: the list marked #20 start, and #20 became status:blocked before the Kanha acted; expected: `aayu build 20 --delegate <tool>` reads the live state first and prints `#20 changed since the list: now blocked; not started`
7. [api-trace] state: Feature #889 has no approval at its current hashes; expected: exit 4 with `#889 is not approved to build`
8. [api-trace] state: #889's approved budget is 660 USD, its measured spend is 620 USD, and Ready story #25 has a per-story cap of 60 USD; expected: the list shows `waits #25: would pass #889's approved budget (620 + 60 > 660 USD)`, and #25 is not a start
9. [api-trace] state: the Tatkal Feature #920 of Release #906 was approved once with its ad hoc budget, and story #930 joined it and became Ready an hour ago, with no new Approve to build and no #903 ack; expected: the next list for #920 marks #930 start
10. [api-trace] state: Ready Bug #26 is a sub-issue of #889 in milestone MS1, next to Ready story #27; expected: `aayu mission next 889` lists both, by the same order, as `start #26 (bug)` and `start #27`
11. [api-trace] state: Feature #874's mission is done, and Ready Bug #940 has parent #874 in Release #906; expected: `aayu mission next 874` lists nothing, and the list for #906's Tatkal Feature #920 marks #940 start
12. [api-trace] state: `aayu mission next 889 --json` with #20 to start and #22 waiting; expected: `{"schema": "aayudha.work/v1", "scope": {"kind": "feature", "id": "889"}, "items": [{"key": "#20", "action": "start"}, {"key": "#22", "action": "waits", "reason": "no free agent"}]}`, with `subthread` set on an item that already has one
13. [api-trace] state: `aayu mission next 874` runs on claude, and #874's recorded tool is codex; expected: it is refused with `#874's mission runs on codex, the tool of @bob, who started it: run this there`, and it lists nothing to start

### Measure

Stories started by a Kanha that the list did not mark start, per 28 days.

### Threshold

0

### Scenario

A merge frees one agent at 14:02. The Kanha asks aayu and starts exactly the one story aayu marks start.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and seeded issues; never production.

### Links

depends-on #997

### Assumptions

#### Notes

Depends on, outside this slice: #877 S1, #877 S2, #915.

The budget filter moves here from #886 S11 (886-Q6 A), so #886 no longer holds that criterion. A story counts at its per-story cap until #872 measures spend per story. The hold on approval needs #877 S1 and S2, not all of #877 (D1 A). The list reads the Feature's current stories on every run, not a copy taken at approval, so a Tatkal story that joins mid-Release appears; whether it may be Ready is #877 S8's rule. Under M1 this command becomes `aayu mission next <f>`; the per-mission running, next, and waiting view is #895's. The `aayudha.work/v1` list is the seam with #911: `aayu loop --due` returns the same shape with scope `repo` and actions `wake` or `waits`, so one Kanha skill reads both.

#### Review guidance

check: The order is priority, then slice order, then Ready time; a gate never holds a start, because merges wait instead (M9 A, ST16) (Decision #915 (M1), F3). The claim uses the ledger claim line and the lease, as `admitClaim` does (buildnext.go:261-330). Bugs are already buildable (`buildable`, internal/cli/buildnext.go:364-368); a done Feature's Bug goes to its Release's Tatkal mission.
ignore: The wording of each waits reason.
risk: Two runs start one story twice. A story outside the Feature leaks into the list.

#### Behaviours

1. [authority] says: aayu decides what can run; the Kanha never starts a story outside the list; source: Decision #915 (M1), F3; criteria: 1, 3
2. [never] says: aayu makes the list with no model call; source: core design G18 (A-dispatch); Decision #915 (M1), F3; criteria: 1
3. [never] says: Two Kanha runs never start one story twice; source: `admitClaim`, buildnext.go:261-330; criteria: 4
4. [limit] says: Nothing starts while the repository is halted or the breaker is paused; source: `checkHalt`, halt.go:105; `Breaker.Paused`, gateway.go:156-160; criteria: 5
5. [never] says: Kanha acts only on the live state, read just before it starts a story; source: J4 rule 3 (mission-ux page, section 5); criteria: 6
6. [authority] says: Only the mission's recorded tool reads its next list; source: R9 per person, @rahulnakmol 2026-10-23; Tatkal R9 (#877); criteria: 13
````
