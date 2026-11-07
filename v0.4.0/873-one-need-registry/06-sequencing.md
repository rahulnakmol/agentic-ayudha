# Sequencing: one Need registry

The slices P1 to P5 are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)). Bug #859 is
filed.

## Before any lane starts (a person)

1. Triage Bug #859 so it can be Ready (Decision #861).
2. Approve #873's baselines and run Approve to build (`aayu ready 873`). None exists on the ledger.

## Waves

The page call sites sit in many files, so the call-site work splits well. The peak is four lanes in wave 3. More
lanes would put two lanes on one file.

### Wave 1

| Lane | Item | Files |
|---|---|---|
| 1 | Bug #859 | `internal/notify/notify.go`, and the command path that calls `Sender.Flush` (inferred: one of `internal/cli/halt.go` `Env.page` or the sweep) |
| 2 | P1 the move | `internal/needs/**` (new), `internal/verbs/registry.go`, generated files, `internal/cli/inbox.go`, `inboxtui.go`, `inboxaudit.go`, `inboxready.go`, `inboxwhy.go`, `inboxwork.go` |

If #859's fix edits `internal/cli/halt.go`, P4b in wave 3 must rebase on it; the waves keep them apart.

### Wave 2 (after P1; P3 also after #859)

| Lane | Item | Files |
|---|---|---|
| 1 | P2 fields and areas | `internal/needs/` (kind and area files), `internal/cli/inbox.go`, `internal/cli/inboxtui.go` |
| 2 | P3 page kind and lint | `internal/notify/notify.go`, a new lint test file in `internal/needs/` |

Lane 1 and lane 2 both touch `internal/needs/` but different files. Merge P2 first if both change the kind type.

### Wave 3 (after P2 and P3)

| Lane | Item | Files |
|---|---|---|
| 1 | P4a | `internal/cli/build.go`, `internal/cli/delegate.go` |
| 2 | P4b | `internal/cli/halt.go`, `internal/cli/status.go`, `internal/cli/streak.go` |
| 3 | P4c | `internal/cli/decide.go`, `internal/cli/inspect.go`, `internal/cli/inspect_app.go`, `internal/inspect/inspect.go`, `internal/cli/scope.go` |
| 4 | P4d | `internal/models/gateway.go` |

### Wave 4 (after P2)

| Lane | Item | Files |
|---|---|---|
| 1 | P5 V9 precision fields | `internal/needs/**` |

P5 can start in wave 3 as a fifth lane if it edits only `internal/needs/`; wave 3 lanes do not touch that package.

## Hot files

- `internal/cli/inbox.go` (the `inbox*.go` files total 14,078 lines with tests on `dev` at bc78349; 13,138 at 884d81b, `wc -l internal/cli/inbox*.go`):
  #881 also edits `inbox.go` (its Expected paths). Merge #873's P1 and P2 before #881.
- `internal/cli/build.go`: shared with #871, #872, #880, and #882.
- `internal/verbs/registry.go` and its generated docs: P1 adds a layer glob.

## Person steps and protected paths

- No planned path is in `[autonomy] always_person` (`.aayudha/config.toml:75-101`) or in `.github/CODEOWNERS`, unless
  a slice adds a config key (none is named). So the PRs can merge on proof through `aayu ship`.
