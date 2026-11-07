# Implementation plan: #900 the factory learns from its own records

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #900. Decision #916 names "ST4, ST5, ST10" of #900; none is filed. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 900. |

## Delivered

Nothing of #900 is delivered. Building blocks on `dev`:

| Commit | What it gives #900 |
|---|---|
| [2e62c537](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c537) (#1016, Story #989) | `aayu measure --metric aayu:founder_minutes_per_pr`: a metric read from aayu's own records. |
| [2bd966e4](https://github.com/rahulnakmol/agentic-ayudha/commit/2bd966e4), [b83e1879](https://github.com/rahulnakmol/agentic-ayudha/commit/b83e1879), [fb412a5a](https://github.com/rahulnakmol/agentic-ayudha/commit/fb412a5a), [884d81b2](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b2) (Refs #1170) | The retro skill claims only what its record shows. Retro is the source of the Feedback that evolve reads. |
| Before v0.3.0 | `aayu evolve` (`internal/cli/evolve.go`, WP7.7) and `aayu retro --classify`. |

## In review

No open PR refs #900 (`_data/open-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The issue gives no effort and no
estimate. It says "Routing evidence is built last" (#900 Scope).

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 900-A | Each run ends with an outcome: success, flawed, blocked, or manual | — | L02 `internal/cli/build.go`, `internal/cli/buildnext.go`, `internal/cli/delegate.go`; L07 `internal/ledger/**` |
| 900-B | A blocked outcome opens an ask | #880, 900-A | L02 `internal/cli/build.go` (after 900-A) |
| 900-C | A finding or failure class seen three times files Feedback, with no model | 900-A | L02 `internal/cli/retro.go`, `internal/cli/fixlayer.go`; L07 `internal/ledger/**` |
| 900-D | evolve runs on the three-times trigger, within a week, eval-first | 900-C, 900-E | L02 `internal/cli/evolve.go`; L03 `loops/evolve.toml` |
| 900-E | evolve refuses shipped skills and evolves a repository's own skills, with `[evolve] cooldown_days` | — | L02 `internal/cli/evolve.go`; L05 `internal/evals/evolve.go`; L07 `internal/config/defaults.toml`, `internal/config/config.go` |
| 900-F | Fix rounds, rework, and cost by lane and model on the Build board, cited by routing changes | #897 merge records plus 4 weeks of data | L05 `internal/models/**`; L02 `internal/cli/models.go`, `internal/cli/streak.go`, `internal/cli/inspect_app.go` |

### Gaps the sources do not close

- The Build board of 900-F is not on `dev`. Which Feature builds it is unverified.
- `internal/delivery/**` is an Expected path that does not exist. #889's planned Story #1010 ("Every build event of a
  Feature goes to `delivery/<f>/events.jsonl`", `status:planned`, `_data/subs.tsv`) names a `delivery/` record.
  Whether #1010 creates the `internal/delivery` package is not stated. 900-A could write its outcome next to that
  record once #1010 exists. Inferred.
- #880 (`aayu ask`) has no filed Stories in `_data/subs.tsv`. 900-B cannot start before it.
- #916's sizing predictor is not in #900's Scope. A person decides whether #900 holds it.
