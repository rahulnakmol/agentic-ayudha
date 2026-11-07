# Specification: Feature #976

## Source and status

There is no spec baseline on the ledger for #976 (`specs/976/` does not exist on `origin/ledger`). The requirements
below come from the Completion Goals of the 13 Story issues. They are planned, not approved, and not built. "cN"
means Completion Goal criterion N of that Story.

## Functional requirements

### Launch and session ([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957))

1. FR-1. `aayu inbox --web` with a local browser opens the browser, prints "Opened the inbox in your browser." and
   "It stops when you quit aayu (Ctrl+C).", and listens on 127.0.0.1 only (#957 c1).
2. FR-2. In an SSH session it opens no browser. It prints the `ssh -L 7431:127.0.0.1:7431 <host>` line and "Then
   open:  `http://localhost:7431/#t=<one-time code>`"; `c` copies it (#957 c2).
3. FR-3. With `AMP_ORB` set it prints the portal link with its one-time code and "Only you and the people this thread
   is shared with can open it." (#957 c3).
4. FR-4. The first page of a session asks for the device flow ("Prove it is you: open github.com/login/device and
   enter `<code>`."), with Copy code, and shows the list read-only behind it (#957 c4).
5. FR-5. After sign-in a header pill shows "Writes armed · 59 m", with the accessible name "Writes armed ·
   @rahulnakmol · 59 m left" (#957 c5).
6. FR-6. A session lasts 60 minutes after the last action and 24 hours after sign-in at most. Each action resets the
   60 minutes; background reads and the stream do not. The expiry texts are fixed (#957 c6, c7).
7. FR-7. `[inbox.web] session_idle_minutes` (default 60) and `session_max_hours` (default 24) set the two limits
   (#957 c8).

### Parity with the terminal ([#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958))

1. FR-8. One shared word list renders both faces for every state of TP1 to TP89 that has a terminal face. A test
   fails on any difference (#958 c1).
2. FR-9. The web card shows the list and the card side by side, counsel above the brief and the choices, buttons that
   show their keys, and Approve and ship fixed in view (#958 c2).
3. FR-10. A web write is `POST /items/{key}/act` with the card hash and an idempotency key. Stages stream to the card.
   A second click while it runs does nothing (#958 c3).
4. FR-11. A card that moves into needs-you while the tab is hidden gives one browser notification ("#811 needs you")
   and the title "(2) aayu inbox"; nothing while the tab has focus (#958 c4).
5. FR-12. Batch ship on the web: checkboxes, "3 selected · Ship 3", and the same 5-second hold (#958 c5).
6. FR-13. The release scope card shows the baseline or the change, the drift budget used, and the items added and
   removed (#958 c6).
7. FR-14. Closing the tab during a ship ends the ship. Log out ends the session and its approval (#958 c7).

### Always live ([#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959))

1. FR-15. A card changes in place within 60 s of a change on GitHub; changed values highlight for 2 s; focus and the
   cursor do not move (#959 c1).
2. FR-16. When the stream drops, the live pill turns amber: "Reconnecting… You see data from 11:42." (#959 c2).
3. FR-17. On wake the stream reconnects with Last-Event-ID, replays missed events, and runs one read at once (#959 c3).
4. FR-18. The terminal inbox on its timer also updates changed cards in place (#959 c5).

### The web Mission page ([#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960))

1. FR-19. Three columns: Sessions, the mission's record, and progress, under a header with the release, the Feature,
   the mission, Time, Progress, Spend, and a status bar with the state word (#960 c1).
2. FR-20. The Record tab shows Needs attention, Summary, Waits on you, Validation, Commits and pull requests, and the
   latest progress comment. No host transcript shows; "Open session ↗" opens the host (#960 c2).
3. FR-21. The Shape tab shows seven stages (Idea, Intent, Spec, Technical design, UX design, Mission plan, Release
   queue), each with its state; the waiting stage has Approve, Open, and Ask why (#960 c3).
4. FR-22. The Deliver tab has Board, List and Deck views. Board has the columns Queued, Working, Waiting on you, and
   Ready for review / shipped. At 390x844 the page has the tabs Sessions · Mission · Progress (#960 c4).
5. FR-23. The command bar takes no free text. It lists only Stop this story `s`, Stop the mission `S`, Extend with a
   reason `e`, Add a Tatkal story `a`, Take over `T`, and Open session `o`, with their guards (#960 c5).
6. FR-24. Spend follows the pay mode ("subscription · usage not measured" until a tool reports a percentage). The
   missions list has one card per mission (#960 c6).
7. FR-25. Take over copies `claude --teleport <id>` (Claude Code), `copilot --resume <id>` (Copilot), or opens the
   session URL (Amp). It posts nothing and changes no state. `T` does the same in the terminal mission view (#960 c7, c8).
8. FR-26. An unreadable session tile shows an error state with Try again and Open session, and the unreadable-source
   rule's words (#960 c9).
9. FR-27. The tool and model on session rows come from Feature [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871)'s
   records, read-only. No control changes a running mission's tool or model (#960 c10).

### The frame ([#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961))

1. FR-28. Overview tiles ("WORKING 2 · BLOCKED 1 · SHIPPED TODAY 3 · IN BUILD 5") filter the list. What waits on you is
   counted once, in the now bar (#961 c1).
2. FR-29. A Team view with one row per seat, its state and age, a flow strip, and each person's tool from
   `[repos."<owner>/<repo>"] tool`. `t` opens it in the terminal (#961 c2, c3, c4).
3. FR-30. A collapsible nav that stays a rail after reload; layouts at 390, 800, 1280 and 3440 px; terminal panes that
   stack under 100 columns and add a Team column at 180 (#961 c5, c6, c7).
4. FR-31. A now bar on every view, one line on a phone, that never covers the deck's buttons (#961 c8, c11).
5. FR-32. Every named object opens a detail sheet with `open=<kind>:<id>` in the URL; "Open on GitHub ↗" is the only
   way out. A click audit finds 0 elements with no destination (#961 c9, c10).

### PR, lineage, browse, release view

1. FR-33. The PR inside the app, read-only: tabs Brief, Files changed, Conversation, Checks, Commits; large files and
   binaries fold; GitHub text renders escaped; `d` and `o` in the terminal ([#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962) c1–c4).
2. FR-34. A lineage rail on every card ("Story #796 › PR #802 › proof ✓ › ship › v0.4.0"), the gap and its fix when a
   PR names no Story, and a Lineage tree (`L` in the terminal) ([#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963) c1–c4).
3. FR-35. Issues view with type chips and a peek; a Releases timeline from tags and milestones; a release filter on
   every view, with `[` and `]`; a Lanes view by the Workstream field ([#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964) c1–c4).
4. FR-36. One release in Tree, Detail, Board, List and Gantt views, with slices per work-item type, filters in the
   URL, scope drift, and history of past releases ([#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983) c1–c10).

### Themes, command bar, empty states, access

1. FR-37. Aruna Light and Aruna Dark by default, 20 theme families, the choice saved in `[ui] theme` for both faces,
   the terminal's own colours with no choice, `aayu theme import`, a Settings view, and `saffron` read as `aruna`
   ([#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c1–c7).
2. FR-38. Kind and seat icons: line SVG on the web, one-cell symbols in the terminal, words always shown
   ([#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c9, c10).
3. FR-39. A command bar (`:` in the terminal, ⌘K on the web) with a fixed grammar; writes show as preview cards and
   run only on a card's Enter; reads run at once; GitHub text never reaches the parser
   ([#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966) c1–c5).
4. FR-40. A designed empty state on every view: a fact, its time, and one next step
   ([#967](https://github.com/rahulnakmol/agentic-ayudha/issues/967) c1–c4).
5. FR-41. Every state is reachable by keyboard and announced to a screen reader; single-character keys can be turned
   off and never fire in a text field ([#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) c2–c7).

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | Listen on 127.0.0.1 only. Refuse a wrong Host, a cross-origin POST, a reused launch token, and a POST with no CSRF header; each writes nothing | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c1, c9 |
| Security | Every page: `default-src 'none'`, nonce scripts, `require-trusted-types-for 'script'`, `frame-ancestors 'none'`, `no-store`; no script from another origin | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c10 |
| Security | A new threat row T22 "local web UI" in `docs/threat-model.md`, in the same PR. Today the table ends at T21 | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) Assumptions; `docs/threat-model.md` |
| Security | GitHub text (PR bodies, comments) renders as escaped text; no script runs | [#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962) c3 |
| Security | Writes need the device-flow presence check; Take over writes nothing and needs none | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c4; [#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960) Assumptions |
| Performance | A GitHub change shows within 60 s | [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) c1 |
| Cost | With no change on GitHub for 60 minutes, every background read is an ETag 304 | [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) c4 |
| Reliability | The stream reconnects with Last-Event-ID and replays missed events | [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) c3 |
| Reliability | Writes are idempotent: an idempotency key and the card hash on each POST | [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) c3 |
| Performance | A 300-item List renders only the rows in view | [#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983) c7 |
| Accessibility | WCAG 2.2 AA; axe-core 4.14 with 0 serious and 0 critical findings; keyboard-only; reduced motion; 200% text; 44×44 px targets | `CONSTRAINTS.md` `verify.floors.a11y.*`; [#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) c1–c7 |
| Accessibility | Every colour-role pair of every theme variant at 4.5:1 for text | [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c2 |
| Operability | Config keys with defaults: `[inbox.web] session_idle_minutes = 60`, `session_max_hours = 24`; `[ui] theme`; `[ui] icons` | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c8; [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c3, c9 |
| Operability | The server stops when aayu quits | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c1 |
| Supply chain | htmx and Alpine.js CSP builds pinned by SHA-256; no remote assets | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) Assumptions |

Inferred, not stated: the screen-reader floor (`verify.floors.screen_reader.layers = ["L01"]` in `CONSTRAINTS.md`)
runs only for footprints with L01. The planned web code is under `internal/cli/` (L02 in `LAYERS.md`), so that floor
would not run on these Stories unless the footprint or the floor changes.

## Test environment the Stories name

`aayu inbox --web` against the seeded `internal/github/fake`, in agent-browser at 1280x800 and 390x844, with
axe-core 4.14 and the WCAG 2.2 AA tags; seeded data only (Environment section of #957 to #968 and #983).
