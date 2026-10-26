# rahulnakmol/agentic-ayudha#1008: A stop, a block, a failed start, or an answered ask tells the Kanha

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `651c5ef0e2aafbd654d882fe6f6eded903ee9c562e7adbdaef22bc58382d84ee` |
| Completion Goal hash | `c5ea809f3689d93ab2430d6b15f95421ea0afaf0dfa8afb34f48ff4fd5e2b251` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1008/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu build 30 --stop "spec unclear"` on a story of #889; expected: #30 is status:blocked, and the Kanha of #889 gets `blocked #30: spec unclear: run aayu mission next 889`
2. [api-trace] state: `aayu build 32 --delegate amp` fails to start the subthread (the CLI exits 1); expected: the claim line, the lease, and status:in-progress are released, #32 is status:ready, and the Kanha gets `start of #32 failed: <reason>`
3. [api-trace] state: an ask from story #33's subthread is answered; expected: #33 keeps its slot, and the Kanha gets `answered: #33 resumes`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want every event that changes a story's slot to reach its Kanha, so that a stopped story never leaves an agent idle.

### Completion Goal

1. [api-trace] state: `aayu build 30 --stop "spec unclear"` on a story of #889; expected: #30 is status:blocked, and the Kanha of #889 gets `blocked #30: spec unclear: run aayu mission next 889`
2. [api-trace] state: `aayu build 32 --delegate amp` fails to start the subthread (the CLI exits 1); expected: the claim line, the lease, and status:in-progress are released, #32 is status:ready, and the Kanha gets `start of #32 failed: <reason>`
3. [api-trace] state: an ask from story #33's subthread is answered; expected: #33 keeps its slot, and the Kanha gets `answered: #33 resumes`

### Measure

Agents idle for more than one hour while a story was runnable, per 28 days.

### Threshold

0

### Scenario

A build stops at 11:20 because its spec is unclear. The Kanha starts the next story at 11:21.

### Environment

go test of internal/cli and internal/delegate with a fake GitHub and a stub adapter that fails; never production.

### Links

depends-on #1004

### Assumptions

#### Notes

Depends on, outside this slice: #882.

The answered-ask event needs #882.

#### Review guidance

check: A failed start leaves no claim that the list still counts. A waiting story keeps its slot (#882 Q2, decided).
ignore: The order of the output lines.
risk: A start that fails on every story can loop; the Kanha tries a failed start again only at the next safety tick.

#### Behaviours

1. [capability] says: Every event that changes a story's slot reaches its Kanha; source: Decision #915 (M1), F3; criteria: 1, 3
2. [never] says: A failed start never holds a slot; source: core design R8; buildnext.go:417-492; criteria: 2
````
