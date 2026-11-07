# #892 Technical design

No tech baseline exists (`specs/892` is absent from the ledger). Everything under "Planned" is planned, not built.
It comes from the Expected paths and Scope of [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) and
from Decisions [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) and
[#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868) (superseded).

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| `[budget]` keys | `internal/config/defaults.toml` `[budget]`; `config.Budget` in `internal/config/config.go:365-370` | `per_story_usd = 60.0`, `per_day_usd = 100.0`, `per_pr_inspect_max = 5.0`, `ci_minutes_month = 2000`. This repository sets `per_pr_inspect_max = 15.0` (`.aayudha/config.toml` `[budget]`). |
| Config docs | `internal/config/gen/main.go:94-99` | Rows for the four `[budget]` keys that render into `CONSTRAINTS.md`. |
| Breaker | `models.Breaker` in `internal/models/gateway.go`: `Seed` (`:113`), `SeedLog` (`:131`), `Check` (`:163-178`), `Add` (`:182-`) | Counts spend per story and per day from the actions log. A limit of 0 means no limit. |
| Init flow | `runInit` and `initSetup` in `internal/cli/init.go:69-189`; `initrepo.Init.Run` and `RunExtras` in `internal/initrepo/` | Confirms admin use, then writes labels, forms, `.aayudha/`, `AGENTS.md`. `Extras.Ask` uses `Env.askLine` (`internal/cli/manifest.go:24`), which returns "" with no terminal. |
| Host detection | `doctor.DetectHost` (`internal/doctor/checks.go:229`); `doctor.Inspect` (`internal/doctor/coexist.go:73`) | Finds the running host and the coexistence report that init reads. |
| Subscription limit | `delegate.UsageLimit` (`internal/delegate/usage.go:37`); `usagePause` in `internal/cli/delegate.go:1187-1234` | Reads a receiver's "usage limit" message and pauses the build until the reset time. |
| Spend wording | `internal/cli/build.go:83-91`; the `status.SpendNote` constant (`internal/status/status.go:375`) | Says "subscription use not counted", and "no cost limit" when no pay-as-you-go key or endpoint is set. |
| Prices | `models.ListPrices` (`internal/models/prices.go:46`), `models.PriceOf` (`:95`) | List prices per model, with source URL and read date. A `[[models.prices]]` entry overrides one. |
| Mission budget | `internal/cli/mission.go:874-911`; `aayu mission budget --to` | A mission's budget is stories × `[budget] per_story_usd`, "no cost estimate from #886 yet". |
| Build cap | `[build] agents = 5` (`internal/config/defaults.toml`), from PR [#1089](https://github.com/rahulnakmol/agentic-ayudha/issues/1089), commit [f563bc4](https://github.com/rahulnakmol/agentic-ayudha/commit/f563bc48) | Replaced `in_flight`, as Decision #918 says. |

The Expected path `internal/delegate/hostpause.go` (#892) does not exist on `dev`.

## Layers touched

From `LAYERS.md`: L02 (`internal/cli/**`), L03 (`internal/initrepo/**`, `internal/delegate/**`, `internal/status/**`,
`internal/verbs/**`), L04 (`CONSTRAINTS.md`), L05 (`internal/models/**`), L07 (`internal/config/**`). `docs/guide/` is
in no layer.

## Planned, not built

1. **Config.** A `[budget.hosts]` table, host name to `"subscription"` or `"metered"` (#868 option 1), and one key for
   the API-call cap (#892 "a separate cap for API calls"; #918 names `metered_per_release_usd`). Add fields to
   `config.Budget`, defaults to `defaults.toml`, and rows to `internal/config/gen/main.go`, then `go generate` updates
   `CONSTRAINTS.md`.
2. **Breaker.** `Breaker.Add` or its callers skip agent-lane cost from a host marked subscription. The API-key cap
   applies to `ModeDirect` verbs, the merge gate's review, and evals (#868 option 1). Callers named in #892:
   `internal/cli/build.go`, `delegate.go`, `buildnext.go`, `lease.go`, `internal/delegate/usage.go`.
3. **Init question.** After `it.Run` succeeds, `runInit` asks per detected host, then the daily budget. It prints what
   the budget buys from `models.PriceOf` for the build lane's model, with the tokens labelled "assumed". It writes
   `.aayudha/config.toml` only on a terminal (#868 option 1).
4. **Read-out.** `aayu status` and `aayu build` show the billing mode per host and the cap that holds (inferred from
   the Expected paths).

## Data and ledger records

No new ledger record is planned. The answers live in `.aayudha/config.toml`. The breaker keeps reading spend from
the actions log (`Breaker.SeedLog`).

## Key decisions

| Decision | State | Effect |
|---|---|---|
| [#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868) | cancelled, superseded by #918 (2026-10-23) | Held the first design of the init budget question. |
| [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) | accepted 2026-10-23 | `[build] agents = N` replaces #868's cap; the init budget question belongs to #892. |
