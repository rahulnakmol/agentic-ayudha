# Specification: Feature #982

## The approved spec

Spec v1 was approved by @rahulnakmol on 2026-11-02T17:25:05Z through the device flow
(`specs/982/baselines/spec-v1.json`). It pins six Stories, each with its Completion Goal hash, effort, layers, and
depends-on:

| Story | Effort | Layers pinned | Depends on pinned |
|---|---|---|---|
| [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) | M | none | none |
| [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) | S | none | none |
| [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) | M | none | none |
| [#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987) | M | none | none |
| [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988) | S | none | none |
| [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989) | M | L02, L03, L04, L07 | none |

The #989 Completion Goal hash in the spec (`0c8ff6dc…`) equals the hash in its Ready snapshot (`specs/989/ready.md`).
The story pins use a hash that is not the plain SHA-256 of the issue body, so I did not check the other five for
drift; unverified. The requirements below quote each Story's Completion Goal ("cN" is criterion N).

## Functional requirements

### The Outcome section ([#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984))

1. FR-1. `aayu grill feature` on a draft with no `### Outcome` and no `outcome:none` label refuses with "not ready:
   the Feature names no outcome: who it helps, the metric it serves, and the expected change. Add ### Outcome, or the
   label outcome:none with a reason.", exit 4, and writes nothing (c1).
2. FR-2. A draft whose Outcome names the beneficiary, a parent-chain metric, the expected change, and "Core: yes"
   passes Create and Ready; the body keeps the Outcome as written (c2).
3. FR-3. `outcome:none` passes only with a "Reason:" line; otherwise "outcome:none needs a Reason line" (c3).
4. FR-4. An Outcome metric that no Goal or Initiative in the parent chain has is refused, naming the allowed metrics,
   such as "github:prs_merged, aayu:founder_minutes_per_pr" (c4).
5. FR-5. The GitHub Feature form has an Outcome field after Problem with four prompts: who it helps, the metric, the
   expected change, core competency (c5). The section has four lines: Who, Metric, Change, Core (Assumptions).

### Approve to build shows the outcome ([#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985))

1. FR-6. `aayu ready <f>` (Approve to build) shows "Outcome: founders · founder minutes per merged PR · falls · core"
   before the budget effect line of [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) S1 (c1).
2. FR-7. With no Outcome and no `outcome:none`: exit 4, `Approve to build is off: #<n> names no outcome. Next: aayu
   grill feature <n>`, nothing written (c2).
3. FR-8. With `outcome:none`: "Outcome: none (CI only; no customer sees it)", and the approval goes on (c3).
4. FR-9. The web inbox Approve to build card shows the same Outcome line, amber when the Feature has none (c4).

### Did it move? ([#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986))

1. FR-10. `[outcomes] readout_days` (default 14) after a Feature's last story merges, an inbox card "Did it move? #982
   Outcomes over output" shows in the Decide area, with the metric before and after from `aayu measure` and the
   Feedback that links the Feature (c1).
2. FR-11. The answer (Moved, No change, Unknown) and one line are written to the ledger as the Feature's outcome, with
   the person and the time; the card leaves (c2).
3. FR-12. A Feature may set `readout_days` in its Outcome section; it wins over the config (c3).
4. FR-13. `outcome:none` gets no card (c4).
5. FR-14. With no source for the metric, the card still shows, with "Measure: no source for
   aayu:founder_minutes_per_pr; record what you saw"; "Unknown" never blocks anything (c5).

### Outcomes beside counts ([#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987))

1. FR-15. `aayu release <n>` prints "20 shipped · 4 moved a metric · 2 no change · 3 unknown · 11 not yet read · 1 no
   customer outcome" (c1).
2. FR-16. `aayu retro` gives a table of each shipped Feature with its outcome, its line, and its estimate accuracy (c2).
3. FR-17. The web Releases page and release Tree show the same counts and mark each Feature node (c3).
4. FR-18. `aayu retro --week` and the web History view show recommendations taken or overridden, estimate accuracy,
   escaped P0 and P1 Bugs, outcomes read that week, and one next step with its command (c4).

### What we do best ([#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988))

1. FR-19. The GitHub Goal form has a "What we do best" field after Summary (c1).
2. FR-20. `aayu grill feature` on "Core: no" passes and asks `#<n> does not serve what we do best (“…”). Keep it, or
   say no?`, naming the Product Strategist (c2).
3. FR-21. `aayu triage <feedback> --decide story` quotes the Goal's line and asks before the person decides (c3).
4. FR-22. With no "What we do best", both skip the question and print `the Goal states no core competency; add “What
   we do best” to #<goal>` (c4).

### Founder minutes per merged PR ([#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989), delivered)

1. FR-23. Human-identity actions form sessions: a gap over `[outcomes] session_gap_minutes` (10) starts a new one;
   each session counts its span plus `[outcomes] action_minutes` (2). The example gives 13 minutes and 1.1 per PR
   (c1). Built: `measure.Founder` in `internal/measure/founder.go`.
2. FR-24. `aayu measure <goal> --metric aayu:founder_minutes_per_pr` writes a Result that names the population, the
   window, both keys, and the value (c2). Built: `founderMinutes` in `internal/cli/measure.go`.
3. FR-25. A window with no merged PR says "no merged pull request in the window; founder minutes per merged PR is
   undefined" and writes no Result (c3). Built: `measure.Founder` returns `ErrNoData`.
4. FR-26. Only `identity: human` records count (c4). Built: the filter `r.Identity == "human"` in `measure.Founder`.

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Reliability | "Unknown never blocks" a merge or anything else | [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope; [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) c5 |
| Reliability | Rollback: form sections are optional on read, so a revert stops asking and old issues still parse; outcome cards are append-only ledger records; no record is rewritten | `specs/982/design.json`, `rollback` |
| Reliability | A broken actions-log line stops the measure with exit Blocked, so a dropped record cannot understate it | `founderMinutes` in `internal/cli/measure.go` |
| Data | The measure is a proxy: it counts aayu's recorded human actions, not time elsewhere | `CONSTRAINTS.md` note under `[outcomes]`; `measure.Founder` doc comment |
| Data | Every Result names its population, window, and query | [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989) c2; `m.Query` in `founderMinutes` |
| Operability | Config keys with defaults and generated rows: `outcomes.session_gap_minutes = 10`, `outcomes.action_minutes = 2` (built); `outcomes.readout_days = 14` (planned) | `CONSTRAINTS.md`; `internal/config/gen/main.go`; [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) Assumptions |
| Safety | The core-competency question informs and never refuses; the founder decides | [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988) Assumptions |
| Compatibility | Existing Features keep their bodies; the Outcome check applies to new drafts and at Ready | [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) Assumptions |
| Accessibility | The web card parts (#985 c4, #987 c3) fall under the a11y floors (inferred; the Stories state no axe criterion) | `CONSTRAINTS.md` `verify.floors.a11y.*` |
