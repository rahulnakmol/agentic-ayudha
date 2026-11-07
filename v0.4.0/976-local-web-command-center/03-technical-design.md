# Technical design: Feature #976

## Status

Planned, not built. There is no tech baseline and no `specs/976/design.json` on the ledger (`git ls-tree -r
--name-only origin/ledger specs/976` is empty). The design below comes from the Feature and Story bodies. The parts
that exist on dev today, and that the web face would reuse, are marked "exists on dev".

## Architecture (planned)

| Part | Planned design | Source |
|---|---|---|
| Server | A local HTTP server started by `aayu inbox --web`; listens on 127.0.0.1 only, port 7431 in the examples; stops when aayu quits | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c1, c2 |
| Rendering | Server-rendered `html/template` with htmx and Alpine.js CSP builds, pinned by SHA-256; no remote assets | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) Assumptions ("UX v27, web architecture") |
| Reach | This computer, an `ssh -L` tunnel, or an Amp portal (when `AMP_ORB` is set). The phone on a tailnet is wave 4 ([#970](https://github.com/rahulnakmol/agentic-ayudha/issues/970)) | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c1–c3; [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope |
| Auth | A one-time launch token in the URL fragment (`#t=<code>`), then a session cookie and a CSRF header; writes armed by the GitHub device flow; idle 60 min, max 24 h | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c2, c4–c9 |
| Writes | `POST /items/{key}/act` with the card hash and an idempotency key; stages stream to the card over SSE | [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) c3 and Provenance (ux27/TP15) |
| Live data | Background GitHub reads with ETags (304 when nothing changed) plus a session event stream with Last-Event-ID replay | [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) c3, c4 |
| One word list | One set of strings for both faces in `internal/cli/`, and a parity test that renders both faces for every state | [#958](https://github.com/rahulnakmol/agentic-ayudha/issues/958) c1; [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) Assumptions |
| State | The JSON cache stays, with writes safe for two sessions; no SQLite | [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope |
| Motion | CSS only; no GSAP | [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope; [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) Assumptions |
| Themes | One generated theme file for Aruna and 20 families; `TestPalettesAreReadable` over every variant | [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) Assumptions |
| Command bar | A fixed grammar parser; writes become preview cards; GitHub text never reaches the parser | [#966](https://github.com/rahulnakmol/agentic-ayudha/issues/966) c2, c4 |

## Layers and paths

The Feature's expected paths are `internal/cli/inboxweb/`, `internal/cli/inbox.go`, `internal/cli/presence.go`,
`internal/config/defaults.toml`, `docs/threat-model.md`, and `docs/guide/` ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976)
Expected paths). The web server, templates, assets, themes, and pairing go in `internal/cli/inboxweb/`, "so no wave
adds a top-level package that LAYERS.md lacks" ([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) Assumptions).

| Path | Layer (`LAYERS.md`) | Owner seat | Notes |
|---|---|---|---|
| `internal/cli/inboxweb/` (new), `internal/cli/inbox*.go` | L02 Interaction / API boundary | Code Architect | Most of the work |
| `internal/cli/presence.go` | L02 | Code Architect | In `[autonomy] always_person` (`.aayudha/config.toml`) |
| `internal/config/defaults.toml`, `internal/config/config.go`, `internal/config/gen` | L07 Operational data / state | Production Engineer | `internal/config/` is in `always_person`; `defaults.toml` is in `.github/CODEOWNERS` |
| `internal/ui/ui.go` | L03 Application / workflow | Engineering Manager | Themes ([#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c8) |
| `docs/threat-model.md`, `docs/guide/` | no layer glob matches | — | T22 row ([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957)) |
| `go.mod`, `THIRD_PARTY_NOTICES.md` | L09 (`go.mod`) | DevOps & DevEx Tinkerer | Only if a Go module is added; htmx and Alpine.js are vendored assets, so a licence row is needed (`AGENTS.md`, Safety). Inferred |

No path is in L01 (`packages/design-system/**`). The Approve to build card needs a UX baseline only when the footprint
has L01 (`approveCard` in `internal/cli/inboxapprove.go`). The Feature body still asks for a UX design baseline and
Hanu's review (D4) before Ready ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope). That is a
process step the code does not force for an L02 footprint.

## What exists on dev that the web face would reuse

| Exists on dev | Where | Use in the plan |
|---|---|---|
| The terminal inbox (Bubble Tea) and `--plain` line mode | `internal/cli/inboxtui.go` (the key switch with `j`, `k`, `x`, `X`, `s`, `C`, `/`, `ctrl+k`, `d`, `o`, `tab`, `R`) and `internal/cli/inbox.go` (`plainInbox`) | The terminal face the web must match |
| The card model with counsel | `internal/cli/inbox.go` (`InboxItem`, fields `Counsel`, `CounselFrom`, `Evidence`) | The data both faces render |
| The JSON inbox cache | `internal/cli/inbox.go` (`readInboxCache`, `recordInboxDone`, `inboxCacheMu`) | "The JSON cache stays" |
| ETag cache on GitHub GETs | `internal/cli/inbox.go` comment near line 462 ("Every GET also goes through the client's ETag cache") | The 304 reads of [#959](https://github.com/rahulnakmol/agentic-ayudha/issues/959) |
| Device-flow presence check | `internal/cli/presence.go` (`presenceCheck`, `requireApproval`) | Sign-in of [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) |
| Approve to build card | `internal/cli/inboxapprove.go` (`approveCard`, `approveItem`), [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b) (PR [#1225](https://github.com/rahulnakmol/agentic-ayudha/issues/1225)) | A card the web must show |
| 12 terminal themes | `internal/ui/ui.go` (`Themes`: auto, saffron, contrast, mono, nord, catppuccin-mocha, catppuccin-latte, gruvbox-dark, gruvbox-light, dracula, tokyonight, solarized); `TestPalettesAreReadable` in `internal/ui/ui_test.go` | [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) renames saffron to Aruna and grows to 20 families |
| A per-repository tool | `[hosts] repos` as `owner/name=tool` (`internal/config/config.go`, `Hosts.Repos`) | [#961](https://github.com/rahulnakmol/agentic-ayudha/issues/961) c4 names `[repos."<owner>/<repo>"] tool`, a key that does not exist; the build must use or migrate `[hosts] repos` |
| The GitHub fake | `internal/github/fake/fake.go` | The test environment of every Story |

## Planned config keys (none exist on dev)

| Key | Default | Story |
|---|---|---|
| `[inbox.web] session_idle_minutes` | 60 | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c8 |
| `[inbox.web] session_max_hours` | 24 | [#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957) c8 |
| `[ui] theme` values `aruna`, `aruna-light`, `aruna-dark` and 20 families; `saffron*` read as `aruna*` | auto | [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c3, c7 |
| `[ui] icons = "emoji"` | not stated | [#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c9 |

`defaults.toml` has no `[inbox]` or `[inbox.web]` section today. A new key in a rendered section needs a row in
`internal/config/gen` (`AGENTS.md`, Generated files).

## Planned new commands

- `aayu inbox --web` ([#957](https://github.com/rahulnakmol/agentic-ayudha/issues/957)). Today the `inbox` verb has
  only `--list` (`internal/verbs/registry.go`).
- `aayu theme import` ([#965](https://github.com/rahulnakmol/agentic-ayudha/issues/965) c5). No `theme` verb exists
  in `internal/verbs/registry.go`. A new verb changes the registry and every generated file from it.

## Dependencies on other Features (none delivered)

| Needs | From | State (GitHub, 2026-11-07) |
|---|---|---|
| Wave 2 cards (day areas, asks, missions, decisions) | [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975) | open, planned; 0 of 20 sub-issues merged (`_data/subs.tsv`) |
| The lineage graph (`lineage.Build`, `Of(n)`) | [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) | open, planned, milestone v0.4.1; no `internal/lineage` package on dev |
| The Workstream field and p50 estimates | [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) | open, planned |
| Missions, the working clock | [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) | open, planned; of 65 sub-issues, 38 are `status:merged` and 2 more are closed (`_data/subs.tsv`) |
| Card fixes | Bugs [#924](https://github.com/rahulnakmol/agentic-ayudha/issues/924), [#925](https://github.com/rahulnakmol/agentic-ayudha/issues/925) | open, planned |
| Mission view and controls | [#943](https://github.com/rahulnakmol/agentic-ayudha/issues/943), [#942](https://github.com/rahulnakmol/agentic-ayudha/issues/942) | open, planned |
| Host and session records | [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) | open, planned |
| Benchmark lines, delivery plan | [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872), [#956](https://github.com/rahulnakmol/agentic-ayudha/issues/956) | open, planned |
| Pay mode for Spend | [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) | open, planned, milestone v0.4.1 |

## Key decisions

- [#974](https://github.com/rahulnakmol/agentic-ayudha/issues/974) (closed, done): a mission locks its issues, runs at
  most 48 hours; "no input changes a running mission" (L5), so the Mission page has a command bar, not a chat
  ([#960](https://github.com/rahulnakmol/agentic-ayudha/issues/960) Assumptions).
- [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903) (closed, done): Approve to build agrees the whole
  Feature once; a Story added after it needs the founder's new acknowledgment.
- Q1 to Q8, K4, R1, R7, R9, V1, V8, X1–X3, G1–G2, M2–M3, P2 are design answers the Story bodies cite from "UX
  proposals v27", "interaction specs v4", and the "web43", "web44" and "kanha-missions" notes. These documents are not
  in this repository or on the ledger. Their content here is only what the issue bodies quote; unverified.
