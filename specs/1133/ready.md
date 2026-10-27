# rahulnakmol/agentic-ayudha#1133: A person's early /aayu review again reopens nothing, and the gate does not say so

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `841fee0b62a8273bec03d26cf48b34e76ca7106eb54f3eb61480ee47b4694b4b` |
| Completion Goal hash | `9da54ea3a5f8e8f8bc5fe01188cc8c7a87246dbed3b089f5fcdc738fd0ba5454` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1133/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the inspect state of #1103 before 03:27Z (one judged round, 173e579, no `reopened`, not stopped), and a `/aayu review again` comment at 03:27Z by an account with write access; expected: the gate posts one PR comment that says `/aayu review again` reopens nothing because the review is not stopped, and that 2 of 3 judged rounds remain in this window
2. [api-trace] state: the same comment, then judged rounds on ff606f2 and 97f1ffc, then head 8d22a48; expected: 8d22a48 gets step-back, as today, and `State.Reopen` returns zero for the 03:27Z command
3. [api-trace] state: no judged round in the window, and a person with write access comments `/aayu review again`; expected: the reply says 3 of 3 judged rounds remain in this window
4. [api-trace] state: three judged rounds, the review stopped at the head, and a person with write access comments `/aayu review again`; expected: the review reopens and the head's inspect job re-runs, as today, and the gate posts no "reopens nothing" reply
5. [api-trace] state: the same early reopen run twice on the same comment (a rerun of the gate); expected: one reply on the PR, not two

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A person's `/aayu review again` before the round cap reopens nothing, as #583 designed (`State.Reopen`, internal/inspect/inspect.go:325-351), but the gate says nothing about it. The person thinks the window was reset. On #1103, @rahulnakmol commented `/aayu review again` at 03:27Z with one judged round (173e579) in the window; three more heads were judged, and the fourth, 8d22a48, got step-back (run 40528962380, comment 6432360946). The cap rule stays; the gate must reply when a reopen does nothing.

### Steps to reproduce

1. A PR has one judged round in its window, and the review is at merge, not stopped.
2. A person with write access comments `/aayu review again`.
3. The merge gate runs on the comment (`aayudha inspect <pr> --ci`, internal/cli/inspect_app.go:106-126).

### Expected behaviour

The gate replies that the reopen does nothing, and says how many judged rounds remain in the window. The round window does not change.

### Actual behaviour

The gate writes nothing on the PR. The reopen is dropped in silence, and the person learns of it only at step-back.

### Completion Goal

1. [api-trace] state: the inspect state of #1103 before 03:27Z (one judged round, 173e579, no `reopened`, not stopped), and a `/aayu review again` comment at 03:27Z by an account with write access; expected: the gate posts one PR comment that says `/aayu review again` reopens nothing because the review is not stopped, and that 2 of 3 judged rounds remain in this window
2. [api-trace] state: the same comment, then judged rounds on ff606f2 and 97f1ffc, then head 8d22a48; expected: 8d22a48 gets step-back, as today, and `State.Reopen` returns zero for the 03:27Z command
3. [api-trace] state: no judged round in the window, and a person with write access comments `/aayu review again`; expected: the reply says 3 of 3 judged rounds remain in this window
4. [api-trace] state: three judged rounds, the review stopped at the head, and a person with write access comments `/aayu review again`; expected: the review reopens and the head's inspect job re-runs, as today, and the gate posts no "reopens nothing" reply
5. [api-trace] state: the same early reopen run twice on the same comment (a rerun of the gate); expected: one reply on the PR, not two

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Run the gate on a reopen comment before the cap with one and with no judged round, on a reopen of a stopped review, and twice on one early reopen; then judge the next heads of #1103.

### Environment

go test of internal/cli with a fake GitHub, in CI mode on an issue_comment event; never production

### Technical Owner

engineering-manager

### Checkpoints

1. The #1103 case: the early reopen gets a reply with 2 of 3 rounds left, and 8d22a48 still steps back (criteria 1, 2)
2. A reopen with no round in the window says 3 of 3, and one early reopen gets one reply (criteria 3, 5)
3. A reopen of a stopped review works as today, with no extra reply (criteria 4)

### Assumptions

- The gate replies only to a reopen from an account that may reopen (`mayReopen`, internal/cli/inspect_app.go); a bot's or a read-only account's comment gets no reply, as today.
- The reply is a bot comment from the gate, never a comment under the person's token.

#### Notes

The cap rule of #583 does not change: a reopen before the cap still reopens nothing (internal/cli/inspect_test.go:693). The early reopen on #1103 was ignored by that rule; no round is missing from its state, because the pack failed on c06add3, f672236 and 47ae2ff and inspect was cancelled on 0621057.

#### Review guidance

- Check: Criterion 2: `State.Reopen` is unchanged, so the reply adds no way to stretch the cap.
- Check: Criterion 5: a gate rerun on the same comment does not post a second reply.
- Risk: the reply must not count as a review or a reviewer's reply.
- Ignore: the wording of the step-back comment.

#### Behaviours

1. says: A person's reopen before the cap reopens nothing, and the gate says so with the rounds left; rule: never; source: criteria 1, 2, 3, 5
2. says: A reopen of a stopped review reopens it for three more rounds; rule: never; source: criteria 4

### Open questions

- answered: the cap rule stays; @rahulnakmol chose option (b), keep #583's rule and reply when a reopen does nothing.

### Parked ideas

- always reset: a person's reopen resets the window whatever came before it (option a, not chosen)

### Provenance

Request: "Option (b), under his blanket approval: keep #583's cap rule, and the gate replies when a reopen does nothing, saying how many judged rounds remain in the window."
1. stated (request): "the gate replies when a reopen does nothing, saying how many judged rounds remain in the window"
2. stated (request): "keep #583's cap rule"
3. stated (request): "saying how many judged rounds remain in the window"
4. stated (request): "keep #583's cap rule"
5. stated (request): "the gate replies when a reopen does nothing"
````
