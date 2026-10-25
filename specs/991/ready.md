# rahulnakmol/agentic-ayudha#991: aayu plan writes a Feature's technical design, and each story plan stays inside it

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `04ca530a4cf697e16bb32ed0515706050169e1e4a3b3547bd213d0e3d97b9a73` |
| Completion Goal hash | `724722654670495defcfdfaad82c9edfa24ae003067f5a26c1737cb6d8961ec8` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/991/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu plan 874` on a Feature with Expected paths and 5 child stories; expected: it reports the footprint, the 5 stories, and "next: draft the technical design (skills/plan), then aayu plan 874 --draft <file>"; nothing is written, and the refusal at internal/cli/plan.go:141 no longer applies to a Feature
2. [api-trace] state: `aayu plan 874 --draft design.json` where the draft's layers include L05 and the footprint is L02, L03, L07; expected: "layer L05 is not in the footprint of #874 (L02, L03, L07)"; nothing is written
3. [api-trace] state: a draft whose story order leaves out child story #915; expected: "story order: #915 is a story of #874 but is not in the order"
4. [api-trace] state: a draft with layer L07 and an empty rollback; expected: "a design that touches L07 names its rollback"
5. [api-trace] state: `aayu plan 874 --draft design.json --create` on a draft that passes; expected: one ledger commit writes `specs/874/design.json` with contracts, layers, story order, rollback, the Feature body hash, `planned_at`, and `planned_by`; the output ends "next: aayu ready 874 --part tech --request"
6. [api-trace] state: `aayu plan 912 --draft plan.json` for a story under #874 whose checkpoint 2 is layer L06, not in the layers of `specs/874/design.json`; expected: "checkpoint 2 (L06) is outside the technical design of #874 (L02, L03, L07)"
7. [api-trace] state: the same story plan for a story whose Feature has no `design.json`; expected: the plan passes as it does today, with no new problem

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As Vishwa, the Code Architect twin, I want `aayu plan` to take a Feature and write its technical design, so that @rahulnakmol approves one design and every story plan stays inside it.

### Completion Goal

1. [api-trace] state: `aayu plan 874` on a Feature with Expected paths and 5 child stories; expected: it reports the footprint, the 5 stories, and "next: draft the technical design (skills/plan), then aayu plan 874 --draft <file>"; nothing is written, and the refusal at internal/cli/plan.go:141 no longer applies to a Feature
2. [api-trace] state: `aayu plan 874 --draft design.json` where the draft's layers include L05 and the footprint is L02, L03, L07; expected: "layer L05 is not in the footprint of #874 (L02, L03, L07)"; nothing is written
3. [api-trace] state: a draft whose story order leaves out child story #915; expected: "story order: #915 is a story of #874 but is not in the order"
4. [api-trace] state: a draft with layer L07 and an empty rollback; expected: "a design that touches L07 names its rollback"
5. [api-trace] state: `aayu plan 874 --draft design.json --create` on a draft that passes; expected: one ledger commit writes `specs/874/design.json` with contracts, layers, story order, rollback, the Feature body hash, `planned_at`, and `planned_by`; the output ends "next: aayu ready 874 --part tech --request"
6. [api-trace] state: `aayu plan 912 --draft plan.json` for a story under #874 whose checkpoint 2 is layer L06, not in the layers of `specs/874/design.json`; expected: "checkpoint 2 (L06) is outside the technical design of #874 (L02, L03, L07)"
7. [api-trace] state: the same story plan for a story whose Feature has no `design.json`; expected: the plan passes as it does today, with no new problem

### Measure

Story plans that pass with a checkpoint layer outside their Feature's technical design.

### Threshold

0, over the seeded runs of criteria 6 and 7.

### Scenario

Vishwa plans a seeded Feature with five stories, writes its design, then plans one story inside it and one outside it.

### Environment

The fake GitHub of internal/github/fake, with its device flow, and a local ledger branch; synthetic issues only, never a production repository.

### Checkpoints

1. `aayu plan <feature>` reports the footprint and the stories (criterion 1)
2. `aayu plan <feature> --draft` checks layers, story order, and rollback (criteria 2, 3, 4)
3. `--create` writes `specs/<f>/design.json` (criterion 5)
4. A story plan stays inside its Feature's design (criteria 6, 7)

### Assumptions

- The design draft is a Go type in internal/planning with strict decoding. No new file in evals/schemas, so the footprint does not gain L05.
- Contracts reuse the shape of `planning.Checkpoint.Contracts` (internal/planning/plan.go).

#### Review guidance (Q-H: here until slice --update, #887 story 2, moves it)

check: 6 the story-plan check reads the Feature's design.json from the ledger, not from the draft
check: 7 a story with no Feature design keeps today's behaviour
risk: internal/cli/plan.go the story path and the Feature path share code; a Feature must never get a story plan record
ignore: category docs

#### Behaviours (Q-H: here until slice --update, #887 story 2, moves it)

1. says: "A story plan never uses a layer outside its Feature's technical design."; rule: never; criteria 6; source: core design S5 (technical design)
2. says: "A Feature's technical design names every story of the Feature in its order."; rule: capability; criteria 3; source: core design S5 (contracts, layers, story order, rollback)

### Open questions

none
````
