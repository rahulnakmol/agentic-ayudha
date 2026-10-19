# rahulnakmol/agentic-ayudha#799: A Solo-mode bypass comment is not tied to a commit: the inbox posts it before ship's checks, and ship accepts one written for an earlier head

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `c34115acccbdbe400df8292b74632c88358e4835773c9daf75601e5c6242d2b1` |
| Completion Goal hash | `8aa367ed162e3be2bd22a75a1285a70339c4d28d9935f6e47c6fe373eb365356` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/799/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: solo mode, a Ready card for a PR whose merge GitHub refuses, then Enter in `aayu inbox`; expected: no Solo-mode bypass comment is posted on the PR, because ship's checks run before the comment and the refusal stops it
2. [api-trace] state: solo mode, a PR with a Solo-mode bypass comment from the merging account naming an earlier head, then `aayu ship <pr> --solo-bypass` on the current head; expected: ship refuses with "post a Solo-mode bypass for the head <current short sha> first", merges nothing, and the ship record never cites the old comment
3. [api-trace] state: solo mode, a PR with a bypass comment for its current head, then `aayu ship <pr> --solo-bypass`; expected: it merges as today, and the ship record's human pass cites that comment and that head

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The Solo-mode bypass that lets `aayu ship --solo-bypass` merge without a second person is not tied to a commit, and the inbox posts it before ship has checked anything. `shipReady` (internal/cli/inbox.go) posts "Solo-mode bypass: @<login> ships this from `aayu inbox` … the merge gate is green on the head `<sha>`" whenever `soloModeComment` (internal/cli/ship.go) finds none, then runs ship; `soloModeComment` accepts the first comment from that account starting "Solo-mode bypass:", whatever head it names. On 2026-10-19 at 11:29Z @rahulnakmol pressed Enter on #661's Ready card: the inbox posted his bypass comment for the head `e51e073`, then GitHub refused the merge (405, a conflict with dev after #707 merged). That comment stays on #661, so a later ship of any head, after any number of pushes, takes it as his bypass without asking again.

### Steps to reproduce

1. In solo mode, open `aayu inbox` with a Ready-to-ship card for PR A whose PR conflicts with dev.
2. Press Enter: the inbox posts the Solo-mode bypass comment for A's head, then ship's merge is refused.
3. Push new commits to A, resolve the conflict, and run `aayu ship A --solo-bypass` (or Enter on A's new Ready card).

### Expected behaviour

A bypass counts only for the head it names, and the inbox posts it only after ship's own checks pass on that head, just before the merge. Step 3 asks for a new bypass on the new head; nothing is posted in step 2.

### Actual behaviour

Step 2 leaves a bypass comment on A for a head that never merged, and step 3 merges the new head under that old comment: `soloModeComment` returns the first matching comment, and `shipReady` posts no new one because one exists.

### Completion Goal

1. [api-trace] state: solo mode, a Ready card for a PR whose merge GitHub refuses, then Enter in `aayu inbox`; expected: no Solo-mode bypass comment is posted on the PR, because ship's checks run before the comment and the refusal stops it
2. [api-trace] state: solo mode, a PR with a Solo-mode bypass comment from the merging account naming an earlier head, then `aayu ship <pr> --solo-bypass` on the current head; expected: ship refuses with "post a Solo-mode bypass for the head <current short sha> first", merges nothing, and the ship record never cites the old comment
3. [api-trace] state: solo mode, a PR with a bypass comment for its current head, then `aayu ship <pr> --solo-bypass`; expected: it merges as today, and the ship record's human pass cites that comment and that head

### Measure

Merges whose recorded Solo-mode bypass names a different head than the one merged.

### Threshold

0.

### Scenario

@rahulnakmol presses Enter on a Ready card for a PR that conflicts with dev; nothing is posted on the PR, and after its builder rebases it the next ship asks for his bypass on the new head.

### Environment

`aayu inbox` and `aayu ship --solo-bypass` in solo mode on rahulnakmol/agentic-ayudha; `internal/cli/ship.go` is in `[autonomy] always_person`, so @rahulnakmol merges the fix.

### Technical Owner

engineering-manager

### Links

### Checkpoints

1. `go test -race ./internal/cli -run 'TestInboxShipsReadyPRs|TestShip'` passes with a refused merge that posts no bypass, an old-head bypass that is refused, and a current-head bypass that merges (criteria 1, 2, 3)

### Assumptions

The bypass comment's existing text already names the head (`the merge gate is green on the head <sha>`), so a head check needs no new comment format for comments the inbox writes; a hand-written bypass names its head the same way.

### Open questions

deferred: Should a bypass written by hand with no head in it still count for the PR's current head only? (The fix can require the head and say so in the refusal.)

### Provenance

Request: "the solo-mode bypass comment is not tied to a commit (shipReady reuses any earlier bypass comment, and posts it before ship's checks)"
1. stated (request): "posts it before ship's checks"
2. stated (request): "the solo-mode bypass comment is not tied to a commit"
3. stated (request): "the solo-mode bypass comment is not tied to a commit"
````
