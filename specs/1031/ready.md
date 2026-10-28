# rahulnakmol/agentic-ayudha#1031: Ship gaps: a nobody-configured inbox, plan layers from labels, the first story line, and no pack after a merge outside ship

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `898b0df1490716fe1f15fdf6cbd57e3d75a7f1e52a7743e5934445ebdd682906` |
| Completion Goal hash | `1890df96008f4d18e9fdffade67dd1b2600f68eafecdd8ee8fba73613d894a1c` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1031/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: solo mode, `[autonomy] approver = "rahulnakmol"`; PR #1016 by @rahulnakmol touches CONSTRAINTS.md (CODEOWNER @rahulnakmol) and internal/config/config.go (no CODEOWNER), both in never layers, and only the human pass is missing; `aayu ship 1016`; expected: the Ready to ship comment and the refusal name "@rahulnakmol" and "aayu ship 1016 --solo-bypass", and name internal/config/config.go as a never-layer path with no CODEOWNER; "nobody configured" does not show
2. [api-trace] state: Story #992 labelled layer:L02 and layer:L07, whose plan has a checkpoint for L04 because CONSTRAINTS.md is generated from internal/config/defaults.toml; `aayu plan 992 --draft plan.json --create`; expected: the plan passes, layer:L04 is added to #992, and `em_review.layers` lists L04 and L07; `aayu plan 992 --approve` records the review, and ship accepts it for a PR that changes CONSTRAINTS.md
3. [api-trace] state: PR body with the line "Story #990 (Feature #874) is the parent" above "Refs #992"; the merge gate runs `aayu verify`; expected: the pack is for #992; a body with two different `Refs`, `Closes`, or `Fixes` lines fails the gate with "the PR body refers to stories #990 and #992: keep one"
4. [api-trace] state: #989's PR #1016 merged at head dafa588 with `gh pr merge --admin`; the gate's `aayudha-pack` artifact for dafa588 is still kept; a person runs `aayu proof land 1016`; expected: after the same artifact checks ship makes (`gatePack`, internal/cli/ship.go:628), one ledger commit lands the pack with Authority authoritative, the gate's workflow and run ID, and the writer "<login> (aayu proof land)"; then `aayu audit` counts #989 as covered; the same for #995's PR #1024 (head f5052d2, the artifact of run 40292062260) counts #995 as covered; for an open PR, a head that is not the merged head, or an expired artifact, it exits 4 and names the reason
5. [api-trace] state: PR #1085 at head 7d9ad20 has two protected-paths check runs from github-actions in two check suites: 121613879571, cancelled at 13:55:54, and 121613890736, which started at 13:55:59 and passed; every other required check is green; `aayu ship 1085`; expected: ship reads protected-paths as success and goes on to the proof pass; with only the cancelled run, ship still refuses with "required check protected-paths is cancelled"; a cancelled run with no later completed run of the same check, a failure in any suite, and a run still in progress each still decide the state, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Five ship gaps hit in 0.4.0. Ship sends a founder's protected-path PR to the inbox of "nobody configured". A plan cannot name a never layer that ship finds from the files. The merge gate takes the first story line of a PR body. A story merged outside ship can never get its gate's pack onto the ledger as authoritative. Ship reads a cancelled run of a required check instead of the passing run on the same head.

### Steps to reproduce

1. In solo mode, open a PR as @rahulnakmol that touches CONSTRAINTS.md and internal/config/config.go. CODEOWNERS lists only the first. Run `aayu ship <pr>`. This is PR #1016.
2. Plan a story labelled layer:L02 and layer:L07 whose change regenerates CONSTRAINTS.md (L04). Add a checkpoint for L04.
3. Open a PR whose body has a line that starts "Story #990" above its "Refs #992" line. This is PR #1022.
4. Merge a story PR with `gh pr merge --squash --admin` after a "Solo-mode bypass:" comment. Then try to land its pack. This is #989 (PR #1016, head dafa588) and #995 (PR #1024, merged as 45a18e7 at head f5052d2).
5. Let the protected-paths workflow's concurrency group cancel one run on a PR's head while a later run on the same head passes. Run `aayu ship <pr>`. This is PR #1085 (head 7d9ad20).

### Expected behaviour

1. The Ready to ship comment names the person who can ship the PR, and the way: in solo mode, the founder, with the Solo-mode bypass.
2. The plan can name every never layer that ship will find, so the plan-time review can be recorded.
3. The gate builds the pack for the story on the `Refs` line.
4. A merged story gets its gate's own authoritative pack on the ledger.
5. Ship uses the latest completed run of each required check on the head. A run that a later run on the same head replaced does not count.

### Actual behaviour

