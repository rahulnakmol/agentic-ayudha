# rahulnakmol/agentic-ayudha#995: A person approves a Feature to build, and the ledger keeps what was agreed

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `3fb20df001443ba9da3488d2e0b3c6f4b1b50293cebbb8fbab0939fe513a3b31` |
| Completion Goal hash | `63730f1ee9f19d0a22596f6543c8780d9f44448ab7b49faa7ee3218db02cbea0` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/995/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: an agent lane (AMP_THREAD_ID set) runs `aayu ready 840` on Feature #840, and an MCP client calls the ready tool for #840; expected: both are refused with exit 4: "Approve to build (aayu ready on a Feature) is a person's decision and is refused for an agent lane (AMP_THREAD_ID)"; no ledger commit, label, or comment is written
2. [api-trace] state: Feature #840 under Initiative #870 with intent v1, spec v2, and technical design v1 approved and not drifted, 6 stories, and [budget] per_story_usd = 60; a person runs `aayu ready 840` in a terminal; expected: before the device code, the effect reads "Approves #840 to build: 6 stories (#851-#856), intent v1, spec v2, technical design v1, budget up to $360 (6 x $60, [budget] per_story_usd)"; after one presence check, specs/840/approved.json on the ledger holds version 1, by, at, presence (login, via, permission, actions record), baselines (part, version, hash), stories (number, title, effort, Completion Goal hash), budget (per_story_usd, stories, total_usd), and dismissals (empty until criterion 7)
3. [api-trace] state: the same approval after the ledger write; expected: Arjun posts the Ready snapshot of #840, labels it status:ready, and the result reads "Approved #840 to build as @rahulnakmol, approval 1. Next: Arjun makes its 6 stories Ready."
4. [api-trace] state: one condition fails at a time on #840: the UX part is needed (L01 in the footprint) and not approved; the technical design changed after it was approved; the Feature has no Initiative parent; expected: each run is refused with exit 4 and one line that names the condition and the next command, for example "Approve to build is off: the UX design is needed (L01) and not approved. Next: aayu ready 840 --part ux --request"; nothing is written
5. [api-trace] state: the Completion Goal of #853 changes while the device code waits; expected: "The stories of #840 changed while the code was waiting; nothing was approved." and no ledger commit
6. [api-trace] state: #840 is approved, then technical design v2 is approved with its drift section, and a person runs `aayu ready 840` again; expected: the effect shows the drift section of technical design v2 (what changed, why it matters, the need behind it); specs/840/approved.json now holds version 2; the ledger log keeps version 1
7. [api-trace] state: critique.State of #876 (its test fake until #876 merges) reports for #840 the open gap tech-v1/quality-architect/1 and 2 of 5 reviews in with fewer than [critique] review_wait_hours passed; then the gap is dismissed with a reason and every review is in, and a person runs `aayu ready 840`; expected: the first runs are refused: "Approve to build is off: technical design v1 has 1 open gap (tech-v1/quality-architect/1). Next: aayu grill feature 840 --findings" and "3 of 5 reviews are not in; Approve to build opens when they are in or [critique] review_wait_hours have passed"; the last shows "1 dismissal ratified: tech-v1/quality-architect/1 (out of scope for v0.4.0)" before the device code, and specs/840/approved.json holds that dismissal (finding, reason, words)

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want aayu ready <feature> to be my Approve to build, showing the stories, the approved designs, and the budget, so that no story under a Feature is built until I agree all of it once.

### Completion Goal

