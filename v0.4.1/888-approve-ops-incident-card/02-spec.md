# #888 Spec

No spec baseline exists (`specs/888` is absent from the ledger) and no Story is filed under #888. Requirements trace
to #888 and to Story #947's Completion Goal, which sits under Feature #975, not under #888. Slices are proposed, not
filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Functional requirements

| # | Requirement | Source | Slice |
|---|---|---|---|
| FR1 | `aayu respond <incident> --approve <run>` approves the pending deployment of run `<run>` as the person. | [#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888) Summary | P1 |
| FR2 | `requireReviewers` runs first, unchanged: an environment with no required reviewers, or that any branch may deploy to, is refused. | #888 Scope; `requireReviewers` (`internal/cli/respond.go:314-339`) | P1 |
| FR3 | The pending-deployments call runs only after the presence check. | #888 Scope; `presenceCheck` (`internal/cli/presence.go:136`) | P1 |
| FR4 | aayu warns when the environment has prevent self-review (F8). | #888 Scope | P1 |
| FR5 | The approval is human-only and a one-way door. | #888 Summary | P1 |
| FR6 | An incident-action Need kind shows in the Now area of the inbox. | #888 Scope | P2 |
| FR7 | The card shows the incident, the timeline, the action with blast radius, undo, rate limit, and environment, and "1 Approve as you · 2 Open on GitHub ↗ · 3 Refuse with a reason". | [#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947) criterion 1 | P2 (overlaps #947) |
| FR8 | Choice 1 does one presence check, then GitHub's pending-deployment approval, and prints "Approved as @`<login>` · run `<id>` started · rollback ready". | #947 criterion 3 | P1, P2 |
| FR9 | With no required reviewers, choice 1 is off and the card says why. | #947 criterion 4 | P2 |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Security | No second way into production. GitHub's environment protection stays the gate. | #888 Out |
| Security | The approval is the person's, never an agent's. `respond` is `IdentityHuman` with gates `ops-catalog`, `environment-approval`, `human-identity`, and `Exposure: ExposeNever`. | `internal/verbs/registry.go`, entry `Name: "respond"` |
| Security | Issue and incident text stays untrusted data. | ETHOS principle 6; `rep.Untrusted` printed by `runRespond` (`internal/cli/respond.go:234`) |
| Operability | The catalog's per-hour limit still applies. | `rateLimit` (`internal/cli/respond.go:341`) |

## Open questions

- Must the incident-action card wait for the Need registry of #873, or may it be an `InboxItem` kind in
  `internal/cli/inbox.go` first? #888 says "Need kind" and `depends-on #873`; #873 is not sliced.
- Which of #888 and #947 owns the card? Both describe it. The sources do not say.
