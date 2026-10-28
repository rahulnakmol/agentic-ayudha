# rahulnakmol/agentic-ayudha#1148: A maybe-started Codex story whose task is done with no PR stays held for good, because recover never re-attaches the task

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `c4b741948f7226d15c6eb99b6d3b6c74f59bffc3a01d65d47af9ceef06fb5585` |
| Completion Goal hash | `0e416864f340b3b789a94fed44566b7ef25399603a44b8cdec0ed5954c0ef661` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1148/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: #64 is maybe started, and the Codex list shows its task completed with no PR; expected: `aayu mission recover 889` records the task's id and URL on #64's delegation, the next `aayu build 64 --delegate codex` opens the PR from the task's diff, and no second task starts
2. [api-trace] state: #64 is maybe started, and the Codex list shows its task running; expected: recover records the task's id and URL, starts nothing, and the next sync reads that task by id

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A story that `aayu mission recover` holds as `maybe started` stays held for good when its Codex task finished with no PR. `delegate.MayRun` counts a done task as one that holds the story (internal/delegate/receivers.go). So each wake prints `maybe started`, `aayu mission next` keeps the slot busy, and `aayu mission retry` refuses: "a codex task for #n exists". The delegation never learns the task's id, so the normal sync (`aayu build <n> --delegate codex`) cannot open the PR from the task's diff. No supported command frees the story. (CodeRabbit on PR #1102, r4399995703.)

### Steps to reproduce

1. Start a mission on codex, and let the start of #64 get no answer, so #64 is `maybe started`.
2. The Codex task list shows `# Delegated story #64: …` with status `completed`, and GitHub has no PR for #64.
3. Run `aayu mission recover 889` and `aayu mission retry 889 --story 64`.

### Expected behaviour

Recover re-attaches the story's task when it finds one: it records the task's id and URL on the delegation. The normal sync then reads that task, and opens the PR from a done task's diff. A live task stays held as now, and nothing starts a second agent.

### Actual behaviour

Recover prints `#64  maybe started: a codex task exists for #64 but no PR yet` on every wake. Retry refuses. The story holds its slot until a person ends the mission.

### Completion Goal

1. [api-trace] state: #64 is maybe started, and the Codex list shows its task completed with no PR; expected: `aayu mission recover 889` records the task's id and URL on #64's delegation, the next `aayu build 64 --delegate codex` opens the PR from the task's diff, and no second task starts
2. [api-trace] state: #64 is maybe started, and the Codex list shows its task running; expected: recover records the task's id and URL, starts nothing, and the next sync reads that task by id

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Recover a maybe-started Codex story whose task is done, then one whose task still runs.

### Environment

go test of internal/cli with the codex world (`newCodexWorld`, internal/cli/missionrecover_test.go) and the fake GitHub; never production

### Technical Owner

engineering-manager

### Checkpoints

1. Recover records a done task on the delegation, and the next sync opens its PR (criteria 1)
2. Recover records a running task, and the next sync reads it by id (criteria 2)

### Assumptions

#### Notes

- Found by CodeRabbit on PR #1102 (Story #1003), at head b572797.
- It holds the story and starts nothing, so it is a liveness gap, not a second-agent risk.

#### Review guidance

- Check: a re-attach writes only the delegation's task id and URL, and only when exactly one task holds the story.
- Risk: two tasks for one story, such as a restart and the first start. Recover must not pick one silently.

#### Behaviours

1. says: A story held as maybe started has a supported way out; rule: never; source: criteria 1, 2

### Provenance

Request: "File the done-task re-attach as a Bug under #889 with grill, add it to #906, and make it Ready."
1. stated (request): "File the done-task re-attach as a Bug under #889 with grill"
2. stated (request): "File the done-task re-attach as a Bug under #889 with grill"
````
