# rahulnakmol/agentic-ayudha#544: Decision: inspect's review converges, and the review practices it adopts from Anthropic's code review article

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `3bb0c29e7e9e6f8bdb137c49cc9fcbbafb44c42cc3c64e3dddfdd0156ab292d6` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/544/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

How should inspect's review converge, and which review practices from Anthropic's "How the Claude Code team does code review" (claude.dev, 2026-10-15) should aayu adopt, in what order, with what costs?

Why now: on the night of 2026-10-17, PR #471 took 27 commits and 27 Codex reviews, with 70 inline findings, each round finding one more construct. PR #483 took 10 commits and 11 reviews. aayu's own cycle cap did not stop either loop (evidence under point 1). The founder approved this work on 2026-10-18. Each point below was checked against `dev` at 9d7af1e before it was written.

**Counsel from Atri, the Founder's Sage.** This is a two-way door: every point is configuration, a prompt, or a new check, and each can be turned off again. One level up, the question is not "which features of Anthropic's process do we copy". It is "who stops the loop". Tonight nobody did: aayu stopped judging after two cycles but kept asking Codex for another review on every new head, and the builder kept fixing whatever came back.
- Operations: every extra round costs a CI run (about 15 minutes of the 8 required checks) and builder time, and CI capacity is tonight's bottleneck.
- Technology: most of the machinery exists (the cycle cap, the coverage seal, finding IDs, dismissals, the seeded review eval, the risk tier). The gap is that none of it closes the loop.
- Product: users of aayu run agent builders against agent reviewers. A pipeline that cannot converge is the first thing they will turn off.

Operations dominates: convergence pays back on every PR from the first day, and the eval and tiers need it first to measure anything. Recommendation: point 1 option 1 and point 2 option 1 as the first story, then points 4, 5, 3 and 6 in that order, with point 7 deferred. What not to do: do not loosen approvals (point 6) before the escapes eval (point 3) and response rates (point 4) exist to show the loosening is safe. What you may not want to hear: the loop on #471 was mostly ours. aayu's own review request asked Codex again on every push, and the builder's standing rule was to fix every P1 and P2. What would change this counsel: if Codex's automatic review could not be limited, the cap would have to live only in the builder's skill, and option 2 of point 1 would be the honest choice. The question for the founder: accept the order and the first story, or reorder?

### Options

1. **Adopt points 1 to 6 as below, build points 1 and 2 first, and defer point 7 (recommended).** Each point lists its own numbered options, and the recommended choice is marked.
2. **Adopt only points 1 and 2 (convergence and Pre-existing) now.** Re-decide points 3 to 7 after 30 PRs under the new rules. This is cheaper now, but the reviewer has no recall measure while it is being made quieter.
3. **Rely on reviewer settings alone.** Turn off Codex's review on every push, ask the builder skill to stop after three rounds, and change no code. This costs nothing, but nothing is enforced, and the next app or builder repeats the loop.

### Evaluation

**What is already in aayu (checked, not changing here):** verification before review (proof packs and the floors in `CONSTRAINTS.md`); an AI review on every non-draft, non-docs PR (`internal/cli/inspect.go`, `DocsOnly`); peer refutation (`internal/inspect/peer.go`); suppression after human dismissals (`internal/inspect/filter.go`, `[inspect] suppress_after`); blocking security floors; protected paths (`.github/CODEOWNERS`) and never-layers (L04, L07, L09) needing a person; bot approvals not counting and approvals bound to the head commit (`identity.HumanPass`, `internal/cli/ship.go:260`); the dev → rolling → main staged release; halt and rollback; incidents leading to a postmortem and a Bug.

#### Point 1. Convergence

