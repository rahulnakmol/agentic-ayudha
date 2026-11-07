# Sequencing: Feature #877

## Shape of the work

Pending: Bug #1086 and Stories #1082, #1083, #1029. The design's story order is #995, #996, #1082, #1083, #1029
(`specs/877/design.json`). The depends-on links make a chain #1082 → #1083 → #1029, and #1029 also needs #1028
from Feature #874. #1086 has no dependency. So at most two lanes run at once. Ten lanes do not make sense: three of
the four items edit the same Approve to build files (`internal/cli/readyapprove.go`, `readycover.go`,
`internal/ready/approve.go`) in a fixed order.

## Before any lane starts (a person)

1. Approve #877 to build: `aayu ready 877` in the founder's terminal, with the device code. There is no
   `specs/877/approved.json`. Until then `aayu ready` refuses each Story of #877 (`earlyCovered`).
2. Arjun makes #1082 Ready under that approval. #1086 is a Bug; #996's rule covers Stories only (#1086 Notes), so
   its Ready does not wait for the approval.

## Waves

| Wave | Lane | Item | Files or packages | Waits for |
|---|---|---|---|---|
| 1 | A | Bug #1086 grill refuses a parentless Story | `internal/cli/grill.go` (the `d.Parent` block), `skills/grill/SKILL.md`, grill eval cases under `evals/cases/grill/` | none |
| 1 | B | #1082 `--ack` | `internal/cli/ready.go` (new path), `internal/cli/readycover.go` (next command `--ack`), `internal/ready/` (new ack record, suggested `ack.go`), `internal/verbs/registry.go` (`--ack`), generated `internal/mcp/schemas/ready.json`, `skills/ready/SKILL.md` frontmatter | Approve to build of #877 |
| 2 | A | #1083 receipts (T23) | `internal/cli/presence.go`, `internal/cli/readyapprove.go`, `internal/cli/readycover.go`, `internal/cli/ready_early.go`, the ack path from #1082, `internal/ready/approve.go` (`Presence.Receipt`), `internal/audit/`, `internal/config/` (`[identity] presence_app`), `docs/threat-model.md` (row T23) | #1082 |
| 3 | A | #1029 high-risk choices | `internal/cli/readyapprove.go`, `internal/ready/approve.go` (`risks`, `appetite`), `internal/verbs/registry.go` (`--choice`), `internal/cli/inboxapprove.go` (the placeholder line), `skills/ready/SKILL.md` | #1083; #1028 (Feature #874) |

Story #1083 is effort L. A person could re-slice it into the receipt write, the Ready check, and the audit, to run in
parallel; no source proposes that split, so it stays one lane here.

## Cross-Feature lanes in the same waves

- Wave 1 can also hold #1028 (Feature #874: `internal/planning/plan.go`, `internal/cli/plan.go`), #919 (Feature
  #875: `internal/verbs/` table, `internal/config/`), and #1114 (Feature #876: `internal/workitems/critique/`).
  #1082 and #919 both regenerate from `internal/verbs/registry.go`; merge one, then rebase the other and run
  `go generate ./...`.
- #1086 and #1114 (Feature #876) both edit `internal/cli/grill.go`; #1086 and #1116 both edit
  `skills/grill/SKILL.md`. Merge #1086 first.
- #1083 and #1029 edit `internal/cli/readyapprove.go`, as does #1114's wiring line. #1082 and #921 (Feature #875)
  and #1106 (Feature #874) all edit `internal/cli/ready.go`. Name the merge order in each wave.

## Hot files that force serialized merges

- `internal/cli/readyapprove.go`, `internal/cli/readycover.go`, `internal/ready/approve.go`: #1082, #1083, #1029.
- `internal/verbs/registry.go` and its generated files (`internal/mcp/schemas/ready.json`, skill frontmatter,
  `docs/README.md`): #1082, #1029.
- `internal/config/defaults.toml`, `internal/config/gen`, and `CONSTRAINTS.md`: #1083.

## Steps that need a person

- Approve to build of #877 (above), and every later approval, acknowledgment, and choice.
- Shipping: #877 Scope says "It changes who may approve, so its PRs need a person to ship"; AGENTS.md lists "a
  change to who may approve or bypass" in the narrow set.
- `internal/cli/presence.go` and `internal/config/` are in `[autonomy] always_person` (`.aayudha/config.toml`), so
  #1083 waits in `aayu inbox` for a person even with proof.
- `skills/` is a CODEOWNERS path: #1082, #1029, and #1086 need the founder's recorded bypass for
  `protected-paths` ("A person ships this Bug, because skills/ is a protected path", #1086 Notes).
- A change to `.github/workflows/aayudha-ready-guard.yml` (in #877's Expected paths) is under `/.github/` in
  CODEOWNERS. No pending item names it.
- After #1083 merges, records written before it carry no receipt and stop counting for Ready (#1083 Review
  guidance, Risk). Today that is `specs/874/approved.json` and `specs/1170/approved.json`. So: make the remaining
  #874 Stories (#1028, #1106) Ready before #1083 merges, or plan for the founder to approve #874 again.
- PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) (draft, Decision #1255) would narrow the
  person-only set to the policy keys. It was closed unmerged on 2026-11-07 when the mission of #1170 ended; branch
  `feat/1281-auto-merge-on-proof` at `6f2aae7` is kept (`_data/closed-prs.tsv`). So the rules above apply today.
