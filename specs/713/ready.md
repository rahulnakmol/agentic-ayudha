# rahulnakmol/agentic-ayudha#713: aayu inbox: the grouped list beside the card, aligned to the cell, with writes in the background and their progress in the body

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `f101ce67e27e4729f4ff3e7f9940671bab5da6cb21ba59df6bace22fa9bcb939` |
| Completion Goal hash | `84f005f65565c318b9b4c51da9b1f04c130734d4c20008671b9be82bf9b69016` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/713/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: `aayu inbox` at 120x34 with two Decisions, a Ready PR, a human-review PR, a capped PR, seven housekeeping items, and one item done this session; expected: the list is on the left in the groups Needs you now, Waiting on the review cap, Housekeeping, and Done this session, each with its count, and each row shows its icon, number, title, why it waits, and age; the card on the right shows the full title, the full why, the twin's counsel, and the numbered choices, each choice on one line with "recommended" at the card's right edge; the key bar is on the last line
2. [screenshot] state: the same inbox at 72x34; expected: the panes stack, the list first, then a rule, then the card; the columns and the key bar are shown
3. [metric] state: the full screen rendered at 120x34 and at 72x30 with rows selected, being written, queued, done, failed, gone, and listed again, with wide icons and titles longer than their column; expected: every line is exactly the terminal's width, the pane divider is at the same display column on every body line, each column header starts at the first cell of its column, the number, title, why, and age columns start at the same display column on every list row, a long choice label wraps under its own text with the tag on its first line, and a help line's description wraps under the description column; 0 lines out of place
4. [api-trace] state: x or space on an item that needs you, then X, then Enter with a selection, then C; expected: x and space select nothing and the status line says only housekeeping can be selected; X selects every housekeeping item that can be and no item that needs you; the confirmations list only housekeeping; nothing is written until Enter on the confirmation
5. [api-trace] state: Enter on a write that is still running after the next keypress, then Enter on another item, before the first ends; expected: the cursor moves on at once, the second write runs alongside the first and finishes first, a write on the same pull request as a running one waits until it ends, Enter on the item being written is refused with the reason in the status line, and at most two writes run at once
6. [screenshot] state: an item's card while its write runs, after it is done, and after it stopped part-way; expected: while it runs the card body shows a plain headline, what is happening now as a sentence, the steps as a checklist, the usual time with the calls and seconds so far, and that other items can be acted on meanwhile; once done it shows what changed on GitHub with a link and what happens next; once stopped it shows the step it stopped at, the error, and only what GitHub answered as written
7. [api-trace] state: q or Ctrl-C while writes run, then w, then q; expected: the inbox asks instead of quitting; w quits once every write has finished; q quits once the writes under way finish and starts none of the queued ones; no write is stopped half done
8. [api-trace] state: an item done in one session, then a new session opened from the last read, then a read of GitHub begun after the write that still lists it; expected: the new session shows it in Done this session with who did it and when, does not count it as its own work, and never runs it again until that later read marks it listed again
9. [api-trace] state: Tab and Shift-Tab from a housekeeping row and from an item that needs you; expected: the cursor moves to the next or previous item that needs you, skipping housekeeping, done, and gone items

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Replaces #706: the founder added two must-haves after #706 was Ready, so its criterion 5 (the status line names the stage, a second Enter queues behind the first) no longer describes the build.

As the founder, I want `aayu inbox` to show the grouped list beside the card with every column aligned, to show what my action is doing in the body of the screen where my eye already is, and to let me act on other items while a write runs, so that I clear my inbox in minutes instead of waiting on each write and never wonder what a keypress did (@rahulnakmol, 2026-10-19).

The frame is concept A's (the list on the left, the card on the right, the key bar always shown, the list above the card on a narrow terminal) with concept C's list in it: the groups Needs you now, Waiting on the review cap, Housekeeping, and Done this session with counts, the why-it-waits and age columns, and multi-select with a bulk bar for housekeeping only. Spacing is a requirement at the level of behaviour: every column, header, and the pane divider start at the same cell on every row, measured in display cells. Writes run in the background, up to two at once, and never two on the same pull request or issue at once. While a write runs, its card body is its progress in plain words; when it ends, the same body is the result.

### Completion Goal

