# rahulnakmol/agentic-ayudha#797: aayu inbox says what failed in plain words, whether anything changed, and who acts next, for every way an inbox action can fail

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `8a1688636c246a50e8ec398c57121808e23a286556a275ea299620927d3fdb02` |
| Completion Goal hash | `18dbe7526892347e1f50427142d38adce74f780fbc9af059cf5d31012f35f064` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/797/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 after Enter on a Ready card whose merge GitHub refuses because the PR conflicts with dev; expected: the card body shows three lines, "#<n> wasn't merged: it conflicts with dev.", "Nothing was changed on GitHub.", and "Next: its builder rebases it onto dev; nothing for you yet."; the status line says "✗ #<n> not shipped; the card says why."; no line is cut off, and none of the three names a URL, an HTTP method, a status code, a SHA, or Go error text
2. [api-trace] state: each failure in the inventory table of this Story's PR (GitHub refusing a write with 401, 403, 404, 405, 409 or 422, a rate limit, a server error, a network error, a write that got no answer, each refusal of `aayu ship`, a Decision accept refused, a review approval refused, a tidy-up refused, an item that changed since it was read, a refresh that fails, and each sign-in failure); expected: for every row, the inbox gives a what-happened line, a line saying whether anything changed on GitHub, and a next line naming who acts, with none of the technical words of criterion 1 in them
3. [screenshot] state: a failed card, then d, then d again; expected: d shows the technical detail under the three lines (the step it stopped at, GitHub's own message, the request, and the commit), and d again hides it; the log printed when the inbox quits keeps the full detail
4. [api-trace] state: Enter on a Ready card whose merge succeeds and whose later step (the Story's record or the ledger write) fails; expected: the card shows "✓ Merged #<n> into dev", then a warning that names the step that did not finish and who acts next; the item is not shown as "not done" and Enter does not offer the merge again
5. [api-trace] state: Enter on an item that changed on GitHub since the card was read (new commits, now conflicting, or no longer waiting); expected: the card says what changed in words, such as "new commits were pushed since you opened it", then "Nothing was written.", and shows the item as it is now with its current choices; nothing is cut off at 72 columns
6. [api-trace] state: the failures of criteria 1, 2 and 5 in `aayu inbox --plain`; expected: the same three lines, wrapped to the terminal's width, with no line cut off

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want every way an `aayu inbox` action can fail to say, in plain words on the card, what happened, whether anything changed on GitHub, and who acts next, with the technical detail one key away, so that I never have to ask an agent what an inbox error means (@rahulnakmol, 2026-10-19: "error message make hard for founder to understand what failed").

On 2026-10-19 #661's ship from the inbox showed `#661 not done: PUT https://api.github.com/repos/…/pulls/661/merge: 405 Pu…`, cut off at the terminal's width; after a rebase it showed `#661: it changed on GitHub since you read it, so nothing was written; here it is as it…`, again cut off, with no cause and no next step. Both texts are Go error strings (`inboxFail.err` and `errInboxChanged` in internal/cli/inbox.go and inboxwork.go) put straight on the card or the status line. A merge that landed and was followed by a failed step (the Story's record or the ledger) also shows as "not done".

Replaces #793 for its failure wording (criteria 2 and 4); #796 delivers its criterion 1, and its `/aayu review again` criterion is a separate Bug.

### Completion Goal

1. [screenshot] state: `aayu inbox` at 120x34 and at 72x34 after Enter on a Ready card whose merge GitHub refuses because the PR conflicts with dev; expected: the card body shows three lines, "#<n> wasn't merged: it conflicts with dev.", "Nothing was changed on GitHub.", and "Next: its builder rebases it onto dev; nothing for you yet."; the status line says "✗ #<n> not shipped; the card says why."; no line is cut off, and none of the three names a URL, an HTTP method, a status code, a SHA, or Go error text
2. [api-trace] state: each failure in the inventory table of this Story's PR (GitHub refusing a write with 401, 403, 404, 405, 409 or 422, a rate limit, a server error, a network error, a write that got no answer, each refusal of `aayu ship`, a Decision accept refused, a review approval refused, a tidy-up refused, an item that changed since it was read, a refresh that fails, and each sign-in failure); expected: for every row, the inbox gives a what-happened line, a line saying whether anything changed on GitHub, and a next line naming who acts, with none of the technical words of criterion 1 in them
3. [screenshot] state: a failed card, then d, then d again; expected: d shows the technical detail under the three lines (the step it stopped at, GitHub's own message, the request, and the commit), and d again hides it; the log printed when the inbox quits keeps the full detail
4. [api-trace] state: Enter on a Ready card whose merge succeeds and whose later step (the Story's record or the ledger write) fails; expected: the card shows "✓ Merged #<n> into dev", then a warning that names the step that did not finish and who acts next; the item is not shown as "not done" and Enter does not offer the merge again
5. [api-trace] state: Enter on an item that changed on GitHub since the card was read (new commits, now conflicting, or no longer waiting); expected: the card says what changed in words, such as "new commits were pushed since you opened it", then "Nothing was written.", and shows the item as it is now with its current choices; nothing is cut off at 72 columns
6. [api-trace] state: the failures of criteria 1, 2 and 5 in `aayu inbox --plain`; expected: the same three lines, wrapped to the terminal's width, with no line cut off

### Measure

Inbox failures in a week whose meaning the person had to ask an agent about.

### Threshold

0.

### Scenario

@rahulnakmol presses Enter on #661, reads "#661 wasn't merged: it conflicts with dev. Nothing was changed on GitHub. Next: its builder rebases it onto dev; nothing for you yet.", presses Tab, and ships the next card without asking anyone.

### Environment

`aayu inbox` in the orb's Terminal tab, at 120 and 72 columns, and `aayu inbox --plain`, on rahulnakmol/agentic-ayudha.

### Technical Owner

ux-researcher-evolutionist

### Links

### Checkpoints

1. `go test -race ./internal/cli -run 'TestInbox'` passes with one case per row of the inventory, each asserting the three lines and the absence of URLs, HTTP methods, status codes, SHAs and Go error text (criteria 1, 2, 6)
2. `go test -race ./internal/cli -run 'TestInbox'` passes with a merge that lands before a later step fails, an item changed since it was read, and d on a failed card (criteria 3, 4, 5)
3. Renders of the real failed card, its details, the partial success, and the changed-since-read card at 120x34 and 72x34 are in the Amp thread (criteria 1, 3, 4, 5)

### Assumptions

The words of each failure come from what GitHub and aayu already report (the status code, GitHub's message, the step that stopped); no new GitHub call is made to explain a failure. The inventory covers every card kind on dev when the PR opens; a kind added later (such as #661's audit card) gets its rows when it lands.

### Open questions

### Parked ideas

### Provenance

Request: "error message make hard for founder to understand what failed"
1. stated (request): "error message make hard for founder to understand what failed"
2. stated (behaviour): "do a broader research what all type of failure exist and make it founder friendly"
3. stated (behaviour): "some text are too nerdy and technical"
4. stated (request): "error message make hard for founder to understand what failed"
5. stated (request): "error message make hard for founder to understand what failed"
6. stated (request): "error message make hard for founder to understand what failed"
````
