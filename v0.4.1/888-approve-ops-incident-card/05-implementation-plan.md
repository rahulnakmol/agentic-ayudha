# #888 Implementation plan

## Stories and Bugs

No Story or Bug is filed under [#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888) (no GitHub
sub-issues; no row in `_data/subs.tsv`; no PR in `_data/merge-refs.tsv` or `_data/open-prs.tsv`). The rows below are
proposed, not filed. Effort and risk are blank where no source gives them. Story #947 is listed for reference: it is
filed, but under Feature #975.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | `aayu respond <incident> --approve <run>` approves the pending deployment as the person (proposed, not filed) | Story | not filed | — | — | — | L02 `internal/cli/respond.go`, `internal/cli/respond_test.go`; L03 `internal/verbs/registry.go` and its generated files; `docs/threat-model.md` | not started |
| P2 | The INCIDENT card in Now runs the approval (proposed, not filed) | Story | not filed | — | — | P1, [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) | L02 `internal/cli/inbox.go`, `internal/cli/inboxtui.go`; `internal/needs/**` (after #873) | not started |
| [#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947) | Approve an incident's ops action from its card, with GitHub as the gate (under #975, reference only) | Story | planned | M | high | #888, #937 (#947 Links) | not stated in this review | not started |

## Delivered

Nothing of #888 is delivered. The respond verb, its catalog, and `aayudha-ops.yml` exist from before v0.3.0
(`internal/cli/respond.go`; the commit is not checked here).

## In review

None.

## Pending

- P1 and P2, and the baselines and Approve to build before them.
- Overlap to settle before slicing: P2 and Story #947 describe the same card. A person decides which Feature owns it.
- No source gives an estimate.
