# rahulnakmol/agentic-ayudha#1007: ship tells the story's Kanha after each merge, in the same process

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `c3aba8abe311bf5450751f23c1b6212540a65942174de2538eeeff252f46d533` |
| Completion Goal hash | `d912f5036ebc5cc21a212a2c8b106cfea2a6b56191d2f155d5d28e651968d403` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1007/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ship 41` merges story #40's PR, and #40 belongs to #889; expected: after the merge lines ship sends the Kanha of #889 one message, `merged #40 (PR #41): run aayu mission next 889`, and prints `next: told the Kanha of #889`
2. [api-trace] state: the message to the Kanha fails; expected: ship exits 0 with the merge, the Complete section, and status:merged in place, and prints `next: the Kanha of #889 was not reached: <reason>; the safety tick wakes it`
3. [api-trace] state: `aayu ship 41` refuses because inspect says fix; expected: ship sends nothing

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want each merge through ship to wake the Feature's Kanha at once, so that the next story starts with no wait.

### Completion Goal

1. [api-trace] state: `aayu ship 41` merges story #40's PR, and #40 belongs to #889; expected: after the merge lines ship sends the Kanha of #889 one message, `merged #40 (PR #41): run aayu mission next 889`, and prints `next: told the Kanha of #889`
2. [api-trace] state: the message to the Kanha fails; expected: ship exits 0 with the merge, the Complete section, and status:merged in place, and prints `next: the Kanha of #889 was not reached: <reason>; the safety tick wakes it`
3. [api-trace] state: `aayu ship 41` refuses because inspect says fix; expected: ship sends nothing

### Measure

Minutes from a merge through ship to the next story's start, p50, when a story is runnable.

### Threshold

1 or less (today up to 120).

### Scenario

A story merges at 14:02. The Kanha starts the next one at 14:03.

### Environment

go test of internal/cli (TestShip* style) with a fake GitHub and a stub adapter; never production.

### Links

depends-on #1004

### Assumptions

#### Notes

ship.go is in `[autonomy] always_person`, so @rahulnakmol ships this PR. The wake uses the existing `Adapter.FollowUp` (receivers.go:48-52).

#### Review guidance

check: The message goes only after the merge, the ship record, and the actions checkpoint (ship.go:547-605). A failed message never changes ship's exit code.
ignore: The text of the `next:` line.
risk: ship.go is in the narrow set (.aayudha/config.toml:77). An early return can skip the lease release or the worktree cleanup.

#### Behaviours

1. [capability] says: A merge through ship wakes the Feature's Kanha at once; source: D1 (factory common view); Decision #915 (M1), F6; criteria: 1
2. [never] says: A failed wake never undoes or hides a merge; source: ship.go:547-605; criteria: 2
````
