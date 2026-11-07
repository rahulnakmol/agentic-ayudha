# #892 Implementation plan

## Stories and Bugs

No Story or Bug is filed under [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892). GitHub lists no
sub-issues, `_data/subs.tsv` has no row for 892, and no PR in `_data/merge-refs.tsv` or `_data/open-prs.tsv` refers
to it. The table below holds proposed slices only. They are proposed, not filed. Effort and risk are blank because
no source estimates them.

| # | Title (proposed, not filed) | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | `[budget.hosts]` and an API-call cap are config keys, documented in CONSTRAINTS.md | Story | not filed | — | — | — | L07 `internal/config/config.go`, `defaults.toml`, `gen/main.go`; L04 `CONSTRAINTS.md` (generated) | not started |
| P2 | The breaker skips a subscription host's agent-lane cost and keeps the API-call cap | Story | not filed | — | — | P1 | L05 `internal/models/gateway.go`; L02 `internal/cli/build.go`, `delegate.go`, `buildnext.go`, `lease.go`; L03 `internal/delegate/usage.go` | not started |
| P3 | `aayu init` asks per host how you pay, then the daily budget and what it buys | Story | not filed | — | — | P1 | L02 `internal/cli/init.go`; L03 `internal/initrepo/**`; reads `internal/models/prices.go` | not started |
| P4 | Build and status show each host's billing mode and the cap that holds | Story | not filed | — | — | P2 | L02 `internal/cli/status.go`; L03 `internal/status/status.go`; `docs/guide/how-models.md` | not started |

## Delivered

Nothing of #892 is delivered. These earlier merges are what it builds on (see
[03-technical-design.md](03-technical-design.md)):

- `[build] agents = 5` replaced `in_flight`: PR [#1089](https://github.com/rahulnakmol/agentic-ayudha/issues/1089),
  commit [f563bc4](https://github.com/rahulnakmol/agentic-ayudha/commit/f563bc48) (Story #1000 under #889).
- Start records the mission's budget: PR [#1084](https://github.com/rahulnakmol/agentic-ayudha/issues/1084), commit
  [d941d15](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d151) (Story #997).
- The breaker, the `[budget]` keys, and the subscription usage-limit pause exist from before v0.3.0
  (`internal/models/gateway.go`, `internal/delegate/usage.go`). The exact commit is not checked here.

## In review

None. No open PR refers to #892 (`_data/open-prs.tsv`).

## Pending

- The whole Feature: P1 to P4.
- Before any Story: the intent, spec, and tech baselines, and Approve to build, all by a person
  (`aayu ready 892 --part <part>`, then `aayu ready 892`).
- Unresolved in the sources: the name of the API-call cap key (#918 versus #868). No source gives an estimate.
