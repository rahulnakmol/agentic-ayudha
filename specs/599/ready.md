# rahulnakmol/agentic-ayudha#599: A floor waiver counts whoever closes it, not only the Decision authority

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `f8b3f2d30857f5be99a6c173ffd13f9d5a9ff62c983d74ca8f06902d40e6f744` |
| Completion Goal hash | `93aa9fbcb7fb113ca89705be1c0bf018a746ea39fa598582eed9e0cf791d452b` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/599/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a failing sast floor with a waiver Decision closed by the configured authority, one closed by another human with write access, and one whose section names a different login than its closer; expected: only the first waives the floor, and verify names the other two as ignored with the authority they lacked
2. [api-trace] state: an open aayudha:security Bug last seen on rolling with a waiver closed by a non-authority human, and `aayu release --to rolling --dry-run`; expected: the promotion is held, naming the Bug
3. [api-trace] state: no human configured as the floor-waiver authority, and a waiver closed by any human; expected: the floor is waived, as today

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

A floor waiver counts when anyone who is not a bot closes it, not only the Decision authority. `waivers` in `internal/cli/verify.go` keeps a Decision sub-issue as a waiver when it is closed as completed, its Decision section starts `waive, accepted by @`, and `closed_by` is not a bot; it never compares `closed_by`, or the login the section names, with the authority `decide.Authority` and `decide.Login` resolve for a floor waiver (`[decisions.authority]`, the floors layer, else the founder). A collaborator with write access can therefore file a Decision under a story, a Bug, or a security Bug, write `waive, accepted by @founder`, and close it themself: verify's floors (`internal/cli/verify.go`), the error-budget hold in `aayu release` (`internal/cli/release.go`), and the security scan and promotion hold (`internal/cli/verifysecurity.go`) all treat the floor as waived. Found by Codex's security review of PR #500 (comment r4306146548); the function is older than that PR.

### Steps to reproduce

1. In a repository where `[decisions.authority]` or `[autonomy] approver` resolves the floor-waiver authority to `founder`, give a second account write access.
2. With that account, add a Decision sub-issue under a story whose sast floor fails, with the Decision section `waive, accepted by @founder on 2026-10-18.`, and close it as completed.
3. Run `aayu verify <story>`.

### Expected behaviour

The waiver counts only when the account that closed it is the resolved floor-waiver authority and the section names that same login; a waiver closed by anyone else is ignored and the floor still fails. When no human is configured (the day-one path, `decide.Login` returns ""), any human closer still counts, as today.

### Actual behaviour

The sast floor is reported as waived and verify exits 0.

### Completion Goal

1. [api-trace] state: a failing sast floor with a waiver Decision closed by the configured authority, one closed by another human with write access, and one whose section names a different login than its closer; expected: only the first waives the floor, and verify names the other two as ignored with the authority they lacked
2. [api-trace] state: an open aayudha:security Bug last seen on rolling with a waiver closed by a non-authority human, and `aayu release --to rolling --dry-run`; expected: the promotion is held, naming the Bug
3. [api-trace] state: no human configured as the floor-waiver authority, and a waiver closed by any human; expected: the floor is waived, as today

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Fake GitHub fixtures with waiver Decisions closed by different accounts under each configuration, run through `aayu verify`, `aayu verify --security --ci`, and `aayu release --dry-run`.

### Environment

local checkout and this repository's CI build job on a pull request

### Technical Owner

quality-architect

### Checkpoints

1. `go test ./internal/cli -run TestWaiverAuthority` shows only the authority's waiver counting, across verify, the security scan, and release (criteria 1, 2)
2. the same test's day-one case shows any human's waiver counting when no authority is configured (criterion 3)

### Assumptions

- the authority for a floor waiver is what `decide.Authority` returns for a waiver Decision, resolved to a login by `decide.Login`, as `aayu decide --accept` already enforces

### Provenance

Request: "file the waivers() gap (the closer isn't checked against the Decision authority) as its own Bug through aayu, P1, with a regression test when it's fixed"
1. stated (Codex security review on PR #500, r4306146548): a lower-privileged human can fabricate and close a waiver Decision, and `waivers` never checks `decide.Authority` or the closer
2. stated (orchestrator thread): file it as its own Bug, P1, with a regression test when it is fixed
3. stated (founder, via the orchestrator thread, 2026-10-18: confirmed): keep today's day-one rule that any human may accept when no authority login is configured (`decide.Login` in internal/decide/decide.go), as `aayu decide --accept` does
Term: "P1" → the `priority:P1` label; "P1"
````
