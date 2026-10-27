# rahulnakmol/agentic-ayudha#1129: A mission claim stays valid after its story moves to another Feature, so the Kanha starts it on the old mission's tool

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `dc808979995bb093180a329b2463a7c658e09baa55b0f442df090c033930322e` |
| Completion Goal hash | `19c9f3237cdfdff4bf9f22897f67a450d7150c94b208dc4cec3733b79f4f1adf` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1129/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu mission next 889` marked #61 start, then a person moved #61 under Feature #920; expected: `aayu build 61 --delegate cursor` exits 4 with `#61 changed since the list: now under #920, not #889; not started`, starts nothing, and releases #61's claim in missions/889/claims.jsonl
2. [api-trace] state: the list marked #61 start, then a person removed #61 from #889 and gave it no parent; expected: the same refusal and release, with `now under no Feature, not #889`
3. [api-trace] state: #61 is still under #889 when the Kanha builds it; expected: `aayu build 61 --delegate cursor` starts it with no `--asked`, as before
4. [api-trace] state: the list of Tatkal Feature #920 marked Bug #940 start, and #940 stays under #874, whose mission is done (Decision #915); expected: the build does not refuse #940 as moved

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A mission claim stays valid after a person moves its story to another Feature. `missionNext` claims each start in `missions/<f>/claims.jsonl` (internal/cli/missionnext.go), and `changedSinceList` (internal/cli/missionnext.go:484) accepts that claim when the mission is not paused, stopped, or ended and the story is still Ready. It does not check that the story is still under Feature f. `workitems.Validate` checks only the parent's type, so a move between two Features keeps the story Ready. The Kanha of f's mission then starts the story on f's tool and budget, and `runBuild` skips `askedGate` (internal/cli/build.go:263). CodeRabbit raised it on PR #1095 (missionnext.go:502, Minor), after inspect said merge.

### Steps to reproduce

1. `aayu mission next 889` marks #61 start and writes its claim line.
2. A person moves #61 from Feature #889 to Feature #920 before the Kanha builds it.
3. The Kanha of #889 runs `aayu build 61 --delegate cursor`.

### Expected behaviour

The build reads #61's live parent. #61 is not under #889 any more, so the build refuses, starts nothing, and releases the claim, as it does for a paused mission.

### Actual behaviour

The build accepts the claim from #889's list, skips the asked gate, and starts #61 on #889's tool.

### Completion Goal

1. [api-trace] state: `aayu mission next 889` marked #61 start, then a person moved #61 under Feature #920; expected: `aayu build 61 --delegate cursor` exits 4 with `#61 changed since the list: now under #920, not #889; not started`, starts nothing, and releases #61's claim in missions/889/claims.jsonl
2. [api-trace] state: the list marked #61 start, then a person removed #61 from #889 and gave it no parent; expected: the same refusal and release, with `now under no Feature, not #889`
3. [api-trace] state: #61 is still under #889 when the Kanha builds it; expected: `aayu build 61 --delegate cursor` starts it with no `--asked`, as before
4. [api-trace] state: the list of Tatkal Feature #920 marked Bug #940 start, and #940 stays under #874, whose mission is done (Decision #915); expected: the build does not refuse #940 as moved

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

List a story, move it to another Feature, then build it as the Kanha; repeat with no parent; build an unmoved story; build a Tatkal Bug that keeps its own parent.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake clock; never production

### Technical Owner

engineering-manager

### Links

depends-on #1005

### Assumptions

#### Notes

Build already reads the story's parent through `ready.Check` (internal/ready/ready.go:100); the fix uses that read and adds no GitHub call. Only a Kanha's `build --delegate` reads mission claims; a person's build is unchanged. A Bug that a Tatkal list starts keeps its own parent by design (Decision #915), so the check skips a Bug of a Tatkal plan.

#### Review guidance

- Check: Criterion 1: the claim line is released, not only the start refused.
- Check: Criterion 4: a Tatkal Bug under its own Feature still starts.
- Ignore: the wording of other refusal lines.

#### Behaviours

1. says: A Kanha never starts a story that left its mission's Feature after the list; rule: never; source: criteria 1, 2
2. says: A story that stays under its mission's Feature starts as before; rule: limit; source: criteria 3, 4
````
