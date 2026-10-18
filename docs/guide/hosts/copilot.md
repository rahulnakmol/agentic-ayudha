# aayudha in GitHub Copilot

If Copilot is your team's only AI tool, aayudha runs inside it. Copilot CLI and VS Code agent mode read aayudha's
skills, hooks, and MCP server from the repository, so the whole flow, from a story to a proven pull request, runs
where you already work.

> **Honest status:** Copilot CLI 1.0.98 ran all of this against a real Copilot Business subscription on 17 October
> 2026: the skills, the `aayudha` MCP tool, both hooks, one call at each Auto tier, and one HydraFusion Critique turn.
> The `copilot-live` workflow repeats the core of it in CI after each change to the Copilot host: a pinned Copilot
> CLI, in a folder it trusts, calls the `aayudha` MCP server's `doctor` tool, and the pre-tool hook stops `aayu ship`.
> Each run costs one premium request. VS Code agent mode reads the same files, but nothing has run aayudha in it
> yet, by hand or in CI, because it has no headless mode. Use Copilot CLI if you need what is proven.

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
Workspace Trust, which aayu cannot read, so `aayu doctor` asks you to check it when Copilot CLI is missing or when you
run doctor in VS Code's terminal.

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
account from `copilot login`. aayu never writes to GitHub with `COPILOT_GITHUB_TOKEN`: `aayu doctor` sends it only to
Copilot's own models endpoint, to list the models Copilot enables. So when Copilot runs on a different account from
the one aayu writes with, set that account's token as `COPILOT_GITHUB_TOKEN` and nothing else. If your shell
exports `GH_TOKEN`, Copilot uses it ahead of your `copilot login`.

`aayu doctor` says which sign-in Copilot uses, and fails if aayu's token is Copilot's own. You should see the `token`
row name the account aayu writes with, and the `copilot` row say
`Copilot signs in with COPILOT_GITHUB_TOKEN; aayu's GitHub writes never use it`.

## Lanes and models

The build, spec, plan, and review lanes name a model, so you know what a call costs before you make it, and a review
is from a family you can prove. Only the chore lane uses an Auto tier. Start a session with `copilot --model <id>`, or
pick the model with `/model` in Copilot CLI or in VS Code's model picker:

| Lane | On Copilot | Why |
|---|---|---|
| chore (triage, status, checks) | Auto, Efficiency | Cost first |
| build, spec, plan | `claude-sonnet-5.5`, then `claude-opus-5.5` when a step needs more | A named model; Opus costs twice Sonnet per token |
| review | `gpt-6.1-sol` on an Anthropic build, `claude-opus-5.5` on one with no Anthropic response, `gemini-4-argon` on one that used both | A different family from the builder's |
| ui-review | `gemini-3.8-flash` at high effort, the same as every host | The founder's default for all UI review |

`aayu models show --host copilot` lists them. Copilot's paid plans take 10 % off a request answered through Auto.

Auto picks a model for each prompt from the models your organisation enables, so its cost is known only afterwards.
On 18 October 2026, with a hard reasoning-and-code prompt, Efficiency chose `gpt-6-luna` (1 premium request, 0.21 AI
credits). Balance and Intelligence both chose `claude-opus-5.5` (15 premium requests and about 20 AI credits, about
0.20 USD, each). That is why the build lane names `claude-sonnet-5.5` instead. An earlier, simpler prompt ran on
`gpt-6-luna` at every tier: that was Auto's choice for the prompt, not a limit of the account. Each figure is from
Copilot CLI's own usage report for that run.

**Gemini 4 Argon for UI review, opt-in.** Copilot lists `gemini-4-argon`, so the UI-review lane can use it without a
Google plan. It stays opt-in until a gating run on it passes and the founder agrees:

```toml
[models.ui-review.copilot]
model = "gemini-4-argon"
```

> **Honest status:** the setting is not wired yet. aayu's UI-review calls still run `gemini-3.8-flash` through the
> AI gateway, whatever this row says, until the route that sends them through `copilot -p` ships.

One UI-review call on it cost about 0.043 USD: Copilot charges AI credits for input at 2 USD per million tokens and
visible output at 10 USD per million, and does not bill reasoning tokens. About 15,000 of a call's 19,000 input tokens
are Copilot CLI's own prompt. The figure is from Copilot CLI's usage report for two such calls on 18 October 2026.

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

## Review from another family

Copilot gives you two ways to review with a different model family from the builder's:

1. **Name the model.** The review lane picks a family the builder was not, from the builder's `Aayudha-Model` lines:
   `gpt-6.1-sol` reviews an Anthropic build, `claude-opus-5.5` an OpenAI one, and `gemini-4-argon` a build that used
   both. A build that used all three families has no other family left, so its review is labelled same-family. Run
   the review with that model, for example `copilot --model claude-opus-5.5`. A single Copilot plan covers every
   reviewer when your organisation enables all three models. `aayu doctor`'s `copilot` row lists the families Copilot
   enables for you and says which builds it can review in another family. It asks Copilot only when Copilot signs in
   to github.com with a token from the environment (`COPILOT_GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_TOKEN`). To pin
   another reviewer, set it in `.aayudha/config.toml`:

   ```toml
   [models.review.copilot]
   model = "claude-haiku-5.5"
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

> **Honest status:** the review lane names its model on Copilot, and `aayu models show --host copilot` shows it. The
> merge gate takes its review from a reviewer app or from a person who is not the author, so a review you run in
> your own Copilot session does not count towards autonomy yet. `aayu inspect` reads every `Aayudha-Model` line,
> but it still runs its own reviewer and does not yet read `Aayudha-Critic-Model`, so a Critique review does not
> count either.
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

aayudha's guard that lets a host's own PostHog or Sentry MCP server or CLI only read covers Copilot: the `preToolUse`
hook reads Copilot's `toolName` and `toolArgs`, splits an MCP tool's `<server>-<tool>` name at the server's name in
`.mcp.json` or `mcp-config.json` in Copilot's home (`$COPILOT_HOME`, else `~/.copilot`), and checks `bash` commands as
it does on the other hosts. `aayu install` also registers the post-tool hook for Copilot's `postToolUse` and
`postToolUseFailure` events, so each call it lets through is logged as it is on the other hosts.

Copilot does not run aayudha's loops either: `aayu install --loops` covers Amp, Cursor, Claude Code, Codex, and GitHub
Actions.
