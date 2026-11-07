# Spec: host and session on every agent write

No spec baseline and no Story exist (`specs/871/` is absent on `origin/ledger`; `_data/subs.tsv` has no row). So no
requirement has a Completion Goal yet. Each requirement below is traced to the #871 body and to a proposed slice (P1
to P4) in [05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | One `Env.source()` returns the host, the session ID, the session URL, and `login@machine`. | #871 Scope, In | P1 |
| FR2 | Each actions record can carry an optional `session` field, and the hash chain stays valid with it. | #871 Scope, In | P2 |
| FR3 | An actions record names its host in a normal run, not only with `--host`. | #871 Problem ("`Action.Host` is empty in a normal run") | P1 |
| FR4 | `build/<n>/state.json` carries the source. | #871 Scope, In | P3 |
| FR5 | The delegate state (`delegate/<n>/state.json`) carries the source. | #871 Scope, In | P3 |
| FR6 | Asks, reviews, and drafts carry the source. | #871 Summary | P3 for reviews (inferred); asks: n/a until #880; drafts: partly delivered (see below) |
| FR7 | A host that exports no session gets one through its install or hook (inferred from #871 Expected paths `internal/hosts/hook.go`, `internal/hosts/install.go`, `internal/cli/install.go`). | inferred | P4 |

Partly delivered for FR6: a baseline draft records `source_session` (`ready.Baseline.SourceSession`,
`internal/ready/baseline.go:51`, set at `internal/cli/ready.go:528`), added in
[af81cb4](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48) (#1018). For example,
`specs/887/baselines/intent-v1.json` has `"source_session": "T-01a18d24-…"`.

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | The source is information only and never grants authority. | #871 Scope, Out |
| Reliability | The actions log stays an append-only, hash-chained log; old records without `session` must still verify. | #871 Scope ("keeps the hash chain valid"); `internal/actions/actions.go` comment "append-only, hash-chained JSONL actions log" |
| Privacy | `login@machine` names a person's login and machine name. The evidence does not show a rule on where it may be written. Inferred: the config paths in #871 Expected paths (`internal/config/defaults.toml`, `CONSTRAINTS.md`) may hold such a rule; no key is named. | inferred |
| Operability | `Env.session()` reads `AMP_THREAD_ID`, `CODEX_THREAD_ID`, `CLAUDE_CODE_SESSION_ID`, and `COPILOT_AGENT_SESSION_ID` today; a host with none of them gives an empty session (`internal/cli/asked.go:38-44`). | code |
