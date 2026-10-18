# `scope`: plan what a release contains, and keep it

Ideas and feedback grow faster than a team can ship them. `scope` keeps one plan per release: the GitHub milestone
named by its version. An agent fills it and drafts it. A person accepts the draft as the baseline. After that, every
change counts as drift against a budget, and a big change waits for a person's decision.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Plans what a release contains, freezes it once a person accepts it, and stops later changes beyond the budget unless a person decides. |
| Journey stage | [Break it down](../journey-2-break-down.md) |
| Accountable seat | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Does the work | Arjun, the Accountable seat's twin, in the [`plan`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona), [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Informed | [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala), [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/scope/SKILL.md](../../../skills/scope/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You know what the next release is for, and you want to say what it contains before anyone builds it.
- New Feedback arrives after the plan is set, and you must put it in this release, the next one, or the backlog.
- Someone wants to add or cut work after the baseline, and you need to know if that is a small nudge or a new plan.

Splitting one Feature into Stories is [`slice`](slice.md). Shipping the release is [`release`](release.md).

## How it works

1. **Intent.** Write the Release's Summary: who gets what, and the number that says it worked.
2. **Breakdown.** Add Initiatives, Features, Stories, and Bugs to the release with `--add`. Every Story and Bug needs
   an effort label from XS to L; points come from `[scope] effort_points` in [CONSTRAINTS.md](../../../CONSTRAINTS.md).
3. **Baseline.** `--draft` saves the plan on the `ledger` branch and asks a person to accept it. The person runs
   `aayu scope <release> --accept`, or accepts the card in [`aayu inbox`](../how-trust.md). Both ask for GitHub's
   device code; in the inbox, one code covers the session.
4. **Hand-off.** `--handoff` writes the Release's `includes` links, which `aayu release` ships, and posts the plan.
5. **Drift.** Each item added, removed, or resized after the baseline counts, weighted by effort. Within
   `[scope] budget_percent` (20% by default), changes go ahead. Past it, scope refuses and points to `aayu decide`.
   With an accepted Decision, the agent can only ask; a person accepts the change.

The date stays fixed and the scope moves: a slip goes to the next release with `--drop`. Feedback moves with
`--assign … --to this|next|backlog`, which sets its milestone and `horizon:` label and never edits its body.

## Run it

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu scope <release>
```

| Input | Required | What it does |
|---|---|---|
| `<release>` | yes | Release issue (Version vX.Y.Z) whose scope to plan; without another flag, scope reports the plan, its points, and its drift. |
| `--add` | no | Initiative, Feature, Story, or Bug to add to the release's milestone; after a baseline it counts as drift. |
| `--drop` | no | Item to remove from the release's milestone, so it rides a later release; after a baseline it counts as drift. |
| `--assign` | no | Feedback issue to assign with --to; sets its milestone and horizon label, never its body, and is not drift. |
| `--to` | no | With --assign: this release, the next release milestone, or the backlog. One of: `this`, `next`, `backlog`. |
| `--draft` | no | Snapshot the plan on the ledger (releases/\<version>/draft.json) and ask a person to accept it; refuses an unsized, XL, or unsliced plan. |
| `--decision` | no | Accepted Decision that names the Release: with --add, --drop, or --draft over budget, it asks a person to accept the change; never applies it. |
| `--accept` | no | Accept the Release's waiting request, a drafted baseline or a change over budget, through GitHub's device flow every time; aayu inbox shows the same card. **Person only.** |
| `--handoff` | no | Hand the baselined plan off: write the Release's includes links and post the plan on it. |

<!-- END GENERATED: skill-usage -->

## What you get back

The plan of the release: each item with its type, effort, and points, and the total. After a baseline, the drift from
it in percent against the budget, with the items added, removed, and resized. `--draft` writes
`releases/<version>/draft.json` on the ledger and a request on the Release. A person's `--accept` writes
`baseline-<n>.json` and `drift.json`. `--handoff` writes the Release's `includes` links and posts the plan on it.

**You should see** the plan's items and points, then a line such as `baseline 1: drift 12.5% of 40 points, budget 20.0%`. Over the budget,
scope stops and names the next step: file a Decision, and a person accepts it.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **scope-budget:** After a person accepts a release's baseline, drift stays within [scope] budget_percent, or a person accepts the change through GitHub's device flow against an accepted Decision.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- Once a person accepts a release's baseline, a change beyond the budget waits for a person: they approve it through GitHub's device code, against an accepted Decision.
- `--accept`: Accept the Release's waiting request, a drafted baseline or a change over budget, through GitHub's device flow every time; aayu inbox shows the same card.

<!-- END GENERATED: skill-human -->

## What's next

See [every skill](README.md).
