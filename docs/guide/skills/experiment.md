# `experiment`: design an A/B test before you release

A number that moved after a release cannot tell you that *your change* moved it. `experiment` designs an A/B test
for a Feature: a hypothesis, the conversion event you measure, the feature flag that splits traffic, and the minimum
number of people each arm needs. It files all of that as an Experiment issue under the Feature.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Designs an A/B test for a Feature: what you expect, what you measure, and how many people you need. |
| Journey stage | [Measure and learn](../journey-8-learn.md) |
| Accountable seat | [Revenue Architect](../../../seats/revenue-architect/CHARTER.md) (Plutus) |
| Does the work | Plutus, the Accountable seat's twin, in the [`plan`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Metis), [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Iris), [Data & AI Architect](../../../seats/data-ai-architect/CHARTER.md) (Mneme) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Prometheus) |
| Layers it can touch | [L03 Application / workflow, L08 Analytics / reporting](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/experiment/SKILL.md](../../../skills/experiment/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You are choosing between two versions of a Feature and need to know which one moves the metric.
- You need evidence that the change caused the difference, and have the traffic to reach a minimum sample.

Only asking whether a Goal's number moved after a release? That is [`measure`](measure.md).

## Run it

Plan the experiment before the release. Your agent asks you for, or works out with you, the hypothesis, the event,
the flag key, and the smallest change worth detecting.

```text
Use the experiment skill to design an A/B test for Feature issue 42: a one-page checkout lifts sign-ups.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu experiment <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<feature>` | yes | Feature (or Initiative) to experiment on. |
| `--hypothesis` | yes | If we do X, the metric moves by Z because W. |
| `--metric` | yes | Conversion event the experiment moves (a PostHog event name). |
| `--flag` | yes | Feature-flag key that splits traffic: the PostHog flag key and GrowthBook tracking key. |
| `--mde` | yes | Minimum detectable effect: an absolute rate change (0.02) or a relative one (10%). |
| `--exposure` | no | HogQL predicate for the population the baseline counts; default every person with an event. |
| `--alpha` | no | Two-sided significance level; default 0.05. |
| `--power` | no | Statistical power; default 0.8. |
| `--days` | no | Baseline window in whole UTC days before today; default 28. |

<!-- END GENERATED: skill-usage -->

The usual call: `aayu experiment 42 --hypothesis "A one-page checkout lifts sign-ups" --metric signed_up --flag
checkout-v2 --mde 10%`.

> **Needs setup:** the baseline rate always comes from PostHog. Connect it with `aayu init --integrations`, then
> store a read-only project key with `aayu auth set-secret posthog`.
>
> **Honest status:** the PostHog adapter has not been proven live against a real PostHog project here.

## What you get back

An Experiment issue, labelled `type:experiment` and `status:planned`, as a sub-issue of the Feature. It holds the
hypothesis, the metric, the baseline rate with its counts, the flag, and the minimum sample per arm.

**You should see** a line with the baseline rate and `minimum sample N per arm`, and the new Experiment under the
Feature. Stuck? See [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **baseline:** A reproducible baseline exists before a target is set.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

In plain words: when PostHog counts nobody in the population over the window, there is no baseline rate, and it
stops with exit 4. It also refuses a closed Feature, or a flag another Experiment under it already uses.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

Integration keys are read-only, so `experiment` never creates the flag. A person creates it in PostHog or GrowthBook
with the same key, and decides what the Result means once [`readout`](readout.md) writes it.

## What's next

- Read out the lift once both arms reach the minimum sample: [`readout`](readout.md).
- Experiments in the story: [the number](../journey-8-learn.md#was-it-this-change-experiments).
- [Every skill](README.md).
