# rahulnakmol/agentic-ayudha#1170: The founder's day: decide in the morning, see each Feature proven by evening

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, feature |
| Body hash | `530158d10965f2e9575f32e2fbb2123676eeb43489a66ec0cdbdd973cbb3c5e1` |
| Completion Goal hash | n/a (this type has no Completion Goal) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1170/ready.md` |

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A founder decides each morning in one `aayu inbox` session, and sees each agreed Feature built and proven by evening. The morning has a card for each baseline and for Approve to build, next to the mission start card that exists. The evening has a card for each story agreed that morning, with each criterion next to its evidence, and says the day is proven only when all of it is. Agents prepare tomorrow's Feature overnight, and each Feature's Bugs come last. This is the 0.4 experience (Decision #1168): every day the founder sees what they made.

### Problem

The steps of a Feature exist as separate commands, so the founder's day is a list of device codes, not a decision. Each Feature takes 5 codes: 3 baselines (`aayu ready <f> --part`, `internal/cli/ready.go` `approve`), Approve to build (`internal/cli/readyapprove.go` `approveToBuild`), and mission start. The inbox shows decisions, scope, mission start, gate, and ship cards (`internal/cli/inbox.go:46-59`), and one session takes one device code (`internal/cli/presence.go` `presenceCheck`). It has no card for a baseline draft, Approve to build, the evening review, or what runs overnight. Proof packs hold the evidence of each criterion, but nothing puts it in front of the founder at the end of the day. A Bug filed during a mission is in no milestone. No aayu verb edits a filed body, so changes go through paste scripts (#1169 needed one on 2026-10-30). Measured on 2026-10-30: 212 of 400 PRs merged since 16 Oct carry a "Solo-mode bypass:" comment, and @rahulnakmol entered 27 `ship --solo-bypass` codes since 26 Oct (actions log).

### Scope

In: inbox cards for a waiting baseline draft (intent, spec, tech, UX) and for Approve to build, under the session's one code; a morning set that orders the day's cards and ends with "the day is set" and today's milestone per Feature; an evening card per story agreed that morning, with each criterion next to its evidence from the proof pack, accept or comment (a comment becomes a story through #1014); "the day is proven" only when every agreed criterion has its evidence, and otherwise what did not land, why, and the next step; an overnight card that sets what runs overnight (the next Feature's authoring, sweeps, and evals), bounded in hours and cost, with no merge overnight; a final BUGS milestone by default at Approve to build, which a Bug filed during the mission joins; and `aayu grill` editing a filed body before Ready.

Out: the areas view and asks from any seat (#975); the web inbox (#976); the deck and phone pairing (#977); per-PR rules inside a mission, which #1027 and #1030 own; changing who may approve.

### Expected paths

- `internal/cli/inbox.go`
- `internal/cli/inboxtui.go`
- `internal/cli/inboxwork.go`
- `internal/cli/inboxcheer.go`
- `internal/cli/inboxready.go`
- `internal/cli/ready.go`
- `internal/cli/readyapprove.go`
- `internal/cli/mission.go`
- `internal/cli/missionnext.go`
- `internal/cli/grill.go`
- `internal/grill/**`
- `internal/planning/**`
- `internal/ready/**`
- `internal/ui/**`
- `internal/verbs/registry.go`
- `skills/grill/SKILL.md`
- `docs/guide/skills/grill.md`
- `GLOSSARY.md`

### Links

depends-on #1029
depends-on #1030
````
