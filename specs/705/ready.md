# rahulnakmol/agentic-ayudha#705: ship computes the autonomy streak even when the human pass is already settled (a settled pass makes none of its reads)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `7348e99eb3d42f740ae66f78eaf80ea09468344ef2ca1770bfa352ce1a2226d5` |
| Completion Goal hash | `705a422bba8cc48657e128a066aea066bdca32275e3ee438f46236b8ac858a84` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/705/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: aayu ship --solo-bypass on a Story PR with a green gate and the bypass comment; expected: no GET /repos/{owner}/{repo}/pulls?state=closed request is made, and ship.json's streak says it was not computed because the human pass was settled
2. [api-trace] state: aayu ship on a Story PR with no approving review and no bypass, below the streak; expected: the streak is computed (GET /repos/{owner}/{repo}/pulls?state=closed is made) and ship refuses for the human pass as before
3. [api-trace] state: aayu ship on a Story PR whose human pass is settled by an approving review of the head; expected: no GET /repos/{owner}/{repo}/pulls?state=closed request is made (none of the streak's reads), ship.json's streak says it was not computed because the human pass was settled, and the PR merges

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Replaces #701, whose criterion 3 asked for a timing that no test can produce; this Bug states it as an api-trace.

`aayu ship` computes the autonomy streak (`shipStreak` in `internal/cli/shippass.go`, called from `runShip` in `internal/cli/ship.go`) on every ship, before it reads the reviews. The streak reads the 100 newest merged PRs with each agent PR's files, reviews, commits, comments and merge-gate job logs (`autonomy.Fetch`). On this repository that takes about 2 minutes: `aayu status --streak`, which runs the same fetch, took 2 m 12 s on 2026-10-18. The streak only matters when no human pass is settled: it can give an autonomous pass. When the pass is already settled (`--solo-bypass`, an approving review of the head, or a merge on proof), it cannot change the outcome, and nothing reads it afterwards except the copy in `ship.json`.

Ship #693 from `aayu inbox` (solo bypass) posted its bypass comment at 23:41:05 UTC and pushed the pack at 23:43:28 UTC (actions segment `actions/2026-10-18/cli-20261018T233934Z.jsonl`). The ledger push itself took 2.3 s on its first attempt. The person saw a frozen inbox for about 2.5 minutes.

### Steps to reproduce

1. Open a Story PR whose merge gate is green, in a repository with about 100 merged PRs.
2. Run `aayu ship <pr> --solo-bypass` after the "Solo-mode bypass:" comment, or ship it from `aayu inbox` in solo mode.
3. Time it.

### Expected behaviour

Ship skips the streak when the human pass is already settled, and records in `ship.json` that it was not computed and why. It keeps computing the streak whenever no human pass is settled, because the streak can then merge the PR.

### Actual behaviour

Every ship spends about 2 minutes on the streak, whatever the human pass.

### Completion Goal

1. [api-trace] state: aayu ship --solo-bypass on a Story PR with a green gate and the bypass comment; expected: no GET /repos/{owner}/{repo}/pulls?state=closed request is made, and ship.json's streak says it was not computed because the human pass was settled
2. [api-trace] state: aayu ship on a Story PR with no approving review and no bypass, below the streak; expected: the streak is computed (GET /repos/{owner}/{repo}/pulls?state=closed is made) and ship refuses for the human pass as before
3. [api-trace] state: aayu ship on a Story PR whose human pass is settled by an approving review of the head; expected: no GET /repos/{owner}/{repo}/pulls?state=closed request is made (none of the streak's reads), ship.json's streak says it was not computed because the human pass was settled, and the PR merges

### Measure

Wall time of a solo-bypass ship on this repository.

### Threshold

Under 30 seconds.

### Scenario

A person ships a Ready card from `aayu inbox` in solo mode.

### Environment

`aayu ship` against GitHub, this repository.

### Technical Owner

engineering-manager

### Links

depends-on #693

### Checkpoints

1. A solo-bypass ship and a review-pass ship in the fake make no closed-PR list request and record why the streak was skipped; a ship below the streak still refuses after computing it (criteria 1, 2, 3)

### Assumptions

`internal/cli/ship.go` and `internal/cli/shippass.go` are in #593's narrow set, so the fix PR waits for a person's inbox pass.

### Open questions

deferred: should a merge on proof also skip the streak? (it already sets the pass before the streak matters; can wait)

### Parked ideas

Cache the streak between ships in one inbox session.

### Provenance

Request: "Reword it as an api-trace that a ship with a settled pass makes none of the streak's reads"
1. stated (behaviour): "skip the streak when the human pass is already settled (solo bypass or an approving review)"
2. stated (behaviour): "Keep it on every path where it can change the outcome"
3. stated (behaviour): "a ship with a settled pass makes none of the streak's reads"
````
