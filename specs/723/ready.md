# rahulnakmol/agentic-ayudha#723: fix(inspect): under reviewer ci, the gate waits forever for an app it cannot ask

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `e4613b05987189590ca5c6e686654d2b528f40282aea1c7305422ef3e174c0b6` |
| Completion Goal hash | `b7b30e703a761f03fc10140250eebb13f7e6ab0904ae10cbe6ccd4852cc96bda` |
| Technical Owner | Engineering Manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/723/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `apps = ["codex", "coderabbit"]` and the CI review declined in the gate; expected: the inspect comment names coderabbit as the reviewer it waits for, and codex as skipped
2. [api-trace] state: `apps = ["codex"]` and the CI review declined in the gate; expected: the gate waits for a human review, and after the approver's `aayu inspect --approve` the gate's disposition is merge at rung human

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Under `[inspect] reviewer = "ci"`, the merge gate can wait for a reviewer it cannot ask. The gate declines the CI model when its family matches the builder's, or when the PR's author lacks write access. It then picks the first installed app in the fixed order. If that app is Codex, Claude or Cursor, only a person's mention asks it. CI holds no person's account, and `aayu inspect` outside CI asks an app only after a failed CI review (#691). No failure is recorded, so the head stays no-reply for good. `aayu inspect --approve` and `aayu inbox` do not clear it either, because an approval counts only when no app is chosen (`internal/cli/inspect_app.go`).

### Steps to reproduce

1. Set `[inspect] reviewer = "ci"`, `apps = ["codex"]`, and a paid reviewer.
2. Open a pull request whose author has read access only.
3. Let the merge gate run `aayu inspect <pr> --ci`.

### Expected behaviour

The gate skips an app it cannot ask. A review service such as CodeRabbit reviews if one is installed; otherwise a person does, and the approver's `aayu inspect --approve` counts.

### Actual behaviour

The gate waits for Codex with no request posted, so the head stays no-reply, and the approver's approval does not count.

### Completion Goal

1. [api-trace] state: `apps = ["codex", "coderabbit"]` and the CI review declined in the gate; expected: the inspect comment names coderabbit as the reviewer it waits for, and codex as skipped
2. [api-trace] state: `apps = ["codex"]` and the CI review declined in the gate; expected: the gate waits for a human review, and after the approver's `aayu inspect --approve` the gate's disposition is merge at rung human

### Measure

Heads left no-reply with no reviewer that can act, per pull request.

### Threshold

0 such heads.

### Scenario

A read-only contributor opens a pull request on a repository with `reviewer = "ci"` and Codex installed.

### Environment

`aayu inspect --ci` in the merge gate, on `dev` at 2f4cbb5.

### Technical Owner

Engineering Manager

### Checkpoints

1. The gate skips a mention-only app when the CI review is declined (criteria 1)
2. The human rung's approval counts after the skip (criteria 2)

### Assumptions

A review service (CodeRabbit, Greptile) reviews every head without being asked, so the gate can read its review.

### Open questions

deferred: Should a failed CI review on a head also skip a mention-only app in the gate? (today `aayu inspect` outside CI asks it then)

### Provenance

Request: "In the gate (CI), skip apps CI cannot ask, and fall to the next rung that can act there: a review-service app, else the human rung, so --approve and the inbox count. Outside CI, keep today's order."
1. default (behaviour): "In the gate (CI), skip apps CI cannot ask, and fall to the next rung that can act there"
2. default (behaviour): "a review-service app, else the human rung, so --approve and the inbox count"
````
