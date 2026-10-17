# loops

Loops (WP7.1), one file each: `loops/<name>.toml`. A loop runs on a schedule (`cron`). Event loops (`on`, `filter`) are
deferred to a later version (Appendix B S-7.1e): `loops.Parse` refuses them.

```toml
description = "Founder digest: the status rollup of every open Goal, paged through [notify]."
command = ["status", "--digest"]   # the aayu command; never --yes or a human-only flag
cron = "5 9 * * 1-5"               # five fields in [time] tz: one minute; hours as a list or *; days of month or week
agent = false                      # needs no model and runs with the workflow token: GitHub Actions runs it
hosts = ["amp", "cursor"]          # optional; default amp, cursor, claude, codex (agent = false: actions)
prompt = "…"                       # optional, agent loops: what the tool's agent does with the output
bot = false                        # true when the command writes as the bot: an agent loop (the workflow token is not)
setup = ["node", "pnpm"]           # optional, agent = false only: pinned tool setups (node, pnpm, bun), once due
setup_run = ["pnpm install --frozen-lockfile"]  # optional, agent = false only: the product's own commands, after setup
```

**Agent loops and no-AI loops (Appendix B D-7.1f).** A loop is an agent loop unless it sets `agent = false`: its
command needs an agent's judgement, the bot App, or a personal token, so it runs in the one agent tool you pick
(`aayu install --loops`) and never in Actions. A loop with `agent = false` is deterministic and runs with the
workflow token, so it runs only in the free GitHub Actions fallback, and no tool is set up for it. `loops.Parse`
refuses a mix: `actions` or `setup` on an agent loop, and an agent tool, `bot = true`, or a `prompt` on an
`agent = false` loop. Here `digest` and `verify-sweep` have `agent = false`; every other loop is an agent loop.

