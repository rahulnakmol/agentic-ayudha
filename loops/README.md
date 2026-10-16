# loops

Loops (WP7.1), one file each: `loops/<name>.toml`. A loop runs on a schedule (`cron`). Event loops (`on`, `filter`) are
deferred to a later version (Appendix B S-7.1e): `loops.Parse` refuses them.

```toml
description = "Founder digest: the status rollup of every open Goal, paged through [notify]."
command = ["status", "--digest"]   # the aayu command; never --yes or a human-only flag
cron = "5 9 * * 1-5"               # five fields in [time] tz: one minute; hours as a list or *; days of month or week
hosts = ["amp", "cursor", "actions"]  # optional; default all three
prompt = "…"                       # optional: what the host's agent does with the output
bot = false                        # true when the command writes as the bot (the Actions job then needs the App)
setup = ["node", "pnpm"]           # optional, Actions only: pinned tool setups, run once the guard says the loop is due
setup_run = ["pnpm install --frozen-lockfile"]  # optional, Actions only: the product's own commands, after setup
```

This repository's loops are also the defaults a product copies. `triage-daily` (every morning) and `triage` (weekly)
replace the retired aayudha-triage workflow: `aayu triage` lists the intake issues with no model, and the agent in
your own session classifies each one with `--as`. `housekeeping` (weekly) runs `aayu gc`, then reports doctor drift,
feature-map drift, and stale Ready issues. `ci-failed` (three times each working day) sends a delegated story's failed
required check back to its builder (#231), `release-measure` (weekly) runs `measure` when a release was published
that week, and `evolve` (monthly) takes the Feedback filed since. They do on a schedule what the deferred event loops
did on arrival.

`verify-sweep` (nightly) and `verify-maintain` (weekly) are the feature maps' loops (WP4.5b.4): the sweep runs every
behaviour check with no model, and both hand the maintain brief to the agent app `[verify] maintainer` names, the
sweep only when a map drifted or a check failed. No model key is used in the Actions jobs.

## The user-side schedule for the maintain hand-off

An Actions job writes as the bot, and the agent apps answer only the person who connected them, so in Actions
`verify --maintain` only **records** the brief (no mention; D-4.5b.7). The agent is woken from your side: a run that
executes the same loop as you posts the brief with `[verify] maintainer`'s mention (`@codex`, `@cursor`, `@claude`),
or, for `amp`, follows the brief itself. Set up one of these per repository, in a checkout of it:

- **Headless Amp (`amp -x`).** `loops/amp/<loop>.json` holds the prompt and the schedule (an RFC 5545 rule in
  `[time] tz`). Run the prompt on that schedule with your scheduler, for example cron on a machine that is signed in
  to Amp and to GitHub (`gh auth status`) and has `aayu` on `PATH`:

  ```sh
  # crontab -e; the times are the rules in loops/amp/verify-sweep.json and verify-maintain.json, in your zone
  30 2 * * * cd ~/src/<repo> && git pull -q --ff-only && amp -x "$(jq -r .prompt loops/amp/verify-sweep.json)"
  0 9 * * 1  cd ~/src/<repo> && git pull -q --ff-only && amp -x "$(jq -r .prompt loops/amp/verify-maintain.json)"
  ```

  A headless run cannot answer an approval prompt: allow the commands the prompt runs (`aayu`, `git`, `gh`) in your
  Amp permissions first. The prompt starts with `aayu hook loop:<name>`, so a halted repository or a paused budget
  stops the run before any write. The same JSON is also the input of Amp's `set_schedule`, if you schedule an Amp
  thread in the project instead.
- **A Cursor automation.** `loops/cursor/aayudha-loops.tf` declares both loops as `cursor_platform_workflow`
  resources that run as you: `terraform apply` with `CURSOR_TOKEN` in the environment.

This repository's `[verify] maintainer` is `amp`: the headless Amp run reads the brief and opens the one PR.

`go generate ./...` renders aayudha's own loops; `aayu init --gates` renders a product's:

- `.github/workflows/aayudha-loops.yml`: the GitHub Actions fallback. Actions cron is UTC, so each loop has one cron
  per UTC offset of the zone, and the guard runs only the one for today's offset.
- `loops/amp/<name>.json`: the input of Amp's `set_schedule` (an RFC 5545 rule in the zone, `run_mode` fresh).
- `loops/cursor/aayudha-loops.tf`: Cursor Automations as `cursor_platform_workflow` resources of Cursor's Terraform
  provider; apply them with `terraform apply` and `CURSOR_TOKEN`. `loops/cursor/automate.md` has the same loops as
  prompts for Cursor's `/automate` skill, for a user without Terraform or a key.
- `loops/claude/routines.md`: one `/schedule` prompt per loop for Claude Code (v2.1.225 or later, claude.ai login),
  which saves it as a routine.
- `loops/codex/scheduled-tasks.md`: one prompt per loop for a Codex or ChatGPT chat in the desktop app, which creates
  the scheduled task, with each loop's RFC 5545 rule.

## Setting the loops up in one tool

`aayu install --loops` is the one entry point, per repository. One tool runs every loop: two would run each loop
twice. It asks which tool should run them, offering only those whose CLI it detects here (amp, cursor-agent, claude,
codex), or takes `--host`, and warns when `loops/active.json` records loops in another tool. Then:

- Amp: with `--apply`, one headless Amp thread per loop in an orb of the project (`amp -ox`) calls `set_schedule` with
  the rendered input and keeps that schedule (one schedule per thread); each loop is recorded.
- Cursor: with `--apply`, `terraform apply` of `loops/cursor/aayudha-loops.tf` when `terraform` and `CURSOR_TOKEN` are
  there, recording each loop; without them, the `/automate` prompts in `loops/cursor/automate.md`.
- Claude Code and Codex: the prompts in `loops/claude/routines.md` and `loops/codex/scheduled-tasks.md` to paste.

`--run-now <loop>` tests one loop once with the tool's own CLI here (`amp -x`, `cursor-agent -p`, `claude -p`,
`codex exec`); at a terminal it is offered after setup. Record each loop you set up by hand with
`aayu install --loops --host <tool> --record <loop>=<ID>` (or `all=<ID>`; an empty ID forgets one); the record is
`loops/active.json`, committed with the loops. `aayu doctor` then shows each loop with the tool that runs it: active,
stale (the loop changed after it was set up), missing, or double (recorded in two tools). In Actions, the guard leaves
a recorded loop to its tool, so the fallback never runs it a second time.

Every run starts with `aayu hook loop:<name>`, in every tool: it exits 4 while the
repository is halted (`aayu halt`), 6 while the `[budget]` breaker has paused agent lanes or the Actions loops have used
`[budget] ci_minutes_month` this month, and otherwise says whether this run is the scheduled one. A loop whose command is not implemented yet is not rendered (n/a until its WP).
