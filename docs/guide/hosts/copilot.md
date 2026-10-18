# aayudha in GitHub Copilot

If Copilot is your team's only AI tool, aayudha runs inside it. Copilot CLI and VS Code agent mode read aayudha's
skills, hooks, and MCP server from the repository, so the whole flow, from a story to a proven pull request, runs
where you already work.

> **Honest status:** Copilot CLI 1.0.98 ran all of this against a real Copilot Business subscription on 17 October
> 2026: the skills, the `aayudha` MCP tool, both hooks, one call at each Auto tier, and one HydraFusion Critique turn.
> VS Code agent mode reads the same files, but no one has run aayudha in it yet. Use Copilot CLI if you need what is
> proven.

## Before you start

Install aayu and Copilot CLI, sign in to Copilot, and check both are on your `PATH` (see
[Your first hour](../start.md)):

```sh
aayu version
copilot --version
```

You should see `GitHub Copilot CLI 1.0.98.` or later.

## Install into the repository

```sh
aayu install --host copilot --apply
```

This writes:

| File | What it holds |
|---|---|
| `.agents/skills/<verb>/` | One skill per verb |
| `.github/skills/aayudha-<name>/` | The home and next commands and the hand-offs, as `/aayudha-home`, `/aayudha-next`, … |
| `AGENTS.md` | The aayudha sections, which Copilot reads as project guidance |
| `.mcp.json` | The `aayudha` MCP server, with the variables it needs to find your GitHub token |
| `.github/hooks/aayudha.json` | The `sessionStart` and `preToolUse` hooks, for bash and PowerShell |

You should see each skill when you list them:

```sh
copilot skill list
```

## Trust the folder

Copilot loads a repository's MCP server and hooks only after you trust the folder. Start `copilot` in the repository
once and answer yes. `copilot -p` never asks, so do this before you run it from a script. VS Code asks through
Workspace Trust, which aayu cannot read, so without Copilot CLI `aayu doctor` keeps asking you to check it.

Trust covers the folder, not each file: a trusted folder's `.mcp.json` and `.github/hooks` run with your environment,
including your GitHub token. Read changes to them before you check out someone else's branch in a trusted folder.

Until you trust it, nothing stops an agent from running a step that belongs to a person. `aayu doctor` shows the step:

```text
WARN  copilot  GitHub Copilot needs a person's step; the lines say which
                 trust this folder in Copilot CLI: start `copilot` here once and answer yes; until then it loads neither the aayudha MCP server nor the hooks
```

Then check:

```sh
copilot mcp get aayudha
aayu doctor
```

You should see `PASS  copilot  GitHub Copilot has the aayudha hooks and trusts this folder`.

## Keep Copilot's account apart from aayu's

Copilot CLI signs in with `COPILOT_GITHUB_TOKEN` first, then `GH_TOKEN`, then `GITHUB_TOKEN`, and only then with the
account from `copilot login`. aayu never reads `COPILOT_GITHUB_TOKEN`. So when Copilot runs on a different account
from the one aayu writes with, set that account's token as `COPILOT_GITHUB_TOKEN` and nothing else. If your shell
exports `GH_TOKEN`, Copilot uses it ahead of your `copilot login`.

`aayu doctor` says which sign-in Copilot uses, and fails if aayu's token is Copilot's own. You should see the `token`
row name the account aayu writes with, and the `copilot` row say
`Copilot signs in with COPILOT_GITHUB_TOKEN; aayu's GitHub writes never use it`.

## Lanes and Auto tiers

aayu cannot choose the model inside Copilot. Each lane maps to an Auto tier instead. Select it with `/model` in
Copilot CLI (choose `auto`), start it with `copilot --model auto --auto-tier <tier>`, or pick Auto in VS Code's model
picker:

| Lane | Auto tier | Why |
|---|---|---|
| chore (triage, status, checks) | Efficiency | Cost first |
| build, spec, plan | Balance, then Intelligence when a step needs more | Cost, quality, and speed together |
| review, ui-review | Intelligence | Quality first |

`aayu models show --host copilot` lists them. Copilot's paid plans take 10 % off a request answered through Auto.

