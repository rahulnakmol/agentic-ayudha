# Getting started

This guide installs `aayudha` (alias `aayu`), checks the machine, and connects a product repository to Amp,
Cursor, or Claude Code. It is published on the docs site by `aayu tell docs --site` (WP8.2).

## 1. Install

You need `git` 2.40 or newer and `git-lfs`: the ledger uses linked worktrees and Git LFS. `gh` is optional.

### macOS and Linux

```sh
curl -fsSL https://raw.githubusercontent.com/rahulnakmol/agentic-ayudha/main/install.sh | sh
```

`install.sh` installs a signed `v*` release and nothing else. Before it copies a file it checks two things:

1. `checksums.txt` carries a Sigstore bundle whose certificate names this repository's `release.yml` workflow
   on the exact tag being installed (keyless cosign). If `cosign` is not on your `PATH`, the script downloads a
   pinned cosign release and checks its SHA-256 first.
2. The archive's SHA-256 matches its line in that signed `checksums.txt`.

If either check fails, it installs nothing and exits non-zero. The binary goes to `~/.local/bin` (set
`AAYU_INSTALL_DIR` to change it) with an `aayu` link beside it; add that directory to your `PATH` if the script
says so. `AAYU_VERSION=v0.1.0` pins a release; the default is the latest.

### Windows

In Git Bash, run the same `install.sh` line; it installs `aayudha.exe` and `aayu.exe`. In PowerShell, use
[Scoop](https://scoop.sh) once the bucket is published with the first stable release:

```powershell
scoop bucket add rahulnakmol https://github.com/rahulnakmol/scoop-bucket
scoop install aayudha
```

Homebrew (`brew install --cask rahulnakmol/tap/aayudha`) and winget (`winget install rahulnakmol.aayudha`)
follow the same stable release. Scoop and winget check the archive's SHA-256 from their manifest; only
`install.sh` checks the cosign signature.

### Check the machine

```sh
aayu version
aayu doctor
```

`doctor` exits 0 when nothing blocks you. `WARN` and `N/A` rows name optional tools and setup you can do later;
a `FAIL ... [blocking]` row (for example, `git` older than 2.40 or no `git-lfs`) must be fixed first.

## 2. Connect a product repository

Run these in a clone of the repository you want the pod to work on.

1. Authenticate with your own GitHub account: `gh auth login`, or set `AAYU_GITHUB_TOKEN`.
2. Set the repository up once, as its admin. `aayu init` creates the labels, issue forms, `.aayudha/config.toml`,
   the `ledger` branch, and its ruleset, after a confirmation on the terminal.
3. Install the skills for your host. Without `--apply`, `install` prints the plan and writes nothing:

   ```sh
   aayu install --host amp      # or cursor, claude, all
   aayu install --host amp --apply
   ```

Skill copies are real files, never symlinks, recorded with their hashes in `.aayudha/lock.toml`;
`aayu doctor --fix` re-syncs them after an upgrade.

## 3. Use it from your host

Each host gets the same skills, the `aayu mcp` server, and hooks that run `aayu hook <name>` and nothing else.

| | Amp | Cursor | Claude Code |
|---|---|---|---|
| Skills | `.agents/skills/<verb>/` | `.agents/skills/<verb>/` | `.claude/skills/<verb>/` |
| MCP server | `amp.mcpServers` in `~/.config/amp/settings.json` (a home-directory edit, confirmed separately) | `.cursor/mcp.json` | `.mcp.json`, enabled in `.claude/settings.json` |
| Hooks | `.amp/plugins/aayudha.ts` | `.cursor/hooks.json` | `.claude/settings.json` |
| Agent guidance | `AGENTS.md` sections | `AGENTS.md` sections | `CLAUDE.md` stub that imports `@AGENTS.md` |

Ask the agent in plain words to use a skill by name, for example "use the grill skill to write a Goal for
faster claims review". Claude Code also offers each skill as a slash command, such as `/grill`. The skill runs
the verb through the binary, so every GitHub write goes through the actions log.

### A first loop

1. `grill` interviews you until a work item is complete, then files it (`aayu grill goal`, then `feature`,
   then `story`).
2. `ready` validates a story's Completion Goal and labels it `status:ready`.
3. `build` builds the story in a worktree with a checkpoint per layer, runs `verify`, and opens a pull request.
4. `inspect` reviews the pull request against its plan and proof pack.
5. `ship` squash-merges it after a green merge gate and a human pass.

`aayu status` rolls up progress and proof coverage. The [verbs reference](../README.md#verbs) lists all 32
verbs; each skill's page explains its steps.

## Where to go next

- [CONTRIBUTING.md](../CONTRIBUTING.md): the branch model, pull request flow, and how releases are promoted.
- [SECURITY.md](../SECURITY.md): reporting a vulnerability.
- [CONSTRAINTS.md](../CONSTRAINTS.md): the default floors, autonomy, and budgets.
