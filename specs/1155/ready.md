# rahulnakmol/agentic-ayudha#1155: A person's dismissal never clears a carried finding on a PR with no Story

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `1ddca16e7b32dce329401835b30a7beb42ebf9a18f595de44f4e1a832751400b` |
| Completion Goal hash | `af4047b0d77f8f041f727b611a9325a28f84bfec77a9b70791e4898951822ea0` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1155/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a PR with no Story, a P1 on `internal/report/percent.go` from the reviewer at the first head, that P1 dismissed by a person on the ledger, then a second head with the same patch and a merge reply; expected: inspect at the second head says merge, with no findings, and the P1 in its dropped list as dismissed
2. [api-trace] state: the same PR and heads with no dismissal on the ledger; expected: inspect at the second head says fix on the carried P1, as today
3. [api-trace] state: a Story PR with a dismissed P1 (TestInspectDismissAndPeer); expected: the dismissal applies, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A person's `aayu inspect <pr> --dismiss <id>` never clears a carried P0 or P1 on a PR with no Story. The gate reads the repository's dismissals only inside `if ins.Story > 0` (`internal/cli/inspect.go:337-371`, `ins.Dismissed = readDismissed(ctx, w)`). On a chore PR, `Input.Dismissed` stays empty. `carryForward` (`internal/inspect/inspect.go:753`) adds the finding back, and `Suppress` (`internal/inspect/inspect.go:785`) has no dismissal to drop it with. On PR 1145 (Bug #1133), @rahulnakmol dismissed P1 `643c24c7e5` (ledger commit 90c6bd08, 19:09Z). Round 3 at fdec5e4 still said fix on that same carried P1 (comment 6466542287, 20:00Z), and the PR reached the round cap.

### Steps to reproduce

1. A PR with no Story reference gets a reviewer reply with a P1 finding on a file.
2. A person dismisses the finding with `aayu inspect <pr> --dismiss <id> --reason …`; the dismissal is on the ledger.
3. A new head leaves that file's patch unchanged, and the reviewer replies merge.
4. The merge gate runs `aayudha inspect <pr> --ci`.

### Expected behaviour

The carried P1 is dropped as dismissed, and inspect says merge, as it does on a Story PR.

### Actual behaviour

Inspect says fix on the carried P1. The dismissal has no effect, and each push uses one more judged round.

### Completion Goal

1. [api-trace] state: a PR with no Story, a P1 on `internal/report/percent.go` from the reviewer at the first head, that P1 dismissed by a person on the ledger, then a second head with the same patch and a merge reply; expected: inspect at the second head says merge, with no findings, and the P1 in its dropped list as dismissed
2. [api-trace] state: the same PR and heads with no dismissal on the ledger; expected: inspect at the second head says fix on the carried P1, as today
3. [api-trace] state: a Story PR with a dismissed P1 (TestInspectDismissAndPeer); expected: the dismissal applies, as today

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the gate on a PR with no Story through a P1, a person's dismissal, and a second head with the same patch; then run it again with no dismissal, and on a Story PR.

### Environment

go test of internal/cli with a fake GitHub and a local ledger, in CI mode; never production

### Technical Owner

engineering-manager

### Checkpoints

1. A dismissed P1 that carries on a PR with no Story no longer blocks it (criteria 1)
2. An undismissed carried P1 still blocks, and a Story PR's dismissal works as today (criteria 2, 3)

### Assumptions

- The gate reads the dismissals with the same human token it uses for a Story PR's ledger reads; when that token or the ledger is not there, a PR with no Story gets no dismissals and is not blocked for that.
- Rule 7 does not change: only a person's dismissal on the ledger drops a carried P0 or P1.

#### Notes

The same pull request also tightens the coverage floor in `ratchets.toml` to 73.6 %. Every pull request to dev fails the build job's coverage ratchet until one does: coverage on dev is 74.1 % against a floor of 73.1 %.

#### Review guidance

- Check: Criterion 1: the dismissal is read from the ledger for a PR with no Story, and only a person's dismissal drops the carried P1.
- Check: Criterion 2: a carried P1 with no dismissal still blocks.
- Risk: a ledger read error on a PR with no Story must not fail the gate.
- Ignore: the ratchet floor value, which the ratchet tool writes.

#### Behaviours

1. says: A person's dismissal drops a carried P0 or P1 on every PR, with or without a Story; rule: never; source: criteria 1, 3
2. says: A carried P0 or P1 with no dismissal blocks the merge; rule: never; source: criteria 2

### Open questions

- answered: build it with the ratchet tighten in the same pull request, as the request says.

### Parked ideas

- none

### Provenance

Term: "P1" → the priority:P1 label, and the severity of the carried finding; "P1, XS, Ready"
Term: "XS" → the effort:XS label; "P1, XS, Ready"
Request: "file the Bug with grill: under #889, in #906, P1, XS, Ready. Name it after the effect: "a person's dismissal never clears a carried finding on a PR with no Story". Include one test that fails without the fix: a chore PR whose dismissed P1 is carried still gets merge."
1. stated (request): "a chore PR whose dismissed P1 is carried still gets merge"
2. stated (request): "a person's dismissal never clears a carried finding on a PR with no Story"
3. stated (request): "a person's dismissal never clears a carried finding on a PR with no Story"
````
