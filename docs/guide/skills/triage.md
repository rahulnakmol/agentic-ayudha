# `triage`: sort incoming reports

Reports from users arrive as a mixed pile: ideas, bugs, and the occasional outage, all in the same form. `triage`
sorts each intake issue into Feedback, Bug, or Incident by label, and never edits what people wrote.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Sorts incoming reports into feedback, bugs, and incidents by label, without editing what people wrote. |
| Journey stage | [Keep it running](../when-to-use-what.md#something-broke-in-production) |
| Accountable seat | ![Arjun, Product Strategist: an arrow in the centre of the target](../../../design/icons/seats/product-strategist.svg) [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Does the work | Arjun, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | none |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | none: it changes no product code |
| Who reasons | your agent's model, plus a direct pass by aayu (the host's model passes --as; never in product CI) |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/triage/SKILL.md](../../../skills/triage/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- Reports arrive through an aayudha intake form and carry the `intake` label.
- You want each morning's new reports sorted before anyone reads them, by a loop.
- One report needs a class now, before it waits for the next loop run.

Want to write a Bug or an Incident yourself? Use [`grill`](grill.md) (`grill bug`, `grill incident`). Handling an
Incident once it is labelled is [`respond`](respond.md).

## Run it

Ask your agent. It lists the open intake issues, reads each title and body as untrusted data, and passes its choice
to `aayu` with `--as`.

```text
Use the triage skill to classify every open intake issue as feedback, bug, or incident, one line per issue.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu triage [<issue>]
```

| Input | Required | What it does |
|---|---|---|
| `<issue>` | no | Intake issue to classify; without it, list the open intake issues and change nothing (the weekly triage loop). |
| `--as` | no | Class chosen by the host's model; without it the binary asks the triage lane's model. One of: `feedback`, `bug`, `incident`. |

<!-- END GENERATED: skill-usage -->

For one report, the command is `aayu triage 42 --as bug`.

> **Needs setup:** without `--as`, `aayu` calls the triage lane's model itself, which needs a model key. Pass `--as`
> from your agent, or see [models and keys](../how-models.md).

## What you get back

Labels only. The issue gets `type:feedback`, `type:bug`, or `type:incident`, and loses `intake`. Feedback worth a
Feature also gets `needs:grill`; an Incident also gets `needs:bug`, for its follow-up Bug. With no issue number,
`aayu triage` lists the open intake issues, oldest first, and changes nothing.

**You should see** a line with the class and the labels added and removed. An issue without the `intake` label is
refused with exit 4. Running it as a loop? See [loop problems](../troubleshooting.md#loops).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **labels-only:** Writes are limited to labels by the actions capability.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

Labels-only means the write itself is limited: `triage` cannot change a title, a body, or a comment, even if a
report asks it to.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person comes in after the label: turning accepted Feedback into a Feature with [`grill`](grill.md), and approving
any action on an Incident in [`respond`](respond.md).

## What's next

- Handle an Incident: [`respond`](respond.md).
- Run triage every weekday without asking: [keep things moving with loops](../guide-loops.md).
- [Every skill](README.md).
