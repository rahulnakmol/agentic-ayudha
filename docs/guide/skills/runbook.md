# `runbook`: write down how to fix a known failure

Under pressure, whoever is on call should follow written steps, not memory. `runbook` writes one entry per known
production failure: how to confirm it with a bounded query, which catalog action mitigates it, how to verify
recovery, and how to roll back.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Writes down how to spot and fix each known production problem with the approved actions, so whoever is on call can follow it. |
| Journey stage | [Keep it running](../when-to-use-what.md#something-broke-in-production) |
| Accountable seat | [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Shesha) |
| Does the work | Shesha, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul), [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/runbook/SKILL.md](../../../skills/runbook/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- An SLO has an alert and no runbook: `aayu runbook` names each one.
- An incident closed, and its investigation note describes a failure worth writing down.
- The catalog or the SLOs changed, and you want every runbook checked against them.

An incident in progress needs [`investigate`](investigate.md). Defining the SLO itself is [`slo`](slo.md).

## Run it

Your agent writes the entry from the alert and the last incident's investigation note, checks it, and writes it once
you agree:

```text
Use the runbook skill to write a runbook entry for the failure in incident issue 42's investigation note, check it against the catalog and the SLOs, and write it to ops/runbooks/ once it passes.
```

> **Needs setup:** an entry mitigates only through an action in `ops/actions.toml` and names an SLO from
> `ops/slos.toml`. With no fitting action, it says "no catalog action" and names the one a person should add.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu runbook
```

| Input | Required | What it does |
|---|---|---|
| `--draft` | no | Runbook entry (Markdown) to check: every part, the stage's artefact, cited evidence, an SLO named, a bounded confirm query, a catalog action, its rollback, and no executed change. |
| `--create` | no | Write the checked draft to ops/runbooks/\<name>.md in the checkout, for a pull request a person reviews; needs --draft. |

<!-- END GENERATED: skill-usage -->

The most common run: `aayu runbook --draft entry.md --create`.

## What you get back

An entry at `ops/runbooks/<name>.md` in your checkout, with six parts: Symptom, Confirm, Mitigate, Verify, Rollback,
and Tried. Point the SLO's `runbook` key at it. Without flags, `aayu runbook` checks every runbook against the
catalog and the SLOs.

**You should see** `aayu runbook` exit 0 with no findings once every alerting SLO has an entry. Exit 4 lists the
findings; see [exit codes](../troubleshooting.md).

> **Honest status:** aayudha has no production service, so its own repository has no runbooks to show.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

The `--draft` check also refuses an unbounded query, an action the catalog would refuse, a missing catalog rollback,
and a claimed execution. Release needs every rollback action in the catalog to have run once.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

A person opens and reviews the pull request that adds the entry, because `ops/` is a protected path. Every action it
names runs only when a person approves it, through [`respond`](respond.md).

## What's next

- Run the entry's action in an incident: [`respond`](respond.md).
- The whole flow: [something broke in production](../when-to-use-what.md#something-broke-in-production).
- [Every skill](README.md).
