# rahulnakmol/agentic-ayudha#1006: A story subthread owns one story, may use subagents, and ships it on proof

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `fb8e5216fd1d5018013e113b23fe516f506743c287632bf23c7dc48213d9c0c2` |
| Completion Goal hash | `dde785bac056a9fa35cdff7ed2ef37be525ce0eac0ede8187abc9610ccd54c84` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1006/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the Kanha starts story #50 on amp; expected: the brief says the subthread owns #50 in its own worktree and branch, may use subagents, acts on inspect's findings, rebases on dev before ship, and runs `aayu ship <pr>` on proof; it still says never `--admin` and never merge another way
2. [api-trace] state: the same brief for a receiver that cannot run aayu (Codex, Cursor, or Claude cloud); expected: the brief says never merge, as today
3. [api-trace] state: story #52 depends on #40, which merged as PR #41; expected: #52's brief lists PR #41, its ship record, and the asks answered on #40

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want each story's subthread to carry its PR through review to ship, so that a merge wakes the Kanha with no wait.

### Completion Goal

1. [api-trace] state: the Kanha starts story #50 on amp; expected: the brief says the subthread owns #50 in its own worktree and branch, may use subagents, acts on inspect's findings, rebases on dev before ship, and runs `aayu ship <pr>` on proof; it still says never `--admin` and never merge another way
2. [api-trace] state: the same brief for a receiver that cannot run aayu (Codex, Cursor, or Claude cloud); expected: the brief says never merge, as today
3. [api-trace] state: story #52 depends on #40, which merged as PR #41; expected: #52's brief lists PR #41, its ship record, and the asks answered on #40

### Measure

Hours from inspect `merge` to ship for story PRs, p50.

### Threshold

0.1 for subthreads that run aayu; 1 for the rest.

### Scenario

Story #50's PR passes the merge gate at 13:40. Its subthread ships it at 13:41.

### Environment

go test of internal/delegate brief rendering with a fake ledger; never production.

### Links

depends-on #1004

### Assumptions

#### Notes

@rahulnakmol decided Q2: the subthread ships on proof, and the safety tick covers cloud receivers. No file locks (F7).

#### Review guidance

check: ship decides every merge; the subthread only asks. A narrow-set PR still waits in `aayu inbox` as Ready to ship.
ignore: The brief's wording outside the ship, merge, and handoff lines.
risk: Prompt injection makes a subthread ship (threat-model T6). ship's own gates stay the control.

#### Behaviours

1. [authority] says: A story's subthread merges only through aayu ship, and only on proof; source: AGENTS.md (A Story PR merges only through aayu ship); brief.go:118; criteria: 1
2. [capability] says: A story that depends on merged work gets that work's PRs and answers in its brief; source: Decision #915 (M1), F6; criteria: 3
````
