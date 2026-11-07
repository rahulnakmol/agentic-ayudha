# Spec: Feature #877

## Spec baseline

`specs/877/baselines/spec-v1.json` on `ledger`: owner `product-strategist`, approved by `rahulnakmol` at
2026-11-02T17:20:56Z through `device-flow`, hash `e33ab327cbaa…`. It pins five stories and the set
`#877 stories`. Bug #1086 is not pinned: the spec part pins child stories only (`featureStories` in
`internal/cli/plan.go` lists sub-issues of type story).

| Story | Effort | Layers (pinned) | Depends on (pinned) |
|---|---|---|---|
| [#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995) | M | L02, L03, L07 | none |
| [#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996) | M | L02, L03, L06 | #995 |
| [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) | M | none | #995, #1028, #1083 |
| [#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082) | M | none | #995, #996 |
| [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083) | L | none | #994, #995, #996, #1082 |

## Functional requirements

| # | Requirement | Story, CG | State |
|---|---|---|---|
| FR1 | Approve to build is refused for an agent lane and over MCP, with exit 4, before any write. | #995 CG 1 | delivered, with a change: see the note below |
| FR2 | A person's `aayu ready <f>` shows the effect line before the device code ("Approves #840 to build: 6 stories (#851-#856), intent v1, spec v2, technical design v1, budget up to $360 (6 x $60, [budget] per_story_usd)"), then writes `specs/<f>/approved.json` (version, by, at, presence, baselines, stories, budget, dismissals). | #995 CG 2 | delivered |
| FR3 | After the write, Arjun posts the Feature's Ready snapshot and labels it `status:ready`; the result line says "Next: Arjun makes its N stories Ready." | #995 CG 3 | delivered |
| FR4 | Approve to build is off, with one line and the next command, when a needed part is not approved, a part drifted, or the Feature has no Initiative parent. | #995 CG 4 | delivered |
| FR5 | When the stories change while the device code waits, nothing is approved. | #995 CG 5 | delivered |
| FR6 | A second approval after a new part version shows that version's what, why, and need, writes version 2, and the ledger log keeps version 1. | #995 CG 6 | delivered |
| FR7 | Approve to build is off while a review gap is open or reviews are missing inside `[critique] review_wait_hours`, and it ratifies dismissals into `approved.json`. | #995 CG 7 | delivered against a test fake; no real critique state exists (Features #875, #876) |
| FR8 | `aayu ready <story>` refuses a Story with no parent Feature, under a Feature with no approval, under a Feature whose parts changed after approval, and a story that changed (Completion Goal, effort, title) or was added after approval. | #996 CG 1-4 | delivered |
| FR9 | A covered story becomes Ready with the snapshot row "Approved to build \| #840 approval 1 by @rahulnakmol". An early-build record covers its one story only. | #996 CG 5, 6 | delivered |
| FR10 | A story made Ready before the rule keeps `status:ready` under the guard, and `build --next` still treats it as Ready (F4). | #996 CG 7 | delivered |
| FR11 | An agent drafts an acknowledgment with `aayu ready <f> --ack <stories> --request --draft ack.json` (what, why, need); the draft goes to `specs/<f>/ack-<k>.draft.json`. Without `--request`, an agent lane is refused. | #1082 CG 1, 2 | not built |
| FR12 | A person's `--ack` shows each story's change and the new budget before the device code, then writes `specs/<f>/ack-<k>.json`. It is refused when the approval does not hold, a story is not a child, is covered already, or changed after the draft. A story covered by the ack becomes Ready with the row "Acknowledged \| #840 ack 1 by @…". | #1082 CG 3-5 | not built |
| FR13 | Before the ledger write, the person's device-flow token posts one receipt comment `<!-- aayudha:presence -->` naming the record and its SHA-256; the record's `presence` gains `receipt`. The same holds for early-build and ack records. | #1083 CG 2, 3 | not built |
| FR14 | Ready refuses a record with no receipt, a receipt by another login or a bot, an edited receipt, a wrong hash, or a receipt on another issue. With `[identity] presence_app` set, a receipt not written through that App is refused. | #1083 CG 4, 5 | not built |
| FR15 | `aayu audit` names each record with no valid receipt as a high T23 finding, and notes the residual when `presence_app` is unset. `docs/threat-model.md` gains row T23. | #1083 CG 6, 7 | not built |
| FR16 | Approve to build lists the high-risk entries by story from each plan's register, asks "1 auto-merge · 2 review the PR · 3 review before build" per entry, or takes `--choice <story>.<n>=<choice>` with no terminal, and writes `risks` rows into `approved.json`. | #1029 CG 1-3 | not built |
| FR17 | An `authority` entry cannot be auto-merged. A story with no plan keeps Approve to build off. A plan change during the device code approves nothing. `--choice all=auto-merge` writes `appetite` with the entries listed by number. | #1029 CG 4-8 | not built |
| FR18 | `aayu grill story` with no `--parent` exits 4 and names the Tatkal route; a parent that is not a Feature is refused; the guard and `workitems.Validate` do not change; `skills/grill/SKILL.md` asks for the parent Feature. | #1086 CG 1-4 | not built |

Note on FR1. #995 CG 1 expects a refusal for `AMP_THREAD_ID`. Bug
[#1182](https://github.com/rahulnakmol/agentic-ayudha/issues/1182) (Feature #1170) changed this in PR
[#1188](https://github.com/rahulnakmol/agentic-ayudha/issues/1188)
([c64bccf](https://github.com/rahulnakmol/agentic-ayudha/commit/c64bccf)). Now `Env.laneRefusal` in
`internal/cli/readyapprove.go` refuses only `AAYU_AGENT=1` (MCP and the loops) and CI. A shell with only a host's
agent marker goes on to the device code, "because an agent never enters or relays it".

## Non-functional requirements

| # | Requirement | Source |
|---|---|---|
| NFR1 | Security (authority): Approve to build, `--ack`, and every choice are a person's decision through GitHub's device flow every time. | #995, #1082 Behaviours; `presenceCheck` in `approveToBuild` |
| NFR2 | Security (forgery, T23): a record with the right shape and a person's name must not make a story Ready without a GitHub-attributed receipt. Today it does: `Presence.ActionsRecord` is a hash of a local log line (`internal/ready/approve.go`). | #1083 Notes |
| NFR3 | Integrity: everything is read again after the device code, and only what the person saw is approved. | #995 CG 5; `approveToBuild` |
| NFR4 | Compatibility: a rule added at Ready never removes a Ready granted before it; the story rules never run in the guard or the build pick (F4). | #996 Behaviours 2; #1086 Behaviours 2; `ready.Ready.Covered` comment in `internal/ready/ready.go` |
| NFR5 | Cost: the effect shows the budget, `[budget] per_story_usd` (default 60.0) × stories. | #995 CG 2; `internal/config/defaults.toml` |
| NFR6 | Records are versioned on the ledger and never rewritten; new fields are optional. | `specs/877/design.json` rollback |
| NFR7 | Operability: Ready, build, and the guard read the ledger, not a comment (F2). Delivered for `aayu ready <story>`; not checked for build and the guard (unverified). | #877 Scope |
| NFR8 | Shipping: "It changes who may approve, so its PRs need a person to ship." | #877 Scope; AGENTS.md (a change to who may approve) |
