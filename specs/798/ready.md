# rahulnakmol/agentic-ayudha#798: aayu inbox celebrates a cleared housekeeping set and an emptied inbox with a short confetti burst and a job well done line

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `463249faf564591ecf3fe5e5a2e5137be3a6a208c41adaf583c496df5a49df6c` |
| Completion Goal hash | `33e5d2a1f8f7a7d45ac0a838e639c6caf374ffc81c995afb634811a50274ce8a` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/798/ready.md` |

Completion Goal at Ready:

1. [recording] state: `aayu inbox` at 120x34 with motion on and three housekeeping items, then C and Enter, and every item succeeds; expected: a confetti burst plays over the card pane for at most 1.5 seconds, the status line says "Housekeeping cleared: 3 tidied. Job well done." and the card shows one short line in the Auditor twin's voice; a key pressed during the burst acts at once, and the burst never covers the key bar
2. [recording] state: `aayu inbox` at 120x34 with motion on and one item that needs you and no housekeeping, then Enter, and the write succeeds; expected: the same burst plays once, and the card says "Inbox zero. Job well done." with one short line in the Founder's Sage twin's voice
3. [screenshot] state: the celebrations of criteria 1 and 2 with `[ui] motion = false`, with `NO_COLOR=1`, and in `aayu inbox --plain`; expected: no animation and no confetti characters; the same words show, as one plain line in --plain
4. [api-trace] state: C and Enter on three housekeeping items where one write fails; expected: no burst and no "Job well done"; the status line says "Applied 3: 2 done, 1 not done." as today
5. [api-trace] state: `aayu inbox` opened with nothing waiting; expected: no burst, since nothing was cleared in this session; it says "Nothing waits for you." as today

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want `aayu inbox` to cheer briefly when I clear a set of housekeeping or empty the inbox, with a confetti burst on the terminal and a "job well done" line with a little sass, so that finishing the queue feels like finishing something (@rahulnakmol, 2026-10-19: "also add some sass when we clear a set of housekeeping may be a confetti to show or say job well done and so forth").

Today a cleared batch says only "Applied 5: 5 done, 0 not done. Written changes are not undone here." (`finished` in internal/cli/inboxtui.go), and an inbox with nothing left shows "Nothing waits for you." The inbox already has short motion (a sliding card, a gliding highlight, a ✓ flash) that `[ui] motion = false` and `NO_COLOR` keep still, and `ui.Printer.Celebrate` marks milestones elsewhere in the CLI.

### Completion Goal

1. [recording] state: `aayu inbox` at 120x34 with motion on and three housekeeping items, then C and Enter, and every item succeeds; expected: a confetti burst plays over the card pane for at most 1.5 seconds, the status line says "Housekeeping cleared: 3 tidied. Job well done." and the card shows one short line in the Auditor twin's voice; a key pressed during the burst acts at once, and the burst never covers the key bar
2. [recording] state: `aayu inbox` at 120x34 with motion on and one item that needs you and no housekeeping, then Enter, and the write succeeds; expected: the same burst plays once, and the card says "Inbox zero. Job well done." with one short line in the Founder's Sage twin's voice
3. [screenshot] state: the celebrations of criteria 1 and 2 with `[ui] motion = false`, with `NO_COLOR=1`, and in `aayu inbox --plain`; expected: no animation and no confetti characters; the same words show, as one plain line in --plain
4. [api-trace] state: C and Enter on three housekeeping items where one write fails; expected: no burst and no "Job well done"; the status line says "Applied 3: 2 done, 1 not done." as today
5. [api-trace] state: `aayu inbox` opened with nothing waiting; expected: no burst, since nothing was cleared in this session; it says "Nothing waits for you." as today

### Measure

Housekeeping batches cleared in full, and sessions that end with the inbox empty, that show no celebration line.

### Threshold

0.

### Scenario

@rahulnakmol presses C and Enter on five housekeeping items, sees confetti for a moment and "Housekeeping cleared: 5 tidied. Job well done.", and goes on to the next card while it fades.

### Environment

`aayu inbox` in the orb's Terminal tab at 120 columns, with `[ui] motion` on and off, with `NO_COLOR=1`, and with --plain, on rahulnakmol/agentic-ayudha.

### Technical Owner

ux-researcher-evolutionist

### Links

### Checkpoints

1. `go test -race ./internal/cli -run 'TestInbox'` passes with a full batch, a batch with one failure, an emptied inbox, and an inbox that opens empty (criteria 1, 2, 4, 5)
2. `go test -race ./internal/cli -run 'TestInbox'` passes with motion off, NO_COLOR and --plain showing the words with no animation (criterion 3)
3. Renders of the real burst's frames and the still versions at 120x34 and 72x34 are in the Amp thread (criteria 1, 2, 3)

### Assumptions

The twins' lines are short, kind, and never about a failure; their exact words are picked in the PR and shown in its renders, and a line that misses the tone is changed in review. A Unicode terminal draws the confetti; NO_COLOR and --plain draw none.

### Open questions

answered: the burst shows on a cleared housekeeping set and on an emptied inbox, never on a failure (the request names housekeeping; "and so forth" is read as the inbox reaching empty).

### Parked ideas

Sound: a terminal bell with the burst.

### Provenance

Request: "also add some sass when we clear a set of housekeeping may be a confetti to show or say job well done and so forth"
1. stated (request): "also add some sass when we clear a set of housekeeping may be a confetti to show or say job well done"
2. stated (request): "say job well done and so forth"
3. stated (request): "may be a confetti to show or say job well done"
4. stated (request): "when we clear a set of housekeeping"
5. stated (request): "when we clear a set of housekeeping"
````
