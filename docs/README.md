# Project reference

The reference for people who work on aayudha itself, or want every detail in one place. New here? Start with the
[README](../README.md) or [your first hour](guide/start.md).

## Documents

- Scope and sequencing: [BUILD-PLAN.md](BUILD-PLAN.md). Its Appendix B decision log up to 2026-10-17 is archived
  privately; deviations since are in pull request descriptions ("Deviation from the plan:"). Principles: [ETHOS.md](../ETHOS.md). Terms:
  [GLOSSARY.md](../GLOSSARY.md). Threats: [threat-model.md](threat-model.md).
- Generated: [CONSTRAINTS.md](../CONSTRAINTS.md) (default floors, autonomy, budgets, and retention, from
  `internal/config/defaults.toml`), [LAYERS.md](../LAYERS.md), and [RACI.md](RACI.md) (from the verb registry).
- Install, hosts, and clouds: [getting-started.md](getting-started.md), with
  [Azure](getting-started-azure.md), [AWS](getting-started-aws.md), and [Google Cloud](getting-started-gcp.md).
- Coding agents: [AGENTS.md](../AGENTS.md). Contributors: [CONTRIBUTING.md](../CONTRIBUTING.md). Vulnerabilities:
  [SECURITY.md](../SECURITY.md). Research notes live in the same private repository, `rahulnakmol/agentic-ayudha-research`.
- The guide, which [the docs site](https://rahulnakmol.github.io/agentic-ayudha/) is built from by
  `aayu tell docs --site`: [guide/](guide/README.md).

## Hosts

Claude Code, Codex, Cursor, and Amp are the hosts. `aayu install` writes each verb's skill as a real copy (never a
symlink) under `.agents/skills/<verb>/SKILL.md` and `.claude/skills/<verb>/SKILL.md`, with the MCP server and hooks.
`aayu install --env` sets up and checks each host's cloud agent environment, and `aayu install --loops` sets up the
scheduled and event loops in each host's own tools. [Your agent](guide/hosts/README.md) covers each host.

## Branches

| Branch | Role |
|---|---|
| `dev` | Default. Integration; every feature pull request targets it and squash-merges when CI is green. |
| `rolling` | Snapshots promoted from `dev` on the release schedule, tagged `rolling-<date>`. |
| `main` | Stable releases promoted from `rolling`; every commit is tagged `v*`. |
| `ledger` | Orphan branch holding this repository's records: Ready snapshots, plans, proof, and the actions log. |

## Verbs

The tables below are generated from `internal/verbs/registry.go` by `go generate ./...`. Do not edit them by hand;
CI fails on drift. The docs site's [verbs reference](https://rahulnakmol.github.io/agentic-ayudha/verbs.html) has
every input of every verb.

<!-- BEGIN GENERATED: verbs -->

