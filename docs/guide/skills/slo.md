# `slo`: agree how reliable is reliable enough

Without an agreed target, every outage turns into an argument about whether to keep shipping. `slo` sets SLOs
(service level objectives) from the user's side, measures the error budget left, and records it for `release`, which
ships only Bugs while a budget is spent.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Sets how reliable the product must be for its users, measures how much room for failure is left, and holds back risky releases when it runs out. |
| Journey stage | [Keep it running](../when-to-use-what.md#something-broke-in-production) |
| Accountable seat | ![Shesha, Production Engineer: an endless coil that holds the world up](../../../design/icons/seats/production-engineer.svg) [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Shesha) |
| Does the work | Shesha, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun), [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona), [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/slo/SKILL.md](../../../skills/slo/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- The product has users and no agreed reliability target yet.
- Each week, to record every SLO's budget: the `error-budget-review` [loop](../guide-loops.md) runs it.
- Before a release, when the recorded status is older than 8 days.

Confirming and fixing a known failure is [`runbook`](runbook.md).

## Run it

Your agent measures a baseline, drafts the SLO document, and checks it. Once you agree, `--create` writes
`ops/slos.toml` and you open the pull request:

```text
Use the slo skill with the telemetry export in ./telemetry to propose an availability SLO for the journey our users depend on most, with a burn-rate alert and the budget policy, and check the draft before you write ops/slos.toml.
```

> **Needs setup:** a sanitised OpenTelemetry export with `traces.jsonl`. Ask whoever has production access for one,
> aggregates only. To record the budget, it must cover the whole SLO window, usually 28 days.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu slo
```

| Input | Required | What it does |
|---|---|---|
| `--telemetry` | no | Sanitised OpenTelemetry export directory to measure each SLO in ops/slos.toml from (traces.jsonl), read with no network. |
| `--service` | no | Service to scope the baseline queries to; default the only service in the export. |
| `--query` | no | Run one more bounded query and log it as qN: \<kind> [by f,…] [where k=v …] [limit N]. |
| `--draft` | no | SLO document (Markdown) to check: every part, the stage's artefact, cited evidence, objectives as percentages with windows, a burn-rate alert, the budget policy, and a proposed ops/slos.toml in a toml block that parses. |
| `--create` | no | Write the checked draft's toml block to ops/slos.toml in the checkout, for a pull request a person reviews; needs --draft. |
| `--record` | no | Record each SLO's measure and budget on the ledger (slo/status.json), which release reads; needs --telemetry. |

<!-- END GENERATED: skill-usage -->

The weekly record: `aayu slo --telemetry ./telemetry --record`.

## What you get back

`ops/slos.toml` in your checkout, from the checked draft's `toml` block. With `--record`, each SLO's measure and
budget on the [ledger](../../../GLOSSARY.md) at `slo/status.json`, which `release` reads.

**You should see** each SLO's value against its objective, the budget left, and its burn rates. `release` refuses
while the record is stale or missing; see [exit codes](../troubleshooting.md).

> **Honest status:** aayudha has no production service, so its own repository has no `ops/slos.toml` to show.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person reviews every change to `ops/slos.toml`, a protected path. While a budget is spent, a Story ships only if a
person accepts an `error-budget` waiver with [`decide`](decide.md).

## What's next

- Give each alert a runbook: [`runbook`](runbook.md).
- The whole flow: [something broke in production](../when-to-use-what.md#something-broke-in-production).
- [Every skill](README.md).
