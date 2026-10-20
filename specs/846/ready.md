# rahulnakmol/agentic-ayudha#846: aayu inbox refuses a choice when only the card's words changed, and never says what changed

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `83c3bb990ea0cbe4ad051f2893ca5f6247e03807dd4894900be70d8a927fa11b` |
| Completion Goal hash | `448b8fce24e7f2554759492444cec9a5633e5a5c66eb3778572b65128d324dd4` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/846/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a capped card whose title, reason, and counsel differ from a fresh read, with the same head and the same ship choice; expected: the re-read before the ship returns no error, and names the fields that differ (counsel, title, why)
2. [api-trace] state: the same card read at head aaaaaaa, while the PR's head is now ddddddd; expected: nothing is written, and the error says "the head moved from aaaaaaa to ddddddd"
3. [api-trace] state: a choice whose label changed, or that is no longer offered; expected: nothing is written, and the error quotes the choice as it reads now, or says it is no longer offered
4. [api-trace] state: a Decision renamed on GitHub after the person read it; expected: nothing is written, and the refusal says "its title changed"
5. [api-trace] state: `aayu inbox -v`, and a choice whose card changed only in fields the choice does not depend on; expected: the choice is done, and the session log names the fields that differed

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu inbox` refuses a choice whenever any word on the card differs from a fresh read, even when what the choice does is unchanged, and the refusal never says what changed. On 2026-10-20 @rahulnakmol pressed Enter on #817's card twice between 17:05 and 17:08 UTC, and both times got "#817: it changed on GitHub since you read it, so nothing was written; here it is as it is now". He could not act on the card. #802's card was refused the same way once.

### Steps to reproduce

1. Open `aayu inbox` with a card on screen, for example a capped PR or a Ready to ship card.
2. Let something change that the choice does not depend on: its reason or counsel wording (a newer build, or a card from the cache), its title, or a refreshed Ready to ship comment on the same head.
3. Pick a choice that writes, such as ship.

### Expected behaviour

The choice goes ahead when its kind, the head commit, and the choice itself are unchanged. When one of them changed, nothing is written, and the refusal names what changed, for example "the head moved from 2b4e947 to eb17918". Any other difference is kept in the session log with `-v`, so a cause stays findable.

### Actual behaviour

`inboxSource.reread` (`internal/cli/inbox.go`) compares the whole card as JSON, except `updated` and `details`, and returns `errInboxChanged` on any difference. `errInboxChanged.Error` has one fixed sentence that names nothing. The card holds no head commit of its own (`inboxFacts.head` is not written to the cache), so a card drawn from the cache cannot be checked against the head at all.
#817: on dev 1643973 and on b823c57 (the build he ran, from before #802), gathering three times on one source and an 8-try replay of his keys gave identical cards each time, so the exact difference in his session is not known. It runs only in his session, which began at 16:50:04Z and shipped #802 at 17:03:51Z.
#802: the card showed head 2b4e947; ship moved its Ready to ship comment to eb17918 at 16:36:19Z (the comment's edit history). That refusal was right, but the message did not say so.

### Completion Goal

1. [api-trace] state: a capped card whose title, reason, and counsel differ from a fresh read, with the same head and the same ship choice; expected: the re-read before the ship returns no error, and names the fields that differ (counsel, title, why)
2. [api-trace] state: the same card read at head aaaaaaa, while the PR's head is now ddddddd; expected: nothing is written, and the error says "the head moved from aaaaaaa to ddddddd"
3. [api-trace] state: a choice whose label changed, or that is no longer offered; expected: nothing is written, and the error quotes the choice as it reads now, or says it is no longer offered
4. [api-trace] state: a Decision renamed on GitHub after the person read it; expected: nothing is written, and the refusal says "its title changed"
5. [api-trace] state: `aayu inbox -v`, and a choice whose card changed only in fields the choice does not depend on; expected: the choice is done, and the session log names the fields that differed

### Measure

Inbox choices refused as changed when the kind, head, and choice were the same, per release.

### Threshold

0 per release (v0.3.0: 2 on #817 on 2026-10-20)

### Scenario

The founder presses Enter on a card whose words changed since he read it, and the choice goes ahead; or its head moved, and the card tells him to which commit.

### Environment

The `internal/github/fake` GitHub with seeded PRs, inspect comments, and Decisions, in `internal/cli` tests; seeded data only.

### Links

### Checkpoints

1. The re-read blocks only on the kind, the head, and the choice, and names what changed (criteria 1, 2, 3, 4)
2. Other differences reach the session log (criterion 5)

### Assumptions

A Decision's title is part of what accepting it does, so a renamed Decision still asks again.
A card from a cache written before this fix has no head, so its first choice asks again once and says so.
The wider Bug W work (the result on the card, not the bottom line; the notice style) stays in that draft.

### Open questions

deferred: what differed in @rahulnakmol's session on #817; the session log with `-v` shows it next time

### Parked ideas

none

### Provenance

Request: "@rahulnakmol pressed Enter on #817's card in aayu inbox twice. Both times it refused with "it changed on GitHub since you read it, so nothing was written". On #802 the same thing happened once before a later try worked. So the second refusal on #817 is not a real change. The inbox cannot act on a card."
Vague: "not a real change" → criteria 1, 5
1. stated (request): "So the second refusal on #817 is not a real change."; facts: code:internal/cli/inbox.go, test:internal/cli/inbox_test.go
2. stated (request): "On #802 the same thing happened once before a later try worked."; facts: code:internal/cli/inbox.go, test:internal/cli/inbox_test.go
3. stated (request): "The inbox cannot act on a card."; facts: code:internal/cli/inbox.go, test:internal/cli/inbox_test.go
4. stated (request): "it changed on GitHub since you read it, so nothing was written"; facts: code:internal/cli/inbox.go, test:internal/cli/inbox_test.go
5. stated (request): "So the second refusal on #817 is not a real change."; facts: code:internal/cli/inbox.go, test:internal/cli/inbox_test.go
````
