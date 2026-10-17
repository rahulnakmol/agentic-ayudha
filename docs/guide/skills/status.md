# `status`: see where every Goal stands

Your boss asks how the Goal is going, and the answer is spread over dozens of issues. `status` rolls up progress,
risk, proof, open decisions, and cost for each Goal, Initiative, and Feature, with no model in the loop.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Rolls up progress, risk, proof, open decisions, and cost for each Goal, Initiative, and Feature. |
| Journey stage | [Explain it](../journey-9-explain.md) |
| Accountable seat | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Does the work | Drona, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | none |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/status/SKILL.md](../../../skills/status/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- You want one table of every open Goal and the work under it.
- You want to know what is Ready to build next, or what is blocked.
- You want to see the work in flight: worktrees, their stories and pull requests, and hand-offs (`--work`).
- You want each layer's autonomy streak from the merged pull requests (`--streak`).

Need a short page for one audience instead of a table? Use [`brief`](brief.md). Need evidence for an auditor? Use
[`audit`](audit.md).

## Run it

Ask your agent. It reads GitHub and the ledger and writes nothing unless you ask it to post.

```text
Use the status skill to roll up issue 42, and tell me what is blocked and what is Ready to build next.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu status [<issue>]
```

| Input | Required | What it does |
|---|---|---|
| `<issue>` | no | Goal, Initiative, or Feature to roll up; defaults to all Goals. |
| `--digest` | no | Send the digest through [notify]. |
| `--all` | no | Iterate [fleet] repos. |
| `--apply` | no | Post the replace-in-place Goal comments and close Features the parent-close rule allows; with --streak, record new corrections on the ledger. |
| `--streak` | no | Compute the autonomy streak of every layer from merged-PR history instead of the rollups. |
| `--work` | no | List the work in flight instead: aayu worktrees (agent, story, branch, PR, age) and stories delegated to cloud agents. |

<!-- END GENERATED: skill-usage -->

To post the rollup as a comment on each Goal: `aayu status --apply`.

## What you get back

A table with one row per item, children indented under their parent: `Item`, `Done`, `Ready`, `In progress`,
`Blocked`, `Proof`, and `Cost`. `aayu status --json` adds health, risk, decisions, and the streak per item. With
`--apply`, each Goal gets one comment that is replaced in place. A Feature is closed once every story under it is
released or cancelled and it has a Result.

**You should see** the table, or `no open Goals (label type:goal)` when there are none to roll up. Then pass a Goal,
Initiative, or Feature by number. Something off? See [everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks no gate before it acts.

<!-- END GENERATED: skill-gates -->

The **streak** counts the merges in a row on one layer with no human correction. It is computed from the merge
history, so no agent can change it ([the human pass](../concepts-human-pass.md)).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person acts on what the rollup shows. An open Decision waits for someone with the authority to accept it
([`decide`](decide.md)), and a blocked story waits for a person to look.

## What's next

- Hand an auditor a report they can reproduce: [`audit`](audit.md).
- See status, audit, and brief on a real day: [the telling](../journey-9-explain.md).
- [Every skill](README.md).
