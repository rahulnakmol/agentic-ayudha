# rahulnakmol/agentic-ayudha#640: A committed .aayudha/config.json or config.local.* changes protected policy

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `3fa881a3281e84d9f100580a7c3eb4187b2aff2f78ea789f0b4e0c17775c36c5` |
| Completion Goal hash | `40cc7ddee0fc342e5e3c6c5ea7c57ac26a7f380e7da255ed04a0e70208a88d31` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/640/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `.aayudha/config.json` sets `[decisions] solo_provisional_after_hours`, and in a second run `[integrations.posthog] allow_pii`; expected: config loading refuses each, naming `.aayudha/config.json` and saying the key is read only from `.aayudha/config.toml`
2. [api-trace] state: a committed `.aayudha/config.local.toml` clears `production` for an environment `.aayudha/config.toml` marks as production, loaded with `GITHUB_ACTIONS=true` and again with `--ci`; expected: the local layer is ignored with a warning naming the file, the environment stays production, and verify refuses the target
3. [api-trace] state: the same committed local file loaded outside CI; expected: the local layer still applies, as it does today for a person's own checkout

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Only `.aayudha/config.toml` is code-owned (`.github/CODEOWNERS`), but config loading (`internal/config/load.go`, `Load`) also reads `.aayudha/config.json` as the repository layer and `.aayudha/config.local.toml` and `.aayudha/config.local.json` as the local layer. A pull request can commit any of them: `config.local.toml` is in `.gitignore` but `git add -f` commits it, and `config.local.json` is not ignored at all. So an ordinary PR can change policy that is meant to be the protected config's alone:

- `[decisions] solo_provisional_after_hours` and `[integrations.<provider>] allow_pii` are refused from every layer except `LayerRepo`, which `.aayudha/config.json` also loads as (`Load`, the checks after `add`);
- a committed `config.local.*` applies in every workflow that loads config, so it can, for example, clear an environment's `production` flag (`[verify.environments.<env>] production`), which `verify.ResolveTarget` uses to refuse a production target for every floor.

#617's thread puts the three files into aayu's `always_person` set (commit 138c4fe, not merged), which covers merge on proof but not GitHub's own rules.

### Steps to reproduce

1. Commit `.aayudha/config.json` with `{"version": 1, "decisions": {"solo_provisional_after_hours": 1}}` and run any verb that loads config.
2. Commit `.aayudha/config.local.toml` (with `git add -f`) that sets `[verify.environments.staging] production = false` for an environment `.aayudha/config.toml` marks as production, and run `aayu verify` with `AAYU_ENV=staging` and `GITHUB_ACTIONS=true`.

### Expected behaviour

A protected-only key is read only from `.aayudha/config.toml`; from `.aayudha/config.json` the load is refused, naming the file. In CI (`--ci`, or `GITHUB_ACTIONS=true`) config loading ignores `.aayudha/config.local.*` and warns, naming each ignored file, so a committed local file never changes policy in a workflow. `.github/CODEOWNERS` also owns `/.aayudha/config.json` and `/.aayudha/config.local.*`.

### Actual behaviour

`.aayudha/config.json` sets the solo window and `allow_pii` without protected-path review, and a committed `config.local.toml` clears the production flag in CI, so verify targets the production environment.

### Completion Goal

1. [api-trace] state: `.aayudha/config.json` sets `[decisions] solo_provisional_after_hours`, and in a second run `[integrations.posthog] allow_pii`; expected: config loading refuses each, naming `.aayudha/config.json` and saying the key is read only from `.aayudha/config.toml`
2. [api-trace] state: a committed `.aayudha/config.local.toml` clears `production` for an environment `.aayudha/config.toml` marks as production, loaded with `GITHUB_ACTIONS=true` and again with `--ci`; expected: the local layer is ignored with a warning naming the file, the environment stays production, and verify refuses the target
3. [api-trace] state: the same committed local file loaded outside CI; expected: the local layer still applies, as it does today for a person's own checkout

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Config fixtures with committed `.aayudha/config.json` and `.aayudha/config.local.*` files, loaded through `config.Load` and the CLI with and without `--ci` and `GITHUB_ACTIONS`.

### Environment

local checkout and this repository's CI build job on a pull request

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/config -run TestProtectedOnlyKeysFromTOML` refuses both keys from `config.json` (criterion 1)
2. `go test ./internal/config ./internal/cli -run TestLocalConfigIgnoredInCI` shows the local layer ignored with a warning in CI and applied outside it (criteria 2, 3)

### Assumptions

- the CODEOWNERS lines touch a path in Decision #593's narrow set, so they ship as a separate commit and merge only with the founder's OK; the code fix and tests do not depend on them
- #509 (Story #496) applies the same TOML-only rule to `verify.environments.<env>.active_scan`; whichever PR merges second rebases onto the other

### Provenance

Request: "After #509 merges, take this as one Bug, filed through aayu, with one PR in v0.3.0: (a) In CI (--ci, or GITHUB_ACTIONS set), config loading ignores config.local.* and warns. A committed local file must never change policy in a workflow. (b) Add /.aayudha/config.json and /.aayudha/config.local.* to .github/CODEOWNERS. (c) A test for each of your two cases: solo_provisional_after_hours or allow_pii set from config.json, and the production flag cleared from config.local.toml."
1. stated (orchestrator thread): only `.aayudha/config.toml` is code-owned; `config.json` and `config.local.*` are not, and a PR can commit them
2. stated (orchestrator thread): "Base the config-layer Bug on dev now and run it in parallel."
3. default: criterion 1 needs the protected-only rules to accept the repo layer only from `config.toml`, as #509 does for `active_scan`; CODEOWNERS alone would not make it testable
Term: "CI" → `--ci`, or `GITHUB_ACTIONS=true` in the environment; "In CI (--ci, or GITHUB_ACTIONS set)"
````
