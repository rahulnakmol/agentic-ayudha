# rahulnakmol/agentic-ayudha#614: ship merges an agent's PR into dev on proof, outside the narrow set

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `619a5a7ffdebed5ca4841a5bd62b0f3ca1ea82f24338806c7072ecedbc5d4048` |
| Completion Goal hash | `46223f719833f87728c57f9cf1c1a1680726536404c9129fa72213cecd65d14b` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/614/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `[autonomy] proof` on; `aayu ship <pr>` on a Story PR into dev with no approving review, a never-layer footprint and no plan review; every required check is green, the pack verifies, and inspect is at merge with a `proof_rungs` rung; expected: ship merges it, and the ship record's human pass is `proof` with the rung, the required checks, and any CODEOWNERS paths it merged
2. [api-trace] state: the same PR with one required check of the base branch's rules red; expected: exit 4 "required check <name> is failure …; never merge with a red required check", and nothing merged
3. [api-trace] state: the same PR touching a path in `[autonomy] always_person`, a workflow that reads a secret, or a change to the `[autonomy]` table of an `always_person_config` file; expected: no merge on proof, and ship names the path or table and stops for a person
4. [api-trace] state: the review rung at the head is not in `[autonomy] proof_rungs`; expected: no merge on proof, and ship says which rung it found
5. [api-trace] state: `.aayudha/config.toml` of this repository; expected: `proof = true`, `proof_rungs` with C until rung B is live, and the narrow set of #593 option 1 in `always_person`, rendered through the protected config

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want `aayu ship` to merge an agent's PR into dev on proof alone, with no per-PR pass from me, outside a narrow set of paths, so that merging into dev is frictionless while the proof still lands.

Decision #593 option 1, accepted 2026-10-18. The founder: "agent PRs to dev must merge automatically, squash only, when CI is green and the proof shows the PR does what it says." This supersedes #572 option 1's per-PR passes for dev. `rolling` and `main` keep their gates. Replaces #575, the intent pass, which Decision #593 superseded for dev.

### Completion Goal

1. [api-trace] state: `[autonomy] proof` on; `aayu ship <pr>` on a Story PR into dev with no approving review, a never-layer footprint and no plan review; every required check is green, the pack verifies, and inspect is at merge with a `proof_rungs` rung; expected: ship merges it, and the ship record's human pass is `proof` with the rung, the required checks, and any CODEOWNERS paths it merged
2. [api-trace] state: the same PR with one required check of the base branch's rules red; expected: exit 4 "required check <name> is failure …; never merge with a red required check", and nothing merged
3. [api-trace] state: the same PR touching a path in `[autonomy] always_person`, a workflow that reads a secret, or a change to the `[autonomy]` table of an `always_person_config` file; expected: no merge on proof, and ship names the path or table and stops for a person
4. [api-trace] state: the review rung at the head is not in `[autonomy] proof_rungs`; expected: no merge on proof, and ship says which rung it found
5. [api-trace] state: `.aayudha/config.toml` of this repository; expected: `proof = true`, `proof_rungs` with C until rung B is live, and the narrow set of #593 option 1 in `always_person`, rendered through the protected config

### Measure

criteria observed

### Threshold

5 of 5

### Scenario

Run the cli tests with a fake GitHub and a local ledger: a Story made Ready, built, and its merge gate green.

### Environment

go test ./internal/cli with internal/github/fake

### Checkpoints

1. ship checks the base branch's required checks and computes the proof pass and the narrow set (criteria 1, 2, 3, 4)
2. this repository turns proof on with the narrow set (criterion 5)

### Assumptions

- the CODEOWNERS check (`protected-paths`) is not one of the required checks ship reads, since it fails by design for paths outside the narrow set; ship records those paths instead
- `proof` is off in the built-in defaults, so a repository opts in
- the sampled audit after merge is Story #589

### Provenance

Request: "Build its Story now: aayu ship merges on proof without a human pass, outside the narrow set, as specified."
Term: "narrow set" → `[autonomy] always_person` paths, the approval tables of `always_person_config` files, and workflows that read a secret or name an environment; "who can approve or bypass ... and release or secret-holding workflows"
Vague: "as specified" → criteria 1 to 5, from Decision #593 option 1
1. stated (founder, 2026-10-18): "agent PRs to dev must merge automatically, squash only, when CI is green and the proof shows the PR does what it says"
2. stated (founder, 2026-10-18): "Never merge with a red required check or an open fix finding."
3. stated (Decision #593 option 1): "A PR touching any of these still needs a person per PR, batched in `aayu inbox`."
4. stated (coordinator, 2026-10-18): "until it lands, accept the head's review as it is and record the rung"
5. stated (Decision #593): "the narrow set as a list in protected configuration"
````
