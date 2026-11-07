# UX design: Feature #975

## Source note

There is no UX design baseline (`origin/ledger` has no `specs/975/`). The Stories cite "interaction specs v4"
(touchpoints TP7 to TP91). Those documents are not in this repository or on the ledger. The planned words below are
only those that the issue bodies quote. Today's parts come from origin/dev
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Who uses it

- The founder, at a terminal in `aayu inbox`, and with `aayu mission <feature>` for the read-only mission view (#943
  C4). The web face of these cards is wave 3 (#976; #943 Assumptions).
- The agents (seats): they ask through `aayu ask` (#880, "Every seat asks only through `aayu ask` (Q14)", #938
  Assumptions) and read the answer in their thread. `aayu ask` does not exist on dev (it exits 2).

## Today on dev

The commands that the planned cards will run exist in part. Their real help (build of origin/dev, shortened):

```text
$ aayu mission --help
Start a Feature's approved mission, show its plan and clock, list what can start now, gate each milestone, stop,
resume, end, extend, recover, retry, or wake it, or run the hourly tick.
Usage:
  aayu mission <start|show|next|gate|budget|stop|resume|extend|tick|recover|retry|wake> [feature] [flags]

$ aayu plan --help
      --approve        Record the Engineering Manager's plan-time review of a plan that touches an [autonomy] never
                       layer (human identity only).

$ aayu halt --help
Stop every agent lane, loop, and workflow in the repo until resume.
      --reason string   Reason recorded with the halt.

$ aayu tell --help
      --publish           Publish the checked copy to the ledger (and the tag's GitHub Release); human identity only,
                          logged.
```

Cards on dev that wave 2 extends (words from `inboxKinds`, `internal/cli/inbox.go:94`):

| Kind | Word | Line | Built by |
|---|---|---|---|
| `baseline` | Baseline | "An agent drafted a part of a Feature; it counts as the baseline only when you approve it." | Story #1172 |
| `approve` | Approve to build | "Every baseline of a Feature is approved; it waits for you to approve it to build." | Story #1173 |
| `mission-start` | Mission start | "A Feature is approved to build; starting its mission locks its issues until the last milestone." | Story #1013 |
| `mission` | Mission | "A Feature's mission stopped at a step only a person takes; its stories wait for you." | Story #1001 |
| `mission-time` | Mission time | "A mission's clock passed one of its limits; past the last one, only a person lets it start new stories." | Story #1015 |
| `decision` | Decision | "A choice only you can make; the work that depends on it waits for your answer." | before v0.3.0 |

## Planned experience (planned, not built)

### The list in areas (#937, #940)

```text
Overdue 1 · Ship 3 · Answer 4 · Decide 2 · Ready 2 · Housekeeping 5        Snoozed 2 · With others 1
```

- Now, only while it has a card: "Now · Sev1 incident #1204 · production checkout errors at 4%" (#937 C2).
- After an area clears: "Ship is clear. 3 PRs merged. Next: Answer (4) · Enter" (#937 C3).
- Under a card title: "Unblocks: Story #851 › Feature #840 › v0.4.0 (Inbox lane) · Nakul idle since 11:20" (#937 C4).
- A refresh while reading: "1 new · R shows it" (#937 C7).
- Overdue banner: "3 decisions passed 24 h · Shuka, Hanu, and Nala wait · take them first" (#940 C2). Red age:
  "Overdue by 2 h · Shuka has waited 26 h. Reversible: the recommendation applies at 48 h unless you answer." (#940 C3).

### Put a card away (#939)

- Not now: "Skip #825 until: 1 it changes · 2 09:00 tomorrow · 3 Monday 09:00" (#939 C1).
- Snooze an ask: "Snooze until: 1 in 2 h · 2 tomorrow 09:00 · 3 when #817 merges" and "It is due in 21 h. A snooze past
  the due time asks for a reason, and Shuka is told." (#939 C4).
- Not sure: "1 Ask Atri for counsel · 2 Ask @alice (posts the question to her) · 3 Ask Shuka for more evidence" (#939
  C6).

### Asks (#938) and Refine (#941)

- "ANSWER · Shuka · Data & AI Architect asks · 3 h ago · due in 21 h", the options, "w Write an answer", and "two-way
  door" (#938 C1).
- Receipt: "✓ Answered as @rahulnakmol · posted on #812 · sent to Shuka's Amp thread · Shuka went on at 14:06" (#938
  C7). Thread archived: "Your answer is on #812, but Shuka's thread did not take it (the thread is archived). The next
  run of Shuka reads it from #812." (#938 C8).
- "Drafted #857 in Claude Code · 2 h ago · status: planned" and "Next: refine it in aayu inbox until it is Ready." (#941
  C1). "Ready check for #857: 6 of 8 pass." with keys "e body · f fields · a answer" (#941 C2).
- Make Ready off: "#857 is not Ready yet: 2 checks fail and 1 question is open." (#941 C5).

### Missions (#942, #943)

- Start: "Start the mission for #840: 7 stories in 3 milestones · up to 5 stories at once.", "2 missions run now. With
  this one, up to 15 story threads run in all.", and "1 Start · 2 Not now" (#942 C1).
- Stop: "Stop the mission for #840? Use this only in an extreme case. No new story starts. …  Write why, in one line."
  (#942 C3).
- Mission view: "Mission #840 · 5 of 5 running · 2 next · 1 waiting · milestone 2 of 3", tabs "Shape · Deliver", and
  the footer "s stop this story · S stop · e extend · a add a Tatkal story · T take over · o open session · ? keys"
  (#943 C1, C3, C14).
- State words: only "queued", "working", "waiting on you", "ready for review", and "shipped" (#943 C5).
- Three failed gates: "Mission #840 stopped: milestone 2's gate failed 3 times. It needs you." and "1 Retry the gate · 2
  Open the failed checks · 3 Stop the mission" (#943 C10).

### Decision cards (#945 to #950)

| Card | Head line | Source |
|---|---|---|
| PLAN | "PLAN · #853 · Vishwa asks you to review a plan that touches a never layer" | #945 C1 |
| BLOCKED | "BLOCKED · #846 · Nakul stopped at checkpoint 3/4" and "Why: the budget is used: $61.20 of $60 (actions log, 14:05)." | #945 C4 |
| RELEASE | "RELEASE · #1220 v0.4.0 · rolling snapshot …" and "1 Ask for the stable release (posts “Stable release: v0.4.0” as you) · 2 Not yet" | #946 C1 |
| PUBLISH | "PUBLISH · Release notes v0.4.0 · Sauti" and "Checked: 14 claims, each cites a proof pack · 0 without a source" | #946 C4 |
| INCIDENT | "INCIDENT · Sev1 · #1204 checkout errors at 4% · open 18 min" and "1 Approve as you · 2 Open on GitHub ↗ · 3 Refuse with a reason" | #947 C1 |
| Emergency stop | "Emergency stop? Use it only for a tool incident. It stops every agent, mission, loop, and workflow. To stop one mission, use Stop on its card. Write why, in one line." | #948 C1 |
| OVERRIDE | "OVERRIDE · #820 for Story #851 · Yama: a floor stops this PR" with "1 Waive the floor", "2 File a Bug and ship", "3 Stop" | #949 C1 |
| LOOPING | "LOOPING · #676 for Story #853 · 15 rounds in 41 h · no merge yet" and "1 Waive: off. No floor stops #676." | #949 C6 |
| FEEDBACK | "FEEDBACK · #1230 · from the v0.3.0 retro · …" and "1 Accept (Arjun grills it) · 2 Decline with a reason · 3 Route to a release" | #950 C1 |

### Feature lifecycle cards (#951 to #956)

| Card | Head line | Source |
|---|---|---|
| FEATURE | "FEATURE · #840 …", "Intent ✓ v1 · Spec ✓ v2 · Technical design: 2 of 5 reviews in · UX design: draft in Hanu's Amp thread ↗", "Next: baseline the UX design." | #951 C1 |
| Drift | "The plan of #840 changed after its baseline: 2 sections. Baseline it again?" | #951 C5 |
| REVIEWED | "REVIEWED · #840 technical design v2 · 5 of 5 reviews in · 2 gaps · 3 inputs · 4 guidance" | #952 C1 |
| APPROVE TO BUILD | "APPROVE TO BUILD · #840 · 7 stories · technical design v2 · UX design v3" and "Start is off: entry 853.1 (contract) has no choice." | #953 C1 |
| TRIAGE | "TRIAGE · Bug #1240 · …" and "1 Hotfix · 2 Carve out a Feature · 3 Need a better repro · 4 Not a bug" | #954 C1 |
| ACK | "ACK · Feature #840 · 3 changes after Approve to build (15:02) · 1 of 3 read" | #955 C1 |
| Delivery plan | "Delivery plan for #840 · indicative 3.5 d (2.8–4.6 d) · basis: 23 finished S and M stories in Engine, 28 days" | #956 C1 |

### Doors and confirmations

- One-way doors ask a second time: an ask (#938 C9), the stable release (#946 C3), publish (#946 C6), an incident
  approval (#947 C3), Emergency stop (#948 C1), Acknowledge all (#955 C6).
- A reason in one line is needed for Stop (#942 C3), Emergency stop (#948 C1), a 48 h override (#953 C3), and an evals
  override of 20 characters or more (#949 C12).
- A late story is amber, "never red" (#956 C4).

## Accessibility

The keyboard-only floor applies (`CONSTRAINTS.md`, `verify.floors.a11y.keyboard_only`). The bodies give no extra
accessibility criteria for wave 2; the shared key rules come from #934 in wave 1.

## Open design points

- #937 sets areas as the order, while Story #1175 set "decision order" for the morning cards
  ([2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2)). Which order shows when both apply is not
  stated (unverified).
- #948 Assumptions: "whether Stop can act on one story's thread is a question sent to the design thread." #942 C6 and
  `aayu mission stop --story` (`aayu mission --help`) suggest yes; the sources do not record the answer.
