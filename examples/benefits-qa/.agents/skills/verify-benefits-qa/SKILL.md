---
name: verify-benefits-qa
description: "Drive the benefits-qa Ask page and its POST /api/ask contract through the feature map's behaviour checks, to preview a change's proof, reproduce a bug, or sweep for regressions."
---

# Verify benefits-qa

The product-owned verify skill of `examples/benefits-qa` (docs/BUILD-PLAN.md WP4.5b). The feature map in
`features/` lists what an employee or an API client can do, and each capability has a behaviour check that
`control.mjs` (Node and Playwright, the product's own stack) runs against the app on its synthetic seed data (V3).
aayudha's `verify` skill holds the procedures for creating and maintaining this map; this file is how to use it.

## What a run proves, and what it does not

A run on your machine or in your session is a **preview**: it tells you, before you push, whether the merge gate
will agree. It never satisfies `aayu ship`. The merge gate runs the same checks with the same control tool on the
PR head, and only its pack counts (S-4.2b). A preview that passes here and fails there is a finding about the
environment, not a reason to retry.

## Launch

aayu launches the app itself: `[verify] seed` (`examples/benefits-qa/deploy/serve.sh`) builds it and starts it on
`[verify.environments.local]` (`http://127.0.0.1:8787`), and is ready only when `/healthz` is answered by the
instance it started. To look at it yourself, run `examples/benefits-qa/deploy/serve.sh`; `BQA_PORT` picks another
port when your own instance is running.

## Drive

Read `features/README.md`, then the feature file for the change. Each `check` block is both the recipe you read
and the script the control tool runs:

```sh
aayu verify --map --json                    # the features and checks, and whether the map matches the source
aayu verify --check ask-answer.typed        # one check; its artifacts go into the local pack
aayu verify --check ask-answer              # every check of a feature
aayu verify --check all                     # every check (the nightly sweep)
```

Drive through the page and the API only. Never call Go functions, set state, or edit `store/seed/plans.json` to make
a check pass; if the seed lacks what a check needs, the check reports `unreachable` and the reason goes in Gotchas.

## Evidence

Each check writes `actions.jsonl` (every step and what it observed), the screenshots it asks for, a video when its
evidence is `recording`, `aria.txt` (the accessibility tree at the end), and `api-trace.jsonl` for API steps. aayu
hashes them into the pack under `behaviour/<feature>.<check>/`. To prove a story's criterion with a check, name the
check in `.aayudha/evidence/<story>.toml`:

```toml
[[criterion]]
id = 1
checks = ["ask-answer.typed"]
```

A criterion is evidenced only by an artifact of its own kind: a recording criterion is not met by a screenshot.

## Cleanup

`deploy/serve.sh` replaces only the instance it started on its port (pid checked against `/healthz`); artifacts
stay in the pack.

## Keep the map honest

`aayu verify --map` is the free source pass; `aayu verify --check all` is the live pass. Fix doc drift in
`features/`, a harness gap in `control.mjs`, and report a product regression as a Bug. Never weaken a check to make
it pass.
