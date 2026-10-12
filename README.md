# agentic-ayudha

`aayudha` (alias `aayu`) is one static Go binary plus a set of host skills that run an agentic software
delivery pod: twelve seats, nine application layers, and 32 verbs that take work from a grilled idea to a
measured release, with proof packs and human gates along the way.

Status: **pre-0.1**. There is no `v*` release yet; v0.1.0 is the first. The build plan is
[docs/BUILD-PLAN.md](docs/BUILD-PLAN.md); it is the source of truth for scope and sequencing.

Documentation: <https://rahulnakmol.github.io/agentic-ayudha/>, built from `dev` by `aayu tell docs --site` (WP8.2).

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/rahulnakmol/agentic-ayudha/main/install.sh | sh
aayu doctor
```

`install.sh` installs only a signed `v*` release: it checks the cosign signature of `checksums.txt` against this
repository's `release.yml` workflow identity, then the archive's SHA-256. It runs on macOS, Linux, and Windows (Git
Bash). [docs/getting-started.md](docs/getting-started.md) covers Scoop, Homebrew, and winget, and connecting Amp,
Cursor, or Claude Code.

## Hosts

Amp, Cursor, and Claude Code are the primary hosts. Skills ship as real copies (never symlinks) under
`.agents/skills/<verb>/SKILL.md` and `.claude/skills/<verb>/SKILL.md`, installed by `aayu install` (WP3.2).

## Branches

| Branch | Role |
|---|---|
| `dev` | Default. Nightly integration; every feature PR targets it and squash-merges when CI is green. |
| `rolling` | Canary snapshots promoted from `dev` on the release schedule. |
| `main` | Stable releases promoted from `rolling`; every commit is tagged `v*`. |
| `ledger` | Orphan branch holding this repository's own proof packs and action segments. |

See [CONTRIBUTING.md](CONTRIBUTING.md) for the PR flow and DCO sign-off, and [SECURITY.md](SECURITY.md) for
reporting vulnerabilities and the tool-incident plan.

Governance: [ETHOS.md](ETHOS.md) (principles), [GLOSSARY.md](GLOSSARY.md) (terms), [CONSTRAINTS.md](CONSTRAINTS.md)
(default floors, autonomy, budgets, and retention, generated), [LAYERS.md](LAYERS.md) (generated),
[AGENTS.md](AGENTS.md) (guidance for coding agents), and [docs/threat-model.md](docs/threat-model.md).

## Verbs

The table is generated from `internal/verbs/registry.go` by `go generate ./...`. Do not edit it by hand; CI
fails on drift.

<!-- BEGIN GENERATED: verbs -->

| Verb | Summary | Seat | Lane | Layers | Identity | Surfaces |
|---|---|---|---|---|---|---|
| `grill` | Interview a human until a work item of the given type is complete and passes validation, then write it as an issue. | Product Strategist | spec | all | bot | cli, skill, mcp |
| `slice` | Split a Feature into XS-L stories with observable criteria, dependencies, and a layer footprint; close XL items as split. | Product Strategist | plan | all | bot | cli, skill, mcp |
| `plan` | Plan a story: one checkpoint per layer, contracts, tests, instrumentation, and rollback. | Code Architect | plan | all | bot | cli, skill, mcp |
| `decide` | Record a Decision with options, evaluation, and recommendation; accept it, supersede it, or file a floor waiver. | Founder | spec | all | human | cli, skill |
| `triage` | Classify intake issues as Feedback, Bug, or Incident by label only; never edits bodies or touches non-intake issues. | Product Strategist | chore | none | bot | cli, skill, mcp |
| `build` | Build a Ready story or bug in a worktree with checkpoint commits per layer, verify each checkpoint, and open a PR. | AI Native Engineer (+SRE hat) | build | all | bot | cli, skill, mcp |
| `verify` | Run the deterministic floors (coverage, SAST, secrets, deps, a11y, perf) and record results in a proof pack. | AI Native Engineer (+SRE hat) | chore | all | bot | cli, skill, mcp |
| `inspect` | Adversarially review a PR against its plan, pack, and criteria; return a structured disposition: merge, fix, or rebuild. | Engineering Manager | review | all | bot | cli, skill, mcp |
| `ship` | Squash-merge a PR after a green merge gate and a human pass, write the Complete section, and clean up. | Engineering Manager | chore | all | human | cli, skill |
| `release` | Promote a Release along dev, rolling, main; stamp included stories released and trigger measurement. | DevOps & DevEx Tinkerer | chore | L09 | human | cli, skill |
| `proof` | Push, get, verify, list, or purge proof packs on the ledger branch. | Quality Architect | chore | all | bot | cli, skill, mcp |
| `ready` | Validate a work item for Ready, label it status:ready, and post the Ready snapshot; --check is read-only. | Product Strategist | chore | all | bot | cli, skill, mcp |
| `research` | Gather pinned design references (URLs and IDs only) and user research for a Feature. | UX Researcher & Evolutionist | spec | L01 | bot | cli, skill, mcp |
| `wireframe` | Produce wireframes for a Feature's states from its research and criteria. | UX Researcher & Evolutionist | build | L01 | bot | cli, skill, mcp |
| `mockup` | Produce design-system mockups for a Feature from its wireframes. | UX Researcher & Evolutionist | build | L01 | bot | cli, skill, mcp |
| `prototype` | Build a flagged, working prototype of a Feature with Code Connect mapping and a Figma screenshot diff. | UX Researcher & Evolutionist | build | L01, L02 | bot | cli, skill, mcp |
| `review` | Critique UI with a model lane and a deterministic lane; dispositions recapture, rebuild, fix, or ship. | UX Researcher & Evolutionist | ui-review | L01 | bot | cli, skill, mcp |
| `polish` | Apply a review critique to a PR's UI and re-run review. | UX Researcher & Evolutionist | build | L01 | bot | cli, skill, mcp |
| `baseline` | Capture visual baselines for a commit SHA. | UX Researcher & Evolutionist | chore | L01 | bot | cli, skill, mcp |
| `experiment` | Design an experiment for a Feature: hypothesis, metric, flag, and minimum sample size. | Revenue Architect | plan | L03, L08 | bot | cli, skill, mcp |
| `readout` | Read out an experiment's lift once it reaches minimum sample and write the Result. | Revenue Architect | spec | L08 | bot | cli, skill, mcp |
| `respond` | Respond to an incident: propose an ops catalog action, execute it only through human approval, keep the timeline. | AI Native Engineer (+SRE hat) | build | L07, L09 | human | cli, skill |
| `status` | Roll up status, health, risk, proof coverage, decisions, and cost per Goal, Initiative, and Feature. | Engineering Manager | chore | all | bot | cli, skill, mcp |
| `audit` | Audit floors, proof coverage, waivers, decision SLAs, autonomy exceptions, and unlogged writes; write a reproducible report. | Auditor | review | all | bot | cli, skill, mcp |
| `retro` | Compare estimates with observed delivery, corrections, floor misses, and cost; file findings as Feedback issues. | Engineering Manager | spec | all | bot | cli, skill, mcp |
| `measure` | Measure a Goal or Initiative against its baseline from analytics exports and write the Result. | Revenue Architect | chore | L08 | bot | cli, skill, mcp |
| `evolve` | Turn accepted retro Feedback into a failing eval case, then a skill diff that passes it, opened as an agent-authored PR. | AI Native Engineer (+SRE hat) | build | L05 | bot | cli, skill, mcp |
| `brief` | Explain work to an audience in a chosen register and purpose as a grounded, accessible HTML brief. | Mentalist | spec | all | bot | cli, skill, mcp |
| `tell` | Write docs, guides, changelogs, release notes, launch copy, or brand copy that cite only proven Features; publish is human-only. | Marketer & Documenter | spec | L01 | human | cli, skill, mcp |
| `init` | Set up a product repo: labels, issue types, forms, config, ledger branch, and optionally projects, gates, identity, integrations. | Founder | chore | L09 | human | cli |
| `install` | Install skill copies, lock.toml, MCP registration, and hooks for each host; shows a diff unless --apply. | DevOps & DevEx Tinkerer | chore | L09 | human | cli |
| `doctor` | Check tools, tokens, identity, config, host installs, and existing CI; --fix re-syncs copies after showing a diff. | DevOps & DevEx Tinkerer | chore | L09 | human | cli, skill, mcp |

<!-- END GENERATED: verbs -->

## Plumbing commands

<!-- BEGIN GENERATED: plumbing -->

| Command | Summary | Seat |
|---|---|---|
| `config` | Show, validate, or initialise layered configuration. | DevOps & DevEx Tinkerer |
| `auth` | Show the human token and bot identity aayudha resolves, with scope warnings; set, delete, or list named secrets. | DevOps & DevEx Tinkerer |
| `actions` | Tail, search, or verify the hash chain of the actions log. | DevOps & DevEx Tinkerer |
| `models` | Show the resolved model for each lane and host. | DevOps & DevEx Tinkerer |
| `mcp` | Serve verbs as MCP tools over stdio. | DevOps & DevEx Tinkerer |
| `self-update` | Replace the binary with a signed, newer release; refuses downgrades and agents. | DevOps & DevEx Tinkerer |
| `upgrade` | Migrate repo config, lock, state, and skill copies to this binary's schema versions. | DevOps & DevEx Tinkerer |
| `version` | Print the binary version. | DevOps & DevEx Tinkerer |
| `halt` | Stop every agent lane, loop, and workflow in the repo until resume. | Founder |
| `resume` | Clear a halt. | Founder |
| `gc` | Prune stale worktrees and leases. | DevOps & DevEx Tinkerer |
| `uninit` | Remove what init created, by stored IDs. | DevOps & DevEx Tinkerer |
| `hook` | Entry point for host hooks; hooks call only `aayu hook <name>`. | DevOps & DevEx Tinkerer |
| `seat` | Add, remove, or list seats and their paired humans. | Founder |
| `lfs-agent` | git-lfs custom transfer agent for S3/R2 storage. | DevOps & DevEx Tinkerer |

<!-- END GENERATED: plumbing -->

## Licence

BSD-3-Clause. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
