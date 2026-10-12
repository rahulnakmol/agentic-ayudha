# work-items

The work-item schema (WP2.1) and the issue forms generated from it (WP2.2). The schema itself is Go data in
[`internal/workitems/schema.go`](../internal/workitems/schema.go): the ten types, their Create, Ready, and Complete
sections, the fields each stage requires, the parents and links each type allows, and the label set. `go generate
./...` writes one GitHub issue form per type into [`forms/`](forms/) and the table below; `aayu init` installs the
forms as `.github/ISSUE_TEMPLATE/aayudha-<type>.yml`. Embedded by the root `embed.go`.

A Story or Bug carries a Completion Goal: numbered observable criteria, one per line, each naming the evidence that
will prove it, plus Measure, Threshold, Scenario, and Environment:

```text
1. [screenshot] state: cart is empty; expected: the "Your cart is empty" message and a Browse button
2. [api-trace] state: POST /cart with no items; expected: 422 with code cart.empty
```

Evidence kinds: `screenshot`, `recording`, `sr-transcript`, `api-trace`, `metric`. A criterion without one is
unmapped and fails Ready. Typed links go in the Links section as `<kind> #<number>`, one per line.

<!-- BEGIN GENERATED: work-items (internal/workitems/gen; DO NOT EDIT) -->

Schema version 1.

| Type | Parent | Links | Completion Goal | Intake | Form |
|---|---|---|---|---|---|
| Goal | none | none | no | no | [`aayudha-goal.yml`](forms/aayudha-goal.yml) |
| Initiative | goal (required at ready) | none | no | no | [`aayudha-initiative.yml`](forms/aayudha-initiative.yml) |
| Feature | initiative (required at ready) | depends-on | no | no | [`aayudha-feature.yml`](forms/aayudha-feature.yml) |
| Story | feature (required at ready) | depends-on | yes | no | [`aayudha-story.yml`](forms/aayudha-story.yml) |
| Bug | feature, story | caused-by, depends-on | yes | no | [`aayudha-bug.yml`](forms/aayudha-bug.yml) |
| Decision | goal, initiative, feature, story, bug, incident | supersedes, constrains, waives | no | no | [`aayudha-decision.yml`](forms/aayudha-decision.yml) |
| Experiment | feature, initiative (required at ready) | none | no | no | [`aayudha-experiment.yml`](forms/aayudha-experiment.yml) |
| Release | goal | includes | no | no | [`aayudha-release.yml`](forms/aayudha-release.yml) |
| Feedback | none | becomes | no | yes | [`aayudha-feedback.yml`](forms/aayudha-feedback.yml) |
| Incident | none | none | no | yes | [`aayudha-incident.yml`](forms/aayudha-incident.yml) |

Labels (one value per prefix; Priority, Effort, Risk, Health, and Horizon live only in labels):

- Delivery Status: `status:` backlog, discovery, planned, ready, in-progress, blocked, validation, merged, released, measured, done, cancelled
- Priority: `priority:` P0, P1, P2, P3
- Effort: `effort:` XS, S, M, L, XL
- Risk: `risk:` low, medium, high
- Health: `health:` on-track, at-risk, off-track
- Horizon: `horizon:` now, next, later
- Fixed: `intake`, `needs:grill`, `needs:bug`, `identity:human-fallback`, `decision:provisional`, `decision:waiver`, `aayu:correction`, `agent-authored`

<!-- END GENERATED: work-items -->
