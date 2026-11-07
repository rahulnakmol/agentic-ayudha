# UX design: Feature #895

## Source note

There is no UX design baseline (`origin/ledger` has no `specs/895/`). The Stories cite "UX proposals v27" and
"interaction specs v4" (touchpoints TP1 to TP60). Those documents are not in this repository or on the ledger, so
the planned words below are only those that the issue bodies quote. "Today" parts come from the code at origin/dev
[bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) (884d81b plus #1238; line numbers checked
2026-11-07).

## Who uses it

- The person: the founder, or the `[autonomy] approver`, at a terminal. The inbox is "Refused for agents and needs a
  terminal, except --list" (`aayu inbox --help`).
- The agent: it may run only `aayu inbox --list` (read-only, Markdown or JSON). `inboxHow` tells it how a person acts:
  "A person acts on these, never an agent." (`internal/cli/inbox.go:1836`).

## Today on dev

### Command

```text
$ aayu inbox --help
Every human-only step waiting for you, one card at a time with the twin's counsel; Enter takes the recommendation
and C clears the housekeeping. Refused for agents and needs a terminal, except --list.

Usage:
  aayu inbox [flags]

Flags:
  -h, --help   help for inbox
      --list   Read-only: print what waits as the same cards, with the counsel and how a person acts on them, as
               Markdown (or JSON with --json); writes nothing, needs no terminal, and an agent may run it.
```

(Output of a build of origin/dev; lines wrapped here for width.)

### Keys today

From `helpView` (`internal/cli/inboxtui.go:2580`): Enter does the selected choice (a one-way door asks again); 1-9
select a choice; s skips for now; Tab and Shift-Tab move between needs-you items; j and k move; / or Ctrl+K find any
item or choice; x selects housekeeping, "never a needs-you item"; X selects all housekeeping; C clears all housekeeping
after one confirmation; Esc clears or goes back; o opens on GitHub; d shows what GitHub said; R refreshes; q quits.
The line-by-line face (`--plain`, `TERM=dumb`, a screen reader) has its own key line, `inboxPlainKeys`
(`internal/cli/inbox.go:1401`).

### Cards today

The card kinds and their words are in `inboxKinds` (`internal/cli/inbox.go:94`). The wave 1 kinds are "Ready to
ship" (🚢), "Not ready yet" (⏳), "Needs your review" (🔍), "At the review cap" (✋), "Merged unreviewed, to audit"
(🧾), and the housekeeping kinds. Results show in the status line at the bottom (`m.status`, for example
`inboxtui.go:1069`: "✓ " plus what was done). An empty list says "Nothing waits for you." (`inboxtui.go:1844`).

### Problems the Stories record against today's UX

- With a cache, the inbox sets `loading` to false as soon as it draws the cached items (`inboxtui.go:185` to `:191`),
  and an empty list shows "Nothing waits for you." (`inboxtui.go:1843`). So a cache with no items can show the empty
  state before the first read ends. This matches Bug #925's report (inferred from the code; not run).
- Results and refusals are on the bottom line, away from the card (#924, Actual behaviour; `m.status` in
  `inboxtui.go`).
