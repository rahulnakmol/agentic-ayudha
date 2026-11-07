# #879 Sequencing

Four proposed slices (see [05-implementation-plan.md](05-implementation-plan.md)). This plan uses at most 2 lanes,
not 10. P2 to P4 form a chain: the Ready rule reads P2's record, and the card runs P2's command.

## Wave 0: person steps

- File P1 to P4 under #879; settle the overlap with #954 (person).
- Baselines (`aayu ready 879 --part <part>`), Approve to build (`aayu ready 879`), `aayu mission start 879`, each by a
  person.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 form field | `internal/workitems/schema.go`, generated `work-items/forms/aayudha-bug.yml`, `.github/ISSUE_TEMPLATE/aayudha-bug.yml` |
| 2 | P2 triage decision (without the field in its hash until P1 merges) | `internal/cli/triage.go`, `internal/cli/triage_test.go`, `internal/triage/**`, `internal/verbs/registry.go` and its generated files, `skills/triage/SKILL.md`, `GLOSSARY.md` |

P1 and P2 edit no file in common. If P2 must hash the new field, run P1 first and P2 in wave 2.

## Wave 2

| Lane | Work | Files |
|---|---|---|
| 1 | P3 Ready rule | `internal/cli/ready.go`, `internal/cli/readycover.go`, `internal/cli/ready_test.go`, `internal/ready/**` |

## Wave 3 (after #873)

| Lane | Work | Files |
|---|---|---|
| 1 | P4 TRIAGE card | `internal/cli/inbox.go`, `internal/cli/inboxtui.go`, `internal/needs/**` |

## Hot files and serialized merges

- `internal/verbs/registry.go` and its generated files (`docs/README.md`, `LAYERS.md`, `docs/RACI.md`,
  `skills/*/SKILL.md` frontmatter, `internal/mcp/schemas/*.json`): shared with #888 and #890.
- Ready files: PRs [#1259](https://github.com/rahulnakmol/agentic-ayudha/issues/1259) and
  [#1262](https://github.com/rahulnakmol/agentic-ayudha/issues/1262) edit `internal/cli/readycover.go` (#1259) and
  `internal/cli/readyapprove.go` (both), not `internal/cli/ready.go` (`gh pr view --json files`). Both were closed
  unmerged on 2026-11-07 when the mission ended; branch `bug-1251-ready-ack` at 2e896ac and branch
  `story/1252-aayu-ready-dry-run-for-approve-to-build` at ac3769a are kept (`_data/closed-prs.tsv`). P3 edits
  `readycover.go` too, so it rebases on #1259 if that PR is reopened and merges first.
- `internal/cli/inbox.go` and `inboxtui.go`: shared with #883, #884, #888.

## Steps that need a person

- P1 regenerates `.github/ISSUE_TEMPLATE/aayudha-bug.yml`, a CODEOWNERS path (`/.github/`): `protected-paths` fails
  by design, and only the founder's recorded admin bypass merges it.
- P2 regenerates `skills/triage/SKILL.md` (CODEOWNERS `/skills/`) and changes who may make a triage decision, a
  change "to who may approve" that AGENTS.md puts in the narrow set (inferred).
- Proof of P2 needs a person to run the device flow.
