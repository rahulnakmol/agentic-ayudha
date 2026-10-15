# rahulnakmol/agentic-ayudha#173: The floors, inspect-live, and screen-reader workflows ignore aayu halt

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `c15e260b428550b8051df49d30258ceb2e1ddc0a417cb42b46c74c7e3fa98dca` |
| Completion Goal hash | `3371a0c58c191b4367ce5f83da9c4f6ed13d516fcafd430e455059a2475a5102` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/173/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a check over every workflow in `.github/workflows/` and every workflow template in `internal/initrepo/`; expected: every job with steps starts with the halt check step, and the check fails naming any workflow and job that does not
2. [api-trace] state: `floors.yml`, `inspect-live.yml`, and `screen-reader.yml` started while `AAYUDHA_HALT` is set; expected: each job fails at its first step with the halt message and runs no other step
3. [api-trace] state: the same three workflows started while `AAYUDHA_HALT` is empty; expected: each runs as before

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu halt` sets the repository variable `AAYUDHA_HALT`, and workflows are meant to stop while it is set: most begin each job with a "Halt check" step. Three of this repository's workflows have no such step: `.github/workflows/floors.yml`, `.github/workflows/inspect-live.yml`, and `.github/workflows/screen-reader.yml`. While the repository is halted they still run. The docs state this limit: docs/guide/how-security.md (the honest-status note) and docs/threat-model.md (T18).

### Steps to reproduce

1. Run `grep -L AAYUDHA_HALT .github/workflows/*.yml`.
2. Halt the repository with `aayu halt`, then dispatch or trigger one of the three workflows.

### Expected behaviour

Every job of every workflow that runs steps begins with the halt check, so a halted repository runs nothing but that check.

### Actual behaviour

The grep lists `floors.yml`, `inspect-live.yml`, and `screen-reader.yml`. Nothing in them reads `AAYUDHA_HALT`, so a halt does not stop them (found by reading the workflows, not yet by a live halted run).

### Completion Goal

1. [api-trace] state: a check over every workflow in `.github/workflows/` and every workflow template in `internal/initrepo/`; expected: every job with steps starts with the halt check step, and the check fails naming any workflow and job that does not
2. [api-trace] state: `floors.yml`, `inspect-live.yml`, and `screen-reader.yml` started while `AAYUDHA_HALT` is set; expected: each job fails at its first step with the halt message and runs no other step
3. [api-trace] state: the same three workflows started while `AAYUDHA_HALT` is empty; expected: each runs as before

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the workflow check, then start each of the three workflows once with the halt variable set and once without, and record each run's steps and outcome.

### Environment

a scratch repository with the three workflows and seeded data; never production
````
