# rahulnakmol/agentic-ayudha#55: WP5.5 `build`, `ship`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `dfb259eaae9cffc5354fca85a96cb909567f701bfae97ff7be79c8060d542065` |
| Completion Goal hash | `f38bd46a3b350550e80fcb9763f8824efd52b23c9364fb2bc2c5d13f3c1b2072` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/55/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: dogfood story Ready; expected: merged
2. [api-trace] state: the WP5.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a bot review is rejected as a pass
3. [api-trace] state: the WP5.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a stale lease is reclaimed by `gc`

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.5 `build`, `ship` built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.5): `build <story|bug>`: refuses unless Ready and body hash = snapshot; per-story **lease** (assignee + lock comment with TTL); worktree under `.aayudha/worktrees/<id>` from `origin/dev`; checkpoint commits with `Aayudha-Checkpoint: n/N`; replace-in-place progress comment and checkpoint record on `ledger`; `verify` per checkpoint; `[autonomy] budget {wall, cost, attempts}` → `status:blocked` + page on breach or second failure on a never-layer; `--resume` from the last trailer; retries 429/5xx with jitter. `ship <pr>`: green gate; human pass = approving review from the seat's `human` (solo → `[autonomy] approver`), `type != Bot`; never-layers require CODEOWNER approval regardless of streak; approval ID and streak state written to the pack; squash-merge; Complete section; status `merged`; branch/worktree cleanup. `aayu gc` prunes stale worktrees/leases. The agent writes the code; the binary owns worktree, commits, verify, PR, merge.

### Completion Goal

1. [api-trace] state: dogfood story Ready; expected: merged
2. [api-trace] state: the WP5.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a bot review is rejected as a pass
3. [api-trace] state: the WP5.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a stale lease is reclaimed by `gc`

### Measure

WP5.5 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.5)

### Threshold

all 3 criteria

### Scenario

Open the WP5.5 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
