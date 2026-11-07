# Spec: Feature #975

## Source note

There is no spec baseline: `origin/ledger` has no `specs/975/` files. The requirements below come from the
Completion Goals of the 20 Stories (`_data/subs.tsv`, feature 975). "Cn" means criterion n of that Story's Completion
Goal. Quoted words are the Story's own expected text. Every requirement is planned, not built, unless
[05-implementation-plan.md](05-implementation-plan.md) says a part exists on dev. Each Story also names the core
Feature that supplies its engine; this Feature only draws the card (for example #937, #940, #947 Assumptions).

## Functional requirements

### Areas and the day (#937, #939, #940, #944)

- FR1: The list shows an area strip in the order Overdue, Ship, Answer, Decide, Ready, Housekeeping, with Snoozed and With
  others to the side; a Now area comes first only while it has a card, such as an open Sev1 incident (#937 C1, C2).
- FR2: Tab and Shift+Tab move between areas; when an area clears, one line names the next area and its count (#937 C3).
- FR3: A card says what its answer unblocks ("Unblocks: Story #851 › Feature #840 › v0.4.0 (Inbox lane) · Nakul idle
  since 11:20"); the lane shows only when the story's Workstream is set; cards sort by what they unblock, then by
  age (#937 C4, C5, C6).
- FR4: A refresh never reorders the list while the person reads; "1 new · R shows it" waits until R (#937 C7).
- FR5: s on a card offers "until it changes", "09:00 tomorrow", and "Monday 09:00", with the morning time taken from
  `[time] business_hours`; the card comes back sooner on a new head, check, or comment (#939 C1, C2, C3).
- FR6: s on an ask offers times and an event ("when #817 merges"); a snooze past the due time asks for a reason and tells
  the asking seat (#939 C4, C5).
- FR7: ? on an ask offers "Ask Atri for counsel", "Ask @alice", and "Ask Shuka for more evidence"; asking a person moves the
  card to With others (#939 C6, C7).
- FR8: An ask shows its due time in amber ("Due in 3 h · Shuka waits"); past 24 h, every view starts with a banner that
  names the waiting seats; past 24 h a two-way ask is red with when the recommendation applies (#940 C1, C2, C3).
- FR9: At 48 h a two-way ask's recommendation applies provisionally and a Decide card offers ratify or revert; a one-way
  ask never applies on its own (#940 C4, C5).
- FR10: Every story card has one timeline line ("drafted 2 h · refined 40 m · Ready 14:02 · building 3 h · PR #858 ·
  merged · released v0.4.0") and a Q&A count; L opens the full timeline with sources (#944 C1, C2).

### Asks and Ready (#938, #941)

- FR11: An ANSWER card shows the asking seat, age, due time, the question, the options with the recommendation and its
  reason, "w Write an answer", and the door (#938 C1). A builder's question at a checkpoint offers "Change the
  story" and "Stop the build" too (#938 C2).
- FR12: An answer posts as the person and shows its receipt: posted on the issue, sent to the thread, and when the seat
  went on; an answer given first in Amp closes the card; a thread that did not take the answer shows an amber line
  (#938 C3, C4, C7, C8).
- FR13: The source line names the asking thread and who started it; t opens it, and over SSH copies its link; d moves
  the ask to its thread and keeps its due time (#938 C5, C6). A one-way ask needs a second press and the presence
  check (#938 C9).
- FR14: A drafted story is a REFINE card with "Ready check for #857: 6 of 8 pass." and one fix line for each failing row;
  f edits a field label and runs the check again (#941 C1, C2, C3).
- FR15: r asks the builder (Nakul) to read the story; its questions come back as ANSWER cards (#941 C4).
- FR16: Make Ready is off while checks fail or a question is open, and before the Feature's Approve to build; when the
  approval covers the body, Arjun runs `aayu ready <story>` with no presence check; when the story changed after
  approval, Enter runs one presence check and `aayu ready <feature> --ack <story>` as the person (#941 C5 to C8).

### Missions (#942, #943)

- FR17: A Start card for an approved Feature shows the story count, milestones, how many run at once, the total threads
  with other missions, time and cost ranges, the budget locked at start, and "1 Start · 2 Not now"; it starts in the
  person's tool from `[repos."<owner>/<repo>"] tool` and offers no host choice (#942 C1).
- FR18: Start runs `aayu mission start <f>` as the person; Stop asks for a one-line reason and a second Enter, runs
  `aayu mission stop <f> --reason <reason>`, and names any thread it could not reach (#942 C2, C3, C4).
- FR19: Resume, Recover, and Retry run `aayu mission resume|recover|retry <f>` as the person; stopping one story runs
  `aayu mission stop <f> --story <n> --reason <reason>`; each result shows on that mission's card (#942 C5, C6, C7).
- FR20: `[build] agents` (default 5) sets "up to N stories at once" on the Start card (#942 C8).
- FR21: M on a Feature card opens the mission view: running, next, and waiting rows with seat, host, session, checkpoint,
  and PR (#943 C1, C2).
- FR22: The view has two tabs, Shape and Deliver; Deliver opens first and names each row's step; Shape lists the stages
  from the Feature's baselines and the release queue position (#943 C3).
- FR23: `aayu mission <feature>`, `--json`, and `aayu mission --release <n>` print the same Shape and Deliver, the spend,
  one JSON object per story, and one line per mission of a release (#943 C4).
- FR24: Story states use only "queued", "working", "waiting on you", "ready for review", and "shipped", from one word list
  (#943 C5).
- FR25: An unreadable session says so and keeps its last state; a capped wake shows "paused: cap", never "quiet" (#943 C6).
- FR26: A PR of a later milestone waits for the earlier milestone's gate; milestone rows show each gate's result, run
  data, and spend; a third failed gate makes a Now card with Retry, Open, and Stop (#943 C7 to C10).
- FR27: The Team view counts all missions and threads; rows show waiting on you, an ended lease, a host limit, and a PR with
  no lease; a section under the rows is reserved for routing evidence (#900 ST8) (#943 C11, C12, C13).
- FR28: Mission view keys: s stops a story, S the mission, e extends, a adds a Tatkal story, T takes over, o opens the
  session; typed text changes nothing (#943 C14).

### Decision cards (#945 to #950)

- FR29: A PLAN card for a plan that touches a never layer shows the layers, checkpoints, contracts, rollback, and Vishwa's
  counsel; Approve runs one presence check and `aayu plan <n> --approve`; a plan whose hash changed approves
  nothing (#945 C1, C2, C3).
- FR30: A BLOCKED card names why a build stopped (for example a used budget, with its source) and offers a fresh budget
  that is recorded on the ledger before the build continues from its checkpoint; a delegated cloud build that needs
  a person links its task (#945 C4, C5, C6).
- FR31: A RELEASE card shows each gate with its run and age and offers "Ask for the stable release (posts “Stable release:
  v0.4.0” as you)"; it is off while the rolling gate is red; it is a one-way door with one presence check; an
  agent that posts it is still refused by the hook (#946 C1, C2, C3).
- FR32: A PUBLISH card counts the claims and their proof packs; Publish is off while a claim cites no pack; Publish runs
  `aayu tell --publish` as the person after a one-way confirm (#946 C4, C5, C6).
- FR33: An INCIDENT card in Now shows the action, blast radius, undo, rate limit, and environment; Approve uses GitHub's
  pending-deployment approval as the person; it is off when the environment has no required reviewers; close needs
  the reviewed postmortem; any view shows a Sev0 or Sev1 banner (#947 C1 to C5).
- FR34: Emergency stop, from the command bar only, asks for a reason and a second Enter; while it holds, every view says
  who stopped and why and Now has a RESUME card; the budget pause shows its spend and when it ends; Resume runs
  one presence check and `aayu resume` as the person; no mission or story card offers a halt (#948 C1 to C5).
- FR35: An OVERRIDE card shows a PR that only a floor or an eval bar stops, with the measured value, the floor, and the
  run; choices are "Waive the floor", "File a Bug and ship", and "Stop"; it does not show when another check is red,
  inspect says fix, or a P0 is open (#949 C1, C2, C3).
- FR36: Waive runs `aayu decide <story> --waiver <floor>` as the person and merges nothing; File a Bug files it through
  `aayu grill bug` and merges nothing, because "a red floor never ships without a waiver" (#949 C4, C5).
- FR37: A LOOPING card shows rounds, hours, open findings per round, and review spend; `[inbox] loop_rounds = 3` and
  `loop_hours = 8` are the defaults; only fix rounds count (#949 C6, C7).
- FR38: File a Bug and ship moves open P2 and P3 findings to one Bug and ships the head the card showed; it refuses while a
  P0 or P1 is open; Stop runs `aayu mission stop <f> --story <n> --reason <reason>` and blocks the Story (#949 C8,
  C9, C10).
- FR39: A Release whose rolling candidate fails only `evals-gate` gets an OVERRIDE card; override runs
  `aayu release <n> --to rolling --override-evals --reason <reason>` (20 characters or more) from the terminal and the web (#949 C11,
  C12).
- FR40: `--list --json` has items of kind `override` and `looping` with their facts and sources (#949 C13).
- FR41: A FEEDBACK card offers Accept (Arjun grills it), Decline with a reason, and Route to a release (#950 C1 to C4).

### Feature lifecycle cards (#951 to #956)

- FR42: A FEATURE card lists each part's stage and the next step; UX design says "not needed" when the footprint has no
  L01 (#951 C1, C2).
- FR43: A BASELINE card approves through one presence check and `aayu ready <f> --part <part>`; a draft that changed after
  it was opened baselines nothing; a plan that drifted after its baseline offers "Baseline again", "Show the diff",
  and "Ask Vishwa why" (#951 C3, C4, C5).
- FR44: A nudge shows only when the next step is the person's (#951 C6).
- FR45: A REVIEWED card gathers the seats' findings and offers "Resolve in a grill session"; over SSH it copies
  `aayu grill feature <f> --findings`, locally it opens the owner's thread; after 24 h it names the missing seat; after the
  session it shows the record (#952 C1 to C4).
- FR46: An APPROVE TO BUILD card shows the stories, approved designs, time, cost, budget lock, locks, and one row for each
  high-risk entry with "Auto-merge", "Review the PR", and "Review before build"; an authority change is never
  auto-merged; Start stays off until every entry has a choice and no gap is open (#953 C1, C2, C3).
- FR47: Start runs one presence check and `aayu ready <f>` with the reason (past 48 h) and the choices recorded; A sets
  auto-merge for every entry except authority entries; a high-risk change declared later is held (#953 C3, C5).
- FR48: A drifted baseline after approval holds the stories until a new approval (#953 C4).
- FR49: A TRIAGE card shows the repro fields and Shesha's counsel and offers Hotfix, Carve out a Feature, Need a better
  repro, and Not a bug; missing fields turn Hotfix and Carve out off; each runs `aayu triage <n> --decide <choice>`
  as the person after one presence check (#954 C1 to C4).
- FR50: An ACK card lists each story added or changed (Completion Goal, effort, or title) after Approve to build, old →
  new; Space marks a change read; Acknowledge runs `aayu ready <f> --ack <story>` for each read change; a change
  made after reading clears the mark; a text-only edit needs nothing; Acknowledge all records mode "delegated"
  (#955 C1 to C6).
- FR51: p on a Feature shows its delivery plan: an indicative time with range and basis, waves, and the critical path;
  with fewer than 5 finished stories of an effort it uses the defaults XS 2 h, S 4 h, M 1 d, L 2 d (#956 C1, C2).
- FR52: A story card shows its p50 and range; a late story is amber, never red; a merged story shows actual against
  p50; `aayu inbox --today` shows the 28-day accuracy line (#956 C3 to C6).

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | Every write that a person owns runs as the person after a presence check; human-only verbs (Stop, Resume, Emergency stop, the stable release, publish, incident approval) stay human-only | #942 Assumptions ("Stop is human-only"), #946 C3, #947 C3, #948 C4 |
| Security | An agent that posts "Stable release:" is still refused by the hook | #946 C3; AGENTS.md, "Promotion and release" |
| Security | Ops approvals go through GitHub's required reviewers, never around them | #947 Measure and C4 |
| Security | A red floor or eval bar never ships without an accepted waiver; a P0 or P1 never moves to a Bug on its own | #949 Assumptions (ground rule 7, `CONSTRAINTS.md`), C9 |
| Security | An authority change is never auto-merged | #953 C3, C5 |
| Reliability | A result or error shows on the card of the mission it acted on, never on another | #942 C7 |
| Reliability | A stale plan, draft, or story read approves nothing (hash check) | #945 C3, #951 C4, #955 C4 |
| Operability | Times come from config: `[decisions] sla_hours = 24`, `provisional_after_hours = 48`, `[time] business_hours`, `[build] agents = 5`, `[inbox] loop_rounds = 3`, `loop_hours = 8` | #939 C3, #940 Assumptions, #942 C8, #949 C7; `defaults.toml` |
| Operability | In solo mode `solo_provisional_after_hours = 24` shortens the Decision window; #883 must set 48 h for two-way asks "or criterion 4 fails in solo mode" | #940 Assumptions; `defaults.toml` `[decisions]` |
| Cost | The Start and Approve to build cards show the budget, locked at the top of the range | #942 C1, #953 C1 |
| Accessibility | Every Completion Goal state is reachable by keyboard alone | `CONSTRAINTS.md`, `verify.floors.a11y.keyboard_only` |
| Quality | Changed lines need 90% coverage | `CONSTRAINTS.md`, `verify.floors.coverage.min_diff_percent` |

## Requirements not yet in the bodies

- Each Story's Review guidance and Behaviours are "kept in the Feature's drafts" until Features #887 and #891 add the
  fields (each Story's Assumptions). Where those drafts live is unverified.
- `aayu release <n> --to rolling --override-evals` "one Tatkal story adds it (not filed yet)" (#949 Assumptions).
- The mode field (read or delegated) in `specs/<feature>/ack-<k>.json` is a field "this story asks #877 to keep"
  (#955 Assumptions).
