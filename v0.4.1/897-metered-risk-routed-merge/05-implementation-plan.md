# Implementation plan: #897 metered, risk-routed merge

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #897. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 897. |

## Delivered

Nothing of #897 is delivered. Building blocks on `dev`:

| Commit | What it gives #897 |
|---|---|
| [469adabb](https://github.com/rahulnakmol/agentic-ayudha/commit/469adabb) (#228, before v0.3.0) | `inspect.RiskOf`: the tier from the diff with no model, and the risk line. #902's rules replace its thresholds. |
| [2e62c537](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c537) (#1016, Story #989) | Founder minutes per merged PR from the actions log: an input of "a person's wait". |
| [9f357b01](https://github.com/rahulnakmol/agentic-ayudha/commit/9f357b01) (#1200), [4e9bd372](https://github.com/rahulnakmol/agentic-ayudha/commit/4e9bd372) (#1160) | The ship record's pack lands with its own gate run's checks. The ship record is the closest per-PR record today. |

## In review

No open PR refs #897. Draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) (Story #1281,
head 6f2aae7) changed `always_person`, which #897's high tier reads. It closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept (`_data/closed-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The issue gives no effort and no
estimate.

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 897-A | The risk tier follows Decision #902, with `[inspect.risk] high_paths` and `prompt_paths` | — | L03 `internal/inspect/risk.go`; L02 `internal/cli/proof.go`; L07 `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen` |
| 897-B | A workflow on push to `dev` writes one record line per merged PR on the ledger | #871 (host and session on every write; what #897 needs from it is not stated) | L09 `.github/workflows/` (new); L07 `internal/ledger/`; L03 `internal/measure/` (record type) |
| 897-C | A nightly sweep back-fills missing records; a missing field is a gap | 897-B | L09 `.github/workflows/` (new or the same file as 897-B); L03 `internal/measure/` |
| 897-D | The factory meter on Initiative #870, each number with source, unit, and age | 897-B, 897-C | L03 `internal/measure/`; L02 `internal/cli/measure.go` |
| 897-E | One configured reviewer on every tier, at most 3 fix rounds, then Bugs for P2 and P3 | 897-A | L03 `internal/inspect/inspect.go`, `internal/inspect/apps.go`; L02 `internal/cli/inspect.go`, `internal/cli/inspect_app.go`; `docs/guide/how-merge-gate.md` |
| 897-F | Rollout silent, visible, enforced, with a kill switch | 897-A, 897-E | L07 `internal/config/`; L02 `internal/cli/ship.go` (if the gate reads the state) |

### Gaps the sources do not close

- 897-E overlaps Decision #910's rules, which no other Feature in this set builds. #899 covers rule 6 (proof) and
  rule 2 (follow-ups). A person confirms that #897 holds rules 1, 3, 4, and 5.
- The record's ledger path and the rollout key are not named.
- The 37-of-528 and 54 % figures in #897's Problem are unverified here.
