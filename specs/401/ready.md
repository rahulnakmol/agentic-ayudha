# rahulnakmol/agentic-ayudha#401: Every deterministic eval rule ships with a match, a negated, and a near-miss fixture

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `37aa05f20c71fd2460e5c93d83d051500927c69ce4c34701e0060d9fd167ea76` |
| Completion Goal hash | `89fbbcbfa7ab55e923ac7270457c4ec4320f0be614a18e89bd4ff910e24d6f8b` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/401/ready.md` |

Completion Goal at Ready:

1. [metric] state: a deterministic rule in an eval case without a matching, a negated, and a near-miss fixture; expected: `TestCaseRulesHaveFixtures` fails naming the rule and the missing fixture kind

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Replaces #333, which retro --classify filed from Feedback #329 without the fields Ready needs. As the founder, I want every deterministic rule in an eval case to ship with a matching, a negated, and a near-miss fixture, enforced by a test, so that a rule that misfires on a negation, a near-miss word, or a paraphrase is caught before it costs an evaluation round (Feedback #329: 9 times in 7 threads).

### Completion Goal

1. [metric] state: a deterministic rule in an eval case without a matching, a negated, and a near-miss fixture; expected: `TestCaseRulesHaveFixtures` fails naming the rule and the missing fixture kind

### Measure

Completion Goal criteria observed

### Threshold

all 1 criteria

### Scenario

Run `go test ./internal/evals -run TestCaseRulesHaveFixtures` over every case's rules (case.toml scope rules and session.toml require, forbid, and say_forbid rules), then remove one fixture kind from one rule and run it again.

### Environment

local checkout and this repository's CI build job; the repository's own eval cases, never production

### Checkpoints

1. `go test ./internal/evals -run TestCaseRulesHaveFixtures` passes over every rule in evals/cases (criteria 1)
2. with one fixture kind removed from one rule, the same test fails naming the case, the rule, and the missing kind (criteria 1)

### Assumptions

- a negated fixture states whether the rule must still fire on it, since a topic the human parked is out of scope even when ruled out
- no existing rule's pattern or allow changes to make its fixtures pass

### Provenance

Request: "build them!"
1. confirmed (request): "build them!"
````
