# rahulnakmol/agentic-ayudha#639: A person-verified aayu inspect --approve clears inspect's human fallback

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `689d3f42d430aee3fcb4d519d7e1e2605dd71063a763145be5daff0aff3547e3` |
| Completion Goal hash | `a52520fc5ab2e6c89d76077514b5146fe33daff3f904ab9a6fb27f9e6a9a9065` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/639/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu inspect <pr> --approve --reason "read the diff"` in a shell with an agent marker, run by the `[autonomy] approver` who completes the device flow; expected: the ledger records the approval for the PR's head with the approver's login and reason, and the PR gets the human-review reply that names the head
2. [api-trace] state: the merge gate's `aayu inspect <pr> --ci` on a PR at the human rung after criterion 1; expected: the disposition is merge at the human rung, and the review line names the ledger record
3. [api-trace] state: a stated merge reply comment from the approver's account with no ledger record for the head; expected: the gate stays at no-reply, and its comment names `aayu inspect <pr> --approve` as the way to clear it
4. [api-trace] state: `aayu inspect <pr> --approve` run by a login other than the `[autonomy] approver`, or with the device flow declined; expected: it refuses with exit blocked, and the ledger and PR are unchanged
5. [api-trace] state: an approval recorded for an earlier head, after a new push; expected: the gate stays at no-reply for the new head

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want to clear inspect's human fallback with a person-verified step, so that a PR whose reviewer app and rung B both cannot review still merges after my review, and an agent holding my token cannot fake that review with a comment (Codex's P2 on PR #634: a comment from the approver's account cannot show that a person wrote it, because agent PRs here are opened under the same account).

Today the only thing that clears the human fallback is a stated-reply comment from the human (`inspect.ReadReply`, internal/inspect/apps.go), and with `[autonomy] approver` set only the approver's comment counts. No presence-gated route covers it: `inspect --dismiss` only dismisses a finding of a finished review, the inbox's capped card runs `ship`, and `[approvals]` batches hold only Decisions. This Story adds a human-only `aayu inspect <pr> --approve` that runs through presence (internal/cli/presence.go, the device flow), as the `[autonomy] approver`, records the PR's head on the ledger, and posts the gate's human-review reply. The human fallback then counts only replies backed by that ledger record.

### Completion Goal

1. [api-trace] state: `aayu inspect <pr> --approve --reason "read the diff"` in a shell with an agent marker, run by the `[autonomy] approver` who completes the device flow; expected: the ledger records the approval for the PR's head with the approver's login and reason, and the PR gets the human-review reply that names the head
2. [api-trace] state: the merge gate's `aayu inspect <pr> --ci` on a PR at the human rung after criterion 1; expected: the disposition is merge at the human rung, and the review line names the ledger record
3. [api-trace] state: a stated merge reply comment from the approver's account with no ledger record for the head; expected: the gate stays at no-reply, and its comment names `aayu inspect <pr> --approve` as the way to clear it
4. [api-trace] state: `aayu inspect <pr> --approve` run by a login other than the `[autonomy] approver`, or with the device flow declined; expected: it refuses with exit blocked, and the ledger and PR are unchanged
5. [api-trace] state: an approval recorded for an earlier head, after a new push; expected: the gate stays at no-reply for the new head

### Measure

Human-fallback merges whose review has no presence-verified ledger record.

### Threshold

0.

### Scenario

Codex is out of quota and the gateway is down; the founder reviews the PR himself and clears the gate from his phone.

### Environment

The merge gate (`aayudha/inspect`) in a repository whose agent PRs are opened under the approver's account.

### Technical Owner

engineering-manager

### Links

depends-on #635

### Checkpoints

1. `go test ./internal/cli -run TestInspectApprove` passes: the record, the gate reading it, the comment that is not enough, the wrong login and the stale head (criteria 1, 2, 3, 4, 5)

### Assumptions

The ledger write for an approval works as inspect --dismiss's Dismissal record does.

### Open questions

deferred: should `aayu inbox` show a card for a PR waiting at the human rung, whose action runs --approve? (a follow-up once the command exists)

### Parked ideas

### Provenance

Request: "a human-only aayu inspect <pr> --approve, run through presence, that records the head on the ledger, and a human fallback that counts only replies backed by that record"
1. default (behaviour): "a human-only aayu inspect <pr> --approve, run through presence, that records the head on the ledger"
2. default (behaviour): "a human fallback that counts only replies backed by that record"
3. default (behaviour): "a human fallback that counts only replies backed by that record"
4. default (behaviour): "a human-only aayu inspect <pr> --approve, run through presence"
5. default (behaviour): "records the head on the ledger"
````
