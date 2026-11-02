# rahulnakmol/agentic-ayudha#1234: fix(ship): ship merges a PR proven on an older dev, so two green PRs can break dev together

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `02f58feac98654ec98612931487bc820deab3aa1181e5f28e1ddcd08a4cf1221` |
| Completion Goal hash | `8a65022adf55a05994c60d9a890df94f2ced05f8ef2506f9c008e0f55222c62a` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1234/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: PR B's head does not contain the dev tip, and the dev tip has a commit after B's base; expected: `aayu ship B` refuses with `#B was proven on <base>, not on dev <tip>: rebase it and let the gate run`, and merges nothing
2. [api-trace] state: PR B's head contains the dev tip, and its pack and gate are on that head; expected: ship merges as today
3. [api-trace] state: the dev tip moved only by a docs-only commit; expected: ship still refuses, because it cannot tell which change matters
4. [api-trace] state: the mission of B's story lists B; expected: `aayu mission next` shows `waits on a rebase` for B, and the builder is told

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu ship` merges a PR whose proof was built on an older dev, so two PRs that each pass alone can break dev together. On 2026-11-02, PR #1216 (#1174) merged at 04:11:10Z and made open Bugs join the BUGS milestone. PR #1227 (#1175) merged 40 seconds later, at 04:11:50Z, with a pack and a green `build` from a base without #1216. Its test `TestDayIsSet/line_by_line` (internal/cli/inboxday_test.go:86) expects Bug #30 in MS1, so dev 2715ef2 fails `build` (CI run 41387106825), and every PR into dev fails until a fix lands. Ship reads the base branch's policy at its tip (internal/cli/ship.go:470), but it does not check that the PR's proof includes the tip's code. GitHub's "require branches to be up to date" is the usual guard, and it is off for dev, or ship merges past it.

### Steps to reproduce

1. PRs A and B to dev each have a proven pack and a green gate on their own heads, built on dev at D.
2. Ship A; dev moves to D+A.
3. Ship B at once.

### Expected behaviour

Ship refuses B until its head includes D+A and its gate passes there, or it proves B on the merge with D+A first.

### Actual behaviour

Ship merges B, and dev breaks when A and B conflict in behaviour.

### Completion Goal

1. [api-trace] state: PR B's head does not contain the dev tip, and the dev tip has a commit after B's base; expected: `aayu ship B` refuses with `#B was proven on <base>, not on dev <tip>: rebase it and let the gate run`, and merges nothing
2. [api-trace] state: PR B's head contains the dev tip, and its pack and gate are on that head; expected: ship merges as today
3. [api-trace] state: the dev tip moved only by a docs-only commit; expected: ship still refuses, because it cannot tell which change matters
4. [api-trace] state: the mission of B's story lists B; expected: `aayu mission next` shows `waits on a rebase` for B, and the builder is told

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Ship two PRs proven on one base one after the other, then rebase the second and ship it; repeat with a docs-only commit in between.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake clock; never production

### Checkpoints

1. Ship merges only a head that contains the dev tip (criteria 1, 2, 3)
2. The mission shows the wait (criteria 4)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

This serializes merges to dev, as the coordinator saw on 30 Oct ("merges are serialized because shared files force rebases"). Each rebase costs one gate run; a later story may batch rebases. ship.go is in the narrow set, so the PR waits for a person's look.

#### Review guidance

- Check: criterion 1: the check uses the PR head's ancestry, not timestamps.
- Ignore: the wording of the refusal.
- Risk: more gate runs per day; the gate is what proves the merge.

#### Behaviours

1. says: Ship merges only what was proven on the code it lands on; rule: never; source: criteria 1, 3

### Provenance

Request: "we need to be carefuly but optimisitic to accelerate the build and release of 0.4.0 and 0.4.1 release"
1. default (scope): refuse a head that lacks the dev tip
2. default (scope): merge a current head as today
3. default (scope): no exception for docs-only commits
4. default (scope): the mission shows the wait
````
