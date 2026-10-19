# rahulnakmol/agentic-ayudha#706: aayu inbox shows the grouped list beside the card, with every column aligned and multi-select for housekeeping only

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `56510ef492850f2173f88e06f1fc73978cfd434e4ee69ae1d7ca6b6360069cf9` |
| Completion Goal hash | `932587faf1d462c29c8444549d9cce1a62611789d24e01286d59bca9f2845fae` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/706/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: `aayu inbox` at 120x34 with two Decisions, a Ready PR, a human-review PR, a capped PR, seven housekeeping items, and one item done this session; expected: the list is on the left in the groups Needs you now, Waiting on the review cap, Housekeeping, and Done this session, each with its count, one blank line between groups, and each row shows its icon, number, title, why it waits, and age; the card on the right shows the full title, the full why, the twin's counsel, and the numbered choices; the key bar is on the last line
2. [screenshot] state: the same inbox at 72x34; expected: the panes stack, the list first, then a rule, then the card; the columns and the key bar are shown, and the card keeps its choices and what the selected one does
3. [metric] state: the full screen rendered at 120x34 and at 72x30 with rows selected, being written, queued, done, failed, gone, and listed again, with wide icons and titles longer than their column; expected: every line is exactly the terminal's width, the pane divider is at the same display column on every body line, the number, title, why, and age columns start at the same display column on every list row, and the head, the group headers, and the key bar start at the pad column; 0 lines out of place
4. [api-trace] state: x or space on an item that needs you, then X, then Enter with a selection, then C; expected: x and space select nothing and the status line says the item needs you and only housekeeping can be selected; X selects every housekeeping item that can be and no item that needs you; the confirmations list only housekeeping; nothing is written until Enter on the confirmation
5. [api-trace] state: Enter on a Ready card, then j, k, and Enter on another item while the write runs; expected: the screen stays live, the row says writing now, the card shows the write's steps as a checklist with the current step, the status line names the stage, and the second Enter queues its item
6. [api-trace] state: an item done in one session, then a new session opened from the last read, then a read of GitHub begun after the write that still lists it; expected: the new session shows it in Done this session with who did it and when and never runs it again, until that later read marks it listed again and Enter asks before doing it again
7. [api-trace] state: Tab and Shift-Tab from a housekeeping row and from an item that needs you; expected: the cursor moves to the next or previous item that needs you, skipping housekeeping, done, and gone items

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want `aayu inbox` to show the grouped triage list inside the two-pane frame, with every column lined up to the cell, so that I can scan what needs me, why it waits, and how old it is, and clear the housekeeping in one go without ever bulk-acting on a step only I should decide (@rahulnakmol, 2026-10-19, picking concept C inside concept A).

Today the full-screen inbox (`internal/cli/inboxtui.go`) shows one card with a slim list beside it, the housekeeping folded into one line, and `A` to tidy it all. The concept review found that A's pipe bar was not aligned on every row, and that C's details panel was cramped, its writing row crowded, and its selected rows dense. This Story builds A's frame (the list on the left, the card on the right, the key bar always shown, the list above the card on a narrow terminal) with C's list in it: the groups Needs you now, Waiting on the review cap, Housekeeping, and Done this session with counts; the why-it-waits and age columns; and multi-select with a bulk bar for housekeeping only. Spacing is a requirement at the level of behaviour: every column and the pane divider start at the same cell on every row, measured in display cells, and every cell is cut to its width. It keeps the defect fixes of the same branch: the screen stays live during a write, a done item says who did it, the cache keeps done items across sessions, and Tab and Shift-Tab cycle the items that need you.

### Completion Goal

1. [screenshot] state: `aayu inbox` at 120x34 with two Decisions, a Ready PR, a human-review PR, a capped PR, seven housekeeping items, and one item done this session; expected: the list is on the left in the groups Needs you now, Waiting on the review cap, Housekeeping, and Done this session, each with its count, one blank line between groups, and each row shows its icon, number, title, why it waits, and age; the card on the right shows the full title, the full why, the twin's counsel, and the numbered choices; the key bar is on the last line
2. [screenshot] state: the same inbox at 72x34; expected: the panes stack, the list first, then a rule, then the card; the columns and the key bar are shown, and the card keeps its choices and what the selected one does
3. [metric] state: the full screen rendered at 120x34 and at 72x30 with rows selected, being written, queued, done, failed, gone, and listed again, with wide icons and titles longer than their column; expected: every line is exactly the terminal's width, the pane divider is at the same display column on every body line, the number, title, why, and age columns start at the same display column on every list row, and the head, the group headers, and the key bar start at the pad column; 0 lines out of place
4. [api-trace] state: x or space on an item that needs you, then X, then Enter with a selection, then C; expected: x and space select nothing and the status line says the item needs you and only housekeeping can be selected; X selects every housekeeping item that can be and no item that needs you; the confirmations list only housekeeping; nothing is written until Enter on the confirmation
5. [api-trace] state: Enter on a Ready card, then j, k, and Enter on another item while the write runs; expected: the screen stays live, the row says writing now, the card shows the write's steps as a checklist with the current step, the status line names the stage, and the second Enter queues its item
6. [api-trace] state: an item done in one session, then a new session opened from the last read, then a read of GitHub begun after the write that still lists it; expected: the new session shows it in Done this session with who did it and when and never runs it again, until that later read marks it listed again and Enter asks before doing it again
7. [api-trace] state: Tab and Shift-Tab from a housekeeping row and from an item that needs you; expected: the cursor moves to the next or previous item that needs you, skipping housekeeping, done, and gone items

### Measure

List rows and lines whose columns or pane divider are out of place, in the full-screen render at 120x34 and 72x30 with every row state.

### Threshold

0.

### Scenario

@rahulnakmol opens `aayu inbox` in the orb's Terminal tab with four items that need him and seventeen housekeeping items, reads why each waits, presses X and Enter to clear the housekeeping, and ships the Ready PR from its card.

### Environment

`aayu inbox` in a person's terminal at 120 columns and at a narrow width, themes auto and named.

### Technical Owner

ux-researcher-evolutionist

### Links

### Checkpoints

1. `go test ./internal/cli -run 'TestInboxTUI|TestInboxAlignment|TestInboxSelectsOnlyHousekeeping|TestInboxAcrossWrites|TestInboxRelisted|TestInboxMotion'` passes (criteria 1, 2, 3, 4, 5, 6, 7)
2. Renders of the real TUI at 120x34 and 72x34 in six states and an alignment close-up are attached to the PR (criteria 1, 2, 3)

### Assumptions

The terminal measures emoji icons as two cells, as `ansi.StringWidth` and tmux do; the renders are captured through tmux to check it.

### Open questions

deferred: should Done this session collapse after a number of rows? (the session list stays short in practice; can wait for use)

### Parked ideas

A per-column sort (by age or by group) in the list.

### Provenance

Request: "I like C and the multi select option. needs you cannot have multi select. issue with A is formatting and alignment, the pipe bar isn't aligned on all rows. I would say C within A is right choice but pixel perfection in padding and spacing and alignment is also key."
1. stated (behaviour): "I would say C within A is right choice"
2. stated (behaviour): "I would say C within A is right choice"
3. stated (behaviour): "pixel perfection in padding and spacing and alignment is also key"
4. stated (behaviour): "needs you cannot have multi select"
5. default (behaviour): "I would say C within A is right choice"
6. default (behaviour): "I would say C within A is right choice"
7. default (behaviour): "I would say C within A is right choice"
````
