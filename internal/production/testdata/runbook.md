# Runbook entry: Ask returns 500 after a deploy

## Symptom

The `ask-availability` alert pages when its 1h burn rate exceeds 14.4 (ops/slos.toml).

## Confirm

Run `aayu investigate <incident> --telemetry <dir> --query "errors by version,route limit 20"`; errors on the newest
revision only confirm it (ops/slos.toml).

## Mitigate

Propose `aayu respond <incident> --action rollback-app` (ops/actions.toml); risk: the previous revision's known bugs
return; blast radius: the service.

## Verify

Rerun `--query "requests by version limit 10"` and expect the serving revision's error rate back to 0 % within 10
minutes (ops/slos.toml).

## Rollback

`deploy-app` with the reverted revision (ops/actions.toml).

## Tried

Not yet.
