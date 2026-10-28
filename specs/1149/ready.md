# rahulnakmol/agentic-ayudha#1149: aayu mission recover lists the Codex environment the delegation started in

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `72b0a948bc00d3d2e067a8a8bcff9daf98f031e56aef2f156a26c5d737da93b8` |
| Completion Goal hash | `10be4ef62477d4a157d29ecd320d5136cb814ca0db5961dc04f5e3ee1532b696` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1149/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: #64 was handed off to Codex under environment A, and `[delegate] codex_env` is now B; expected: #64's delegation records environment A, `aayu mission recover 889` lists the tasks of A, holds #64 as maybe started, and starts no second task
2. [api-trace] state: #64's delegation has no recorded environment, and `[delegate] codex_env` is set; expected: recover holds #64 as unreadable, names the missing environment, and starts nothing

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu mission recover` lists Codex tasks under the current `[delegate] codex_env` only (`CodexCloud.List`, internal/delegate/receivers.go). The delegation does not record the Codex environment that it started in (`delegate.State`, internal/delegate/state.go). When a person changes `codex_env` between a start and a recover, the story's task is in the old environment, so the list does not show it. Recover can then read "no task" and restart the story, and two agents work on one story. (Open assumption on PR #1102, Story #1003.)

### Steps to reproduce

1. Set `[delegate] codex_env` to environment A, and start a mission on codex. Let the start of #64 get no answer, so #64 is `maybe started` while its task runs in A.
2. Change `[delegate] codex_env` to environment B.
3. Run `aayu mission recover 889`.

### Expected behaviour

The delegation records the Codex environment of its start. Recover lists the tasks of that environment, so it finds #64's task and holds the story. A delegation with no recorded environment holds the story as unreadable when `codex_env` is not the one it started in, or when aayu cannot tell.

### Actual behaviour

Recover lists environment B. The list has no task for #64, so recover can restart #64 while its first task still runs in A.

### Completion Goal

1. [api-trace] state: #64 was handed off to Codex under environment A, and `[delegate] codex_env` is now B; expected: #64's delegation records environment A, `aayu mission recover 889` lists the tasks of A, holds #64 as maybe started, and starts no second task
2. [api-trace] state: #64's delegation has no recorded environment, and `[delegate] codex_env` is set; expected: recover holds #64 as unreadable, names the missing environment, and starts nothing

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Recover a maybe-started Codex story after a change of `[delegate] codex_env`, then one whose delegation records no environment.

### Environment

go test of internal/cli with the codex world (`newCodexWorld`, internal/cli/missionrecover_test.go) and the fake GitHub; never production

### Technical Owner

engineering-manager

### Checkpoints

1. The delegation records its Codex environment, and recover lists that one (criteria 1)
2. A delegation with no recorded environment holds the story (criteria 2)

### Assumptions

#### Notes

- Found while building Story #1003 (PR #1102); it is an open assumption in that PR's RAID log.
- It can start a second agent on one story, so it is a correctness risk, not only a liveness gap.

#### Review guidance

- Check: recover never reads "no task" from a list of an environment other than the one the delegation started in.
- Risk: old delegations have no recorded environment. They must hold, not restart.

#### Behaviours

1. says: Recover starts no second agent on one story; rule: never; source: criteria 1, 2

### Provenance

Request: "file the `[delegate] codex_env` gap as a Bug under #889 with grill: the delegation records its Codex environment, and recover lists that one."
1. stated (request): "the delegation records its Codex environment, and recover lists that one"
2. stated (request): "the delegation records its Codex environment, and recover lists that one"
````
