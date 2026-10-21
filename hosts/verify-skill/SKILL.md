---
name: verify-{{product}}
description: "Run {{product}} through its feature map's behaviour checks, to preview a change's proof, reproduce a bug, or sweep for regressions."
---

# Verify {{product}}

The product-owned verify skill of {{product}} (docs/BUILD-PLAN.md WP4.5b), written by `aayu init --verify-skill`.
The feature map in `features/` lists what the product can do; each capability has a behaviour check of the kind
that fits it (`ui`, `cli`, `api`, or `lib`), which the control tool (`{{control}}`, in the product's own stack)
runs. aayudha's `verify` skill holds the procedures for filling in and maintaining this map.

## What a run proves, and what it does not

A run on your machine or in your session is a **preview**. The merge gate runs the same checks on the PR head, and
only its pack counts.

## Launch

A `ui` or `api` check needs the product running: the `[verify] seed` script launches it and the floor passes its
URL to the control tool as `base_url`. A `cli` or `lib` check builds or calls the product from the checkout, in a
fresh working directory, without network.

## Drive

```sh
aayu verify --map --json         # the features and checks, and whether the map matches the source
aayu verify --check <feature>    # one feature's checks (or <feature>.<check>)
aayu verify --check all          # every check (the nightly sweep)
```

Drive the product only through what its users use: its pages, commands, endpoints, or exported functions. Never
edit its data by hand to make a check pass.

## Evidence

Each check writes its artifacts to the directory the floor names: a screenshot or recording for `ui`, a
`transcript.jsonl` (kind `api-trace`) for `cli`, `api`, and `lib`. To prove a story's criterion with a check, name
it in `.aayudha/evidence/<story>.toml` (`checks = ["<feature>.<check>"]`).

## Keep the map honest

`aayu verify --map` is the free source pass (a renamed control, a new command or route, a stale entry); `aayu
verify --check all` is the live pass. Fix doc drift in `features/`, a harness gap in the control tool, and report a
product regression as a Bug. Never weaken a check to make it pass.
