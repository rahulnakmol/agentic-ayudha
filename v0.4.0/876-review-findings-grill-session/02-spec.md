# Spec: Feature #876

## Spec baseline

`specs/876/baselines/spec-v1.json` on `ledger`: owner `product-strategist`, approved by `rahulnakmol` at
2026-11-02T17:18:33Z through `device-flow`, hash `5f69d4833129…`. It pins three stories and the set
`#876 stories`:

| Story | Effort | Layers (pinned) | Depends on (pinned) |
|---|---|---|---|
| [#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114) | M | L02, L03 | none |
| [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) | S | L02, L03, L07 | #1114 |
| [#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116) | S | L03, L05 | #1114, #1115 |

All requirements below are not built.

## Functional requirements

| # | Requirement | Story, CG |
|---|---|---|
| FR1 | `aayu ready <f>` exits 4 before any device code while a gap is open: "Approve to build is off: technical design v1 has 1 open gap (tech-v1/production-engineer/1). Next: aayu grill feature 840 --findings". Nothing is written. | #1114 CG 1 |
| FR2 | A gap closes only when a later version of its own part, approved by a person, names it in `--fixes`. A draft that names it does not close it. | #1114 CG 2, 3 |
| FR3 | A gap from any seat blocks, the Mentalist's included. Inputs and guidance never block; no review record means no block. | #1114 CG 4, 5 |
| FR4 | A review record that cannot be read keeps Approve to build off and names the file. | #1114 CG 6 |
| FR5 | `aayu grill feature <f> --findings` prints a header per part version (reviews in, gaps, inputs, guidance), then one line per finding (ID, kind, severity, seat, applies_to, state: open, fixed in v&lt;k&gt;, or dismissed), gaps first, then the two next commands. It writes nothing. | #1114 CG 7 |
| FR6 | `--findings --json` gives one object per part version with `reviews_in`, `findings`, and `open_gaps`. With no records: "#840 has no seat reviews yet", exit 0. | #1114 CG 8, 9 |
| FR7 | `aayu critique <f> --resolve <id> --as dismissed --reason "<line>"` runs one device-flow check, then appends one line to `specs/<f>/resolutions.jsonl` (finding, as, reason, by, at, presence). | #1115 CG 1 |
| FR8 | It refuses with no write: no reason or only spaces (exit 2); an agent lane (exit 4); an unknown ID or an input (exit 2); a gap already dismissed or already fixed (exit 4); a reason longer than `[critique] reason_max_chars` (default 500, exit 2). | #1115 CG 2-5, 7 |
| FR9 | Approve to build shows "1 dismissal ratified: &lt;id&gt; (&lt;reason&gt;)" before the device code and writes the dismissal into `approved.json`. | #1115 CG 6 |
| FR10 | `skills/grill/SKILL.md` gains a "Review findings" section: run `--findings`, take each open gap one at a time with a recommendation, draft a fix with `aayu ready <f> --part <p> --request --fixes <id>`, and give the person the `--resolve` command to run, never running it. | #1116 CG 1 |
| FR11 | A new eval case `grill/findings-walk` passes at the grill group's bar: it drafts the fix for one gap, hands over the dismissal command with the person's reason for the other, runs no `--resolve`, and leaves an unanswered gap open. | #1116 CG 2, 3 |

The delivered code already does part of FR1 and FR9. `approveRun.prepare` refuses on `Critique.Open` with
`ready.OpenGaps`, and adds `ready.Ratified` to the effect lines (`internal/cli/readyapprove.go`). What is missing is
a real critique state that fills them (#1114 Assumptions; #1115 Assumptions).

## Non-functional requirements

| # | Requirement | Source |
|---|---|---|
| NFR1 | Security (authority): only a person dismisses a gap, through the device flow; an agent never does. A dismissal never goes through MCP (core design T25). | #1115 Behaviours and Assumptions; #1116 Behaviours |
| NFR2 | Integrity: a gap closes only on an approved later version or a dismissal record. An unreadable record fails closed. | #1114 Behaviours and Review guidance |
| NFR3 | Records are append-only on the ledger and never rewritten. | `specs/876/design.json` rollback |
| NFR4 | Limits are config keys: `[critique] reason_max_chars = 500`, owned by a person. | #1115 CG 7; `specs/876/design.json` contract 5 |
| NFR5 | Quality: the eval bars only rise (`evals/README.md`); the eval case fails a run that dismisses a gap itself. | #1116 Review guidance |
| NFR6 | Performance, cost, accessibility: no source states a requirement. | — |
