# Technical design: Feature #982

## The approved technical design

Tech v1 was approved by @rahulnakmol on 2026-11-02T17:26:27Z through the device flow. It pins `specs/982/design.json`
(hash `fead5ccf…`) (`specs/982/baselines/tech-v1.json`). The design record, planned 2026-10-31 by `human-fallback`:

- Footprint and layers: L03 only (`specs/982/design.json`, `footprint`, `layers`).
- Story order: 984, 985, 986, 987, 988, 989.
- Contracts, quoted:
  1. "the Feature form gains an Outcome section: who it helps, the Goal or Initiative metric, the expected change, and
     whether it serves the core competency; outcome:none with a reason for work with no customer outcome; aayu grill
     feature checks it (#984)"
  2. "Approve to build shows the Feature's outcome next to its budget (#985)"
  3. "an outcome card [outcomes] readout_days after a Feature's last story merges reads aayu measure and linked
     Feedback, and records moved, no change, or unknown; unknown never blocks (#986)"
  4. "releases and the weekly retro show outcome counts beside output counts (#987)"
  5. "the Goal form gains What we do best, which triage and grill ask about (#988)"
  6. "#989 (founder minutes per merged PR) is merged"
- Rollback: "Form sections are optional on read, so a revert stops asking for them and old issues still parse.
  Outcome cards are append-only ledger records that a revert stops writing. No record is rewritten."

The L03-only footprint is narrower than the expected paths: `internal/cli/*.go` is L02, `internal/config/**` is L07,
`CONSTRAINTS.md` is L04, and `.github/ISSUE_TEMPLATE/*` is L09 (`LAYERS.md`;
[#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Expected paths). #989 itself was pinned with L02,
L03, L04 and L07 (`spec-v1.json`).

## Delivered: founder minutes per merged PR (#989)

Merged in PR [#1016](https://github.com/rahulnakmol/agentic-ayudha/issues/1016) as
[2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53) on 2026-10-25 (`_data/merge-refs.tsv`).

| Part | Code on dev | What it does |
|---|---|---|
| Metric name | `measure.FounderMetric = "founder_minutes_per_pr"` in `internal/measure/founder.go` | `aayu measure --metric aayu:founder_minutes_per_pr` |
| Calculation | `measure.Founder(records, merged, w, o, dev)` in `internal/measure/founder.go` | Keeps `identity: human` records in the window, sorts them, splits sessions on a gap over `SessionGapMinutes`, adds `ActionMinutes` per session, divides by merged PRs, rounds to 0.1 |
| Data read | `founderMinutes` in `internal/cli/measure.go` | Reads every `actions/*.jsonl` segment on the ledger tip, and lists closed PRs with base dev, counting `merged_at` in the window |
| Dispatch | `case "aayu":` in the measure run in `internal/cli/measure.go` | Routes the `aayu:` source to `founderMinutes` |
| Config | `config.Outcomes` (`SessionGapMinutes`, `ActionMinutes`) in `internal/config/config.go`; `[outcomes]` in `internal/config/defaults.toml` | Defaults 10 and 2 |
| Generated rows | `internal/config/gen/main.go` rows `outcomes.session_gap_minutes`, `outcomes.action_minutes`; `CONSTRAINTS.md` lines for both | Docs of the keys |
| Flag help | `internal/verbs/registry.go`, the `metric` flag of `measure` | Names `aayu:founder_minutes_per_pr (from the actions log on the ledger)` |
| Tests | `TestFounderMinutesPerPR`, `TestFounderSessionGapBoundary`, `TestFounderNoMergedPR` (`internal/measure/founder_test.go`); `TestMeasureFounderMinutesPerPR` (`internal/cli/measure_test.go`) | Mapped to criteria 1–4 in `.aayudha/evidence/989.toml`. Both packages' tests pass on 884d81b, whose measure code is the same as on bc78349 (`go test -run Founder ./internal/measure/`, `go test -run TestMeasureFounderMinutesPerPR ./internal/cli/`, run 2026-11-07) |

Proof and ship: #989 merged by a Solo-mode bypass, not through `aayu ship`. Its proof pack exists:

- The merge gate posted "Proof pack: proven" for head `dafa588` on PR #1016 (2026-10-25T16:03:45Z). `aayu ship`
  posted "Ready to ship … its pack verifies on `dafa588`" and stopped for the human pass, because the PR touches
  `internal/config/`, in `[autonomy] always_person` (2026-10-25T17:30:46Z). @rahulnakmol then merged it after a
  "Solo-mode bypass:" comment (17:55:51Z). So there is no ship record.
- The post-gate audit comment on [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989)
  (2026-10-25T17:57:24Z) says "no authoritative pack exists for it on the ledger … It is done but unproven", and
  `aayu audit --mark-merged` set `status:merged`. That was true when it was written. On 2026-10-30, the pack from
  merge-gate run 40213240191 (`created_at` 2026-10-25T16:58:00Z) landed on the ledger: ledger commit `cb814b7`,
  `proof/989/dafa5887fd0579c49bf8902ec704e4171e54d915/authoritative/pack.json`, writer "rahulnakmol (aayu proof
  land)". Whether `aayu audit` now counts #989 as covered is unverified (not run).
- The issue is still open (GitHub, read 2026-11-07).

## Planned, not built (#984 to #988)

| Story | Planned change | Files (from expected paths and the Story) | Evidence that it is not built |
|---|---|---|---|
| [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) | Outcome section with Who, Metric, Change, Core; `outcome:none` with Reason; grill check; metric must be in the parent chain | `work-items/forms/aayudha-feature.yml`, `.github/ISSUE_TEMPLATE/aayudha-feature.yml`, `internal/grill/**`, `internal/cli/grill.go`, `internal/workitems` (inferred: the parser) | The Feature form has no Outcome field; no `outcome:none` label exists in the repository (`gh label list --search outcome` is empty) |
| [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) | Outcome line in the Approve to build text and card; refusal when missing | `internal/ready/**`, `internal/cli/ready.go`, `internal/cli/inboxapprove.go` (inferred: `approveItem`) | No Outcome line in `internal/ready` or `internal/cli/ready*.go` (the only `Outcome` there is a device-flow test field) |
| [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) | "Did it move?" card as a Need kind in [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873)'s registry; outcome record on the ledger; `[outcomes] readout_days = 14` | `internal/cli/inbox.go`, `internal/measure/**`, `internal/config/defaults.toml`, `internal/config/gen` | No `readout_days` key anywhere in the code |
| [#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987) | Outcome counts in `aayu release`, a table in `aayu retro`, a new `aayu retro --week` | `internal/cli/release.go`, `internal/cli/retro.go`, `internal/verbs/registry.go` (the `--week` flag) | `retro` has no `week` flag (`internal/verbs/registry.go`, `Name: "retro"`) |
| [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988) | "What we do best" on the Goal form; the question in grill and triage | `work-items/forms/aayudha-goal.yml`, `.github/ISSUE_TEMPLATE/aayudha-goal.yml`, `internal/grill/**`, `internal/triage/**`, `skills/grill/SKILL.md` | The Goal form has Summary, Beneficiaries, Metric, Baseline, Target (`work-items/forms/aayudha-goal.yml`) |

Note: `internal/grill/gate.go` has `ReasonOutcome = "outcome"` today. That is the vagueness gate's "no observable
result", not a customer outcome. A new check must not reuse the name in a confusing way (inferred).

## Data and ledger records

- Built: the measure reads the `actions/` segments on the `ledger` branch (`founderMinutes`), and writes the Result
  into the Goal or Initiative body through `writeResult` (called in `internal/cli/measure.go`, defined in
  `internal/cli/experiment.go`).
- Planned: one append-only outcome record per Feature on the ledger: the answer, the person, the time, and the line
  ([#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) c2; `specs/982/design.json`, rollback). The path
  is not named in any source; unverified.

## Config keys

| Key | Default | State |
|---|---|---|
| `[outcomes] session_gap_minutes` | 10 | built ([2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53)) |
| `[outcomes] action_minutes` | 2 | built ([2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53)) |
| `[outcomes] readout_days` | 14 | planned, needs a row in `internal/config/gen` ([#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) Assumptions) |

## Key decisions and dependencies

- Decision [#981](https://github.com/rahulnakmol/agentic-ayudha/issues/981), closed `status:done`: option 1 accepted
  on 2026-10-24, marked a one-way door (comments on #981).
- Feature depends on [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) (Need registry) and
  [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) (Approve to build) (#982 Links). Both are open and
  planned; #877 has 2 of 6 sub-issues merged or done (`_data/subs.tsv`).
- Web parts depend on Feature [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976), which is not built:
  #985 c4 (web card), #987 c3 (Releases page and Tree), #987 c4 (History view). The Story Links do not name #976.
- [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903): a Story added after Approve to build, or a changed
  Completion Goal, needs the founder's new acknowledgment.
