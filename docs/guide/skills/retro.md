# `retro`: learn from how the work went

Plans miss and agents repeat corrections, and nobody has time to add it up by hand. `retro` compares plans with what
happened and files each lesson as a Feedback issue a person accepts or rejects.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Compares plans with what happened and files what to improve as feedback you can accept or reject. |
| Journey stage | [Measure and learn](../journey-8-learn.md) |
| Accountable seat | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor) |
| Does the work | Nestor, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Auditor](../../../seats/auditor/CHARTER.md) (Argus), [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Talos) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Prometheus) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/retro/SKILL.md](../../../skills/retro/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- Once a week, to see which stories ran over their Effort's budget, cost too much, merged without proof, or missed a
  [floor](../concepts-floors.md).
- The agents keep making the same correction, and you want it filed so it can be fixed for good.
- A Feedback issue needs its fix layer recorded: codebase, check, prose, or review (`--classify`).

Did the product's number move? That is [`measure`](measure.md). A better skill from prose Feedback is
[`evolve`](evolve.md).

## Run it

Run it in your agent, or let the weekly `retro` [loop](../guide-loops.md) run it. Then ask:

```text
Use the retro skill to run this week's retro, comment on each Feedback issue it files with what the numbers cannot say, and classify each one by the highest layer that can fix it for good.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu retro
```

| Input | Required | What it does |
|---|---|---|
| `--since` | no | Window start (YYYY-MM-DD UTC or RFC 3339); default the cursor of the newest retro report on the ledger, else 7 days back. |
| `--until` | no | Window end (RFC 3339, exclusive); default now. |
| `--ledger` | no | Ledger commit to read; default the ledger tip. |
| `--out` | no | Also write the report to this file. |
| `--replay` | no | Regenerate a report from the GitHub inputs snapshot at this ledger path (retro/\<date>/…inputs.json); files nothing. |
| `--classify` | no | Feedback issue to classify by the highest layer that can fix its correction for good (needs --layer and --reason); writes no report. |
| `--layer` | no | With --classify: codebase (a type, invariant, or boundary), check (a lint rule, test, or CI check), prose (skill or AGENTS.md guidance), or review (what is surfaced for human review). One of: `codebase`, `check`, `prose`, `review`. |
| `--reason` | no | With --classify: why this is the highest layer that can fix the correction for good. |
| `--not-catchable` | no | With --classify: why no type, invariant, lint rule, or test can catch it; required for prose and review, and lets evolve rewrite a skill for a codebase or check correction. |
| `--criterion` | no | With --classify and a layer other than prose: the Completion Goal criterion of the Story filed for it, `[evidence] state: …; expected: …`, whose proof is the new type, invariant, lint rule, test, or review surface. |

<!-- END GENERATED: skill-usage -->

To record a fix layer: `aayu retro --classify <n> --layer <layer> --reason "<why this layer>"`.

## What you get back

A report under `retro/<date>/` on the [ledger](../../../GLOSSARY.md), with its inputs in the header and flow numbers
that are reported, never filed. Each new finding becomes a Feedback issue titled `Feedback: retro: …`, labelled
`type:feedback`, `status:backlog`, and `intake`. retro never opens a pull request.

**You should see** one `Feedback: retro: …` issue per new finding, counted in the report's "Fix layer of Feedback"
table. A loop run stopped? See [loop problems](../troubleshooting.md#loops).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person accepts a Feedback issue with the `needs:grill` label, and can reclassify its fix layer by running
`--classify` again. The fix goes through a Story or [`evolve`](evolve.md), and a person reviews either one.

## What's next

- Turn accepted prose Feedback into a tested skill change: [`evolve`](evolve.md).
- The whole flow after a release: [did it work?](../guide-learn.md#learn-from-the-work-retro)
- [Every skill](README.md).
