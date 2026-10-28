# rahulnakmol/agentic-ayudha#1150: inspect reads a person's /aayu review again comment as their review reply and fails closed

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `31d23555a8d57f709cec9980243f2f8da908bc63eb9f50585b0fa77abbfc2a44` |
| Completion Goal hash | `3f47b5513939b080eacda437ad5298ca6d8b2aa2cc6929434f93a94b7c3e2c70` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1150/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the human reviewer's only post after the review request is a `/aayu review again` comment; expected: `ReadReply` returns no reply (`ErrNoReply`), not `ErrMalformed`, and inspect's disposition is `no-reply`
2. [api-trace] state: the same person later posts a comment with no `aayudha-review` block that is not an `/aayu` command; expected: inspect still says `malformed-reply` for it, as today
3. [api-trace] state: the person posts a valid `aayudha-review` reply for the head after the `/aayu review again` comment; expected: inspect reads that reply and its disposition

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

When the review of a PR falls to a person, inspect reads that person's `/aayu review again` comment as their review reply. The comment has no `aayudha-review` block, so inspect says `malformed-reply` and the gate fails closed. On PR #1147 at 5facdb5, the paid reviewer failed (a stream error) and CodeRabbit did not reply in 30 minutes, so the review fell to @rahulnakmol. His reopen comment 6471958245 ("/aayu review again", posted at 14:38 to reopen the review after step-back) was then read as his reply: "comment 6471958245 by rahulnakmol has no `aayudha-review` block for 5facdb50bca4". `ReadReply` (internal/inspect/apps.go:343) keeps every non-inline post by the human reviewer that is not a review request (line 357). A post updated after the request with no reply block is malformed (lines 410-423). A reopen command is not a reply, but nothing skips it.

### Steps to reproduce

1. On an agent PR, let inspect reach step-back. Then a person posts `/aayu review again`.
2. Push a new head. Make rung B fail on it and let the reviewer app not reply, so the review falls to that person.
3. Read the merge gate's inspect comment for the new head.

### Expected behaviour

Inspect skips the person's `/aayu` command comments when it reads their reply. It says `no-reply` and waits for the person's review, or their `aayu inspect <pr> --approve`.

### Actual behaviour

Inspect says `malformed-reply`: the reopen comment "has no `aayudha-review` block". The inspect check fails until the person approves through `aayu inspect --approve`.

### Completion Goal

1. [api-trace] state: the human reviewer's only post after the review request is a `/aayu review again` comment; expected: `ReadReply` returns no reply (`ErrNoReply`), not `ErrMalformed`, and inspect's disposition is `no-reply`
2. [api-trace] state: the same person later posts a comment with no `aayudha-review` block that is not an `/aayu` command; expected: inspect still says `malformed-reply` for it, as today
3. [api-trace] state: the person posts a valid `aayudha-review` reply for the head after the `/aayu review again` comment; expected: inspect reads that reply and its disposition

### Measure

inspect's disposition for the PR head, from `aayu inspect <pr> --json` and the unit test of `ReadReply`

### Threshold

all three criteria hold in `go test ./internal/inspect/...`

### Scenario

The human-review fallback of the merge gate's inspect, on a PR that a person reopened with `/aayu review again`.

### Environment

aayudha-merge-gate.yml inspect job on a pull request to dev; dev at bdd02c1

### Technical Owner

engineering-manager

### Checkpoints

1. A `/aayu review again` comment by the human reviewer no longer fails the gate: inspect says no-reply and waits (criteria 1)
2. A stray comment by the reviewer that is not a command still fails closed, and a valid reply is still read (criteria 2, 3)

### Assumptions

#### Notes

The #1001 thread owns the merge gate, so it builds this fix. Seen on PR #1147, inspect comment 6469008584 at 5facdb5. Keep the fail-closed rule for every other post that has no reply block.

#### Review guidance

- Check: Criterion 2: a stray comment that is not a command is still malformed.
- Check: Other `/aayu` commands (`/aayu approve`, `/aayu skip`) are skipped the same way.
- Ignore: reviewer-app replies; this changes only the human reviewer's posts.

#### Behaviours

1. says: A person's `/aayu` command comment is never read as their review reply; rule: never; source: criteria 1
2. says: A post by the reviewer with no reply block that is not a command still fails closed; rule: must; source: criteria 2

### Provenance

Request: "File the other one (inspect reads a `/aayu review again` comment as the human review's reply and fails closed) under #889 with grill, add it to #906, and make it Ready."
1. stated (request): "inspect reads a `/aayu review again` comment as the human review's reply and fails closed"
2. default (scope): "inspect reads a `/aayu review again` comment as the human review's reply"
3. default (scope): "inspect reads a `/aayu review again` comment as the human review's reply"
````
