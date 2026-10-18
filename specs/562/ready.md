# rahulnakmol/agentic-ayudha#562: The merge gate's Codex reader skips Codex's security-review findings

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `2f7dcd1e56f91dacec8d3b34cf48a1ac5d2be9e499716379a49584c009210af7` |
| Completion Goal hash | `b5e154863a65525580a03c08fc43dbfa2c8995f9678edd9158d3cced8d1e1fe3` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/562/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a fake PR whose head has Codex's code-review receipt (no findings), its security-review receipt, and one inline comment recorded from #550 r4305772538 (marker codex-security-review-finding, P1 badge); `aayu inspect <pr> --ci --json`; expected: disposition fix, exit blocked, one finding with severity P1, category security, and the title from the comment
2. [api-trace] state: the same PR with the security comment's badge P2 instead (as r4305813429); expected: disposition merge with that finding listed, severity P2, category security
3. [api-trace] state: a fake PR whose head has only Codex's code-review receipt and Codex's summary comment whose codex-review-state names a security_review of the head with status in_progress; expected: disposition no-reply, exit blocked

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The merge gate's reader of Codex's own review (internal/inspect/apps.go `codexNative`) counts only inline comments marked `codex-code-review-finding`. Codex's security review posts its findings as inline comments marked `<!-- codex-security-review-finding:v1 -->`, so the reader drops them: the gate reported PR #550 at 9bb384e as merge while Codex's security review of that exact commit had an open P1 (r4305772538) and a P2 (r4305813429). A gate that says merge with an open security P1 is a false pass. The reader also says merge while Codex's security review of the head is still running, because Codex's code-review receipt arrives first.

### Steps to reproduce

1. On a PR with `[inspect] apps = ["codex"]`, let Codex review the head: its code-review receipt (an issue comment with `<!-- codex-review-receipt:v1 -->` and "Didn't find any major issues", **Reviewed commit:** the head) and its security review (a PR review with the same receipt marker and "Codex Security Review") holding one inline comment on the head marked `<!-- codex-security-review-finding:v1 -->` with a P1 badge, as Codex wrote r4305772538 on #550.
2. Run the gate's inspect: `aayu inspect <pr> --ci`.
3. Separately, with only the code-review receipt posted and Codex's summary comment (`<!-- codex-pull-request-review-summary -->`) recording in `codex-review-state:v1` a `security_review` of the head whose status is not `completed`, run the gate's inspect again.

### Expected behaviour

1. The security finding is a finding of the review: category security, severity from its badge, so a P0 or P1 makes the disposition fix.
2. While Codex's summary records a security review of the head that has not completed, the review of the head is not complete, so the gate has no reply yet and fails closed.

### Actual behaviour

1. `inspect #550 at 9bb384e: merge`, with "Codex reviewed 9bb384e: 0 finding(s) on that commit."
2. The gate passes on the code-review receipt alone while the security review is still running.

### Completion Goal

1. [api-trace] state: a fake PR whose head has Codex's code-review receipt (no findings), its security-review receipt, and one inline comment recorded from #550 r4305772538 (marker codex-security-review-finding, P1 badge); `aayu inspect <pr> --ci --json`; expected: disposition fix, exit blocked, one finding with severity P1, category security, and the title from the comment
2. [api-trace] state: the same PR with the security comment's badge P2 instead (as r4305813429); expected: disposition merge with that finding listed, severity P2, category security
3. [api-trace] state: a fake PR whose head has only Codex's code-review receipt and Codex's summary comment whose codex-review-state names a security_review of the head with status in_progress; expected: disposition no-reply, exit blocked

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Build aayu from the fix branch and run the inspect tests in internal/cli against the GitHub fake, with the security-review comment recorded from #550.

### Environment

aayu built from dev at 3acc841 (internal/inspect/apps.go `codexNative`), GitHub fake; observed live on PR #550, Codex app chatgpt-codex-connector[bot]. Any OS.

### Checkpoints

1. The native Codex reader counts an inline comment marked codex-security-review-finding on the head as a security finding with its badge's severity (criteria 1, 2)
2. A summary that records a running security review of the head leaves the head without a reply (criteria 3)

### Links

- caused-by #225

### Assumptions

Codex marks every security finding with `codex-security-review-finding` and a P0–P3 badge, as on #550; a repository not opted into Codex security review has no security_review in the summary state, so nothing waits for it.

### Provenance

Request: "File the Bug for the Codex reader skipping security-review findings, and fix it first. A gate that reports merge with an open security P1 is a false pass. Add a regression test with a recorded security-review comment."
1. stated (correctness/security findings): "A gate that reports merge with an open security P1 is a false pass."
2. stated (correctness/security findings): "Add a regression test with a recorded security-review comment."
3. default (correctness/running review): a review still running is no reply yet, so the gate fails closed (docs/guide/how-merge-gate.md)
Term: "P1" → Codex's severity badge P1 (high), which aayu treats like P0 as a defect to fix before merge; "an open security P1"
````
