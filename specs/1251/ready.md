# rahulnakmol/agentic-ayudha#1251: fix(ready): a story added after Approve to build names aayu ready --ack, a flag that does not exist

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `25e0f0f8af5771c5c6e8b34b1139e752287d0b762579bc6fc7ffee3e0ff0c189` |
| Completion Goal hash | `9c47d7e71d102f99cf37cdb46b9ded70348443eb5a68bb72e91a1c3f0255b5e0` |
| Technical Owner | product-strategist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1251/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a story is added under a Feature after its Approve to build; expected: `aayu ready <story>` says it waits for spec v<k+1> and Approve to build again, and names `aayu ready <f> --part spec --request --changes <file>` and `aayu inbox`, and no `--ack`
2. [api-trace] state: a covered story's body changed after Approve to build; expected: the same steps, with the changes listed, and no `--ack`
3. [api-trace] state: every command that any refusal of `aayu ready` names; expected: a test runs each against the command tree and finds every flag

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu ready <story>` tells a person to run `aayu ready <f> --ack <story>` for a story added after Approve to build, but `--ack` does not exist, so the command fails with "unknown flag: --ack". The message is in `storyCovered` (internal/cli/readycover.go:46 and :49), and the comment above it says "--ack is not built yet" (internal/cli/readycover.go:43). On 2026-11-02, @rahulnakmol ran `aayu ready 1170 --ack 1250` for Story #1250 and got "aayu: unknown flag: --ack". The step that works is a new spec baseline, then Approve to build again, which the message gives only as the `next:` line.

### Steps to reproduce

1. Feature #1170 has an Approve to build.
2. Story #1250 is filed under #1170 after it.
3. Run `aayu ready 1250`, then the command it names.

### Expected behaviour

The message names only a command that exists: the spec draft, then the inbox, where the person approves the new spec and Approve to build with one code.

### Actual behaviour

The message names `aayu ready 1170 --ack 1250`, which fails.

### Completion Goal

1. [api-trace] state: a story is added under a Feature after its Approve to build; expected: `aayu ready <story>` says it waits for spec v<k+1> and Approve to build again, and names `aayu ready <f> --part spec --request --changes <file>` and `aayu inbox`, and no `--ack`
2. [api-trace] state: a covered story's body changed after Approve to build; expected: the same steps, with the changes listed, and no `--ack`
3. [api-trace] state: every command that any refusal of `aayu ready` names; expected: a test runs each against the command tree and finds every flag

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Add a story after Approve to build, change a covered story, and check every named command against the command tree.

### Environment

go test of internal/cli with a fake GitHub and a fake ledger; never production

### Checkpoints

1. Ready names the steps that work (criteria 1, 2)
2. A test keeps named commands real (criteria 3)

### Technical Owner

product-strategist

### Assumptions

#### Notes

Building `--ack` itself is a separate choice: it would let a person cover a new story without a new spec version. This Bug only makes the message true.

#### Review guidance

- Check: criterion 3: the test parses each named command with the real flag set.
- Ignore: the wording.
- Risk: a message that drifts again; criterion 3 catches it.

#### Behaviours

1. says: Every command aayu names exists; rule: never; source: criteria 3

### Provenance

Request: "learn from what we have built so far so we can really understand the friction here and what are are causing al lot of halts and user intervention"
1. default (scope): name the steps that work
2. default (scope): the same for a changed story
3. default (scope): a test keeps named commands real
````
