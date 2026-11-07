# Sequencing: host and session on every agent write

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts (a person)

1. File the Stories (an agent can run `aayu slice 871 --draft <file> --create`; `aayu slice --help`).
2. A person approves the intent, spec, and tech baselines (`aayu ready 871 --part <part>`, through GitHub's device
   flow; `aayu ready --help`).
3. A person runs Approve to build: `aayu ready 871`.

## Waves

Four slices, two of them independent at the start. So at most two lanes run at once. More lanes would split one
small change across files that must agree on one type.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 `Env.source()` and the host on every actions record | `internal/cli/env.go`, `internal/cli/asked.go`, `internal/cli/actions.go` |
| 2 | P2 the optional `session` field | `internal/actions/actions.go` and its tests |

The lanes edit no common file.

### Wave 2 (after P1 and P2 merge)

| Lane | Slice | Files |
|---|---|---|
| 1 | P3 the source in build and delegate state | `internal/cli/build.go`, `internal/cli/actions.go` (pass the session to `actions.Options`), `internal/delegate/state.go`, `internal/cli/delegate.go` |
| 2 | P4 hosts and config (inferred) | `internal/cli/install.go`, `internal/hosts/hook.go`, `internal/hosts/install.go`, `internal/config/**`, `CONSTRAINTS.md` |

The lanes edit no common file.

## Hot files

- `internal/cli/build.go`: Features #872, #880, and #882 also plan to edit it (their Expected paths). Merge #871's
  P3 first, because #872 and #880 depend on #871 (their Links).
- `internal/config/defaults.toml` and `CONSTRAINTS.md`: generated together (`go generate ./...`; AGENTS.md,
  "Generated files"). #872, #880, #881, #882, and #886 list config paths too.

## Person steps and protected paths

- P4 touches `internal/config/`, which is in `[autonomy] always_person` (`.aayudha/config.toml:83`), and
  `internal/config/defaults.toml` and `CONSTRAINTS.md`, which are in `.github/CODEOWNERS`. So P4's PR needs a
  person to ship, and fails `protected-paths` by design.
- P1 to P3 touch no protected path, so each can merge on proof through `aayu ship`.
