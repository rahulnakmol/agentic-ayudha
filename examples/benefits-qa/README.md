# benefits-qa

The nine-layer example product of WP8.1 (docs/BUILD-PLAN.md): employees of the fictional employer Example Co ask
benefits questions and get the plan passage that answers them, quoted with its source; questions about another
person's records or asking for advice are refused, and the rest are referred to HR. Every piece of data is synthetic
(V3). It was built by the pod from `aayu grill goal` (Goal #125) to `aayu release` (Release #144), one story per
pull request through the merge gate and `aayu ship`.

| Layer | Directory | What it holds | Story |
|---|---|---|---|
| L01 | `web/` | The Ask page, styled only with the design-system tokens | #134 |
| L02 | `api/`, `cmd/` | `POST /api/ask`, `GET /healthz`, and the `benefits-qa` command | #132, #134 |
| L03 | `ask/` | The question workflow and its product events | #132 |
| L04 | `policy/` | Refusal and grounding rules | #131 |
| L05 | `answer/` | BM25 retrieval of the best passage, with a grounding score | #131 |
| L06 | `contracts/` | The API contract and its generated `ask.schema.json` | #132 |
| L07 | `store/` | The checked, embedded synthetic plan documents | #130 |
| L08 | `analytics/` | The Application Insights export and the seeded simulation the Goal is measured on | #133 |
| L09 | `deploy/` | `serve.sh`, the `[verify] seed` command the web floors run | #134 |

Run it on its seed data and open `http://127.0.0.1:8787`:

```sh
examples/benefits-qa/deploy/serve.sh
```

Regenerate the synthetic exports (the tests check that the committed ones match byte for byte):

```sh
go run ./examples/benefits-qa/analytics/simulate -channel hr-ticket -seed 1 -from 2026-08-10 \
  -out examples/benefits-qa/analytics/testdata/hr-ticket-seed1-2026-08-10.jsonl
go run ./examples/benefits-qa/analytics/simulate -channel benefits-qa -seed 2 -from 2026-09-14 \
  -out examples/benefits-qa/analytics/testdata/benefits-qa-seed2-2026-09-14.jsonl
```

## Running it in production (WP5.10)

With `-telemetry <dir>` the server writes OpenTelemetry traces (`traces.jsonl`, OTLP JSON; a recovered panic becomes
a 500 with an `exception` event and its stack trace) and JSON logs that carry each span's `trace_id`
(`logs.jsonl`), tagged with the revision it was built from (`service.version`) and its environment
(`api.Observe`). `deploy/deploy.sh <revision>` deploys one commit as a production-like service on port 8788 and
appends a deploy marker to `deploys.jsonl`; `deploy/deploy.sh --rollback` redeploys the revision it replaced. Those
three files are the sanitised export `aayu investigate` reads, and the two deploy commands are what the example's
catalog actions run through `aayu respond`, after a person approves them.
