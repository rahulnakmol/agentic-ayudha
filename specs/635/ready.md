# rahulnakmol/agentic-ayudha#635: inspect reviews with the one tool the repository names, with CodeRabbit and Greptile as review services

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `07c1cf39969fc22b0da05be560b52d2a6b7e6050158f0e6f6ea9bc3bc3613510` |
| Completion Goal hash | `ef2177f09a62d0078d1e62eefd476719ce0ee2859f1afd6327e51730936da63e` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/635/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu inspect <pr>` with `[inspect] reviewer` set to each of codex, coderabbit, greptile, claude, cursor, and human; expected: the review request mentions that tool only (`@codex review`, `@coderabbitai review`, `@greptileai`, `@claude review`, `@cursor`, or the approver), and no model is called
2. [api-trace] state: `aayu inspect <pr> --ci` with `reviewer = "codex"` on a Claude-built PR after Codex replied that it reached its usage limit, and CodeRabbit reviewed the head with one Major inline finding; expected: CodeRabbit's review is read, the finding is a P1, the disposition is fix, the review is labelled `different-family review (A): coderabbit (review-service)`, and the comment says codex could not review
3. [api-trace] state: `aayu inspect <pr> --ci` with `reviewer = "greptile"` on an OpenAI-built PR whose Greptile summary names the head as last reviewed, with one P2 inline finding; expected: the finding is a P2, the disposition is merge, and the review is rung A
4. [api-trace] state: `[inspect] reviewer` unset, or `auto`; expected: the gate requests a human review and its comment says `set [inspect] reviewer = "<tool>"`, `aayu doctor` fails with the same step, and `aayu install --apply` stops until a tool is chosen, then writes it to `[inspect]`

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Engineering Manager, I want every repository to name its one review tool in `[inspect] reviewer` (codex, coderabbit, greptile, claude, cursor, grok, or human), with CodeRabbit and Greptile read as dedicated review services that count as a different-family review for any builder, so that the merge gate reviews with the tool the founder chose and falls back in a fixed, documented order when it cannot answer.

Founder decisions, 2026-10-18: "we need to add support for CodeRabbit and Greptile. When CodeRabbit or Greptile is used for code review we don't need to worry about model family, as they are more robust." (a deliberate exception to Decision #469's family rule); "People need to choose one code review tool"; "auto cannot be an option"; for this repository, "Use codex for now".

The chosen tool is the only reviewer while it answers. When an app cannot review a head (a usage-limit, quota, or rate-limit reply, or no reply 30 minutes after the request), the next installed app reviews in the fixed order codex, coderabbit, greptile, claude, cursor, then grok (`[inspect] paid_reviewer` through the AI gateway), then a person. Unset or a leftover `auto`, a person reviews and the PR says how to choose; `aayu doctor` fails with that step; `aayu install --apply` asks for the tool and writes it.

### Completion Goal

1. [api-trace] state: `aayu inspect <pr>` with `[inspect] reviewer` set to each of codex, coderabbit, greptile, claude, cursor, and human; expected: the review request mentions that tool only (`@codex review`, `@coderabbitai review`, `@greptileai`, `@claude review`, `@cursor`, or the approver), and no model is called
2. [api-trace] state: `aayu inspect <pr> --ci` with `reviewer = "codex"` on a Claude-built PR after Codex replied that it reached its usage limit, and CodeRabbit reviewed the head with one Major inline finding; expected: CodeRabbit's review is read, the finding is a P1, the disposition is fix, the review is labelled `different-family review (A): coderabbit (review-service)`, and the comment says codex could not review
3. [api-trace] state: `aayu inspect <pr> --ci` with `reviewer = "greptile"` on an OpenAI-built PR whose Greptile summary names the head as last reviewed, with one P2 inline finding; expected: the finding is a P2, the disposition is merge, and the review is rung A
4. [api-trace] state: `[inspect] reviewer` unset, or `auto`; expected: the gate requests a human review and its comment says `set [inspect] reviewer = "<tool>"`, `aayu doctor` fails with the same step, and `aayu install --apply` stops until a tool is chosen, then writes it to `[inspect]`

### Measure

Merge-gate reviews on this repository by a tool other than the one `[inspect] reviewer` names, while that tool answers.

### Threshold

0 per week.

### Scenario

A repository chooses CodeRabbit; another keeps Codex and Codex runs out of quota mid-day.

### Environment

The merge gate (`aayudha/inspect`), `aayu doctor`, and `aayu install --apply`.

### Technical Owner

engineering-manager

### Links

depends-on #631

### Checkpoints

1. `go test ./internal/inspect -run TestReviewLadder` and `go test ./internal/cli -run TestInspectReviewTools` pass (criteria 1, 2, 3)
2. `go test ./internal/doctor` and `go test ./internal/cli -run 'TestInspectReviewTools|TestInstall$'` pass (criterion 4)

### Assumptions

CodeRabbit and Greptile review the PR automatically once installed, so the gate needs no mention to reach them in the fallback.

### Open questions

deferred: should `aayu init` ask for the tool itself? (init hands over to `aayu install --apply`, which asks)

### Parked ideas

Read CodeRabbit's nitpick comments from its review body as P3 findings.

### Provenance

Request: "we need to add support for CodeRabbit and Greptile. When CodeRabbit or Greptile is used for code review we don't need to worry about model family, as they are more robust."
1. default (behaviour): "People need to choose one code review tool"
2. default (behaviour): "When CodeRabbit or Greptile is used for code review we don't need to worry about model family"
3. default (behaviour): "When CodeRabbit or Greptile is used for code review we don't need to worry about model family, as they are more robust."
4. default (behaviour): "auto cannot be an option"
Vague: "robust" → criteria 2, 3
````
