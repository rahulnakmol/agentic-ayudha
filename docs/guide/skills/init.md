# `init`: set up a repository

Before aayudha can file and check work, your repository needs its labels, issue forms, config, and a place to keep
records. `init` adds them, shows what it will change, and asks before it uses your admin access.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Sets up a repository for aayudha: labels, issue forms, config, and the records branch; more on request. |
| Journey stage | [Set up](../start.md) |
| Accountable seat | [Founder](../../../seats/founder/CHARTER.md) (Prometheus) |
| Does the work | Prometheus, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Hercules) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor) |
| Layers it can touch | [L09 Platform / operations](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command |

<!-- END GENERATED: skill-card -->

## When to use it

- A new repository: `aayu tour` runs `init` for you, then installs the skills in your agent.
- You are at Level 1 and want the merge gate: `aayu init --gates`.
- You want agents to write as a bot, or baselines from your analytics: `--identity`, then `--integrations`.
- You want the gate to check what your product does: `--verify-skill` starts a feature map.

Setting up your agent rather than the repository is [`install`](install.md). Checking a setup that already exists
is [`doctor`](doctor.md).

## Run it

Run it yourself, in your own terminal, inside a clone of the repository. It is not a skill and never offered to
agents: it asks a yes on a terminal before it uses your admin access. Most people run it through the tour.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu init
```

| Input | Required | What it does |
|---|---|---|
| `--resume` | no | Resume a partial run from .aayudha/state.json. |
| `--projects` | no | Create Projects v2 views. |
| `--gates` | no | Install gate workflows. |
| `--identity` | no | Create the GitHub App bot identity. |
| `--integrations` | no | Configure external integrations. |
| `--verify-skill` | no | Write the product's verify skill: an empty feature map and a control tool in the product's stack. |

<!-- END GENERATED: skill-usage -->

The step up to Level 2 is `aayu init --gates`; see [adopt at your own pace](../adopt.md).

> **Needs setup:** `--identity` creates a GitHub App and stores its key as a secret. `--integrations` needs read-only
> tokens for the services you connect, such as PostHog or Sentry. Until a bot App exists, agents write as you,
> tagged as human fallback.

## What you get back

On GitHub: the `type:` and other labels, issue types on an organisation's repository, and the `ledger` branch, where
aayudha keeps its records, with its ruleset. In your checkout: the issue forms
(`.github/ISSUE_TEMPLATE/aayudha-*.yml`), `.aayudha/` with the config and its run state in `.aayudha/state.json`,
and one marked section in `AGENTS.md`.

**You should see** one line per change, a count of GitHub calls, and `Commit the written files … through a pull
request.` A run that stopped partway says to run `aayu init --resume`. Stuck? See
[tour problems](../troubleshooting.md#tour).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **tty-confirm:** Interactive confirmation on a TTY; refused for agents.

<!-- END GENERATED: skill-gates -->

Answer no, or run it from a script with no terminal, and it stops with exit 4 and changes nothing.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It is never offered to agents as an MCP tool.
- It asks a person to confirm in a terminal, and refuses agents.

<!-- END GENERATED: skill-human -->

Running it again changes nothing that is already in place. `aayu uninit` removes what `init` created, after the same
yes; external resources such as the bot App are listed for you to remove by hand.

## What's next

- File your first story: [`grill`](grill.md).
- The whole first run, step by step: [your first hour](../start.md).
- [Every skill](README.md).
