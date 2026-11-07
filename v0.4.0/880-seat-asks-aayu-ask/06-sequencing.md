# Sequencing: aayu ask

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts

1. #871 P1 (the session source) and #873 P1 (the Need registry) merge, because #880 depends on both (#880 Links).
   P1 and P3 here need only #871; P2 needs #873.
2. A person approves #880's baselines and runs Approve to build (`aayu ready 880`). None exists on the ledger.

## Waves

The verb file `internal/cli/ask.go` is the centre of P1 and P3, so they go in order. Three lanes at most run at
once. More lanes would put two lanes on `ask.go`.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 `aayu ask` and the ask record | `internal/cli/ask.go` (new), `internal/verbs/registry.go`, generated files |

### Wave 2 (after P1)

| Lane | Slice | Files |
|---|---|---|
| 1 | P3 `--answer` and first answer wins | `internal/cli/ask.go` |
| 2 | P2 the Need kinds (also after #873 P1) | `internal/needs/**` (new kind entries) |

### Wave 3 (after P3)

| Lane | Slice | Files |
|---|---|---|
| 1 | P4 `/aayu answer` | `internal/cli/approve.go`, `.github/workflows/aayudha-approve.yml`, `docs/threat-model.md` |
| 2 | P5 build asks, `answers.md`, Kanha | `internal/cli/build.go`, `internal/cli/buildnext.go`, `internal/cli/kanha.go` |

The lanes edit no common file.

## Hot files

- `internal/verbs/registry.go` and its generated outputs (AGENTS.md, "Generated files"): #881 also adds to the
  registry. Merge #880 P1 first.
- `internal/cli/build.go`: shared with #871, #872, #882, and #873 P4a. #882 depends on #880 (#882 Links), so P5 merges
  before #882's build changes.
- `internal/needs/**`: owned by #873; P2 only adds kind entries.

## Person steps and protected paths

- P4 edits `.github/workflows/aayudha-approve.yml`. It is in `[autonomy] always_person` (`.aayudha/config.toml:87`)
  and under `/.github/` in `.github/CODEOWNERS`. So the PR needs a person to ship and fails `protected-paths` by
  design; only the founder's recorded admin bypass merges it (AGENTS.md, "Protected paths").
- P4 adds who may answer, which is close to "a change to who may approve or bypass" (AGENTS.md, the narrow set).
  Inferred: a person reviews it.
- If P1 adds a `skills/ask/SKILL.md` (#881 lists `skills/ask/SKILL.md` in its Expected paths), that path is under
  `/skills/` in `.github/CODEOWNERS`.
- A config key under `internal/config/` needs a person (always_person, `.aayudha/config.toml:83`).
