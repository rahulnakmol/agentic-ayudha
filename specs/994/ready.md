# rahulnakmol/agentic-ayudha#994: A person approves one early build before Approve to build, and sees its budget

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `85039086c1d4e421f8c124a9a55ce994a477f198d00bfd54b0c0d47ffd8e13f9` |
| Completion Goal hash | `d8aab46337227b586969d8d19b300fefa21a6ce5def5effbbdaa56ceef4f8c17` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/994/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a person runs `aayu ready 874 --early-build 916`, where #916 is a child story of #874 with the label `early-build`, `[autonomy.budget] cost_usd` is 60, and `[baselines] early_build_budget` is 1; expected: the effect "Approve the early build #916 of #874. Budget up to $60.00 (1 × [autonomy.budget] cost_usd)." shows before the device code; after it, one ledger commit writes `specs/874/approved-early-916.json` with the story, its body hash, `budget_usd` 60, `approved_by`, `presence`, and `at`
2. [api-trace] state: `--early-build 917`, where #917 has no `early-build` label, then `--early-build 920`, where #920 is not a child of #874; expected: each exits 4 before the device code and names the condition that fails; nothing is written
3. [api-trace] state: `[baselines] early_build_budget` 2 and `[autonomy.budget] cost_usd` 60; expected: the effect says "Budget up to $120.00 (2 × [autonomy.budget] cost_usd)" and the record has `budget_usd` 120
4. [api-trace] state: a PR with `Refs #916`, where #916 has the label `early-build`, every required check green and inspect otherwise at merge; expected: `aayu inspect` gives the disposition hold with "#916 is an early build: its PR never merges into dev (E1)", and `aayu ship` exits 4 with the same reason and merges nothing
5. [api-trace] state: `aayu ready 874 --early-build 916` with AAYU_AGENT=1 set; expected: exit 4 with "ready --early-build is a person's decision and is refused for an agent lane (AAYU_AGENT=1)"; nothing is written

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want to approve one flagged early-build story before Approve to build, with its budget shown, so that Hanu can show a design running while nothing else spends build tokens.

### Completion Goal

1. [api-trace] state: a person runs `aayu ready 874 --early-build 916`, where #916 is a child story of #874 with the label `early-build`, `[autonomy.budget] cost_usd` is 60, and `[baselines] early_build_budget` is 1; expected: the effect "Approve the early build #916 of #874. Budget up to $60.00 (1 × [autonomy.budget] cost_usd)." shows before the device code; after it, one ledger commit writes `specs/874/approved-early-916.json` with the story, its body hash, `budget_usd` 60, `approved_by`, `presence`, and `at`
2. [api-trace] state: `--early-build 917`, where #917 has no `early-build` label, then `--early-build 920`, where #920 is not a child of #874; expected: each exits 4 before the device code and names the condition that fails; nothing is written
3. [api-trace] state: `[baselines] early_build_budget` 2 and `[autonomy.budget] cost_usd` 60; expected: the effect says "Budget up to $120.00 (2 × [autonomy.budget] cost_usd)" and the record has `budget_usd` 120
4. [api-trace] state: a PR with `Refs #916`, where #916 has the label `early-build`, every required check green and inspect otherwise at merge; expected: `aayu inspect` gives the disposition hold with "#916 is an early build: its PR never merges into dev (E1)", and `aayu ship` exits 4 with the same reason and merges nothing
5. [api-trace] state: `aayu ready 874 --early-build 916` with AAYU_AGENT=1 set; expected: exit 4 with "ready --early-build is a person's decision and is refused for an agent lane (AAYU_AGENT=1)"; nothing is written

### Measure

Early-build approvals that cover more than one story, or that do not show their budget before the device code.

### Threshold

0, over the seeded runs of criteria 1 and 2.

### Scenario

Hanu flags one prototype story under a seeded Feature. A person approves it, then tries two stories that do not qualify.

### Environment

The fake GitHub of internal/github/fake, with its device flow, and a local ledger branch; synthetic issues only, never a production repository.

### Links

depends-on #990

### Checkpoints

1. `--early-build` checks the story and shows the budget (criteria 1, 2)
2. The budget follows `[baselines] early_build_budget` (criterion 3)
3. An early-build PR never merges: inspect holds it and ship refuses it (criterion 4)
4. An agent lane is refused (criterion 5)

### Assumptions

- The Ready rule that accepts this record for that story only is #877's (core design S8).
- The merge block reads the story label at the PR head, as inspect and ship read `Refs #n` today (internal/cli/inspect.go:568, internal/cli/ship.go:163).

#### Review guidance (Q-H: here until slice --update, #887 story 2, moves it)

check: 2 all refusals happen before the presence check and before any write
check: 1 the budget shown is the one written
risk: internal/ready/early.go an approval record that a story other than the named one can use
ignore: category docs

#### Behaviours (Q-H: here until slice --update, #887 story 2, moves it)

1. says: "An early-build approval covers one story and shows its budget before you approve."; rule: authority; criteria 1, 2; source: E1 (core design, decided A)
2. says: "An early-build PR never merges into dev."; rule: never; criteria 4; source: E1 and Q19; design thread assigned it to #874 S5

### Open questions

none
````
