# Implementation plan: Feature #874

## All Stories and Bugs

Labels from `_data/subs.tsv`; merges from `_data/merge-refs.tsv`; open PRs from `_data/open-prs.tsv` (none for
this Feature).

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#990](https://github.com/rahulnakmol/agentic-ayudha/issues/990) | An agent drafts a Feature baseline, and only a person approves it at the hash they saw | story | merged | L | medium | none | L02, L03: `internal/cli/ready.go`, `internal/ready/baseline.go` | PR [#1018](https://github.com/rahulnakmol/agentic-ayudha/issues/1018), [af81cb4](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb4), 2026-10-25 |
| [#991](https://github.com/rahulnakmol/agentic-ayudha/issues/991) | aayu plan writes a Feature's technical design, and each story plan stays inside it | story | merged | M | medium | none | L02, L03: `internal/cli/plan.go`, `internal/planning/design.go` | PR [#1017](https://github.com/rahulnakmol/agentic-ayudha/issues/1017), [0b4e9b3](https://github.com/rahulnakmol/agentic-ayudha/commit/0b4e9b3); fix PR [#1020](https://github.com/rahulnakmol/agentic-ayudha/issues/1020), [b220fa0](https://github.com/rahulnakmol/agentic-ayudha/commit/b220fa0), 2026-10-25 |
| [#992](https://github.com/rahulnakmol/agentic-ayudha/issues/992) | Each baseline part pins what it owns: the spec's stories, the technical design, and the UX stages | story | merged | M | low | #990, #991 | L02: `internal/ready/pins.go`, `internal/cli/ready.go` | PR [#1022](https://github.com/rahulnakmol/agentic-ayudha/issues/1022), [de79988](https://github.com/rahulnakmol/agentic-ayudha/commit/de79988), 2026-10-25 |
| [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) | A drift after a baseline shows at once, and a new version says what changed, why it matters, and the need behind it | story | merged | M | high | #992 | L02, L03, L07: `internal/ready/drift.go`, `internal/config/` | PR [#1023](https://github.com/rahulnakmol/agentic-ayudha/issues/1023), [118408f](https://github.com/rahulnakmol/agentic-ayudha/commit/118408f); fix PR [#1025](https://github.com/rahulnakmol/agentic-ayudha/issues/1025), [4aac162](https://github.com/rahulnakmol/agentic-ayudha/commit/4aac162), 2026-10-26 |
| [#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994) | A person approves one early build before Approve to build, and sees its budget | story | merged | S | high | #990 | L02, L03, L07: `internal/ready/early.go`, `internal/cli/ready_early.go`, `inspect.go`, `ship.go` | PR [#1021](https://github.com/rahulnakmol/agentic-ayudha/issues/1021), [9362979](https://github.com/rahulnakmol/agentic-ayudha/commit/9362979), 2026-10-26 |
| [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) | Each story plan lists its high-risk changes in a risk register | story | planned | M | medium | none (spec pin); Decision #1027 | `internal/planning/plan.go` (`CheckPlan`), `internal/cli/plan.go` | not started |
| [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106) | A state-changing high-risk story plans its failure modes, and its open questions age like asks | story | planned (milestone v0.4.1) | M | low | #1028, #883 | `internal/planning/plan.go`, `internal/cli/plan.go`, `internal/cli/ready.go`, `internal/cli/ask.go`, `internal/config/` | not started |

## Delivered

What works on `dev` (884d81b) now:

- Draft and approve a Feature part. An agent runs `aayu ready <f> --part <p> --request`; a person runs
  `aayu ready <f> --part <p>` with the device code. Code: `runReadyPart`, `partRun.request`, `partRun.approve` in
  `internal/cli/ready.go` ([af81cb4](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb4)).
- `aayu plan <feature>` writes `specs/<f>/design.json`, and story plans stay inside its layers (`planFeature`,
  `featureDesign` in `internal/cli/plan.go`; `CheckDesign`, `CheckInDesign` in `internal/planning/design.go`)
  ([0b4e9b3](https://github.com/rahulnakmol/agentic-ayudha/commit/0b4e9b3),
  [b220fa0](https://github.com/rahulnakmol/agentic-ayudha/commit/b220fa0)).
- Per-part pins (`internal/ready/pins.go`) ([de79988](https://github.com/rahulnakmol/agentic-ayudha/commit/de79988)).
- Drift in `--check`, reasons for new versions, `--fixes`, and the `[baselines]` keys (`internal/ready/drift.go`)
  ([118408f](https://github.com/rahulnakmol/agentic-ayudha/commit/118408f),
  [4aac162](https://github.com/rahulnakmol/agentic-ayudha/commit/4aac162)).
- Early-build approval, with the inspect hold and the ship refusal (`internal/ready/early.go`,
  `internal/cli/ready_early.go`) ([9362979](https://github.com/rahulnakmol/agentic-ayudha/commit/9362979)).

Use on the real repository: Features #874, #875, #876, and #877 each have intent, spec, and tech v1 approved by
`rahulnakmol` through `device-flow` on `ledger` (`specs/<f>/baselines/*-v1.json`). #874 also has
`specs/874/approved.json` (version 1, 2026-11-02T17:27:21Z, 7 stories, budget $420 = 7 × $60) and
`missions/874/plan.json` with milestone MS1 holding all seven stories and its gate in state `waits`.

## In review

None. `_data/open-prs.tsv` lists no open PR that refs #874 or its Stories.

## Pending

- [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028): the risk register. Not started; no `risks`
  field in `internal/planning/plan.go`. Two other Stories wait on it:
  [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) (Feature #877) and #1106. The inbox Approve to
  build card says "High-risk entries by story: none shown. They come with #1029, which has not merged"
  (`internal/cli/inboxapprove.go`).
- [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106): failure modes and open questions. Not
  started. It waits on #1028 and on Feature [#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883)
  (v0.4.1, `status:planned`). CG 7 also files asks through Feature
  [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) (`status:planned`); `dev` has no
  `internal/cli/ask.go` and no `ask` verb in `internal/verbs/registry.go`. `missions/874/plan.json` puts it in
  mission 2 with "waits on #883". Its own
  milestone is v0.4.1, while its Feature is in v0.4.0; Approve to build approval 1 of #874 still lists it.
- Mission Start of #874: a person's step. `missions/874/plan.json` has no `started_by` field. The brief for this
  document says the Start card waits in the founder's inbox; I did not open `aayu inbox` to check (unverified).
- Work not sliced into Stories: none found. The Feature's Scope items (`--part`, `--request`, the baseline
  records, `aayu plan <feature>`, UX pins, `--early-build`) each map to a merged Story above. No source gives an
  estimate for further work.
