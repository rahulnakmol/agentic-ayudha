# Goal and intent: Feature #889

## Goal

The approved intent baseline (`specs/889/baselines/intent-v1.json` on `origin/ledger`, approved by @rahulnakmol on
2026-11-02T17:23:07Z through the device flow) says:

> `aayu build --next --dispatch` fills every free slot by handing each story to a new thread, and `ship` dispatches
> again after each merge. This is step S20 of the core design (aayu core: the human-dependency engine, agreed by
> @rahulnakmol on 2026-10-21).

The baseline pins the hash of the #889 body (`b52f2240…`), and its summary is the body's Summary word for word
([#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889)). The core design it names is Initiative
[#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) "aayu core: the human-dependency engine" (title
match; the parent link is not checked here, unverified).

The intent was baselined on 2026-10-30 and approved on 2026-11-02. Most of the Feature's code merged between
2026-10-26 and 2026-11-03 (`_data/merge-refs.tsv`). So the baseline was approved after most of the work it covers
was built.

How the goal is met on `dev` is different from the words of the summary. There is no `--dispatch` flag
(`aayu build --help` at `884d81b`). A person starts a mission (`aayu mission start <f>`), and the mission's Kanha
starts one thread per story with `aayu build <n> --delegate <tool>` (`KanhaBrief`, `internal/delegate/brief.go`).
`ship` tells the Kanha after each merge, in the same process ([#1007](https://github.com/rahulnakmol/agentic-ayudha/issues/1007),
`tellKanha` call in `internal/cli/ship.go`).

## Customer outcome and who it serves

- The customer is the founder, @rahulnakmol. Each story under #889 starts "As @rahulnakmol, I want …" or "As the
  founder, …" (for example [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010),
  [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011),
  [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030)).
- The outcome: an approved Feature builds to `dev` with many stories in parallel, each in its own agent thread, and
  the person is asked to act only where a person must. #1011's Scenario says: "Ten stories build across two
  Features. @rahulnakmol gets a page at each wave end and each block, and nothing else."
- Teammates are a second audience. Several criteria cover a second person (@alice, @bob, @dana) who stops, resumes,
  or retries a mission that another person started on another tool
  ([#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002) criteria 12-16,
  [#1003](https://github.com/rahulnakmol/agentic-ayudha/issues/1003) criteria 8-9).

## Problem

The #889 body says: "The loop builds one story per run in its own session, every 2 hours
(`loops/build-ready.toml`) (G18, E5)." G18 and E5 are labels in the core design; the design itself is not in the
public repository (unverified).

## Scope as the Feature states it

The #889 body lists "In": dispatch through the delegate adapters; dispatch on merge, block, stop, and a failed
hand-off; an hourly tick as the safety net; the Feature parent in `approved.json`; events in
`delivery/<f>/events.jsonl`; and a parent that wakes only at a wave end, a block or an ask, and the Feature's end. It
notes that `loops/build-ready.toml` is in the narrow set, so its PR needs a person to ship.

It also depends on two other Features: `depends-on #886` (Approve to build learns time and cost) and
`depends-on #881` (the answer reaches the asking session). Both are still OPEN and `status:planned`
([#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886),
[#881](https://github.com/rahulnakmol/agentic-ayudha/issues/881)).

## Success measures

The Feature body states no measure. Its stories state these (Measure and Threshold sections):

| Story | Measure | Threshold |
|---|---|---|
| [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) | Story events with no line in `delivery/<f>/events.jsonl` | 0 |
| [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011) | Pages to @rahulnakmol per Feature with 7 stories | About 5, never one per event |
| [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) | Comments that change a spec, a behaviour, or a definition of done and get no story, per mission | 0 |
| [#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126) | Events for a local Kanha that waited more than one check while its machine was on and the inbox was closed | 0 |
| [#1154](https://github.com/rahulnakmol/agentic-ayudha/issues/1154) | Seconds `go test` reports for `internal/cli` in the CI `build` job of dev | 300 s or less |
| [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030), [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104), [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109), [#1134](https://github.com/rahulnakmol/agentic-ayudha/issues/1134) | Completion Goal criteria observed | All criteria |

## Out of scope

The #889 body has no "Out" list. These limits come from the sources:

- The four learning stories are v0.4.1 work: red runs in proof packs, real-run journeys, function beyond the ask, and
  milestone findings that order the next milestone (`specs/889/design.json`, `contracts`, last entry; milestone
  v0.4.1 on [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) to
  [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112)).
- Tier-2 eval results as merge evidence on a PR head: Bug
  [#1157](https://github.com/rahulnakmol/agentic-ayudha/issues/1157) is closed as not planned, because "the
  ai-gateway environment allows only dev, rolling, and main" (comment by @rahulnakmol, 2026-10-30). Bug
  [#1163](https://github.com/rahulnakmol/agentic-ayudha/issues/1163) replaced it.
- The Kanha never merges, approves, ships, or skips a gate (`KanhaBrief`, `internal/delegate/brief.go`, Rules;
  `skills/mission/SKILL.md`, "Kanha's protocol").
- A mission does not change other missions or loops: a stop leaves them running
  ([#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002) criterion 4).
