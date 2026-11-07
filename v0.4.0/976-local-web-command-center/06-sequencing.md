# Sequencing: Feature #976

## Summary

All 13 Stories are pending. The depends-on links make a narrow start: every Story except #957 needs
[#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958), and #958 needs
[#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957). So waves 1 and 2 have one lane each, wave 3 has
up to 8, and the tail has 3. The issue names the same longest chain: "#957 → #958 → #961 → #968"
([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope). Ten lanes never make sense here: the most
Stories that can run at once is 8.

## Wave 0: before any build (people and upstream work)

| Step | Who | Source |
|---|---|---|
| Upstream Features merge: [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895), [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975), [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885), [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886), [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889); Bugs [#924](https://github.com/rahulnakmol/agentic-ayudha/issues/924), [#925](https://github.com/rahulnakmol/agentic-ayudha/issues/925) | agents | [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Links; [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) Links |
| Decide the milestone of #885 (v0.4.1) against #976 (v0.4.0) | founder | GitHub milestones, read 2026-11-07 |
| Approve the intent, spec, and UX design baselines (device flow) | founder | [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope |
| Hanu's review of the web and terminal prototype states (D4) | Hanu, the UX Researcher & Evolutionist twin (`AGENTS.md`, Seats); a paired human if the seat has one | [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope |
| Approve to build ([#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877)), which writes `specs/976/approved.json` | founder | `internal/cli/inboxapprove.go` (`approveCard`) |

## Wave 1: the server and the session (1 lane)

| Lane | Story | Files and packages |
|---|---|---|
| 1 | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) | new `internal/cli/inboxweb/` (server, auth, headers); `internal/cli/inbox.go` (`--web`); `internal/cli/presence.go`; `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen` (`[inbox.web]`); `internal/verbs/registry.go` (the `web` flag) and its generated files; `docs/threat-model.md` (T22) |

One lane only: every other Story depends on #958, which depends on #957.

## Wave 2: one word list for both faces (1 lane)

| Lane | Story | Files and packages |
|---|---|---|
| 1 | [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) | a new shared word-list file in `internal/cli/`; `internal/cli/inbox*.go` (terminal reads the list); `internal/cli/inboxweb/` (card, write endpoint, notifications, batch) |

Recommendation (inferred, not in a source): #958 should leave one file per view in `internal/cli/inboxweb/` and a
route and nav table that each later lane adds one row to. Without that, every wave 3 lane edits the same router and
layout template.

## Wave 3: the views (up to 8 lanes)

| Lane | Story | Files and packages | Note |
|---|---|---|---|
| 1 | [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) frame, Team, now bar, sheets | `inboxweb/` layout, nav, sheet; `internal/cli/inboxtui.go` (`t`, panes) | Merge first: other views sit in its frame and sheet |
| 2 | [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) live | `inboxweb/` stream; `internal/cli/inboxtui.go` (timer) | |
| 3 | [#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960) Mission page | `inboxweb/` mission files; terminal mission view (`T`) | Needs [#942](https://github.com/rahulnakmol/agentic-ayudha/issues/942), [#943](https://github.com/rahulnakmol/agentic-ayudha/issues/943) merged |
| 4 | [#962](https://github.com/rahulnakmol/agentic-ayudha/issues/962) PR viewer | `inboxweb/` PR files; `internal/cli/inboxtui.go` (`d`) | `d` already toggles details today (`inboxtui.go`) |
| 5 | [#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963) lineage | `inboxweb/` lineage files; `internal/cli/inboxtui.go` (`L`) | Only after [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) merges |
| 6 | [#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964) browse | `inboxweb/` issues, releases, lanes; `internal/cli/inboxtui.go` (space, `[ ]`) | Only after [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) merges |
| 7 | [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) themes | `internal/ui/ui.go`, `internal/ui/ui_test.go`; `internal/config/config.go` (`[ui] theme` enum, `[ui] icons`); `internal/verbs/registry.go` (`aayu theme import`); `inboxweb/` theme and settings | The only lane in this wave that edits config or the registry |
| 8 | [#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966) command bar | new parser file in `internal/cli/`; `inboxweb/` palette; `internal/cli/inboxtui.go` (`:`) | |

Shared hot files in this wave:

- `internal/cli/inboxtui.go`, the key switch: lanes 1, 2, 4, 5, 6 and 8 add keys. Merge in this order: 1, 2, 4, 5,
  6, 8. Each later lane rebases on the one before.
- The shared word list from #958: every lane adds words. Same merge order; lane 3 and lane 7 go after lane 1.
- `internal/cli/inboxweb/` route and nav table: same order, unless #958 made it one row per view.
- The parity test of #958 runs over every lane's states, so a lane that adds words without the terminal face fails it.

## Wave 4: release view and empty states (2 lanes)

| Lane | Story | Files and packages |
|---|---|---|
| 1 | [#983](https://github.com/rahulnakmol/agentic-ayudha/issues/983) release whole | `inboxweb/` release Tree, slices, Board, List, Gantt; needs [#956](https://github.com/rahulnakmol/agentic-ayudha/issues/956) and [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872) merged |
| 2 | [#967](https://github.com/rahulnakmol/agentic-ayudha/issues/967) empty states | the empty templates of each view in `inboxweb/`; the inbox empty lines in `internal/cli/inbox*.go` |

Story #967 must also give #983's views an empty state (c4 names Releases). Merge #983 first, then #967.

## Wave 5: access across every state (1 lane)

| Lane | Story | Files and packages |
|---|---|---|
| 1 | [#968](https://github.com/rahulnakmol/agentic-ayudha/issues/968) keyboard, screen reader, WCAG 2.2 AA | every template in `inboxweb/`; `internal/cli/inboxtui.go` (key safety switch) |

It runs last because c1 runs axe on "every web state of waves 1 to 3 in 5 themes" and c7 walks every view by
keyboard. It touches every template, so it cannot share a wave with another view lane.

## Steps that need a person

- `internal/cli/presence.go` and `internal/config/` are in `[autonomy] always_person` (`.aayudha/config.toml`). So
  #957 (presence, `[inbox.web]`) and #965 (`[ui]` keys) wait for a person at ship.
- `internal/config/defaults.toml` is a protected path (`.github/CODEOWNERS`) and in `always_person_config`. A Story
  that adds a default there (#957) needs the founder's recorded admin bypass (`AGENTS.md`, Protected paths).
- A new verb or flag in `internal/verbs/registry.go` (#957 `--web`, #965 `theme`) regenerates `docs/README.md`,
  `LAYERS.md`, `docs/RACI.md`, `skills/*/SKILL.md` frontmatter, and `internal/mcp/schemas/*.json` (`AGENTS.md`,
  Generated files). `skills/` is a protected path. Run these two Stories in different waves (they are: 1 and 3).
- Vendored htmx and Alpine.js need a licence row in `THIRD_PARTY_NOTICES.md` in the same PR (`AGENTS.md`, Safety).
  A new Go module would also touch `go.mod`, which is in `always_person`.
- A Story added after Approve to build, or a changed Completion Goal, needs the founder's new acknowledgment
  ([#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903); each Story's Assumptions).
