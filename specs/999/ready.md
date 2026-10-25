# rahulnakmol/agentic-ayudha#999: The runnable list holds back a story whose depends-on target is still open

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `b42dac58454b8c2fc8f01ddad557c58ab7f3eaa9ef62d08ec1b3aaec332eced6` |
| Completion Goal hash | `2204da36c54d7093cf13fced627a10d6d494edef9d0fbb564d1720c3661aef69` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/999/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Ready story #11 of #889 has `depends-on #10` in its Links, and #10 is open with status:in-progress; expected: `aayu mission next 889` prints `waits #11: depends-on #10 is open`
2. [api-trace] state: the same Feature after #10 closes with status:merged; expected: the list marks #11 start
3. [api-trace] state: #11 depends on #10 (open) and #12 (closed), and Ready story #13 with no link is later in the order; expected: the list holds #11 and marks #13 start

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want a story to wait while an issue it depends on is open, so that a merge never starts a story too early.

### Completion Goal

1. [api-trace] state: Ready story #11 of #889 has `depends-on #10` in its Links, and #10 is open with status:in-progress; expected: `aayu mission next 889` prints `waits #11: depends-on #10 is open`
2. [api-trace] state: the same Feature after #10 closes with status:merged; expected: the list marks #11 start
3. [api-trace] state: #11 depends on #10 (open) and #12 (closed), and Ready story #13 with no link is later in the order; expected: the list holds #11 and marks #13 start

### Measure

Starts of a story that had an open depends-on target, per 28 days.

### Threshold

0

### Scenario

Story B needs a field that story A adds. A merges at 14:02. B starts after that, never before.

### Environment

go test of internal/cli with a fake GitHub and seeded issues; never production.

### Links

depends-on #998

### Assumptions

#### Notes

This rule moves here from #885 (core design v12).

#### Review guidance

check: The link comes from the issue body that Ready froze (`ready.Check`), not from a PR body. Criterion 3 shows that a hold does not stop the list.
ignore: The wording of the waits line.
risk: A link form that the parser misses lets a story start early. A target in another repository counts as open.

#### Behaviours

1. [never] says: A story never starts while an issue it depends on is open; source: F19 (core design v13); Decision #915 (M1), F3; criteria: 1, 2
````
