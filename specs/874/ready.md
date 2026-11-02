# rahulnakmol/agentic-ayudha#874: Feature baselines are drafted, approved, and pinned

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, feature |
| Body hash | `5f052d2259d64b66ea4f875e68f695f7f397bad77c680b1ece08e1afa2683a9f` |
| Completion Goal hash | n/a (this type has no Completion Goal) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/874/ready.md` |

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Intent, spec, technical design, and UX design each get a baseline: an agent drafts it, a person approves it, and later edits show as drift. This is step S5 of the core design (aayu core: the human-dependency engine, agreed by @rahulnakmol on 2026-10-21).

### Problem

Only the story Ready snapshot and the release scope baseline freeze anything (`internal/ready/ready.go:146-303`). `aayu plan` refuses a Feature (`internal/cli/plan.go:144-146`) (G5, Q16).

### Scope

In: `aayu ready <f> --part <p> --request` (bot) and `--part <p>` (human-only); `specs/<f>/baselines/<p>-v<k>.json` with pins and a hash; `aayu plan <feature>` writing `specs/<f>/design.json`; UX pins of the design stage manifests; the scoped early-build approval `aayu ready <f> --early-build <story>` (E1). Out: the seat reviews (S6).

### Expected paths

- `internal/ready/**`
- `internal/cli/ready.go`
- `internal/cli/plan.go`
- `internal/cli/ask.go`
- `internal/planning/plan.go`
- `internal/design/**`
- `internal/cli/inspect.go`
- `internal/cli/ship.go`
- `internal/planning/design.go`
- `internal/verbs/registry.go`
- `internal/config/defaults.toml`
- `internal/config/config.go`
- `skills/ready/SKILL.md`
- `skills/plan/SKILL.md`

### Links

depends-on #871
````
