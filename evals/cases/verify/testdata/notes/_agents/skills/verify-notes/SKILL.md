---
name: verify-notes
description: "Run the notes CLI and its store library through the feature map's behaviour checks, to preview a change's proof, reproduce a bug, or sweep for regressions."
---

# Verify notes

The product-owned verify skill of `notes`, a CLI with no user interface beyond
its commands. The feature map in `features/` lists its capabilities; each has a behaviour check that `control/`
(Go, the product's own stack) runs: CLI commands against a notes file of the check's own, and the store library
through its own test. aayudha's `verify` skill holds the procedures for creating and maintaining this map.

## What a run proves, and what it does not

A run on your machine or in your session is a **preview**. The merge gate runs the same checks on the PR head, and
only its pack counts (S-4.2b).

## Launch

Nothing to launch: each check builds `cmd/notes` from the checkout (its `build` step) and runs in a
fresh working directory, without network.

## Drive

```sh
aayu verify --map --json                # the features and checks, and whether the map matches the source
aayu verify --check mark-done.missing   # one check
aayu verify --check all                 # every check (the nightly sweep)
```

Drive through the CLI and the store's exported functions only; never edit a notes file by hand to make a check
pass.

## Evidence

Each check writes `transcript.jsonl` (kind `api-trace`): every command with its exit code, stdout, and stderr, and
every test event. To prove a story's criterion with a check, name it under the criterion in
`.aayudha/evidence/<story>.toml`:

```toml
[[criterion]]
id = 1
checks = ["mark-done.done"]
```

## Cleanup

The control tool removes its build and notes file; artifacts stay in the pack.

## Keep the map honest

`aayu verify --map` is the free source pass (a renamed command or flag, a new exported function); `aayu verify
--check all` is the live pass. Fix doc drift in `features/`, a harness gap in `control/`, and report a product
regression as a Bug. Never weaken a check to make it pass.
