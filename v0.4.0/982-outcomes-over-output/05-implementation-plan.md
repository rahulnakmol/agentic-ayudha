# Implementation plan: Feature #982

## All Stories and Bugs

Type, status, effort and risk are from `_data/subs.tsv`. No Story has a depends-on link (each Links section is empty,
and `spec-v1.json` pins `depends_on: []` for all six). The "Needs" column is inferred from each Story's Assumptions,
which name O1 (#984) and O3 (#986) as sources. Paths are from #982's expected paths.

| # | Title | Type | Status | Effort | Risk | Depends on (link / needs) | Layers / paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) | A Feature names the customer outcome it serves, or says why it has none | Story | planned | M | medium | — / a parent-chain metric for c4 (the new Goal, not sliced) | L03 `work-items/forms/aayudha-feature.yml`, `internal/grill/**`, `internal/workitems`; L02 `internal/cli/grill.go`; L09 `.github/ISSUE_TEMPLATE/aayudha-feature.yml` | not started |
| [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) | Approve to build shows the Feature's outcome next to its budget, time and cost | Story | planned | S | medium | — / #984 ("reuses the Outcome section from O1"); #877 S1 budget phrase; web card needs #976 | L03 `internal/ready/**`; L02 `internal/cli/ready.go`, `internal/cli/inboxapprove.go` | not started |
| [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) | Did it move? An outcome card some days after a Feature ships | Story | planned | M | medium | — / #984; #873 (Need kind); #989 (measure) | L02 `internal/cli/inbox.go`; L03 `internal/measure/**`; L07 `internal/config/` (`readout_days`) | not started |
| [#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987) | Releases and a weekly retro show outcomes beside counts, and the retro ends with a next step | Story | planned | M | low | — / #986 ("Counts read the outcomes O3 records"); web parts need #976 | L02 `internal/cli/release.go`, `internal/cli/retro.go`; L03 `internal/verbs/registry.go` (`--week`) | not started |
| [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988) | A Goal states what we do best, and new work is checked against it | Story | planned | S | medium | — / none | L03 `work-items/forms/aayudha-goal.yml`, `internal/grill/**`, `internal/triage/**`, `skills/grill/SKILL.md`; L09 `.github/ISSUE_TEMPLATE/aayudha-goal.yml` | not started |
| [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989) | Measure founder minutes per merged PR | Story | merged | M | medium | — | L03 `internal/measure/founder.go`; L02 `internal/cli/measure.go`; L07 `internal/config/`; L04 `CONSTRAINTS.md` | PR [#1016](https://github.com/rahulnakmol/agentic-ayudha/issues/1016), squash [2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53), 2026-10-25 |

No Bug is filed under #982 (`_data/subs.tsv`).

## Delivered

- [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989): `aayu measure --metric aayu:founder_minutes_per_pr`
  reads the actions log and merged PRs, with `[outcomes] session_gap_minutes = 10` and `action_minutes = 2`
  ([2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53); `measure.Founder` in
  `internal/measure/founder.go`; `founderMinutes` in `internal/cli/measure.go`). Its four mapped tests pass on
  884d81b, whose measure code is the same as on bc78349 (run 2026-11-07; see
  [03-technical-design.md](03-technical-design.md)).
- Caveats:
  - It merged before the Feature's baselines (2026-10-25, against intent, spec and tech drafted 2026-10-31 and
    approved 2026-11-02; ledger commits `423c2840`, `940544dc`).
  - It merged without `aayu ship`, but not without proof. On PR #1016, the merge gate posted "Proof pack: proven"
    for head `dafa588` (2026-10-25T16:03:45Z). `aayu ship` then posted "Ready to ship … its pack verifies on
    `dafa588`" and stopped only for the human pass, because the PR touches `internal/config/` in the narrow set
    (17:30:46Z). @rahulnakmol merged it after a "Solo-mode bypass:" comment (17:55:51Z), so there is no ship
    record (no `proof/989/…/ship.json` on the ledger).
  - The audit comment on #989 (2026-10-25T17:57:24Z) says "no authoritative pack exists for it on the ledger … It is
    done but unproven". That was true then, but it is stale: the pack, built by merge-gate run 40213240191 at
    `dafa588`, landed on the ledger on 2026-10-30 (ledger commit `cb814b7`,
    `proof/989/dafa5887fd0579c49bf8902ec704e4171e54d915/authoritative/pack.json`, writer "rahulnakmol (aayu proof
    land)"). Whether `aayu audit` still counts #989 as uncovered is unverified (not run).
  - The issue is still open with `status:merged` (GitHub, read 2026-11-07).

## In review

Nothing. No open PR into dev refers to #982 or its Stories (`_data/open-prs.tsv`).

## Pending

| Story | What is missing on dev | Blocker |
|---|---|---|
| #984 | Outcome field, `outcome:none` label, grill check | Approve to build of #982 (no `specs/982/approved.json`); Ready after Decision #981 (accepted) and Approve to build (#984 Assumptions) |
| #985 | Outcome line and refusal in Approve to build | #984; the web card (c4) needs #976 |
| #986 | Outcome card, ledger record, `readout_days` | #984; #873 (open, planned) |
| #987 | Outcome counts in release and retro; `retro --week` | #986; the web parts (c3, c4) need #976 |
| #988 | "What we do best" on the Goal form; grill and triage question | Approve to build only |

## Gaps not sliced into Stories

1. **The 0.4.0 Goal.** #982's scope and Decision [#981](https://github.com/rahulnakmol/agentic-ayudha/issues/981)
   option 1 include "a 0.4.0 Goal with a customer outcome measure that replaces Goal #29 as #870's parent". No Story
   covers it, and #870's parent is still [#29](https://github.com/rahulnakmol/agentic-ayudha/issues/29) (GitHub
   sub-issue parent API, 2026-11-07). The only Goals are #29 and the example [#125](https://github.com/rahulnakmol/agentic-ayudha/issues/125).
   #984 c4 and #985 c1 use `aayu:founder_minutes_per_pr` as a metric of #870 or its Goal, but #870's Metric is
   `github:prs_merged base:dev` and #29's is a work-package count (issue bodies). So those criteria cannot pass until
   the founder files the new Goal with that metric. Filing a Goal is a person's step (Decision #981 says "a new 0.4.0
   Goal under the founder names its customer and an outcome measure that he picks"). No estimate in any source.
2. **The web parts.** #985 c4, #987 c3 and c4 need web screens of Feature #976, which is not built and not in their
   Links. Either those criteria wait for #976, or a person splits them out.
3. **The UX baseline.** #982 says its stories are made Ready only after the "intent, spec, and UX design baselines are
   approved". There is no UX baseline on the ledger. The Approve to build card needs one only for an L01 footprint
   (`approveCard` in `internal/cli/inboxapprove.go`), and #982's design footprint is L03, so the code does not force
   this step.
