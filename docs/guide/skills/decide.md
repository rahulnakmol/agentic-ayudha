# `decide`: record a choice and who made it

A choice made in a chat thread is forgotten by the next sprint, and an agent should never make it for you. `decide`
records a Decision with its options, how each scores, and a recommendation, chases it until someone decides, and
lets only a person with the authority accept it.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | ![decide: a person chooses one path](../../../design/icons/verbs/decide.svg) Records a real choice with its options and a recommendation; only a person with the authority accepts it. |
| Journey stage | [Break it down](../journey-2-break-down.md) |
| Accountable seat | ![Atri, Founder's Sage: his star in the Saptarishi, where the handle meets the bowl](../../../design/icons/seats/founder.svg) [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Does the work | Atri, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun), [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona), [Auditor](../../../seats/auditor/CHARTER.md) (Vidhi) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/decide/SKILL.md](../../../skills/decide/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You must choose between two designs, two vendors, or two scopes, and want the options and trade-offs on record.
- You are stuck on an issue and want a one-page decision pack you can decide from in five minutes.
- A floor fails for a reason you accept, and you want a floor waiver on record instead of a quiet skip.
- You want to see which open Decisions are overdue.

No real choice, only work to describe? Use [`grill`](grill.md). How to build a chosen option is [`plan`](plan.md).

## Run it

Ask your agent for a decision pack. The seat that owns the topic works it, cites every fact, and stops at the
choice: nothing is written.

```text
Use the decide skill to write a decision pack for issue 42, and stop at the choice for me to make.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu decide [<decision>]
```

| Input | Required | What it does |
|---|---|---|
| `<decision>` | no | Decision issue; without one, the SLA sweep over every open Decision. With --waiver, the story or bug to waive a floor on. With --pack, any issue, or a question in quotes. |
| `--accept` | no | Option to accept, by its number or its name (human identity only). **Person only.** |
| `--waiver` | no | Floor to waive; creates a floor-waiver Decision. |
| `--pack` | no | On demand, read-only: the owning seat's twin, its persona at the team stage, the cited facts, and the decision-pack outline. |
| `--seat` | no | With --pack: the seat whose twin works the decision instead of the one the topic picks. |
| `--draft` | no | With --pack: a pack draft (Markdown file) to check for every part and cited claims. |

<!-- END GENERATED: skill-usage -->

To file the Decision itself, use its issue form or `aayu grill decision`. A Decision offers at least two numbered
options, three when it can, each `N. **Name.** what it does`; `aayu ready --check` refuses one with fewer. A person then
accepts one by its number or its name, for example `aayu decide 42 --accept 2`. A floor waiver is filed with two
options, 1. Waive and 2. Don't waive, and the waiver is never the default recommendation.

A Decision written before options were numbered, with lettered lines such as `A: in-house.` or named ones such as
`waive: …`, still works: its options read as 1 (A), 2 (B), and so on, and you accept one by its number, its letter,
or its name. `aayu ready --check` warns about it, so that it gets rewritten as numbered lines.

## What you get back

- **A decision pack:** a page with the question, cited facts, two or three options, a recommendation, the cost of
  waiting, a pre-mortem, and the questions that would change the answer. It is read-only.
- **A Decision issue:** while it is open, the items it constrains show as blocked in `aayu status`. Accepted, it is
  closed, and any Decision it supersedes is closed too.
- **A floor waiver:** `--waiver <floor>` files a Decision that is never reversible and never provisional.

The sweep, `aayu decide` with no number, pages the authority on any Decision open past `[decisions] sla_hours`
(24 business hours by default). After `provisional_after_hours` (48 by default), a reversible Decision's
recommendation applies provisionally, labelled `decision:provisional`, with a deadline to ratify or revert it.

**You should see** the Decision issue closed after `--accept`, and its blocked items free again in `aayu status`.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **decision-authority:** The actor holds [decisions] authority for the decision type or layer.
- **human-identity:** Runs only under a human identity.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- Only a person who holds `[decisions] authority` for its type or layer accepts it.
- It runs only under a person's identity, never the bot's.
- `--accept`: Option to accept, by its number or its name (human identity only).

<!-- END GENERATED: skill-human -->

The agent drafts and recommends; you decide. Run `aayu decide <n> --accept <option>` in your own terminal. Only
your `--accept waive` waives a floor, and the redaction floor cannot be waived.

## What's next

- Plan the work the Decision unblocks: [`plan`](plan.md).
- When a choice comes up while breaking work down: [the plan](../journey-2-break-down.md) and
  [we have to choose between options](../when-to-use-what.md#we-have-to-choose-between-options).
- [Every skill](README.md).
