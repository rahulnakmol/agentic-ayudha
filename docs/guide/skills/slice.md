# `slice`: split a Feature into small stories

A Feature is too big to build, check, and ship in one go, and big pieces hide their risk. `slice` splits it into
stories sized XS to L, each with promises a person can check, the [layers](../concepts-layers.md) it touches, and
the order to build them in.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | ![slice: break a Feature into stories](../../../design/icons/verbs/slice.svg) Splits a Feature into small stories, each with checkable promises, the parts of the code it touches, and its order. |
| Journey stage | [Break it down](../journey-2-break-down.md) |
| Accountable seat | ![Arjun, Product Strategist: an arrow in the centre of the target](../../../design/icons/seats/product-strategist.svg) [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Does the work | Arjun, the Accountable seat's twin, in the [`plan`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa), [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Informed | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/slice/SKILL.md](../../../skills/slice/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A Feature is filed and you need stories you could each build in a day.
- A story turned out to be XL: it cannot pass Ready, so slice it into stories of L or smaller.
- You have a request in words and no issue yet: the skill unpacks it into a Feature first, then slices it.

Only one change, already small? Write it as a story with [`grill`](grill.md). Planning how to build a story is
[`plan`](plan.md).

## Run it

Ask your agent with the Feature's number. It reads the Feature, drafts the stories, has `aayu` check the draft, and
creates them once every problem is fixed.

```text
Use the slice skill to split Feature 42 into stories, and show me the draft before you create them.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu slice <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<feature>` | yes | Feature (or XL story) to slice. |
| `--draft` | no | JSON file with the stories the host's model sliced (evals/schemas/slice.json); without it slice reports the footprint to slice within. |
| `--create` | no | Create the validated stories under the Feature (through the actions log) and close an XL story as split; without it slice only validates. |

<!-- END GENERATED: skill-usage -->

The final step is `aayu slice 42 --draft slice.json --create`.

## What you get back

New story issues under the Feature, created in dependency order. Each has an `effort:` label, a Completion Goal of
numbered criteria, one `layer:Lxx` label per layer it touches, and a link to each story it depends on. An XL story
you sliced is closed as split. Questions the Feature does not answer come back as open questions, not as criteria.

**You should see** the new stories listed under the Feature, each with its `layer:` labels. Without `--create`,
nothing is written: `slice` only checks the draft.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **validator:** The work item passes stage validation for its type.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

Every story is held to the Feature's scope: a criterion with no words in the Feature behind it becomes an open
question, so nothing is added or restricted that nobody asked for.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

You agree the split: does each story deliver something you could show someone, and is anything missing? A person
comes in again when each story is made Ready.

## What's next

- Plan each story one layer at a time: [`plan`](plan.md).
- See a Feature become five stories in the story: [the plan](../journey-2-break-down.md).
- [Every skill](README.md).
