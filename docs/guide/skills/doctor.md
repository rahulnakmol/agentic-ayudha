# `doctor`: find out what is wrong with your setup

Something fails and you cannot tell whether it is your machine, your sign-in, or your repository. `doctor` runs one
check per part of your setup and says what is wrong and how to fix it.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Checks your machine and setup, and says exactly what to fix. |
| Journey stage | [Set up](../start.md) |
| Accountable seat | ![Nala, DevOps and DevEx Tinkerer: a bridge of floating stones](../../../design/icons/seats/devops-devex-tinkerer.svg) [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala) |
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

**PostHog and Sentry.** When `[integrations.posthog]` or `[integrations.sentry]` is set, the `integrations` row
first checks that the key reads only, then tries each thing aayudha reads from the provider with one cheap read
(PostHog: a `SELECT 1` query and one alert; Sentry: the organization, its region, and the project, then one hour of sessions,
error counts, and issues, and one release and one alert). A Sentry token needs `org:read`, `event:read`,
`project:read`, and `alerts:read`; a PostHog personal API key needs `query:read` and `alert:read`. Each line says `works`, or `does not work` with the exact fix: the scope to add to the key, the other
region's URL, or the project or organization ID to set. The `provider-mcp` row reads every PostHog or Sentry MCP
server your hosts have registered and says how to make it read-only and pinned to one project. It fails when a
committed file holds a key.

```text
FAIL  integrations  1 of 1 configured integration(s) reachable with a read-only key
                      posthog: reachable as personal API key aayudha; read-only scope verified (query:read, insight:read per GET /api/personal_api_keys/@current/)
                        posthog, HogQL queries (query:read; used by measure, grill, experiment, readout): does not work: PostHog at https://eu.posthog.com has no project 12345. Fix: set project = to the numeric ID in Project settings → Project ID; if the project is on the other cloud, set url = "https://us.posthog.com"
```

> **Honest status:** the PostHog and Sentry checks are tested against answers shaped from each provider's published
> API specification, not against a live account. Your first `aayu doctor` with your own key is the first real run.
> If a line looks wrong for your account, open an issue with that line.

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
