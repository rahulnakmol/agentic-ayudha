# `release`: promote merged work to a snapshot or a stable release

Merged is not released: shipping puts stories on `dev`, where work integrates. `release` moves a Release issue's work
one rung up, `dev` to `rolling` or `rolling` to `main`, after a full gate on the exact commit that will land.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | ![release](../../../design/icons/verbs/release.svg) Moves shipped work up to a snapshot or stable release, after a full gate, and schedules the measurement. |
| Journey stage | [Release it](../journey-7-release.md) |
| Accountable seat | ![Nala](../../../design/icons/seats/devops-devex-tinkerer.svg) [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala) |
| Does the work | Nala, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Quality Architect](../../../seats/quality-architect/CHARTER.md) (Yama), [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona), [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Shesha) |
| Informed | [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti), [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Layers it can touch | [L09 Platform / operations](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/release/SKILL.md](../../../skills/release/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A Release issue (`type:release`) passes Ready and lists the stories, Bugs, or Features it ships.
- It is time for a `rolling` snapshot for early adopters: `--to rolling`.
- The Release already went to `rolling` and has a `vX.Y.Z` version: `--to main` for a stable release.

Work still on a pull request? Merge it first with [`ship`](ship.md).

## Run it

Your agent may run a release under your account, because the gate decides, not the prompt. CI and bot accounts are
refused. Promote on your release schedule or when someone asks for a release, never only because the gate is green:

```text
Use the release skill to promote Release 120 to rolling: dry-run it first, then run it if the gate is green.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu release <release>
```

| Input | Required | What it does |
|---|---|---|
| `<release>` | yes | Release issue. |
| `--to` | yes | Branch to promote to. One of: `rolling`, `main`. |
| `--at` | no | With --to rolling: promote this dev commit instead of dev's newest, such as the newest one with tier-2 eval results for every gating group. |

<!-- END GENERATED: skill-usage -->

Try it first with `aayu release 120 --to rolling --dry-run`: it shows the candidate, the tag, the stories it would
stamp, and the release notes, and pushes nothing.

> **Needs setup:** each rung dispatches your repository's `ci.yml` workflow on the candidate with a `gate` input.
> Without that workflow, `release` refuses and promotes nothing. See [releases](../how-releases.md).

## What you get back

The branch moved to the exact candidate commit and a tag: `rolling-<date>` (with `.n` for later snapshots that day)
or an annotated `vX.Y.Z` on `main`. Every shipped story stamped `released`, release notes that cite only proven
Features, and the Release issue's Shipping evidence section filled in. The Goals and Initiatives above those stories
become due for [`measure`](measure.md) 28 days later.

**You should see** the new tag and the stories labelled `released`. A failing gate is re-run once; if it stays red,
nothing is promoted and the message names the run. Exit codes: [troubleshooting](../troubleshooting.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **human-identity:** Runs only under a human identity.
- **stable-request:** Promoting to main needs a person's stable-release request: their comment "Stable release: \<version>" on the Release issue after its rolling snapshot, or their yes at a terminal.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

Before the candidate, it also reads your reliability targets when you have them: while an error budget is spent, only
Bugs ship ([`slo`](slo.md)).

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It runs only under a person's identity, never the bot's.
- A stable release to main waits for a person's request: their comment on the Release issue, or their yes at a terminal.

<!-- END GENERATED: skill-human -->

A stable release is a person's decision. After the rolling snapshot, comment on the Release issue with this first
line, from your own account:

```text
Stable release: v1.2.0
```

Agents never write that comment, and the pre-tool hook refuses a command that would post it. At your own terminal,
your yes at the prompt is the request. Never release to work around a red gate: fix the cause through `dev`.

## What's next

- After the release, find out whether the number moved: [`measure`](measure.md).
- See a real release, step by step: [the release](../journey-7-release.md).
- [Every skill](README.md).
