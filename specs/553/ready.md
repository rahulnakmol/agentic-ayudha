# rahulnakmol/agentic-ayudha#553: hook: a Copilot shell call that exits non-zero is logged as citable

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `7052de90285722d66bd484fc5cecf9e4387401a796778ea16a7f9f9714d6ac7b` |
| Completion Goal hash | `2c189939257228c388a970f95650fe34cee24ef1f953cafb60c94572152b065e` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/553/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Copilot `postToolUse` events whose shell result text ends with `<shellId: 1 completed with exit code 2>` and with `<shellId: 0 completed with exit code 0>`; expected: the first is failed and the second is not
2. [api-trace] state: `aayu hook post-tool --host copilot` on a `bash` Sentry CLI call with a window and a limit that exited 2; expected: the call log entry is failed and not citable

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Under GitHub Copilot, a provider CLI call that fails is logged as citable evidence (Decision #449: every call through a host's CLI is logged and cited like aayu's own queries, and a failed call is never citable). Copilot's `postToolUse` event reports a shell command that exits non-zero as `resultType: "success"`; the exit code appears only in the result text, as a trailing `<shellId: N completed with exit code K>` marker. `hostcall.FromPostHook` fails a Copilot result only when `resultType` is not `success`, so `sentry-cli releases list --limit 5` that exits 2 becomes a citable entry in the call log.

### Steps to reproduce

1. In a repository set up with `aayu install --host copilot` and Copilot's post-tool hooks registered, let Copilot run `sentry-cli releases list --limit 5` in its `bash` tool with no organization configured, so it exits 2.
2. Read the session's call log under `.aayudha/state/calls`.

### Expected behaviour

The entry is marked failed and is not citable.

### Actual behaviour

The entry is not marked failed, and is citable when it names a window and a limit.

### Completion Goal

1. [api-trace] state: Copilot `postToolUse` events whose shell result text ends with `<shellId: 1 completed with exit code 2>` and with `<shellId: 0 completed with exit code 0>`; expected: the first is failed and the second is not
2. [api-trace] state: `aayu hook post-tool --host copilot` on a `bash` Sentry CLI call with a window and a limit that exited 2; expected: the call log entry is failed and not citable

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Run the post-tool parser and the post-tool hook on Copilot's captured event format for shell calls that exit 0 and 2, and read the failed flag and the call log entry.

### Environment

local checkout and this repository's CI build job on a pull request; event shapes captured live with Copilot CLI 1.0.98 on 2026-10-18 by the Copilot host thread; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/integrations/hostcall -run 'TestFromPostHook'` and `go test ./internal/cli -run 'TestHookLogsProviderCalls'` show the failed flag and the entry (criteria 1, 2)

### Assumptions

- the trailing exit-code marker is the only form Copilot CLI 1.0.98 printed live; a result without it keeps `resultType` as the only signal

### Provenance

Request: "a provider CLI call that names a window and a limit but exits 2 becomes Citable in NewEntry"
1. stated (Copilot host thread): "for a Copilot result, treat a trailing \"<shellId: N completed with exit code K>\" with K != 0 as Failed"
2. stated (Copilot host thread): "Copilot sends postToolUse with toolResult.resultType \"success\" for a bash call that exits non-zero"
````
