# Technical design: Feature #876

## Tech baseline

`specs/876/baselines/tech-v1.json` on `ledger`: owner `code-architect`, approved by `rahulnakmol` at
2026-11-02T17:19:21Z, hash `ff5eaa1d7c6b…`. It pins `specs/876/design.json` (SHA-256 `90db8eca7dc2…`). The design
(planned 2026-10-31T10:05:47Z) has footprint and layers L02, L03, L05, L07, story order #1114, #1115, #1116, and
these contracts (quoted):

1. "critique state of a Feature (internal/critique): each seat's review findings with their state (open, fixed by
   a later approved baseline version, dismissed); Approve to build reads it through Env.CritiqueState, today a stub
   (internal/cli/readyapprove.go critique)"
2. "Approve to build stays off while any finding is open, and lists each finding with its state (#1114)"
3. "aayu grill feature &lt;f&gt; --findings: lists each finding; --resolve &lt;id&gt; --as dismissed --reason
   &lt;line&gt; is human-only, through GitHub's device flow, and Approve to build ratifies each dismissal (#1115)"
4. "the grill skill walks each open gap with a recommendation, drafts the fix as a new baseline version (aayu ready
   &lt;f&gt; --part &lt;p&gt; --request --fixes &lt;ids&gt;), and hands the person the dismissal command; an eval
   fails when the agent dismisses a gap itself (#1116)"
5. "[critique] keys stay owned by a person (#1032)"

Rollback (quoted): "Findings and dismissals are append-only ledger records; a revert stops reading them, and Approve
to build returns to the stub's no-findings state, which is today's behaviour. No record is rewritten."

Path differences to settle at build time: the design names `internal/critique`; #1114 says it "writes
critique.State in internal/workitems/critique". The design hangs `--resolve` under `aayu grill feature`; #1115 puts
it on a new verb, `aayu critique <f> --resolve`.

## Layers touched

L02 (`internal/cli/grill.go`, `internal/cli/critique.go`, `internal/cli/readyapprove.go`), L03 (the critique
package, `skills/grill/`), L05 (`evals/`), L07 (`internal/config/`, `specs/<f>/resolutions.jsonl`). Layer paths from
`LAYERS.md`.

## What exists on `dev` that this Feature builds on

| Piece | Code | From |
|---|---|---|
| The hook Approve to build reads | `Env.CritiqueState` (`internal/cli/env.go`); `Env.critique` returns `ready.Critique{}` when it is nil (`internal/cli/readyapprove.go`) | [#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995), PR [#1024](https://github.com/rahulnakmol/agentic-ayudha/issues/1024) |
| Refusal on open gaps and on missing reviews | `approveRun.prepare` uses `ready.OpenGaps(cs.Open)` and `[critique] review_wait_hours` (`internal/cli/readyapprove.go`) | #995 |
| Dismissal record in the approval | `ready.Dismissal{Finding, Reason, Words}`, `Approval.Dismissals`, `ready.Ratified` (`internal/ready/approve.go`) | #995 |
| Fixes in a new version | `Baseline.Fixes`, inside `DraftHash`; `partRun.fixes` checks each ID against the review record on the ledger (`internal/ready/baseline.go`, `internal/cli/ready.go`) | [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) |
| Finding IDs and review path | `ready.Finding`, `ready.ReviewPath`, `ready.Review` (`internal/ready/drift.go`) | #993 |
| Owner of `[critique]` | `critique` in `[autonomy] always_person_sections` (`internal/config/defaults.toml`) | PR [#1032](https://github.com/rahulnakmol/agentic-ayudha/issues/1032), [42bb6e8](https://github.com/rahulnakmol/agentic-ayudha/commit/42bb6e8) |

## Planned, not built

| Story | Code to add | Records | Config |
|---|---|---|---|
| [#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114) | `critique.State` in `internal/workitems/critique`: reads `specs/<f>/reviews/<p>-v<k>/<seat>.json` and each approved version's `Fixes`; one wiring line that sets `Env.CritiqueState` (in `readyapprove.go` or `env.go`); `--findings` on `aayu grill feature` in `internal/cli/grill.go` | reads the review records | none |
| [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) | new verb `critique` with `--resolve` only, on the CLI only (`internal/verbs/registry.go`, `internal/cli/critique.go`); human-only, with the presence check Approve to build uses; `critique.State` returns the dismissals as `ready.Dismissal` | appends `specs/<f>/resolutions.jsonl` (finding, as, reason, by, at, presence) | `[critique] reason_max_chars = 500` |
| [#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116) | "Review findings" section in `skills/grill/SKILL.md`; eval case `grill/findings-walk` under `evals/cases/grill/` and its entry in `evals/eval.toml` | none | none |

## Key decisions and inputs

- Q18 (A): findings are resolved in a grill session with the founder in the agent thread; inputs and guidance never
  block (#1114, #1116 Open questions).
- @rahulnakmol, 2026-10-26: a dismissal is a person's own step with a reason, so `--resolve --as dismissed` is
  human-only and takes no `--words` (#1115 Open questions).
- Core design T25: a dismissal never goes through MCP (#1115 Assumptions). Not checked against the core design,
  which is private (unverified).
- No Decision issue is linked from #876 or its Stories.