1. [api-trace] state: an agent lane (AMP_THREAD_ID set) runs `aayu ready 840` on Feature #840, and an MCP client calls the ready tool for #840; expected: both are refused with exit 4: "Approve to build (aayu ready on a Feature) is a person's decision and is refused for an agent lane (AMP_THREAD_ID)"; no ledger commit, label, or comment is written
2. [api-trace] state: Feature #840 under Initiative #870 with intent v1, spec v2, and technical design v1 approved and not drifted, 6 stories, and [budget] per_story_usd = 60; a person runs `aayu ready 840` in a terminal; expected: before the device code, the effect reads "Approves #840 to build: 6 stories (#851-#856), intent v1, spec v2, technical design v1, budget up to $360 (6 x $60, [budget] per_story_usd)"; after one presence check, specs/840/approved.json on the ledger holds version 1, by, at, presence (login, via, permission, actions record), baselines (part, version, hash), stories (number, title, effort, Completion Goal hash), budget (per_story_usd, stories, total_usd), and dismissals (empty until criterion 7)
3. [api-trace] state: the same approval after the ledger write; expected: Arjun posts the Ready snapshot of #840, labels it status:ready, and the result reads "Approved #840 to build as @rahulnakmol, approval 1. Next: Arjun makes its 6 stories Ready."
4. [api-trace] state: one condition fails at a time on #840: the UX part is needed (L01 in the footprint) and not approved; the technical design changed after it was approved; the Feature has no Initiative parent; expected: each run is refused with exit 4 and one line that names the condition and the next command, for example "Approve to build is off: the UX design is needed (L01) and not approved. Next: aayu ready 840 --part ux --request"; nothing is written
5. [api-trace] state: the Completion Goal of #853 changes while the device code waits; expected: "The stories of #840 changed while the code was waiting; nothing was approved." and no ledger commit
6. [api-trace] state: #840 is approved, then technical design v2 is approved with its drift section, and a person runs `aayu ready 840` again; expected: the effect shows the drift section of technical design v2 (what changed, why it matters, the need behind it); specs/840/approved.json now holds version 2; the ledger log keeps version 1
7. [api-trace] state: critique.State of #876 (its test fake until #876 merges) reports for #840 the open gap tech-v1/quality-architect/1 and 2 of 5 reviews in with fewer than [critique] review_wait_hours passed; then the gap is dismissed with a reason and every review is in, and a person runs `aayu ready 840`; expected: the first runs are refused: "Approve to build is off: technical design v1 has 1 open gap (tech-v1/quality-architect/1). Next: aayu grill feature 840 --findings" and "3 of 5 reviews are not in; Approve to build opens when they are in or [critique] review_wait_hours have passed"; the last shows "1 dismissal ratified: tech-v1/quality-architect/1 (out of scope for v0.4.0)" before the device code, and specs/840/approved.json holds that dismissal (finding, reason, words)

### Measure

Completion Goal criteria observed

### Threshold

all 7 criteria

### Scenario

Approve Feature #840 from an agent lane and over MCP (refused), then from a person's terminal with each blocking condition in turn, a change during the device code, and a second approval after a new technical design.

### Environment

local checkout with the GitHub fake and a synthetic ledger (internal/github/fake); never production

### Assumptions

#### Review guidance

- Check: Criterion 1: the Feature path of `ready` refuses an agent lane and MCP before any write.
- Check: Criterion 5: the pins are read again after the presence check, as `scope --accept` does (internal/cli/scope.go:1060-1064).
- Check: Criterion 7: S1 reads #876's critique.State through its interface; until #876 merges, the proof uses its test fake, so S1 builds right after #874 (D1 A).
- Risk: internal/cli/ready.go: who may approve.
- Risk: internal/ready/: the approval record and what it pins.
- Ignore: category docs
- Ignore: settled #903 (one Approve to build per Feature)
- Ignore: other stories' lines on the same approval: the Outcome line (#985), time, cost and the lock (#889, #953), and Start / Not now (#953, #942)

#### Behaviours

1. says: Nothing under a Feature is built until a person approves the Feature to build, with its stories and budget shown; rule: authority; source: criteria 1, 2
2. says: Approve to build is off while a needed part is not approved or has drifted, a gap is open, or the reviews are not in; rule: never; source: criteria 4, 7
3. says: A person approves only what the screen showed; rule: never; source: criteria 5
````
