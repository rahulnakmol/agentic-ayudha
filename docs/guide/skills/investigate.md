# `investigate`: find what broke in production

In an incident, the code on your branch can mislead you: what matters is the code that is actually deployed.
`investigate` works from a telemetry export to the deployed revision and the implicated code, weighs at least two
hypotheses, and proposes a catalog action for a person to approve.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Works out what went wrong in production from your telemetry and the code that was actually deployed, and suggests a fix for a person to approve. |
| Journey stage | [Keep it running](../when-to-use-what.md#something-broke-in-production) |
| Accountable seat | [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Atlas) |
| Does the work | Atlas, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Talos) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/investigate/SKILL.md](../../../skills/investigate/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- An Incident issue is open and nobody knows the cause yet.
- Errors started near a deploy, and you need to know whether the deploy caused them.
- The weekday `alert-triage` [loop](../guide-loops.md) lists an open incident with no investigation note.

Running the action and keeping the timeline is [`respond`](respond.md).

## Run it

Your agent runs bounded queries on the export, drafts the note, checks it, and posts it once you agree:

```text
Use the investigate skill on incident issue 42 with the telemetry export in ./telemetry: establish the deployed revision, weigh at least two hypotheses, and draft the investigation note for me to check before you post it.
```

> **Needs setup:** a sanitised OpenTelemetry export: `traces.jsonl`, `logs.jsonl`, and `deploys.jsonl`. Ask whoever
> has production access for one, aggregates first and no customer records. For a live system it does not cover, the
> agent writes a bounded query template for that person to run.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu investigate <incident>
```

| Input | Required | What it does |
|---|---|---|
| `<incident>` | yes | Incident issue. |
| `--telemetry` | yes | Sanitised OpenTelemetry export directory: traces.jsonl (OTLP JSON), logs.jsonl (JSON lines with trace_id), deploys.jsonl (deploy markers); read with no network. |
| `--service` | no | Service to scope every query to; default the only service in the export. |
| `--environment` | no | Deployment environment to scope to; default the deploy marker's. |
| `--revision` | no | Deployed revision; default the deploy marker serving the window, else service.version on the spans. |
| `--from` | no | Window start (RFC 3339, UTC); default an hour before the incident started. The window is at most 7 days. |
| `--to` | no | Window end, exclusive (RFC 3339, UTC); default the export's last record. |
| `--query` | no | Run one more bounded query and log it as qN: \<kind> [by f,…] [where k=v …] [limit N] [bucket 5m]. |
| `--draft` | no | Investigation note (Markdown) to check: every part, the stage's artefact, cited evidence, two hypotheses, bounded queries, and a mitigation only proposed from the catalog. |
| `--create` | no | Post the checked note on the incident, replaced in place, and add a timeline line (through the actions log); needs --draft. |

<!-- END GENERATED: skill-usage -->

The most common run: `aayu investigate 42 --telemetry ./telemetry --draft note.md --create`.

## What you get back

An investigation note on the incident, replaced in place on a rerun, and a new line in its timeline. It names the
deployed revision, cites every query as `qN`, weighs the hypotheses, proposes a Mitigation from the catalog, and
describes the Durable fix for the follow-up Bug.

**You should see** a comment on the incident titled for your team's stage: `Investigation note: …` at a startup,
`Incident assessment: …` at a scaleup. The `--draft` check exits 4 and names each problem to fix.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

The note only proposes, and never claims anything ran. You decide whether to post it, and a person approves any
mitigation it names, through [`respond`](respond.md) and its GitHub Environment.

## What's next

- Propose the mitigation and keep the timeline: [`respond`](respond.md).
- The whole flow: [something broke in production](../when-to-use-what.md#something-broke-in-production).
- [Every skill](README.md).
