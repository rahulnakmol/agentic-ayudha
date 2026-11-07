# #1170 Goal and intent

## Goal

The intent baseline `specs/1170/baselines/intent-v1.json` (approved by @rahulnakmol on 2026-10-31T09:44:52Z through
the device flow) states the goal in its `summary`:

> A founder decides each morning in one `aayu inbox` session, and sees each agreed Feature built and proven by evening.
> The morning has a card for each baseline and for Approve to build, next to the mission start card that exists. The
> evening has a card for each story agreed that morning, with each criterion next to its evidence, and says the day is
> proven only when all of it is. Agents prepare tomorrow's Feature overnight, and each Feature's Bugs come last. This
> is the 0.4 experience (Decision #1168): every day the founder sees what they made.

The intent pins the #1170 body at hash `530158d1…` (`intent-v1.json`, `pins`). The same body is in the Ready snapshot
`specs/1170/ready.md`. All three spec baselines and the tech baseline carry the same summary.

## Who it serves and the customer outcome

- The founder, @rahulnakmol, who is the `[autonomy] approver` (`.aayudha/config.toml`, `[autonomy]`). The Stories
  name him as the person in each criterion (for example [#1172](https://github.com/rahulnakmol/agentic-ayudha/issues/1172),
  criterion 1; [#1254](https://github.com/rahulnakmol/agentic-ayudha/issues/1254), Summary).
- The outcome: "every day the founder sees what they made" (`intent-v1.json`). Decision
  [#1168](https://github.com/rahulnakmol/agentic-ayudha/issues/1168) is titled "The founder's day: decide in one
  morning inbox session, see each Feature built and proven by evening, Bugs last, and author overnight".
- Later Stories widen it to "every pod with one person in one time zone" (PR
  [#1270](https://github.com/rahulnakmol/agentic-ayudha/issues/1270), At a glance) and to the cost of CI rounds
  ([#1250](https://github.com/rahulnakmol/agentic-ayudha/issues/1250), Summary).

## The problem

From the #1170 body (Problem section, frozen in `specs/1170/ready.md`):

- The steps of a Feature are separate commands, so "the founder's day is a list of device codes, not a decision".
- Each Feature takes 5 codes: 3 baselines (`aayu ready <f> --part`), Approve to build, and mission start.
- The inbox had no card for a baseline draft, Approve to build, the evening review, or what runs overnight.
- Proof packs hold each criterion's evidence, but nothing showed it to the founder at the end of the day.
- A Bug filed during a mission was in no milestone.
- No aayu verb edited a filed body. Changes went through paste scripts
  ([#1169](https://github.com/rahulnakmol/agentic-ayudha/issues/1169) needed one on 2026-10-30).
- Measured on 2026-10-30: 212 of 400 PRs merged since 16 Oct carry a "Solo-mode bypass:" comment, and @rahulnakmol
  entered 27 `ship --solo-bypass` codes since 26 Oct (actions log). This folder did not check these numbers again
  (unverified).

Two problems joined the Feature during its mission (`specs/1170/baselines/spec-v2.json` and `spec-v3.json`,
`changes`):

- spec v2 (#1250): "Each push to a ready PR runs the merge gate and a paid review for about 25 to 30 minutes, and PRs
  in this mission took two or three rounds, so most of a story's time went to waiting on CI."
- spec v3 (#1254): "Missions held new starts and asked the founder to act at any hour, so work waited for him between
  his sittings."

## Success measures

The Feature issue type has no Completion Goal (`specs/1170/ready.md`: "n/a (this type has no Completion Goal)"). The
sources give these measures:

| Measure | Source |
|---|---|
| Each Story and Bug meets "all N criteria" of its Completion Goal | each issue's Measure and Threshold sections (for example #1172: "all 6 criteria") |
| A Feature's morning needs one device code per inbox session, not 5 | #1170 Problem (5 codes); #1172 criterion 2 and #1173 criterion 2 (one code, "no second device code") |
| The evening says "The day is proven" only when every agreed criterion has evidence | #1177 criteria 1 and 2 |
| The Feature is done when BUGS passes its gate | `missions/1170/plan.json`: `"complete_when": "BUGS passes its gate"` |
| Fewer CI rounds per story | #1250 spec-v2 `changes` (why); no number is set as a target (unverified as a measure) |

No source sets a target for the share of solo-mode bypasses or the codes per day. A count of device codes per
Feature after this Feature is not recorded anywhere this folder could find (unverified).

## Scope

In (the #1170 body, Scope): inbox cards for a waiting baseline draft (intent, spec, tech, UX) and for Approve to
build, under the session's one code; a morning set that orders the cards and ends with "the day is set"; an evening
card per story agreed that morning, with evidence, accept or comment; "the day is proven" only with full evidence; an
overnight card bounded in hours and cost, "with no merge overnight"; a final BUGS milestone; and `aayu grill` editing a
filed body before Ready. Spec v2 and v3 added #1250 and #1254.

## Out of scope

From the #1170 body (Scope, "Out"):

- the areas view and asks from any seat ([#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975));
- the web inbox ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976));
- the deck and phone pairing ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977));
- per-PR rules inside a mission, which [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027) and
  [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) own;
- changing who may approve.

## A scope change after the baselines

Decision [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) ("Every agent PR into dev merges on
proof, at any risk and any hour", accepted by @rahulnakmol on 2026-11-02, `constrains #1170`) says in its context:
"Decision #1168 keeps overnight for authoring only, so no code merges between the evening review and the morning."
Its option 2 lets merges run at any hour. The #1170 Scope still says "with no merge overnight". No new intent or spec
baseline records this change; Story [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281) carries it
and is not in `specs/1170/approved.json`. A person must say which rule governs overnight merges. #1178's overnight
card merged on 2026-11-07 ([bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349)); this folder
found no ruling on the question, and it still matters for #1281 (its PR #1282 was closed unmerged when the mission
ended).
