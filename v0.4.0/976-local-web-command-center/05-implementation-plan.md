# Implementation plan: Feature #976

## All Stories and Bugs

Type, status, effort and risk are from `_data/subs.tsv`. Depends-on is from each issue's Links section. Paths are the
Feature's expected paths ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976)) split by what each
Completion Goal touches; the split per Story is inferred. Evidence: no PR into dev since v0.3.0 refers to any of these
issues (`_data/merge-refs.tsv`), and no open PR does (`_data/open-prs.tsv`).

| # | Title | Type | Status | Effort | Risk | Depends on | Layers / paths (planned) | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) | Open the web inbox from this computer, an SSH tunnel, or an Amp portal, and sign in once per session | Story | planned | L | high | — | L02 `internal/cli/inboxweb/` (server, auth), `internal/cli/inbox.go`, `internal/cli/presence.go`; L07 `internal/config/` (`[inbox.web]`); `docs/threat-model.md` (T22) | not started |
| [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) | Every card and state of the terminal inbox shows on the web with the same words | Story | planned | L | medium | #957, #924, #925 | L02 `internal/cli/` shared word list, `internal/cli/inboxweb/`, `internal/cli/inbox*.go` | not started |
| [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) | Always live: cards change in place without a reload, and the stream reconnects | Story | planned | M | low | #958 | L02 `internal/cli/inboxweb/` (SSE), `internal/cli/inboxtui.go` (timer) | not started |
| [#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960) | The web Mission page: Sessions, the mission's record, and progress in three columns, with a command bar and Take over | Story | planned | L | medium | #943, #942, #958, #889 | L02 `internal/cli/inboxweb/`, the terminal mission view in `internal/cli/` | not started |
| [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) | The command center frame: overview tiles, the Team view, a now bar, a detail sheet for everything, a collapsible nav, any screen size | Story | planned | L | low | #958 | L02 `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` (`t`, panes) | not started |
| [#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962) | Read the PR inside the app, with Approve and ship always in reach | Story | planned | M | medium | #958 | L02 `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` (`d`) | not started |
| [#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963) | Lineage on every card: from Story, Bug, or Feedback to PR, proof, ship, and release | Story | planned | M | low | #958, #885 | L02 `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` (`L`) | not started |
| [#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964) | Browse issues, release by release, and by workstream | Story | planned | L | low | #958, #886 | L02 `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` (space, `[ ]`) | not started |
| [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) | Aruna and 20 themes, light and dark, custom themes, and settings on both faces | Story | planned | L | medium | #958 | L03 `internal/ui/ui.go`; L07 `internal/config/` (`[ui] theme`, `[ui] icons`); L03 `internal/verbs/registry.go` (`aayu theme import`); L02 `internal/cli/inboxweb/` | not started |
| [#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966) | Say what to do: a command bar with a fixed grammar and preview cards | Story | planned | M | medium | #958 | L02 `internal/cli/` parser, `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` (`:`) | not started |
| [#967](https://github.com/rahulnakmol/agentic-ayudha/issues/967) | Every view has a designed empty state with what happens next | Story | planned | S | low | #961, #964, #963 | L02 `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` | not started |
| [#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) | Everything works with a keyboard and a screen reader, at WCAG 2.2 AA | Story | planned | M | medium | #958, #961 | L02 `internal/cli/inboxweb/`, `internal/cli/inboxtui.go` | not started |
| [#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983) | See a release whole: its Tree, a slice per type, and Board, List and Gantt views | Story | planned | L | medium | #964, #961, #956, #886, #872 | L02 `internal/cli/inboxweb/` | not started |

No Bug is filed under #976 (`_data/subs.tsv`).

## Delivered

Nothing. No commit on dev builds any part of the web face: `internal/cli/inboxweb/` does not exist, and the `inbox`
verb has only `--list` (`internal/verbs/registry.go`). The terminal inbox that wave 3 mirrors exists and keeps
changing on dev, for example [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b) (Approve to build
card, PR [#1225](https://github.com/rahulnakmol/agentic-ayudha/issues/1225)) and
[66397d3](https://github.com/rahulnakmol/agentic-ayudha/commit/66397d3) (PR [#1237](https://github.com/rahulnakmol/agentic-ayudha/issues/1237)).
Those commits belong to other Features.

## In review

Nothing. No open PR into dev refers to #976 or its Stories (`_data/open-prs.tsv`).

## Pending

All 13 Stories. Before any can start:

1. The upstream Features must merge: wave 1 [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895) (0 of
   15 sub-issues merged), wave 2 [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975) (0 of 20),
   [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885), [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886),
   and [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) (`_data/subs.tsv`;
   [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Links).
2. A person approves the intent, spec, and UX design baselines, Hanu reviews the prototype states (D4), and the
   founder gives Approve to build ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope).

## Gaps not sliced into Stories

- The design source. The prototypes and specs the Stories cite (UX proposals v27, interaction specs v4, prototypes
  v43 and v44) are not in the repository. A UX baseline must pin them before Ready. No estimate in any source.
- Milestone mismatch. #976 is in v0.4.0, but it depends on [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885)
  (v0.4.1). [#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963) also depends on it, and
  [#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960) reads Spend from
  [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) (v0.4.1). A person must move one or the other.
- The per-repository tool key. [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c4 reads
  `[repos."<owner>/<repo>"] tool`; dev has `[hosts] repos` as `owner/name=tool` (`internal/config/config.go`,
  `Hosts.Repos`). The Story must be corrected or a migration sliced.
- The estimate. #983 (L) came after the "12 stories, about 15.5 h" estimate
  ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope). No source re-estimates it.
- Review guidance and Behaviours for each Story are "kept in the Feature's drafts" until Features
  [#887](https://github.com/rahulnakmol/agentic-ayudha/issues/887) and [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891)
  add the form fields (each Story's Assumptions). I did not find these drafts in the repository or on the ledger;
  unverified.
