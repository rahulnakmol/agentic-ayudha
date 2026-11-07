# Technical design: Feature #977

## Status

Planned, not built. There is no tech baseline and no `specs/977/design.json` on the ledger. Wave 4 adds to the web
face of wave 3 ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976)), which is also not built. The design
below comes from the Feature and Story bodies.

## Architecture (planned)

| Part | Planned design | Source |
|---|---|---|
| Deck | A mode of the web inbox: one card at a time; swipes for two-way answers only; a 5 s Undo before any post; buttons and keys for every swipe; `D` in the terminal | [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) c1–c5, c7 |
| Phone reach | Tailscale Serve in front of the 127.0.0.1 server (`tailscale serve --bg 7431`); aayu never binds the Wi-Fi address | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c4, c7 |
| Pairing | A one-time code in the URL fragment shown as a QR code; one phone, one use, 5 minutes; the founder's tailnet identity checked before the sign-in page; then the phone's own device flow and session | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c1–c3, c5 |
| Behaviours view | A read-only view over #891's behaviour records, by Feature | [#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971) Assumptions |
| Ask why | An action that runs `aayu explain <n> "<question>"` to the Accountable seat, or an ask to a paired human | [#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972) c2, c3 |
| Ask Atri | A read-only panel built from the card's facts, "as on Decisions today (`CounselFrom` facts in `internal/cli/inbox.go`)" | [#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) Assumptions |

How the server learns the tailnet user's identity is not stated (inferred options: Tailscale Serve identity headers,
or the local Tailscale API). Unverified.

## Layers and paths

Expected paths: `internal/cli/inboxweb/`, `internal/cli/inbox.go`, `internal/config/defaults.toml`, `go.mod`,
`THIRD_PARTY_NOTICES.md` ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Expected paths).

| Path | Layer (`LAYERS.md`) | Notes |
|---|---|---|
| `internal/cli/inboxweb/`, `internal/cli/inbox*.go` | L02 Interaction / API boundary | Deck, pairing, views, panels |
| `internal/config/` (`[inbox] undo_seconds`, `[inbox.web] pair_code_minutes`) | L07 Operational data / state | In `[autonomy] always_person`; `defaults.toml` is a protected path |
| `go.mod`, `go.sum` | L09 Platform / operations | In `always_person` |
| `THIRD_PARTY_NOTICES.md` | no layer glob matches | Licence of any new dependency (`AGENTS.md`, Safety) |

## What exists on dev that wave 4 would reuse

| Exists on dev | Where |
|---|---|
| Card counsel and its source | `internal/cli/inbox.go`, `InboxItem` fields `Counsel`, `CounselFrom` ("posted" or "facts"), `Evidence` |
| The `counsel` verb that #973 c2 tells the person to run | `internal/verbs/registry.go`, `Name: "counsel"` |
| The terminal inbox key switch where `D`, `Q` and `?c` would go | `internal/cli/inboxtui.go`; today `?` is bound (help) and `C`/`A` clear housekeeping |
| Behaviour checks in the mission gate (not the view #971 needs) | `internal/cli/missiongate.go` reads `reports/behaviour.json` |

Not on dev:

- No web server (`internal/cli/inboxweb/` does not exist).
- No `explain` verb in `internal/verbs/registry.go`. #972 depends on it ([#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890)).
- No defense record: no non-test Go file mentions "whiteboard defense" or "defense record".
- No Tailscale or QR code code, and no such module in `go.mod`.
- No `[inbox]` or `[inbox.web]` section in `internal/config/defaults.toml`.

## Planned config keys (none exist on dev)

| Key | Default | Story |
|---|---|---|
| `[inbox] undo_seconds` | 5 | [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) c7 (the key may come from [#928](https://github.com/rahulnakmol/agentic-ayudha/issues/928), "every inbox time is a config key"; inferred from its title) |
| `[inbox.web] pair_code_minutes` | 5 | [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) c6 |

## Dependencies on other work (none delivered)

| Story | Needs | State (GitHub, 2026-11-07) |
|---|---|---|
| [#969](https://github.com/rahulnakmol/agentic-ayudha/issues/969) | [#937](https://github.com/rahulnakmol/agentic-ayudha/issues/937) (day areas), [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) (web cards), [#928](https://github.com/rahulnakmol/agentic-ayudha/issues/928) (batch ship and inbox times) | all open, planned |
| [#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970) | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) (web server and session) | open, planned |
| [#971](https://github.com/rahulnakmol/agentic-ayudha/issues/971) | [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) (behaviours), #958 | open, planned |
| [#972](https://github.com/rahulnakmol/agentic-ayudha/issues/972) | [#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890) (defense, `aayu explain`), #958 | open, planned; #890 is in milestone v0.4.1 |
| [#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973) | [#930](https://github.com/rahulnakmol/agentic-ayudha/issues/930) (card counsel above choices), [#964](https://github.com/rahulnakmol/agentic-ayudha/issues/964) (Browse), #958 | open, planned |

## Key decisions

- Q6 is "A + B + C": no Wi-Fi mode, and the open internet is never offered ([#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970)
  Assumptions). Q2: a model lane for counsel stays later ([#973](https://github.com/rahulnakmol/agentic-ayudha/issues/973)).
  Q-B places Ask Atri in wave 4. These answers come from "UX proposals v27" and "interaction specs v4", which are not
  in this repository or on the ledger; unverified beyond the issue text.
- [#974](https://github.com/rahulnakmol/agentic-ayudha/issues/974) (closed, done) gives the mission time the estimate
  uses.
- [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903) (closed, done): a Story added after Approve to build
  needs the founder's new acknowledgment.
