# `measure`: find out whether the Goal's number moved

After a release, the question is whether the number moved, counted the same way as before. `measure` computes a
Goal's metric exactly as its baseline was computed, over a new window, and writes the Result into the Goal.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Measures a Goal against its baseline, the same way as before, and writes the result into the Goal. |
| Journey stage | [Measure and learn](../journey-8-learn.md) |
| Accountable seat | [Data & AI Architect](../../../seats/data-ai-architect/CHARTER.md) (Mneme) |
| Owns the thresholds | [Revenue Architect](../../../seats/revenue-architect/CHARTER.md) (Plutus) |
| Does the work | Mneme, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Metis) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Prometheus) |
| Layers it can touch | [L08 Analytics / reporting](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/measure/SKILL.md](../../../skills/measure/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A release went out and the Goal's measurement is due; the `release-measure` [loop](../guide-loops.md) can run it.
- You want the weekly AARRR cohorts and the North Star from your exports, without a Goal.
- Someone asks whether a Goal is done, and you need the verdict in writing.

Did *your change* cause the difference? Plan an [`experiment`](experiment.md) before the release.

## Run it

The binary computes the number, with no model, so anyone can run it again. Your agent explains the Result:

```text
Use the measure skill to measure Goal issue 12 against its baseline from payments.csv and events.jsonl, then tell me what the Result means and how far it can be trusted.
```

> **Needs setup:** a billing export (CSV) or an events export from Application Insights, CloudWatch RUM, or GA4
> BigQuery (JSON lines). For `--live`, connect [Azure](../../getting-started-azure.md),
> [AWS](../../getting-started-aws.md), or [Google Cloud](../../getting-started-gcp.md) first.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu measure [<issue>]
```

| Input | Required | What it does |
|---|---|---|
| `<issue>` | no | Goal or Initiative to measure; without it, only the cohorts and North Star are reported. |
| `--metric` | no | acquisition, activation, retention, referral, revenue, north_star, or \<source>:\<metric> [filter]; default the issue's Metric. |
| `--from` | no | Window start (YYYY-MM-DD, UTC); default 28 days before --to. |
| `--to` | no | Window end, exclusive (YYYY-MM-DD, UTC); default today. |
| `--billing` | no | Billing export (CSV). |
| `--appinsights` | no | Application Insights events export (JSON lines). |
| `--aws` | no | AWS CloudWatch RUM events export (JSON lines). |
| `--gcp` | no | Google Analytics 4 / Firebase BigQuery events export (JSON lines). |
| `--live` | no | Pull the window's events live from configured cloud integrations instead of an export file: azure, aws, gcp, comma-separated. |
| `--engine` | no | Query engine for the snapshot; duckdb is optional and shelled out. One of: `sqlite`, `duckdb`. |

<!-- END GENERATED: skill-usage -->

The most common run: `aayu measure 12 --billing payments.csv --appinsights events.jsonl`.

## What you get back

A SQLite snapshot of the window, and a Result on the Goal: the value, baseline, target, verdict, snapshot hash, and
query. The Goal moves to `status:done` when the target is met, else `status:measured`.

**You should see** a Result whose first line starts `Target met:` or `Target not met:`. Exit 4 with no Result means
it refused, for example because no cohort is complete yet: rerun with a later `--to`. See
[exit codes](../troubleshooting.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **baseline:** A reproducible baseline exists before a target is set.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person decides what the Result means. A target is never moved to make it pass: a new target for the next period is
a Decision only a person accepts, through [`decide`](decide.md).

## What's next

- Learn from how the work itself went: [`retro`](retro.md).
- The whole flow after a release: [did it work?](../guide-learn.md#measure-the-goal)
- [Every skill](README.md).
