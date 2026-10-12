# rahulnakmol/agentic-ayudha#91: doctor names the floor each missing tool serves

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `12c10e7dde181f2aabf3c83a57ebae9831a924904ca924e558e34160352a609b` |
| Completion Goal hash | `a6abff0b46f34e6c923c4c125cc9f7a77ae6ef25c3aeb0d4bf4e632233e76a4c` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/91/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the doctor optional-tools check on a machine without semgrep and docker; expected: the warning names "semgrep (sast floor)" and "docker (dast floor: ZAP)" and says that a floor whose tool is missing blocks verify
2. [api-trace] state: the doctor optional-tools check on a machine with every optional tool installed; expected: the check passes with "all optional tools found"

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As someone setting up aayudha on a repository, I want `aayu doctor` to name each missing floor tool together with the floor it serves, and to say that such a floor blocks verify, so that I know what to install before the merge gate refuses my pull request.

### Completion Goal

1. [api-trace] state: the doctor optional-tools check on a machine without semgrep and docker; expected: the warning names "semgrep (sast floor)" and "docker (dast floor: ZAP)" and says that a floor whose tool is missing blocks verify
2. [api-trace] state: the doctor optional-tools check on a machine with every optional tool installed; expected: the check passes with "all optional tools found"

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Run the doctor optional-tools check against a probe whose PATH lacks semgrep and docker, then against a probe that finds every tool, and read each result's status and detail.

### Environment

go test on the pull request and in the merge gate, with synthetic probes only; no external data
````
