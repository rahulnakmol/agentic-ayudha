# rahulnakmol/agentic-ayudha#690: ship: a skipped merge-gate check from an inspect-reply run hides the gate's green check

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `cfea0af2d821557ef092df15feec048d4d0835cb965a3eeb71c8d3cc9cb89325` |
| Completion Goal hash | `bb6aa9c1dbf6ab7683cfe463cae7c3398e5d5c418f2ca3e4659bd34472a2714e` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/690/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a Story PR head with a green aayudha/merge-gate check from a pull_request_target run, then a newer skipped aayudha/merge-gate check from a reply-triggered run; expected: aayu ship passes the merge-gate check step and does not refuse with "aayudha/merge-gate is skipped"
2. [api-trace] state: the same head, but the newest pull_request_target run that ran the merge-gate job failed or is still in progress; expected: aayu ship refuses and names that run's state, not the older green one

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu ship` reads the merge gate's checks on the head by name and takes the newest check run of each name (`internal/cli/ship.go`, the `gateChecks` loop: "the API lists the newest run of a name first"). A comment or review on the PR starts a reply-triggered run of `aayudha-merge-gate.yml`. In that run only `aayudha/inspect-reply` runs, and the `aayudha/merge-gate`, `aayudha/inspect` and `aayudha/pack` jobs are skipped. A review event (`pull_request_review`) attaches those skipped check runs to the PR head. When the review arrives after the gate's `aayudha/merge-gate` job started, the skipped check is the newest one, and ship refuses: "aayudha/merge-gate is skipped on <head>; the merge gate must be green". The green check from the gate's own `pull_request_target` run is hidden. `gateState` in `internal/cli/home.go` and the gate read in `internal/cli/inbox.go` use the same newest-first rule, so they report the gate as failed too.

On PR #678 at fcd9e73, gate run 39163879159 made the green check 117782671465 at 20:35 UTC. Reply run 39164323984 made the skipped check 117783986384 at 20:39 UTC, and ship refused. Of the last 15 PRs merged to dev, 682, 657, 623 and 617 also end with a skipped `aayudha/merge-gate` check as the newest on their head. Each review-triggered reply run after the gate hides the green check until the gate run is re-run.

### Steps to reproduce

1. Open a Story PR and let the merge gate's `pull_request_target` run pass on the head.
2. Submit a review on the PR (an app review, or a reply to a review thread), so that a reply-triggered run starts and skips the merge-gate job.
3. Run `aayu ship <pr>`.

### Expected behaviour

Ship judges the newest completed merge-gate run that ran the merge-gate job for the head. A skipped job from a reply-triggered run never hides it.

### Actual behaviour

Ship refuses with "aayudha/merge-gate is skipped on <head>; the merge gate must be green". The only way through is to re-run the gate run, which also re-runs the CI review.

### Completion Goal

1. [api-trace] state: a Story PR head with a green aayudha/merge-gate check from a pull_request_target run, then a newer skipped aayudha/merge-gate check from a reply-triggered run; expected: aayu ship passes the merge-gate check step and does not refuse with "aayudha/merge-gate is skipped"
2. [api-trace] state: the same head, but the newest pull_request_target run that ran the merge-gate job failed or is still in progress; expected: aayu ship refuses and names that run's state, not the older green one

### Measure

Ship refusals for "aayudha/merge-gate is skipped" on a head whose gate run passed.

### Threshold

0.

### Scenario

A Story PR that gets a review after its merge gate passed, then `aayu ship`.

### Environment

`aayu ship`, `aayu home` and `aayu inbox` against GitHub check runs written by `.github/workflows/aayudha-merge-gate.yml`.

### Technical Owner

engineering-manager

### Checkpoints

1. `aayu ship` on a head with a green gate check and a newer skipped reply-run check gets past the gate step, and still refuses when the newest gate run failed (criteria 1, 2)

### Assumptions

`gateState` in home.go and the inbox gate read should share the fixed rule. `internal/cli/ship.go` is in #593's narrow set, so the fix PR waits for a person. An issue_comment reply run attaches its check runs to the default branch, not the PR head, so only review events hide the green check.

### Open questions

deferred: filter skipped check runs, or read the gate checks from the run gatePack chose? (design; can wait)

### Parked ideas

None.

### Provenance

Request: "ship judges the newest completed merge-gate run that ran the merge-gate job for the head, and a skipped job from a reply-triggered run never hides it"
1. stated (behaviour): "a skipped job from a reply-triggered run never hides it"
2. stated (behaviour): "ship judges the newest completed merge-gate run that ran the merge-gate job for the head"
````
