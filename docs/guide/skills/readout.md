# `readout`: read out an experiment's lift

Calling an experiment early, or quoting a number nobody can check, turns a test into an opinion. `readout` reads
both arms' counts once each has reached the Experiment's minimum sample, and writes the lift, the significance test,
and the confidence interval into the Experiment as its Result.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Reads out an experiment once it has enough people, and writes down the measured lift. |
| Journey stage | [Measure and learn](../journey-8-learn.md) |
| Accountable seat | [Revenue Architect](../../../seats/revenue-architect/CHARTER.md) (Kuber) |
| Does the work | Kuber, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Atri), [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti) |
| Layers it can touch | [L08 Analytics / reporting](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/readout/SKILL.md](../../../skills/readout/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- An Experiment has run long enough that both arms may have reached its minimum sample.
- You need the measured lift written down where anyone can reproduce it, before deciding which version stays.

Measuring a Goal against its baseline after a release is [`measure`](measure.md). No Experiment yet? Start with
[`experiment`](experiment.md).

## Run it

Run it once the experiment has had time to collect people. Your agent reports the Result exactly as written.

```text
Use the readout skill to read out Experiment issue 57 and tell me the Result as written.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu readout <experiment>
```

| Input | Required | What it does |
|---|---|---|
| `<experiment>` | yes | Experiment to read out. |
| `--source` | no | Where the per-arm counts come from; default posthog. One of: `posthog`, `growthbook`. |
| `--metric` | no | GrowthBook metric ID (with --source growthbook); PostHog uses the Experiment's Metric. |
| `--from` | no | Start of the PostHog readout window (YYYY-MM-DD, UTC); default the Experiment's creation. |

<!-- END GENERATED: skill-usage -->

With GrowthBook: `aayu readout 57 --source growthbook --metric met_signup`.

> **Needs setup:** the counts come from PostHog or GrowthBook. Connect the one you use with `aayu init
> --integrations`, then store its read-only key with `aayu auth set-secret posthog` or `aayu auth set-secret
> growthbook`.
>
> **Honest status:** the PostHog and GrowthBook readouts have not been proven live against real accounts here.

## What you get back

A Result section in the Experiment issue: the lift of the treatment over the control, relative and absolute, marked
significant or not significant at the chosen alpha, with the z-test, the confidence interval, a table of users and
conversions per arm, the window, and the source query. The Experiment moves to `status:measured`.

**You should see** the Experiment labelled `status:measured` and its Result starting `Lift of`. Stuck? See
[everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **min-sample:** The experiment reached its minimum sample size.

<!-- END GENERATED: skill-gates -->

While either arm is under the minimum sample, or the stated minimum disagrees with the Experiment's own parameters,
it stops with exit 4 and writes nothing. Wait, then run it again.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The Result says what the data shows, "not significant" included. Deciding what it means for the product, and which
version stays, is a person's call.

## What's next

- Measure the Goal the Feature serves: [`measure`](measure.md).
- The whole after-release flow: [did it work?](../guide-learn.md)
- [Every skill](README.md).
