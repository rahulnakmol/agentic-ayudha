# rahulnakmol/agentic-ayudha#796: aayu inbox reads each Ready card's pull request again on every refresh and marks one that cannot ship as not ready yet, with the reason

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `eb558f5440f9669b7cc1db4db92d04ad15040c6c68e0e2e0bbf49ddafed63ce7` |
| Completion Goal hash | `d66f91ff428ddf2d3a60247cb2ff57e9fe10c4513f8ef3cbceef7d618f95b26b` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/796/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 with a Ready-to-ship card whose PR conflicts with dev because another PR merged into dev after ship's Ready to ship comment; expected: the card sits in a "Not ready yet" group, says "#<n> can't merge: it conflicts with dev, which took #<m> since ship checked it" and "Next: its builder rebases it onto dev; nothing for you yet", offers only opening it on GitHub (no ship choice), is not counted in "needs you", and every line is exactly the terminal's width
2. [api-trace] state: an open inbox with a Ready card for PR A that merges cleanly, then a merge into dev that makes A conflict while A's `updated_at` stays the same, then one refresh (R, or the 45-second timer); expected: the refresh reads A's pull request again and the card becomes the not-ready card of criterion 1; after A is rebased and ship comments Ready to ship on the new head, the next refresh shows the Ready card with its ship choice again
3. [api-trace] state: an open inbox with two Ready cards whose PRs' required checks (every required check of dev, not only merge-gate, inspect and pack) go from pending to failed on one and from pending to green on the other, while neither PR's `updated_at` changes, then one refresh; expected: the failed one is not ready and names the check in words ("its lint check failed on its latest commit"), with no ship choice; the pending one said "its checks are still running" before the refresh; the green one offers ship
4. [api-trace] state: a PR with ship's Ready to ship comment on an earlier head and new commits pushed after it; expected: its card stays in the inbox as not ready, says "new commits were pushed since ship checked it, so ship has to check them first", and offers no ship choice
5. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 with any card on screen; expected: the key bar names "R refresh", and the status line says when the inbox last read GitHub, such as "read 2 min ago"
6. [api-trace] state: `aayu inbox --list`, as Markdown and with --json, with the not-ready cards of criteria 1, 3 and 4; expected: each shows the same plain reason and next actor as the full-screen inbox, and none offers ship
7. [screenshot] state: the not-ready card of criterion 1, then d, then d again; expected: d shows GitHub's own facts (its mergeable state, the head commit, each required check by name and result) under the card's main lines, and d again hides them; the main lines name no SHA, URL, HTTP method or status code

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want every Ready-to-ship card in `aayu inbox` to show its pull request as it is now, and a card that cannot ship to say "not ready yet" with the reason in plain words and who acts next before I press Enter, so that I never press Enter on a ship that GitHub will refuse (@rahulnakmol, 2026-10-19: "we need to know in inbox that it not ready yet").

On 2026-10-19 #661 conflicted with dev twice under an open Ready card: after #707 merged, and again after #725 merged. Both times the card still offered ship, because a refresh re-reads only the issues whose `updated_at` moved (`inboxSource.refresh` in internal/cli/inbox.go) and a PR whose base branch moved keeps its `updated_at`. The card also checks only the three gate checks (`readyItem`), never every required check of dev and never whether GitHub can merge the PR, and a card whose head moved after ship's Ready to ship comment simply leaves the inbox. The refresh key `R` is listed only under `?`.

Replaces #794 (the refresh half) and delivers criterion 1 of #793; #793's failure wording is a separate Story, and its `/aayu review again` criterion a separate Bug.

### Completion Goal

1. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 with a Ready-to-ship card whose PR conflicts with dev because another PR merged into dev after ship's Ready to ship comment; expected: the card sits in a "Not ready yet" group, says "#<n> can't merge: it conflicts with dev, which took #<m> since ship checked it" and "Next: its builder rebases it onto dev; nothing for you yet", offers only opening it on GitHub (no ship choice), is not counted in "needs you", and every line is exactly the terminal's width
2. [api-trace] state: an open inbox with a Ready card for PR A that merges cleanly, then a merge into dev that makes A conflict while A's `updated_at` stays the same, then one refresh (R, or the 45-second timer); expected: the refresh reads A's pull request again and the card becomes the not-ready card of criterion 1; after A is rebased and ship comments Ready to ship on the new head, the next refresh shows the Ready card with its ship choice again
3. [api-trace] state: an open inbox with two Ready cards whose PRs' required checks (every required check of dev, not only merge-gate, inspect and pack) go from pending to failed on one and from pending to green on the other, while neither PR's `updated_at` changes, then one refresh; expected: the failed one is not ready and names the check in words ("its lint check failed on its latest commit"), with no ship choice; the pending one said "its checks are still running" before the refresh; the green one offers ship
4. [api-trace] state: a PR with ship's Ready to ship comment on an earlier head and new commits pushed after it; expected: its card stays in the inbox as not ready, says "new commits were pushed since ship checked it, so ship has to check them first", and offers no ship choice
5. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 with any card on screen; expected: the key bar names "R refresh", and the status line says when the inbox last read GitHub, such as "read 2 min ago"
6. [api-trace] state: `aayu inbox --list`, as Markdown and with --json, with the not-ready cards of criteria 1, 3 and 4; expected: each shows the same plain reason and next actor as the full-screen inbox, and none offers ship
7. [screenshot] state: the not-ready card of criterion 1, then d, then d again; expected: d shows GitHub's own facts (its mergeable state, the head commit, each required check by name and result) under the card's main lines, and d again hides them; the main lines name no SHA, URL, HTTP method or status code

### Measure

Ships started from the inbox on a card whose PR GitHub reported, at the inbox's last read, as conflicting, behind dev, or with a required check not green.

### Threshold

0.

### Scenario

@rahulnakmol keeps `aayu inbox` open while agents merge PRs into dev, sees "#661 can't merge: it conflicts with dev" under Not ready yet with nothing to press, and ships the next card that is ready.

### Environment

`aayu inbox` in the orb's Terminal tab, at 120 and 72 columns, on rahulnakmol/agentic-ayudha.

### Technical Owner

ux-researcher-evolutionist

### Links

### Checkpoints

1. `go test -race ./internal/cli -run 'TestInbox'` passes with a conflict, a behind, a new-commits, a failed and a pending required check, each read on a refresh that no `updated_at` change triggered (criteria 1, 2, 3, 4, 6, 7)
2. Renders of the real not-ready card, its details, and the key bar at 120x34 and 72x34 are in the Amp thread (criteria 1, 5, 7)

### Assumptions

Re-reading the pull request behind each Ready card on every refresh stays within GitHub's rate limit, because conditional requests answer 304 when nothing changed. The PRs named in "dev took #<m>" are the ones merged into dev after ship's Ready to ship comment; aayu does not work out which of them caused the conflict. GitHub's mergeable state can be unknown for a moment after a push; an unknown state does not block the card, and ship's own checks still apply.

### Open questions

deferred: Should a not-ready card ask its builder thread to rebase, or only say that its builder will? (#793 deferred it; the card names the next actor either way.)

### Parked ideas

Rebase from the inbox: a choice that rebases the PR for the builder (it would write code, not only state).

### Provenance

Request: "we need to know in inbox that it not ready yet"
1. stated (request): "we need to know in inbox that it not ready yet"
2. stated (behaviour): "update old thing in the view"
3. stated (request): "we need to know in inbox that it not ready yet"
4. stated (request): "we need to know in inbox that it not ready yet"
5. stated (behaviour): "doesnt have arefresh"
6. stated (request): "we need to know in inbox that it not ready yet"
7. stated (behaviour): "some text are too nerdy and technical"
````