1. [screenshot] state: `aayu inbox` at 120x34 with two Decisions, a Ready PR, a human-review PR, a capped PR, seven housekeeping items, and one item done this session; expected: the list is on the left in the groups Needs you now, Waiting on the review cap, Housekeeping, and Done this session, each with its count, and each row shows its icon, number, title, why it waits, and age; the card on the right shows the full title, the full why, the twin's counsel, and the numbered choices, each choice on one line with "recommended" at the card's right edge; the key bar is on the last line
2. [screenshot] state: the same inbox at 72x34; expected: the panes stack, the list first, then a rule, then the card; the columns and the key bar are shown
3. [metric] state: the full screen rendered at 120x34 and at 72x30 with rows selected, being written, queued, done, failed, gone, and listed again, with wide icons and titles longer than their column; expected: every line is exactly the terminal's width, the pane divider is at the same display column on every body line, each column header starts at the first cell of its column, the number, title, why, and age columns start at the same display column on every list row, a long choice label wraps under its own text with the tag on its first line, and a help line's description wraps under the description column; 0 lines out of place
4. [api-trace] state: x or space on an item that needs you, then X, then Enter with a selection, then C; expected: x and space select nothing and the status line says only housekeeping can be selected; X selects every housekeeping item that can be and no item that needs you; the confirmations list only housekeeping; nothing is written until Enter on the confirmation
5. [api-trace] state: Enter on a write that is still running after the next keypress, then Enter on another item, before the first ends; expected: the cursor moves on at once, the second write runs alongside the first and finishes first, a write on the same pull request as a running one waits until it ends, Enter on the item being written is refused with the reason in the status line, and at most two writes run at once
6. [screenshot] state: an item's card while its write runs, after it is done, and after it stopped part-way; expected: while it runs the card body shows a plain headline, what is happening now as a sentence, the steps as a checklist, the usual time with the calls and seconds so far, and that other items can be acted on meanwhile; once done it shows what changed on GitHub with a link and what happens next; once stopped it shows the step it stopped at, the error, and only what GitHub answered as written
7. [api-trace] state: q or Ctrl-C while writes run, then w, then q; expected: the inbox asks instead of quitting; w quits once every write has finished; q quits once the writes under way finish and starts none of the queued ones; no write is stopped half done
8. [api-trace] state: an item done in one session, then a new session opened from the last read, then a read of GitHub begun after the write that still lists it; expected: the new session shows it in Done this session with who did it and when, does not count it as its own work, and never runs it again until that later read marks it listed again
9. [api-trace] state: Tab and Shift-Tab from a housekeeping row and from an item that needs you; expected: the cursor moves to the next or previous item that needs you, skipping housekeeping, done, and gone items

### Measure

List rows and lines whose columns, headers, choices, or pane divider are out of place, in the full-screen render at 120x34 and 72x30 with every row state.

### Threshold

0.

### Scenario

@rahulnakmol opens `aayu inbox` in the orb's Terminal tab, ships a Ready PR, moves on at once to accept a Decision while the ship runs, clears seventeen housekeeping items in the background, and reads on each card what happened.

### Environment

`aayu inbox` in a person's terminal at 120 columns and at 72, themes auto and named.

### Technical Owner

ux-researcher-evolutionist

### Links

### Checkpoints

1. `go test -race ./internal/cli -run 'TestInboxTUI|TestInboxAlignment|TestInboxSelectsOnlyHousekeeping|TestInboxWritesInBackground|TestInboxWriteFailures|TestInboxAcrossWrites|TestInboxRelisted|TestInboxMotion'` passes (criteria 1, 2, 3, 4, 5, 6, 7, 8, 9)
2. Renders of the real TUI at 120x34 and 72x34 (the six states, the moment after Enter, mid-write, done, failed, two writes in flight, a bulk clear, and an alignment close-up) are in the Amp thread (criteria 1, 2, 3, 6)

### Assumptions

The terminal measures emoji icons as two cells, as `ansi.StringWidth` and tmux do. The GitHub client, the actions log, and the ledger writer are safe for two writers at once (a write mutex, a mutex, and a file lock).

### Open questions

deferred: should the number of writes at once be configurable? (two keeps GitHub's secondary rate limits far away; can wait for use)

### Parked ideas

The device-flow login as the inbox's first screen (a separate PR, because presence.go is in #593's narrow set).

### Provenance

Request: "it needs to surface more in the body, and intuitively. it takes minutes to move from one to another, can't this be async. I would say C within A is right choice but pixel perfection in padding and spacing and alignment is also key. needs you cannot have multi select."
1. stated (behaviour): "I would say C within A is right choice"
2. stated (behaviour): "I would say C within A is right choice"
3. stated (behaviour): "pixel perfection in padding and spacing and alignment is also key"
4. stated (behaviour): "needs you cannot have multi select"
5. stated (behaviour): "it takes minutes to move from one to another, can't this be async"
6. stated (behaviour): "it needs to surface more in the body, and intuitively"
7. default (behaviour): "it takes minutes to move from one to another, can't this be async"
8. default (behaviour): "I would say C within A is right choice"
9. default (behaviour): "I would say C within A is right choice"
````
