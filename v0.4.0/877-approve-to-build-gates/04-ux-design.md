# UX design: Feature #877

The surface is the terminal (`aayu ready`), the device-flow prompt, the Ready snapshot comment, and the inbox
Approve to build card (Story [#1173](https://github.com/rahulnakmol/agentic-ayudha/issues/1173), Feature #1170).
The footprint has no L01 (`specs/877/design.json`), so there is no UX baseline.

## Commands and flags (delivered)

From the `ready` entry in `internal/verbs/registry.go`:

| Command | Who | What it does |
|---|---|---|
| `aayu ready <feature> [--milestones 'MS1=20,21 MS2=22,23'] [--agents N]` | a person | Approve to build: approves the Feature's baselines, stories, and budget through GitHub's device flow; refused for an agent lane. Writes `specs/<f>/approved.json` and `missions/<f>/plan.json`. |
| `aayu ready <story>` | Arjun (the bot) | Makes a Story Ready only while its Feature's approval covers it. |
| `aayu ready <issue> --check` | anyone | Validate only; write nothing. |

The positional help reads: "Work item to make Ready. On a Feature this is Approve to build: a person approves its
baselines, stories, and budget through GitHub's device flow, and it is refused for an agent lane."

## Messages (delivered, from the code)

| Moment | Text | Code |
|---|---|---|
| Effect before the device code | "Approves #840 to build: 6 stories (#851-#856), intent v1, spec v2, technical design v1, budget up to $360 (6 x $60, [budget] per_story_usd)" | `ready.Effect` |
| Approved | "Approved #840 to build as @rahulnakmol, approval 1. Next: Arjun makes its 6 stories Ready." | `approveRun.ready` |
| Already approved | "#840 is approved to build already: approval 1 by @… agrees it as it is now. Next: …" | `approveRun.ready` |
| Agent lane | "Approve to build (aayu ready on a Feature) is a person's decision and is refused for an agent lane (AAYU_AGENT=1)" | `Env.laneRefusal` |
| A part missing | "Approve to build is off: the UX design is needed (L01) and not approved. Next: aayu ready 840 --part ux --request" | `approveRun.prepare` |
| No Initiative | "Approve to build is off: #840 has no Initiative parent (…). Next: add #840 as a sub-issue of its Initiative, then aayu ready 840" | `approveRun.prepare` |
| Open gap | "Approve to build is off: technical design v1 has 1 open gap (…). Next: aayu grill feature 840 --findings" | `approveRun.prepare`, `ready.OpenGaps` |
| Changed while waiting | "The stories of #840 changed while the code was waiting; nothing was approved." | `approveToBuild` |
| Story with no parent | "#857 has no parent Feature. A Story is Ready only under a Feature approved to build (Decision #861)." | `storyCovered` |
| Feature not approved | "Feature #840 is not approved to build. A person runs aayu ready 840." | `earlyCovered` |
| Feature changed | "#840 changed after Approve to build: technical design v1 to v2. Its stories wait until a person approves it again." | `storyCovered`, `featureChanges` |
| Story changed or added | "#853 changed after Approve to build (effort S to M). A person acknowledges it: aayu ready 840 --ack 853" | `storyCovered` |
| Snapshot row | "Approved to build \| #840 approval 1 by @rahulnakmol" | `storyCovered` return value |

The "acknowledges it" messages name `--ack`, which does not exist yet. The structured next command points to
`aayu ready <f> --part spec --request` until #1082 ships (`internal/cli/readycover.go` comment).

## Inbox card (delivered under Feature #1170)

The Approve to build card shows the stories and Bugs, the budget, the agents, the milestones, and a placeholder:
"High-risk entries by story: none shown. They come with #1029, which has not merged, so the card cannot read them
yet." It comes after the Feature's Baseline cards, in the same session, so one device code covers the Feature
(`internal/cli/inboxapprove.go`; PR [#1225](https://github.com/rahulnakmol/agentic-ayudha/issues/1225)).

## Planned, not built

- `aayu ready <f> --ack <stories> [--request --draft ack.json]`: effect "#853: effort S to M. #858: added (S).
  Budget $360 to $420." and the snapshot row "Acknowledged | #840 ack 1 by @rahulnakmol" (#1082 CG 3, 5).
- High-risk choices at Approve to build: "High-risk changes: 3 in 2 stories. Choose for each one.", the prompt
  "1 auto-merge · 2 review the PR · 3 review before build", `--choice 853.1=auto-merge`, `--choice
  all=auto-merge`, and "No high-risk change in #840's 7 stories: every story merges on proof." (#1029 CG 1-8).
  #1029 Notes say "Its words go into the UX baseline first, because v44 has no frame for them."
- Receipt comment on the Feature: "&lt;!-- aayudha:presence --&gt; Approved #840 to build, approval 1. Record
  specs/840/approved.json, sha256 &lt;hash&gt;." and the refusal "#840's approval 1 has no receipt that
  @rahulnakmol wrote (T23), so the ledger record is not trusted. A person approves it again: aayu ready 840"
  (#1083 CG 2, 4).
- Grill refusal: "A Story needs a parent Feature: it is Ready only under a Feature approved to build (Decision
  #861). Pass --parent &lt;feature&gt;, or file it under the Release's Tatkal Feature." (#1086 CG 1).
- Inbox card for acknowledgments: Story #955 (Feature #975) "Acknowledge each story added or changed after Approve
  to build" (`_data/subs.tsv`), outside this Feature.

## Agent experience

`skills/ready/SKILL.md` has the section "A Feature: Approve to build is a person's", which tells the agent that
`aayu ready <feature>` is Approve to build and is a person's decision. `skills/grill/SKILL.md` still tells it a Story "is Ready without" a parent Feature (line
383), which Bug #1086 fixes.

Accessibility: no source states a requirement beyond plain terminal text.
