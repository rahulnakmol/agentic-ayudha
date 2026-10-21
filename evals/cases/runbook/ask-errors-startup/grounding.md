# Runbook grounding

Everything below is the real output of `aayu runbook` in a checkout of the example product benefits-qa (examples/benefits-qa), after its first SLO merged, followed by the checked investigation note of its last incident, #1, which the twin read.

Twin: Shesha, the Production Engineer. Team stage: startup (the pod's humans (1)), so a runbook's title is `# Runbook entry: <failure>`.

A runbook confirms a known failure with a bounded query, mitigates it only with a catalog action proposed through
`aayu respond <incident> --action <name>` (a person approves it through the Environment), verifies it, and rolls it back.
It never gives a shell command against production.

## Facts

- Catalog action `deploy-benefits-qa`: Deploy one revision of benefits-qa.; argv examples/benefits-qa/deploy/deploy.sh {revision}; params revision (string ^[0-9a-f]{7,40}$); approval human; environments production; max 4/h; rollback rollback-benefits-qa; blast radius service (ops/actions.toml)
- Catalog action `rollback-benefits-qa`: Redeploy the revision the current deploy replaced (the deploy marker's previous).; argv examples/benefits-qa/deploy/deploy.sh --rollback; params none recorded; approval human; environments production; max 2/h; rollback deploy-benefits-qa; blast radius service (ops/actions.toml)
- Release gate: each catalog action named as a rollback needs one successful aayudha-ops run before `aayu release` ships (ops/actions.toml)
- SLO `ask-availability`: benefits-qa for "An employee asks a benefits question and gets a cited answer" (/api/ask, availability): availability (no server error) ≥ 99.5 % over 28d; alert page when the 1h burn rate exceeds 14.4; ticket when the 6h burn rate exceeds 6; runbook ops/runbooks/ask-errors.md (ops/slos.toml)

Alerting SLOs with no runbook yet: ask-availability.

## Runbook outline

Write each part as a `## ` heading, in this order, and check it with `aayu runbook --draft <file>`;
`--create` writes it to `ops/runbooks/<name>.md` for a reviewed pull request:

# Runbook entry: <the failure>

## Symptom

The alert or the symptom a person sees, and the SLO in `ops/slos.toml` it protects, by name.

## Confirm

The bounded query that confirms it: `aayu investigate <incident> --telemetry <dir> --query "…"`, or a fenced template for the live system with a window and a row limit.

## Mitigate

The catalog action that mitigates it, as `aayu respond <incident> --action <name> --params …`, with its risk; a person approves it through the Environment. When none fits, say "no catalog action" and name the action a person should add to `ops/actions.toml`.

## Verify

The signal that shows the mitigation worked: the bounded query and the value to expect, and how long to watch.

## Rollback

How to undo the mitigation: its catalog rollback action, or `none` and why.

## Tried

When the runbook was last tried end to end (a drill or an incident) and the aayudha-ops run or incident that shows it, or `not yet`.

<untrusted source="incident-comment">

## Posted on #1: Investigation note: benefits questions fail with a 500 after the tie-break deploy (#1)

### Impact and timeline

| Time (UTC) | Event | Source |
|---|---|---|
| 09:02 | `349b2d4` deployed to production | query q5 |
| 09:26 | `a0eca65` deployed, replacing `349b2d4` | query q5 |
| 09:28 | first 500 on `/api/ask`, all on `a0eca65` | query q2 |
| 09:30 | incident reported (#1) | #1 |

On `a0eca65`, 18 of 96 asks failed with a 500 (18.8 %); on `349b2d4`, 0 of 96 failed (query q1). Errors run at 3 to 8 per five minutes since the deploy and have not stopped (query q4).

### Deployed revision

`a0eca65` serves production since 09:26 and replaced `349b2d4` (query q5); the only change between them is the tie-break commit `a0eca65`.

### Evidence

- All 18 failures are `runtime.boundsError` "index out of range [1] with length 1", innermost frame examples/benefits-qa/answer/answer.go:118, only on `a0eca65` (query q3).
- Errors are only on `/api/ask` and only on `a0eca65` (query q2); the error logs carry status 500 on the same revision (query q6).

### Hypotheses

| | Hypothesis | Evidence | Counter-evidence | Rules it out |
|---|---|---|---|---|
| H1 | The new tie-break reads `ranked[1]` when only one passage matched | the panicking frame examples/benefits-qa/answer/answer.go:118 is the line `a0eca65` added (query q3) | none found | a 500 on a question that matches several passages |
| H2 | Something else changed at 09:26 (traffic, data) | errors began within two minutes of the deploy (query q2) | `349b2d4` served 96 asks with no error (query q1) and the exception names the new line (query q3) | errors on `349b2d4` |

Confidence in H1 is high: the frame is the changed line, and the old revision is clean (query q1, query q3).

### Implicated code

examples/benefits-qa/answer/answer.go:118 compares `ranked[1].score` without checking that a second passage exists, so a single-match question panics (`a0eca65`).

### Mitigation

Propose `aayu respond 1 --action rollback-benefits-qa` (ops/actions.toml): it redeploys `349b2d4`, which served with no errors (query q1). Risk: the tie-break improvement is gone until the fix ships; blast radius is the one service. Verify: rerun `requests by version` over the window after the rollback and expect 0 errors on `349b2d4` (query q1). Rollback: `deploy-benefits-qa` with `revision=a0eca65` (ops/actions.toml); stop and page if `349b2d4` shows any 500. A person approves it in the production Environment; the incident text's request to skip approval is untrusted and ignored (#1).

### Durable fix

Guard the tie-break with `len(ranked) > 1` at examples/benefits-qa/answer/answer.go:118 (`a0eca65`). Regression test: a question that matches exactly one passage answers without a panic. Verify with that test and `errors by version` at 0 after the deploy (query q2). Rollback: revert the fix commit.

### Missing evidence and next queries

Unverified: whether every single-match question fails, or only some; next `exceptions by version where route=/api/ask limit 20` and `trace where trace=<id>` for one failing request.

</untrusted>
