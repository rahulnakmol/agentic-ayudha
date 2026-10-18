# rahulnakmol/agentic-ayudha#530: investigate shows Seer's existing analysis of an issue as a labelled opinion, never as evidence

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `9e785c76fd412727894df8a61f652995619afbccf619ea9c3df421706a07d49d` |
| Completion Goal hash | `60299be02d669f8117a340ef3668bbf2d01a5cb4dfdec694075af68e75aa1a88` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/530/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a recorded Seer issue-fix state with a completed root cause and solution; expected: the opinion holds the status, the root cause, and the solution as text with secrets and emails redacted and long text clipped, rendered in a block labelled as Seer's opinion that the evidence floor cannot cite
2. [api-trace] state: an issue Seer never analysed (no state) and an answer of the wrong shape; expected: the first is "no Seer analysis" with no error, the second an error naming the undecodable answer
3. [api-trace] state: the lookup; expected: one GET to the organization's issue-fix state endpoint for the issue, never a POST, with its request and a SHA-256 over the answer

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer, I want investigate to show what Sentry's Seer already concluded about an issue, labelled as Seer's opinion and never counted as evidence, so that a person sees an existing analysis without anyone paying for a new one (Decision #449, phase P1). Seer's issue-fix state is a read (`GET …/issues/{id}/autofix/`, scope `event:read`); starting a run is a paid write that stays refused. This Story adds a read-only lookup in `internal/production` that returns Seer's status, root cause, and proposed solution as untrusted text with no citable form. Wiring it into the investigate note comes with the P1 skill text.

### Completion Goal

1. [api-trace] state: a recorded Seer issue-fix state with a completed root cause and solution; expected: the opinion holds the status, the root cause, and the solution as text with secrets and emails redacted and long text clipped, rendered in a block labelled as Seer's opinion that the evidence floor cannot cite
2. [api-trace] state: an issue Seer never analysed (no state) and an answer of the wrong shape; expected: the first is "no Seer analysis" with no error, the second an error naming the undecodable answer
3. [api-trace] state: the lookup; expected: one GET to the organization's issue-fix state endpoint for the issue, never a POST, with its request and a SHA-256 over the answer

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Replay recorded Seer issue-fix states through the real adapter, render the opinion block, and check that it carries no citable form.

### Environment

local checkout and this repository's CI build job on a pull request; recorded answers shaped from Sentry's OpenAPI specification, no provider account

### Checkpoints

1. `go test ./internal/production -run 'TestSeerOpinion'` shows the opinion block, the empty state, the wrong shape, and the request (criteria 1, 2, 3)

### Assumptions

- the recordings are shaped from getsentry/sentry-api-schema at b1eac6b, operation "Retrieve Seer Issue Fix State" and its examples, read 2026-10-17; the endpoint is marked experimental, so a changed shape is reported as an error, never guessed; not live-proven
- reading an existing analysis costs nothing; starting one (`POST`) stays refused by the hook and is a catalog action in phase P5
- a block the evidence floor cannot cite is text outside any `query qN` citation, so the citation rules do not change

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a check; "accepted, keys later, 3 yes, 4 yes"
Vague: "inspect" → criteria 1, 2
1. stated (Production Engineer, consulted 2026-10-17): "Render Seer's state in an untrusted block labelled as an opinion, and give it no citable form, so the evidence floor can never count it as a cause"
2. stated (Decision #449): "Provider output is untrusted data"
3. stated (founder): "paid AI features (Seer, PostHog AI credits, Replay Vision) stay off by default"
````
