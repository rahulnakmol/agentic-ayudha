# rahulnakmol/agentic-ayudha#631: inspect falls back to rung B through the gateway, then a person, when Codex cannot review

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `a836b4058d26e656a31fedfa4f2bb6d3be3eb0f3509831e76e55a10aa49447c3` |
| Completion Goal hash | `c121bd6f3122381ffe871aca7cd2cbf3c8e8c64a5f49e1c261b1263e9cd7bff2` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/631/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu inspect <pr> --ci` on a Claude-built PR whose only reviewer app, Codex, replied "You have reached your Codex usage limits for code reviews", with `[inspect] paid_reviewer = "grok-4.7"` and its key; expected: grok-4.7 reviews at high effort, the disposition is the review's, the review is labelled `different-family review (B, paid)`, and the inspect comment says the reviewer app cannot review the head and quotes the usage-limit reply
2. [api-trace] state: the same PR when the grok-4.7 call fails; expected: no second paid call, the review is requested from a human, the gate fails closed with no-reply, and a person's stated reply then gives merge at the human rung
3. [api-trace] state: a review request for the head 31 minutes old with no reply from Codex, and one 29 minutes old; expected: past 30 minutes grok-4.7 reviews and the comment says Codex has not replied in 30 minutes, within 30 minutes the gate waits with no-reply and makes no model call
4. [api-trace] state: the paid review's request on the Chat and Responses wire formats; expected: the body carries `service_tier: "priority"` (Messages never does), the cost is the priority rate (2x the standard grok-4.7 rate), and a reply served at another tier is flagged `not fast` in the inspect comment

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Engineering Manager, I want inspect to fall back to the paid rung B reviewer through the AI gateway when Codex cannot review a head, and to a person when rung B cannot either, so that a Codex usage limit no longer blocks every PR in review (Codex replied "You have reached your Codex usage limits for code reviews" on PR #509 at 12:47 UTC on 2026-10-18, and the merge gate failed closed on every PR until the quota returned).

Codex stays the default and first reviewer in every case. The fallback steps in only when Codex replies with a usage-limit or quota message, or has not replied 30 minutes after the review request. The order is Codex, then `[inspect] paid_reviewer` (grok-4.7 at high effort through the founder's AI gateway, within `[budget] per_pr_inspect_max`), then a person (`[autonomy] approver`). The reviewer's family differs from every family the builder may be at every step. The inspect comment names the rung and why. Every paid review asks for fast serving (`service_tier: "priority"`), is charged at the priority rate so the 5 USD cap holds, and the comment records the served tier and flags one that is not fast.

### Completion Goal

1. [api-trace] state: `aayu inspect <pr> --ci` on a Claude-built PR whose only reviewer app, Codex, replied "You have reached your Codex usage limits for code reviews", with `[inspect] paid_reviewer = "grok-4.7"` and its key; expected: grok-4.7 reviews at high effort, the disposition is the review's, the review is labelled `different-family review (B, paid)`, and the inspect comment says the reviewer app cannot review the head and quotes the usage-limit reply
2. [api-trace] state: the same PR when the grok-4.7 call fails; expected: no second paid call, the review is requested from a human, the gate fails closed with no-reply, and a person's stated reply then gives merge at the human rung
3. [api-trace] state: a review request for the head 31 minutes old with no reply from Codex, and one 29 minutes old; expected: past 30 minutes grok-4.7 reviews and the comment says Codex has not replied in 30 minutes, within 30 minutes the gate waits with no-reply and makes no model call
4. [api-trace] state: the paid review's request on the Chat and Responses wire formats; expected: the body carries `service_tier: "priority"` (Messages never does), the cost is the priority rate (2x the standard grok-4.7 rate), and a reply served at another tier is flagged `not fast` in the inspect comment

### Measure

PRs blocked by a reviewer app's usage-limit reply while rung B is configured and its key is present.

### Threshold

0 per day.

### Scenario

Codex's code review quota runs out mid-day while several Claude-built PRs wait in the merge gate.

### Environment

The merge gate (`aayudha/inspect`) with Codex installed and rung B through the AI gateway.

### Technical Owner

engineering-manager

### Checkpoints

1. `go test ./internal/cli -run TestInspectReviewerFallback` passes: usage-limit reply, failed rung B, and the 30-minute wait (criteria 1, 2, 3)
2. `go test ./internal/models -run 'TestWireFormats|TestCostAtListPrices'` passes: the tier on the wire and the priority price (criterion 4)

### Assumptions

The gateway forwards `service_tier` to xAI, as the gateway investigation saw it served at priority.

### Open questions

deferred: should a scheduled sweep re-run the inspect job 30 minutes after a request with no reply? (it changes the merge-gate workflow, which is in #593's narrow set)

### Parked ideas

Ask Amp with a specific model as a further rung (the founder dropped it: the gateway already holds the models and meters usage).

### Provenance

Request: "When the chosen reviewer app can't review (a usage-limit or quota reply, or no reply within the gate's wait), inspect falls to the next reviewer the repository has access to."
1. default (behaviour): "So the order is: Codex first, then the gateway model when Codex replies with a usage or quota limit or doesn't reply, then a human."
2. default (behaviour): "then the gateway model when Codex replies with a usage or quota limit or doesn't reply, then a human"
3. default (behaviour): "or no reply within the gate's wait"
4. default (behaviour): "every code review through the gateway (Grok 4.7 at high effort) uses fast serving, always"
````
