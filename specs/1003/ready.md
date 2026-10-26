# rahulnakmol/agentic-ayudha#1003: aayu mission recover re-attaches what lives, and a failed step retries up to retry_max

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `0a23f7aece953279fbce7be4ac5fc87363e396e789e4b7753026c248e2826d6a` |
| Completion Goal hash | `e2c7ed6843df11c2b6c433de5375f278feae180282d66e6c35809cc23a86819b` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1003/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the Kanha's orb of #889 was lost; the record shows 3 running stories: #60 has a live Amp thread, #61 has a PR, and #62's Cursor agent is ARCHIVED with no PR; expected: `aayu mission recover 889` re-attaches #60, keeps #61 on its PR, restarts #62 from its branch and brief, and prints one line per story
2. [api-trace] state: recover runs twice in a row; expected: the second run changes nothing, and no story has two live agents
3. [api-trace] state: a Codex task list cannot be read (401); expected: recover marks those stories `unreadable: codex task list (401)`, keeps that source's cursor, starts nothing for them, and `aayu mission show 889` says `unreadable`, not `quiet`
4. [api-trace] state: the start of #64 sends its request, the host gives no answer before the timeout, and a codex task for #64 exists with no PR yet; expected: the record marks #64 `maybe started`, `aayu mission recover 889` prints `#64  maybe started: a codex task exists for #64 but no PR yet; aayu does not start it again`, and the next wake checks the host and GitHub for #64 before it starts it again
5. [api-trace] state: the start of #63 fails with a host error; expected: aayu retries after `[mission] retry_backoff_minutes` (10), then after 20; after `[mission] retry_max` (2) retries it waits on @rahulnakmol with the errors listed
6. [api-trace] state: `aayu mission retry 889 --story 63` after that, and `aayu mission retry 889 --story 941` for a failed Bug; expected: one more attempt each, counted and costed in the record
7. [api-trace] state: a retry of a story whose PR had a judged inspect review on its head; expected: the retry does not run that review again
8. [api-trace] state: @carol, on cursor, runs `aayu mission recover 874` or `aayu mission retry 874 --story 950`, and #874's recorded tool is codex; expected: each is refused with `#874's mission runs on codex, the tool of @bob, who started it: run this there`, and the record does not change
9. [api-trace] state: @dana, on codex, runs `aayu mission retry 874 --story 950` on @bob's mission #874; expected: the retry runs, and `controls` in missions/874/plan.json names `retry`, @dana, codex, and the time

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want a mission to come back from any crash with no story started twice, so that a lost orb or a host error costs minutes, not a day.

### Completion Goal

1. [api-trace] state: the Kanha's orb of #889 was lost; the record shows 3 running stories: #60 has a live Amp thread, #61 has a PR, and #62's Cursor agent is ARCHIVED with no PR; expected: `aayu mission recover 889` re-attaches #60, keeps #61 on its PR, restarts #62 from its branch and brief, and prints one line per story
2. [api-trace] state: recover runs twice in a row; expected: the second run changes nothing, and no story has two live agents
3. [api-trace] state: a Codex task list cannot be read (401); expected: recover marks those stories `unreadable: codex task list (401)`, keeps that source's cursor, starts nothing for them, and `aayu mission show 889` says `unreadable`, not `quiet`
4. [api-trace] state: the start of #64 sends its request, the host gives no answer before the timeout, and a codex task for #64 exists with no PR yet; expected: the record marks #64 `maybe started`, `aayu mission recover 889` prints `#64  maybe started: a codex task exists for #64 but no PR yet; aayu does not start it again`, and the next wake checks the host and GitHub for #64 before it starts it again
5. [api-trace] state: the start of #63 fails with a host error; expected: aayu retries after `[mission] retry_backoff_minutes` (10), then after 20; after `[mission] retry_max` (2) retries it waits on @rahulnakmol with the errors listed
6. [api-trace] state: `aayu mission retry 889 --story 63` after that, and `aayu mission retry 889 --story 941` for a failed Bug; expected: one more attempt each, counted and costed in the record
7. [api-trace] state: a retry of a story whose PR had a judged inspect review on its head; expected: the retry does not run that review again
8. [api-trace] state: @carol, on cursor, runs `aayu mission recover 874` or `aayu mission retry 874 --story 950`, and #874's recorded tool is codex; expected: each is refused with `#874's mission runs on codex, the tool of @bob, who started it: run this there`, and the record does not change
9. [api-trace] state: @dana, on codex, runs `aayu mission retry 874 --story 950` on @bob's mission #874; expected: the retry runs, and `controls` in missions/874/plan.json names `retry`, @dana, codex, and the time

### Measure

Stories with two live agents at once, per 28 days.

### Threshold

0

### Scenario

An orb pauses for a night. In the morning recover finds two live threads and one lost one, and restarts only the lost one.

### Environment

go test of internal/cli and internal/delegate with a fake ledger and stub adapters that fail; never production.

### Links

depends-on #997
depends-on #1002

### Assumptions

#### Notes

Recover also runs on every Kanha wake (ST4). `[mission] retry_max` and `retry_backoff_minutes` are defined in ST6.

#### Review guidance

check: Recover reads the record and each host; it never trusts the Kanha's memory. A retry never reruns a judged review (AGENTS.md, Code review: do not rerun inspect for a different answer).
ignore: The order of the recover lines.
risk: A recover that misreads a host starts a story twice; criterion 2 and the claim line guard it.

#### Behaviours

1. [never] says: One story never has two live agents; source: Decision #915 (controls); criteria: 1, 2
2. [limit] says: A failed step retries at most retry_max times, then waits for a person; source: Decision #915 (controls); criteria: 5
3. [never] says: A step counts as done only when GitHub or the ledger confirms it; source: J4 rules 2 and 4 (mission-ux page, section 5); criteria: 3, 4
4. [authority] says: Recover and retry run only in the mission's recorded tool; source: R9 per person, @rahulnakmol 2026-10-23; Tatkal R9 (#877); criteria: 8, 9
````
