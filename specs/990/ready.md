# rahulnakmol/agentic-ayudha#990: An agent drafts a Feature baseline, and only a person approves it at the hash they saw

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `b09cea0ccc384408c7017e1b88c07898115d716e6e03dad56b003b8fcb25b93b` |
| Completion Goal hash | `ba8fe3e1c170a4dd8eeb2aab193b8f72f399f1fd8fa223735142096193bb05df` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/990/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ready 874 --part intent --request` from an Amp thread (AMP_THREAD_ID set) on a Feature with no intent baseline; expected: one ledger commit writes `specs/874/baselines/intent-v1.draft.json` with part `intent`, version 1, owner `product-strategist`, one pin `#874 body` with the `workitems.BodyHash` of the body, the hash, the summary, and the source session; no `intent-v1.json` exists
2. [api-trace] state: the same request; expected: one marker comment `<!-- aayudha:baseline part=intent -->` on #874 says "Intent v1 of #874 waits for a person: aayu ready 874 --part intent"
3. [api-trace] state: `aayu ready 874 --part intent` with AAYU_AGENT=1 set; expected: exit 4 with "ready --part is a person's decision and is refused for an agent lane (AAYU_AGENT=1)"; no ledger commit and no comment
4. [api-trace] state: a person runs `aayu ready 874 --part intent` on a terminal, the effect shows before the device code, and the body is unchanged since the draft; expected: after the device-flow check, one ledger commit writes `specs/874/baselines/intent-v1.json` with the draft's hash, `approved_by` the person's login, `approved_at`, and `presence` `device-flow`; the marker comment says "Intent v1 of #874 baselined by @<login> · hash <12 hex>"
5. [api-trace] state: the body of #874 is edited after the device code shows and before the person enters it; expected: "The intent of #874 changed after the draft (hash <old 12> → <new 12>). Nothing was baselined. Draft it again."; no `intent-v1.json` is written
6. [api-trace] state: `aayu ready 874 --part spec` when no spec draft is on the ledger; expected: exit 4 with "No spec draft of #874 waits for a person. An agent drafts it: aayu ready 874 --part spec --request"; nothing is written

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want an agent to draft each baseline of a Feature and only me to approve it, at the hash I saw, so that what I agreed is frozen and cannot be approved for me.

### Completion Goal

1. [api-trace] state: `aayu ready 874 --part intent --request` from an Amp thread (AMP_THREAD_ID set) on a Feature with no intent baseline; expected: one ledger commit writes `specs/874/baselines/intent-v1.draft.json` with part `intent`, version 1, owner `product-strategist`, one pin `#874 body` with the `workitems.BodyHash` of the body, the hash, the summary, and the source session; no `intent-v1.json` exists
2. [api-trace] state: the same request; expected: one marker comment `<!-- aayudha:baseline part=intent -->` on #874 says "Intent v1 of #874 waits for a person: aayu ready 874 --part intent"
3. [api-trace] state: `aayu ready 874 --part intent` with AAYU_AGENT=1 set; expected: exit 4 with "ready --part is a person's decision and is refused for an agent lane (AAYU_AGENT=1)"; no ledger commit and no comment
4. [api-trace] state: a person runs `aayu ready 874 --part intent` on a terminal, the effect shows before the device code, and the body is unchanged since the draft; expected: after the device-flow check, one ledger commit writes `specs/874/baselines/intent-v1.json` with the draft's hash, `approved_by` the person's login, `approved_at`, and `presence` `device-flow`; the marker comment says "Intent v1 of #874 baselined by @<login> · hash <12 hex>"
5. [api-trace] state: the body of #874 is edited after the device code shows and before the person enters it; expected: "The intent of #874 changed after the draft (hash <old 12> → <new 12>). Nothing was baselined. Draft it again."; no `intent-v1.json` is written
6. [api-trace] state: `aayu ready 874 --part spec` when no spec draft is on the ledger; expected: exit 4 with "No spec draft of #874 waits for a person. An agent drafts it: aayu ready 874 --part spec --request"; nothing is written

### Measure

Baseline approvals written to the ledger without a person's device-flow check, and approvals written at a hash other than the one shown.

### Threshold

0 of each, over every seeded run of the six criteria.

### Scenario

An agent drafts the intent of a seeded Feature from a thread. A person approves it on a terminal with the device flow. A second run edits the body while the device code waits.

### Environment

The fake GitHub of internal/github/fake, with its device flow, and a local ledger branch; synthetic issues only, never a production repository.

### Checkpoints

1. `aayu ready <f> --part intent --request` writes the draft and the marker comment (criteria 1, 2)
2. `aayu ready <f> --part intent` refuses an agent lane and approves for a person (criteria 3, 4)
3. The approval re-reads the pins after the device code and refuses a moved draft (criteria 5, 6)

### Assumptions

- The source session comes from #871 (Env.source()). Until #871 ships, it comes from the session IDs that `asked.go` reads.
- The presence check is `presenceCheck` in internal/cli/presence.go, as `scope --accept` uses it (internal/cli/scope.go:1278-1288).

#### Review guidance (Q-H: here until slice --update, #887 story 2, moves it)

check: 3 the refusal for an agent lane comes before any ledger read or write
check: 5 the approval reads the pins again after the device code and compares the hash with the draft's
risk: internal/ready/baseline.go a person's record written without the presence check (threat T19, T23)
risk: internal/cli/ready.go flag wiring that lets --request reach the approval path
ignore: category docs

#### Behaviours (Q-H: here until slice --update, #887 story 2, moves it)

1. says: "A baseline is approved only by a person, at the hash they saw."; rule: authority; criteria 4, 5; source: Q16 (inbox design v27); core design S5
2. says: "An agent drafts a baseline but never approves one."; rule: never; criteria 3; source: Q16; threat T19

### Open questions

none
````
