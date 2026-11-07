# Implementation plan: aayu ask

## Stories and Bugs

Feature #880 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed, one per part of the #880
Scope. No source estimates effort or risk.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | `aayu ask` posts one structured question on an item and records `asks/<item>/<id>.json` | Story (proposed, not filed) | not filed | not estimated | not estimated | #871 (the asking session) | `internal/cli/ask.go` (new), `internal/verbs/registry.go`, generated files | not started |
| P2 | The `ask` and `ask-provisional` Need kinds show each open ask in the inbox | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, #873 | `internal/needs/**`, inbox files that #873 leaves (inferred) | not started |
| P3 | A person answers with `aayu ask --answer`, in the inbox or the agent tool (two-way only), and the earliest verified answer wins | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | `internal/cli/ask.go`, `internal/cli/presence.go` reuse (read only) | not started |
| P4 | An `/aayu answer` reply answers an ask, with the checks of `/aayu approve` | Story (proposed, not filed) | not filed | not estimated | not estimated | P3 | `internal/cli/approve.go`, `.github/workflows/aayudha-approve.yml`, `docs/threat-model.md` | not started |
| P5 | A build asks instead of only blocking; a clarifying answer goes to `specs/<n>/answers.md` and tells the Kanha | Story (proposed, not filed) | not filed | not estimated | not estimated | P3 | `internal/cli/build.go`, `internal/cli/buildnext.go`, `internal/cli/kanha.go` (call `answeredAsk`) | not started |

## Delivered

Nothing under #880. Hooks that other Stories added for it:

- `Env.answeredAsk` (`internal/cli/kanha.go:198`), in
  [7a9f3d7](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e) (#1097, Story #1008 of Feature #889). No
  caller yet.
- The brief's reader of `specs/<n>/answers.md` (`internal/cli/delegate.go:481`), in
  [53186c9](https://github.com/rahulnakmol/agentic-ayudha/commit/53186c97) (#1094, Story #1006 of Feature #889).
- Mission asks on a Feature (`internal/cli/missionask.go`), in
  [3ece84e](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7) (#1090, Story #1001). These are a separate
  mechanism; the evidence does not show a plan to fold them into `aayu ask`.

## In review

None (`_data/open-prs.tsv`).

## Pending

- P1 to P5.
- Person steps: #880's baselines and Approve to build (none on the ledger).
- The count "16 checks" (#880 Scope): unverified against `internal/cli/approve.go`.
- The config key implied by `internal/config/defaults.toml` in Expected paths: unverified.
