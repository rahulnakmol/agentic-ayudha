# rahulnakmol/agentic-ayudha#1001: A milestone gate checks the dev tip, and each failed check gets one pre-authorised fix PR

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `0817514b6e9295ae06ab5673852f8eda090b9bc2cdde5e0e32ca721b069b520f` |
| Completion Goal hash | `f563727fc513fc88513b6ea2912d5feda97ea638db93a9b257a68e50559d435f` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1001/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the last story of milestone MS1 of #889 merges; expected: aayu runs the gate on the dev tip: the required checks, `aayu verify --check all`, and the evals of the seats the milestone touched, with no model deciding pass or fail
2. [api-trace] state: the gate passes; expected: missions/889/plan.json marks MS1 passed with the dev SHA, and PRs of MS2 may now merge (ST16)
3. [api-trace] state: the gate fails on `aayu verify --check all` (check `inbox.ship-once`); expected: the Kanha starts one fix subthread for that check, which opens one fix PR with `Refs #889` and files no issue; the PR is in MS1, so MS1's gate runs again when it merges
4. [api-trace] state: that fix PR #981 opens; expected: missions/889/plan.json gains one `gate_fixes` entry `{pr: 981, milestone: "MS1", check: "verify:inbox.ship-once", dev_sha: "9f8e7d6…", approval: {record: "specs/889/approved.json", sha256: "<hash>"}, at: "<RFC 3339>"}`, and no issue and no triage record is written
5. [api-trace] state: the gate fails on two checks, `verify:inbox.ship-once` and the eval `mission/stays-in-list`; expected: two fix subthreads open two fix PRs, one for each check, each with its own `gate_fixes` entry
6. [api-trace] state: fix PR #981 spent 14 USD, #889 has 40 USD of its budget left, and fix PR #982 is estimated at 64 USD; expected: `aayu mission show 889` counts the 14 USD in #889's budget used; a fix that would pass the budget waits, and one inbox card `Fix PR #982 would pass #889's budget` shows the spend and the fix's estimate and offers `Raise the budget to $720, with a reason` (presence check) and `Keep #982 waiting`
7. [api-trace] state: MS1's gate is failed, and stories of MS2 are startable; expected: `aayu mission next 889` still starts them, and their PRs wait to merge (ST16)
8. [api-trace] state: MS1's gate fails for the third time (`[mission] max_no_go = 3`); expected: the mission stops, and one inbox card `MS1 of #889 failed its gate 3 times: its stories wait` says `The gate of MS1 failed 3 times, so the mission stopped and asks you.`, lists each run with its check and time, gives no recommendation, and offers `Run the gate once more` and `Stop the mission, with a reason`
9. [api-trace] state: a required check failed with a runner error (cancelled, or a 5xx from GitHub); expected: aayu reruns that check once and records why; a second failure counts as a failed gate

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want each milestone proven on the integrated dev tip, so that each merged milestone is a safe checkpoint.

### Completion Goal

1. [api-trace] state: the last story of milestone MS1 of #889 merges; expected: aayu runs the gate on the dev tip: the required checks, `aayu verify --check all`, and the evals of the seats the milestone touched, with no model deciding pass or fail
2. [api-trace] state: the gate passes; expected: missions/889/plan.json marks MS1 passed with the dev SHA, and PRs of MS2 may now merge (ST16)
3. [api-trace] state: the gate fails on `aayu verify --check all` (check `inbox.ship-once`); expected: the Kanha starts one fix subthread for that check, which opens one fix PR with `Refs #889` and files no issue; the PR is in MS1, so MS1's gate runs again when it merges
4. [api-trace] state: that fix PR #981 opens; expected: missions/889/plan.json gains one `gate_fixes` entry `{pr: 981, milestone: "MS1", check: "verify:inbox.ship-once", dev_sha: "9f8e7d6…", approval: {record: "specs/889/approved.json", sha256: "<hash>"}, at: "<RFC 3339>"}`, and no issue and no triage record is written
5. [api-trace] state: the gate fails on two checks, `verify:inbox.ship-once` and the eval `mission/stays-in-list`; expected: two fix subthreads open two fix PRs, one for each check, each with its own `gate_fixes` entry
6. [api-trace] state: fix PR #981 spent 14 USD, #889 has 40 USD of its budget left, and fix PR #982 is estimated at 64 USD; expected: `aayu mission show 889` counts the 14 USD in #889's budget used; a fix that would pass the budget waits, and one inbox card `Fix PR #982 would pass #889's budget` shows the spend and the fix's estimate and offers `Raise the budget to $720, with a reason` (presence check) and `Keep #982 waiting`
7. [api-trace] state: MS1's gate is failed, and stories of MS2 are startable; expected: `aayu mission next 889` still starts them, and their PRs wait to merge (ST16)
8. [api-trace] state: MS1's gate fails for the third time (`[mission] max_no_go = 3`); expected: the mission stops, and one inbox card `MS1 of #889 failed its gate 3 times: its stories wait` says `The gate of MS1 failed 3 times, so the mission stopped and asks you.`, lists each run with its check and time, gives no recommendation, and offers `Run the gate once more` and `Stop the mission, with a reason`
9. [api-trace] state: a required check failed with a runner error (cancelled, or a 5xx from GitHub); expected: aayu reruns that check once and records why; a second failure counts as a failed gate

### Measure

Milestones marked passed with a failing gate check, per 28 days.

### Threshold

0

### Scenario

Three stories that change a contract merge. The gate finds one eval below its bar and opens a fix PR. The next milestone keeps building, and its PRs wait until the fix merges and the gate passes.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and stub verify and eval runners; never production.

### Links

depends-on #997
depends-on #998

### Assumptions

#### Notes

Depends on, outside this slice: #915, #974.

The merge order between milestones (M9) is ST16's rule. Decision #974 (amends #915): gate fixes are PRs, not Bugs. `gate_fixes` keeps its ledger path and fields, with `pr` in place of `bug`, so #877's gate-fix-Bug criterion and #879 S4's acceptance have no Bug to read. ST16 makes ship merge a listed fix PR on proof. A fix subthread takes one `[build] agents` slot. `[mission] max_no_go` is defined in ST6.

#### Review guidance

check: No model decides a gate. A fix PR comes only from a failed gate check (Decision #974 (amends #915)); every Bug still follows #861 and #903.
ignore: The text of the fix PR's title.
risk: Fix PRs could grow past the Feature's budget: they count against it, and a fix that would pass it waits for him. A flaky check can open a fix PR that is not needed; criterion 9 reruns a runner error once.

#### Behaviours

1. [order] says: A failed gate never stops other stories from building; only merges of later milestones wait; source: Decision #915 (M1); M9 answer A; criteria: 2, 3, 7
2. [authority] says: The gate records each fix PR with its failed check and the plan's approval, and files no issue; source: Decision #974 (amends #915); criteria: 4
3. [limit] says: One failed check gets exactly one fix PR, and its cost counts against the Feature's budget; source: Decision #974 (amends #915); criteria: 5, 6
4. [never] says: No model decides whether a gate passes; source: Decision #915 (M2); criteria: 1
````
