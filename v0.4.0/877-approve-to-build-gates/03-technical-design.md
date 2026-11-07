# Technical design: Feature #877

## Tech baseline

`specs/877/baselines/tech-v1.json` on `ledger`: owner `code-architect`, approved by `rahulnakmol` at
2026-11-02T17:21:59Z, hash `9fe90f2ecc4c…`. It pins `specs/877/design.json` (SHA-256 `b6e8ea4de521…`). The design
(planned 2026-10-30T18:28:15Z) has footprint L02, L03, L04, L07, L09, layers L02, L03, L07, the story
order #995, #996, #1082, #1083, #1029, and these contracts (quoted):

1. "specs/&lt;f&gt;/approved.json (merged, #995): unchanged fields; it gains, per high-risk entry of each story's
   register, the person's choice: auto-merge, review the PR, or review before build (#1029)"
2. "aayu ready &lt;f&gt; --ack &lt;story&gt;: a person acknowledges a story added or changed after Approve to
   build, with what changed and why (#1082)"
3. "an approval record counts for Ready only with a receipt that GitHub shows its person wrote (#1083, threat
   T23); a record with no receipt reads as no approval"
4. "storyCovered (internal/cli/readycover.go, #996): unchanged rule, reads the receipt"

Rollback (quoted): "Each record is versioned on the ledger and never rewritten. New fields are optional: a reverted
binary reads a record with choices or a receipt as before. A revert of #1083 makes Ready trust records without
receipts again, which is today's rule."

## Layers touched

