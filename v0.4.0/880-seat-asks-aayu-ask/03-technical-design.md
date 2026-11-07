# Technical design: aayu ask

Status: planned, not built. No tech baseline exists. The planned design is the #880 body.

## Layers touched

| Layer | Owner seat | Paths (from #880 Expected paths) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/ask.go` (new), `internal/cli/approve.go`, `internal/cli/build.go`, `internal/cli/buildnext.go` |
| L03 Application / workflow | Engineering Manager | `internal/verbs/registry.go` |
| L07 Operational data / state | Production Engineer | `internal/config/defaults.toml` |
| L09 Platform / operations | DevOps & DevEx Tinkerer | `.github/workflows/aayudha-approve.yml` |
| none today | — | `internal/needs/**` (created by #873) |
| not in a layer glob | — | `docs/threat-model.md` |

## What exists on dev (884d81b)

| Part | Where | Today | Since |
|---|---|---|---|
| `aayu ask` | none | `unknown command "ask" for "aayu"`. No `ask` verb in `internal/verbs/registry.go`; no `skills/ask/`. | — |
| `/aayu answer` | none | `aayudha-approve.yml` runs only for `/aayu approve` or `/aayu skip` (`.github/workflows/aayudha-approve.yml:34`). | — |
| Approval batches | `internal/cli/approve.go:33-41` comment, `batchItem` | A batch comment that a recorded approver answers with `/aayu approve all <hash>`, `/aayu approve 1 3 <hash>`, or `/aayu skip 2 <hash>`. `batchItem.Kind` is `decision` only (comment at `batchItem`). | before v0.3.0 (Decision #488, Story #481) |
| Answer reader | `internal/cli/delegate.go:481`; `delegate.Upstream.Answers`, `internal/delegate/brief.go:41-45`, rendered at `:94-99` | The brief of a story quotes `specs/<n>/answers.md` of each merged upstream story, as untrusted data. No code writes the file. | [53186c9](https://github.com/rahulnakmol/agentic-ayudha/commit/53186c97) (#1094, Story #1006) |
| Kanha hook | `internal/cli/kanha.go:198` `Env.answeredAsk` | Tells the Kanha `answered: #<n> resumes`; the story keeps its slot ("#882 Q2"). No caller outside tests (`internal/cli/kanhaevents_test.go:175`): "until #880 is built, nothing does." | [7a9f3d7](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e) (#1097, Story #1008) |
| Mission clock | `internal/cli/missiontime.go:454-459` comment | "an open ask waits on #880" among a mission's waits on a person. | [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) (#1091, Story #1015) |
| Mission asks (a different thing) | `internal/cli/missionask.go` `missionAsk`; inbox kind `mission` | A mission's own ask on its Feature, as one comment and one inbox card: a gate that failed `max_no_go` times, or a fix PR over budget. It is not `aayu ask` and has no answer record. | [3ece84e](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7) (#1090, Story #1001) |
| Block today | `internal/cli/build.go:1193` `blockBuild`, `:1208` `markBlocked` | `status:blocked`, a page with key `build:<repo>#<n>`, and a Kanha line. `aayu ready <n>` unblocks with a fresh budget (`:337-341`). | before v0.3.0 |

## Planned design (from #880)

1. Ledger records: `asks/<item>/<id>.json` and `asks/<item>/<id>-answer.json` (#880 Scope). Planned, not built.
2. `aayu ask` in `internal/cli/ask.go` with a registry entry (#880 Expected paths). Inferred: a registry entry
   regenerates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, `skills/*/SKILL.md` frontmatter, and
   `internal/mcp/schemas/*.json` (AGENTS.md, "Generated files"). Its seat and model lane are not named.
3. `--answer`: human-only (#880 Scope).
4. `/aayu answer`: a reply that `aayudha-approve.yml` verifies with the checks of `/aayu approve` (#880 Scope;
   Expected paths `internal/cli/approve.go`, `.github/workflows/aayudha-approve.yml`).
5. The earliest verified answer wins (#880 Scope). Inferred: "verified" means it passed presence or the reply checks.
6. Need kinds `ask` and `ask-provisional` in `internal/needs` (#880 Scope; needs #873).
7. Clarifying answers go to `specs/<n>/answers.md` (#880 Scope), which the brief already reads.
8. Config: `internal/config/defaults.toml` is listed; no key is named. Unverified.

## Decisions

- [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862) (accepted 2026-10-23): asks follow the Decision
  SLA, with an Overdue area; a two-way question takes the seat's recommendation provisionally at 48 h; a one-way
  question never applies on its own. The `ask-provisional` kind (#880 Scope) matches this; inferred.
- [#867](https://github.com/rahulnakmol/agentic-ayudha/issues/867) (accepted 2026-10-23): a builder that doubts a
  behaviour runs `aayu ask`.
