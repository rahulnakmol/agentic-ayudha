# `plan`: plan a story one layer at a time

An agent that starts coding without a plan touches more than it should, and nobody can review it until it is
done. `plan` writes one checkpoint per [layer](../concepts-layers.md) the story touches: what changes, the tests
that prove it, and how to roll it back, checked and kept on the ledger before any code exists.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Plans a story one layer at a time: what changes, how it is tested, and how to roll it back. |
| Journey stage | [Break it down](../journey-2-break-down.md) |
| Accountable seat | [Code Architect](../../../seats/code-architect/CHARTER.md) (Vishwa) |
| Does the work | Vishwa, the Accountable seat's twin, in the [`plan`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona), [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama), [Data & AI Architect](../../../seats/data-ai-architect/CHARTER.md) (Shuka), [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Shesha) |
| Informed | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/plan/SKILL.md](../../../skills/plan/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A story is sliced and you want its build order, tests, and rollback agreed before `build` starts.
- A story touches stored data, domain policy, or the platform, and a person must review the plan first.
- A data or AI story has a chart note: the plan builds its checkpoints from it.

The story is not written yet? Start with [`grill`](grill.md) or [`slice`](slice.md). A choice between designs
is [`decide`](decide.md).

## Run it

Ask your agent with the story's number. It reads the story's layers and criteria, drafts the plan, has `aayu` check
it, and records it once every problem is fixed.

```text
Use the plan skill to plan story 42, and tell me if any of its layers needs a person to review the plan.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu plan <story>
```

| Input | Required | What it does |
|---|---|---|
| `<story>` | yes | Story or bug to plan. |
| `--draft` | no | JSON file with the plan the host's model wrote (evals/schemas/plan.json); without it plan reports the footprint and criteria to plan. |
| `--create` | no | Write the validated plan to the ledger (specs/\<story>/plan.json) and the story; without it plan only validates. |
| `--approve` | no | Record the Engineering Manager's plan-time review of a plan that touches an [autonomy] never layer (human identity only). **Person only.** |
| `--solo-bypass` | no | Solo mode only (one human, no bot App): record that a never-layer plan proceeds with no plan-time review, after your story comment starting "Solo-mode bypass:"; a bypass, never an approval. **Person only.** |

<!-- END GENERATED: skill-usage -->

The final step is `aayu plan 42 --draft plan.json --create`.

## What you get back

A plan at `specs/<story>/plan.json` on the `ledger`, the branch where aayudha keeps its records. It holds one
checkpoint per layer in build order, each with its contracts, tests, instrumentation, the criteria it proves, and
its rollback; a stored-data checkpoint must name its down migration or restore step. When your product defines SLOs,
the plan also says how the change rolls out and what rolls it back.

**You should see** `aayu plan <story> --draft plan.json` report no problems, then the plan on the ledger. A risky
layer without a review stops `ship` later; see [ship problems](../troubleshooting.md#ship).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **validator:** The work item passes stage validation for its type.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

The story must pass the Ready check before it can be planned: when it does not, `plan` stops with exit 4 and
names the `aayu ready <story> --check` command that lists the problems.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- `--approve`: Record the Engineering Manager's plan-time review of a plan that touches an [autonomy] never layer (human identity only).
- `--solo-bypass`: Solo mode only (one human, no bot App): record that a never-layer plan proceeds with no plan-time review, after your story comment starting "Solo-mode bypass:"; a bypass, never an approval.

<!-- END GENERATED: skill-human -->

By default the risky layers are domain policy (L04), stored data (L07), and the platform (L09). A person in the
Engineering Manager [seat](../concepts-seats.md) reviews such a plan and runs `aayu plan <story> --approve`. Working
alone, post a story comment that starts `Solo-mode bypass:`, then run `--solo-bypass`; see
[the human pass and solo mode](../concepts-human-pass.md).

## What's next

- Build the story checkpoint by checkpoint: [`build`](build.md).
- See three risky plans recorded honestly in the story: [the plan](../journey-2-break-down.md).
- [Every skill](README.md).
