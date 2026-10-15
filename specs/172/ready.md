# rahulnakmol/agentic-ayudha#172: Nothing runs proof purge on a schedule, so media retention depends on a person

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `3208e22e6601db08a66a5e1a2428ce7ff68c3c221f6a91334671b6c715fcf051` |
| Completion Goal hash | `1417f49d09777e50f96a69db13ffd8322db58c1cd6d4dd6ff436201d75350204` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/172/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu init --gates` on a scratch repository, and `go generate ./...` in this repository; expected: the rendered loops include one that runs `aayu proof purge` on a schedule and starts with `aayu hook loop:<name>`
2. [api-trace] state: that loop run against a ledger with one pack older than `[proof] retention_days` and one newer, each with seeded media in a fake bucket; expected: the older pack's media are deleted and its pack.json tombstoned, and the newer pack is untouched
3. [api-trace] state: the same loop run while `aayu halt` is in force; expected: it exits 4 and purges nothing

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`[proof] retention_days` defaults to 90, and `aayu proof purge` deletes proof media older than that and tombstones the pack, but nothing runs it on a schedule: no loop in `loops/`, no workflow in `.github/workflows/`, and no template that `aayu init --gates` renders calls `proof purge`. Retention of screenshots and recordings, which can hold personal data, depends on a person remembering to run it. The docs state this limit: docs/guide/how-ledger.md (the honest-status note) and docs/threat-model.md (T14, "retention and purge").

### Steps to reproduce

1. Search `loops/*.toml`, `.github/workflows/`, and `internal/initrepo/*.yml` for `proof purge`.
2. Run `aayu init --gates` on a scratch repository and list the loops and workflows it renders.

### Expected behaviour

A scheduled run, rendered for products by `aayu init --gates` and for this repository by `go generate`, runs `aayu proof purge` for packs past `[proof] retention_days`, and respects `aayu halt` and the budget like every other loop.

### Actual behaviour

The search finds nothing: no loop, workflow, or template runs `proof purge`.

### Completion Goal

1. [api-trace] state: `aayu init --gates` on a scratch repository, and `go generate ./...` in this repository; expected: the rendered loops include one that runs `aayu proof purge` on a schedule and starts with `aayu hook loop:<name>`
2. [api-trace] state: that loop run against a ledger with one pack older than `[proof] retention_days` and one newer, each with seeded media in a fake bucket; expected: the older pack's media are deleted and its pack.json tombstoned, and the newer pack is untouched
3. [api-trace] state: the same loop run while `aayu halt` is in force; expected: it exits 4 and purges nothing

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Render the loops in a scratch repository, seed a ledger with an old and a new pack and their media in a fake bucket, run the purge loop once normally and once while halted, and record what was deleted.

### Environment

local scratch repository with seeded, synthetic packs and a fake bucket; never production
````
