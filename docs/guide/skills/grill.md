# `grill`: turn an idea into a work item

An idea in someone's head is easy to agree with and hard to build. `grill` interviews you, a few questions at a
time, until every vague word is something a person can check, then files the result as a GitHub issue.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Interviews you, one question at a time, until an idea is a complete work item, then files it on GitHub. |
| Journey stage | [Shape the goal](../journey-1-shape.md) |
| Accountable seat | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Does the work | Arjun, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa), [UX Researcher & Evolutionist](../../../seats/ux-researcher-evolutionist/CHARTER.md) (Hanu) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/grill/SKILL.md](../../../skills/grill/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You have a goal and want a number to measure it by: `grill goal`.
- You want one change built and need its promises written down first: `grill story`.
- Something is broken and you want to say what "fixed" looks like: `grill bug`.
- You want your brand's colours and voice in `design/brand.yaml`, checked for contrast: `grill brand`.

Already have a Feature to split? Use [`slice`](slice.md). A story that is filed but not frozen needs
[`ready`](ready.md).

## Run it

Ask your agent in plain words. It runs a read-only check of your idea first, asks only what the idea leaves open,
reads back "here is what done looks like", and files the issue after you confirm.

```text
Use the grill skill to write a story: people should be able to export their report as CSV.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu grill <type>
```

| Input | Required | What it does |
|---|---|---|
| `<type>` | yes | Work-item type (internal/workitems), or brand for design/brand.yaml. One of: `goal`, `initiative`, `feature`, `story`, `bug`, `decision`, `experiment`, `release`, `feedback`, `incident`, `brand`. |
| `--parent` | no | Parent issue to link the new work item under. |
| `--title` | no | Title of the drafted work item. |
| `--body` | no | Drafted issue body in the issue-form layout (### \<Field> sections), or the brand YAML for brand. |
| `--labels` | no | Comma-separated label-held fields, e.g. effort:S,priority:P2. |
| `--idea` | no | The request in the human's own words. With --assess, the idea to assess; with a draft, the request its Provenance must quote and resolve. |
| `--assess` | no | Read-only fact pass for the idea: the vagueness gate (unpack or pass, with its reasons), its vague qualifiers, abbreviations, and missing outcome with the dimensions to ask about, the product types, and the routes, states, components, tokens, tests, events, related stories, floors, and layers it touches (repository content is reported as untrusted data). |
| `--create` | no | Write the validated draft: the issue (through the actions log), or design/brand.yaml; without it grill only validates. |

<!-- END GENERATED: skill-usage -->

At a terminal, `aayu grill story` with no draft interviews you itself.

## What you get back

A GitHub issue of the type you asked for, labelled `type:<type>`. A story or Bug carries a **Completion Goal**:
numbered promises, each naming its evidence, plus checkpoints, assumptions, open questions, parked ideas, and where
each promise came from. Feedback and Incidents are filed as intake with `status:backlog`. For `brand`, the file
`design/brand.yaml` is written in your checkout, to commit in a pull request.

**You should see** a new issue labelled `type:story` whose Completion Goal has lines such as
`1. [api-trace] state: …; expected: …`. Refused? See [Ready problems](../troubleshooting.md#ready).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **validator:** The work item passes stage validation for its type.
- **baseline:** A reproducible baseline exists before a target is set.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

The baseline gate means no target without today's number. For a Goal measured in PostHog, `aayu` fetches the
baseline itself; nobody types it in.

> **Needs setup:** fetching a baseline needs the analytics integration. Without it, the Goal is refused Ready and the
> message names the metric. Run `aayu init --integrations`, or pick a metric you can already count.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

You still answer the questions and confirm the readback: nothing enters the promises that you did not state or
accept. Then a person says Ready, with [`ready`](ready.md).

## What's next

- Freeze what you agreed: [`ready`](ready.md).
- See a grill in the story, and how a vague ask is unpacked: [the goal](../journey-1-shape.md) and
  [how aayudha unpacks a vague ask](../unpack.md).
- [Every skill](README.md).