L02 (`internal/cli/ready.go`, `readyapprove.go`, `readycover.go`, `presence.go`, `grill.go`), L03 (`internal/ready/`,
`internal/workitems/`, `internal/audit/`, `skills/ready/`, `skills/grill/`), L04 (`CONSTRAINTS.md`), L06
(`internal/github/fake/fake.go`, changed by PR #1026), L07 (`internal/config/`), L09
(`.github/workflows/aayudha-ready-guard.yml`, in the Expected paths). Layer paths from `LAYERS.md`.

## Delivered parts (code on `dev` at 884d81b)

| Concern | Code | Story, PR |
|---|---|---|
| Route a Feature to Approve to build | `runReady` reads the issue type and calls `approveRefusal`, then `approveToBuild` (`internal/cli/ready.go`) | #995, [#1024](https://github.com/rahulnakmol/agentic-ayudha/issues/1024) |
| Who may approve | `Env.approveRefusal` → `Env.laneRefusal` refuses `AAYU_AGENT=1` and CI before any ledger setup; `presenceCheck(…, Pending{What: "Approve to build", …})` (`internal/cli/readyapprove.go`) | #995; refusal order fixed in [#1032](https://github.com/rahulnakmol/agentic-ayudha/issues/1032); host markers allowed to the device code in [#1188](https://github.com/rahulnakmol/agentic-ayudha/issues/1188) (Bug #1182) |
| Conditions | `approveRun.prepare`: Initiative parent, Feature Ready, stories, each needed part approved and not drifted, open gaps, missing reviews, budget (`internal/cli/readyapprove.go`) | #995 |
| Read again after the code | `approveToBuild` compares stories, effect lines, previous version, and mission plan after `presenceCheck`; `ledger.ErrMoved` approves nothing | #995 |
| Record | `ready.Approval`, `ApprovalSchema = "aayudha.feature.approved/v1"`, `ApprovalPath`, `Presence`, `ApprovedBaseline`, `ApprovedStory`, `Budget`, `Dismissal`, `Critique`, `Agreed`, `Effect`, `Ratified`, `OpenGaps` (`internal/ready/approve.go`) | #995 |
| Feature Ready snapshot | `approveRun.ready` posts it as the bot after the approval (`internal/cli/readyapprove.go`) | #995 |
| Mission plan with the approval | `approveRun.missionPlan`, `MissionPlanPath` (written in the same ledger commit) | Story #997, PR [#1084](https://github.com/rahulnakmol/agentic-ayudha/issues/1084) ([d941d15](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d15)) |
| Story coverage | `Env.storyCovered`, `Env.earlyCovered`, `approveRun.featureChanges` (`internal/cli/readycover.go`); `ready.StoryOf`, `ready.StoryChanges` (`internal/ready/cover.go`) | #996, [#1026](https://github.com/rahulnakmol/agentic-ayudha/issues/1026) |
| Rule at Ready only (F4) | `ready.Ready.Covered` runs in `Apply` only; `Check` and `Guard` never call it (`internal/ready/ready.go`) | #996 |
| Inbox card | Approve to build card pinned to the shown stories through `readyExpectHash` (`internal/cli/inboxapprove.go`, `internal/cli/ready.go`) | Story #1173, PR [#1225](https://github.com/rahulnakmol/agentic-ayudha/issues/1225) |

## Ledger records

| Path | Writer | State |
|---|---|---|
| `specs/<f>/approved.json` | a person, `aayu ready <f>`; a new approval replaces it with the next version | delivered (#995) |
| `missions/<f>/plan.json` | the same commit as the approval | delivered (#997) |
| `specs/<f>/approved-early-<s>.json` | a person, `--early-build` | delivered (#994, Feature #874) |
| `specs/<f>/ack-<k>.draft.json`, `specs/<f>/ack-<k>.json` | an agent drafts, a person approves | planned (#1082) |
| `presence.receipt` (comment ID and URL) inside each person's record | the person's device-flow token | planned (#1083) |
| `risks` rows and `appetite` inside `approved.json` | a person at Approve to build | planned (#1029) |

Real records on `ledger`: `specs/874/approved.json` (version 1) and `specs/1170/approved.json` (version 3, by
`rahulnakmol`, 2026-11-02T17:45:16Z, 10 stories). #875, #876, and #877 have none.

## Config keys

| Key | Default | State |
|---|---|---|
| `[budget] per_story_usd` | 60.0 | delivered, read by `approveRun.prepare` |
| `[critique] review_wait_hours` | 24 | delivered (#995, #1032) |
| `[identity] presence_app` | `""` | planned (#1083 CG 5) |
| `[budget] unplanned_usd` | not stated | named in #877 Scope for the Tatkal Feature; not in `internal/config/defaults.toml`; no Story |

## Pending parts (planned, not built)

- [#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082): a second human-only path in
  `internal/cli/ready.go` (`--ack`), records in `internal/ready/`, and a change to `storyCovered` so its refusals
  name `aayu ready <f> --ack <s>` as the next command again. Today the code comments "--ack is not built yet" and
  sets the next command to `aayu ready <f> --part spec --request` (`internal/cli/readycover.go`).
- [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083): spend the device-flow token on one receipt
  comment before it is dropped (`internal/cli/presence.go`), check it in `storyCovered` and `earlyCovered`, add an
  audit finding (`internal/audit/`), and add row T23 to `docs/threat-model.md` (no T23 row exists on `dev`). The
  issue weighs four options and recommends this one (option 4). Story #1030 must use the same check (#1083 Notes).
  Risk named by the Story: records approved before it stop counting, so already-approved Features (today #874)
  need a person to approve them again.
- [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029): reads each story's register from
  `specs/<story>/plan.json` (#1028, not built), asks per entry, adds `--choice`, and writes `risks` and `appetite`.
  The inbox card already holds a placeholder line for these entries (`internal/cli/inboxapprove.go`).
- [#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086): a refusal in `aayu grill story` only, before
  any write (`internal/cli/grill.go`, today it checks a parent only when `--parent` is given); no change to
  `internal/workitems/schema.go`, where the Story type has `Parents: []string{"feature"}` and no `ParentAt`; and a
  rewrite of `skills/grill/SKILL.md`, which still says "A `story` may have a parent Feature, and is Ready without
  one" (line 383).

## Key decisions

- Decision [#861](https://github.com/rahulnakmol/agentic-ayudha/issues/861) (closed, done): every Story needs an
  approved parent Feature, and every Bug a triage decision, before Ready.
- Decision [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903) (closed, done): Approve to build
  agrees the whole Feature once: its baselines, its budget, and every story listed then.
- Decision [#974](https://github.com/rahulnakmol/agentic-ayudha/issues/974) (closed, done): a mission locks its
  issues at start. The lock is delivered under Feature #889 (Story #1013, PR
  [#1096](https://github.com/rahulnakmol/agentic-ayudha/issues/1096),
  [3d42ff1](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff1)).
- Decision [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027) (closed, done): inside an approved
  mission the approval replaces the per-PR person, and `risk:` means rollback impact. It drives #1029.
- Decision [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) (closed, done, 2026-11-02): every
  agent PR into dev merges on proof at any risk and hour. Its Story #1281 is not built: draft PR #1282 was closed
  unmerged on 2026-11-07 when the mission of #1170 ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` is
  kept (`_data/closed-prs.tsv`). It may
  change what #1029's per-entry choices control; no source says so (unverified).
- Deviations recorded in PR bodies: #1024 built `ready.Critique` and `Options.CritiqueState` because #876's state
  did not exist; #1026 built only the refusal that names `--ack`, and made the fake GitHub list every issue.
