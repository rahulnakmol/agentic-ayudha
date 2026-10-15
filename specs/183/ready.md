# rahulnakmol/agentic-ayudha#183: Audit reports no longer replay: the recorded config hash covers keys audit never reads

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `482da6481648cad66fbe583980eec5172a7ad26643db552b537a5b800d2975a5` |
| Completion Goal hash | `769cd4bb77b29fd272d1827bb4a58148030a7c7d9e401aae21587a3276d25e5c` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/183/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the inputs snapshots of the audit reports of 12 October 06:35 and 15 October 04:35 UTC and the ledger at the commits they record, replayed on the current binary; expected: each replay exits 0 and its report is byte-identical to the report on the ledger
2. [api-trace] state: a new report, then a configuration change to a key audit does not read (for example `[ui] theme`), then a replay; expected: the replay exits 0 and is byte-identical
3. [api-trace] state: a new report, then a change to a key audit reads (for example `[identity] bot_logins`), then a replay; expected: the replay refuses with exit 4 naming the configuration change

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu audit` records `config_hash` in each report's inputs snapshot as the SHA-256 of the whole effective configuration (`openAudit` in `internal/cli/audit.go`), and `audit --replay` refuses (exit 4) when the current binary's hash differs. Every new configuration key changes that hash even when audit never reads it: the `[ui]` section added by WP1.1b (#164) and the `[team]` section added by WP5.1b (#167) did. Earlier reports therefore no longer replay on the current binary, which breaks WP7.6's acceptance ("re-running audit with the same recorded inputs produces a byte-identical report") and the grandfathering WP1.9 promises for files written by an earlier binary. The docs state the replay promise: docs/guide/how-config-and-audit.md.

### Steps to reproduce

1. In a checkout of this repository on `dev`, build the binary: `go build -o /tmp/aayu ./cmd/aayudha`.
2. Run `/tmp/aayu audit --replay audit/2026-10-15/audit-20261015T043536Z.inputs.json --out /tmp/replay.md`.
3. Run the same for `audit/2026-10-12/audit-20261012T063500Z.inputs.json`.

### Expected behaviour

A report replays from its recorded inputs on a later binary whose configuration differs only in keys audit does not read, and the replay is byte-identical to the report on the ledger.

### Actual behaviour

Observed on 2026-10-15 with the `dev` binary at 37388b1: both replays exit 4 with "the configuration changed since the report (config hash sha256:de8b9584… , now sha256:6e98bd59…)" (and sha256:ec631ea8… for the 12 October report), although nothing audit reads changed.

### Completion Goal

1. [api-trace] state: the inputs snapshots of the audit reports of 12 October 06:35 and 15 October 04:35 UTC and the ledger at the commits they record, replayed on the current binary; expected: each replay exits 0 and its report is byte-identical to the report on the ledger
2. [api-trace] state: a new report, then a configuration change to a key audit does not read (for example `[ui] theme`), then a replay; expected: the replay exits 0 and is byte-identical
3. [api-trace] state: a new report, then a change to a key audit reads (for example `[identity] bot_logins`), then a replay; expected: the replay refuses with exit 4 naming the configuration change

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Replay the two recorded reports from a copy of the ledger on the current binary, then write a report in a scratch checkout, change an unrelated key and an audited key in turn, and replay after each, recording each exit code and the byte comparison.

### Environment

a copy of this repository's public ledger and a scratch checkout with a fake GitHub; never production
````
