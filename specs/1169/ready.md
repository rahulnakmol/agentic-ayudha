# rahulnakmol/agentic-ayudha#1169: aayu mission next holds a story whose depends-on target merged but is still open

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `abb224452999ebcf7054f387930cc8abc920fb4ea9a780415e6e5f6a13863966` |
| Completion Goal hash | `f840ca5f08043fd50fbfc27e64f45cf5b3367f5fecb807f77367e33903902b64` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1169/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Ready story #11 of #889 has `depends-on #10`, and #10 is open with status:merged; expected: `aayu mission next 889` marks #11 start
2. [api-trace] state: #10 is open with status:in-progress; expected: the list prints `waits #11: depends-on #10 is open`, as today
3. [api-trace] state: #11 depends on #40, a story of another Feature, open with status:merged; expected: `aayu ready 889` puts #11 in mission 1 of the plan, and `aayu mission next 889` marks it start
4. [api-trace] state: #10 is open with status:merged and has no status:ready; expected: `aayu mission next 889` never lists #10 as a start
5. [api-trace] state: #10 was closed as not planned (status:cancelled); expected: the list prints `waits #11: depends-on #10 was cancelled`, and #11 never starts

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A mission holds a story whose depends-on target has merged but is still open. `aayu mission next` counts a target as done only when the issue is closed (`internal/cli/missionnext.go:384`). Approve to build's mission plan does the same (`done[...] = si.GetState() == "closed"`, `internal/cli/mission.go:365`; `outsideDep`, `internal/cli/mission.go:516`). `aayu ship` sets only `status:merged` (`internal/cli/ship.go:1043`), and a story closes at the final release (`internal/cli/release.go:1157`). So after the first merge in a chain, every later story waits until the release. On 2026-10-30, 24 Stories and 10 Bugs in v0.4.0 were open with status:merged, so #889's mission would hold #1010, #1011, #1014, #1101, #1104 and #1126. Story #999's criterion 2 assumed the target "closes with status:merged". Found by the release review for Decision #1168.

### Steps to reproduce

1. Story #10 of #889 merges through `aayu ship`; it stays open with status:merged.
2. Ready story #11 of #889 has `depends-on #10` in its Links.
3. Run `aayu mission next 889`.

### Expected behaviour

The list marks #11 start, because its depends-on target has merged.

### Actual behaviour

The list prints `waits #11: depends-on #10 is open`, until a release closes #10.

### Completion Goal

1. [api-trace] state: Ready story #11 of #889 has `depends-on #10`, and #10 is open with status:merged; expected: `aayu mission next 889` marks #11 start
2. [api-trace] state: #10 is open with status:in-progress; expected: the list prints `waits #11: depends-on #10 is open`, as today
3. [api-trace] state: #11 depends on #40, a story of another Feature, open with status:merged; expected: `aayu ready 889` puts #11 in mission 1 of the plan, and `aayu mission next 889` marks it start
4. [api-trace] state: #10 is open with status:merged and has no status:ready; expected: `aayu mission next 889` never lists #10 as a start
5. [api-trace] state: #10 was closed as not planned (status:cancelled); expected: the list prints `waits #11: depends-on #10 was cancelled`, and #11 never starts

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Merge one story of a chain with `aayu ship`, then list the Feature's mission; repeat with a target in another Feature, an unmerged target, and a cancelled target.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and seeded issues; never production

### Checkpoints

1. A merged but open target no longer holds its dependent in `aayu mission next`, in the Feature and across Features (criteria 1, 3)
2. An unmerged or cancelled target still holds it, and a merged story is never started again (criteria 2, 4, 5)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

One helper decides "done" for a depends-on target: closed as completed, or open with status:merged. Every place that reads a target's state uses it: `aayu mission next`, the mission plan at Approve to build, `aayu mission show`, and the milestone gates. A cancelled target is never done: the dependent story waits, and the reason names it. Closing stories at ship instead was not chosen: release stamps each story at close (`internal/cli/release.go`), and that flow stays as it is.

#### Review guidance

- Check: every read of a depends-on target uses the one helper; grep for `GetState() == "closed"` in internal/cli/mission*.go.
- Check: criterion 5: a cancelled target never counts as done.
- Ignore: the wording of the waits line outside criterion 5.
- Risk: a story labelled status:merged by hand with no merged PR now unblocks its dependents; the label is written only by ship and release.

#### Behaviours

1. says: A story whose depends-on targets have all merged can start, before any release closes them; rule: limit; source: criteria 1, 3
2. says: A cancelled or unmerged target always holds its dependents; rule: never; source: criteria 2, 5

### Provenance

Request: "file the merged-but-open dependency Bug first, before any mission starts"
1. stated (request): a story whose depends-on target merged starts in its mission
2. default (scope): an unmerged target still holds its dependents, as #999 intends
3. stated (request): the same holds for a target in another Feature, at Approve to build and in the list
4. default (scope): a merged story that is not Ready is never started again
5. default (scope): a cancelled target is never done, so its dependents wait with the reason
````
