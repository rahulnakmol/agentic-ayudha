# `mission`

Starts the build of a Feature a person approved, and shows where its mission stands.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Starts the build of a Feature a person approved, shows where its mission stands, and says what can start next. |
| Journey stage | [Build it](../journey-4-build.md) |
| Accountable seat | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Does the work | Drona, the Accountable seat's twin, in the [`mission`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun), [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/mission/SKILL.md](../../../skills/mission/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

Starts the build of a Feature a person approved, and shows where its mission stands. The mission's Kanha runs
`aayu mission next <feature>` on each event and starts only the stories and Bugs it marks `start`.

A Ready story waits while an issue in its `depends-on` links is open: `waits #11: depends-on #10 is open`. It
starts after that issue closes. A target in another repository always counts as open.

## Run it

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu mission <action> <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<action>` | yes | start: a person starts the approved mission, through GitHub's device flow, and the plan records them, their tool for this repository, and the budget; refused for an agent lane. show: print the plan, each number with its source. next: list the mission's stories and Bugs that can start now, in order (priority, slice order, Ready time), and why each other one waits, with no model call; it claims each start on the ledger (missions/\<f>/claims.jsonl), so two runs never start one story, and only the mission's recorded tool runs it. With --json it is an aayudha.work/v1 list: {schema, scope {kind, id}, items [{key, action, reason, subthread}]}. One of: `start`, `show`, `next`. |
| `<feature>` | yes | Feature whose mission to start, show, or list. |

<!-- END GENERATED: skill-usage -->

## What you get back

mission-plan.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **human-identity:** Runs only under a human identity.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It runs only under a person's identity, never the bot's.

<!-- END GENERATED: skill-human -->

## What's next

See [every skill](README.md).
