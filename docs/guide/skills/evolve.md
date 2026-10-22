# `evolve`: stop the agents repeating a mistake

When an agent keeps making a mistake that only better guidance can fix, piling on another rule rarely helps. `evolve`
turns accepted retro Feedback into a failing test for the skill first, then a rewrite of the skill that passes it,
opened as a pull request a person reviews.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Turns accepted feedback into a test for a skill, then a skill change that passes it, for a person to review. |
| Journey stage | [Measure and learn](../journey-8-learn.md) |
| Accountable seat | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Does the work | Nakul, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama), [Data & AI Architect](../../../seats/data-ai-architect/CHARTER.md) (Shuka) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L05 Intelligence](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/evolve/SKILL.md](../../../skills/evolve/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- At least two accepted retro Feedback issues name the same skill, and each is classified as prose.
- The monthly `evolve` [loop](../guide-loops.md) reports a skill as ready.
- You want to see which skills have the evidence evolve needs: run it with no flags.

A mistake a type, an invariant, a lint rule, or a test could catch becomes a Story instead, through
[`retro`](retro.md) `--classify`.

## Run it

The first run lists accepted Feedback by skill. With `--skill` and `--feedback` it cuts a worktree; run again with the
case and the rewrite in place, it opens the pull request:

```text
Use the evolve skill to list the skills with accepted retro Feedback, then evolve the one that is ready, answering Feedback issue 42: add the failing eval case first, then rewrite the guidance.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu evolve
```

| Input | Required | What it does |
|---|---|---|
| `--skill` | no | Skill to evolve: skills/\<skill>/, or a stub under evals/skills/\<skill>/. |
| `--feedback` | no | Accepted Feedback issue filed by `aayu retro` that the change answers; without it, list the accepted retro Feedback. |

<!-- END GENERATED: skill-usage -->

The most common run: `aayu evolve --skill <skill> --feedback 42`.

## What you get back

An eval case at `evals/cases/<skill>/<name>/case.toml`, with the current skill's failing run under `before/` and the
new run under `output/`. A pull request from `evolve/<skill>-<n>`, labelled `agent-authored` and `aayu:evolve`,
citing every Feedback issue that names the skill.

**You should see** a pull request labelled `aayu:evolve` that changes one skill and adds the case that reproduces
the finding. A refusal exits 4 and names what is missing; see [exit codes](../troubleshooting.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **eval-first:** A failing eval case reproducing the finding exists before the skill diff.
- **codeowner-review:** A human CODEOWNER review on protected paths.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

Eval-first means the case fails on the current skill before any skill change, and the change rewrites the section
instead of only adding lines. A skill gets at most one evolve pull request in 30 days.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- A person who owns the changed paths (a CODEOWNER) reviews the change.

<!-- END GENERATED: skill-human -->

A person accepts each Feedback issue first, with the `needs:grill` label; the agent never accepts its own. A
CODEOWNER then reviews the pull request, because `skills/` is a protected path. `aayu install --pin` rolls a skill
back.

## What's next

- The next [`retro`](retro.md) reports, for 30 days, whether the problem came back.
- The whole loop: [stop repeating mistakes](../guide-learn.md#stop-repeating-mistakes-evolve).
- [Every skill](README.md).
