# `respond`: handle a production incident

During an incident, a rushed command in production can make things worse. `respond` proposes one action from your
team's ops catalog, runs it only after a person approves it, keeps the timeline, and closes the incident with its
follow-up Bug.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Helps handle an incident: proposes a fix from your runbook, runs it only after a person approves, and keeps the timeline. |
| Journey stage | [Keep it running](../when-to-use-what.md#something-broke-in-production) |
| Accountable seat | [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Atlas) |
| Does the work | Atlas, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Talos), [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Hercules) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor), [Founder](../../../seats/founder/CHARTER.md) (Prometheus) |
| Layers it can touch | [L07 Operational data / state, L09 Platform / operations](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent |
| The skill your agent reads | [skills/respond/SKILL.md](../../../skills/respond/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- An Incident issue is open and its investigation note names a catalog action, such as a rollback.
- The impact is over and the blameless postmortem needs checking and posting (`--postmortem`).
- It is time to close the incident and file its follow-up Bug (`--close`).

Not sure what broke yet? Start with [`investigate`](investigate.md).

## Run it

respond writes under your own GitHub identity and is never offered to agents as an MCP tool, so you run it yourself,
in your own terminal. The skill helps you pick the action:

```text
Use the respond skill on incident issue 42: read the investigation note, tell me which catalog action to propose with which parameters, what verifies it, and what rolls it back.
```

> **Needs setup:** respond runs only actions in your `ops/actions.toml` on the default branch. Write your catalog
> ([ops/README.md](../../../ops/README.md) has the schema), merge it through a reviewed pull request, and give each
> human-approved action a GitHub Environment with required reviewers.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu respond <incident>
```

| Input | Required | What it does |
|---|---|---|
| `<incident>` | yes | Incident issue. |
| `--action` | no | Catalog action to propose. |
| `--params` | no | Action parameters as key=value pairs separated by commas. |
| `--postmortem` | no | Blameless postmortem (Markdown) to check against the incident, its timeline, and its investigation note, and post on the incident (replaced in place). |
| `--close` | no | Close the incident: file its linked Bug (carrying the investigation note's implicated code and durable fix) and the retro entry; a Sev0 or Sev1 incident needs its postmortem first. |

<!-- END GENERATED: skill-usage -->

With the example action from the schema: `aayu respond 42 --action restart-api --params service=api,replicas=2`.

## What you get back

A timeline on the incident, with each finished run and who approved it. `--postmortem` checks and posts the
postmortem. `--close` files the linked Bug, appends the retro entry to the [ledger](../../../GLOSSARY.md), and closes
the incident; a Sev0 or Sev1 incident needs its postmortem first.

**You should see** `awaiting a human reviewer's approval of environment …` with the run's link, then the run and its
approver in the timeline. A refusal exits 4 and names what is missing; see [exit codes](../troubleshooting.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **ops-catalog:** The action is in the protected ops catalog at a pinned SHA.
- **environment-approval:** Execution passes a GitHub Environment with required human reviewers.
- **human-identity:** Runs only under a human identity.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It is never offered to agents as an MCP tool.
- A person approves the GitHub Environment before anything runs in production.
- It runs only under a person's identity, never the bot's.

<!-- END GENERATED: skill-human -->

To approve, open the waiting `aayudha-ops.yml` run on GitHub and approve its environment as a required reviewer. A
person also decides when the impact is over, after the recovery query from the note.

## What's next

- Write the fix down for next time: [`runbook`](runbook.md).
- The whole flow: [something broke in production](../when-to-use-what.md#something-broke-in-production).
- [Every skill](README.md).
