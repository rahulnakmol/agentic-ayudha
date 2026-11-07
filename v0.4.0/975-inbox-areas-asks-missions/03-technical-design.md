# Technical design: Feature #975

## Source note

There is no technical design baseline (`origin/ledger` has no `specs/975/`). The planned design comes from the
Feature's "Expected paths" and the Stories' Assumptions. "Today on dev" parts cite code at origin/dev
[bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) (884d81b plus #1238; line numbers checked
2026-11-07). No commit since v0.3.0 refs a sub-issue of #975 (`_data/merge-refs.tsv`).

## Architecture

The inbox architecture is the one in
[../895-inbox-ship-review-cards/03-technical-design.md](../895-inbox-ship-review-cards/03-technical-design.md):
`inboxSource` builds `InboxItem` values, `inboxKinds` and `inboxOrder` fix each kind's words and order, and the
Bubble Tea `inboxModel` draws and acts (`internal/cli/inbox.go`, `internal/cli/inboxtui.go`).

Wave 2 adds two things on top (issue #975, Summary and Scope):

1. A Need registry with areas. Today the order is one flat list of kinds, `inboxOrder`, set by Story #1175 for the
   morning's decisions ([2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2);
   `internal/cli/inbox.go:69`). #937 replaces it with areas that come from "the Need registry's areas (#873)" (#937
   Assumptions). #873 has no sub-issues (GitHub API), so where the registry lives is not stated (unverified).
2. Cards over core engines. Each Story says it is "the card" or "the inbox face", and a core Feature owns the verb
   and its rules (for example #942: "#889 owns the mission record, the verbs … this story is the inbox face"; #947:
   "the approval verb and its rules are #888's; this story is the card").

## Layers touched

From `LAYERS.md` and the Expected paths in issue #975:

