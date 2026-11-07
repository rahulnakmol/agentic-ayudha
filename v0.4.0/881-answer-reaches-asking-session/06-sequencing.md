# Sequencing: the answer reaches the asking session

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts

1. #880 P1 and P3 merge (the ask and the answer record), because #881 depends on #880 (#881 Links).
2. A person approves #881's baselines and runs Approve to build (`aayu ready 881`).

## Waves

P2 and P3 both edit `internal/cli/ask.go`, which #880 owns. So the waves keep them apart or name a merge order. Two
lanes at most run at once.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 the receipt | `internal/cli/ask.go`, `internal/delegate/state.go` |

### Wave 2 (after P1)

| Lane | Slice | Files |
|---|---|---|
| 1 | P2 cloud push | `internal/delegate/receivers.go`, `internal/cli/ask.go` (push call) |
| 2 | P5 receipt in the inbox (also after #873 P2) | `internal/cli/inbox.go` or `internal/needs/**` |

### Wave 3 (after P2)

| Lane | Slice | Files |
|---|---|---|
| 1 | P3 `--wait` | `internal/cli/ask.go`, `internal/verbs/registry.go`, generated files |

### Wave 4 (after P2 and P3)

| Lane | Slice | Files |
|---|---|---|
| 1 | P4 skill and host files | `skills/ask/SKILL.md`, `hosts/**`, `internal/hosts/**`, `internal/cli/install.go` |

## Hot files

- `internal/cli/ask.go`: shared with #880 and #882 (#882 Expected paths). Merge order: #880, then #881 P1, then #882.
- `internal/verbs/registry.go` and generated docs: shared with #880 and #872 P1.
- `internal/cli/inbox.go`: owned by #873's move; merge #873 P1 and P2 first.

## Person steps and protected paths

- P4 adds `skills/ask/SKILL.md`: `/skills/` is in `.github/CODEOWNERS`, so `protected-paths` fails by design. Some
  host files are in CODEOWNERS too: `/hosts/claude/hooks.json`, `/hosts/codex/hooks.json`, `/hosts/cursor/hooks.json`,
  `/hosts/amp/aayudha.ts`.
- A config key under `internal/config/` needs a person to ship (`.aayudha/config.toml:83`).
