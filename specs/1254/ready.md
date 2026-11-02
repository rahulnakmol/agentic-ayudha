# rahulnakmol/agentic-ayudha#1254: Two person windows a day: outside them no work waits for a person and nobody is asked to act

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `78689b0cc892aee22e74e02c4abe83721add038e188a5ab6737751b18082858a` |
| Completion Goal hash | `39aa091505baf8ce09aeee6e6988692f0d204825ae461de8d4248efa96cf018c` |
| Approved to build | #1170 approval 3 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1254/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a repository with no `[time] windows`; expected: `aayu config show` lists two windows, 10:30-11:30 and 18:30-19:30, in `[time] tz`, and this repository's `.aayudha/config.toml` sets `[time] tz = "Asia/Kolkata"`
2. [api-trace] state: a mission passes `[mission] max_hours` outside a window; expected: `aayu mission next` still lists its startable stories, and the time-limit card waits for the next window
3. [api-trace] state: a mission passes `[mission] max_hours`, and its card stays unanswered to the end of a window; expected: the mission holds new starts from the end of that window, as today, and running stories go on
4. [api-trace] state: a mission's own budget is used up outside a window; expected: mission next still lists its startable stories, and a budget card waits for the next window; the `[budget]` breaker pause (WP1.6) still stops every agent lane, as today
5. [api-trace] state: any aayu command asks a person to act (a notify, a comment-approval batch, an inbox card) outside a window; expected: nothing is sent then, and at the next window's start one message lists every queued item
6. [api-trace] state: inside a window; expected: `aayu inbox` holds every item that needs a person (baselines, Approve to build, mission start, time and budget cards, narrow-set ships, inspect approvals, P0 and P1 findings) in decision order, with one device code for the session
7. [api-trace] state: the day plan and the evening review; expected: they name the windows in `[time] tz`, not the end of `[time] business_hours`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want two person windows a day, set in `[time] windows` (default 10:30-11:30 and 18:30-19:30 in `[time] tz`), so that work never waits for me and nobody asks me to act outside them, and every item that needs me is resolved in one inbox session in each window.

### Completion Goal

1. [api-trace] state: a repository with no `[time] windows`; expected: `aayu config show` lists two windows, 10:30-11:30 and 18:30-19:30, in `[time] tz`, and this repository's `.aayudha/config.toml` sets `[time] tz = "Asia/Kolkata"`
2. [api-trace] state: a mission passes `[mission] max_hours` outside a window; expected: `aayu mission next` still lists its startable stories, and the time-limit card waits for the next window
3. [api-trace] state: a mission passes `[mission] max_hours`, and its card stays unanswered to the end of a window; expected: the mission holds new starts from the end of that window, as today, and running stories go on
4. [api-trace] state: a mission's own budget is used up outside a window; expected: mission next still lists its startable stories, and a budget card waits for the next window; the `[budget]` breaker pause (WP1.6) still stops every agent lane, as today
5. [api-trace] state: any aayu command asks a person to act (a notify, a comment-approval batch, an inbox card) outside a window; expected: nothing is sent then, and at the next window's start one message lists every queued item
6. [api-trace] state: inside a window; expected: `aayu inbox` holds every item that needs a person (baselines, Approve to build, mission start, time and budget cards, narrow-set ships, inspect approvals, P0 and P1 findings) in decision order, with one device code for the session
7. [api-trace] state: the day plan and the evening review; expected: they name the windows in `[time] tz`, not the end of `[time] business_hours`

### Measure

Completion Goal criteria observed

### Threshold

all 7 criteria

### Scenario

Move a fake clock through a day: before, inside, and after each window, with a mission past its time limit, a used-up mission budget, and a card that asks a person to act.

### Environment

go test of internal/cli and internal/config with a fake GitHub, a fake ledger, and a fake clock; never production

### Checkpoints

1. The windows are configured, with the IST default for this repository (criteria 1, 7)
2. Outside a window, time limits and the mission budget do not stop starts (criteria 2, 3, 4)
3. Requests for action wait for a window, and the window's inbox holds them all (criteria 5, 6)

### Assumptions

#### Notes

Today a mission past `[mission] max_hours` starts no new story until a person extends it (internal/cli/missiontime.go:35, internal/cli/missionnext.go:356). The `[budget]` breaker pauses every agent lane (internal/cli/missionnext.go:85). The repository sets no `[time]`, so the day plan uses the default `Europe/London` and `Mon-Fri 09:00-18:00` (internal/config/defaults.toml:212, internal/cli/inboxday.go:19). Criterion 3 keeps a person's control of a long mission: a hold starts only after a window in which the card went unanswered. Criterion 4 keeps the gateway's spend breaker: it is money, not a decision card. This change touches internal/config/, which is in the narrow set, so its PR needs a person's ship in a window.

#### Review guidance

- Check: criterion 3: a hold starts at a window's end, never between windows.
- Check: criterion 4: the `[budget]` breaker still stops every lane.
- Ignore: the wording of the queued message.
- Risk: a request for action that is lost between windows; criterion 5 lists every queued item.

#### Behaviours

1. says: Outside a window, no work waits for a person; rule: never; source: criteria 2, 4
2. says: Nobody is asked to act outside a window; rule: never; source: criteria 5
3. says: Every item that needs a person is in the window's inbox; rule: limit; source: criteria 6

### Provenance

Request: "there should be no blocker to continute work or push notifications to ask user to tkae action from 11:30AM to 6:30PM and 7:30PM to next day 10:30AM. user has only two windows wich is configurable default is 1 hr window in moring and evening and all that needs actiion shoudl be resolved"
Term: "30AM" → minute 30 of a morning hour in IST, as in 10:30 and 11:30; "11:30AM to 6:30PM and 7:30PM to next day 10:30AM"
Term: "30PM" → minute 30 of an afternoon or evening hour in IST, as in 18:30 and 19:30; "11:30AM to 6:30PM and 7:30PM to next day 10:30AM"
1. stated (request): two configurable windows, 1 hour each by default, morning and evening
2. stated (request): no blocker to continue work outside the windows
3. default (scope): a hold starts only after an unanswered window
4. default (scope): the spend breaker stays
5. stated (request): no notifications asking for action outside the windows
6. stated (request): everything that needs action is resolved in the window
7. default (scope): the day plan and the evening review use the windows
````
