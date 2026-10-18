# rahulnakmol/agentic-ayudha#680: ship: with merge on proof off, the streak path merges a narrow-set change without a person

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `b3b84bd8e8a6fb887d333a26d40529c49eef878afca9e8bc56c2036f25db5077` |
| Completion Goal hash | `6ee5e4f99833ac5b5c0135d4fd6bc2af1d287f3d3a96e29ead3a6950519914f8` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/680/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: proof = false, every footprint layer's streak qualifies, review rung A, required checks green, and the Story PR changes `.aayudha/config.local.json`; expected: aayu ship refuses with exit 4, names the narrow-set path in its refusal, records no autonomous pass, and the PR stays open for a person
2. [api-trace] state: proof = false, the same qualifying streak, and the Story PR changes the `[autonomy]` table of `.aayudha/config.toml`; expected: aayu ship refuses with exit 4, names the always_person_config section, and the PR stays open for a person
3. [api-trace] state: proof = false, the same qualifying streak, and the Story PR touches no narrow-set path; expected: aayu ship records the autonomous pass and merges, as before
4. [api-trace] state: proof = true, a Story PR that changes `.aayudha/config.local.json`, and a second Story PR that touches no narrow-set path; expected: the first is refused with the same narrow-set reason as on dev 70299db, and the second merges on proof as before

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

With `[autonomy] proof = false`, the default, `aayu ship` does not read the narrow set before the streak path lets a Story PR merge without a person. `proofPass` (`internal/cli/shipproof.go:176-177`) returns `needsPerson = false` as soon as proof is off, before it scans the changed files. The streak branch at `internal/cli/ship.go:335` then trusts `!needsPerson`, although its own comment says "the narrow set: a person, whatever the streak". So an agent's Story PR that changes an `always_person` path or an `always_person_sections` table of an `always_person_config` file gets an autonomous human pass when every layer of its footprint has a qualifying streak. Codex found this in its review of PR #638 at 0b5e18b, and it was confirmed on dev at 70299db.

### Steps to reproduce

1. Leave `[autonomy] proof` unset or `false` in `.aayudha/config.toml`.
2. Reach a qualifying correction-free streak on every layer of a Story's footprint (`aayu status --streak`).
3. Open an agent Story PR in that footprint that also changes `.aayudha/config.local.json` (an `always_person` path) or the `[autonomy]` table of `.aayudha/config.toml` (an `always_person_config` file), with a different-family review (rung A) and every required check green.
4. Run `aayu ship <pr>`.

### Expected behaviour

ship refuses: the PR touches the narrow set, so it waits for a person whatever the streak, as the comment at `internal/cli/ship.go:335` says and as it does with `proof = true`.

### Actual behaviour

ship records an autonomous human pass ("every layer of footprint … has a streak of at least N correction-free agent merges") and merges the PR with no person.

### Completion Goal

1. [api-trace] state: proof = false, every footprint layer's streak qualifies, review rung A, required checks green, and the Story PR changes `.aayudha/config.local.json`; expected: aayu ship refuses with exit 4, names the narrow-set path in its refusal, records no autonomous pass, and the PR stays open for a person
2. [api-trace] state: proof = false, the same qualifying streak, and the Story PR changes the `[autonomy]` table of `.aayudha/config.toml`; expected: aayu ship refuses with exit 4, names the always_person_config section, and the PR stays open for a person
3. [api-trace] state: proof = false, the same qualifying streak, and the Story PR touches no narrow-set path; expected: aayu ship records the autonomous pass and merges, as before
4. [api-trace] state: proof = true, a Story PR that changes `.aayudha/config.local.json`, and a second Story PR that touches no narrow-set path; expected: the first is refused with the same narrow-set reason as on dev 70299db, and the second merges on proof as before

### Measure

Agent Story PRs that touch a narrow-set path and merge with an autonomous pass while `[autonomy] proof = false`.

### Threshold

0.

### Scenario

`aayu ship <pr>` on an agent Story PR with a qualifying footprint streak, with merge on proof off and then on.

### Environment

`internal/cli` tests against the fake GitHub client (`internal/github/fake`), with synthetic PRs and ledger data only.

### Technical Owner

engineering-manager

### Links

### Checkpoints

1. ship scans the changed files against the narrow set whether or not merge on proof is on, and the streak branch refuses when the scan finds a narrow-set path (criteria 1, 2, 3)
2. The merge-on-proof path keeps its refusal and its merge as on dev 70299db (criterion 4)

### Assumptions

The narrow set is the one `proofPass` already judges when proof is on: the `always_person` paths, the `always_person_sections` tables of an `always_person_config` file, credential workflows, action manifests, and changed symbolic links. The fix reuses that scan rather than a second list.

### Open questions

deferred: whether the fix lands in v0.3.0 (the founder decides; until it merges, the v0.3.0 notes in PR #638 say the narrow set holds only with merge on proof on)

### Parked ideas

None.

### Provenance

Request: "File it as a Bug through aayu, P1. Its Completion Goal: with [autonomy] proof = false, a Story PR whose footprint streak qualifies but which touches a narrow-set path (always_person or always_person_config) is refused by ship and waits for a person; with proof = true, the behaviour is unchanged."
1. stated (orchestrator thread): "a Story PR whose footprint streak qualifies but which touches a narrow-set path (always_person ...) is refused by ship and waits for a person"
2. stated (orchestrator thread): "a Story PR whose footprint streak qualifies but which touches a narrow-set path (... always_person_config) is refused by ship and waits for a person"
3. default: a Story PR with a qualifying streak and no narrow-set path still merges on the streak, as the comment at internal/cli/ship.go:335 intends; the fix must not widen the refusal
4. stated (orchestrator thread): "with proof = true, the behaviour is unchanged"
Term: "P1" → the `priority:P1` label; "File it as a Bug through aayu, P1."
````
