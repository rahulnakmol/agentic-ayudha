# rahulnakmol/agentic-ayudha#502: Code-quality floors: no new lint finding on a changed line, and no changed function newly more complex than 15

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `1a449061cc0a500fce038101dcf76037b05492514d37b8ebfb063d18307d2401` |
| Completion Goal hash | `cf58a6258f999008a55eaf3ca643ea41e9a61e169fd2e09c3f70cb59660b4f6c` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/502/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a stand-in linter whose SARIF 2.1.0 report has one finding on a changed line, then only one finding on the unchanged line directly after the last changed line, then the same two as ESLint JSON; expected: the first fails the lint floor naming the file, the line, and the rule; the second passes with the finding outside the diff not counted; the ESLint JSON results match the SARIF results; the check records the linter's version
2. [api-trace] state: a checkout with `.golangci.yml`, one with `eslint.config.js`, one with `ruff.toml`, one with `[tool.ruff]` in `pyproject.toml`, one with `[verify.floors.lint] command` set, and one with none of these; expected: golangci-lint runs with `--output.sarif.path`, ESLint with `-f json`, Ruff with `--output-format sarif`, the configured command as written; with none the check reads `skipped: lint: no linter detected or configured` and blocks ship until a floor waiver names lint
3. [api-trace] state: base and head with lizard reports for four changed functions: one at CCN 15 that becomes 16, one at 20 that becomes 21, a new one at 16, and a new one at 15; and an unchanged function at 30; expected: the 15 → 16 and the new 16 fail the complexity floor by name; the 20 → 21 and the unchanged 30 are listed as already more than 15 and do not fail; the new 15 passes; a change in no language lizard reads skips without blocking
4. [api-trace] state: a lint or complexity miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
5. [api-trace] state: the security job of `ci.yml`, the merge gate and `aayudha-security.yml` as `aayu init --gates` renders them, run with stand-ins whose semgrep and lizard wheels do not match their pinned SHA-256; expected: each install step fails before the floors run; with matching stand-ins semgrep and lizard install from `--require-hashes` locks and the floors run
6. [api-trace] state: `aayu verify` on this repository's own pull request; expected: the lint check runs golangci-lint from `.golangci.yml` and the complexity check runs lizard 1.24.1; both checks appear in the proof pack with their versions

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a product team using aayu, I want every change's proof pack to say whether it kept the code clean, so that a pull request cannot add lint findings or a newly tangled function and still pass. Decision #480 (accepted, option 1, section 1) adds two floors that run on every footprint. `lint` runs the repository's own linter (golangci-lint, ESLint, or Ruff, detected or configured) and fails on any finding on a changed line. `complexity` runs lizard 1.24.1, pinned by SHA-256, and fails a changed function that is new with a cyclomatic complexity above 15 or that crosses from 15 or less to above 15. The same story installs semgrep from a hashed requirements lock, closing the one unhashed tool download. Misses are advisory until `[verify] new_floors_blocking_from` (2026-11-14), then they fail.

### Completion Goal

1. [api-trace] state: a stand-in linter whose SARIF 2.1.0 report has one finding on a changed line, then only one finding on the unchanged line directly after the last changed line, then the same two as ESLint JSON; expected: the first fails the lint floor naming the file, the line, and the rule; the second passes with the finding outside the diff not counted; the ESLint JSON results match the SARIF results; the check records the linter's version
2. [api-trace] state: a checkout with `.golangci.yml`, one with `eslint.config.js`, one with `ruff.toml`, one with `[tool.ruff]` in `pyproject.toml`, one with `[verify.floors.lint] command` set, and one with none of these; expected: golangci-lint runs with `--output.sarif.path`, ESLint with `-f json`, Ruff with `--output-format sarif`, the configured command as written; with none the check reads `skipped: lint: no linter detected or configured` and blocks ship until a floor waiver names lint
3. [api-trace] state: base and head with lizard reports for four changed functions: one at CCN 15 that becomes 16, one at 20 that becomes 21, a new one at 16, and a new one at 15; and an unchanged function at 30; expected: the 15 → 16 and the new 16 fail the complexity floor by name; the 20 → 21 and the unchanged 30 are listed as already more than 15 and do not fail; the new 15 passes; a change in no language lizard reads skips without blocking
4. [api-trace] state: a lint or complexity miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
5. [api-trace] state: the security job of `ci.yml`, the merge gate and `aayudha-security.yml` as `aayu init --gates` renders them, run with stand-ins whose semgrep and lizard wheels do not match their pinned SHA-256; expected: each install step fails before the floors run; with matching stand-ins semgrep and lizard install from `--require-hashes` locks and the floors run
6. [api-trace] state: `aayu verify` on this repository's own pull request; expected: the lint check runs golangci-lint from `.golangci.yml` and the complexity check runs lizard 1.24.1; both checks appear in the proof pack with their versions

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Run the lint and complexity floors against git fixtures of a base and a head commit with stand-in linters and a stand-in lizard that print recorded reports. Run the workflow install steps with stand-in pip, curl, and wheel files. Run `aayu verify` on this repository with the real golangci-lint and lizard.

### Environment

local checkout and this repository's CI build job on a pull request; stand-in tools in a temporary PATH, never production

### Links

depends-on #496

### Checkpoints

1. `go test ./internal/verify -run 'TestLintFloor|TestComplexityFloor'` shows the changed-line rule, detection, the crossing rule, and the advisory date (criteria 1, 2, 3, 4)
2. `go test . -run 'TestHashedPythonTools'` and a local `aayu verify` on this repository (criteria 5, 6)

### Assumptions

- "changed lines" are `Run.Lines`, the same set the coverage floor uses
- the linter runs at the repository's own version; aayu pins no linter and records the version that ran
- `verify.new_floors_blocking_from` is 2026-11-14, shared by every floor Decision #480 added, and lands with #496

### Open questions

- deferred: a finding a change causes on an unchanged line (for example a variable left unused) is not counted; Decision #480 records this gap

### Provenance

Request: "in proof floors should talk about code quality and user experience if there is a UI, or developer experience if it's an API or CLI. Goal: proof floors that, chosen by the story's layer footprint, cover: Code quality, for every change. Code quality: the repo's own linter with zero new findings on changed lines, and a complexity limit on changed functions. Ground rule 7 still holds: a floor is deterministic, and no model opinion can be a floor. It should list each floor, its default, the layers it applies to, the tool pinned by SHA-256, and what skipped means. Build only after it is accepted, one story per floor family, each with tests that a plausible wrong implementation would fail. Start the stories outside DAST now under #480's acceptance (code quality, UX, API and CLI developer experience)."
Term: "UI" → the product's user interface, layer L01; "user experience if there is a UI"
Term: "API" → a product's HTTP or RPC interface; "developer experience if it's an API or CLI"
Term: "CLI" → a product's command-line interface; "developer experience if it's an API or CLI"
Term: "UX" → user experience; "user experience if there is a UI"
Term: "SHA-256" → the hash each downloaded tool is checked against; "the tool pinned by SHA-256"
Term: "DAST" → the ZAP scan of the running app, built in #496; "Start the stories outside DAST now"
1. stated (request): "the repo's own linter with zero new findings on changed lines"
2. stated (request): "the repo's own linter" / "what skipped means"
3. stated (request): "a complexity limit on changed functions"
4. stated (request): "Start the stories outside DAST now under #480's acceptance"
5. stated (request): "the tool pinned by SHA-256"
6. stated (request): "Code quality, for every change"
````
