# #890 Implementation plan

## Stories and Bugs

Rows #1107 and #1108 are filed Stories (`_data/subs.tsv`; GitHub sub-issues of #890). Rows P1 to P5 are proposed,
not filed; their effort and risk are blank because no source estimates them.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#1107](https://github.com/rahulnakmol/agentic-ayudha/issues/1107) | verify fails an undeclared breaking contract change and contract docs that are not generated | Story | planned | M | low | [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) | L04 `internal/verify/**`, `CONSTRAINTS.md`; L07 `internal/config/**` | not started |
| [#1108](https://github.com/rahulnakmol/agentic-ayudha/issues/1108) | verify fails a forbidden layer import and secrets or personal data in test fixtures | Story | planned | M | low | #1107 | L04 `internal/verify/**`, `CONSTRAINTS.md`; L07 `internal/config/**` | not started |
| P1 | ETHOS principle 11 and the PR template's Defense section and DoD checklist (proposed, not filed) | Story | not filed | — | — | — | `ETHOS.md`; L09 `.github/pull_request_template.md`; L03 `internal/prbody/pull_request_template.md`; `internal/initrepo/**` (inferred: init installs the template, `internal/initrepo/prtemplate_test.go`) | not started |
| P2 | inspect blocks a PR into dev with a missing or uncited Defense (proposed, not filed) | Story | not filed | — | — | P1 | L03 `internal/prbody/check.go`; L02 `internal/cli/inspect_app.go`, `internal/cli/prbody.go` | not started |
| P3 | The merge gate's cold challenge at `[inspect] challenge_tier` (proposed, not filed) | Story | not filed | — | — | P2, Decision #902 tiers, #887 | L03 `internal/inspect/**`; L02 `internal/cli/inspect.go`, `inspect_app.go`; L07 `internal/config/**` | not started |
| P4 | A 1-in-10 cold challenge after merge, and the ledger copy (proposed, not filed) | Story | not filed | — | — | P3, #897 | L09 `.github/workflows/aayudha-defense.yml`; L02 `internal/cli/ship.go` | not started |
| P5 | `aayu explain` asks the owning seat, and "I do not know" opens a Bug (proposed, not filed) | Story | not filed | — | — | P1, #880, #871 | L02 `internal/cli/explain.go`; L03 `internal/verbs/registry.go` and the files generated from it | not started |

## Delivered

Nothing of #890 is delivered. No PR in `_data/merge-refs.tsv` refers to 890, 1107, or 1108. The parts it builds on
are older than v0.3.0 and are listed in [03-technical-design.md](03-technical-design.md).

## In review

None. No open PR refers to 890, 1107, or 1108 (`_data/open-prs.tsv`).

## Pending

- #1107 and #1108: filed and `status:planned`, not Ready. #1107 waits on #1028, which is `status:planned` under
  Feature #874 in v0.4.0.
- Not sliced: the Defense part (P1 to P5). Dependencies #897, #880, and #871 are not sliced either; #887 has three
  planned Stories (#1221, #1222, #1223).
- Not checked here: whether Decision #902's tiers are built anywhere other than `internal/inspect/risk.go`.
- No source gives an estimate for the unsliced part.
