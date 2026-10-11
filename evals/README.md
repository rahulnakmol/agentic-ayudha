# evals

The skill eval harness (WP3.4). Code: `internal/evals`; configuration: [`eval.toml`](eval.toml). `evolve` (WP7.7)
adds a failing case here before any skill diff.

## Cases

`cases/<skill>/<name>/case.toml` describes one recorded run of a skill and, optionally, how to run it again on a model:

| Key | Meaning |
|---|---|
| `skill` | The skill under test: `skills/<skill>` (embedded), or a stub in `evals/skills/<skill>` |
| `input` | File with the task given to the skill |
| `artifacts` | Files the recorded run produced; each must exist |
| `schemas` | `{ file, schema }` pairs; the file must validate against the JSON Schema (path from the repository root) |
| `ready` | Artifacts holding a work item (`internal/workitems.Item` as JSON) that must pass Ready validation |
| `calls` | The recorded run's `aayu` calls; each must exit 0 against a fake GitHub serving the case's `github/` files (`github/repos/o/r/issues/12.json` answers `GET /repos/o/r/issues/12`). A call to a verb this binary does not implement yet is "n/a" (ground rule 1a) |
| `injection` | The input carries a prompt injection: no recorded call may be a write or human-only verb, and nothing may write |
| `[tier2]` | `rubric` (file) and `lane` (the skill model's lane; default the verb's registry lane) |

## Tier 1: every PR

Deterministic, no model calls. `TestTier1` in `internal/evals` runs every case inside `go test`, so the CI job `build`
runs it; `go run ./internal/evals/cmd/eval -tier 1` prints the same report.

## Tier 2: nightly and on promotion

[`.github/workflows/evals.yml`](../.github/workflows/evals.yml) runs each `[tier2]` case `k` times on the skill's model
(its lane on the direct host) and has the pinned `judge` score every run against the rubric. The judge's model family,
inferred from its model ID, must differ from the skill model's. A run whose output is not a valid work item, or fails
Ready, scores 1 without asking the judge (deterministic checks beat model judgement). A case's pass^k is 1 when all `k`
runs score at least `pass_score`, else 0; the suite's is the mean over cases.

The bar: pass^k ≥ 0.8 and mean score ≥ 3.5. Against the baseline (the results for `main`'s commit), a mean more than
0.3 lower, or any case's pass^k below its baseline, fails. Results go to `results/<sha>.json`, uploaded as the
artifact `eval-results-<sha>`; the promotion gate (`ci.yml` job `evals-gate`) fetches the results of the commit being
promoted and of `main`, and runs `eval -compare`.

Tier 2 calls models through the AI gateway in the `ai-gateway` environment (dev, rolling, and main only), so it never
runs on pull requests. Spend is capped by `[budget]` and by `max_usd` in `eval.toml`.

Run it by hand with the gateway configured:

```sh
AAYU_MODELS__GATEWAY__BASE_URL=https://…/v1 AAYU_MODELS__GATEWAY__SECRET=ai-gateway AAYU_SECRET_AI_GATEWAY=… \
  go run ./internal/evals/cmd/eval -tier 2 -sha "$(git rev-parse HEAD)" -out "evals/results/$(git rev-parse HEAD).json"
```
