# aayu v0.4.0 and v0.4.1: what was planned, what was built

This orphan branch holds documentation only. It records every Feature of milestone v0.4.0 ([Release #906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)) and milestone v0.4.1 ([Release #1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)): its goal and intent, its functional and non-functional spec, its technical and UX design, what was delivered on `dev`, what is pending, and how the pending work could be built with up to 10 implementation threads in parallel. It was written on 2026-11-07, when the founders paused both releases to regroup. Nothing here is merged into `dev`, and nothing should be.

Nothing here is a plan of record until the founders decide the new direction. The pending work is described so it can be restarted, changed, or dropped.

Start here:

- [00-state-at-pause.md](00-state-at-pause.md): what was stopped, the PRs the mission end closed (with their kept branches), the labels it changed, and how to restart.
- [00-merges-since-v0.3.0.md](00-merges-since-v0.3.0.md): all 86 squash merges into `dev` since the v0.3.0 tag, by Feature.
- [v0.4.1/_unparented/README.md](v0.4.1/_unparented/README.md): v0.4.1 issues that are not under a v0.4.1 Feature.
- `_data/`: the raw tables the docs were built from (sub-issues, merges, open and closed PRs, label changes).

Each Feature folder has the same seven files: `README.md`, `01-goal-and-intent.md`, `02-spec.md`, `03-technical-design.md`, `04-ux-design.md`, `05-implementation-plan.md`, `06-sequencing.md`.

How the docs were checked: each folder was written from the Feature and sub-issues on GitHub, the baselines on the `ledger` branch, and the code on `dev`, then checked again by a separate reviewer against the same sources. Every cited commit, issue and PR was confirmed to exist. Claims a source could not confirm are marked "unverified" or "inferred" in place.

Sub-issue counts below use the status labels from before the mission end of 2026-11-07 (see the state page): merged, in progress (a PR existed), ready, and planned or other. "Not sliced" means the Feature has no sub-issues yet; its plan proposes slices. #1178 counts as merged (PR #1238, bc78349, 2026-11-07). #889 has 41 delivered issues: 38 with `status:merged`, plus #1130, #1133 and #1195, which were fixed by PRs with no `Refs` line and still carry `status:ready`.

## v0.4.0

| Feature | Status | Sub-issues | Merged | In progress | Ready | Planned or other | Folder |
|---|---|---|---|---|---|---|---|
| [#1170](https://github.com/rahulnakmol/agentic-ayudha/issues/1170) The founder's day: decide in the morning, see each Feature proven by evening | ready | 34 | 16 | 9 | 6 | 3 | [v0.4.0/1170-founders-day-proven-by-evening/](v0.4.0/1170-founders-day-proven-by-evening/README.md) |
| [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Outcomes over output: every Feature names the customer outcome it serves, and aayu reads it after it ships | planned | 6 | 1 | 0 | 0 | 5 | [v0.4.0/982-outcomes-over-output/](v0.4.0/982-outcomes-over-output/README.md) |
| [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) aayu inbox, wave 4: the deck, phone pairing, the Behaviours view, and Ask why | planned | 5 | 0 | 0 | 0 | 5 | [v0.4.0/977-deck-phone-pairing-behaviours/](v0.4.0/977-deck-phone-pairing-behaviours/README.md) |
| [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) aayu inbox, wave 3: the local web command center, with the same words as the terminal | planned | 13 | 0 | 0 | 0 | 13 | [v0.4.0/976-local-web-command-center/](v0.4.0/976-local-web-command-center/README.md) |
| [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975) aayu inbox, wave 2: the day in areas, asks from any seat, missions, and every decision a person takes | planned | 20 | 0 | 0 | 0 | 20 | [v0.4.0/975-inbox-areas-asks-missions/](v0.4.0/975-inbox-areas-asks-missions/README.md) |
| [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895) aayu inbox, wave 1: every ship and review step a person takes is a card in the terminal inbox | planned | 15 | 0 | 0 | 0 | 15 | [v0.4.0/895-inbox-ship-review-cards/](v0.4.0/895-inbox-ship-review-cards/README.md) |
| [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) You write behaviours, and a change to one needs you | planned | not sliced | - | - | - | - | [v0.4.0/891-you-write-behaviours/](v0.4.0/891-you-write-behaviours/README.md) |
| [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) Agents pull the next story on merge, each in its own thread | planned | 65 | 38 | 0 | 12 | 15 | [v0.4.0/889-agents-pull-next-story/](v0.4.0/889-agents-pull-next-story/README.md) |
| [#887](https://github.com/rahulnakmol/agentic-ayudha/issues/887) Every Story and Bug carries Review guidance | planned | 3 | 0 | 0 | 0 | 3 | [v0.4.0/887-story-bug-review-guidance/](v0.4.0/887-story-bug-review-guidance/README.md) |
| [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) Approve to build learns time and cost from merged stories | planned | not sliced | - | - | - | - | [v0.4.0/886-approve-learns-time-cost/](v0.4.0/886-approve-learns-time-cost/README.md) |
| [#882](https://github.com/rahulnakmol/agentic-ayudha/issues/882) A build waits on a person without a reset | planned | not sliced | - | - | - | - | [v0.4.0/882-build-waits-without-reset/](v0.4.0/882-build-waits-without-reset/README.md) |
| [#881](https://github.com/rahulnakmol/agentic-ayudha/issues/881) The answer reaches the asking session | planned | not sliced | - | - | - | - | [v0.4.0/881-answer-reaches-asking-session/](v0.4.0/881-answer-reaches-asking-session/README.md) |
| [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) Any seat asks through aayu ask, and the first answer wins | planned | not sliced | - | - | - | - | [v0.4.0/880-seat-asks-aayu-ask/](v0.4.0/880-seat-asks-aayu-ask/README.md) |
| [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) Approve to build gates every story | planned | 6 | 2 | 0 | 0 | 4 | [v0.4.0/877-approve-to-build-gates/](v0.4.0/877-approve-to-build-gates/README.md) |
| [#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Review findings are resolved in a grill session | planned | 3 | 0 | 0 | 0 | 3 | [v0.4.0/876-review-findings-grill-session/](v0.4.0/876-review-findings-grill-session/README.md) |
| [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Seats review each baseline in a fresh session | planned | 5 | 0 | 0 | 0 | 5 | [v0.4.0/875-seats-review-each-baseline/](v0.4.0/875-seats-review-each-baseline/README.md) |
| [#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Feature baselines are drafted, approved, and pinned | ready | 7 | 5 | 0 | 0 | 2 | [v0.4.0/874-feature-baselines-approved-pinned/](v0.4.0/874-feature-baselines-approved-pinned/README.md) |
| [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) One Need registry, and every page names a kind | planned | not sliced | - | - | - | - | [v0.4.0/873-one-need-registry/](v0.4.0/873-one-need-registry/README.md) |
| [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872) Each merged story adds one benchmark line to the ledger | planned | not sliced | - | - | - | - | [v0.4.0/872-merged-story-benchmark-line/](v0.4.0/872-merged-story-benchmark-line/README.md) |
| [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) Every agent write names its host and session | planned | not sliced | - | - | - | - | [v0.4.0/871-agent-write-names-host-session/](v0.4.0/871-agent-write-names-host-session/README.md) |

## v0.4.1

| Feature | Status | Sub-issues | Merged | In progress | Ready | Planned or other | Folder |
|---|---|---|---|---|---|---|---|
| [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) GitHub Copilot is a first-class host, on this machine and in the cloud, with Auto by default | planned | not sliced | - | - | - | - | [v0.4.1/917-copilot-first-class-host/](v0.4.1/917-copilot-first-class-host/README.md) |
| [#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911) A repository's loops run under Kala, on every host | planned | not sliced | - | - | - | - | [v0.4.1/911-loops-run-under-kala/](v0.4.1/911-loops-run-under-kala/README.md) |
| [#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901) Lean standing context, cut by experiment | planned | not sliced | - | - | - | - | [v0.4.1/901-lean-standing-context/](v0.4.1/901-lean-standing-context/README.md) |
| [#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900) The factory learns from its own records | planned | not sliced | - | - | - | - | [v0.4.1/900-factory-learns-from-records/](v0.4.1/900-factory-learns-from-records/README.md) |
| [#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899) A review finding blocks a merge only with proof | planned | not sliced | - | - | - | - | [v0.4.1/899-finding-blocks-only-with-proof/](v0.4.1/899-finding-blocks-only-with-proof/README.md) |
| [#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898) MCP 2.0: the 2026-07-28 spec on stdio and on a local stateless HTTP endpoint | planned | not sliced | - | - | - | - | [v0.4.1/898-mcp-2-stdio-http/](v0.4.1/898-mcp-2-stdio-http/README.md) |
| [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) Metered, risk-routed merge: one record per merged PR and review depth by risk | planned | not sliced | - | - | - | - | [v0.4.1/897-metered-risk-routed-merge/](v0.4.1/897-metered-risk-routed-merge/README.md) |
| [#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893) OpenCode is a host, and it can maintain the feature maps | planned | not sliced | - | - | - | - | [v0.4.1/893-opencode-host-feature-maps/](v0.4.1/893-opencode-host-feature-maps/README.md) |
| [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) aayu init asks how you pay for models, and sets the budget | planned | not sliced | - | - | - | - | [v0.4.1/892-init-asks-model-budget/](v0.4.1/892-init-asks-model-budget/README.md) |
| [#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890) Every PR into dev carries a whiteboard defense | planned | 2 | 0 | 0 | 0 | 2 | [v0.4.1/890-pr-whiteboard-defense/](v0.4.1/890-pr-whiteboard-defense/README.md) |
| [#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888) Approve an ops action from the INCIDENT card | planned | not sliced | - | - | - | - | [v0.4.1/888-approve-ops-incident-card/](v0.4.1/888-approve-ops-incident-card/README.md) |
| [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) Each item knows what it unblocks, and depends-on is enforced | planned | not sliced | - | - | - | - | [v0.4.1/885-unblocks-and-depends-on/](v0.4.1/885-unblocks-and-depends-on/README.md) |
| [#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884) Snooze, with others, and wake survive the session | planned | not sliced | - | - | - | - | [v0.4.1/884-snooze-wake-survive-session/](v0.4.1/884-snooze-wake-survive-session/README.md) |
| [#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883) Questions have an SLA, and overdue ones come first | planned | not sliced | - | - | - | - | [v0.4.1/883-questions-sla-overdue-first/](v0.4.1/883-questions-sla-overdue-first/README.md) |
| [#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879) Bugs are triaged before Ready | planned | not sliced | - | - | - | - | [v0.4.1/879-bugs-triaged-before-ready/](v0.4.1/879-bugs-triaged-before-ready/README.md) |
| [#673](https://github.com/rahulnakmol/agentic-ayudha/issues/673) Feature: Use XERJ code search beside aayu, documented | planned | not sliced | - | - | - | - | [v0.4.1/673-xerj-code-search-documented/](v0.4.1/673-xerj-code-search-documented/README.md) |