> **Honest status:** on the account that proved this, Auto offered one model, `gpt-6-luna`, at every tier, so the
> three tiers ran the same model. Which models Auto chooses from is your organisation's Copilot settings.

## Which model built it

Copilot reports the model behind each response. When `aayu build` runs in a Copilot session, its `preToolUse` hook
notes the session, and `build` reads the session's log (`~/.copilot/session-state/<session>/events.jsonl`). Each
checkpoint commit then carries:

```text
Aayudha-Host: copilot
Aayudha-Model: claude-opus-5.5
Aayudha-Model: gpt-6-luna
```

There is one `Aayudha-Model` line for each model that answered, most recent first. `aayu inspect` uses them to make sure
the reviewer's model family differs from the builder's. If no model is recorded, the builder's family is unknown, and the
review counts as a same-family review, which never raises how much aayu does on its own. A pull request opened outside
`build` declares the same lines in its body.

> **Honest status:** `aayu inspect` does not read every `Aayudha-Model` line yet; that change is in progress. Until it
> lands, when Copilot reports models of two families, `build` leaves the model out of its ledger record, so inspect
> treats the builder as unknown instead of trusting one family.

## Review from another family

Copilot gives you two ways to review with a different model family from the builder's:

1. **Pick the model.** In Copilot's model picker, choose a model from a family other than the builder's. If the builder
   was `gpt-6-luna`, `claude-haiku-4.5` is a different family. To make it the default, add this to
   `.aayudha/config.toml`:

   ```toml
   [models.review.copilot]
   model = "claude-haiku-4.5"
   ```

2. **HydraFusion Critique.** HydraFusion is a research preview in Copilot's model picker. For each prompt it picks a
   workflow, and in Critique a read-only critic from another model family reviews the draft once. It is never the
   default. Opt in with:

   ```toml
   [models.review.copilot]
   model = "hydrafusion"
   ```

   Then run Copilot CLI with `copilot --experimental --model hydrafusion`. In VS Code 1.140 or later, set
   `chat.copilot.hydraFusion.enabled`. On Business and Enterprise plans, an admin must turn on preview features.

`build` records a Critique turn's critic as `Aayudha-Critic-Model: <model>`. It counts as a different-family review only
when the critic's family is outside every builder family on the pull request.

> **Honest status:** today `[models.review.copilot]` sets the reviewer you pick inside Copilot, and
> `aayu models show --host copilot` shows it. `aayu inspect` still runs its own reviewer and does not yet read
> `Aayudha-Critic-Model`, so a Critique review does not count towards autonomy until that inspect change lands.
>
> One trivial Critique turn cost 16 premium requests, against 1 for an Auto call. HydraFusion is
> billed at each model's standard rate, gets no Auto discount, and has no SLA. You cannot choose its models: on the
> proving run it drafted with `claude-opus-5.5` and critiqued with `gpt-6-luna`.

## Human-only steps

Shipping, releasing, accepting a Decision, and responding to an incident are for a person. The MCP server never offers
them. The `preToolUse` hook stops Copilot if it tries to run them in a shell and gives you the exact command to run:

```text
Denied by preToolUse hook: aayudha: aayu ship is for a person, not an agent (…); ask them to run `aayu ship 12` in their own terminal.
```

## Not built yet

Two parts of Copilot are not wired to aayudha yet:

- **The cloud agent.** A hand-off would assign the story to `copilot-swe-agent[bot]`, create a task with
  `gh agent-task create`, or send a session there with `/delegate` in Copilot CLI. The repository would need a `copilot-setup-steps.yml` workflow that runs `.agents/setup`, and
  the `aayudha` MCP server in the repository's Copilot settings, with its token as a `COPILOT_MCP_` secret.
- **Copilot code review.** Requesting `copilot-pull-request-reviewer[bot]` on a pull request would make it a reviewer,
  once `inspect` can record which model family it used.

aayudha's guard that lets a host's own PostHog or Sentry MCP server or CLI only read does not cover Copilot yet:
it reads the hook format of Claude Code, Codex, and Cursor, and Copilot sends `toolName` and `toolArgs` instead.

Copilot does not run aayudha's loops either: `aayu install --loops` covers Amp, Cursor, Claude Code, Codex, and GitHub
Actions.
