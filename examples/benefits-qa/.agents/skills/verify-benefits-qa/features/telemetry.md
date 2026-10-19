# Operational telemetry

Started with `-telemetry <dir>`, the app writes every request except `GET /healthz` as an OpenTelemetry server span
(`traces.jsonl`, OTLP JSON) and one JSON log line (`logs.jsonl`) that carries the span's `trace_id` and `span_id`.
Both name the deployed revision (`service.version`, from `-revision`) and the environment (`-environment`). A handler
that panics answers 500 and leaves an `exception` event on its span. `aayu investigate` reads this export.

```feature
{"id": "telemetry", "layers": ["L02", "L08"],
 "sources": ["examples/benefits-qa/api/observe.go", "examples/benefits-qa/cmd/benefits-qa/main.go"],
 "verified_at": "2f4cbb57b32c55835a2267885ccdc9fb8f876885"}
```

## Sub-features

- `ask-traced`: one answered `POST /api/ask` appends a server span for that route with status 200 and the launch's
  revision, and a log line with the same trace ID; a `/healthz` poll appends neither.

## How to reach it

- Not visible to the employee. An operator reads the export, through `aayu investigate <incident> --telemetry <dir>`.
- `deploy/deploy.sh <revision>` (the ops catalog's deploy and rollback actions) starts the app with `-telemetry`.

## Behaviour checks

```check
{"id": "ask-traced", "kind": "api", "evidence": "api-trace", "requires": "telemetry", "steps": [
  {"api": {"path": "/api/ask", "json": {"question": "What is the dental annual deductible?", "user_id": "v-verify"},
           "status": 200, "body": {"status": "answered"}}},
  {"telemetry": {"route": "POST /api/ask", "status": 200, "logJoinsTrace": true, "untraced": "/healthz"}}
]}
```

## Gotchas

- `deploy/serve.sh` starts the app without `-telemetry`, and `deploy/deploy.sh` is a person-approved catalog action,
  not a seed, so this check is unreachable through the `[verify] seed` launch.
- Nothing proves this capability today: no Go test in `examples/benefits-qa` starts the command with `-telemetry` or
  exercises `api.Observe` (`internal/cli/investigate_test.go` builds its own export with its own tracer). Until the
  seed passes `-telemetry` and `control.mjs` learns the `telemetry` step, treat it as unproven.
