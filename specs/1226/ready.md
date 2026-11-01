# rahulnakmol/agentic-ayudha#1226: fix(verify): the gate cannot find a pnpm-installed Playwright, so its browser checks fail and list a fix

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `e80e218b501890ff82c64dca68c7a8134f7c516197ea248468486c802339b265` |
| Completion Goal hash | `b0e9cb01046a3a7c16ee892c67589743a1b872b8cb33c3194d338dd36af9379b` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1226/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the repository depends on `@playwright/test` through pnpm, `playwright` is only under node_modules/.pnpm, and NODE_PATH is unset; expected: `aayu verify --check all` resolves Playwright and runs the browser checks
2. [api-trace] state: no Playwright is installed anywhere; expected: each browser check is reported as `unreachable: Playwright is not installed` with the command to install it, and the gate lists no fix for it
3. [api-trace] state: NODE_PATH names a node_modules that has Playwright; expected: that one is used, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu mission gate` fails four verify checks of a milestone because control.mjs cannot find Playwright, although the repository installs it. control.mjs resolves `playwright` from the product's package.json, then from NODE_PATH (`hosts/verify-skill/node/control.mjs:53-60`). The root package.json depends on `@playwright/test`, and pnpm keeps `playwright` only in `node_modules/.pnpm/playwright@1.64.0/node_modules`, so `createRequire` does not find it. On 2026-11-01 at 14:40 UTC, the gate of #1170's MS1 on dev eb51d1b in the Kanha's Amp orb failed `verify:ask-answer.empty`, `verify:refer-to-hr.typed`, `verify:refusals.personal` and `verify:refusals.advice` with "Playwright is not installed". The same run with NODE_PATH set to that folder passed all four. A failed gate check lists a fix to start, so a runner gap can start a fix PR for code that is correct.

### Steps to reproduce

1. In a checkout after `pnpm install`, with no NODE_PATH, a milestone's last story merges.
2. Run `aayu mission gate <f>`.

### Expected behaviour

The gate finds the Playwright that the repository installs, and the browser checks run.

### Actual behaviour

The four browser checks fail with "control.mjs: Playwright is not installed".

### Completion Goal

1. [api-trace] state: the repository depends on `@playwright/test` through pnpm, `playwright` is only under node_modules/.pnpm, and NODE_PATH is unset; expected: `aayu verify --check all` resolves Playwright and runs the browser checks
2. [api-trace] state: no Playwright is installed anywhere; expected: each browser check is reported as `unreachable: Playwright is not installed` with the command to install it, and the gate lists no fix for it
3. [api-trace] state: NODE_PATH names a node_modules that has Playwright; expected: that one is used, as today

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the gate's verify checks with Playwright only in pnpm's store, with none installed, and with NODE_PATH set.

### Environment

go test of internal/verify with a temporary pnpm-shaped node_modules and a fake node; never production

### Checkpoints

1. The verify runner finds a pnpm-installed Playwright (criteria 1, 3)
2. A missing Playwright is a runner gap, not a failed check (criteria 2)

### Technical Owner

quality-architect

### Assumptions

#### Notes

The runner already passes NODE_PATH through (`internal/verify/exec.go:28`) and searches it for the screen-reader check (`internal/verify/sr.go:190-194`). The fix adds the store folder of `playwright` to that search for control.mjs.

#### Review guidance

- Check: criterion 2: a missing tool never lists a fix PR.
- Ignore: the wording of the unreachable line.
- Risk: a runner gap reported as a pass; it stays unreachable, and the gate still waits.

#### Behaviours

1. says: A gate fails a check only for the code, never for a missing runner tool; rule: never; source: criteria 2

### Provenance

Request: "learn from what we have built so far so we can really understand the friction here and what are are causing al lot of halts and user intervention"
1. default (scope): find the Playwright that pnpm installed
2. default (scope): a missing tool is unreachable, not a failure
3. default (scope): NODE_PATH still wins
````
