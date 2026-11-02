# rahulnakmol/agentic-ayudha#1268: A milestone's gate failures from before its last pass still count toward max_no_go, so one new failure stops the mission

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `2f6653098836c4e00b5863fa38411f309c52f732b6a23e1049b5e0adafa72c9a` |
| Completion Goal hash | `30628eb6c18165d5e5b041e2fb6ec52c23eb4cc8381749afade0cd0c808ea638` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1268/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a milestone with two failed runs, then a pass, then one failed run; expected: the gate records failure 1 of `max_no_go`, the mission is not stopped, and the gate lists one fix for each failed check
2. [api-trace] state: a milestone with `max_no_go` failed runs since its last pass, or since the start when it never passed; expected: the mission stops as today, and the stop says the count since the last pass
3. [api-trace] state: a stopped milestone, and a person's `--once-more` that passes; expected: the mission resumes as today, and a later failure counts from 1

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A milestone's failed gate runs from before its last pass still count toward `max_no_go`. So one new failure after a pass can stop the whole mission. `missionGate` sets the stop from `len(gateFailures(runs, o.m))` (internal/cli/missiongate.go:349), and `gateFailures` counts every failed run of the milestone in the gate log (internal/cli/missiongate.go:912). It never stops at the last pass.

On 2026-11-02:
- MS1 of #1170 failed twice, then passed on dev b83e187 at 04:05Z.
- That evening, Approve to build again reset its gate to waits (Bug #1267).
- The next gate run, on dev ab8ab07, failed once, on eval:retro, with 1 run in 5 at score 3 in two cases.
- That one failure made the count 3. The mission stopped: "MS1 of #1170 failed its gate 3 times: its stories wait". `aayu mission next 1170` says "Nothing starts", and only a person's `aayu mission gate 1170 --once-more` resumes it.

### Steps to reproduce

1. A milestone fails its gate twice, then passes.
2. Its gate runs again later and fails once.
3. Run `aayu mission show <f>` and `aayu mission next <f>`.

### Expected behaviour

The count of failed gate runs starts again after a pass. One failure after a pass is failure 1 of `max_no_go`. The gate lists a fix and asks no person.

### Actual behaviour

The failures from before the pass count, so the first failure after the pass stops the mission.

### Completion Goal

1. [api-trace] state: a milestone with two failed runs, then a pass, then one failed run; expected: the gate records failure 1 of `max_no_go`, the mission is not stopped, and the gate lists one fix for each failed check
2. [api-trace] state: a milestone with `max_no_go` failed runs since its last pass, or since the start when it never passed; expected: the mission stops as today, and the stop says the count since the last pass
3. [api-trace] state: a stopped milestone, and a person's `--once-more` that passes; expected: the mission resumes as today, and a later failure counts from 1

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Write gate logs for a milestone with fails before and after a pass, and run the gate against a fake GitHub.

### Environment

go test of internal/cli with a fake GitHub and a fake ledger; never production

### Checkpoints

1. The count starts again after a pass (criteria 1, 3)
2. A real run of failures still stops the mission (criteria 2)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

Bug #1267 stops Approve to build again from resetting a pass. This Bug is the other half: even with #1267 fixed, a passed milestone that fails later inherits its old failures. Both stop a running mission, so both stay in v0.4.0. The Kanha started a gate fix for MS1 in thread T-01a19e18-6c8d-76f3-8bf7-88fdcbce08f7.

#### Review guidance

- Check: criterion 2: a milestone that never passed still stops after `max_no_go` failures.
- Ignore: the wording of the stop line.
- Risk: a flaky gate that never stops because each pass resets the count; that is the intent, because a pass shows the milestone can pass.

#### Behaviours

1. says: A milestone's failures count from its last pass; rule: limit; source: criteria 1, 3
2. says: max_no_go failures in a row still stop the mission; rule: never; source: criteria 2

### Provenance

Request: "there should be no blocker to continute work or push notifications to ask user to tkae action from 11:30AM to 6:30PM and 7:30PM to next day 10:30AM. user has only two windows wich is configurable default is 1 hr window in moring and evening and all that needs actiion shoudl be resolved"
Term: "30AM" → minute 30 of a morning hour in IST, as in 10:30 and 11:30; "11:30AM to 6:30PM and 7:30PM to next day 10:30AM"
Term: "30PM" → minute 30 of an afternoon or evening hour in IST, as in 18:30 and 19:30; "11:30AM to 6:30PM and 7:30PM to next day 10:30AM"
1. default (scope): count failures from the last pass
2. default (scope): a real run of failures still stops the mission
3. default (scope): a once-more pass resets the count
````
