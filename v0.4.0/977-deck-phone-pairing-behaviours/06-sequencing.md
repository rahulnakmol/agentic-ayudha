# Sequencing: Feature #977

## Summary

All 5 Stories are pending. "No story depends on another story of this wave"
([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope), so once their upstream work has merged, all
five can run as one wave of 5 lanes. Five is the most that makes sense: there are only five Stories. Their upstream
work is not built, and it decides when each lane can start.

## Wave 0: before any build

| Step | Who | Source |
|---|---|---|
| Wave 3 merges: at least [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957), [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958), [#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964); and [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) for the phone frame | agents | Story Links; [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c8 |
| Wave 2 Story [#937](https://github.com/rahulnakmol/agentic-ayudha/issues/937) (#975) and wave 1 Stories (#895) [#928](https://github.com/rahulnakmol/agentic-ayudha/issues/928), [#930](https://github.com/rahulnakmol/agentic-ayudha/issues/930) | agents | #969, #973 Links |
| [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) and [#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890) merge | agents | #971, #972 Links |
| Decide #890's milestone (v0.4.1) against #977 (v0.4.0) | founder | GitHub milestones |
| Approve the intent, spec, and UX design baselines | founder (device flow) | [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope |
| Hanu's review of the prototype states (D4) | Hanu, the UX Researcher & Evolutionist twin | [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope |
| Approve to build | founder | [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877); `internal/cli/inboxapprove.go` |

## Wave 1: the five Stories (5 lanes)

| Lane | Story | Can start when | Files and packages |
|---|---|---|---|
| 1 | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) pair a phone | #957 merged | `inboxweb/` pairing files; `internal/cli/inboxtui.go` (`Q`); `internal/config/` (`pair_code_minutes`); `go.mod`, `go.sum`; `THIRD_PARTY_NOTICES.md` |
| 2 | [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) deck | #958, #937, #928 merged | `inboxweb/` deck files; `internal/cli/inboxtui.go` (`D`); `internal/config/` (`undo_seconds`, unless #928 adds it) |
| 3 | [#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971) Behaviours view | #958, #891 merged | `inboxweb/` behaviours view only |
| 4 | [#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972) Ask why | #958, #890 merged | `inboxweb/` Done card peek; `internal/cli/inbox*.go` (Done card action) |
| 5 | [#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) Ask Atri | #958, #964, #930 merged | `inboxweb/` counsel panel; `internal/cli/inbox.go`; `internal/cli/inboxtui.go` (`?c`) |

Shared hot files and merge order:

- `internal/cli/inboxtui.go`, the key switch: lanes 1, 2 and 5 add keys (`Q`, `D`, `?c`). Merge 1, then 2, then 5.
- `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen`, and the generated
  `CONSTRAINTS.md` and `internal/config/schema.json`: lanes 1 and 2 add keys. Merge lane 1 first; lane 2 rebases and
  runs `go generate ./...`.
- The shared word list of [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) and its parity test: every
  lane adds words. "The web faces of wave 2 and wave 4 stories join the parity test as they land"
  ([#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) Assumptions). Merge in lane order.
- The `inboxweb/` route and nav table: every lane adds a route or an action. Same order.

Lanes 3 and 4 touch no shared file except the word list and routes, so they can merge at any point.

## Steps that need a person

- `internal/config/` and `go.mod`/`go.sum` are in `[autonomy] always_person` (`.aayudha/config.toml`). Lanes 1 and 2
  wait for a person at ship. `internal/config/defaults.toml` is also a protected path (`.github/CODEOWNERS`): it needs
  the founder's recorded admin bypass (`AGENTS.md`, Protected paths).
- A new dependency needs its licence in `THIRD_PARTY_NOTICES.md` and a passing `security` licence scan (`AGENTS.md`,
  Safety).
- Risk high on #969 and #970 (`_data/subs.tsv`). Their criteria are safety rules (no ship by swipe; no reach beyond
  the tailnet), so a person should read the proof before Approve to build (recommendation).
