# rahulnakmol/agentic-ayudha#38: WP2.7 Parser and `ready`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `c0991c9e430c7631a36994cc6dd7d02a7c378f34b217cb57e33d03786b00b92f` |
| Completion Goal hash | `a4e8f845e0b4da037567d969a3e727de5aa289d70f28522eaaaee7f2f29f8c45` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/38/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP2.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixtures pass/fail with expected messages
2. [api-trace] state: the WP2.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: edited-after-Ready fixture is stripped by the guard
3. [api-trace] state: the WP2.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: human-fallback snapshot carries the tag and `audit` fixture lists it

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP2.7 Parser and `ready` built as docs/BUILD-PLAN.md specifies, so that M2 (Work items) delivers: A founder can init a repository, author issues through forms, make them Ready, see status, and triage intake (§4 'founder-usable early' after PR12).

Plan text (docs/BUILD-PLAN.md, WP2.7): Body → typed struct by heading; stage validation; parent type; Effort ≠ XL; Completion Goal criteria complete; Technical Owner resolved (Code Architect twin in solo). `ready <issue>` sets `status:ready`, posts replace-in-place Ready snapshot (body hash + Completion Goal hash), writes `specs/<id>/ready.md` to `ledger` under the bot identity, or under the human identity tagged `identity:human-fallback` when no bot exists (WP1.4 day-one path); Technical Owner resolution is "the Code Architect twin" as a literal string until seats exist (n/a until WP5.1). `ready --check` read-only (MCP-safe). Ready guard workflow (`labeled`/`edited`): re-validates, strips `status:ready` with a comment when validation fails or body hash drifts.

### Completion Goal

1. [api-trace] state: the WP2.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixtures pass/fail with expected messages
2. [api-trace] state: the WP2.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: edited-after-Ready fixture is stripped by the guard
3. [api-trace] state: the WP2.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: human-fallback snapshot carries the tag and `audit` fixture lists it

### Measure

WP2.7 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP2.7)

### Threshold

all 3 criteria

### Scenario

Open the WP2.7 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