Evidence:
- **A cap exists but does not stop the loop.** `MaxCycles = 2` (`internal/inspect/inspect.go:38`) counts fix or rebuild dispositions on distinct diffs. At the cap, `Runner.begin` pages and returns `ErrCycles`. But `inspectThroughApp` posts the review request for each new head before it judges (`internal/cli/inspect_app.go`, `UpsertComment` before `Judge`), so aayu keeps asking the reviewer app after the cap. #471's state comment records `cycles: 2` with two diff hashes, beside 27 Codex reviews. aayudha/inspect is not a required check, so its failure blocked nothing.
- **There is no "step back".** At the cap inspect pages a person. It never tells the author to reconsider the approach.
- **There is no diff-only re-review.** The model path sends the whole diff on every run (`DiffHash` over `Input.Diff`), and the app request asks for a review of "this pull request at head".
- **Repeats are caught only in narrow cases.** Within one review, duplicates merge (`DropDuplicate`, `filter.go:33`), and the model path never reposts an inline finding ID (`State.Posted`, `internal/cli/inspect.go:482`). But `FindingID` hashes file, category and title (`filter.go:128`), so a reworded repeat in a later round gets a new ID and is posted again.
- **There is no class of bug.** The finding schema (`Schema`, `inspect.go`) has no field for the class or its other instances.
- **Rounds-to-pass is not tracked.** retro's flow section counts fix cycles (`audit.InspectFlow.Cycles`, `internal/audit/flow.go:184`), not rounds per PR, and not the app's own review rounds.

Options:
1. **Converge in the gate, enforced (recommended).**
   - A round is one reviewer reply on a distinct head. After round 3, inspect posts no new review request. Its disposition becomes `step-back`, which asks the builder to reconsider the approach. It is a fix disposition for the merge gate, and only a person's comment `/aayu review again` reopens review. This replaces today's silent `ErrCycles` stop.
   - From round 2, the request names the last reviewed head and asks for a review of `git diff <last>..<head>` only, plus any P0 anywhere.
   - The request lists every earlier finding and says not to repeat one. The gate drops a finding that matches an earlier round's finding on file and class, or by the existing word-similarity test (`jaccard` in `filter.go`), as `repeat`, counted, never posted. A protected P0 is never dropped as a repeat.
   - Each finding gets a `class` (for example "static methods treated as private") and `instances` (every file and line of that class in the PR), and the author is asked to fix every instance.
   - Rounds-to-pass per PR goes into the inspect state, retro, `status` and `audit`, beside recall.
   - The builder skills (`skills/build`, `skills/ship`) act only on findings in aayu's inspect result, never on raw app comments, within the existing 2-round fix cap for failing checks.
   - Cost: about 2 to 3 days of build. No model spend.
2. **The same rules, enforced only in the builder skill and the review request.** No gate change: the cap and diff-only scope are instructions. Cost: half a day. A builder or app that ignores the text loops again.
3. **Keep `MaxCycles = 2` and turn its page into the step-back.** This is the smallest code change. The cap stays at two, below the article's three.

The founder's step under any option: in Codex's repository settings, turn off review on every push and keep review when a PR opens or is marked ready, so aayu's request is the only trigger after the first review.

#### Point 2. A Pre-existing tag

Evidence: the system prompt asks the reviewer not to report "problems in code the pull request does not change" (`System`, `inspect.go`), and the filter drops findings in files outside the diff (`DropOutsideDiff`). A finding whose quote is not an added line gets `line: 0` and becomes summary-only, but it still counts towards the disposition. Nothing tags it, and nothing collects it into a Bug.

Options:
1. **The diff decides, and blame attributes (recommended).** A finding whose quoted line is not an added or changed line of the PR's diff against the merge base is tagged `pre-existing`. A Pre-existing finding never changes the disposition or blocks the PR. `git blame` at the merge base names the commit and PR that introduced the line. All of a PR's Pre-existing findings go into one Bug, filed through `aayu grill bug`, deduplicated against open Bugs by `FindingID`, and linked from the inspect comment. A protected P0 that is Pre-existing still does not block this PR, but its Bug gets `priority:P1` and pages. Cost: about 1 day, the same story as point 1.
2. **Blame is the rule.** A line counts as Pre-existing when blame at the merge base shows a commit older than the PR's first commit. It agrees with option 1 except on moved code, which blame credits to the move. Cost: one extra `git blame` per finding in the gate. The extra blame calls give no better answer.

#### Point 3. Bugs, incidents and reverts as the reviewer's eval

