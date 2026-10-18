# rahulnakmol/agentic-ayudha#675: Approve several human-review PRs in one aayu inbox session with one device code

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `998991cad06a7b73b08c2da50990cac1f7942d0dbcd13d1ccf667e7eaa8f9d34` |
| Completion Goal hash | `ebfc22d9a74d957eb75bc4d66884d3303b2c90249a3bd8bee10520678ff14b8d` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/675/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu inbox --list` in a repository with `[autonomy] approver` set, where the merge gate's inspect waits at the human rung for one PR's head and another PR waits for a reviewer app; expected: the first PR is listed as a human-review item for that head, and the second is not listed
2. [api-trace] state: one `aayu inbox` session by the approver, in a shell with a host agent marker, that approves two human-review PRs with a typed reason each and accepts a Decision; expected: the device flow runs once, the ledger gets one approval record for each PR's head with the approver's login and reason, and each PR gets the human-review reply that `aayu inspect <pr> --approve` posts
3. [api-trace] state: the merge gate's `aayu inspect <pr> --ci` on a PR approved in criterion 2; expected: the disposition is merge at the human rung, and the review line names the ledger record
4. [api-trace] state: the human-review card's approve choice with an empty reason, or after a push moved the PR's head since the card was read; expected: nothing is recorded on the ledger or posted on the PR
5. [api-trace] state: `aayu inspect <pr> --approve` by the approver in a shell with a host agent marker whose device code is approved, then the same under AAYU_AGENT=1; expected: the first records the approval, and the second is refused for an agent lane that names AAYU_AGENT=1

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the approver, I want to clear several PRs that wait for inspect's human review in one `aayu inbox` session with one GitHub device code, so that each `aayu inspect <pr> --approve` no longer costs its own code (@rahulnakmol, 2026-10-18: "See what we can minimize").

Today a PR whose reviewer apps and rung B cannot review it waits at the human rung until the `[autonomy] approver` runs `aayu inspect <pr> --approve` (Story #639, #647). Each run asks for its own device code (internal/cli/presence.go). `aayu inbox` asks for one code per session, and that approval covers every later step of the session at the same or a lower permission (internal/cli/inbox.go, humanCaller). But no inbox kind shows a PR at the human rung: the kinds are decision, ready, capped, unshipped, fixed and replaced. This Story adds a human-review card whose approve choice records the same ledger approval as `--approve`, under the session's one approval. presence.go stays unchanged: no cached token, no time window, no stored approval.

### Completion Goal

1. [api-trace] state: `aayu inbox --list` in a repository with `[autonomy] approver` set, where the merge gate's inspect waits at the human rung for one PR's head and another PR waits for a reviewer app; expected: the first PR is listed as a human-review item for that head, and the second is not listed
2. [api-trace] state: one `aayu inbox` session by the approver, in a shell with a host agent marker, that approves two human-review PRs with a typed reason each and accepts a Decision; expected: the device flow runs once, the ledger gets one approval record for each PR's head with the approver's login and reason, and each PR gets the human-review reply that `aayu inspect <pr> --approve` posts
3. [api-trace] state: the merge gate's `aayu inspect <pr> --ci` on a PR approved in criterion 2; expected: the disposition is merge at the human rung, and the review line names the ledger record
4. [api-trace] state: the human-review card's approve choice with an empty reason, or after a push moved the PR's head since the card was read; expected: nothing is recorded on the ledger or posted on the PR
5. [api-trace] state: `aayu inspect <pr> --approve` by the approver in a shell with a host agent marker whose device code is approved, then the same under AAYU_AGENT=1; expected: the first records the approval, and the second is refused for an agent lane that names AAYU_AGENT=1

### Measure

Device codes per approval session that clears two or more human-review PRs.

### Threshold

1.

### Scenario

Codex is out of quota and the gateway is down, so three PRs wait at the human rung; @rahulnakmol opens `aayu inbox` in the orb's Terminal tab, enters one code, and clears all three.

### Environment

`aayu inbox` in a person's terminal, including an orb's Terminal tab with a host agent marker.

### Technical Owner

engineering-manager

### Links

depends-on #639

### Checkpoints

1. `go test ./internal/cli -run 'TestInboxApprovesHumanReviews|TestInspectThroughApp'` passes: the listing, one session with one device code, the gate reading the record, the refusals, and the host marker (criteria 1, 2, 3, 4, 5)

### Assumptions

The merge gate's inspect state can record the head it waits on at the human rung, and the inbox reads that state only from a trusted writer (inspectWriter).

### Open questions

### Parked ideas

A release-baseline card (T-01a15019's scope kind) lands separately; the kind order is agreed: decision, scope, ready, human-review, capped, unshipped, fixed, replaced.

### Provenance

Request: "See what we can minimize"
1. default (behaviour): "See what we can minimize"
2. default (behaviour): "See what we can minimize"
3. default (behaviour): "See what we can minimize"
4. default (behaviour): "See what we can minimize"
5. default (behaviour): "See what we can minimize"
````
