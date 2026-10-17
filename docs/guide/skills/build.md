# `build`: turn a Ready story into a pull request

A Ready story is a promise, not code, and one big unchecked change is hard to review. `build` writes the code in its
own worktree, one layer at a time, checks each step, and opens a pull request.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Writes the code for a Ready story in small checked steps, one layer at a time, and opens a pull request. |
| Journey stage | [Build it](../journey-4-build.md) |
| Accountable seat | [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Talos) |
| Does the work | Talos, the Accountable seat's twin, in the [`build`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Code Architect](../../../seats/code-architect/CHARTER.md) (Daedalus), [Production Engineer](../../../seats/production-engineer/CHARTER.md) (Atlas) |
| Informed | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Nestor) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill |
| Writes as | the bot identity (a GitHub App); without one, your identity, tagged as human fallback |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/build/SKILL.md](../../../skills/build/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A story or Bug is labelled `status:ready`, and you want it built.
- `inspect` said `fix` on the pull request, or you moved machines: `--resume` carries on after the last checkpoint.
- You want another agent's cloud to build it: `--delegate codex`, `cursor`, `claude`, `amp`, or `auto`.

Not Ready yet? Make it Ready first with [`ready`](ready.md).

## Run it

Ask your agent. It writes the code; `aayu` owns the worktree, the commits, the checks, and the pull request.

```text
Use the build skill to build issue 42, and tell me when it reaches each checkpoint I asked to see.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu build <issue>
```

| Input | Required | What it does |
|---|---|---|
| `<issue>` | yes | Ready story or bug to build. |
| `--resume` | no | Resume from the last Aayudha-Checkpoint trailer. |
| `--delegate` | no | Hand the story to another agent's cloud agent with the delegation brief, or sync it: its PR, inspect's findings, and follow-ups (auto picks the cheapest adequate connected one). One of: `codex`, `cursor`, `claude`, `amp`, `auto`. |
| `--local` | no | With --delegate: run the receiver's CLI on this machine in an aayu worktree, with no GitHub credential; aayu commits, verifies, pushes, and opens the PR. |
| `--asked` | no | The person's own words asking for this story. An agent needs them to pick up a Ready story that did not come out of grill in this session; without them build stops with a warning before taking the story. |

<!-- END GENERATED: skill-usage -->

> **Needs setup:** `aayu build 42 --delegate auto` needs the receiver's CLI, sign-in, and cloud environment. Run
> `aayu install --env --verify` once; see [hand work to an agent](../guide-build.md).

## What you get back

A lease on the story, so no other builder picks it up, and a worktree under `.aayudha/worktrees/<story>`. One
**checkpoint** commit per layer ([glossary](../../../GLOSSARY.md)), each with an `Aayudha-Checkpoint: k/N` trailer
and its own `verify` run, then a pushed branch and an open pull request.

**You should see** a pull request, and a comment on the story that reads like a flight log: checkpoint, layer,
result, and budget used. Stuck? See [Ready problems](../troubleshooting.md#ready) or
[hand-off problems](../troubleshooting.md#hand-offs).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **ready:** The issue is Ready and its body hash matches the Ready snapshot.
- **budget:** The [budget] and [autonomy] budget limits are not exceeded.
- **halt:** The repository is not halted (aayu halt).

<!-- END GENERATED: skill-gates -->

The default budget is 8 hours, 25 USD of model spend aayudha can see, and 3 failed attempts. Past it, the story
moves to `status:blocked` and a person is paged.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

The registry reserves no step of this verb for a person.

<!-- END GENERATED: skill-human -->

An agent builds only a story you asked for: otherwise it stops, asks you, and passes your words with `--asked`.
After a budget stop, you run `aayu ready <story>` to continue. Later, a person approves and runs [`ship`](ship.md).

## What's next

- The pull request is open; prove it: [`verify`](verify.md) and [`inspect`](inspect.md).
- The whole flow, here or in another agent's cloud: [hand work to an agent](../guide-build.md).
- [Every skill](README.md).
