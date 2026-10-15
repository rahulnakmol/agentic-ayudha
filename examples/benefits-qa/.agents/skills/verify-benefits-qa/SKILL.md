---
# PROTOTYPE (docs/research/verification-skills-2026-10.md): what `aayu init --verify-skill` could generate for a
# product repository. Not part of aayudha; lives on the scratch branch scratch/verify-skill-prototype only.
name: verify-benefits-qa
description: "Drive the benefits-qa Ask page and its POST /api/ask contract the way an Example Co employee does, and capture evidence for Completion Goal criteria. Use to reproduce a bug, to preview the proof of a story before the merge gate, or to sweep the app for regressions."
disable-model-invocation: false
---

# Verify benefits-qa

Drives the real app on its synthetic seed data (V3) through `control-bqa`, a small CLI next to this file. Every
command prints JSON on stdout and exits non-zero with a message that says what to do next.

```sh
C="node examples/benefits-qa/.agents/skills/verify-benefits-qa/control-bqa.mjs"
$C --help
```

## What a run proves, and what it does not

A run of this skill on your machine or in your session is a **preview**. It is how you find out, before you push,
whether the merge gate will agree. It never satisfies `aayu ship`: only the pack the merge gate builds from its own
run on the PR head counts (ETHOS 1, S-4.2b). The gate runs the same journeys with the same CLI, so a preview that
passes here and fails there is a finding about the environment, not a reason to retry.

## Launch

```sh
$C up --port 8791            # runs deploy/serve.sh (the repository's [verify] seed) on its own port and run dir
```

Ready means `/healthz` is answered by the process `up` started (its pid in the `X-Benefits-Qa-Pid` header), not just
that something listens. Use a port other than 8787 when the developer's own instance may be running.

## Doctor

```sh
$C doctor --port 8791
```

Checks: the port answers, the pid on `/healthz` is the one this run started, and the binary was built from the
current `HEAD`. Run it before the first drive, after any failed drive, and whenever a result surprises you.

## Drive

Read `features/README.md`, then the feature file for the change. Each journey is a fenced `journey` block that is both
the recipe you follow and the script the CLI runs:

```sh
$C list                                   # features and journeys
$C journey ask-answer.typed --port 8791   # one journey; artifacts under .aayudha/verify/<run>/
$C sweep --port 8791                      # every journey, in the README's order
```

Drive through the page and the API only. Never call Go functions, set state, or edit `store/seed/plans.json` to
make a journey pass; if the seed lacks what a journey needs, the journey fails and the reason goes in `Gotchas`.

## Evidence

`journey` writes, per run, `actions.jsonl` (each step and what it observed), the screenshot or video the journey
asks for, an ARIA snapshot of the end state, an API cross-check where the feature names one, and `result.json` with the
SHA-256 of every artifact. Artifacts stay in `.aayudha/verify/` after `down`.

For a story, map criteria to journeys in `.aayudha/evidence/<story>.toml` (beside the `tests` the coverage floor
already reads) and run:

```sh
$C evidence --story 412 --criteria criteria.txt --port 8791
```

It writes `evidence.json` in the shape `aayu verify` packs (`path`, `kind`, `sha256`, `criteria`), marked
`"authority": "preview"`. A criterion whose journey failed gets no artifact and is listed under `unmet`.

## Cleanup

```sh
$C down --port 8791
```

Stops only the process `up` started (pid checked against `/healthz`), removes the run directory, and keeps
`.aayudha/verify/`.

## Keep the map honest

```sh
$C drift            # deterministic: handles still in source, sources changed since verified_at, routes not mapped
```

`drift` is the source half of the maintain pass. The live half is `sweep`. Fix doc drift in `features/`, a harness
gap in `control-bqa.mjs`, and report a product regression as a Bug; never weaken a journey to make it pass.
