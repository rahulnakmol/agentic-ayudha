# loops

Scheduled loops (WP7.1), one file each: `loops/<name>.toml`.

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

`verify-sweep` (nightly) and `verify-maintain` (weekly) are the feature maps' loops (WP4.5b.4): the sweep runs every
behaviour check with no model, and both hand the maintain brief to the agent app `[verify] maintainer` names, the
sweep only when a map drifted or a check failed. No model key is used in the Actions jobs.

`go generate ./...` renders aayudha's own loops; `aayu init --gates` renders a product's:

- `.github/workflows/aayudha-loops.yml`: the GitHub Actions fallback. Actions cron is UTC, so each loop has one cron
  per UTC offset of the zone, and the guard runs only the one for today's offset.
- `loops/amp/<name>.json`: the input of Amp's `set_schedule` (an RFC 5545 rule in the zone, `run_mode` fresh); ask an
  Amp thread in the project to set it.
- `loops/cursor/aayudha-loops.tf`: Cursor Automations as `cursor_platform_workflow` resources of Cursor's Terraform
  provider; apply them with `terraform apply` and `CURSOR_TOKEN`.

Every run starts with `aayu hook loop:<name>`: it exits 4 while the repository is halted (`aayu halt`), 6 while the
`[budget]` breaker has paused agent lanes or the Actions loops have used `[budget] ci_minutes_month` this month, and
otherwise says whether this run is the scheduled one. A loop whose command is not implemented yet is not rendered
(n/a until its WP).
