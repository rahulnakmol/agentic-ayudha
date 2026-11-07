# Sequencing: you write behaviours

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts

1. #873 P1 and P2 (the Need registry) and #880 P1 and P3 (ask and answer) merge for P3 and P4. P1, P5, P6, and P7 do
   not need them.
2. A person approves #891's baselines and runs Approve to build (`aayu ready 891`).

## Waves

The check type in `internal/verify/featuremap.go` is the shared contract, so P1 goes alone first. After it, the work
spreads over distinct files. The peak is four lanes in wave 3. More would put two lanes on `decide.go` or on
`internal/verify/`.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 check fields and status | `internal/verify/featuremap.go`, `internal/verify/behaviour.go` |

### Wave 2 (after P1)

| Lane | Slice | Files |
|---|---|---|
| 1 | P2 approvals and `decide --behaviour` | `internal/cli/decide.go`, `internal/verify/approvals.go` (new), `internal/verbs/registry.go`, generated files |
| 2 | P6 standing criteria | `internal/workitems/`, `internal/cli/ship.go` |
| 3 | P7 this repository's map | `.agents/skills/verify-aayudha/` |

### Wave 3 (after P2; P3 also after #873, P4 also after #880)

| Lane | Slice | Files |
|---|---|---|
| 1 | P3 the BEHAVIOUR CHANGE hold | `internal/inspect/`, `internal/cli/inspect.go`, `internal/needs/**` |
| 2 | P4 build stops on an approved check | `internal/cli/build.go` |
| 3 | P5 `--sets` | `internal/cli/decide.go`, `internal/verbs/registry.go`, generated files |
| 4 | P9 Behaviours view data | `internal/verify/view.go` (new, name inferred) |

### Wave 4 (after P3)

| Lane | Slice | Files |
|---|---|---|
| 1 | P8 ETHOS and the narrow set | `ETHOS.md`, `.aayudha/config.toml` |

## Hot files

- `internal/cli/decide.go`: P2 and P5 are in different waves for this reason.
- `internal/verbs/registry.go` and generated docs: P2 and P5 add flags. #880 and #881 edit it too.
- `internal/cli/inspect.go` and `internal/inspect/`: shared with #887's #1223 and #873's P4c.
- `internal/cli/build.go`: shared with #871, #872, #873, #880, and #882.
- `internal/cli/ship.go`: shared with #872 P2.

## Person steps and protected paths

- "Its PR needs a person to ship." (#891 Scope). The narrow set covers more than one slice here:
  - P6 edits `internal/cli/ship.go`, which is in `[autonomy] always_person` (`.aayudha/config.toml:77`).
  - P7 adds files under `/.agents/`, which is in `.github/CODEOWNERS`, so `protected-paths` fails by design and only
    the founder's admin bypass merges it (AGENTS.md, "Protected paths").
  - P8 edits `.aayudha/config.toml`, which is in `always_person_config` (`.aayudha/config.toml:102`) and in
    `.github/CODEOWNERS`. After P8, every change to `internal/verify/featuremap.go`, `behaviour.go`, and the approval
    reader needs a person (#867 option 1).
  - P2 and P3 change who may approve a behaviour change, which is the narrow set's "a change to who may approve or
    bypass" (AGENTS.md). Inferred: each needs a person.
- The founder writes each `says` sentence and approves it (#867 option 1). P7's map needs his sentences before its
  checks count as enforced.
