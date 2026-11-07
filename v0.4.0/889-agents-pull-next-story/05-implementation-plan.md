# Implementation plan: Feature #889

## Every Story and Bug

Sources: `_data/subs.tsv` (sub-issues and labels), `gh issue view` for each issue (milestone, state, body), and
`_data/merge-refs.tsv` and `_data/open-prs.tsv` for the evidence. "Depends on" is the `depends-on #n` lines of each
body. "Layers or paths" is the main files its commit changed (`git show --stat`), else the files its body names
(for pending work); only #1151 and #1198 also show their `layer:` label (most issues have `layer:` labels, for
example #997 has L02 and L03, but the table shows paths). The status column gives this document's
reading first, then the issue's `status:` label and milestone. All commits are squash merges on `dev`.

| # | Title | Type | Status (label; milestone) | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) | Approve to build writes the mission plan, and Start records the starter, the tool, and the budget | story | delivered (label: merged; v0.4.0) | M | high | — | internal/cli/mission.go, readyapprove.go | PR [#1084](https://github.com/rahulnakmol/agentic-ayudha/issues/1084) [d941d151](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d151); PR [#1085](https://github.com/rahulnakmol/agentic-ayudha/issues/1085) [b3c2cda0](https://github.com/rahulnakmol/agentic-ayudha/commit/b3c2cda0) |
| [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | `aayu mission next <f>` lists the mission's startable stories in order, with no model call | story | delivered (label: merged; v0.4.0) | M | medium | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) | internal/cli/missionnext.go | PR [#1087](https://github.com/rahulnakmol/agentic-ayudha/issues/1087) [2114e008](https://github.com/rahulnakmol/agentic-ayudha/commit/2114e008) |
| [#999](https://github.com/rahulnakmol/agentic-ayudha/issues/999) | The runnable list holds back a story whose depends-on target is still open | story | delivered (label: merged; v0.4.0) | S | medium | [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | internal/cli/missionnext.go | PR [#1088](https://github.com/rahulnakmol/agentic-ayudha/issues/1088) [9f8e457b](https://github.com/rahulnakmol/agentic-ayudha/commit/9f8e457b) |
| [#1000](https://github.com/rahulnakmol/agentic-ayudha/issues/1000) | `[build] agents = 5` per Kanha, and the mission limits are config keys | story | delivered (label: merged; v0.4.0) | S | high | [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | internal/config/defaults.toml, internal/cli/mission.go | PR [#1089](https://github.com/rahulnakmol/agentic-ayudha/issues/1089) [f563bc48](https://github.com/rahulnakmol/agentic-ayudha/commit/f563bc48); PR [#1113](https://github.com/rahulnakmol/agentic-ayudha/issues/1113) [f2a10a60](https://github.com/rahulnakmol/agentic-ayudha/commit/f2a10a60) |
| [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) | A milestone gate checks the dev tip, and each failed check gets one pre-authorised fix PR | story | delivered (label: merged; v0.4.0) | M | high | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | internal/cli/missiongate.go, missionask.go | PR [#1090](https://github.com/rahulnakmol/agentic-ayudha/issues/1090) [3ece84e7](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7) |
| [#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002) | `aayu mission stop` brings a mission or one story to a safe stop, resume continues it, and ship merges milestones in order | story | delivered (label: merged; v0.4.0) | L | high | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | internal/cli/missionstop.go, ship.go | PR [#1093](https://github.com/rahulnakmol/agentic-ayudha/issues/1093) [33aa6bf4](https://github.com/rahulnakmol/agentic-ayudha/commit/33aa6bf4) |
| [#1003](https://github.com/rahulnakmol/agentic-ayudha/issues/1003) | `aayu mission recover` re-attaches what lives, and a failed step retries up to `retry_max` | story | delivered (label: merged; v0.4.0) | M | medium | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002) | internal/cli/missionrecover.go | PR [#1102](https://github.com/rahulnakmol/agentic-ayudha/issues/1102) [bdd02c13](https://github.com/rahulnakmol/agentic-ayudha/commit/bdd02c13) |
| [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | Start on the mission start card starts the mission's Kanha on the tool recorded in the plan | story | delivered (label: merged; v0.4.0) | M | high | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | internal/cli/kanha.go, internal/delegate/brief.go | PR [#1092](https://github.com/rahulnakmol/agentic-ayudha/issues/1092) [93255510](https://github.com/rahulnakmol/agentic-ayudha/commit/93255510) |
| [#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005) | The mission skill: on each event, Kanha asks aayu what can run and starts one subthread per start | story | delivered (label: merged; v0.4.0) | M | high | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | skills/mission/SKILL.md, evals/cases/mission/ | PR [#1095](https://github.com/rahulnakmol/agentic-ayudha/issues/1095) [941f9ae5](https://github.com/rahulnakmol/agentic-ayudha/commit/941f9ae5) |
| [#1006](https://github.com/rahulnakmol/agentic-ayudha/issues/1006) | A story subthread owns one story, may use subagents, and ships it on proof | story | delivered (label: merged; v0.4.0) | M | high | [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | internal/cli/delegate.go, internal/delegate/brief.go | PR [#1094](https://github.com/rahulnakmol/agentic-ayudha/issues/1094) [53186c97](https://github.com/rahulnakmol/agentic-ayudha/commit/53186c97) |
| [#1007](https://github.com/rahulnakmol/agentic-ayudha/issues/1007) | ship tells the story's Kanha after each merge, in the same process | story | delivered (label: merged; v0.4.0) | S | high | [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | internal/cli/ship.go | PR [#1098](https://github.com/rahulnakmol/agentic-ayudha/issues/1098) [00f7fc2a](https://github.com/rahulnakmol/agentic-ayudha/commit/00f7fc2a) |
| [#1008](https://github.com/rahulnakmol/agentic-ayudha/issues/1008) | A stop, a block, a failed start, or an answered ask tells the Kanha | story | delivered (label: merged; v0.4.0) | S | medium | [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | internal/cli/build.go, kanha.go | PR [#1097](https://github.com/rahulnakmol/agentic-ayudha/issues/1097) [7a9f3d7e](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e) |
| [#1009](https://github.com/rahulnakmol/agentic-ayudha/issues/1009) | An hourly safety tick wakes each Kanha that missed an event, and syncs cloud PRs | story | delivered (label: merged; v0.4.0) | S | high | [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | internal/cli/missiontick.go, loops/build-ready.toml | PR [#1103](https://github.com/rahulnakmol/agentic-ayudha/issues/1103) [e43876ec](https://github.com/rahulnakmol/agentic-ayudha/commit/e43876ec) |
| [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) | Every build event of a Feature goes to `delivery/<f>/events.jsonl` and one progress comment | story | pending (label: planned; v0.4.0) | M | high | [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | internal/delivery/ (new), ship.go, build.go | not started |
| [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011) | @rahulnakmol is paged only at a milestone end, a block, and the mission's end | story | pending (label: planned; v0.4.0) | S | medium | [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) | internal/cli, internal/notify | not started |
| [#1012](https://github.com/rahulnakmol/agentic-ayudha/issues/1012) | A stateless wake on GitHub events, and a local Kanha for Codex, Copilot, and OpenCode | story | delivered (label: merged; v0.4.0) | L | high | [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) | internal/cli/missionwake.go, kanhalocal.go | PR [#1125](https://github.com/rahulnakmol/agentic-ayudha/issues/1125) [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) |
| [#1013](https://github.com/rahulnakmol/agentic-ayudha/issues/1013) | Kanha's lock at mission start: the bodies and the size, risk, and priority labels of the Feature, its stories, and its Bugs become read-only | story | delivered (label: merged; v0.4.0) | M | high | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) | internal/cli/missionlock.go | PR [#1096](https://github.com/rahulnakmol/agentic-ayudha/issues/1096) [3d42ff15](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff15) |
| [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) | Kanha's feedback pass at mission end turns people's comments into new stories that @rahulnakmol approves once | story | pending (label: planned; v0.4.0) | M | medium | [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) [#1013](https://github.com/rahulnakmol/agentic-ayudha/issues/1013) | internal/cli (new action), skills/mission/SKILL.md | not started |
| [#1015](https://github.com/rahulnakmol/agentic-ayudha/issues/1015) | Mission time limits: warn at 24 hours, hold new starts at 48 hours, and a person's reason to start or extend past them | story | delivered (label: merged; v0.4.0) | M | medium | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) | internal/cli/missiontime.go | PR [#1091](https://github.com/rahulnakmol/agentic-ayudha/issues/1091) [2c6c6b04](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) |
| [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) | Inside an approved mission, ship merges on proof and holds only what the person must see | story | pending (label: planned; v0.4.0) | L | high | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083) | internal/cli/ship.go, shipproof.go, internal/config/ | not started |
| [#1031](https://github.com/rahulnakmol/agentic-ayudha/issues/1031) | Ship gaps: a nobody-configured inbox, plan layers from labels, the first story line, and no pack after a merge outside ship | bug | delivered (label: merged; v0.4.0) | M | medium | — | internal/cli/ship.go, shipproof.go, home.go, plan.go | PR [#1147](https://github.com/rahulnakmol/agentic-ayudha/issues/1147) [7759eb05](https://github.com/rahulnakmol/agentic-ayudha/commit/7759eb05) |
| [#1099](https://github.com/rahulnakmol/agentic-ayudha/issues/1099) | Mission start and its Kanha's start record two different times, so TestMissionStartBriefsDraftingThread fails when they cross a second | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/cli/kanha.go, mission.go | PR [#1117](https://github.com/rahulnakmol/agentic-ayudha/issues/1117) [4957f378](https://github.com/rahulnakmol/agentic-ayudha/commit/4957f378) |
| [#1100](https://github.com/rahulnakmol/agentic-ayudha/issues/1100) | The merge gate refuses a pre-authorised gate-fix PR, because it needs a Ready snapshot of the Feature the PR refs | bug | delivered (label: merged; v0.4.0) | S | medium | [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) | internal/cli/missiongate.go, verify.go, proof.go | PR [#1120](https://github.com/rahulnakmol/agentic-ayudha/issues/1120) [4f924f52](https://github.com/rahulnakmol/agentic-ayudha/commit/4f924f52) |
| [#1101](https://github.com/rahulnakmol/agentic-ayudha/issues/1101) | `aayu halt` leaves mission claims in place, so a story claimed just before a halt still counts as claimed after aayu resume | bug | delivered (label: merged; v0.4.0) | S | medium | [#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005) | internal/cli/halt.go | PR [#1187](https://github.com/rahulnakmol/agentic-ayudha/issues/1187) [685e6949](https://github.com/rahulnakmol/agentic-ayudha/commit/685e6949) |
| [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) | The story subthread brief carries the 10-step build loop, and an eval fails when a step is missing | story | pending (label: planned; v0.4.0) | S | medium | [#1003](https://github.com/rahulnakmol/agentic-ayudha/issues/1003) [#1006](https://github.com/rahulnakmol/agentic-ayudha/issues/1006) | internal/delegate/brief.go, evals/cases/build/ | not started |
| [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) | Proof packs record the red run of each test, and a missing red run warns | story | pending (label: planned; v0.4.1) | M | medium | [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) | internal/proof/pack.go, internal/verify/, internal/cli/build.go | not started |
| [#1110](https://github.com/rahulnakmol/agentic-ayudha/issues/1110) | Real-run journeys on the built binary prove each story, and the milestone gate re-runs them at the dev tip | story | pending (label: planned; v0.4.1) | L | high | [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) | journeys/ (proposed), internal/verify/, internal/cli/missiongate.go | not started |
| [#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111) | Inspect flags function beyond the story's ask, and each milestone records whether its work will matter | story | pending (label: planned; v0.4.1) | M | medium | [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) | internal/inspect/, internal/cli/missiongate.go | not started |
| [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112) | Each milestone's findings propose the order of the next milestone's stories | story | pending (label: planned; v0.4.1) | M | medium | [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) | internal/cli/mission*.go, inbox | not started |
| [#1118](https://github.com/rahulnakmol/agentic-ayudha/issues/1118) | A send to the Kanha has no time limit, so a stalled receiver CLI makes ship or build hang | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/cli/kanha.go, delegate.go | PR [#1119](https://github.com/rahulnakmol/agentic-ayudha/issues/1119) [34f22b7c](https://github.com/rahulnakmol/agentic-ayudha/commit/34f22b7c) |
| [#1121](https://github.com/rahulnakmol/agentic-ayudha/issues/1121) | The safety tick wakes or syncs a mission paused, ended, or stopped during the tick | bug | closed (label: planned; none) | S | low | — | internal/cli/missiontick.go | closed: fixed by PR [#1103](https://github.com/rahulnakmol/agentic-ayudha/issues/1103) (comment) |
| [#1124](https://github.com/rahulnakmol/agentic-ayudha/issues/1124) | A hand-off or follow-up to a receiver has no time limit, so a stalled receiver CLI makes build or mission start hang | bug | delivered (label: merged; v0.4.0) | S | low | — | internal/cli/delegate.go, kanha.go | PR [#1136](https://github.com/rahulnakmol/agentic-ayudha/issues/1136) [7239cb04](https://github.com/rahulnakmol/agentic-ayudha/commit/7239cb04) |
| [#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126) | The local Kanha's machine runs `aayu mission wake` every `check_minutes`, set up by `aayu install --loops` | story | pending (label: planned; v0.4.0) | S | low | [#1012](https://github.com/rahulnakmol/agentic-ayudha/issues/1012) [#1103](https://github.com/rahulnakmol/agentic-ayudha/issues/1103) | internal/cli/installloops.go, kanhalocal.go | not started |
| [#1128](https://github.com/rahulnakmol/agentic-ayudha/issues/1128) | A sync sends a delegation's findings before it records them, so a lost save sends them again | bug | pending (label: planned; v0.4.0) | S | low | — | internal/cli/delegate.go, internal/delegate/state.go | not started |
| [#1129](https://github.com/rahulnakmol/agentic-ayudha/issues/1129) | A mission claim stays valid after its story moves to another Feature, so the Kanha starts it on the old mission's tool | bug | delivered (label: merged; v0.4.0) | S | low | [#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005) | internal/cli/missionnext.go, internal/ready/ready.go | PR [#1131](https://github.com/rahulnakmol/agentic-ayudha/issues/1131) [e9870d09](https://github.com/rahulnakmol/agentic-ayudha/commit/e9870d09) |
| [#1130](https://github.com/rahulnakmol/agentic-ayudha/issues/1130) | inspect drops CodeRabbit findings whose severity badge is an icon, so a Major reads as no finding | bug | delivered (label: ready; v0.4.0) | S | medium | — | internal/inspect/apps.go | PR [#1135](https://github.com/rahulnakmol/agentic-ayudha/issues/1135) [18845b72](https://github.com/rahulnakmol/agentic-ayudha/commit/18845b72) (no Refs line) |
| [#1132](https://github.com/rahulnakmol/agentic-ayudha/issues/1132) | The Kanha's report restates issue text it did not act on, so mission/stays-in-list scores 3.00 against its 3.50 bar | bug | delivered (label: merged; v0.4.0) | XS | low | [#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005) | skills/mission/SKILL.md | PR [#1146](https://github.com/rahulnakmol/agentic-ayudha/issues/1146) [c60f3cf6](https://github.com/rahulnakmol/agentic-ayudha/commit/c60f3cf6) |
| [#1133](https://github.com/rahulnakmol/agentic-ayudha/issues/1133) | A person's early `/aayu review again` reopens nothing, and the gate does not say so | bug | delivered (label: ready; v0.4.0) | S | low | — | internal/cli/inspect_app.go, .github/workflows/aayudha-merge-gate.yml | PR [#1145](https://github.com/rahulnakmol/agentic-ayudha/issues/1145) [d40b7c9b](https://github.com/rahulnakmol/agentic-ayudha/commit/d40b7c9b) (no Refs line) |
| [#1134](https://github.com/rahulnakmol/agentic-ayudha/issues/1134) | The mission lock ends when the last milestone's gate passes, not when every story merges | story | pending (label: planned; none) | S | medium | — | internal/cli/missiongate.go, missionlock.go | not started |
| [#1137](https://github.com/rahulnakmol/agentic-ayudha/issues/1137) | build's sync reads a receiver's task with no time limit, so a stalled receiver makes build hang | bug | delivered (label: merged; v0.4.0) | S | low | — | internal/cli/build.go, delegate.go, internal/delegate/receivers.go | PR [#1279](https://github.com/rahulnakmol/agentic-ayudha/issues/1279) [0cd53b4b](https://github.com/rahulnakmol/agentic-ayudha/commit/0cd53b4b) |
| [#1138](https://github.com/rahulnakmol/agentic-ayudha/issues/1138) | CWE-1427: tellKanha sends event text to a cloud Kanha's prompt unmarked | bug | in review (label: ready; v0.4.0) | S | medium | — | internal/cli/kanha.go, kanhalocal.go, build.go | open PR [#1260](https://github.com/rahulnakmol/agentic-ayudha/issues/1260) at `9f3033b` |
| [#1139](https://github.com/rahulnakmol/agentic-ayudha/issues/1139) | CWE-1427: mission next prints delegation events to the Kanha unmarked | bug | pending (label: ready; v0.4.0) | S | medium | [#1138](https://github.com/rahulnakmol/agentic-ayudha/issues/1138) | internal/cli/missionnext.go, missionrecover.go | not started |
| [#1140](https://github.com/rahulnakmol/agentic-ayudha/issues/1140) | Mission tests fix a story's start on 2026-10-26, so the 48-hour mission clock fails them on every branch | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/cli/mission tests | PR [#1141](https://github.com/rahulnakmol/agentic-ayudha/issues/1141) [a3c43bf3](https://github.com/rahulnakmol/agentic-ayudha/commit/a3c43bf3) |
| [#1148](https://github.com/rahulnakmol/agentic-ayudha/issues/1148) | A maybe-started Codex story whose task is done with no PR stays held for good, because recover never re-attaches the task | bug | delivered (label: merged; v0.4.0) | S | low | — | internal/cli/missionrecover.go, delegate.go | PR [#1185](https://github.com/rahulnakmol/agentic-ayudha/issues/1185) [53b7a8aa](https://github.com/rahulnakmol/agentic-ayudha/commit/53b7a8aa) |
| [#1149](https://github.com/rahulnakmol/agentic-ayudha/issues/1149) | `aayu mission recover` lists the Codex environment the delegation started in | bug | delivered (label: merged; v0.4.0) | S | medium | — | internal/cli/missionrecover.go, internal/delegate/state.go | PR [#1190](https://github.com/rahulnakmol/agentic-ayudha/issues/1190) [53c3d2f3](https://github.com/rahulnakmol/agentic-ayudha/commit/53c3d2f3) |
| [#1150](https://github.com/rahulnakmol/agentic-ayudha/issues/1150) | inspect reads a person's `/aayu review again` comment as their review reply and fails closed | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/inspect/apps.go | PR [#1180](https://github.com/rahulnakmol/agentic-ayudha/issues/1180) [b7b36fda](https://github.com/rahulnakmol/agentic-ayudha/commit/b7b36fda) |
| [#1151](https://github.com/rahulnakmol/agentic-ayudha/issues/1151) | CI build's `go test` has no timeout, so a slow internal/cli fails it at random | bug | delivered (label: merged; v0.4.0) | XS | low | — | .github/workflows/ci.yml (L09) | PR [#1153](https://github.com/rahulnakmol/agentic-ayudha/issues/1153) [3a59e635](https://github.com/rahulnakmol/agentic-ayudha/commit/3a59e635) |
| [#1154](https://github.com/rahulnakmol/agentic-ayudha/issues/1154) | internal/cli's tests run well below `go test`'s 10-minute default | story | pending (label: planned; none) | M | low | — | internal/cli/*_test.go | not started |
| [#1155](https://github.com/rahulnakmol/agentic-ayudha/issues/1155) | A person's dismissal never clears a carried finding on a PR with no Story | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/cli/inspect.go | PR [#1156](https://github.com/rahulnakmol/agentic-ayudha/issues/1156) [1fe6b53a](https://github.com/rahulnakmol/agentic-ayudha/commit/1fe6b53a) |
| [#1157](https://github.com/rahulnakmol/agentic-ayudha/issues/1157) | A Story whose criteria are tier-2 eval results can never reach merge before it merges | bug | closed (label: ready; v0.4.0) | M | medium | — | internal/verify/evidence.go | closed not planned; replaced by [#1163](https://github.com/rahulnakmol/agentic-ayudha/issues/1163) |
| [#1159](https://github.com/rahulnakmol/agentic-ayudha/issues/1159) | `proof land` can never land a merged pull request: its gate runs list no pull request | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/cli/home.go, ship.go | PR [#1160](https://github.com/rahulnakmol/agentic-ayudha/issues/1160) [4e9bd372](https://github.com/rahulnakmol/agentic-ayudha/commit/4e9bd372) |
| [#1161](https://github.com/rahulnakmol/agentic-ayudha/issues/1161) | A gate run of a pull request that once shared the branch and head can count for a merged pull request | bug | in review (label: ready; v0.4.1) | S | low | — | internal/cli/home.go | open PR [#1183](https://github.com/rahulnakmol/agentic-ayudha/issues/1183) at `30c9105` (held) |
| [#1162](https://github.com/rahulnakmol/agentic-ayudha/issues/1162) | ship and `proof land` can take the pack and the gate checks from two gate runs | bug | delivered (label: merged; v0.4.0) | S | low | — | internal/cli/proof.go, ship.go | PR [#1200](https://github.com/rahulnakmol/agentic-ayudha/issues/1200) [9f357b01](https://github.com/rahulnakmol/agentic-ayudha/commit/9f357b01) |
| [#1163](https://github.com/rahulnakmol/agentic-ayudha/issues/1163) | A person's recorded approval of a head never clears a rung-B verdict whose only gap is unclear criteria | bug | delivered (label: merged; v0.4.0) | S | medium | — | internal/cli/inspect.go | PR [#1164](https://github.com/rahulnakmol/agentic-ayudha/issues/1164) [28243587](https://github.com/rahulnakmol/agentic-ayudha/commit/28243587) |
| [#1165](https://github.com/rahulnakmol/agentic-ayudha/issues/1165) | A cached merge from an approval lift is not checked again against the current approver | bug | delivered (label: merged; v0.4.0) | S | low | — | internal/cli/inspect.go | PR [#1167](https://github.com/rahulnakmol/agentic-ayudha/issues/1167) [8c28b132](https://github.com/rahulnakmol/agentic-ayudha/commit/8c28b132) |
| [#1166](https://github.com/rahulnakmol/agentic-ayudha/issues/1166) | Every mission eval case scores 1 because the case worlds have no answer for `aayu mission recover` | bug | partly delivered (label: ready; v0.4.0) | S | low | — | evals/cases/mission/ | part: PR [#1181](https://github.com/rahulnakmol/agentic-ayudha/issues/1181) [ceea0479](https://github.com/rahulnakmol/agentic-ayudha/commit/ceea0479) (no Refs line); halted case open |
| [#1169](https://github.com/rahulnakmol/agentic-ayudha/issues/1169) | aayu mission next holds a story whose depends-on target merged but is still open | bug | delivered (label: merged; v0.4.0) | S | medium | — | internal/cli/missionnext.go, mission.go | PR [#1184](https://github.com/rahulnakmol/agentic-ayudha/issues/1184) [c2d6bd70](https://github.com/rahulnakmol/agentic-ayudha/commit/c2d6bd70) |
| [#1186](https://github.com/rahulnakmol/agentic-ayudha/issues/1186) | TestMissionEndOnlyByStarter fails when the minute changes during the test | bug | delivered (label: merged; v0.4.0) | XS | low | — | internal/cli/mission tests | PR [#1205](https://github.com/rahulnakmol/agentic-ayudha/issues/1205) [66420442](https://github.com/rahulnakmol/agentic-ayudha/commit/66420442) |
| [#1195](https://github.com/rahulnakmol/agentic-ayudha/issues/1195) | security fails on every PR: glob and minimatch advisories in `pnpm-lock.yaml` | bug | delivered (label: ready; v0.4.0) | S | low | — | pnpm-workspace.yaml, pnpm-lock.yaml | PR [#1196](https://github.com/rahulnakmol/agentic-ayudha/issues/1196) [f95b9fa0](https://github.com/rahulnakmol/agentic-ayudha/commit/f95b9fa0) (no Refs line) |
| [#1197](https://github.com/rahulnakmol/agentic-ayudha/issues/1197) | CWE-1427: build, mission wake, and mission tick print receiver error text to a model unmarked | bug | pending (label: planned; none) | S | medium | [#1139](https://github.com/rahulnakmol/agentic-ayudha/issues/1139) | internal/cli/build.go, delegate.go, missionwake.go, missiontick.go | not started |
| [#1198](https://github.com/rahulnakmol/agentic-ayudha/issues/1198) | config validation prints a rejected secret value | bug | in review (label: ready; v0.4.0) | XS | low | — | internal/config/ (L07) | open PR [#1202](https://github.com/rahulnakmol/agentic-ayudha/issues/1202) at `a4ab2d8` |
| [#1203](https://github.com/rahulnakmol/agentic-ayudha/issues/1203) | A closed pull request whose commit list lost the head can still let its gate run count for a merged pull request | bug | pending (label: ready; v0.4.1) | S | low | — | internal/cli/home.go | not started |
| [#1204](https://github.com/rahulnakmol/agentic-ayudha/issues/1204) | A pull request from the branch outside the open window blocks a merged pull request's own gate run | bug | in review (label: ready; v0.4.1) | XS | low | — | internal/cli/home.go | open draft PR [#1277](https://github.com/rahulnakmol/agentic-ayudha/issues/1277) at `9c450d7` |
| [#1264](https://github.com/rahulnakmol/agentic-ayudha/issues/1264) | A delegated build reports a state the ledger did not take when the ledger write fails | bug | delivered (label: merged; v0.4.1) | M | medium | — | internal/cli/delegate.go, build.go, inspect.go, counsel.go | PR [#1272](https://github.com/rahulnakmol/agentic-ayudha/issues/1272) [6cf3876c](https://github.com/rahulnakmol/agentic-ayudha/commit/6cf3876c) |
| [#1266](https://github.com/rahulnakmol/agentic-ayudha/issues/1266) | The Amp host hand-off lapses by the hand-off's age and records a late or malformed thread ID | bug | in review (label: ready; v0.4.1) | S | medium | — | internal/cli/delegate.go, missionnext.go, internal/delegate/, skills/mission/SKILL.md | open draft PR [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269) at `7b8fc9b` |

## Delivered

What works on `dev` at `884d81b`, grouped by part. Each commit is a squash merge since v0.3.0
(`_data/merge-refs.tsv`), and the code exists at the paths in [03-technical-design.md](03-technical-design.md).

- **Plan and start.** Approve to build writes the mission plan; Start records the starter, tool, and budget
  ([#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997):
  [d941d151](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d151),
  [b3c2cda0](https://github.com/rahulnakmol/agentic-ayudha/commit/b3c2cda0)). One start time for the mission and its
  Kanha ([#1099](https://github.com/rahulnakmol/agentic-ayudha/issues/1099):
  [4957f378](https://github.com/rahulnakmol/agentic-ayudha/commit/4957f378)). `[build] agents = 5` and the
  `[mission]` keys ([#1000](https://github.com/rahulnakmol/agentic-ayudha/issues/1000):
  [f563bc48](https://github.com/rahulnakmol/agentic-ayudha/commit/f563bc48),
  [f2a10a60](https://github.com/rahulnakmol/agentic-ayudha/commit/f2a10a60)).
- **The list.** `aayu mission next` with claims ([#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998):
  [2114e008](https://github.com/rahulnakmol/agentic-ayudha/commit/2114e008)); depends-on holds
  ([#999](https://github.com/rahulnakmol/agentic-ayudha/issues/999):
  [9f8e457b](https://github.com/rahulnakmol/agentic-ayudha/commit/9f8e457b);
  [#1169](https://github.com/rahulnakmol/agentic-ayudha/issues/1169):
  [c2d6bd70](https://github.com/rahulnakmol/agentic-ayudha/commit/c2d6bd70)); a moved story does not start
  ([#1129](https://github.com/rahulnakmol/agentic-ayudha/issues/1129):
  [e9870d09](https://github.com/rahulnakmol/agentic-ayudha/commit/e9870d09)).
- **Kanha and story threads.** Start begins the Kanha on the plan's tool
  ([#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004):
  [93255510](https://github.com/rahulnakmol/agentic-ayudha/commit/93255510)); the mission skill and its evals
  ([#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005):
  [941f9ae5](https://github.com/rahulnakmol/agentic-ayudha/commit/941f9ae5);
  [#1132](https://github.com/rahulnakmol/agentic-ayudha/issues/1132):
  [c60f3cf6](https://github.com/rahulnakmol/agentic-ayudha/commit/c60f3cf6)); the story subthread owns its PR
  ([#1006](https://github.com/rahulnakmol/agentic-ayudha/issues/1006):
  [53186c97](https://github.com/rahulnakmol/agentic-ayudha/commit/53186c97)); ship, stop, block, failed start, and
  answered ask tell the Kanha ([#1007](https://github.com/rahulnakmol/agentic-ayudha/issues/1007):
  [00f7fc2a](https://github.com/rahulnakmol/agentic-ayudha/commit/00f7fc2a);
  [#1008](https://github.com/rahulnakmol/agentic-ayudha/issues/1008):
  [7a9f3d7e](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e)); stateless wake and local Kanha
  ([#1012](https://github.com/rahulnakmol/agentic-ayudha/issues/1012):
  [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a)).
- **Safety and recovery.** Hourly tick ([#1009](https://github.com/rahulnakmol/agentic-ayudha/issues/1009):
  [e43876ec](https://github.com/rahulnakmol/agentic-ayudha/commit/e43876ec), which also fixed
  [#1121](https://github.com/rahulnakmol/agentic-ayudha/issues/1121) by its comment); recover and retry
  ([#1003](https://github.com/rahulnakmol/agentic-ayudha/issues/1003):
  [bdd02c13](https://github.com/rahulnakmol/agentic-ayudha/commit/bdd02c13);
  [#1148](https://github.com/rahulnakmol/agentic-ayudha/issues/1148):
  [53b7a8aa](https://github.com/rahulnakmol/agentic-ayudha/commit/53b7a8aa);
  [#1149](https://github.com/rahulnakmol/agentic-ayudha/issues/1149):
  [53c3d2f3](https://github.com/rahulnakmol/agentic-ayudha/commit/53c3d2f3)); halt releases claims
  ([#1101](https://github.com/rahulnakmol/agentic-ayudha/issues/1101):
  [685e6949](https://github.com/rahulnakmol/agentic-ayudha/commit/685e6949)).
- **Control.** Milestone gate and fix PRs ([#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001):
  [3ece84e7](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7);
  [#1100](https://github.com/rahulnakmol/agentic-ayudha/issues/1100):
  [4f924f52](https://github.com/rahulnakmol/agentic-ayudha/commit/4f924f52)); stop, resume, end, and merge order
  ([#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002):
  [33aa6bf4](https://github.com/rahulnakmol/agentic-ayudha/commit/33aa6bf4)); the lock
  ([#1013](https://github.com/rahulnakmol/agentic-ayudha/issues/1013):
  [3d42ff15](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff15)); time limits
  ([#1015](https://github.com/rahulnakmol/agentic-ayudha/issues/1015):
  [2c6c6b04](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04)).
- **Receiver time limits and ledger truth.** Two-minute limits on sends, hand-offs, follow-ups, and status reads
  ([#1118](https://github.com/rahulnakmol/agentic-ayudha/issues/1118):
  [34f22b7c](https://github.com/rahulnakmol/agentic-ayudha/commit/34f22b7c);
  [#1124](https://github.com/rahulnakmol/agentic-ayudha/issues/1124):
  [7239cb04](https://github.com/rahulnakmol/agentic-ayudha/commit/7239cb04);
  [#1137](https://github.com/rahulnakmol/agentic-ayudha/issues/1137):
  [0cd53b4b](https://github.com/rahulnakmol/agentic-ayudha/commit/0cd53b4b)); a delegated build reports only what
  the ledger holds ([#1264](https://github.com/rahulnakmol/agentic-ayudha/issues/1264):
  [6cf3876c](https://github.com/rahulnakmol/agentic-ayudha/commit/6cf3876c)).
- **Merge gate, inspect, and proof fixes found while building.**
  [#1031](https://github.com/rahulnakmol/agentic-ayudha/issues/1031)
  ([7759eb05](https://github.com/rahulnakmol/agentic-ayudha/commit/7759eb05)),
  [#1150](https://github.com/rahulnakmol/agentic-ayudha/issues/1150)
  ([b7b36fda](https://github.com/rahulnakmol/agentic-ayudha/commit/b7b36fda)),
  [#1155](https://github.com/rahulnakmol/agentic-ayudha/issues/1155)
  ([1fe6b53a](https://github.com/rahulnakmol/agentic-ayudha/commit/1fe6b53a)),
  [#1159](https://github.com/rahulnakmol/agentic-ayudha/issues/1159)
  ([4e9bd372](https://github.com/rahulnakmol/agentic-ayudha/commit/4e9bd372)),
  [#1162](https://github.com/rahulnakmol/agentic-ayudha/issues/1162)
  ([9f357b01](https://github.com/rahulnakmol/agentic-ayudha/commit/9f357b01)),
  [#1163](https://github.com/rahulnakmol/agentic-ayudha/issues/1163)
  ([28243587](https://github.com/rahulnakmol/agentic-ayudha/commit/28243587)),
  [#1165](https://github.com/rahulnakmol/agentic-ayudha/issues/1165)
  ([8c28b132](https://github.com/rahulnakmol/agentic-ayudha/commit/8c28b132)),
  [#1130](https://github.com/rahulnakmol/agentic-ayudha/issues/1130)
  ([18845b72](https://github.com/rahulnakmol/agentic-ayudha/commit/18845b72), PR
  [#1135](https://github.com/rahulnakmol/agentic-ayudha/issues/1135): "Bug #1130, as a chore"; test
  `TestReadCodeRabbitIconBadges` in `internal/inspect/inspect_test.go`), and
  [#1133](https://github.com/rahulnakmol/agentic-ayudha/issues/1133)
  ([d40b7c9b](https://github.com/rahulnakmol/agentic-ayudha/commit/d40b7c9b), PR
  [#1145](https://github.com/rahulnakmol/agentic-ayudha/issues/1145): "Bug #1133, as a chore"; test
  `TestInspectAnswersAnIdleReopen` in `internal/cli/inspect_test.go`).
- **CI and tests.** `go test -timeout 30m` in CI ([#1151](https://github.com/rahulnakmol/agentic-ayudha/issues/1151):
  [3a59e635](https://github.com/rahulnakmol/agentic-ayudha/commit/3a59e635)); mission test clocks
  ([#1140](https://github.com/rahulnakmol/agentic-ayudha/issues/1140):
  [a3c43bf3](https://github.com/rahulnakmol/agentic-ayudha/commit/a3c43bf3);
  [#1186](https://github.com/rahulnakmol/agentic-ayudha/issues/1186):
  [66420442](https://github.com/rahulnakmol/agentic-ayudha/commit/66420442)); the `security` job's pnpm advisories
  ([#1195](https://github.com/rahulnakmol/agentic-ayudha/issues/1195):
  [f95b9fa0](https://github.com/rahulnakmol/agentic-ayudha/commit/f95b9fa0), PR
  [#1196](https://github.com/rahulnakmol/agentic-ayudha/issues/1196), which says "No `Refs` line"; the overrides are
  in `pnpm-workspace.yaml`); two mission eval worlds answer recover
  ([#1166](https://github.com/rahulnakmol/agentic-ayudha/issues/1166) part:
  [ceea0479](https://github.com/rahulnakmol/agentic-ayudha/commit/ceea0479), PR
  [#1181](https://github.com/rahulnakmol/agentic-ayudha/issues/1181), "Part of Bug #1166").

Related merges with no sub-issue of #889: PR [#1258](https://github.com/rahulnakmol/agentic-ayudha/issues/1258)
([ab8ab076](https://github.com/rahulnakmol/agentic-ayudha/commit/ab8ab076), a `--thread` record that loses the ledger
race), PR [#1263](https://github.com/rahulnakmol/agentic-ayudha/issues/1263)
([f06f301d](https://github.com/rahulnakmol/agentic-ayudha/commit/f06f301d), mission-lock test clocks), and PR
[#1152](https://github.com/rahulnakmol/agentic-ayudha/issues/1152)
([76f8c479](https://github.com/rahulnakmol/agentic-ayudha/commit/76f8c479), docs: Cursor with no API key runs a local
Kanha) (`_data/merge-refs.tsv`).

Status labels lag: #1130, #1133, and #1195 still say `status:ready`, and every delivered issue is still OPEN.

## In review

| Bug | PR and head | State on 2026-11-07 | What blocks it |
|---|---|---|---|
| [#1138](https://github.com/rahulnakmol/agentic-ayudha/issues/1138) | [#1260](https://github.com/rahulnakmol/agentic-ayudha/issues/1260) at `9f3033b`, ready for review | `build` failed; `aayudha/pack` and `aayudha/merge-gate` failed; the proof comment says "floors do not hold" and its coverage row reads `fail`, value 100, threshold 90; CodeRabbit was rate-limited | A red `build` and a floor miss; the cause of the coverage row is unverified. Bug #1139 builds on this PR (its RAID log) |
| [#1161](https://github.com/rahulnakmol/agentic-ayudha/issues/1161) | [#1183](https://github.com/rahulnakmol/agentic-ayudha/issues/1183) at `30c9105` | inspect `fix`; @rahulnakmol "chose to hold this PR at 30c9105 and to push nothing more for now" (comment, 2026-10-31); milestone v0.4.1 | Two open P1 findings: `9d13bc7c7f` (filed as #1203) and `ba9b6c4d00` (the builder found it false; not dismissed). Three judged rounds ran, so a fourth head needs a person's `/aayu review again` (AGENTS.md, round cap) |
| [#1198](https://github.com/rahulnakmol/agentic-ayudha/issues/1198) | [#1202](https://github.com/rahulnakmol/agentic-ayudha/issues/1202) at `a4ab2d8` | inspect failed; "a human review is needed: no installed reviewer app can review this pull request" (comment, 2026-11-02) | A person's review, and a person's ship: it changes `internal/config/`, which is in `[autonomy] always_person` (`.aayudha/config.toml`) |
| [#1204](https://github.com/rahulnakmol/agentic-ayudha/issues/1204) | [#1277](https://github.com/rahulnakmol/agentic-ayudha/issues/1277) at `9c450d7`, draft | draft; most gate checks skipped | Its body has "Deviation from the Bug": "a person amends criterion 1 to match". It changes the same files as #1183 (`internal/cli/home.go`, `ship_test.go`, `internal/github/fake/fake.go`) |
| [#1266](https://github.com/rahulnakmol/agentic-ayudha/issues/1266) | [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269) at `7b8fc9b`, draft | draft; `protected-paths` failed | It changes `skills/mission/SKILL.md`, which `.github/CODEOWNERS` assigns to @rahulnakmol, so it needs the founder's solo-mode bypass (AGENTS.md, Protected paths) |

## Pending

### Stories (12, not started)

| Story | Ready? | What it waits on |
|---|---|---|
| [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) | `status:planned`, no Ready snapshot on the ledger | Nothing in the code: #1003 and #1006 merged. Open question: which eval case kind holds a rendered brief |
| [#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126) | planned | Nothing in the code: #1012 and PR #1103 merged |
| [#1134](https://github.com/rahulnakmol/agentic-ayudha/issues/1134) | planned, no milestone | Nothing. Criterion 1 appears met on `dev` already (see [02-spec.md](02-spec.md), FR-24) |
| [#1154](https://github.com/rahulnakmol/agentic-ayudha/issues/1154) | planned, no milestone | A person confirms the 300 s and 450 s thresholds "before Ready" (Open questions) |
| [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) | planned | `internal/delivery/` from Feature [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872) (OPEN, planned); `aayu ask` from Feature [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) for criterion 2 (no `ask` verb on `dev`) |
| [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011) | planned | #1010 |
| [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) | planned | #1010; `aayu triage --decide story --changes` from [#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879) S8 (Feature in v0.4.1; not on `dev`) |
| [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) | planned | Stories [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028), [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029), [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083) (all planned); and a person's call on its overlap with Decision [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) and Story [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281) |
| [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) | planned, v0.4.1 | #1104 |
| [#1110](https://github.com/rahulnakmol/agentic-ayudha/issues/1110) | planned, v0.4.1 | #1104 |
| [#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111) | planned, v0.4.1 | #1104; Story [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) (the Outcome line; planned) |
| [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112) | planned, v0.4.1 | #1014 |

### Bugs (4 open, plus the rest of #1166)

| Bug | Ready? | What it waits on |
|---|---|---|
| [#1139](https://github.com/rahulnakmol/agentic-ayudha/issues/1139) | Ready (`specs/1139/ready.md`) | #1138 (PR #1260) |
| [#1197](https://github.com/rahulnakmol/agentic-ayudha/issues/1197) | planned, no milestone | #1139 |
| [#1128](https://github.com/rahulnakmol/agentic-ayudha/issues/1128) | planned | Nothing |
| [#1203](https://github.com/rahulnakmol/agentic-ayudha/issues/1203) | Ready (`specs/1203/ready.md`), v0.4.1 | `hadHead` comes from PR #1183 and is not on `dev`, so #1161 merges first |
| [#1166](https://github.com/rahulnakmol/agentic-ayudha/issues/1166), halted case | Ready (`specs/1166/ready.md`) | PR #1181's RAID log: "The halted case needs an accepted eval-bar waiver for its require change" (a person) |

### Work not yet sliced into Stories

- **The Feature body's own words.** `aayu build --next --dispatch` and "`ship` dispatches again after each merge"
  are not built as written; the mission verbs replaced them. The current #889 body still says it
  (`gh issue view 889`), and the approved intent pins the body as it was on 2026-10-30 (hash `b52f2240…`). A person decides whether to edit
  the body, which needs a new intent baseline.
- **Approve to build for #889 itself.** No `specs/889/approved.json` and no `missions/889/` exist on the ledger.
  The remaining stories cannot run as a #889 mission until a person approves the Feature to build.
- **The overlap of #1030 with Decision #1255.** #1281's title is "Every agent PR into dev merges on proof at any risk
  and hour, and only the policy keys wait for a person (Decision #1255)". Its draft PR
  [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) was closed unmerged on 2026-11-07 when the
  mission of #1170 ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` is kept (`_data/closed-prs.tsv`).
  #1281 is still OPEN and not built. If #1281 merges, most of #1030's
  per-PR person rule is gone outside and inside a mission. No issue says which parts of #1030 remain (gap; no
  estimate in any source).
- **Paging and asks** depend on two other Features (#872, #880). No #889 story covers the case where they do not
  ship in v0.4.0.
- **Stale references.** #1014 names "Release #906's Tatkal Feature #920". On GitHub, #920 is a Story ("aayu
  critique records one review per seat …"), so #920 is an example number, not a link; the same number appears in
  examples in #997 and #998. Bug #1128 cites line numbers that moved (see
  [03-technical-design.md](03-technical-design.md)).

No source estimates any of these gaps.
