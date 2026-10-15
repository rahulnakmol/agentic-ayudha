# Research material for docs/research/verification-skills-2026-10.md (scratch branch; never merged)

- `prototype-evidence/`: what the prototype verify skill (`examples/benefits-qa/.agents/skills/verify-benefits-qa/`)
  produced: a journey's screenshot, recording, ARIA snapshot, and action log; the API trace; `evidence.json` for the
  sample story 412 (`criteria-412.txt`); the drift report and failed sweep after the simulated product change
  (commit 30468e3), and the failure screenshot.
- `review-cmp/`: the OCR-versus-inspect comparison. `setup.sh` builds the inputs (the twelve seeded cases of
  `evals/cases/inspect/`, PR 182 as merged, and PR 182 with three planted defects, whose `case.toml` and `pr.diff` are
  in `cases/`). `inspect/` holds the replies to inspect's own prompt; `ocr/` the OCR delegation-mode comments; both
  were written by the same host model. `internal/evals/cmd/reviewcmp` runs inspect's real post-processing on a reply
  and scores both with `evals.ScoreFindings`. `scores.txt` is its output; `timings.txt` start and end per review.
