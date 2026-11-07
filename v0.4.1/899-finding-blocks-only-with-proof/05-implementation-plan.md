# Implementation plan: #899 a finding blocks a merge only with proof

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #899. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 899. |

## Delivered

Nothing of #899 is delivered. The review rules of Decision #910 that #899 carries (proof attempt, follow-ups) are not
in `internal/inspect/inspect.go` on 884d81b: the file has no "Follow-up" path and no probe field. Inspect work merged
since v0.3.0 is about reading and carrying findings, not proving them:
[18845b72](https://github.com/rahulnakmol/agentic-ayudha/commit/18845b72) (#1135),
[1fe6b53a](https://github.com/rahulnakmol/agentic-ayudha/commit/1fe6b53a) (#1156),
[b7b36fda](https://github.com/rahulnakmol/agentic-ayudha/commit/b7b36fda) (#1180),
[d40b7c9b](https://github.com/rahulnakmol/agentic-ayudha/commit/d40b7c9b) (#1145).

## In review

No open PR refs #899 (`_data/open-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The issue gives no effort and no
estimate.

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 899-A | A replay of PR #828 measures how many findings would have blocked | — | L05 `evals/cases/inspect/` (new case), `internal/evals/review.go` (inferred) |
| 899-B | A finding carries a probe or a candidate patch, and a prover pass writes a probe for each P0 or P1 | — | L03 `internal/inspect/inspect.go` (`Finding`), reviewer prompt files (inferred) |
| 899-C | The sandbox runs a finding's probe or patch on the PR head | 899-B | L04 `internal/verify/**`; L09 `.github/workflows/aayudha-merge-gate.yml` |
| 899-D | Only a validated finding blocks; an unproven P0 or P1 becomes its own Bug; a P0 on a CODEOWNERS path always blocks | 899-B, 899-C | L03 `internal/inspect/inspect.go` (`finish`, `Enforce`); L02 `internal/cli/inspect.go` |
| 899-E | After `merge`, a late P2 or P3 is a follow-up and one Bug | — | L03 `internal/inspect/inspect.go`; L02 `internal/cli/inspect.go` |
| 899-F | Finding precision goes into the merge record | #897 (merge record), 899-D | L07 `internal/ledger/**`; the merge-record writer of #897 |
| 899-G | A declined finding needs no RAID row | 899-D, 899-E | `AGENTS.md`, `skills/build/**`, `internal/delegate/brief.go`, `docs/guide/how-merge-gate.md`; the PR-body check in inspect (inferred) |
| 899-H | The threat model names the probe sandbox | 899-C | `docs/threat-model.md` |

### Gaps the sources do not close

- What "the sandbox" is. #899 does not say. 899-C starts with that choice.
- 899-E is Decision #910 rule 2. It is not in #899's Scope list, but #899's Summary says "Other findings become
  follow-ups". A person confirms whether it belongs here or to another Feature.
- The `review-escapes` eval that #910 keeps as the recall check is not found on 884d81b.
