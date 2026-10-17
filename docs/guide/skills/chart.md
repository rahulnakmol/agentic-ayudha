# `chart`: design the data or AI behind a change

A dashboard nobody decides from, or an AI answer nobody can trace, is expensive to build and hard to undo. `chart`
interviews you before anyone plans the change, then posts one chart note on the issue: the decision it serves, the
data or evidence that answers it, and how to prove it right.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Before a data or AI change is planned, works out with you which decisions it serves, which data or evidence answers them, and how to prove it right. |
| Journey stage | [Break it down](../journey-2-break-down.md) |
| Accountable seat | [Data & AI Architect](../../../seats/data-ai-architect/CHARTER.md) (Mneme) |
| Does the work | Mneme, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Code Architect](../../../seats/code-architect/CHARTER.md) (Daedalus), [Revenue Architect](../../../seats/revenue-architect/CHARTER.md) (Plutus) |
| Informed | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Talos), [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Themis) |
| Layers it can touch | [L05 Intelligence, L07 Operational data / state, L08 Analytics / reporting](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/chart/SKILL.md](../../../skills/chart/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A Feature asks for a number, a dashboard, a pipeline, or a dataset: `--kind data`.
- A Feature asks for an answer, a retrieval, a model, or an agent: `--kind ai`.
- Someone has already picked a technology, such as a vector index, and you want to know what result would
  justify it.

An AI feature that also reports numbers gets two notes, one of each kind. Ready to lay out the build? Use
[`plan`](plan.md), which reads the note.

## Run it

Ask your agent with the issue's number and the kind. It asks two to four questions a round, riskiest first, each
with a default, then reads the design back to you in three lines before it posts.

```text
Use the chart skill on Feature 42 with kind data: work out with me which decision the dashboard serves before
anyone plans it.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu chart <issue>
```

| Input | Required | What it does |
|---|---|---|
| `<issue>` | yes | Feature, Initiative, Story, or Bug whose data or AI design to chart. |
| `--kind` | yes | data: decisions, metrics with grain and window, sources, platform and its trigger, retention and deletion; ai: simpler options, evidence and permissions, abstention, and an evaluation set. One of: `data`, `ai`. |
| `--draft` | no | Markdown chart note the host's model wrote; chart checks every part, the stage's artefact, and every citation. Without it, chart prints the grounding and the outline. |
| `--create` | no | Post the checked note on the issue as one comment per kind, replaced in place on a rerun (through the actions log); needs --draft. |

<!-- END GENERATED: skill-usage -->

The final step is `aayu chart 42 --kind data --draft note.md --create`.

## What you get back

One comment on the issue per kind, replaced in place when you run it again. A data note covers the decision, the
metrics with their grain and window, the smallest design that serves today's decision and the trigger for the next,
privacy and retention, and the checks. An AI note covers the question, simpler options, evidence and permissions,
when to abstain, and an evaluation set with adversarial cases. Its length follows your team stage: half a page at a
startup.

**You should see** a comment on the issue whose title names the note, such as `# Data sketch: …` at a startup. `plan`
then builds the story's checkpoints and tests from it.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

Before posting, `--draft` checks every part of the note and every citation. A claim the grounding does not show
is refused unless it is marked `Assumption:` or `Unverified:`.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

`chart` writes no code and touches no data. A design choice that changes what data is kept or who may read it is
yours: the agent offers a Decision draft, and you accept it with [`decide`](decide.md).

## What's next

- Plan the story from the note: [`plan`](plan.md).
- Where it fits in breaking work down: [the plan](../journey-2-break-down.md).
- [Every skill](README.md).
