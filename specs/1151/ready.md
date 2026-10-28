# rahulnakmol/agentic-ayudha#1151: CI build's go test has no timeout, so a slow internal/cli fails it at random

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `37e20467d078bfd6e053b0b3ebb75d53fa47d017ced6bd55b05f9fb0c6205844` |
| Completion Goal hash | `1d0e8c3f7760f2a9fdcc22b64ab070b55f2d2c72891fb6432381073aec7f2dd7` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1151/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the step "Build, vet, test" of the `build` job in .github/workflows/ci.yml at the PR head; expected: its `go test` runs with `-timeout 30m`, as `go test -timeout 30m -coverprofile=coverage.out ./...`
2. [api-trace] state: each `go test` in .github/workflows/*.yml that runs every test of its packages (no `-run` filter); expected: each one sets `-timeout`, the `integration` job's `go test -race -count=1 -tags integration ./...` too, so no step uses go test's 10-minute default

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

The CI `build` job can fail at random, because its `go test` has no `-timeout`. The step "Build, vet, test" runs `go test -coverprofile=coverage.out ./...` (.github/workflows/ci.yml:93). So go test uses its 10-minute default for each package. `internal/cli` took 576 s on dev (run 40710734386), and #1147 reached the limit once (590 s on its rerun). When the limit is reached, go test stops the package with a panic, and `build` fails.

The `integration` job has the same gap: `go test -race -count=1 -tags integration ./...` (.github/workflows/ci.yml:475) runs `internal/cli` with the race detector and no `-timeout`. The `extended-integration` job runs the same command with `-timeout 30m` (ci.yml:587) for this reason.

### Steps to reproduce

1. Check out origin/dev at bdd02c13.
2. Read .github/workflows/ci.yml:93 and ci.yml:475: neither `go test` sets `-timeout`.
3. Open the `build` job of dev's ci run 40710734386: `internal/cli` took 576.111 s, 24 s below the 600 s default.

### Expected behaviour

Each `go test` step in CI that runs the tests of a whole package sets `-timeout`. The `build` job runs `go test -timeout 30m -coverprofile=coverage.out ./...`, so a slow runner does not fail it.

### Actual behaviour

The `build` and `integration` jobs use go test's 10-minute default. `internal/cli` is near that limit on CI, and #1147's build reached it once.

### Completion Goal

1. [api-trace] state: the step "Build, vet, test" of the `build` job in .github/workflows/ci.yml at the PR head; expected: its `go test` runs with `-timeout 30m`, as `go test -timeout 30m -coverprofile=coverage.out ./...`
2. [api-trace] state: each `go test` in .github/workflows/*.yml that runs every test of its packages (no `-run` filter); expected: each one sets `-timeout`, the `integration` job's `go test -race -count=1 -tags integration ./...` too, so no step uses go test's 10-minute default

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Read the go test commands of the CI workflows at the PR head.

### Environment

go test of the repository root package (workflows_test.go), which reads .github/workflows/*.yml; never production

### Technical Owner

devops-devex-tinkerer

### Assumptions

#### Notes

The fix adds `-timeout 30m` to two commands in ci.yml. It does not change which tests run. #1123 gave the local gate the same timeout; it did not change ci.yml.

`internal/cli` is slow, and that is a separate problem: 576 s on CI, and 787 s to 826 s on a 2-core orb. This Bug does not split or speed up the package. A later Story can do that.

#### Review guidance

- Check: Criterion 2: a `go test` with `-run '^$'` (benchmarks only) or another `-run` filter is out of scope.
- Ignore: the time that `internal/cli` takes.
- Risk: a test that hangs now runs up to 30 minutes before go test stops it.

#### Behaviours

1. says: A CI go test of whole packages never uses the 10-minute default; rule: never; source: criteria 1, 2

### Checkpoints

1. The `build` job's go test in ci.yml runs with `-timeout 30m` (criteria 1)
2. No other ci.yml go test of whole packages uses the 10-minute default; the `integration` job gets `-timeout 30m` (criteria 2)

### Parked ideas

- Split or speed up `internal/cli`, so its tests run well below 10 minutes.

### Provenance

Request: "Fix: add `-timeout 30m` to that go test command, and check the other go test lines in ci.yml for the same gap."
1. stated (request): "add `-timeout 30m` to that go test command"
2. stated (scope): "check the other go test lines in ci.yml for the same gap"
````
