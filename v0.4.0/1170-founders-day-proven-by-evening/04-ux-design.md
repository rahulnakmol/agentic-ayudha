# #1170 UX design

The person's surface is the terminal inbox, `aayu inbox`. The agent's surface is the read-only list,
`aayu inbox --list --json`, and the refusals that keep person-only steps away from agent lanes. Command help below is
from a binary built from `dev` `884d81b` (`go build ./cmd/aayudha`, then `<binary> <verb> --help`). The Overnight
card merged after that, in `bc78349`; its words below come from `inboxKinds` in `internal/cli/inbox.go` on `bc78349`. No web or phone
screen is in scope (#1170 Scope, Out: [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976),
[#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977)).

## The person's day (delivered)

1. Morning. The person runs `aayu inbox` in a terminal. The cards come in decision order (`internal/cli/inbox.go`,
   `inboxOrder`): Decision, Baseline, Approve to build, Mission start, then Release scope, Mission, Mission time,
   Evening review, Overnight, and the other kinds.
2. One device code covers the session (`aayu inbox` Explain text of the Baseline and Approve to build kinds: "one code
   covers the session").
3. After the person starts a mission in the session, the last card is "The day is set" with one line per Feature
   (`internal/cli/inboxday.go`, `daySet`):
   `#<f> · today <milestone>: <stories> · evening review <Mon 2 Jan 18:00 zone>, the end of [time] business_hours`.
4. Evening. At the end of `[time] business_hours`, one Evening review card per story agreed that morning shows each
   criterion next to its evidence (`internal/cli/inboxevening.go`, `eveningItems`). The choices are
   `accept #<n>: it does what you agreed` and `say what is wrong, in a comment on #<n>`. A story that did not merge
   shows `not merged: <why>.` and `Next: <step>.`
5. When every criterion has evidence and each story is accepted, the inbox shows
   `The day is proven: <n> stories and <m> criteria, each with its evidence. Job well done.` in the Quality
   Architect's voice (`inboxevening.go`, `dayTally.cheer`).

6. Overnight. Once the evening review is done, an Overnight card proposes tonight's runs. Approving it records them
   in `missions/overnight/<date>.json`; leaving it writes nothing (`inboxKinds[InboxOvernight].Explain`). Outside
   `[time] business_hours`, `aayu mission next` says `outside the day: starts at <time>` (`internal/cli/missionnext.go`).
   No agent carries out a run yet (PR #1238 body, criterion 4; accepted follow-up F1).

## Card kinds this Feature added

Words and lines come from `inboxKinds` in `internal/cli/inbox.go`.

| Kind | Icon and word | Line on the card | Story |
|---|---|---|---|
| `baseline` | 📌 Baseline | "An agent drafted a part of a Feature; it counts as the baseline only when you approve it." | #1172 |
| `approve` | 🚀 Approve to build | "Every baseline of a Feature is approved; it waits for you to approve it to build." | #1173 |
| `evening` | 🌇 Evening review | "A story you agreed this morning, each criterion next to its evidence; only you accept it or say what is wrong." | #1176 |
| `overnight` | 🌙 Overnight | "What runs tonight, each run bounded in hours and cost; nothing runs overnight that you did not set." | #1178 (`bc78349`) |

The Baseline card shows the pins, the checklist, each change since the last version, and the counsel of the owner
seat (`inboxKinds[InboxBaseline].Explain`). A UX part shows only when the Feature's footprint has L01
(`internal/cli/inboxbaseline.go`, line 69).

## Messages and errors (delivered)

| When | Message | Source |
|---|---|---|
| A draft changed after the card was read | `The spec of #889 changed after the draft. An agent drafts it again.` | #1172 criterion 3; `inboxbaseline.go` `baselineChanged` |
| Stories changed while the person read the card | `The stories of #889 changed while you read it.` | #1173 criterion 4; `inboxapprove.go` `approveChanged` |
| A criterion has no evidence | `#1011 criterion 3 has no evidence` and the next step | #1177 criterion 2 |
| A BUGS gate with a merged Bug that has no pack | `#31 merged with no proof pack` | `internal/cli/missiongate.go` `bugPacks` |
| `grill --edit` of a Ready issue | `#<n> is Ready; its body is frozen` | `internal/cli/grill.go` `runGrillEdit` |
| A PR of a later milestone | `ship #<pr>: #<n> is in <ms>: it merges after <earlier>'s gate passes` | `internal/cli/missionstop.go` `missionHold` |

## Commands and flags (delivered)

`aayu inbox`: "Every human-only step waiting for you, one card at a time with the twin's counsel; Enter takes the
recommendation and C clears the housekeeping. Refused for agents and needs a terminal, except --list."

| Flag | Help text |
|---|---|
| `--list` | "Read-only: print what waits as the same cards, with the counsel and how a person acts on them, as Markdown (or JSON with --json); writes nothing, needs no terminal, and an agent may run it." |
| `--plain` (global) | "words only on a terminal: no colour, motion, emoji, or box drawing (also [ui] plain = true, AAYU_PLAIN=1)" |

`aayu grill <type>` (Story #1179):

| Flag | Help text |
|---|---|
| `--edit` | "A filed issue of the type to change before Ready: --body (and --title) are checked as a draft and, when they pass Create, written in one write through the actions log, with the diff shown. Refused for a Ready issue and for one a running mission locks." |
| `--body`, `--title` | the new body (issue-form layout) and title |

`aayu ready <issue>` flags that the morning cards run for the person: `--part` ("a person approves its waiting
draft, through GitHub's device flow every time; refused for an agent lane"), `--request`, `--changes`, `--agents`,
and `--milestones` ("Each story and Bug of the Feature is in exactly one").

`aayu mission <start|show|next|gate|budget|stop|resume|extend|tick|recover|retry|wake>`: the person's controls of the
mission (`--once-more`, `--reason`, `--to`, `--keep`, `--end`, `--story`). The help text of `--end` says: "each open
story and Bug loses its claims and goes back to status:ready, its open PRs close".

## Accessibility

- `--plain` and `[ui] plain = true` keep the same words with no motion (#1177 criterion 3; help text above).
- The icons have Emoji_Presentation and "a word always stands next to one, and --plain drops them" (comment above
  `inboxKinds`, `internal/cli/inbox.go`).
- Evidence images show inline only where the terminal shows images; else the card gives the path and a link
  (#1176 criterion 1; `inboxevening.go` `showsImages`, `evidenceLink`).
- No screen-reader or contrast check of these cards is recorded (the evidence does not show one).

## The agent's surface

- `aayu inbox --list --json` lists Baseline, Approve to build, and Evening cards with their checklists and with no
  approve or accept action (#1172 criterion 4; #1176 criterion 5; `inboxevening.go` `withoutAccept`).
- Each person-only verb refuses an agent lane (`AAYU_AGENT=1`, `--ci`) before any read (#1182 criteria 3, 4).
- `aayu grill --edit` is an agent's way to change a filed body before Ready (#1179).

## Planned, not built

The PRs below were closed unmerged on 2026-11-07 when the mission ended; each branch is kept
([05-implementation-plan.md](05-implementation-plan.md), "Closed unmerged").

| Surface | What the person would see | Source |
|---|---|---|
| Window message (#1254) | Outside the windows nothing is sent; at a window's start one message lists every queued item; the evening cards name "18:30-19:30 …, the evening window" | PR [#1270](https://github.com/rahulnakmol/agentic-ayudha/issues/1270) body, criteria 5 and 7 |
| Extend card (#1248) | `#889 may extend before its 48 h time limit`, with hours used, what runs, what waits, and the command | PR [#1249](https://github.com/rahulnakmol/agentic-ayudha/issues/1249) body, criterion 4 |
| One card per Feature (#1253) | One card approves every part, then Approve to build, then mission start; Open a part shows the part's detail | PR [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265); #1253 criteria 1-4 |
| Draft refusal (#1250) | `#<pr> is a draft: mark it ready after the local gate passes` | PR [#1261](https://github.com/rahulnakmol/agentic-ayudha/issues/1261); #1250 criterion 6 |
| Rebase wait (#1234) | `waits #2: waits on a rebase: PR #7 was proven on a dev without <sha>` in `aayu mission next` | PR [#1236](https://github.com/rahulnakmol/agentic-ayudha/issues/1236) body |
| Audit card for freed paths (#1281) | An audit card for each merge that touched a former narrow-set path, in the next window | #1281 criterion 4; PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) |
