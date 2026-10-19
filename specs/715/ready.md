# rahulnakmol/agentic-ayudha#715: install --loops --apply leaves a re-installed loop's old Amp schedule running

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `d7cefaacfc7e4c7a8883bc587e2ada42ffef53b25a26c1ec96b5dcd1c12b7578` |
| Completion Goal hash | `240de5c813ff1107e87cc758755751f890c6c41cc2f1781af553fe8868ce0faa` |
| Technical Owner | DevOps & DevEx Tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/715/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `install --loops --host amp --apply` for a scheduled loop that `loops/active.json` records on thread A; expected: the new setup thread's prompt asks it to call `update_schedule` with thread A and `enabled: false` after its own `set_schedule`, and install's output names thread A as the old schedule being paused
2. [api-trace] state: the same command for a loop that `loops/active.json` does not record, or records with no thread; expected: the prompt asks for no `update_schedule`, and a recorded ID with no thread gets a step to pause that schedule by hand

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

A re-run of `aayu install --loops --host amp --apply` leaves each scheduled loop's older Amp schedule running next to the new one. `ampLoops` (`internal/cli/installloops.go`) starts a new thread per loop, and that thread calls `set_schedule` on itself. Nothing pauses the schedule that `loops/active.json` already records for the loop on an older thread. After the 2026-10-19 install, nine loops (ci-failed, decisions, evolve, housekeeping, release-measure, retro, seat-review, triage-daily, verify-maintain) had two enabled schedules each: the 2026-10-16 ones on threads T-01a144e7-… and the new ones. @rahulnakmol paused the old ones by hand before they fired twice.

### Steps to reproduce

1. In a checkout where `loops/active.json` records a scheduled Amp loop on thread A (for example `schedule <ID> on T-A`), run `aayu install --loops --host amp --apply`.
2. Look at the schedule on thread A.

### Expected behaviour

The new setup thread pauses the schedule on thread A (`update_schedule` with `thread` A and `enabled: false`) after it sets its own, and install says which old schedule it asked to pause.

### Actual behaviour

The schedule on thread A stays enabled, so the loop runs twice. install says nothing about it.

### Completion Goal

1. [api-trace] state: `install --loops --host amp --apply` for a scheduled loop that `loops/active.json` records on thread A; expected: the new setup thread's prompt asks it to call `update_schedule` with thread A and `enabled: false` after its own `set_schedule`, and install's output names thread A as the old schedule being paused
2. [api-trace] state: the same command for a loop that `loops/active.json` does not record, or records with no thread; expected: the prompt asks for no `update_schedule`, and a recorded ID with no thread gets a step to pause that schedule by hand

### Measure

Scheduled Amp loops left with two enabled schedules after a re-install.

### Threshold

0 loops with two enabled schedules after `aayu install --loops --host amp --apply` is run again.

### Scenario

A person re-runs `aayu install --loops --host amp --apply` in a repository whose `loops/active.json` already records Amp schedules.

### Environment

aayu built from `dev` at 3d988a9, in an Amp orb.

### Technical Owner

DevOps & DevEx Tinkerer

### Checkpoints

1. ampLoops reads the loop's recorded Amp entry before overwriting it, and asks the new thread to pause the schedule on the recorded thread (criteria 1)
2. With no recorded thread there is no pause request; a recorded ID with no thread gets a manual step (criteria 2)

### Assumptions

Amp's `update_schedule` pauses another thread's schedule when given `thread` and `enabled: false`; a recorded Amp ID names its thread as `T-…`, as `schedule <ID> on T-…` or a thread URL.

### Provenance

Request: "When it sets up a loop on a new thread while active.json records an older Amp schedule for the same loop, it should pause the old one (update_schedule enabled:false on the recorded thread) and say so, so a re-install never leaves duplicates."
1. default (behaviour): "it should pause the old one (update_schedule enabled:false on the recorded thread)"
2. default (behaviour): "and say so, so a re-install never leaves duplicates"
````
