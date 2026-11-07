# Technical design: Feature #875

## Tech baseline

`specs/875/baselines/tech-v1.json` on `ledger`: owner `code-architect`, approved by `rahulnakmol` at
2026-11-02T17:18:58Z, hash `ded88143beb8…`. It pins `specs/875/design.json` (SHA-256 `883340bcafec…`). The design
(planned 2026-10-31T12:06:09Z) has footprint and layers L02, L03, L07, story order #919, #920, #921, #922, #923, and
these contracts (quoted):

1. "the reviewer table (which seat reviews which baseline part, Hanu only with L01) and one finding type are in
   internal/verbs/registry.go; every review timing is a [critique] key that a person owns (#919, #1032)"
2. "aayu critique &lt;f&gt; --part &lt;p&gt; --seat &lt;s&gt; --review &lt;file&gt;: records one review per seat
   per baseline version on the ledger (specs/&lt;f&gt;/reviews/&lt;part&gt;-v&lt;k&gt;/&lt;seat&gt;.json,
   ready.ReviewPath) and refuses a review that is not independent: same session, same model family as the
   drafter, or seen another seat's review (#920)"
3. "an approved baseline starts one fresh review session per reviewing seat on the approver's agent tool, through
   the existing delegate receivers (#921)"
4. "the slow tick (loops/) starts each missed review, and the review window closes at [critique]
   review_wait_hours; Approve to build then proceeds and names the missing reviews (#922, readyapprove.go)"
5. "a local agent CLI (Codex, Copilot, OpenCode, Cursor without a key) runs a review session, and aayu critique
   records it when the session ends (#923)"
6. "#876 reads the recorded findings through the critique state"

Rollback (quoted): "Reviews are append-only ledger records under each baseline version; a revert stops writing and
starting them, and Approve to build sees no reviews, which is today's behaviour. The slow tick loop is removed with
its file. No record is rewritten."

Two differences between the design and the Stories, to settle at build time:

- The design names the flag `--review <file>`; [#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920)
  CG 1 uses `--draft findings.json`.
- The design's independence rule includes "same model family as the drafter" and "seen another seat's review";
  #920's criteria test the same session, the owner seat, one review per seat, and one seat per session only.

## Layers touched

L02 (`internal/cli/critique.go`, `internal/cli/ready.go`, `internal/mcp/schemas/`), L03 (`internal/verbs/`,
`internal/delegate/`, `loops/`, `skills/critique/`, the finding package), L07 (`internal/config/`, the ledger
records). Layer paths from `LAYERS.md`; the file list from the
[#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Expected paths.

## What exists on `dev` that this Feature builds on

| Piece | Code | From |
|---|---|---|
| Review record path | `ready.ReviewPath(f, part, v, seat)` → `specs/<f>/reviews/<part>-v<k>/<seat>.json` in `internal/ready/drift.go` | [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) |
| Finding ID parser | `ready.Finding(id)` for `<part>-v<k>/<seat>/<n>`; `ready.Review`, `ReviewFinding`, `Review.Has` (IDs only) in `internal/ready/drift.go` | #993 |
| Critique summary that Approve to build reads | `ready.Critique{Reviews, In, AskedAt, Open, Dismissed}` in `internal/ready/approve.go` | [#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995) |
| Hook | `Env.CritiqueState` in `internal/cli/env.go`; `Env.critique` in `internal/cli/readyapprove.go` returns an empty `ready.Critique` when it is nil | #995 |
| Config | `[critique] review_wait_hours = 24` in `internal/config/defaults.toml`; `config.Critique` with `minimum=0` in `internal/config/config.go`; `critique` in `always_person_sections` | #995, PR [#1032](https://github.com/rahulnakmol/agentic-ayudha/issues/1032) |
| RACI data | `internal/verbs/raci.go` | WP5.1 |
| Receivers | `delegate.Receivers = {Codex, Cursor, Claude, Amp, OpenCode}` in `internal/delegate/state.go`; adapters in `internal/delegate/receivers.go` | earlier work |
| Local session environment | `delegate.ScrubEnv` in `internal/delegate/local.go`; `[delegate] local_timeout_minutes = 60` | earlier work |
| Race rule to reuse | the ledger append order of build's loop claims, `internal/cli/buildnext.go` (#922 Assumptions) | earlier work |

## Planned, not built

| Story | Packages and files | Records | Config |
|---|---|---|---|
| [#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919) | reviewer table in `internal/verbs/` (beside `raci.go`) with a lint like `verbs.LintRACI`; the finding type in `internal/workitems/critique` (CG 4 names `go test ./internal/workitems/critique`); generated `docs/RACI.md` | none | `[critique] redispatch_after_minutes`, `attempts`, `sessions_per_day`, `max_findings`; minimum 1 for `review_wait_hours` and `attempts` |
| [#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920) | new verb `critique` in `internal/verbs/registry.go`; `internal/cli/critique.go`; `skills/critique/SKILL.md`; generated MCP schema and skill frontmatter | `specs/<f>/reviews/<p>-v<k>/<seat>.json` | reads `max_findings` |
| [#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) | dispatch after `partRun.approve` in `internal/cli/ready.go`; briefs and adapters in `internal/delegate/` | `specs/<f>/reviews/<p>-v<k>/dispatch.jsonl` | reads `sessions_per_day`, the approver's `[repos."<owner>/<repo>"] tool` |
| [#922](https://github.com/rahulnakmol/agentic-ayudha/issues/922) | `critique --sweep` and `--status` in `internal/cli/critique.go`; `loops/baseline-reviews.toml` and rendered `loops/{amp,cursor,claude,codex}` files | appends to `dispatch.jsonl` | reads `attempts`, `review_wait_hours`, `[time]` business hours |
| [#923](https://github.com/rahulnakmol/agentic-ayudha/issues/923) | `critique --session local` in `internal/cli/critique.go`; `internal/delegate/local.go` | the review record, with `mode local` | reads `[delegate] local_timeout_minutes` |

The finding package path is open. The Feature's Expected paths list both `internal/critique/**` and
`internal/workitems/critique/**`; #919 and #1114 name `internal/workitems/critique`.

## Key decisions and inputs

- Q16 reviewer table and the mentalist row on tech (@rahulnakmol, 2026-10-26, cited in #919 Behaviours).
- R9: one agent tool per setup, read from the approver's user config; no `receiver = "auto"` (#919 Open
  questions; #921 Assumptions).
- Q2: every installed host runs review sessions, started by the approval event, with no poll (#921 Open
  questions).
- E2: one fresh session per seat; any thread may run one review on demand (#920 Open questions; #922 Behaviours).
- No Decision issue is linked from #875 or its Stories. The core design and the Q/R/E IDs are in private research
  notes; I did not read them.
