# Sequencing: Feature #982

## Summary

Five Stories are pending: #984 to #988. The tech baseline orders them 984, 985, 986, 987, 988
(`specs/982/design.json`, `story_order`). No Story has a depends-on link, but the Assumptions make a short chain.
Stories #985 and #986 reuse #984's Outcome section, and #987 reads the outcomes #986 records. So the work fits in
three waves of at most 2 lanes. Ten lanes do not make sense: there are five Stories, and four of them sit on one chain.

## Wave 0: people first

| Step | Who | Source |
|---|---|---|
| Approve to build of #982. Intent, spec and tech v1 are approved (2026-11-02); `specs/982/approved.json` does not exist | founder, through the inbox card (device flow) | `specs/982/baselines/*.json`; `approveCard` in `internal/cli/inboxapprove.go` |
| Decide whether #982 needs a UX baseline first. The Feature says yes; the code does not force it for an L03 footprint | founder | [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope; `approveCard` |
| File the new 0.4.0 Goal with the outcome measure he picks, and make it #870's parent. No Story covers this | founder (Decision [#981](https://github.com/rahulnakmol/agentic-ayudha/issues/981) option 1) | [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope; #870 parent is #29 |
| Check whether `aayu audit` now counts #989 as proven, then decide on #989 (still open, `status:merged`): the audit comment calls it "done but unproven", but its pack landed on the ledger on 2026-10-30 (ledger commit `cb814b7`) | founder | audit comment on [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989); `proof/989/dafa588…/authoritative/pack.json` on `origin/ledger` |

The new Goal must exist before #984 c4 and #985 c1 can pass, because they need `aayu:founder_minutes_per_pr` in the
parent chain (see [05-implementation-plan.md](05-implementation-plan.md)).

## Wave 1: the two form sections (2 lanes)

| Lane | Story | Files and packages |
|---|---|---|
| 1 | [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) Outcome section | `work-items/forms/aayudha-feature.yml`, `.github/ISSUE_TEMPLATE/aayudha-feature.yml`, `internal/workitems` (parse the section), `internal/grill/**` (the check), `internal/cli/grill.go`, `GLOSSARY.md` |
| 2 | [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988) What we do best | `work-items/forms/aayudha-goal.yml`, `.github/ISSUE_TEMPLATE/aayudha-goal.yml`, `internal/triage/**`, `internal/grill/**` (the question), `skills/grill/SKILL.md` |

Shared: both lanes edit `internal/grill/**` and, through the "Core" line, read the same Outcome field. Merge #984
first; #988 rebases and reads `Core: no` from #984's parser. If #988 must merge first, it should put the question in
its own file in `internal/grill/` (recommendation).

## Wave 2: Approve to build and the outcome card (2 lanes)

| Lane | Story | Files and packages | Needs |
|---|---|---|---|
| 1 | [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) Outcome line | `internal/ready/**`, `internal/cli/ready.go`, `internal/cli/inboxapprove.go` | #984 merged |
| 2 | [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) Did it move? | `internal/cli/inbox.go` (a Need kind), a new inbox file for the card, `internal/measure/**`, `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen` (`readout_days`), generated `CONSTRAINTS.md` and `internal/config/schema.json` | #984 merged; [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) Need registry merged |

The lanes touch different files. Both change inbox card lists, so if #986 adds its card kind to the same list that
`approveCard` feeds, merge #985 first (inferred).

## Wave 3: outcomes beside counts (1 lane)

| Lane | Story | Files and packages | Needs |
|---|---|---|---|
| 1 | [#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987) release and retro | `internal/cli/release.go`, `internal/cli/retro.go`, `internal/verbs/registry.go` (`retro --week`) and every file generated from it | #986 merged |

## Later: the web parts

Criteria #985 c4, #987 c3 and #987 c4 need the web inbox, the Releases page, the release Tree and the History view of Feature
[#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976), none built. Build them after #976's waves 3 and 4
(see `v0.4.0/976-local-web-command-center/06-sequencing.md`), or split them out of these Stories. A changed Completion
Goal after Approve to build needs the founder's new acknowledgment ([#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903)).

## Hot files and steps that need a person

- `.github/ISSUE_TEMPLATE/*` (#984, #988) is under `/.github/` in `.github/CODEOWNERS`: a protected path. It needs
  the founder's recorded admin bypass, with a `Solo-mode bypass:` comment first (`AGENTS.md`, Protected paths).
- `skills/grill/SKILL.md` (#988) is under `/skills/`, also protected.
- `internal/config/` (#986) is in `[autonomy] always_person`; `internal/config/defaults.toml` is protected and in
  `always_person_config` (`.aayudha/config.toml`). `CONSTRAINTS.md`, regenerated from it, is protected too.
- `internal/verbs/registry.go` (#987) regenerates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, `skills/*/SKILL.md`
  frontmatter and `internal/mcp/schemas/*.json` (`AGENTS.md`, Generated files). Only one lane in any wave edits it.
- Every Story is made Ready only after Decision #981 (accepted 2026-10-24) and #982's Approve to build (each Story's
  Assumptions).
