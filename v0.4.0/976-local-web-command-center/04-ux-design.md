# UX design: Feature #976

## Read this first

**None of the web screens below is built.** Dev at bc78349 (884d81b plus #1238) has no web server, no templates, and no
`internal/cli/inboxweb/` directory. The `inbox` verb has one flag, `--list` (`internal/verbs/registry.go`,
`plumb("inbox", …)`). There is no UX design baseline on the ledger (`specs/976/` does not exist on `origin/ledger`).

The repository holds no design files for these screens. `design/` has only `design/brand.yaml`, the docs brand
(voice, a docs palette, WCAG pairs). Open PR [#817](https://github.com/rahulnakmol/agentic-ayudha/issues/817)
("docs(design): add the aayu iconography: App marks, seat emblems, and verb icons", opened 2026-10-20, not merged)
would add `design/iconography.md` and SVG icons under `design/icons/`. The Story bodies cite "UX proposals v27",
"interaction specs v4" (touchpoints TP1 to TP89), and prototypes "v43" and "v44". None of these is in the repository
or on the ledger, so I could not check them. Every screen below is described from the Completion Goals of the Story
issues, which quote the exact words. Viewports: 1280x800 (laptop), 390x844 (phone), 120x34 (terminal).

## Starting the web face ([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957))

Planned terminal output of `aayu inbox --web`:

| Where | What the person sees |
|---|---|
| This computer | "Opened the inbox in your browser." and "It stops when you quit aayu (Ctrl+C)." (c1) |
| Over SSH | No browser opens. The line `ssh -L 7431:127.0.0.1:7431 <host>`, then "Then open:  `http://localhost:7431/#t=<one-time code>`"; `c` copies it (c2) |
| In an Amp orb (`AMP_ORB` set) | The portal link with its one-time code and "Only you and the people this thread is shared with can open it." (c3) |

## Sign-in and the session ([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957))

- First page: "Prove it is you: open github.com/login/device and enter `<code>`.", a Copy code button, and the inbox
  list behind it, read-only (c4).
- After sign-in: a header pill "Writes armed · 59 m", with the accessible name "Writes armed · @rahulnakmol · 59 m
  left". The footer shows no time (c5).
- After 24 hours, on the card: "Your session ended 24 hours after sign-in. Sign in again to act. You can still read."
  with a Sign in button (c6).
- After 61 minutes with no action: "Your session ended after 1 hour with no action. Sign in again to act. You can
  still read." with a Sign in button (c7).

## The inbox screen ([#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958), [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959), [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961))

Laptop layout, from top to bottom:

1. Header with the "Writes armed" pill and the live pill "● Live · refreshed 4 s ago" (#959 Provenance, ux27/TP37).
   When the stream drops the pill turns amber: "Reconnecting… You see data from 11:42." (#959 c2).
2. Now bar, on every view: "RELEASE v0.4.0 base camp · build · 20 of 55 merged · founder minutes per merged PR: not
   measured yet · 3 missions · 1 running, 1 on hold, 1 shaping · 20 wait on you · 11:42 PR #825 round 3 of 3 · Drona ·
   Activity ›". Activity opens the activity panel (#961 c8).
3. Overview tiles: "WORKING 2 · BLOCKED 1 · SHIPPED TODAY 3 · IN BUILD 5". A click or key on a tile filters the list.
   The Inbox nav item shows no count (#961 c1).
4. Nav: Inbox, Build, Issues, Team, Lineage, Releases, History, Settings (#961 c8). A chevron collapses it to an icon
   rail with count dots; the rail stays after reload on this device (#961 c5).
5. The list and the card side by side. On the card: counsel first, above the brief and the choices, in a panel
   labelled "counsel, not approval" with the seat emblem; buttons that show their keys; Approve and ship fixed in view
   (#958 c2 and Provenance ux27/TP9).
6. Lineage rail on the card: "Story #796 › PR #802 › proof ✓ › ship › v0.4.0"; each chip opens its item (#963 c1).

Behaviour:

- Cards change in place within 60 s; changed values highlight for 2 s; focus and cursor do not move (#959 c1).
- While the tab is hidden: one browser notification "#811 needs you" and the tab title "(2) aayu inbox". Nothing
  while it has focus (#958 c4).
- Batch ship: `x` on Ship cards shows checkboxes and the bar "3 selected · Ship 3", then a 5-second hold (#958 c5).
- Every named object (issue, PR, story, mission, release, seat, check, session, file) opens a side sheet on a laptop
  and a full-screen sheet on a phone, with `open=<kind>:<id>` in the URL. Back and Esc close it. "Open on GitHub ↗" is
  the only way out to github.com. A card is one link and holds no nested control (#961 c9, c10).

## Screen sizes ([#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c6, c7)

| Width | Layout |
|---|---|
| 390 px | One column with a bottom sheet; the now bar is one line, "v0.4.0 36% · 3 missions · 20 wait on you ›", and never covers the deck's buttons |
| 800 px | Rail, list, and detail |
| 1280 px | Rail, list, and detail |
| 3440 px | Adds a Team column |
| Terminal under 100 columns | Panes stack |
| Terminal at 180 columns or more | Adds a Team column |

## Team view ([#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c2–c4)

One row per seat, for example "Nakul · working · #830 · 12 m" and "Drona · waits on you · #802 · 25 m", and the flow
strip "shape › build › prove › ship › release". Each person's row names their tool, such as "@rahulnakmol · Claude
Code". `t` opens the same view in the terminal.

## The PR inside the app ([#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962))

Tabs "Brief · Files changed 6 · Conversation 4 · Checks 8/8 · Commits 3", with the Approve and ship bar fixed in
view. The diff is escaped text. A 900-line file and a binary fold with "Open on GitHub". In the terminal, `d` shows
the details rows and `o` opens GitHub. Read-only; no inline review comments.

## Lineage ([#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963))

- A bug from Feedback: "Feedback #770 › Bug #801 › PR #799 › shipped v0.3.1" (c2).
- A PR that names no Story: "This PR names no Story; ship refuses it: add Refs #n" (c3).
- A Lineage view (`L` in the terminal): the tree for the initiative, each node with its state (c4).

## Issues, Releases, and Lanes ([#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964))

- Issues: type chips "Goal · Initiative · Feature · Story · Bug · Decision · Experiment · Release · Feedback ·
  Incident" and a peek panel; space peeks in the terminal (c1).
- Releases: a timeline "7 releases · 3 shipped · 1 in progress · 3 planned" (c2).
- A release filter on every view; `[` and `]` step between releases on both faces (c3).
- Lanes: one swimlane per workstream, such as "Inbox · Hanu · Initiative #840", and "Unassigned" (c4).

## One release whole ([#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983))

- Tree view: a level table (Goal, Initiative, Features, Missions, Stories, Bugs, Decisions, Feedback), each with a
  count and a state bar "planned · ready · building · merged · done"; budget and dates in the header (c1).
- Slice tabs "Stories", "Bugs", "Features", "Missions", "Decisions", "Feedback", "Experiments", "Incidents", each with
  a count. Filters for Feature, wave, mission, status, priority, effort, risk, seat and tool. The slice, view, filters
  and grouping are in the URL (c3).
- Four views: Tree, Board, List, Gantt; `v` cycles them (c4).
- Gantt: Feature rows that open to missions and stories, bars from start to merge, depends-on arrows, the critical
  path, a today line, milestone diamonds, and the 24 h and 48 h marks. A planned bar is dashed with "estimate"; a story
  with no estimate is a dot with "no estimate" (c5, c6).
- Scope drift: "Scope drift since the baseline (2026-10-21, 31 items)"; "Feedback not yet assigned" with Route; "No
  Experiments in v0.4.0." with its next step (c8).
- Phone: slices and filters open as a sheet ("Slice", "Filters · n"); the Gantt becomes a list grouped by Feature (c10).
- Keys: ↑ ↓ rows, → open, ← back, Enter sheet, + and − zoom, `t` today, `[ ]` release, `< >` slice, `v` view, `f`
  filters; "Show as a table" gives the same rows for screen readers (c11).

## The Mission page ([#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960))

Laptop, three columns:

1. Sessions: "Coordinator · Kanha" first, then each story thread by milestone (number, title, tool, state word, age),
   rows that wait on you first, then Reviews: "CI review · rung B", "inspect", and the milestone gate.
2. The mission's record (Record tab): "Needs attention", "Summary", "Waits on you · 1" with the card inline,
   "Validation", "Commits and pull requests", "Latest progress comment". No host transcript; "Open session ↗".
3. "Stories by milestone 2/10" and "Progress log".

Header: "Release #906 v0.4.0 › Feature #975", "Mission #840 · Inbox command center", Time "9 h 12 m working · 31 h
elapsed" with the 24 h and 48 h marks, Progress "2 of 10 stories merged", Spend "$214 of $660 · pay as you go", and a
status bar with the state word "running" (c1, c2).

Tabs:

- Shape: seven stages; the waiting stage's tile "A3 technical design: waits on you" with "Approve A3", "Open A3", and
  "Ask why" (c3).
- Deliver: views Board, List and Deck; Board columns "Queued", "Working", "Waiting on you", "Ready for review /
  shipped"; the track line "M1 contract · M2 Feature done · M3 polish · 5 of 5 slots" (c4).
- Phone: tabs "Sessions · Mission · Progress", one column at a time (c4).

Command bar: no free text ("No free text: a running mission takes no new input."). It lists only "Stop this story
s", "Stop the mission S", "Extend, with a reason e", "Add a Tatkal story a", "Take over T", "Open session o". Extend
asks for one line, then presence. `S` asks for a reason, the key again, then presence. Esc cancels (c5).

Take over: copies `claude --teleport <id>` or `copilot --resume <id>`, or opens the Amp session URL; the tile says
"Opened #851's session · you work in it now" (c7). An unreadable tile: "#1244 Brief lists merged PRs" in an error
state with "Try again" and "Open session" (c9).

## Themes and Settings ([#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965))

- Default: Aruna Light or Aruna Dark by the system theme, before first paint, with no flash (c1).
- Settings view: "Theme · Motion · Density · Keys"; Keys lists the fixed keys and a switch that turns single-character
  keys off (c6).
- Import: "Imported “Monokai Pro” from a VS Code theme: 14 of 16 roles mapped; 2 use the nearest Aruna colour.
  Contrast AA: pass." (c5).
- Terminal with no `[ui] theme`: the terminal's own colours; `aayu doctor` says "Theme: your terminal (auto)." (c4).
- Icons: a line SVG in the role colour on the web, `aria-hidden`; a one-cell symbol in the terminal; the word or
  twin name always shown (c9, c10). No deity figures or sacred marks (M2 A, Assumptions).

## Command bar ([#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966))

- `:` in the terminal, ⌘K on the web: "Type a command or search: approve 802 · ship ready · skip 825 · open 796" (c1).
- "ship 802 and 811" gives two preview lines: "Will do: approve and ship #802. Nothing happens until you press Enter
  on the card." (c2).
- Text it cannot place: "not understood" with the words it could not place (c4).
- On a view, its own actions come first, each with its key, such as "Go to Releases g r", and a guard such as "(a
  one-line reason, the key again, then the presence check)" (c5).

## Empty states ([#967](https://github.com/rahulnakmol/agentic-ayudha/issues/967))

- "Nothing needs you." / "3 agents are working. The next card is likely #817 (its review ends in about 20 min)." /
  "Checked 31 items at 09:14; the next read is at 09:29." (c1).
- "No items match these filters." with Clear filters (c2).
- "No aayu work items in this repository yet. aayu init sets it up." (c3).
- Other views: a fact, its time, and one next step, such as "No release planned; aayu scope drafts one" (c4).

## Keyboard and screen reader ([#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968))

- The list is a listbox with roving focus; dialogs trap focus and give it back (c2).
- A ship result is read in a polite live region, a refusal in an assertive one (c3).
- `aayu inbox --plain` reads every card and its choices in order. `--plain` is a global flag today
  (`GlobalFlags` in `internal/verbs/registry.go`), and the terminal inbox already has a line mode (`plainInbox`,
  `internal/cli/inbox.go`).
- 200% text cuts no text (c5).
- Never bound: ctrl+L, ctrl+T, ctrl+W, and the screen readers' keys. No single-character key fires inside a text
  field. AltGr keys act on the character (c6).

## Agent-facing surface

Agents keep `aayu inbox --list` (Markdown, or JSON with `--json`), which "writes nothing, needs no terminal, and an
agent may run it" (`internal/verbs/registry.go`). The web face is for a person: every write needs the device flow
([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c4).
