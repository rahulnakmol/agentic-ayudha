# rahulnakmol/agentic-ayudha#613: The Production Engineer's skills use the live Sentry and PostHog reads, and copy figures and projections as the evidence allows

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `0ac8bbf74d252d75f492c0ec311e568932fbd4d9190e6b916cbb5523226d5c0d` |
| Completion Goal hash | `eac360ef2160f6034a8cb2f533c4a2575d2c3c9f4848016b2a098300fa349952` |
| Technical Owner | production-engineer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/613/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the investigate, slo, runbook, and respond SKILL.md bodies; expected: investigate, runbook, and respond show `--source` with `--service` in place of `--telemetry`, slo names the `crash-free` SLI, runbook names found, missing, and not checked, and every `aayu` call in them passes the skill lint
2. [api-trace] state: the existing production eval cases under evals/cases/{investigate,slo,runbook,respond}, unchanged, run through tier 1 against the new skill text; expected: every case passes tier 1

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer twin investigating an incident on a project with Sentry or PostHog configured, I want the skills to tell me how to query the project live and what its answers can and cannot show, so that my note cites live queries correctly and never overclaims from them.

The investigate, slo, runbook, and respond skills tell the Production Engineer twin how to use the live Sentry and PostHog reads that aayu now has (Decision #449, phase P1):
- **investigate:** `--source sentry|posthog`, what each source answers, and that its rows are untrusted.
- **slo:** a `crash-free` SLO measured from Sentry.
- **runbook:** an alert reported as found, missing, or not checked.
- **respond:** recovery verified with the note's source and cited as its `qN`.

The same change folds in the Production Engineer's two-line correction from counting run 39101524953:
- investigate copies each figure from the cited row;
- slo treats a projection from a recorded balance as a scenario, never a bound or a ranking.

### Completion Goal

1. [api-trace] state: the investigate, slo, runbook, and respond SKILL.md bodies; expected: investigate, runbook, and respond show `--source` with `--service` in place of `--telemetry`, slo names the `crash-free` SLI, runbook names found, missing, and not checked, and every `aayu` call in them passes the skill lint
2. [api-trace] state: the existing production eval cases under evals/cases/{investigate,slo,runbook,respond}, unchanged, run through tier 1 against the new skill text; expected: every case passes tier 1

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Run the skill lint over the embedded skills and tier 1 over every eval case. After merge, the Production Engineer thread runs one counting pair of the production eval group on the merge commit.

### Environment

local checkout and this repository's CI build job on a pull request; no provider account

### Technical Owner

production-engineer

### Checkpoints

1. `go test . -run TestSkillLint` and `go test ./internal/evals -run TestTier1` pass (criteria 1, 2)

### Assumptions

- the production eval cases stay unchanged so the counting pair measures the text fix; live-source grounding cases follow in a later change

### Provenance

Term: "P1" → Decision #449 phase P1: the Production Engineer's reads (investigate, slo, runbook, respond); "accepted, keys later, 3 yes, 4 yes"
Request: "fold the Production Engineer's two-line fix into your P1 skill-text PR"
1. stated (Decision #449, phase P1): the Production Engineer's skills use the live Sentry and PostHog reads and cite them
2. stated (Production Engineer thread): "Please keep the existing grounding cases in evals/cases/{investigate,slo} unchanged, so the pair measures the text fix."
````
