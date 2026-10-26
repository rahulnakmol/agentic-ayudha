# rahulnakmol/agentic-ayudha#1100: The merge gate refuses a pre-authorised gate-fix PR, because it needs a Ready snapshot of the Feature the PR refs

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `3ac6d5cfa0871fa6b4055babda3ac273903f6f25cb199d54f0a24dabef2d349d` |
| Completion Goal hash | `584910984504473f2a6ed831dc3b611141f957c7b2ca69879f53b4651562e23a` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1100/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: PR #981 says `Refs #889` and `Aayudha-Gate-Fix: MS1 verify:inbox.ship-once`, `gate_fixes` of missions/889/plan.json lists `{pr: 981, milestone: "MS1", check: "verify:inbox.ship-once"}`, its `approval.sha256` equals the current specs/889/approved.json, and the check passes at the head; expected: the gate reports no problem about a Ready snapshot, and its verdict rests on that check and the floors
2. [api-trace] state: the same PR, and `inbox.ship-once` still fails at head 4c1d2e3; expected: the gate fails with `#981 fixes verify:inbox.ship-once in MS1 of #889, and that check still fails at 4c1d2e3`
3. [api-trace] state: PR #982 says `Refs #889`, and no `gate_fixes` entry names it; expected: the gate fails as today, with `story #889 has no Ready snapshot on the ledger`
4. [api-trace] state: the entry's `approval.sha256` differs from the current approved record, because #889 was approved again; expected: the gate fails with `the gate fix entry of #981 was made under an earlier approval of #889`
5. [api-trace] state: a story PR that says `Refs #20`, where #20 has a Ready snapshot; expected: the gate result is unchanged

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

The merge gate refuses a pre-authorised gate-fix PR. A fix PR of #1001 says `Refs #889` and `Aayudha-Gate-Fix: <milestone> <check>`. `verify` takes #889 as the PR's story (`StoryOf`, internal/cli/verify.go:304), finds no `specs/889/ready.md` on the ledger (verify.go:332-341), and `gateProblems` returns `story #889 has no Ready snapshot on the ledger` (verify.go:551-554). So `aayudha/merge-gate` fails, and the fix PR can never merge on proof.

### Steps to reproduce

1. With #1001 (PR #1090) merged, fail a milestone gate of a Feature so that `aayu mission gate` records fix PR #981 in `gate_fixes` of missions/889/plan.json.
2. The fix PR's body says `Refs #889` and `Aayudha-Gate-Fix: MS1 verify:inbox.ship-once`.
3. The merge gate runs `aayu verify` in CI on PR #981.

### Expected behaviour

The gate accepts a PR that the mission plan's `gate_fixes` names, with an approval that still matches, and checks it against the failed check it fixes: that check passes at the PR's head. A PR that only says `Refs #<feature>` and that no entry names fails as it does today.

### Actual behaviour

The gate fails with `story #889 has no Ready snapshot on the ledger`, because it treats every `Refs` target as a Story with a Ready snapshot.

### Completion Goal

1. [api-trace] state: PR #981 says `Refs #889` and `Aayudha-Gate-Fix: MS1 verify:inbox.ship-once`, `gate_fixes` of missions/889/plan.json lists `{pr: 981, milestone: "MS1", check: "verify:inbox.ship-once"}`, its `approval.sha256` equals the current specs/889/approved.json, and the check passes at the head; expected: the gate reports no problem about a Ready snapshot, and its verdict rests on that check and the floors
2. [api-trace] state: the same PR, and `inbox.ship-once` still fails at head 4c1d2e3; expected: the gate fails with `#981 fixes verify:inbox.ship-once in MS1 of #889, and that check still fails at 4c1d2e3`
3. [api-trace] state: PR #982 says `Refs #889`, and no `gate_fixes` entry names it; expected: the gate fails as today, with `story #889 has no Ready snapshot on the ledger`
4. [api-trace] state: the entry's `approval.sha256` differs from the current approved record, because #889 was approved again; expected: the gate fails with `the gate fix entry of #981 was made under an earlier approval of #889`
5. [api-trace] state: a story PR that says `Refs #20`, where #20 has a Ready snapshot; expected: the gate result is unchanged

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Run the gate on a listed fix PR whose check passes, on one whose check still fails, on an unlisted PR, on an entry under an old approval, and on a story PR.

### Environment

go test of internal/cli with a fake GitHub and ledger, in CI mode (`e.CI`); never production

### Technical Owner

engineering-manager

### Links

depends-on #1001

### Assumptions

#### Notes

#1001 (PR #1090) writes the `gate_fixes` entry and the `Aayudha-Gate-Fix` line (gateFixLine in that PR). This Bug changes only the merge gate (`gateProblems` and the snapshot read in internal/cli/verify.go). Ship's own check of a listed fix PR is #1002's (ST16 c17, c18). Fix PRs merge on proof and count against the Feature's budget (Decision #974).

#### Review guidance

- Check: Criterion 3: an unlisted `Refs #<feature>` PR still fails, so the entry is the only way past the snapshot check.
- Check: Criterion 4: the approval hash is read from the ledger, never from the PR body.
- Risk: a PR body can claim any check; the gate trusts only the entry in the plan.
- Ignore: the wording of the gate summary outside these lines.

#### Behaviours

1. says: The gate passes a fix PR only when the mission plan lists it under the current approval; rule: authority; source: criteria 1, 3, 4
2. says: A fix PR merges only when the check it fixes passes at its head; rule: never; source: criteria 2

### Open questions

- A failed check that is a required CI job or an eval, not a verify check: the gate cannot run it. Proposed: the gate names the job, and ship's required checks decide.
````
