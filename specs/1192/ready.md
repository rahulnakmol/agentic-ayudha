# rahulnakmol/agentic-ayudha#1192: ship asks for a second code after a person approved the head with inspect --approve

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `cfcb727da06a78b78fdaf814b33ae580a62f81e21f7a36093dfa6c6e9a8a8df0` |
| Completion Goal hash | `9fa006bdd64e2fed7e62dc152680e1c1c4a3031244ef7cb08e70184f943056c0` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1192/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: PR #20 is outside the narrow set, its head has a person's `inspect --approve` record on the ledger, and its gate passed with a pack; expected: `aayu ship 20` merges it with no device code, and the ship record names the approval as the human pass
2. [api-trace] state: the approval names an older head than the PR's head; expected: ship refuses as today
3. [api-trace] state: the approval was written by a login that is not `[autonomy] approver`; expected: ship refuses as today
4. [api-trace] state: the PR touches the narrow set (`[autonomy] always_person`); expected: ship still needs `--solo-bypass` or the inbox, as today
5. [api-trace] state: the head has no approval and its rung is human; expected: ship refuses as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

After a person approves a PR's head with `aayu inspect <pr> --approve`, `aayu ship <pr>` still stops for a human pass, so the founder enters a second device code for the same head. Merge on proof needs the head's review rung in `[autonomy] proof_rungs` (`internal/cli/shipproof.go:391-392`), which lists A, B, and C (`.aayudha/config.toml`). A recorded person's approval is rung "human", so ship refuses with `the review rung at the head is human, not one of A, B, C`, comments Ready to ship, and waits for `aayu inbox` or `aayu ship <pr> --solo-bypass`. Seen on PR #1184 at 3363488 on 2026-10-31: @rahulnakmol approved the head through the device flow (inspect/approved.jsonl), the gate passed with a pack, and ship asked for him again (comment 6541802240).

### Steps to reproduce

1. On a Bug PR outside the narrow set, rung B fails and the reviewer app has seen the head, so inspect says no-reply.
2. The person runs `aayu inspect <pr> --approve --reason …` and enters the device code.
3. The gate re-runs on the head and passes with a pack.
4. An agent runs `aayu ship <pr>`.

### Expected behaviour

Ship merges on proof: the person's recorded approval of this exact head is the human pass.

### Actual behaviour

Ship refuses with `no human pass: … the review rung at the head is human`, comments Ready to ship, and the person enters a second code.

### Completion Goal

1. [api-trace] state: PR #20 is outside the narrow set, its head has a person's `inspect --approve` record on the ledger, and its gate passed with a pack; expected: `aayu ship 20` merges it with no device code, and the ship record names the approval as the human pass
2. [api-trace] state: the approval names an older head than the PR's head; expected: ship refuses as today
3. [api-trace] state: the approval was written by a login that is not `[autonomy] approver`; expected: ship refuses as today
4. [api-trace] state: the PR touches the narrow set (`[autonomy] always_person`); expected: ship still needs `--solo-bypass` or the inbox, as today
5. [api-trace] state: the head has no approval and its rung is human; expected: ship refuses as today

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Approve a PR head with inspect, pass its gate, and ship it; repeat with an older head, another login, a narrow-set PR, and no approval.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake device flow; never production

### Checkpoints

1. A person's recorded approval of the exact head is the human pass for ship (criteria 1)
2. Any other case refuses as today (criteria 2, 3, 4, 5)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The approval record is already the human review the merge gate trusts (`approveHead`, internal/cli/inspect.go:870), written after a device flow by `[autonomy] approver`. Ship reads the same record; it does not add rung "human" to proof_rungs. ship.go and shipproof.go are in the narrow set, so the PR waits for a person's look under Decision #1027.

#### Review guidance

- Check: criterion 2: the approval's head must equal the PR head.
- Check: criterion 4: the narrow set still needs a person per PR.
- Ignore: the wording of the ship record.
- Risk: trusting a ledger record a bot could write; #1083 adds a receipt GitHub shows the person wrote.

#### Behaviours

1. says: A person approves a head once, and ship merges it on that approval; rule: limit; source: criteria 1
2. says: An approval never covers another head, another login, or the narrow set; rule: never; source: criteria 2, 3, 4

### Provenance

Request: "I can only check in every 4 hrs so we need to ensure we can move the journey ahead"
1. stated (request): one approval of a head is enough to ship it
2. default (scope): an approval covers only its head
3. default (scope): only [autonomy] approver approves
4. default (scope): the narrow set keeps its per-PR person
5. default (scope): no approval, no merge
````
