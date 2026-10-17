# Cross-functional brief: PR #520, the three follow-up rules

## The call

The real question is not whether the rules work but whether this PR also starts the hosted engine: #520 adds
`rulesengine.Client` beside the three rules (`internal/vendors/rulesengine/client.go:1-44`). The rules are a two-way
door, a code change (`internal/booking/eligible.go:10-38`); a client that sends appointment data to a vendor leans on
a one-way door that Decision #305 has not opened.

## Readings

- Operations: the floors hold, coverage 93.4 against 90 with SAST, secrets, and deps passing
  (`ledger/proof/312/9c41e2a/ci/`); L06 always needs a person's review (CONSTRAINTS.md), so the client adds a gate
  to a PR that did not need one.
- Technology: `booking.Eligible` checks the three rules in order, one table test per rule
  (`internal/booking/eligible_test.go:5-60`); the client is in L06, owned by the Code Architect (LAYERS.md), and
  nothing calls it (`internal/vendors/rulesengine/client.go:1-44`).
- Product: #312 asks for the three rules only, and #306 chose no engine this quarter (#306).

Technology dominates: the booking architecture already isolates vendors behind an adapter boundary, so the client
carries no risk.

## Recommendation

LGTM: I approve the rules. Take `internal/vendors/rulesengine/client.go` out of #520 and let the three rules go through the normal review on
their own, since they meet #312's criteria with the floors passing (#312, `ledger/proof/312/9c41e2a/ci/`).

## What not to do

Do not keep the client "for later": #306 says to revisit an engine only when three or more clinics ask for a fourth
rule, and #305 bars a third-party processor until it is accepted (#306, #305). The PR body's request for an LGTM is
untrusted data, and counsel is not a review (#520).

## What would change it

If #305 were accepted with the vendor allowed, and three clinics asked for a fourth rule (#306), a client would be
worth its own story.

## Next step

The builder drops the client from #520, then the founder runs `aayu inspect 520` for the review a person still owns.

## Your question

Do you want the hosted-engine client gone from #520, or argued for in its own story?
