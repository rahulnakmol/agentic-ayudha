# #879 Implementation plan

## Stories and Bugs

No Story or Bug is filed under [#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879). The P rows are
proposed, not filed; effort and risk are blank because no source estimates them. #954 is filed under Feature #975
and listed for reference.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | The Bug form asks where it happens (proposed, not filed) | Story | not filed | — | — | — | L03 `internal/workitems/schema.go`, generated `work-items/forms/aayudha-bug.yml`; L09 generated `.github/ISSUE_TEMPLATE/aayudha-bug.yml` | not started |
| P2 | A person triages a Bug with `aayu triage <bug> --decide`, recorded in `specs/<bug>/triage.json` (proposed, not filed) | Story | not filed | — | — | P1 (reads the field into the repro hash) | L02 `internal/cli/triage.go`, `triage_test.go`; L03 `internal/triage/**`, `internal/verbs/registry.go` and its generated files, `skills/triage/SKILL.md`; `GLOSSARY.md` | not started |
| P3 | Ready refuses a Bug with no hotfix triage (proposed, not filed) | Story | not filed | — | — | P2 | L02 `internal/cli/ready.go`, `readycover.go`, `ready_test.go`; L03 `internal/ready/**` | not started |
| P4 | The TRIAGE card in the inbox (proposed, not filed) | Story | not filed | — | — | P2, [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) | L02 `internal/cli/inbox.go`, `inboxtui.go`; `internal/needs/**` | not started |
| [#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954) | Triage a bug from its repro report: hotfix, carve out a Feature, better repro, or not a bug (under #975, reference only) | Story | planned | S | medium | #879, #937 (#954 Links) | not stated in this review | not started |

## Delivered

Nothing of #879 is delivered. The Story half of Decision #861 is delivered under Feature #877: PR #1026
([52f8b96](https://github.com/rahulnakmol/agentic-ayudha/commit/52f8b96b)) for Story #996.

## In review

None. No open PR refers to #879.

## Pending

- P1 to P4, after the baselines and Approve to build.
- Overlap to settle: #954 under #975 designs the same card as P4. #954 depends on #879 and says "this card only runs
  the verb as the person" (#954 Links and Assumptions).
- What `feature` and `repro` write is not in any source (see [02-spec.md](02-spec.md)).
- No source gives an estimate.