Evidence:
- The `review` eval group already scores inspect's precision and recall, with no judge, on 14 seeded-defect cases (`evals/cases/inspect/`; 4 are clean controls). It is advisory and runs on dispatch only (`evals/eval.toml`, group `review`). Run 39008726458 cost 1.66 USD for nine cases.
- Real escapes are few: 14 Bug issues, of which 7 are closed with a fix-forward (#170, #171, #172, #173, #177, #183, #417); 0 Incident issues; 0 revert commits in 407 commits on `dev`.
- **This eval can only score aayu's model reviewer**, called through the AI gateway in aayudha's own CI. It cannot call the Codex or Claude app, because the apps answer @-mentions on GitHub.

Options:
1. **A new advisory group, `review-escapes` (recommended).**
   - Case set: each closed Bug or Incident whose introducing PR can be named. The introducing PR's diff is the case, and the fix-forward is the ground truth: a finding on a line the fix changed counts as a hit. The engineer who fixed it (the twin, then the founder) confirms where the bug lived. Clean controls are merged PRs with no Bug within `[autonomy] correction_days`.
   - Scores: recall on the escapes, and a false-alarm rate on the controls, counting only a finding at P1 or above whose suggested fix would change correct behaviour.
   - Rescoring: an `evals.yml` path trigger on the reviewer's prompt (`System`, `defectRule`, the app request in `internal/inspect/apps.go`), its tools (`internal/inspect/context.go`), and the review lane's model in `internal/verbs/registry.go`; otherwise dispatch only.
   - Each new Bug that names an introducing PR becomes a candidate case.
   - Cost: about 0.18 USD per case per run (1.66 / 9). With about 7 escapes and 6 controls, that is about 2.40 USD a rescore. `max_usd = 5`. Building it takes about 2 days, and each case needs a person to confirm it.
2. **Add the escapes to the existing `review` group.** It is cheaper to run, but seeded and real recall mix into one number.
3. **Also score the apps passively.** Replay each case as a PR in the scratch repository and @-mention the app. It costs no API spend, but each run takes hours, and an app may answer differently each time. This is a later option, not part of this Decision.

#### Point 4. Measuring noise by response

Evidence: retro reports "inspect advice acted on" (`internal/audit/flow.go:409`): posted findings that are gone from the latest inspection count as acted on, beside dismissed and open at merge. It counts only findings aayu posted in the model path (`State.Posted`), so app findings are invisible. Dismissal is human-only (`inspect --dismiss`, `internal/cli/inspect.go:566`), and the ledger's `inspect/dismissed.jsonl` holds 0 entries. Nothing records "intentional" or "didn't matter", and `audit` and `status` show none of it.

Options:
1. **Record a response for every finding (recommended).**
   - Each finding takes one response: `fixed`, `wrong`, `intentional`, or `not-worth`. It comes from a first-line keyword in a reply on the finding's thread, or from `aayu inspect <pr> --respond <id> <kind> --reason`.
   - `fixed` is verified: the finding must be gone at a later head, or the response does not count.
   - A response by an agent is recorded with its author. Only a person's `wrong` feeds suppression, as today.
   - A finding with no response at merge counts as `unanswered`.
   - Per-PR rates go into `audit` and `status`, beside rounds-to-pass and the escapes eval's recall.
   - The priority is written into `docs/guide/how-merge-gate.md`: real bugs first, fewer rounds second. A change that lowers rounds is rejected when it lowers recall on the escapes eval.
   - Cost: about 2 days.
2. **Infer from outcomes only.** Gone means fixed, and open at merge means ignored. It is free, but it cannot tell `wrong` from `intentional`, and those are the noise.

#### Point 5. REVIEW.md

Evidence: there is no REVIEW.md in the repository. The review plan already builds areas by directory (`BuildContext`, `internal/inspect/context.go`), and generated files are skipped through `[inspect] generated`. Claude's code review reads a root REVIEW.md itself, so the Claude app (being installed) would use one with no aayu work.

Options:
1. **Path-local REVIEW.md files, nearest ancestor, protected (recommended).**
   - A root REVIEW.md, plus one in any directory whose owner wants one. Each area of the review plan loads its nearest REVIEW.md into the model prompt and the app request; it is owner text, since only an owner can merge a change to it.
   - A fixed short shape is linted (at most 40 lines): what failure looks like here, a nits cap, paths to skip, evidence as file and line, and a summary that opens with the count of findings.
   - The `REVIEW.md` pattern is added to `.github/CODEOWNERS`, making it a protected path. Each layer's owner seat in `LAYERS.md` writes theirs: the twin drafts, and the owner merges.
   - inspect drops a finding on a path the nearest REVIEW.md says to skip, as `review-md-skip`, and it checks that replies keep the nits cap.
   - Cost: about 1.5 days, plus each owner's text.
2. **One file per layer** (`.aayudha/review/L0N.md`), keyed by `LAYERS.md` paths. This maps cleanly to seats, but the apps do not read it natively, so aayu must paste it into every request.

#### Point 6. Risk tiers decide approvals

Evidence: `inspect.RiskOf` (`internal/inspect/risk.go`) computes the tier from the diff only.
- high: a never-layer footprint, a protected path, a dependency manifest, or a migration.
- medium: 3 or more layers, more than 400 changed lines, or more than 20 files.
- low: anything else.

It appears only on the proof comment (`prRisk`, `internal/cli/proof.go:443`) and "approves nothing and changes no gate". ship needs one human approval on the head (`identity.HumanPass`). The exception is an autonomous streak with rung A or B, which is still barred on never-layers and protected paths (`internal/cli/ship.go:274`). The pod has one human today (`aayu decide --pack`: "the pod's humans (1)").

Options:
1. **Tiers gate autonomy now, and two people later (recommended, strict start).**
   - low: today's rule (an autonomous streak with rung A or B may merge without a person).
   - medium: one person, always; autonomy cannot skip it.
   - high: one person now. Two distinct people automatically once the pod has two or more humans (`[team] size` or paired seats). In solo mode, the founder's approval is recorded with "high risk, single approver (solo)".
   - The tier comes only from `RiskOf`'s inputs (the diff, the layer footprint, CODEOWNERS), never from the PR body, labels or author. A person may raise a tier with a label, but never lower it.
   - The person directing an agent's build counts as one of the required approvers, as today.
   - Loosening any tier later needs a Decision citing points 3 and 4. Cost: about 1 day.
2. **Two people for high now.** This is impossible in a solo pod without a bypass on every high-risk PR, which would turn the bypass into routine.
3. **Keep the tier advisory.** It costs nothing. The tier stays decoration.

#### Point 7. A merge queue

Evidence: no workflow listens to `merge_group` (`.github/workflows/`), so there is no queue. A queue re-runs the required checks on the combined head, roughly doubling CI per PR while capacity is short.

Options:
1. **Defer, with a trigger (recommended).** Revisit when `dev` first goes red from two PRs that were each green, or when CI capacity is no longer the bottleneck. Cost now: nothing.
2. **Enable it now on `dev`.** Cost: `merge_group` triggers on the 8 required jobs, and about twice the CI minutes per PR.

#### What is not changing

- The different-family rule, the review ladder (Decision #469: the rungs, their labels, and that C never earns autonomy), and the rung B decision (the founder, 2026-10-18).
- "No AI keys in a product repo's CI" (founder, 2026-10-15). Rung B is its only, opt-in, exception, and the escapes eval runs only in aayudha's own CI.
- The deterministic floors and rule 7: no reviewer disposition overrides a floor.
- Protected paths, never-layers, bot approvals never counting, and approvals bound to the head commit.
- Peer refutation, dismissal suppression, the coverage seal, test-gap verdicts, and the existing `review` eval and its bars (no bar is lowered).
- The release path dev → rolling → main, halt and rollback.
- The reviewer app stays the user's choice (`[inspect] apps`, `[inspect] reviewer`).

### Recommendation

Option 1: points 1 to 6, each with its recommended choice (point 1 option 1, point 2 option 1, point 3 option 1, point 4 option 1, point 5 option 1, point 6 option 1), and point 7 deferred (option 1).

Build order once accepted:
1. Story: convergence and the Pre-existing tag (points 1 and 2). It is built as soon as this Decision is accepted, and not before.
2. Responses and rates (point 4).
3. REVIEW.md (point 5).
4. The escapes eval (point 3), dispatched under the founder's spend rules, with its cost reported.
5. Risk tiers in ship (point 6).
6. Point 7 when its trigger fires.

Decider line: the founder decides. The twins recommend; the Engineering Manager twin (Nestor) owns inspect and ship.

### Reversible

true
````
