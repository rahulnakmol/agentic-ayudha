# `install`: put aayudha into your agent

Your agent cannot use a verb it has never been told about. `install` copies one skill per verb into your agent,
registers the MCP server and hooks, and, on request, sets up cloud environments and scheduled loops.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Installs the aayudha skills, tools, and hooks into your agent, and sets up its cloud environment and loops. |
| Journey stage | [Set up](../start.md) |
| Accountable seat | ![Nala, DevOps and DevEx Tinkerer: a bridge of floating stones](../../../design/icons/seats/devops-devex-tinkerer.svg) [DevOps & DevEx Tinkerer](../../../seats/devops-devex-tinkerer/CHARTER.md) (Nala) |
| Does the work | Nala, the Accountable seat's twin, in the [`chore`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | none |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Layers it can touch | [L09 Platform / operations](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command |

<!-- END GENERATED: skill-card -->

## When to use it

- You set up a repository and want your agent (Amp, Cursor, Claude Code, or Codex) to know the verbs: `--host`.
- You want another agent's cloud to build your stories: `--env`, then `--env --verify`.
- You want work to happen on a schedule without anyone asking: `--loops`.

First time in this repository? `aayu tour` runs `init` and `install` for you ([your first hour](../start.md)).
Skills out of sync after an upgrade? Use [`doctor`](doctor.md) with `--fix`.

## Run it

`install` runs only in your own terminal: it is not a skill and is never offered to agents. It asks a person to
confirm, and without `--apply` it only shows a diff of what it would write.

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu install
```

| Input | Required | What it does |
|---|---|---|
| `--host` | no | Host to install for. One of: `amp`, `cursor`, `claude`, `codex`, `copilot`, `all`. |
| `--apply` | no | Write the changes instead of showing a diff. |
| `--pin` | no | Skill version to pin. |
| `--env` | no | Set up agent environments instead: discover them from the code, ask what it cannot say, and show each host's generated configuration and UI values. |
| `--verify` | no | With --env: write the files and verify each host's environment in a clean container (setup cold, setup warm, the build and tests), fixing and retrying. |
| `--cloud` | no | With --env: prove the live cloud environment with a probe task where the host's CLI can (Amp, Codex) and record it. |
| `--record` | no | With --env --host: record a probe reply copied from the host's UI; with --loops --host: record loops as set up there, \<loop>=\<ID>[,…] or all=\<ID> (an empty ID forgets one). |
| `--loops` | no | Set up this repository's scheduled loops (loops/*.toml) in the one agent tool you pick from those detected: Amp's schedules and Cursor's Terraform with --apply, a ready prompt per loop for Claude Code and Codex; warns when another tool already runs them. |
| `--run-now` | no | With --loops: test one loop now, once, with the tool's own CLI in this checkout. **Person only.** |

<!-- END GENERATED: skill-usage -->

The most common run: `aayu install --host claude --apply`.

## What you get back

A skill copy per verb in your agent's skill folder, `.aayudha/lock.toml` recording the skills and their version, the
MCP server registered with your host, and its hooks. `--env` writes one setup script every host calls, and
`--loops` sets up the loop files in the one tool you pick, or prints the prompts to paste.

**You should see** new files to commit through a pull request, and `aayu doctor`'s `skills-copies` row as `PASS`.
Stuck? See [loop problems](../troubleshooting.md#loops) or [hand-off problems](../troubleshooting.md#hand-offs).

> **Honest status:** loops were set up live only on Amp. The Cursor, Claude Code, and Codex loop prompts follow each
> tool's documentation and have not been run on a real account yet ([loops](../guide-loops.md)).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **tty-confirm:** Interactive confirmation on a TTY; refused for agents.

<!-- END GENERATED: skill-gates -->

A change to files in your home directory waits for your yes on a terminal, or for `--yes`.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It is never offered to agents as an MCP tool.
- It asks a person to confirm in a terminal, and refuses agents.
- `--run-now`: With --loops: test one loop now, once, with the tool's own CLI in this checkout.

<!-- END GENERATED: skill-human -->

You run it and commit what it wrote. Commit the environment files too: a cloud agent sees only your default branch.

## What's next

- Check that everything landed: [`doctor`](doctor.md).
- The whole first run, step by step: [your first hour](../start.md).
- [Every skill](README.md).
