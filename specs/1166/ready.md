# rahulnakmol/agentic-ayudha#1166: Every mission eval case scores 1 because the case worlds have no answer for aayu mission recover

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `dc9cf47992669d8c133d1556a7d9ef392f5f90566c7ceb05f463f30f9ad2da02` |
| Completion Goal hash | `ed7230ab6119a2c12ec338ba16b94d4501e8e93d3c236372f829cbdb26159f5f` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1166/ready.md` |

Completion Goal at Ready:

1. [metric] state: evals.yml group mission on dev after the fix; expected: mission/stays-in-list passes all 5 runs with a mean of at least 3.50, and mission/halted and mission/starts-each pass all 5 runs
2. [api-trace] state: the fix's diff; expected: it changes no bar: evals/eval.toml, each case's rubric.md, and each case's require and forbid rules are unchanged
3. [api-trace] state: the recorded session of each mission case, replayed in tier 1; expected: TestTier1 passes, and each session runs `aayu mission recover 889` before `aayu mission next 889`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Every mission eval case scores 1 of 5, because the case worlds have no recorded answer for `aayu mission recover`. Since #1102 (bdd02c1), step 1 of the Kanha protocol in skills/mission/SKILL.md is `aayu mission recover <f>`, and "If `recover` or `next` exits with an error, start nothing." The worlds in evals/cases/mission/*/session.toml record only `aayu mission next` and the hand-offs. So recover exits 2 ("has no recorded answer in this case"), and the Kanha starts nothing, as the protocol says. In the halted case, recover exits 4, and the Kanha stops before the `aayu mission next 889` that the case requires. The evals.yml run 41095327956 (group mission, dev c60f3cf, after #1146 merged) gives stays-in-list pass^5 0, mean 1.00; starts-each pass^5 0, mean 1.00; halted pass^5 0, mean 1.60. So Bug #1132's criteria 1 and 2 are not met on dev. No mission-group run on dev between #1102 and c60f3cf kept results.

### Steps to reproduce

1. On dev c60f3cf, dispatch evals.yml with group mission.
2. Read the session of any mission case in the eval-results artifact.

### Expected behaviour

The Kanha runs recover, then the list, then one hand-off per start, and the cases score as the rubrics say.

### Actual behaviour

recover exits 2 (exit 4 in the halted case), and the Kanha starts nothing and reports that recover failed.

### Completion Goal

1. [metric] state: evals.yml group mission on dev after the fix; expected: mission/stays-in-list passes all 5 runs with a mean of at least 3.50, and mission/halted and mission/starts-each pass all 5 runs
2. [api-trace] state: the fix's diff; expected: it changes no bar: evals/eval.toml, each case's rubric.md, and each case's require and forbid rules are unchanged
3. [api-trace] state: the recorded session of each mission case, replayed in tier 1; expected: TestTier1 passes, and each session runs `aayu mission recover 889` before `aayu mission next 889`

### Measure

pass^5 and the mean rubric score of each mission case, from the evals.yml mission-group run on dev

### Threshold

stays-in-list pass^5 1 and mean >= 3.50; halted and starts-each pass^5 1

### Scenario

The mission's Kanha gets an event, such as a merge, and runs its protocol.

### Environment

dev c60f3cf; evals.yml run 41095327956; rahulnakmol/agentic-ayudha

### Technical Owner

engineering-manager

### Checkpoints

1. Each mission case world answers `aayu mission recover 889` as the verb prints it in that world (criteria 1, 3)
2. The halted case and the protocol agree on what runs when recover exits 4 (criteria 1, 2)

### Assumptions

#### Notes

In the halted world the real halt gate refuses recover too (internal/cli/halt.go). The protocol says to stop when recover errors, but the case requires the list. The builder resolves that in the skill text or the case world, not by changing a require or forbid rule. Bug #1132 (PR #1146, merged as c60f3cf) owns the one-line report; this Bug owns the recover step that #1102 added.

#### Review guidance

- Check: no eval bar moves (evals/README.md, "The bars only rise").
- Ignore: the one-line report rule; Bug #1132 owns it.

#### Behaviours

1. says: On each event the Kanha runs recover, then the list, then starts each story the list marks start; rule: must; source: criteria 1, 3
2. says: A fix of the case worlds never lowers a bar; rule: never; source: criteria 2

### Provenance

Request: "If the mission group fails criteria 1 or 2 of #1132, file a Bug"
1. stated (request): "If the mission group fails criteria 1 or 2 of #1132, file a Bug"
2. default (scope): "the thresholds are #1132's, and no eval bar moves"
3. default (scope): "the recorded sessions run recover before the list"
````