This repository's loops are also the defaults a product gets: `aayu install --loops` in a repository with no
`loops/*.toml` writes them and renders them for every tool (Appendix B D-7.1g). This repository's own `setup` and
`setup_run` are replaced by the product's, from its lockfile (D-7.1h): `pnpm-lock.yaml` sets up node and pnpm, `bun.lock`
or `bun.lockb` sets up Bun, both set up all three and install with pnpm, and `package-lock.json` sets up node and runs
`npm ci`. Each then installs the Playwright browser through the same package manager; with no lockfile there is no
setup. `triage-daily` (each weekday morning) replaces the
retired aayudha-triage workflow and the weekly `triage` loop: `aayu triage` lists every open intake issue with no
model, and the agent in your own session classifies each one with `--as`. `housekeeping` (weekly) runs `aayu gc`, then reports doctor drift,
feature-map drift, and stale Ready issues. `ci-failed` (each weekday morning) sends a delegated story's failed
required check back to its builder (#231), `release-measure` (weekly) runs `measure` when a release was published
that week, and `evolve` (monthly) takes the Feedback filed since. They do on a schedule what the deferred event loops
did on arrival.

The Production Engineer's loops (WP5.10, Decision #324) are agent loops, so telemetry and its credentials stay in your
own session: `alert-triage` (twice each weekday) reports fast-burning SLOs and open incidents with no investigation
note, `error-budget-review` (weekly) measures every SLO from a fresh sanitised export and records the budget that
`release` reads, and `patch-sweep` (weekly) runs the deps floor and files a Bug for each fixable advisory. None of them
runs a catalog action: a person approves every one through `aayu respond`.

`verify-sweep` (nightly) and `verify-maintain` (weekly) are the feature maps' loops (WP4.5b.4). The sweep runs every
behaviour check with no model in Actions, and its run is red when a check fails. The maintain pass is an agent loop:
it hands the maintain brief to the agent app `[verify] maintainer` names. No model key is used in the Actions jobs.

## The user-side schedule for the maintain hand-off

The agent apps answer only the person who connected them, so in Actions `verify --maintain` would only **record**
the brief (no mention; D-4.5b.7). That is why `verify-maintain` is an agent loop: a run that executes it as you posts
the brief with `[verify] maintainer`'s mention (`@codex`, `@cursor`, `@claude`), or, for `amp`, follows the brief
itself. Besides `aayu install --loops`, you can set it up by hand, in a checkout of the repository:

- **Headless Amp (`amp -x`).** `loops/amp/<loop>.json` holds the prompt and the schedule (an RFC 5545 rule in
  `[time] tz`). Run the prompt on that schedule with your scheduler, for example cron on a machine that is signed in
  to Amp and to GitHub (`gh auth status`) and has `aayu` on `PATH`:

  ```sh
  # crontab -e; the time is the rule in loops/amp/verify-maintain.json, in your zone
  0 9 * * 1  cd ~/src/<repo> && git pull -q --ff-only && amp -x "$(jq -r .prompt loops/amp/verify-maintain.json)"
  ```

  A headless run cannot answer an approval prompt: allow the commands the prompt runs (`aayu`, `git`, `gh`) in your
  Amp permissions first. The prompt starts with `aayu hook loop:<name>`, so a halted repository or a paused budget
  stops the run before any write. The same JSON is also the input of Amp's `set_schedule`, if you schedule an Amp
  thread in the project instead.
- **A Cursor automation.** `loops/cursor/aayudha-loops.tf` declares the loop as a `cursor_platform_workflow`
  resource that runs as you: `terraform apply` with `CURSOR_TOKEN` in the environment.

This repository's `[verify] maintainer` is `amp`: the headless Amp run reads the brief and opens the one PR.

`go generate ./...` renders aayudha's own loops; `aayu init --gates` renders a product's:

- `.github/workflows/aayudha-loops.yml`: the GitHub Actions fallback, with a job for each `agent = false` loop only.
  Actions cron is UTC, so each loop has one cron per UTC offset of the zone, and the guard runs only the one for
  today's offset.
- `loops/amp/<name>.json`: the input of Amp's `set_schedule` (an RFC 5545 rule in the zone, `run_mode` fresh).
- `loops/cursor/aayudha-loops.tf`: Cursor Automations as `cursor_platform_workflow` resources of Cursor's Terraform
  provider; apply them with `terraform apply` and `CURSOR_TOKEN`. `loops/cursor/automate.md` has the same loops as
  prompts for Cursor's `/automate` skill, for a user without Terraform or a key.
- `loops/claude/routines.md`: one `/schedule` prompt per loop for Claude Code (v2.1.225 or later, claude.ai login),
  which saves it as a routine.
- `loops/codex/scheduled-tasks.md`: one prompt per loop for a Codex or ChatGPT chat in the desktop app, which creates
  the scheduled task, with each loop's RFC 5545 rule.

## Setting the loops up in one tool

`aayu install --loops` is the one entry point, per repository. One tool runs every agent loop: two would run each
loop twice. It sets up no `agent = false` loop, and says that Actions runs those. It asks which tool should run them, offering only those whose CLI it detects here (amp, cursor-agent, claude,
codex), or takes `--host`, and warns when `loops/active.json` records loops in another tool. Then:

- Amp: with `--apply`, one headless Amp thread per loop in an orb of the project (`amp -ox`) calls `set_schedule` with
  the rendered input and keeps that schedule (one schedule per thread); each loop is recorded.
- Cursor: with `--apply`, `terraform apply` of `loops/cursor/aayudha-loops.tf` when `terraform` and `CURSOR_TOKEN` are
  there, recording each loop; without them, the `/automate` prompts in `loops/cursor/automate.md`.
- Claude Code and Codex: the prompts in `loops/claude/routines.md` and `loops/codex/scheduled-tasks.md` to paste.

`--run-now <loop>` tests one loop once with the tool's own CLI here (`amp -x`, `cursor-agent -p`, `claude -p`,
`codex exec`); at a terminal it is offered after setup. Record each loop you set up by hand with
`aayu install --loops --host <tool> --record <loop>=<ID>` (or `all=<ID>`; an empty ID forgets one); the record is
`loops/active.json`, committed with the loops. `aayu doctor` then shows each loop with what runs it, its tool or
`actions`: active, stale (the loop changed after it was set up), missing, or double (recorded in two tools, or an
`agent = false` loop still recorded in a tool).

Every run starts with `aayu hook loop:<name>`, in every tool: it exits 4 while the
repository is halted (`aayu halt`), 6 while the `[budget]` breaker has paused agent lanes or the Actions loops have used
`[budget] ci_minutes_month` this month, and otherwise says whether this run is the scheduled one. A loop whose command is not implemented yet is not rendered (n/a until its WP).
