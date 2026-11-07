# Goal and intent: #897 metered, risk-routed merge

No intent baseline exists (no `specs/897/` on `origin/ledger`, 2026-11-07). The sources are
[#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) and Decisions
[#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902),
[#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910), and
[#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"Every merged pull request leaves one record that the pull request cannot write, and review depth follows a risk tier
that the binary computes from the diff." (#897, Summary.) This is "D2 and D3 of the Vercel and Warp research, agreed by
@rahulnakmol on 2026-10-21 for v0.4.0" (issue text; the research is a private note, unverified here).

## Who it serves

The founder, who needs cost, model, and rounds per pull request to steer the pod (#897, Problem), and #900, whose
routing evidence reads these records (#900 depends-on #897).

## Problem

From #897, Problem. The numbers come from the private Vercel fit-gap note (F4) and the review-speed analysis:

- "The ledger records 37 of 528 merges into dev, and 117 of 528 pull request bodies name a host". Unverified here.
- "Fix rounds take 68% of the hours a pull request is open, and 44% of rounds ran after inspect said merge". #910 gives
  the base: 506 of 741 open hours, and 75 of 169 rounds, for 2026-10-11 to 2026-10-21.
- "Today's tier rule puts about 54% of pull requests in high risk, because `skills/` and `seats/` are protected paths
  (`internal/inspect/risk.go:45`)." The rule is true on `dev`: a CODEOWNERS-protected path makes a PR high
  (`RiskOf`), and `/skills/` and `/seats/` are in `.github/CODEOWNERS`. The 54 % is unverified here.

## How later decisions change the Scope

| Scope item in #897 | Later decision | Effect |
|---|---|---|
| Tier rules (high, medium, low) | #902, done 2026-10-23, Option 1 | Same tiers as #897, plus: the threat-model paths and prompt files are config lists (`[inspect.risk] high_paths`, `prompt_paths`); medium also covers `evals/`; the 600 lines exclude tests and generated files. |
| "The low-risk path: … one review from any configured reviewer …; at most 3 rounds; a later finding is filed as a Bug unless it is P0 or P1." | #910, done 2026-10-23, Option 1 | "Review on every tier … There is no path that skips the review for low risk." One configured reviewer and the 3-round rule apply on every tier, so the "low-risk path" is no longer a separate path. |
| High = "the `always_person` set" | #1255, done; Story #1281 (open); draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282), closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept (`_data/closed-prs.tsv`) | Every agent PR merges on proof at any risk; only `always_person_config` and secret or environment workflows stay with a person. The high tier still exists, but what it gates changes. |

## Success measures

From #897 Scope: the tier shows with its reason; one JSON line per merged PR; a nightly back-fill where a missing
field stays a gap; the factory meter on Initiative #870 with each number's source, unit, and age; a rollout that is
silent for 2 weeks, then visible, then enforced, with a kill switch.

## Out of scope

"Skipping floors or checks on any tier; an LLM classifier as a gate." (#897, Scope Out.)
