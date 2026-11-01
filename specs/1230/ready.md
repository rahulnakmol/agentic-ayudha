# rahulnakmol/agentic-ayudha#1230: fix(mission): a gate fix PR records only its first failed check, so a fix for several checks leaves the gate waiting

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `32d286174d508ab065d2b6c6c2a29d367a722ac38f9db0d6ab0f3a6b47a26ac3` |
| Completion Goal hash | `fe528b7079c6dc64151005e6932a4dc47f2c9f9f0a3578cf28125dffab51d7c9` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1230/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a fix PR names two failed checks of one milestone on two `Aayudha-Gate-Fix:` lines; expected: `aayu mission gate <f> --pr <n>` records the PR for both, and prints one line per check
2. [api-trace] state: that PR merged; expected: `aayu mission gate <f>` runs the gate again on the dev tip
3. [api-trace] state: one of the named checks did not fail in the last gate run, or has another fix PR; expected: that line is refused with today's reason, and the other check is recorded
4. [api-trace] state: the fix is counted against the budget; expected: one PR counts once, at the per-story cap, however many checks it names

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

One gate fix PR records only one failed check, so a fix that covers several checks leaves the gate waiting for PRs that will never come. `aayu mission gate <f> --pr <n>` reads the first `Aayudha-Gate-Fix:` line of the body (`gateFixLine.FindStringSubmatch`, internal/cli/missiongate.go:1172), and the gate runs again only when every failed check has a fix PR that merged or closed (`openFixes`, internal/cli/missiongate.go:836-858). On 2026-11-01, the MS1 gate of #1170 failed three cases of the retro eval (em-enterprise, em-scaleup, em-startup) for one cause in `skills/retro/SKILL.md`. PR #1229 fixed all three and named all three on `Aayudha-Gate-Fix:` lines, but the gate recorded it only for em-enterprise. After #1229 merged as 2bd966e, the gate on dev 2bd966e said "the gate runs again when its fixes merge: eval:retro/em-scaleup (no PR yet), eval:retro/em-startup (no PR yet)", so MS1, and with it MS2 and BUGS, waits with no way forward.

### Steps to reproduce

1. A milestone's gate fails checks A and B for one cause.
2. One draft PR to dev says `Refs #<f>`, `Aayudha-Gate-Fix: MS1 A`, and `Aayudha-Gate-Fix: MS1 B`.
3. Run `aayu mission gate <f> --pr <n>`, merge the PR, and run `aayu mission gate <f>`.

### Expected behaviour

The PR is recorded as the fix of A and B, and the gate runs again after it merges.

### Actual behaviour

Only A is recorded. The gate waits for a fix PR of B.

### Completion Goal

1. [api-trace] state: a fix PR names two failed checks of one milestone on two `Aayudha-Gate-Fix:` lines; expected: `aayu mission gate <f> --pr <n>` records the PR for both, and prints one line per check
2. [api-trace] state: that PR merged; expected: `aayu mission gate <f>` runs the gate again on the dev tip
3. [api-trace] state: one of the named checks did not fail in the last gate run, or has another fix PR; expected: that line is refused with today's reason, and the other check is recorded
4. [api-trace] state: the fix is counted against the budget; expected: one PR counts once, at the per-story cap, however many checks it names

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Fail two checks, record one PR for both, merge it, and run the gate; then name a check that did not fail.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake clock; never production

### Checkpoints

1. One fix PR records every check it names (criteria 1, 3, 4)
2. The gate runs again once that PR merges (criteria 2)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The fix brief (`internal/cli/missiongate.go:1145`) asks for one line per PR; it gains "one line per check it fixes". Until this merges, a person or agent works around it with one PR per check.

#### Review guidance

- Check: criterion 3: each line is checked on its own.
- Check: criterion 4: the budget counts the PR once.
- Ignore: the wording of the recorded lines.
- Risk: a PR that names a check it does not fix; the gate re-runs every check, so a wrong claim fails again there.

#### Behaviours

1. says: One fix PR can fix several failed checks, and the gate runs again when it merges; rule: limit; source: criteria 1, 2

### Provenance

Request: "learn from what we have built so far so we can really understand the friction here and what are are causing al lot of halts and user intervention"
1. default (scope): one PR records every check it names
2. default (scope): the gate re-runs after it merges
3. default (scope): each line is checked as today
4. default (scope): the budget counts the PR once
````
