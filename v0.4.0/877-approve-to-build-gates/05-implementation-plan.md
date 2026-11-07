# Implementation plan: Feature #877

## All Stories and Bugs

Labels from `_data/subs.tsv`; merges from `_data/merge-refs.tsv`; no open PR in `_data/open-prs.tsv` refs these
issues.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995) | A person approves a Feature to build, and the ledger keeps what was agreed | story | merged | M | high | none | L02, L03, L07: `internal/cli/readyapprove.go`, `internal/ready/approve.go`, `internal/cli/ready.go`, `internal/config/` | PR [#1024](https://github.com/rahulnakmol/agentic-ayudha/issues/1024), [45a18e7](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e7), 2026-10-26 (merged without `aayu ship`: no pack, per the post-gate audit comment on #995); fix PR [#1032](https://github.com/rahulnakmol/agentic-ayudha/issues/1032), [42bb6e8](https://github.com/rahulnakmol/agentic-ayudha/commit/42bb6e8), 2026-10-26 (pack `proof/995/7b7f8bc…/authoritative/pack.json`, solo-mode bypass) |
| [#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996) | A story is Ready only while its Feature's approval covers it | story | merged | M | high | #995 | L02, L03, L06: `internal/cli/readycover.go`, `internal/ready/cover.go`, `internal/ready/ready.go` | PR [#1026](https://github.com/rahulnakmol/agentic-ayudha/issues/1026), [52f8b96](https://github.com/rahulnakmol/agentic-ayudha/commit/52f8b96), 2026-10-26 (merged on proof, Decision #593) |
| [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) | Approve to build shows the high-risk changes by story, and records a choice for each | story | planned | M | high | #995, #1028, #1083 | `internal/cli/readyapprove.go`, `internal/ready/approve.go`, `internal/verbs/registry.go` (`--choice`) | not started |
| [#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082) | A story added or changed after Approve to build needs its own acknowledgment | story | planned | M | high | #995, #996 | `internal/cli/ready.go`, `internal/cli/readycover.go`, `internal/ready/`, `internal/verbs/registry.go` (`--ack`) | not started |
| [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083) | An approval record counts only with a receipt that GitHub shows its person wrote (threat T23) | story | planned | L | high | #994, #995, #996, #1082 | `internal/cli/presence.go`, `readyapprove.go`, `readycover.go`, `ready_early.go`, `internal/ready/`, `internal/audit/`, `internal/config/`, `docs/threat-model.md` | not started |
| [#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086) | aayu grill story files a Story with no parent Feature, which can never be made Ready | bug | planned | S | medium | none | `internal/cli/grill.go`, `skills/grill/SKILL.md`, grill eval cases | not started |

Related work merged under other Features that changes this Feature's code:

| Issue | What | Commit |
|---|---|---|
| Story #997 (Feature #889) | Approve to build writes the mission plan | [d941d15](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d15), PR #1084 |
| Story #1013 (Feature #889) | Start locks the Feature's bodies and labels | [3d42ff1](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff1), PR #1096 |
| Bug #1182 (Feature #1170) | A shell with only a host's agent marker approves to build with the device code | [c64bccf](https://github.com/rahulnakmol/agentic-ayudha/commit/c64bccf), PR #1188 |
| Story #1173 (Feature #1170) | Approve to build is an inbox card | [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b), PR #1225 |

## Delivered

- Approve to build. A person runs `aayu ready <feature>`; aayu checks the conditions, shows the effect, runs the
  device flow, reads everything again, writes `specs/<f>/approved.json` and `missions/<f>/plan.json` in one ledger
  commit, and Arjun posts the Feature's Ready snapshot (`approveToBuild`, `approveRun.prepare`,
  `approveRun.ready` in `internal/cli/readyapprove.go`).
- Story coverage at Ready (`storyCovered`, `earlyCovered`, `featureChanges` in `internal/cli/readycover.go`), with
  the snapshot row "Approved to build", and no change to the guard or the build pick (F4).
- Used for real on two Features: `specs/874/approved.json`, version 1, by `rahulnakmol` through `device-flow` with
  permission `admin`, 2026-11-02T17:27:21Z, 7 stories, budget $420, dismissals empty; and
  `specs/1170/approved.json`, version 3, by `rahulnakmol`, 2026-11-02T17:45:16Z, 10 stories, budget $600 (the
  mission of #1170 was ended on 2026-11-07, `missions/1170/plan.json`).

## In review

None.

## Pending

- Bug #1086 can start now: it has no depends-on, and the current code still shows the problem
  (`skills/grill/SKILL.md` line 383; `internal/workitems/schema.go` Story type with no `ParentAt`).
- #1082 can start next: its dependencies #995 and #996 are merged.
- #1083 waits for #1082. #1029 waits for #1083 and for #1028 (Feature #874, not built).
- Approve to build of #877 itself: a person's step; no `specs/877/approved.json`. Under the code today, the
  approval would list the five pinned stories and a budget of 5 × $60 = $300, because `approveRun.prepare` counts
  child stories only, not Bugs (computed from the code, not from a record).
- Bug #1086 is not pinned in the spec baseline, so it does not need a new spec version. Whether a Bug under a
  Feature needs Approve to build coverage to be Ready is not in #996's rule, which covers Stories only (#1086
  Notes: "A Bug keeps an optional parent: #996 covers Stories only").

### Work not sliced into Stories

Items in [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) Scope with no Story or Bug under #877:

1. One Tatkal Feature per Release, approved once with `[budget] unplanned_usd`, whose stories need no new
   approval or acknowledgment and move to the next Release's Tatkal. On `dev`, `Env.TatkalRelease` is a hook that
   returns 0 unless a test sets it (`Env.tatkalRelease` in `internal/cli/mission.go`), and `unplanned_usd` is not
   in `internal/config/defaults.toml`.
2. A Tatkal story with a high-risk change waits for a person's review of its PR (Decision #1027).
3. The Replaced card for a Feature. `InboxReplaced` exists for "an open Story or Bug that another issue replaces"
   (`internal/cli/inbox.go`); a Feature card is not checked here (unverified).
4. The Ready snapshot's read-only notice, and the guard that puts back a locked body and its size, risk, and
   priority labels and keeps the change as a comment (Decision #974). The lock itself shipped in #1013; the
   guard's restore was not checked here (unverified).
5. An approved story that counts as asked (`internal/cli/asked.go`); not checked here (unverified).
6. Build and the Ready guard reading the ledger, not a comment (F2). Delivered for `aayu ready <story>`; not
   checked for `internal/cli/build.go` and the guard (unverified).

No source gives an estimate for these items.
