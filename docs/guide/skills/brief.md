# `brief`: explain the work to one audience

Good work that nobody understands does not get funded, adopted, or trusted. `brief` turns the record of one work item
into a short web page for one audience, where every claim cites its source and a model from another family looks for
claims the sources do not support.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Explains a piece of work to one audience in a short page, where every claim is checked against its sources. |
| Journey stage | [Explain it](../journey-9-explain.md) |
| Accountable seat | [Mentalist](../../../seats/mentalist/CHARTER.md) (Pythia) |
| Does the work | Pythia, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Metis) |
| Informed | [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Clio) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's model, plus a direct pass by aayu (adversarial pass direct) |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/brief/SKILL.md](../../../skills/brief/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- Your boss needs a one-page explanation in everyday words (`--register eli5`).
- A technical lead wants the components, floors, and values named exactly (`--register engineer`).
- Finished work needs a close-out against its target, once it has a Result (`--purpose close`).

Docs or release notes for everyone are [`tell`](tell.md). A roll-up across every Goal is [`status`](status.md).

## Run it

Your agent reads the grounding, writes a Markdown draft, and renders it with `--draft` until every check passes:

```text
Use the brief skill to write a close-out of issue 42 for the engineers, with register engineer and purpose close, and render it once every check passes.
```

> **Needs setup:** the checking pass calls a model directly, from a different family than the writer's. When aayu
> cannot resolve one, brief renders nothing. See [who calls models](../how-models.md#who-calls-models-from-the-binary).

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu brief <subject>
```

| Input | Required | What it does |
|---|---|---|
| `<subject>` | yes | Work item to brief on. |
| `--register` | no | Audience register. One of: `eli5`, `ste`, `engineer`, `regulatory`. |
| `--purpose` | no | Purpose of the brief. One of: `explain`, `ideate`, `innovate`, `close`. |
| `--draft` | no | Markdown draft to check and render; without it, print the grounding to write from. |

<!-- END GENERATED: skill-usage -->

The most common run: `aayu brief 42 --register engineer --purpose close --draft brief.md`.

## What you get back

One accessible HTML page with its sources in the footer, at `.aayudha/state/briefs/<issue>/<register>-<purpose>.html`
in your checkout. It is checked for alt text and with axe against WCAG 2.2 AA. Without `--draft`, you get the
grounding instead: every source you may cite, each with an ID.

**You should see** exit 0 and the path of the page. Exit 3 lists the checks that failed; exit 4 means a gate refused
or the reviewer found unsupported claims. See [exit codes](../troubleshooting.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **authoritative-pack:** An authoritative (CI-written) proof pack exists.
- **result:** A Result exists for the work item.

<!-- END GENERATED: skill-gates -->

They stop a `close` brief when the work item has no CI-written [proof pack](../../../GLOSSARY.md) or no Result yet.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

You pick the audience and the purpose, read the page, and decide whether to share it.

## What's next

- Write for everyone, in your brand's voice: [`tell`](tell.md).
- A brief refused, then accepted: [chapter 9, the telling](../journey-9-explain.md).
- [Every skill](README.md).
