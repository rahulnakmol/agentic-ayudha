# rahulnakmol/agentic-ayudha#698: install --loops --apply refuses a person in an orb's Terminal tab instead of asking for a device code

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `18e9a1f641e5b90066e25aac0d5b0af380228050cb044d7aaffd539460035e51` |
| Completion Goal hash | `6d66ad771e056da7d2b4f25841bd05e01f976734a541f3c4361d06d20ebe9fa5` |
| Technical Owner | DevOps & DevEx Tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/698/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `install --loops --host amp --apply` at a terminal in a shell with `AMP_THREAD_ID` set; expected: aayu prints a device code and the loops it starts, and after approval it starts the Amp threads
2. [api-trace] state: the same command, and the device code is denied; expected: exit 4, and no Amp thread starts
3. [api-trace] state: the same command with `AMP_THREAD_ID` set and no terminal; expected: exit 4 with the agent-lane refusal, and no device code is issued

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu install --loops --apply` and `--run-now` refuse a person in an agent-marked shell. An orb's Terminal tab inherits `AMP_THREAD_ID`, so `internal/cli/installloops.go:89` (`e.agentSignal()`, from #192) refuses @rahulnakmol there. Other human-only commands in that shell use the device-flow presence check: `aayu inbox`, `decide --accept`, `inspect --approve` (`inspect.go:907`), and `release` (`release.go:268`).

### Steps to reproduce

1. Open an orb's Terminal tab in a checkout of rahulnakmol/agentic-ayudha.
2. Run `aayu install --loops --host amp --apply`.

### Expected behaviour

aayu asks for a GitHub device code, says which loops it starts and that they act as the person, and goes on after the person approves.

### Actual behaviour

It exits 4 at once: "install --loops --apply and --run-now start runs that act as you; a human runs them (agent lane: AMP_THREAD_ID=...)".

### Completion Goal

1. [api-trace] state: `install --loops --host amp --apply` at a terminal in a shell with `AMP_THREAD_ID` set; expected: aayu prints a device code and the loops it starts, and after approval it starts the Amp threads
2. [api-trace] state: the same command, and the device code is denied; expected: exit 4, and no Amp thread starts
3. [api-trace] state: the same command with `AMP_THREAD_ID` set and no terminal; expected: exit 4 with the agent-lane refusal, and no device code is issued

### Measure

Device-flow prompts instead of refusals for `install --loops --apply` in an agent-marked terminal.

### Threshold

1 device-flow prompt and 0 Amp threads before approval; 0 device codes with no terminal.

### Scenario

A person runs `aayu install --loops --host amp --apply` in an orb's Terminal tab.

### Environment

aayu built from `dev` at d17b165, in an Amp orb's Terminal tab.

### Technical Owner

DevOps & DevEx Tinkerer

### Checkpoints

1. `install --loops --apply` and `--run-now` call the presence check when an agent marker is set at a terminal (criteria 1, 2)
2. With an agent marker and no terminal, the refusal stays (criteria 3)

### Assumptions

`presence.go` stays as it is; `installloops.go` only calls `presenceCheck`.

### Provenance

Request: "Switch install --loops --apply and --run-now to e.presenceCheck, with a Pending that says which loops it starts and that they act as him. Keep the refusal when there is no terminal. Add one test showing that the agent marker now leads to the device flow instead of a refusal, and that it still refuses without approval."
1. default (behaviour): "the agent marker now leads to the device flow instead of a refusal"
2. default (behaviour): "it still refuses without approval"
3. default (behaviour): "Keep the refusal when there is no terminal"
````
