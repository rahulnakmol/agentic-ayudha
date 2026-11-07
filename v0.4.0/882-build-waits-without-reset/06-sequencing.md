# Sequencing: a build waits on a person without a reset

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts

1. #880 merges its ask, answer, and build path (#880 P1, P3, P5), because #882 depends on #880 (#882 Links).
2. A person approves #882's baselines and runs Approve to build (`aayu ready 882`).

## Waves

Three small slices in a chain. One lane at a time. A second lane would edit the same build state type as P1.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 waiting and paused | `internal/cli/build.go`, `internal/cli/ask.go` |

### Wave 2 (after P1)

| Lane | Slice | Files |
|---|---|---|
| 1 | P2 lease and stale sweep | `internal/cli/lease.go`, `internal/cli/gc.go`, `internal/cli/buildnext.go` |

### Wave 3 (after P2)

| Lane | Slice | Files |
|---|---|---|
| 1 | P3 config key (inferred) | `internal/config/**`, `CONSTRAINTS.md` |

P3 can run beside P2 if the key's name is agreed first, because the two edit no common file.

## Hot files

- `internal/cli/build.go`: shared with #871 P3, #872 P2, #873 P4a, and #880 P5. Merge order: #871, #880, then #882.
- `internal/cli/ask.go`: owned by #880, also edited by #881.
- `internal/cli/buildnext.go`: shared with #880 P5.

## Person steps and protected paths

- P3 edits `internal/config/` (`[autonomy] always_person`, `.aayudha/config.toml:83`) and
  `internal/config/defaults.toml` and `CONSTRAINTS.md` (`.github/CODEOWNERS`). It needs a person to ship and fails
  `protected-paths` by design. A new key in `[build]` or `[autonomy]` is in `always_person_sections`
  (`internal/config/defaults.toml:70`).
- P1 and P2 touch no protected path.
