# rahulnakmol/agentic-ayudha#993: A drift after a baseline shows at once, and a new version says what changed, why it matters, and the need behind it

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `d91fba615be23f31d0ee48dd619d8c018a459ad4e2720a0dc6645eb9b3d4ca09` |
| Completion Goal hash | `96d5ae311f5ae7475c7c60d78639160e2b591080747dab5837a9ae762a9a6625` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/993/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ready 874 --check` on a Feature with intent v1 and spec v2 approved, after one child story's Completion Goal changed and one story was added; expected: a Baselines block with "intent v1 · no drift" and "spec v2 · drift: #912 Completion Goal changed · #915 added"; no write
2. [api-trace] state: `aayu ready 874 --part spec --request --changes changes.json` for v3, where the file gives what, why, and need for #912 and nothing for #915; expected: exit 4 with "spec v3 of #874 needs what changed, why it matters, and the need behind it for each change. Missing: #915 (added): what, why, need."; nothing is written
3. [api-trace] state: the same request where the need for #912 has 2 words and `[baselines] why_min_words` is 5; expected: exit 4 naming "#912 need: 2 words, at least 5 ([baselines] why_min_words)"
4. [api-trace] state: the request with each change complete; expected: `specs/874/baselines/spec-v3.draft.json` holds `changes` with `ref`, `what`, `why`, and `need` for #912 and #915, and its hash covers them; the marker comment shows a table "What changed since v2" with the columns Change, Why it matters, and Need
5. [api-trace] state: `aayu ready 874 --part intent --request` when the pins equal intent v1; expected: "Nothing changed since intent v1 of #874. No draft written."
6. [api-trace] state: the first request of a part (no approved version yet) with no changes file; expected: the v1 draft is written and has no `changes` field
7. [api-trace] state: a draft of spec v4 with `[baselines] version_warn` 4; expected: the warning "The spec of #874 reaches v4 ([baselines] version_warn). Check why each review finds new gaps."
8. [api-trace] state: `aayu ready 874 --part spec --request --changes changes.json --fixes spec-v1/quality-architect/2` where `specs/874/reviews/spec-v1/quality-architect.json` holds finding 2, then `--fixes tech-v1/devops-devex-tinkerer/1` and `--fixes spec-v1/quality-architect/9` (no such finding); expected: the first draft has `fixes` `["spec-v1/quality-architect/2"]` and its hash covers it; each of the other two exits 2, names the ID, and writes no draft
9. [api-trace] state: `aayu config show` with no `[baselines]` in `.aayudha/config.toml`; expected: `why_min_words = 5` and `version_warn = 4` from the base layer, and `baselines` is in `[autonomy] always_person_sections`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want every drift after a baseline to show, and every new version to say what changed, why it matters, and the need behind it, so that I approve a change knowing its reason.

### Completion Goal

1. [api-trace] state: `aayu ready 874 --check` on a Feature with intent v1 and spec v2 approved, after one child story's Completion Goal changed and one story was added; expected: a Baselines block with "intent v1 · no drift" and "spec v2 · drift: #912 Completion Goal changed · #915 added"; no write
2. [api-trace] state: `aayu ready 874 --part spec --request --changes changes.json` for v3, where the file gives what, why, and need for #912 and nothing for #915; expected: exit 4 with "spec v3 of #874 needs what changed, why it matters, and the need behind it for each change. Missing: #915 (added): what, why, need."; nothing is written
3. [api-trace] state: the same request where the need for #912 has 2 words and `[baselines] why_min_words` is 5; expected: exit 4 naming "#912 need: 2 words, at least 5 ([baselines] why_min_words)"
4. [api-trace] state: the request with each change complete; expected: `specs/874/baselines/spec-v3.draft.json` holds `changes` with `ref`, `what`, `why`, and `need` for #912 and #915, and its hash covers them; the marker comment shows a table "What changed since v2" with the columns Change, Why it matters, and Need
5. [api-trace] state: `aayu ready 874 --part intent --request` when the pins equal intent v1; expected: "Nothing changed since intent v1 of #874. No draft written."
6. [api-trace] state: the first request of a part (no approved version yet) with no changes file; expected: the v1 draft is written and has no `changes` field
7. [api-trace] state: a draft of spec v4 with `[baselines] version_warn` 4; expected: the warning "The spec of #874 reaches v4 ([baselines] version_warn). Check why each review finds new gaps."
8. [api-trace] state: `aayu ready 874 --part spec --request --changes changes.json --fixes spec-v1/quality-architect/2` where `specs/874/reviews/spec-v1/quality-architect.json` holds finding 2, then `--fixes tech-v1/devops-devex-tinkerer/1` and `--fixes spec-v1/quality-architect/9` (no such finding); expected: the first draft has `fixes` `["spec-v1/quality-architect/2"]` and its hash covers it; each of the other two exits 2, names the ID, and writes no draft
9. [api-trace] state: `aayu config show` with no `[baselines]` in `.aayudha/config.toml`; expected: `why_min_words = 5` and `version_warn = 4` from the base layer, and `baselines` is in `[autonomy] always_person_sections`

### Measure

New baseline versions written with a change that has no what, why, or need.

### Threshold

0, over the seeded runs of criteria 2 to 4.

### Scenario

After v1 and v2 of a seeded Feature's parts, a story changes and a story is added. An agent drafts v3 first without reasons, then with short reasons, then with full ones.

### Environment

The fake GitHub of internal/github/fake, with its device flow, and a local ledger branch; synthetic issues only, never a production repository.

### Links

depends-on #992

### Checkpoints

1. `aayu ready <feature> --check` shows each part's drift from the pins at read time (criterion 1)
2. A new version needs what, why, and need for each change (criteria 2, 3, 4)
3. No-op and first versions behave as stated (criteria 5, 6)
4. The `[baselines]` keys and their defaults (criteria 7, 9)
5. `--fixes` names reviewed findings of the same part, inside the hash (criterion 8)

### Assumptions

- Agent text in what, why, and need is untrusted (threat T6). The marker comment escapes it, and a model reads it wrapped.

#### Review guidance (Q-H: here until slice --update, #887 story 2, moves it)

check: 1 drift is computed from the pins when read; nothing writes a drift record
check: 8 `fixes` is inside the hashed content; #874 never closes a finding, #876 does
check: 4 the draft hash covers the changes, so a reason edited after the person saw it baselines nothing
risk: internal/ready/drift.go a missed changed pin lets a change through with no reason
ignore: category docs

#### Behaviours (Q-H: here until slice --update, #887 story 2, moves it)

1. says: "A new baseline version is never written without what changed, why it matters, and the need behind it, for each change."; rule: never; criteria 2, 3, 4; source: @rahulnakmol on #874, 2026-10-21 (0.4.0 review); 874-Q1 decided: one entry per changed pin
2. says: "Drift shows from the pins at the time you read; nothing polls for it."; rule: capability; criteria 1; source: @rahulnakmol: event-driven, not polling (0.4.0 review)
3. says: "Each of what, why, and need has at least 5 words by default."; rule: limit; criteria 3; source: @rahulnakmol: every limit is a config key with a default
4. says: "A new version names only findings of its own part as fixed, and its hash covers them."; rule: never; criteria 8; source: #876 contract (resolutions: fixed only in a later approved baseline)

### Open questions

none
````
