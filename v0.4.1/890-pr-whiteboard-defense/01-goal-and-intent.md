# #890 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/890`. This page uses
[#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890), Decision
[#904](https://github.com/rahulnakmol/agentic-ayudha/issues/904), Release
[#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105), and Stories
[#1107](https://github.com/rahulnakmol/agentic-ayudha/issues/1107) and
[#1108](https://github.com/rahulnakmol/agentic-ayudha/issues/1108).

## Goal

Quoted from #890: "Every PR that can merge into dev has a Defense section that cites code, and a cold challenge
blocks a risky PR whose defense the code does not support."

Decision #904 states the founder's rule: he "may ask anyone, at any time, to explain a system they shipped".
Customer-facing means "anything that can merge into `dev`, because `dev` reaches `main` with no human step".

## Who it serves and the customer outcome

- The founder, who must be able to question any shipped system (#904 Question).
- Every person and agent who opens a PR into `dev`: a missing or uncited Defense blocks a person's PR as well as an
  agent's (#904 option 1).
- From #1107 and #1108: "so that done means compatible and documented, not only green", and "so that done means
  the architecture still holds and the tests leak nothing".

## Problem

Quoted from #890: "Nothing enforces that a shipped system can be defended (V10, C34, G19, Q19). Decision #865."

The code confirms it on `dev`: `ETHOS.md` lists principles 1 to 10 and no principle 11, and the PR template
(`.github/pull_request_template.md`) has no Defense section. Its sections are "What this does", "Why", "What
changed", "How we know it works", "Risk and rollback", and "For reviewers".

## Scope

In, quoted from #890:

> ETHOS principle 11; the Defense section in the PR template (five parts); the deterministic check in `prBodyFloor`
> for every PR into dev; the cold challenge in the merge gate for risk medium or high, the narrow set, and
> threat-model paths; the 1-in-10 spot check after merge; the ledger copy; `aayu explain`.

Decision #904 changes the challenge trigger: it blocks a PR whose risk tier is at or above `[inspect]
challenge_tier`, `"high"` by default, using the tier from Decision #902.

Issue #890 also says: "when #890 is sliced, its PR template story adds the DoD checklist, one row per item, each naming
the verify floor that checks it (#1107 contracts and generated docs, #1108 layer boundaries and fixtures, both
v0.4.1) or the person who answers it."

## Out of scope

Quoted from #890: "PoCs, demos, spikes, and early-build prototypes, which never merge." #890 also says "Its PR needs
a person to ship."

## Success measures

The sources state no success measure. Decision #904 names a review trigger: "if the 1-in-10 spot check fails on
more than 1 medium PR in 10, set the tier to `medium`." The Stories measure "Completion Goal criteria observed",
threshold "all 5 criteria" (#1107) and "all 6 criteria" (#1108).
