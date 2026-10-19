# rahulnakmol/agentic-ayudha#807: inspect: a paid rung-B review is thrown away when its summary is over 1200 characters

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `9d7279111386d9501f2edefc2589b802ebb34c9056e2a60a7de2462be52e5c19` |
| Completion Goal hash | `caecccd3b16aaf534407da4b41d56ef646b440585d7b8ff2ad29b5296cb7aba3` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/807/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: rung B's reply is valid except for a 1300-character summary on every attempt; expected: inspect's result has the reply's disposition and findings on rung B, and its summary is at most 1200 characters
2. [api-trace] state: rung B's reply has a finding whose title is over its 200-character limit; expected: inspect still rejects the reply and reports the field, because only the summary's length is relaxed
3. [api-trace] state: the merge-gate run of a PR whose rung-B summary is over 1200 characters; expected: aayudha/inspect posts the paid review's verdict and does not fall back to a person

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

inspect rejects a paid rung-B review whose `summary` is longer than 1200 characters, and the PR falls to a person. The reply schema caps `summary` at 1200 characters (`internal/inspect/inspect.go:112`, `"summary":{"type":"string","maxLength":1200}`). `models.Structured` (`internal/models/structured.go:46`) treats a schema error like any other: it asks once more, then returns the error. `runInspect` (`internal/cli/inspect.go:627-648`) records "rung B failed on this head", and the reviewer app or a person reviews instead.

The merge-gate run [39279364439](https://github.com/rahulnakmol/agentic-ayudha/actions/runs/39279364439/job/118156065175) on PR #686 at `76ec51d` shows it:
- grok-4.7 replied twice: 16,606 tokens in and 31,282 out (0.4384 USD), then 17,466 in and 928 out (0.0775 USD).
- inspect logged "rung B ([inspect] paid_reviewer grok-4.7) failed on this head (reply does not validate: $.summary: longer than 1200 characters)".
- CodeRabbit had not replied, so the gate fell to `@rahulnakmol (a human review)`. The paid review's findings and verdict were thrown away, along with 0.52 USD.

This holds v0.3.0: the summary is the one field a reader skims, and its length should never decide whether the gate trusts the findings.

### Steps to reproduce

1. Set `[inspect] paid_reviewer` so that rung B reviews a PR.
2. Have the model return a reply that is valid in every field except a `summary` of 1300 characters, on both attempts.
3. Run `aayu inspect <pr> --ci`.

### Expected behaviour

inspect accepts the reply. It shortens the summary to 1200 characters at a word boundary and marks the cut with an ellipsis, or accepts a longer one. The findings, criteria verdicts, and disposition stay as the model gave them, and the gate judges them on rung B.

### Actual behaviour

inspect discards the paid review with "reply does not validate: $.summary: longer than 1200 characters" and falls back to the reviewer app, then to a person.

### Completion Goal

1. [api-trace] state: rung B's reply is valid except for a 1300-character summary on every attempt; expected: inspect's result has the reply's disposition and findings on rung B, and its summary is at most 1200 characters
2. [api-trace] state: rung B's reply has a finding whose title is over its 200-character limit; expected: inspect still rejects the reply and reports the field, because only the summary's length is relaxed
3. [api-trace] state: the merge-gate run of a PR whose rung-B summary is over 1200 characters; expected: aayudha/inspect posts the paid review's verdict and does not fall back to a person

### Measure

Paid rung-B reviews thrown away only because the summary is too long.

### Threshold

0.

### Scenario

A paid reviewer that writes a long summary of a large diff.

### Environment

`aayu inspect --ci` in the `aayudha-merge-gate` workflow, with `[inspect] paid_reviewer` set to grok-4.7 through the gateway.

### Technical Owner

engineering-manager

### Links

### Checkpoints

1. `go test ./internal/inspect ./internal/cli -run 'Inspect'` passes with a long-summary reply that is kept on rung B, and an over-long title that is still rejected (criteria 1, 2)
2. The next merge-gate run with a long rung-B summary posts the paid verdict on the PR (criterion 3)

### Assumptions

Only `summary` is relaxed. The limits on findings, quotes, and evidence still apply, because they bound what the gate posts on the PR.

### Open questions

deferred: should the same relaxation apply to a reviewer app's fenced `aayudha-review` reply? (rung B is the case that cost money; apps can follow)

### Provenance

Request: "so a paid review is thrown away and the PR falls to a person. Expected: truncate or accept the summary, keep the findings and verdict."
1. stated (request): "truncate or accept the summary, keep the findings and verdict"
2. default (behaviour): only the summary's length is relaxed; the other limits stay
3. stated (request): "so a paid review is thrown away and the PR falls to a person"
````