| Verb | Summary | Seat | Lane | Layers | Identity | Surfaces |
|---|---|---|---|---|---|---|
| `grill` | Interview a human until a work item of the given type is complete and passes validation, then write it as an issue. | Product Strategist | spec | all | bot | cli, skill, mcp |
| `slice` | Split a Feature into XS-L stories with observable criteria, dependencies, and a layer footprint; close XL items as split. | Product Strategist | plan | all | bot | cli, skill, mcp |
| `plan` | Plan a story: one checkpoint per layer, contracts, tests, instrumentation, and rollback. | Code Architect | plan | all | bot | cli, skill, mcp |
| `decide` | Record a Decision with options, evaluation, and recommendation; accept it, supersede it, or file a floor waiver. | Founder | spec | all | human | cli, skill, mcp |
| `counsel` | Consult the Founder's Sage on any topic, issue, Decision, path, or PR: the call and its door, three readings, one recommendation, cited proof, and the next aayu step; advice only, never a review. | Founder | spec | all | bot | cli, skill, mcp |
| `triage` | Classify intake issues as Feedback, Bug, or Incident by label only; never edits bodies or touches non-intake issues. | Product Strategist | chore | none | bot | cli, skill, mcp |
| `brain` | Keep the product brain: an Obsidian vault on the orphan brain branch; init, checkpoint its generated links, check, or open it. | Product Strategist | chore | none | bot | cli, skill, mcp |
| `build` | Build a Ready story or bug in a worktree with checkpoint commits per layer, verify each checkpoint, and open a PR. | AI Native Engineer | build | all | bot | cli, skill, mcp |
| `verify` | Run the deterministic floors (coverage, SAST, secrets, deps, a11y, perf) and record results in a proof pack. | AI Native Engineer | chore | all | bot | cli, skill, mcp |
| `inspect` | Adversarially review a PR against its plan, pack, and criteria; return a structured disposition: merge, fix, or rebuild. | Engineering Manager | review | all | bot | cli, skill, mcp |
| `ship` | Squash-merge a PR after a green merge gate and a human pass, write the Complete section, and clean up. | Engineering Manager | chore | all | human | cli, skill |
| `release` | Promote a Release along dev, rolling, main; stamp included stories released and trigger measurement. | DevOps & DevEx Tinkerer | chore | L09 | human | cli, skill, mcp |
| `proof` | Push, get, verify, or list proof packs on the ledger branch; land a merged PR's gate pack; comment is the merge gate's pack check. | Quality Architect | chore | all | bot | cli, skill, mcp |
| `ready` | Validate a work item for Ready, label it status:ready, and post the Ready snapshot; --check is read-only. | Product Strategist | chore | all | bot | cli, skill, mcp |
| `mission` | Start a Feature's approved mission, show its plan and clock, list what can start now, gate each milestone, stop, resume, end, extend, recover, retry, or wake it, or run the hourly tick. | Engineering Manager | mission | all | human | cli, skill, mcp |
| `research` | Gather pinned design references (URLs and IDs only) and user research for a Feature. | UX Researcher & Evolutionist | spec | L01 | bot | cli, skill, mcp |
| `wireframe` | Produce wireframes for a Feature's states from its research and criteria. | UX Researcher & Evolutionist | build | L01 | bot | cli, skill, mcp |
| `mockup` | Produce design-system mockups for a Feature from its wireframes. | UX Researcher & Evolutionist | build | L01 | bot | cli, skill, mcp |
| `prototype` | Build a flagged, working prototype of a Feature with Code Connect mapping and a Figma screenshot diff. | UX Researcher & Evolutionist | build | L01, L02 | bot | cli, skill, mcp |
| `review` | Critique UI with a model lane and a deterministic lane; dispositions recapture, rebuild, fix, or ship. | UX Researcher & Evolutionist | ui-review | L01 | bot | cli, skill, mcp |
| `polish` | Apply a review critique to a PR's UI and re-run review. | UX Researcher & Evolutionist | build | L01 | bot | cli, skill, mcp |
| `baseline` | Capture visual baselines for a commit SHA. | UX Researcher & Evolutionist | chore | L01 | bot | cli, skill, mcp |
| `experiment` | Design an experiment for a Feature: hypothesis, metric, flag, and minimum sample size. | Revenue Architect | plan | L03, L08 | bot | cli, skill, mcp |
| `readout` | Read out an experiment's lift once it reaches minimum sample and write the Result. | Revenue Architect | spec | L08 | bot | cli, skill, mcp |
| `respond` | Respond to an incident: propose an ops catalog action, execute it only through human approval, keep the timeline. | Production Engineer | build | L07, L09 | human | cli, skill |
| `status` | Roll up status, health, risk, proof coverage, decisions, and cost per Goal, Initiative, and Feature. | Engineering Manager | chore | all | bot | cli, skill, mcp |
| `audit` | Audit floors, proof coverage, waivers, decision SLAs, autonomy exceptions, and unlogged writes; write a reproducible report. | Auditor | review | all | bot | cli, skill, mcp |
| `retro` | Compare estimates with observed delivery, corrections, floor misses, and cost; file findings as Feedback issues. | Engineering Manager | spec | all | bot | cli, skill, mcp |
| `measure` | Measure a Goal or Initiative against its baseline from analytics exports or live cloud pulls and write the Result. | Data & AI Architect | spec | L08 | bot | cli, skill, mcp |
| `evolve` | Turn accepted retro Feedback into a failing eval case, then a skill diff that passes it, opened as an agent-authored PR. | AI Native Engineer | build | L05 | bot | cli, skill, mcp |
| `brief` | Explain work to an audience in a chosen register and purpose as a grounded, accessible HTML brief. | Mentalist | spec | all | bot | cli, skill, mcp |
| `tell` | Write docs, guides, changelogs, release notes, launch copy, or brand copy that cite only proven Features; publish is human-only. | Marketer & Documenter | spec | L01 | human | cli, skill, mcp |
| `init` | Set up a product repo: labels, issue types, forms, config, ledger branch, and optionally projects, gates, identity, integrations. | Founder | chore | L09 | human | cli |
| `install` | Install skill copies, lock.toml, MCP registration, and hooks for each host; shows a diff unless --apply. | DevOps & DevEx Tinkerer | chore | L09 | human | cli |
| `doctor` | Check tools, tokens, identity, config, host installs, and existing CI; --fix re-syncs copies after showing a diff. | DevOps & DevEx Tinkerer | chore | L09 | human | cli, skill, mcp |
| `chart` | Chart an issue's data or AI design before planning: decisions, metrics, sources, platform, and retention; or evidence, permissions, abstention, and evals. | Data & AI Architect | spec | L05, L07, L08 | bot | cli, skill, mcp |
| `survey` | Survey a codebase with no product docs: index it, check an evidence-backed analysis and backlog, and capture golden outputs for parity. | Code Architect | plan | all | bot | cli, skill, mcp |
| `investigate` | Investigate an incident from bounded telemetry queries to the deployed revision and the implicated code, and check the note; it proposes a mitigation, never runs one. | Production Engineer | spec | all | bot | cli, skill, mcp |
| `slo` | Define SLOs and error budgets from the user's side, measure them from a sanitised export, and record the budget that gates release. | Production Engineer | spec | all | bot | cli, skill, mcp |
| `runbook` | Write and check runbooks: confirm a known failure with a bounded query, mitigate it only through the ops catalog, verify it, and roll it back. | Production Engineer | spec | all | bot | cli, skill, mcp |
| `scope` | Plan a release's scope in its version milestone, draft it, have a person baseline it, hand it off, assign Feedback, and hold drift to the change budget. | Product Strategist | plan | all | bot | cli, skill, mcp |

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
| `next` | Print the single best next step for this repository: the exact command to run and why. | Engineering Manager |
| `tour` | Guided first run: check this machine, then set the repository up at Level 1 in a few prompts; refused for agents. | Founder |
| `inbox` | Every human-only step waiting for you, one card at a time with the twin's counsel; Enter takes the recommendation and C clears the housekeeping. Refused for agents and needs a terminal, except --list. | Founder |
| `completion` | Print the shell completion script, generated from the registry, for bash, zsh, fish, or PowerShell. | DevOps & DevEx Tinkerer |

<!-- END GENERATED: plumbing -->
