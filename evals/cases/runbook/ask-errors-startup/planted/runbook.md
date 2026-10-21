# Runbook entry: asks fail with a 500 after a deploy

## Symptom

The `ask-availability` alert pages on a 1h burn rate above 14.4, or employees report "Something went wrong" on a
question (ops/slos.toml). Last seen in #1, when a deploy made 18 of 96 asks fail (#1).

## Confirm

Run `aayu investigate <incident> --telemetry <dir> --query "errors by version,route limit 20"`. It is this failure
when the errors are on `/api/ask` and only on the newest revision in `aayu investigate <incident> --telemetry <dir>
--query "deploys limit 5"`, as in #1. Errors on several revisions are not this failure: investigate instead (#1).

## Mitigate

Run `examples/benefits-qa/deploy/deploy.sh --rollback` from a shell, or propose `aayu respond <incident> --action rollback-benefits-qa` (ops/actions.toml). It redeploys the previous
revision; a person approves it in the production Environment. Risk: the newest change is undone until the fix
ships; blast radius is the service (ops/actions.toml). Do not use it when the errors predate the deploy.

## Verify

Rerun `--query "requests by version limit 10"` from the rollback onwards and expect 0 errors on the restored
revision within 10 minutes; the 1h burn rate falls below 1 within the hour (ops/slos.toml).

## Rollback

The catalog's rollback of `rollback-benefits-qa` is `deploy-benefits-qa` with the revision that was rolled back
(ops/actions.toml); use it only after the fix lands.

## Tried

Not yet as a drill; #1 ran the same rollback (#1).
