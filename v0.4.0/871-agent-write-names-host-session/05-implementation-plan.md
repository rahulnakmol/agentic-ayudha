# Implementation plan: host and session on every agent write

## Stories and Bugs

Feature #871 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed. Each comes from one part of
the #871 Scope. No source estimates effort or risk, so those columns say "not estimated".

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | One `Env.source()` names the host, session, URL, and `login@machine`, and every actions record names its host | Story (proposed, not filed) | not filed | not estimated | not estimated | none | L02: `internal/cli/env.go`, `internal/cli/asked.go`, `internal/cli/actions.go` | not started |
| P2 | An actions record can name its session, and the hash chain stays valid | Story (proposed, not filed) | not filed | not estimated | not estimated | none | L07: `internal/actions/**` | not started |
| P3 | The build state and the delegate state name their source | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, P2 | L02, L03: `internal/cli/build.go`, `internal/cli/actions.go`, `internal/delegate/**` | not started |
| P4 | A host that exports no session gets one at install, and the config keys for the source (inferred) | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | L02, L06, L07, L04: `internal/cli/install.go`, `internal/hosts/hook.go`, `internal/hosts/install.go`, `internal/config/**`, `CONSTRAINTS.md` | not started |

P4 is inferred from #871 Expected paths only; the body names no host gap and no key.

## Delivered

Nothing under #871. Parts that other Features delivered and that P1 to P3 can reuse:

- `ready.Parent` with host, session, URL, mode, and `login@machine` for a mission's Kanha:
  [9325551](https://github.com/rahulnakmol/agentic-ayudha/commit/93255510) (#1092, Story #1004 of Feature #889).
- `source_session` on each baseline draft:
  [af81cb4](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48) (#1018, Story #990 of Feature #874).
- From before v0.3.0: `Env.session` ([86f4a85](https://github.com/rahulnakmol/agentic-ayudha/commit/86f4a85d),
  #280), `doctor.DetectHost` ([34e6b92](https://github.com/rahulnakmol/agentic-ayudha/commit/34e6b92d), #15),
  `leaseHolder` ([bf0a837](https://github.com/rahulnakmol/agentic-ayudha/commit/bf0a8372), #109), and the
  `Aayudha-Host` checkpoint trailer (`internal/cli/build.go:670-673`).

## In review

None (`_data/open-prs.tsv`).

## Pending

- All of #871 Scope: P1 to P4 above.
- Person steps before build: the Feature's baselines (intent, spec, tech) and Approve to build. None exists on the
  ledger (`specs/871/` is absent).
- The meaning of "reviews" and "drafts" in the Summary: drafts are partly covered by `source_session`; the evidence
  does not show which review record is meant. Unverified.
