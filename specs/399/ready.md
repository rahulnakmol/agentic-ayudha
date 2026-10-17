# rahulnakmol/agentic-ayudha#399: Fix at the static check layer (Feedback #328): one local gate that mirrors the required CI jobs, run before a push

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `3c26afa4edf10e2d4d3f8b79665285d029c34eae0c12a6123b228084362c1f86` |
| Completion Goal hash | `233e4e407f0e24d0b05d836696adbaf4be03ea5a6b559186bce67a0a4bd69fc5` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/399/ready.md` |

Completion Goal at Ready:

1. [metric] state: a branch with an unformatted Go file, a Markdown lint error, generated-file drift, or a commit without a sign-off; expected: the generated `.agents/check --full` exits non-zero naming the CI job it mirrors, and `TestLocalGateMirrorsRequiredJobs` fails when a required CI job has no local step

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Feedback #328 is a correction classified at the static check layer: Every failure in this pattern is already a required CI job; running the same checks before a push catches it every time, and generating the local gate from the workflow keeps the two from drifting. No judgement is needed.

Replaces #332, which has the same Completion Goal but lacked the Ready fields (Measure, Threshold, Scenario, Environment, effort); no aayu verb edits an existing issue, so this Story carries the completed spec. This Story fixes it there, so the mistake becomes impossible or caught rather than warned against in a skill. Its proof is the new lint rule, test, or CI check that the Completion Goal names.

### Completion Goal

1. [metric] state: a branch with an unformatted Go file, a Markdown lint error, generated-file drift, or a commit without a sign-off; expected: the generated `.agents/check --full` exits non-zero naming the CI job it mirrors, and `TestLocalGateMirrorsRequiredJobs` fails when a required CI job has no local step

### Measure

Completion Goal criteria observed

### Threshold

all 1 criteria

### Scenario

On a scratch branch, introduce each fault in turn: an unformatted Go file, a Markdown lint error, a generated file edited by hand, and a commit without a Signed-off-by trailer. Run `.agents/check --full` after each and read its exit code and the CI job it names. Then run `go test -run TestLocalGateMirrorsRequiredJobs` with a required CI job that has no local step.

### Environment

A local checkout of this repository with the toolchains `.aayudha/env.toml` pins, plus go test on the pull request and in the merge gate; no external data
````