- o runs a fixed opener and does nothing useful over SSH (`openURL`, `internal/cli/inbox.go:2223`; #929 Summary).

## Planned experience (planned, not built)

### Card anatomy (#930)

A Ship card shows, top to bottom (#930 C1):

```text
SHIP · #802 …
Why you: a Story PR waits for its ship decision.
checks 8/8 · inspect merge · floors hold · +312 −41
Drona · counsel, not approval
  …
1 Approve and ship: approve #802, then squash-merge it into dev after a re-read.
```

The choice line is from #926 C4. At 100x24 the brief shrinks first and the counsel and choices stay (#930 C2). d
opens the brief rows: what, size, pushed, checks, review, last comment, each with value and time (#930 C3). A card
with no recommendation from its source says "No recommendation: the Release issue gives none." (#930 C6).

### Outcomes on the card (#924)

| State | Words on the card | Source |
|---|---|---|
| Changed | "#802 changed on GitHub after you read it. Nothing was written." with "head  4e1c2d9 → 7b0e113  (1 new commit by Nakul, 11:40)", "checks  8/8 → 6/8, 2 running", and "What now: read the new commit, then choose again. Choices are back when the checks finish." | #924 C4 |
| Stages | "Re-reading #802…", "Checks 8/8 at 4e1c2d9", "Approving…", "Merging into dev…", "Waiting for GitHub…" | #924 C8 |
| Done | "✓ Shipped #802 into dev as 9f3a1be · by @rahulnakmol · 11:43" and "o opens the commit · n next card" | #924 C9 |
| Failed | "Shipping #802 failed.", "Written: approval by @rahulnakmol.", "Not written: the merge.", "GitHub said: …", "Next: Enter retries · o opens the PR · s skips until it changes" | #924 C6 |

The amber block is for a change and the red block for a failure (#924 C2, C6).

### Reads and the header (#925, #932)

- Loading: "From cache · 3 min old · reading GitHub…", or "Reading GitHub for the first time… 12 of 31 items" with a
  bar; a cached card's choices say "reading…" (#925 C1, C3, C4).
- After R: "Refreshed 11:42 · 1 new · 1 changed"; later "● Live · refreshed 2 min ago" (#925 C5, C6).
- Slow GitHub banner: "GitHub is not answering. You see data from 11:42 (6 min ago)." and "Reads retry every 30 s.
  Writes wait until GitHub answers." (#925 C7). Rate limit: "GitHub rate limit reached. Reads and writes wait until
  12:05 (in 9 min)." (#925 C8).
- Header at 160x40: "आयु aayu inbox · rahulnakmol/agentic-ayudha · Initiative #`<n>` · 4 to do · 3 done today · writes
  armed 3 h 59 m"; repository line "rahulnakmol/agentic-ayudha · public"; footer "⌘K command · ? keys" (#932 C1, C3,
  C4).

### Ship flows

- Needs your review, for a stalled review or a used review budget: "#676 used its review budget: $14.07 of $15
  (actions log, `<age>`). No more paid reviews run on it." (#926 C6).
- Bypass card: Ship asks "Why bypass CODEOWNERS? One line. It is posted as the “Solo-mode bypass:” comment.", then a
  second Enter (#927 C3).
- Batch: x on Ship cards shows "3 selected · B ships them"; B shows "Ship 3 PRs? Each one is re-read first." and
  "Starting in 5 s · Esc cancels"; results "✓ #802 shipped 9f3a1be", "⚠ #805 changed, nothing written" (#928 C1 to
  C3). A fold line: "3 green PRs wait only for you" with "1 Ship all 3 after one presence check · 2 Open them one by
  one" (#928 C8).
- Code owner card: approve posts a real GitHub review; an author-only owner sees that GitHub never counts the author's
  own approval (#935 C3, C5).
- Findings and waivers: "Findings at 7b0e113: 2 open"; "Dismiss F1? Write why, in one line. It is posted as you on #802
  and recorded on the ledger."; for a P1, "F1 is P1. A dismissal is a one-way door here. Write why, then press Enter
  again."; the WAIVER card "WAIVER · Decision #1210 · Yama asks to waive a floor" (#936 C1 to C5).

### Links, signals, leaving, keys

- c: "Copied the link to #802."; o over SSH: "Copied the link to #802. This is an SSH session, so no browser opened
  here."; o locally: "Opened #802 in your browser." (#929 C2 to C4).
- Unfocused signal: title "aayu inbox (2)"; herdr pane blocked with "2 to do" (#931 C1). Brief: "Since 09:12: 2 shipped
  · 1 needs you · 1 changed." (#931 C3).
- q during a write: "1 write is still running (#802). aayu finishes it, then quits."; then "This session: 2 shipped · 1
  decided · 1 skipped."; `aayu inbox --today` lines such as "11:43 shipped #802 · TUI · 9f3a1be" (#933 C1 to C3).
- ? overlay: "j ↓ Next card · Enter Open the card · 1–9 Take a choice · x Space Select · s Not now · D Deck on or off ·
  ctrl+k Palette · R Read GitHub again · ? Keys", then "Go to: g i · g b · g m · g s · g t · g l · g r · g h" (#934
  C1). Hidden choice: "Only @rahulnakmol can ship this: it touches a protected path." (#934 C2). Another session:
  "@alice is shipping this now." (#934 C3).

### Command output wrap (#980)

At 80x24, "aayu: Approve to build (aayu ready on a Feature) is a person's decision and is refused for an agent lane
(AMP_THREAD_ID)" breaks only at spaces, and each continuation starts at column 7 (#980 C1). At 120 columns, prose
breaks before column 100 (#980 C2).

## Accessibility

- `--plain` gives words only, with no colour, motion, emoji, or box drawing (global flag help).
- #980 C4 keeps the wrap under `--plain` and `NO_COLOR`; `--json` and pipes never wrap.
- #934 C5 asks for `aria-keyshortcuts` on each web action; the web face itself is wave 3 (#976).
- The a11y floor needs every Completion Goal state reachable by keyboard alone (`CONSTRAINTS.md`,
  `verify.floors.a11y.keyboard_only`).

## Open design points

- #934 C1 lists `s` as "Not now" and `D` for the deck, while the deck is wave 4 (#977). Which keys ship in wave 1 is
  not stated (unverified).
- #932 C4 names "⌘K command", while the terminal binds ctrl+k today (`inboxtui.go:1402`). The body does not say how a
  terminal draws ⌘K (unverified).
