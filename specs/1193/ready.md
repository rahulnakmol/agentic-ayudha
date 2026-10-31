# rahulnakmol/agentic-ayudha#1193: aayu mission start always refuses, because no user config key sets the person's tool for a repository

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `21b33cb3c1de793c5acc379f616c5d9a9925e1d1b15942271dd13b2fafff2711` |
| Completion Goal hash | `2c90876e5fae649019c0f5a13629d94c656e17751abd7067d762ce027a50a4f5` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1193/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the user config sets no tool for o/r; expected: `aayu mission start` refuses, names the key and the value to set for o/r, and starts nothing
2. [api-trace] state: the user config sets the tool for o/r to amp; expected: `aayu mission start` records amp as the mission's tool and starts its Kanha
3. [api-trace] state: the repository's .aayudha/config.toml sets the key; expected: config loading refuses it, as for [hosts.cursor] plan, and names the user config

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

aayu mission start always refuses, because no user config key sets the person's tool for a repository. `Env.repoTool` (internal/cli/mission.go) reads `Env.RepoTool`, and only tests set it: the comment says "Until that entry lands it is not set". So both Start paths refuse with "Start records your tool for <repo> from your user config, and none is set there. Nothing was started." (internal/cli/mission.go, the Feature start and the Tatkal start). No story added the user config entry that R9 names.

### Steps to reproduce

1. Approve Feature #1170 to build.
2. Run `aayu mission start 1170` as @rahulnakmol, on any tool.

### Expected behaviour

A user-level-only config key names the person's tool for each repository. Start reads it, records the tool, and starts the mission. When the key is not set, the refusal names the exact key to set. A checkout cannot set the key.

### Actual behaviour

Start refuses on every run, and the refusal names no key, because no key exists.

### Completion Goal

1. [api-trace] state: the user config sets no tool for o/r; expected: `aayu mission start` refuses, names the key and the value to set for o/r, and starts nothing
2. [api-trace] state: the user config sets the tool for o/r to amp; expected: `aayu mission start` records amp as the mission's tool and starts its Kanha
3. [api-trace] state: the repository's .aayudha/config.toml sets the key; expected: config loading refuses it, as for [hosts.cursor] plan, and names the user config

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Start a mission with the key unset, set in the user config, and set in the checkout.

### Environment

go test of internal/cli with the mission world (internal/cli/mission_test.go) and the fake GitHub, and go test of internal/config; never production

### Technical Owner

engineering-manager

### Checkpoints

1. The key loads from the user config, an AAYU_ variable, or --set, and its value is checked against the known hosts (criteria 2, 3)
2. Env.RepoTool reads the key, and the refusal names it (criteria 1, 2)

### Assumptions

#### Notes

- Found when @rahulnakmol ran `aayu mission start 1170` on 2026-10-30.
- It blocks every mission start, so it is P1.

#### Review guidance

- Check: no checkout layer (.aayudha/config.toml, config.json, or config.local.toml) can set the key.
- Risk: a repository name can contain dots, so the key must not split it.

#### Behaviours

1. says: A person can start a mission on the tool they choose; rule: never; source: criteria 1, 2, 3

### Provenance

Request: "aayu mission start always refuses, because no user config key sets the person's tool for a repository"
1. stated (request): "aayu mission start always refuses, because no user config key sets the person's tool for a repository"
2. stated (request): "aayu mission start always refuses, because no user config key sets the person's tool for a repository"
3. stated (request): "aayu mission start always refuses, because no user config key sets the person's tool for a repository"
````
