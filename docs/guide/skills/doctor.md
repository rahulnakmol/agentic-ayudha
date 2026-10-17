# `doctor`: find out what is wrong with your setup

Something fails and you cannot tell whether it is your machine, your sign-in, or your repository. `doctor` runs one
check per part of your setup and says what is wrong and how to fix it.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Checks your machine and setup, and says exactly what to fix. |
| Journey stage | [Set up](../start.md) |
| Accountable seat | [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala) |
| Does the work | Nala, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | none |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L09 Platform / operations](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/doctor/SKILL.md](../../../skills/doctor/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A command fails and you do not know why: start here before anything else.
- You upgraded aayudha and your agent's skill copies are out of date: `--fix` re-syncs them.
- You want to confirm a reviewer app, an agent environment, or a loop is set up.
- You look after several repositories: `--all` checks every one in `[fleet]`.

Not sure what to do next at all? `aayu next` names one step. Skills not installed yet? Use [`install`](install.md).

## Run it

Ask your agent, or run `aayu doctor` in your repository yourself.

```text
Use the doctor skill to check my setup and tell me what to fix first.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu doctor
```

| Input | Required | What it does |
|---|---|---|
| `--fix` | no | Re-sync copies and adapters after showing a diff. |
| `--all` | no | Iterate [fleet] repos. |

<!-- END GENERATED: skill-usage -->

After an upgrade: `aayu doctor --fix`.

## What you get back

One row per check, in a fixed order: `PASS`, `WARN`, `FAIL`, or `N/A`, the check's name, and what it found. The
checks cover the binary, `git`, `git-lfs`, your sign-in, the bot identity, the config, installed skills, the MCP
server, optional tools, agent environments, reviewer apps, loops, and existing CI. With `--fix`, it shows the diff,
then re-syncs the skill copies and `.aayudha/lock.toml`.

**You should see** `PASS` on every row you need. `WARN` and `N/A` rows are optional. A `FAIL` marked `[blocking]`
exits 3 until you fix it. See [install problems](../troubleshooting.md#install), or
[everything else](../troubleshooting.md#everything-else).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks no gate before it acts.

<!-- END GENERATED: skill-gates -->

Each row that is not `PASS` names the fix, such as `aayu doctor --fix` for skill copies that differ from the lock
file.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.

<!-- END GENERATED: skill-human -->

A person makes most fixes it names: installing a tool, signing in with `gh auth login`, or installing a reviewer app
on the account that owns the repository.

## What's next

- Install what it found missing: [`install`](install.md).
- The whole first run, step by step: [your first hour](../start.md).
- [Every skill](README.md).
