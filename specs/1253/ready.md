# rahulnakmol/agentic-ayudha#1253: aayu inbox asks up to five decisions to start one Feature: a card per baseline part, then Approve to build, then Mission start

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `7b3fc4fb75d81f8d8c339e1864d6d5133489180c808d68a15cf6e22568b8fdf7` |
| Completion Goal hash | `6fd20f88331f3c16e416849cb9f584d9ad992171295775f6cfd28afcc63b7756` |
| Technical Owner | ux-researcher-evolutionist |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1253/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a Feature with intent, spec, and tech drafts waiting and no Approve to build; expected: `aayu inbox --list --json` shows one card for the Feature, with each part's version and hash and the Approve to build it leads to
2. [api-trace] state: a person chooses Approve and start on that card; expected: the ledger has an approval for each part pinned to the hash the card showed, then the Feature's Approve to build, then its mission start, after one device-flow approval
3. [api-trace] state: a part's pins move after the card is shown, and the person chooses Approve and start; expected: nothing is written for that Feature, and the card says which part changed
4. [api-trace] state: a person chooses Open a part; expected: the part's detail shows what today's baseline card shows, and approving or rejecting there changes only that part
5. [api-trace] state: an agent lane runs `aayu inbox --list`; expected: it sees the Feature card and no approve action

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu inbox` asks a person for up to five decisions to start one Feature. Each baseline part gets its own card (intent, spec, tech), then Approve to build, then Mission start. `baselineCards` appends one card per part (internal/cli/inboxbaseline.go:16 and :86). `approveCard` returns no card while a part is unapproved (internal/cli/inboxapprove.go:60), and Mission start is a separate card (`InboxStart`, internal/cli/inbox.go:51). On 2026-11-02, the inbox held 18 baseline cards for six Features (#875, #876, #877, #887, #889, #982) and #874's Approve to build card. @rahulnakmol said: "each holing one card for spec and tech approval which is unneccesary". One device code already covers the session (presenceCheck), so the cost is decisions, not codes. In the sitting model of Decision #1168, he decides a whole Feature at once.

### Steps to reproduce

1. A Feature has intent, spec, and tech drafts that wait for a person, and no Approve to build.
2. Run `aayu inbox`.

### Expected behaviour

The Feature has one card. It shows each waiting part, with its version, hash, and changes since the version before. It also shows the Approve to build that follows: the stories, the mission plan, the budget, and the agents. Its choices are: Approve and start, Approve only, Open a part (the per-part detail of today's card), and Not now. Approve and start writes each part's approval pinned to the hash on the card, then Approve to build, then the mission start, with one device code. The ledger keeps one approval record per part, as today.

### Actual behaviour

The Feature has one card per part. Its Approve to build card shows only after the last part is approved, and its Mission start card shows after that. That is up to five decisions per Feature.

### Completion Goal

1. [api-trace] state: a Feature with intent, spec, and tech drafts waiting and no Approve to build; expected: `aayu inbox --list --json` shows one card for the Feature, with each part's version and hash and the Approve to build it leads to
2. [api-trace] state: a person chooses Approve and start on that card; expected: the ledger has an approval for each part pinned to the hash the card showed, then the Feature's Approve to build, then its mission start, after one device-flow approval
3. [api-trace] state: a part's pins move after the card is shown, and the person chooses Approve and start; expected: nothing is written for that Feature, and the card says which part changed
4. [api-trace] state: a person chooses Open a part; expected: the part's detail shows what today's baseline card shows, and approving or rejecting there changes only that part
5. [api-trace] state: an agent lane runs `aayu inbox --list`; expected: it sees the Feature card and no approve action

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Draft three baselines for a fake Feature, list the inbox, approve and start from the one card, then repeat with a pin moved after the card is shown.

### Environment

go test of internal/cli with a fake GitHub and a fake ledger; never production

### Checkpoints

1. One card per Feature that shows every waiting part and the Approve to build (criteria 1, 5)
2. Approve and start writes every part, Approve to build, and the start with one code, or nothing (criteria 2, 3)
3. Per-part detail still works (criteria 4)

### Technical Owner

ux-researcher-evolutionist

### Assumptions

#### Notes

The per-part ledger records, hashes, and pins stay as Story #1022 and Story #1172 made them. Only the card and the choice change. A UX part is shown only when the Feature's footprint has L01, as today.

#### Review guidance

- Check: criterion 3: a moved pin writes nothing for the whole Feature, and never writes some parts and not others.
- Ignore: the card's wording and layout.
- Risk: a person approves a part they did not see; the card must show every part's hash and changes before the choice.

#### Behaviours

1. says: A person decides a Feature once; rule: one card per Feature; source: criteria 1, 2
2. says: A part is approved only at the hash the person saw; rule: never; source: criteria 3

### Provenance

Request: "875, #876, #877, #887, #889 and #982, plus #874's each holing one card for spec and tech approval which is unneccesary"
1. default (scope): one card per Feature
2. default (scope): approve and start with one code
3. default (scope): a moved pin writes nothing
4. default (scope): per-part detail kept
5. default (scope): agent lanes see no approve action
````
