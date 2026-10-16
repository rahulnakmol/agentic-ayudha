# loops

Loops (WP7.1), one file each: `loops/<name>.toml`. A loop runs on a schedule (`cron`) or on a GitHub event (`on`).

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

An event loop sets `on` instead of `cron` (Appendix B S-7.1b):

```toml
description = "Triage each new issue."
command = ["triage", "{number}"]   # {number}: the issue or pull request; {tag}: the release
on = "issues.opened"               # issues.opened|labeled, pull_request.opened|synchronize, check_suite.completed, release.published
filter = { label = "type:feedback" }  # optional: label (issues, pull requests), conclusion = "failure"|"success" (check suites)
```

Each host starts an event loop with its own trigger where it has one: an Amp orb spawner (`create_github_subscription`)
for a newly opened issue or pull request, or a Cursor Automation trigger (`git_pull_request`, `git_ci_completed`, or
`git_label` for an issue with the label). A host without the trigger does not render the loop, and the rendered file
says why. The Actions fallback never answers an event with a model: its job only records the event in the run summary.

This repository's loops are also the defaults a product copies. `triage` (weekly) and `triage-new` (when an intake form
opens an issue) replace the retired aayudha-triage workflow: `aayu triage` lists the intake issues with no model, and
the agent in your own session classifies each one with `--as`. `housekeeping` (weekly) runs `aayu gc`, then reports
doctor drift, feature-map drift, and stale Ready issues. `feedback`, `ci-failed`, and `release-measure` start on a new
Feedback issue, a failed check suite (the delegated builder gets the failed check, #231), and a published release.

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
- `loops/amp/<name>.json`: the input of Amp's `set_schedule` (an RFC 5545 rule in the zone, `run_mode` fresh), or for
  an event loop of Amp's `create_github_subscription` (an orb spawner, `destination` new-orb, with the project's
  `project_id` and an `agent_mode` added); ask an Amp thread in the project to set it.
- `loops/cursor/aayudha-loops.tf`: Cursor Automations as `cursor_platform_workflow` resources of Cursor's Terraform
  provider; apply them with `terraform apply` and `CURSOR_TOKEN`.
- `loops/claude/routines.md`: Claude Code routines, as the values for the claude.ai/code/routines form or `/schedule`
  (Claude Code has no file or API that creates a routine).
- `loops/codex/scheduled-tasks.md`: Codex scheduled tasks for the ChatGPT desktop app, with each loop's RFC 5545 rule
  (Codex has no file or API that creates one, and no GitHub event trigger).

## Setting the loops up on your agent hosts

`aayu install --loops` is the one entry point, per repository: it detects the agent hosts you have here (the amp,
cursor-agent, claude, and codex CLIs) and, for each, sets up this repository's loops where the host's own tooling can,
and shows the exact steps for the rest:

- Amp: one prompt per loop for an Amp thread of the project, which calls `set_schedule` (one schedule per thread) or
  `create_github_subscription` with the rendered input and the project's ID. Amp has no CLI for either.
- Cursor: with `--apply`, `terraform apply` of `loops/cursor/aayudha-loops.tf` when `terraform` and `CURSOR_TOKEN` are
  there; it records the Cursor loops as set up.
- Claude Code and Codex: the sections of `loops/claude/routines.md` and `loops/codex/scheduled-tasks.md` to enter.

Record each loop you set up with `aayu install --loops --host <host> --record <loop>=<ID>` (or `all=<ID>`); the record
is `loops/active.json`, committed with the loops. `aayu doctor` then reports every loop, scheduled or event, on each of
your hosts: active, stale (the loop changed after it was set up), or missing.

Every run starts with `aayu hook loop:<name>`, on every host and for both kinds of loop: it exits 4 while the
repository is halted (`aayu halt`), 6 while the `[budget]` breaker has paused agent lanes or the Actions loops have used
`[budget] ci_minutes_month` this month, and otherwise says whether this run is the scheduled one (an event loop is due
whenever its own event started it). A loop whose command is not implemented yet is not rendered (n/a until its WP).