1. The comment on #1016 says: "Nobody configured (no approver is a CODEOWNER of CONSTRAINTS.md, internal/config/config.go, …) ships it from `aayu inbox`, or approves the head and runs `aayu ship 1016`." `PassRule.who` keeps only the CODEOWNERS common to every never-layer path (internal/cli/shippass.go:55-71, :106-113). internal/config/config.go has no CODEOWNER, so the list is empty.
2. Plan takes its footprint from the layer labels (internal/cli/plan.go:152) and refuses "checkpoint for L04 is outside the story's footprint" (internal/planning/plan.go:164-167). Ship adds the layers of the PR's files (internal/cli/ship.go:292) and asks for a plan-time review of L04 (internal/cli/ship.go:297-313). `aayu plan --approve` refuses a plan with no never layer (internal/cli/plan.go:457-458).
3. `StoryOf` takes the first line that matches `storyRef`, which includes "story:? #n" (internal/cli/verify.go:34, :205-216). The first gate run of #1022 at 401989f built the pack for #990, not #992.
4. `aayu proof push` needs a local pack and writes `Authority advisory` (internal/cli/proof.go:441-456). Audit covers a story only from an authoritative pack (`covers`, internal/audit/fetch.go:152-153, :226). Only ship lands one (internal/cli/ship.go:448-449), and ship refuses a merged PR (internal/cli/ship.go:154-156). #989 shows "Merged without proof (post-gate)". #995 is the same: PR #1024 merged as 45a18e7 at head f5052d2, and its pack is only in the artifact of merge-gate run 40292062260, kept until 2027-01.

### Completion Goal

1. [api-trace] state: solo mode, `[autonomy] approver = "rahulnakmol"`; PR #1016 by @rahulnakmol touches CONSTRAINTS.md (CODEOWNER @rahulnakmol) and internal/config/config.go (no CODEOWNER), both in never layers, and only the human pass is missing; `aayu ship 1016`; expected: the Ready to ship comment and the refusal name "@rahulnakmol" and "aayu ship 1016 --solo-bypass", and name internal/config/config.go as a never-layer path with no CODEOWNER; "nobody configured" does not show
2. [api-trace] state: Story #992 labelled layer:L02 and layer:L07, whose plan has a checkpoint for L04 because CONSTRAINTS.md is generated from internal/config/defaults.toml; `aayu plan 992 --draft plan.json --create`; expected: the plan passes, layer:L04 is added to #992, and `em_review.layers` lists L04 and L07; `aayu plan 992 --approve` records the review, and ship accepts it for a PR that changes CONSTRAINTS.md
3. [api-trace] state: PR body with the line "Story #990 (Feature #874) is the parent" above "Refs #992"; the merge gate runs `aayu verify`; expected: the pack is for #992; a body with two different `Refs`, `Closes`, or `Fixes` lines fails the gate with "the PR body refers to stories #990 and #992: keep one"
4. [api-trace] state: #989's PR #1016 merged at head dafa588 with `gh pr merge --admin`; the gate's `aayudha-pack` artifact for dafa588 is still kept; a person runs `aayu proof land 1016`; expected: after the same artifact checks ship makes (`gatePack`, internal/cli/ship.go:628), one ledger commit lands the pack with Authority authoritative, the gate's workflow and run ID, and the writer "<login> (aayu proof land)"; then `aayu audit` counts #989 as covered; the same for #995's PR #1024 (head f5052d2, the artifact of run 40292062260) counts #995 as covered; for an open PR, a head that is not the merged head, or an expired artifact, it exits 4 and names the reason
5. [api-trace] state: PR #1085 at head 7d9ad20 has two protected-paths check runs from github-actions in two check suites: 121613879571, cancelled at 13:55:54, and 121613890736, which started at 13:55:59 and passed; every other required check is green; `aayu ship 1085`; expected: ship reads protected-paths as success and goes on to the proof pass; with only the cancelled run, ship still refuses with "required check protected-paths is cancelled"; a cancelled run with no later completed run of the same check, a failure in any suite, and a run still in progress each still decide the state, as today

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Ship a founder-authored PR in solo mode, plan a story whose change regenerates CONSTRAINTS.md, gate a PR whose body names another story first, land the pack of a story that was merged outside ship, and ship a PR whose required check has a cancelled run beside a later passing run on the same head.

### Environment

local checkout with the GitHub fake and a synthetic ledger (internal/github/fake); never production

### Assumptions

#### Notes

Criterion 4 is the post-merge path. #1030 (Decision #1027) sends every merge inside a mission through ship, but a solo-mode merge outside a mission can still use `gh --admin`, as AGENTS.md describes for protected paths. Once criterion 1 names `aayu ship <n> --solo-bypass`, that path is rarer, so criterion 4 is mostly for stories that are already merged, such as #989 and #995. #995's artifact is kept until 2027-01, so this Bug must land before then for #995 to be covered. A person ships this Bug, because it changes internal/cli/ship.go and internal/cli/shippass.go, which are in the narrow set.

#### Review guidance

- Check: Criterion 4: `proof land` reads the gate's artifact by run, head, and story, and never accepts a local pack.
- Check: Criterion 3: the body's Refs line decides, and a second, different story reference is an error, never a silent pick.
- Check: Criterion 5: only a cancelled run that a later completed run of the same check, app, and head replaced is skipped; every suite must still pass (`checkState`, internal/cli/shipproof.go:189).
- Risk: internal/cli/proof.go: an authoritative pack written from a source the gate did not produce.
- Ignore: category docs
- Ignore: settled #593 (merge on proof)

#### Behaviours

1. says: A refusal names the person who can act and how; rule: never; source: criteria 1
2. says: Only the merge gate's own pack is authoritative, landed through aayu; rule: authority; source: criteria 4
3. says: Ship judges each required check by its latest completed run on the head, and never merges past a failure; rule: never; source: criteria 5
````
