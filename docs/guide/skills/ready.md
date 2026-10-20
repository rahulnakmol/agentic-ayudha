# `ready`: freeze what done means

If the promises can change after building starts, nobody can say whether the work is done. `ready` checks that a
work item is complete, then freezes exactly what was agreed, so any later edit is visible instead of silent.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | ![ready: a story meets the bar to start](../../../design/icons/verbs/ready.svg) Checks that a work item is complete and freezes exactly what was agreed, so later edits are visible. |
| Journey stage | [Get a story Ready](../journey-3-ready.md) |
| Accountable seat | ![Arjun, Product Strategist: an arrow in the centre of the target](../../../design/icons/seats/product-strategist.svg) [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Does the work | Arjun, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa), [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/ready/SKILL.md](../../../skills/ready/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A story or Bug is filed and you have read its promises: freeze it before anyone builds.
- You want to know what is missing before you commit: `--check` writes nothing.
- A Ready story lost its label after an edit, and you agree the new version.

Promises still vague or missing? Go back to [`grill`](grill.md). An XL story cannot pass: split it with
[`slice`](slice.md).

## Run it

Ask your agent to read the promises back to you first. It runs the read-only check, restates every criterion in
plain words, and applies Ready only after your yes.

```text
Use the ready skill on story 42: tell me what we are committing to before you make it Ready.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu ready <issue>
```

| Input | Required | What it does |
|---|---|---|
| `<issue>` | yes | Work item to make Ready. |
| `--check` | no | Validate only; write nothing. |
| `--guard` | no | Ready guard (the aayudha-ready-guard workflow): strip status:ready when the issue no longer validates or drifted from its snapshot. |

<!-- END GENERATED: skill-usage -->

Check first with `aayu ready 42 --check`, then apply with `aayu ready 42`.

## What you get back

The `status:ready` label and a **Ready snapshot**: a comment with two fingerprints, one of the whole issue and one
of the Completion Goal alone, also kept at `specs/<n>/ready.md` on the ledger. From then on the body is frozen. When
refused, nothing is written, and the agent names each problem by field and criterion with the smallest fix.

**You should see** `--check` exit cleanly, then the `status:ready` label and the snapshot comment on the issue.
Refused, or lost the label? See [Ready problems](../troubleshooting.md#ready).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **validator:** The work item passes stage validation for its type.

<!-- END GENERATED: skill-gates -->

The validator refuses a criterion with no evidence kind or nothing to observe, such as "feels fast". Once
`aayu init --gates` has added its workflows, the Ready guard removes `status:ready` when the body drifts from the
snapshot, and `build` refuses to start.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

Ready is a promise, so you say yes to it: read the promises and ask whether you would be happy if all of them came
true. To change scope later, edit the body and run `aayu ready <n>` again, or file a new story.

## What's next

- Build the Ready story: [`build`](build.md).
- See promises written and frozen in the story: [the promises](../journey-3-ready.md).
- [Every skill](README.md).