| Layer | Owner seat | Paths |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/inbox.go`, `inboxtui.go`, `inboxwork.go`, `mission.go`, `build.go`, `delegate.go`, `halt.go`, `decide.go`, `triage.go`, `release.go` |
| L05 Intelligence | Data & AI Architect | `internal/models/gateway.go` (the budget pause, `ErrBudget` and `Breaker`) |
| L03 Application / workflow | Engineering Manager | `internal/verbs/registry.go` for new flags (inferred: `aayu mission --release`, `aayu triage --decide`) |
| L07 Operational data / state | Production Engineer | `internal/config/defaults.toml` for `[inbox] loop_rounds` and `loop_hours` (#949 C7) |

## Core Features this one depends on

Issue #975 lists 19 depends-on links. State on 2026-11-07 (GitHub API `sub_issues_summary`, `_data/subs.tsv`):

| Feature | Title | Milestone | Sub-issues | Merged sub-issues | Needed by |
|---|---|---|---|---|---|
| [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895) | inbox wave 1 | v0.4.0 | 15 | 0 | all; #949 needs #924 and #936 |
| [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) | Every agent write names its host and session | v0.4.0 | 0 | — | #938, #941, #943 |
| [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872) | Each merged story adds one benchmark line to the ledger | v0.4.0 | 0 | — | #944, #956 |
| [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) | One Need registry, and every page names a kind | v0.4.0 | 0 | — | #937, #945, #946, #948, #949, #950 |
| [#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) | Feature baselines are drafted, approved, and pinned | v0.4.0 | 7 | 5 | #951 |
| [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) | Seats review each baseline in a fresh session | v0.4.0 | 5 | 0 | #952 |
| [#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) | Review findings are resolved in a grill session | v0.4.0 | 3 | 0 | #952 |
| [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) | Approve to build gates every story | v0.4.0 | 6 | 2 | #941, #945, #953, #955 |
| [#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879) | Bugs are triaged before Ready | v0.4.1 | 0 | — | #954 |
| [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) | Any seat asks through aayu ask, and the first answer wins | v0.4.0 | 0 | — | #938, #941 |
| [#881](https://github.com/rahulnakmol/agentic-ayudha/issues/881) | The answer reaches the asking session | v0.4.0 | 0 | — | #938 |
| [#882](https://github.com/rahulnakmol/agentic-ayudha/issues/882) | A build waits on a person without a reset | v0.4.0 | 0 | — | #938, #945 |
| [#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883) | Questions have an SLA, and overdue ones come first | v0.4.1 | 0 | — | #940 |
| [#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884) | Snooze, with others, and wake survive the session | v0.4.1 | 0 | — | #939 |
| [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) | Each item knows what it unblocks, and depends-on is enforced | v0.4.1 | 0 | — | #937, #944 |
| [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) | Approve to build learns time and cost from merged stories | v0.4.0 | 0 | — | #956 |
| [#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888) | Approve an ops action from the INCIDENT card | v0.4.1 | 0 | — | #947 |
| [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) | Agents pull the next story on merge, each in its own thread | v0.4.0 | 65 | 38 (`status:merged`) | #942, #943, #949 |
| [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) | Metered, risk-routed merge | v0.4.1 | 0 | — | #949 |

Six of these Features are in milestone v0.4.1 while #975 is in v0.4.0: #879, #883, #884, #885, #888, #897. So the
Stories that need them cannot finish in v0.4.0 as planned. The sources do not say how this was meant to resolve
(unverified).

## Today on dev, by Story

| Story | What exists on dev | Code | What is still missing |
|---|---|---|---|
| #937 | One flat kind order, decisions first | `inboxOrder` (`inbox.go:69`, Story #1175) | Areas, Now, Tab between areas, the unblocks line |
| #938, #941 | No `aayu ask` verb: `aayu ask` exits 2 with the top-level help | build of origin/dev | ANSWER and REFINE cards, `aayu ready <f> --ack` (not in `aayu ready --help`) |
| #939 | s skips a card only for this session: "Skipped #n for now; j and k come back to it." | `inboxtui.go:1394` | Snooze times, wake events, With others |
| #942 | A Start card that runs `aayu mission start <f>` | `missionlock.go:698` (Story #1013, [3d42ff1](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff1)) | Total threads, time, cost, and budget on the card; Stop, Resume, Recover, Retry from a card |
| #942 | Mission verbs `start`, `stop` (with `--story`, `--reason`, `--end`), `resume`, `recover`, `retry` | `aayu mission --help`; [33aa6bf](https://github.com/rahulnakmol/agentic-ayudha/commit/33aa6bf) (#1002), [bdd02c1](https://github.com/rahulnakmol/agentic-ayudha/commit/bdd02c1) (#1003) | — (core is there) |
| #942, #943 | A mission-time card with Extend and Stop | `missiontime.go:964` (Story #1015, [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b0)) | — |
| #943 | A gate-failed card with "Run the gate once more" and "Stop the mission, with a reason" | `missionask.go:54` (Story #1001, [3ece84e](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e)) | The mission view, Shape and Deliver, `aayu mission <f>` as a view, `--release` (not in `aayu mission --help`) |
| #945 | `aayu plan <story> --approve` and `--solo-bypass` | `aayu plan --help` | PLAN and BLOCKED cards |
| #946 | `aayu tell --publish`; `aayu release <n> --to <rolling or main>` | `aayu tell --help`, `aayu release --help` | RELEASE and PUBLISH cards |
| #948 | `aayu halt --reason` and `aayu resume`; the breaker's `ErrBudget` | `internal/cli/halt.go` (`runHalt`, `runResume`); `internal/models/gateway.go:86` | Emergency stop and RESUME cards, banners |
| #949 | `aayu decide --waiver <floor>` | `aayu decide --help` | OVERRIDE and LOOPING kinds, `[inbox] loop_*`, `aayu release --override-evals` |
| #951 | A BASELINE card per waiting draft | `inboxbaseline.go` (Story #1172, [c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80)) | FEATURE card, drift card, nudges. PR [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265) (Bug #1253) would have changed this card to one card per Feature; it closed unmerged on 2026-11-07 when the mission ended; branch `story/1253-aayu-inbox-asks-up-to-five-decisions-to` at 306d1a8 kept |
| #953 | An Approve to build card with stories, budget, agents, and milestones | `inboxapprove.go` (Story #1173, [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b)) | High-risk entries and their choices (also Story #1029 under #877, planned), the 48 h override |
| #954 | `aayu triage [issue] --as <feedback, bug, or incident>` | `aayu triage --help` | `--decide`, the TRIAGE card |
| #955 | None | — | `aayu ready <f> --ack`, the ACK card (also Story #1082 under #877, planned) |
| #940, #944, #947, #950, #952, #956 | None found | — | All of it |

## Planned design notes (planned, not built)

- New card kinds: answer, refine, plan-review, build-blocked, release, publish, incident, halt, paused, override,
  looping, feedback, feature, reviewed, ack, triage. The bodies name some of them as Need kinds: "kinds plan-review
  and build-blocked (#873)" (#945), "kinds halt … and paused" (#948), "kinds override and looping are new in the
  Need registry" (#949), "the release and publish Need kinds … this story adds them to the registry" (#946), "the
  feedback Need kind" (#950). The other names in this list are this record's labels from the card titles (inferred).
- Mission view: "#975 owns `aayu mission <feature>` and `--release` (the read-only view) and the Deliver tab; #889
  owns the record they read and the control verbs" (#943 Assumptions, J1).
- Stop inside a mission is not a halt: "Stop blocks the story inside its mission (#889), not a halt" (#949
  Assumptions); `aayu halt` is "only the emergency stop for a tool incident (threat model T18)" (#948 Assumptions).
- Release override: "`aayu release <n> --to rolling --override-evals --reason`, human-only, with a record in
  `releases/<v>/evals-override-<sha>.json`; one Tatkal story adds it (not filed yet)" (#949 Assumptions).
- Ack record: the read or delegated mode in `specs/<feature>/ack-<k>.json`, kept by #877 (#955 Assumptions).
- Loop limits: "default to the 3 fix rounds of Decision #905 and to `[autonomy.budget] wall_hours` (8)" (#949
  Assumptions).

## Data and ledger records

| Record | Use | Source |
|---|---|---|
| Ask records | Answer within 24 h measure | #938 Measure |
| `specs/<feature>/ack-<k>.json` | Acknowledgment with mode | #955 Assumptions |
| `releases/<v>/evals-override-<sha>.json` | Evals override | #949 Assumptions |
| Mission record and plan (`missions/<f>/plan.json`) | Mission view | #943 Assumptions; `aayu ready --help` (`--milestones`) |
| `specs/<f>/approved.json` | Approve to build | `aayu ready --help` (`--agents`) |
| Benchmark lines on the ledger | Delivery plan and timeline | #944, #956 Assumptions (#872) |

## Config keys

| Key | Default | Status | Source |
|---|---|---|---|
| `[decisions] sla_hours` | 24 | exists | `defaults.toml` |
| `[decisions] provisional_after_hours` | 48 | exists | `defaults.toml` |
| `[decisions] solo_provisional_after_hours` | 24 | exists | `defaults.toml` |
| `[time] business_hours` | "Mon-Fri 09:00-18:00" | exists | `defaults.toml` |
| `[build] agents` | 5 | exists | `defaults.toml` |
| `[mission] warn_hours`, `max_hours` | 24, 48 | exist | `defaults.toml` |
| `[inbox] loop_rounds`, `loop_hours` | 3, 8 | planned | #949 C7 |
| `[repos."<owner>/<repo>"] tool` (user config) | — | not checked | #942 C1 |

## Key decisions

| Decision | State | Effect here |
|---|---|---|
| [#913](https://github.com/rahulnakmol/agentic-ayudha/issues/913) Feature missions | closed, superseded by [#915](https://github.com/rahulnakmol/agentic-ayudha/issues/915) (`status:done`) | #942, #943, #948 cite #913 and its letters M1 to M11; check them against #915 before Ready |
| [#905](https://github.com/rahulnakmol/agentic-ayudha/issues/905) Review speed | closed, superseded by [#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910) (`status:done`) | #949 bases `loop_rounds` on #905's 3 fix rounds |
| [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903) Approve to build agrees the whole Feature once | `status:done` | #941, #955 |
| [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862) Agent questions follow the Decision SLA | `status:done` | #940 |
| [#974](https://github.com/rahulnakmol/agentic-ayudha/issues/974) Mission rules and the 48-hour limit | `status:done` | The estimate basis in issue #975 |

## Overlap with Feature #1170

Feature [#1170](https://github.com/rahulnakmol/agentic-ayudha/issues/1170) "The founder's day" built inbox cards that #975 also describes (BASELINE, Approve to build, decision order). Its body puts "the areas view and asks from any seat
(#975)" out of its scope (issue #1170, Scope). The areas of #937 and the decision order of #1175 both set the card
order. The sources do not say which one wins (unverified; a person should decide before #937 is made Ready).
