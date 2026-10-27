# `mission`

Starts the build of a Feature a person approved, and shows where its mission stands.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Starts a Feature a person approved, shows where its mission stands, says what can start next, checks each milestone, and lets a person stop it safely. |
| Journey stage | [Build it](../journey-4-build.md) |
| Accountable seat | [Engineering Manager](../../../seats/engineering-manager/CHARTER.md) (Drona) |
| Does the work | Drona, the Accountable seat's twin, in the [`mission`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun), [AI Native Engineer](../../../seats/ai-native-engineer/CHARTER.md) (Nakul) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Layers it can touch | [every layer](../concepts-layers.md) |
| Who reasons | no model: it is deterministic |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (read-only unless `[mcp] expose_writes` is on) |
| The skill your agent reads | [skills/mission/SKILL.md](../../../skills/mission/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

Starts the build of a Feature a person approved, and shows where its mission stands. The mission's Kanha runs
`aayu mission next <feature>` on each event and starts only the stories and Bugs it marks `start`. After each
merge it runs `aayu mission gate <feature>`, which proves each finished milestone on the dev tip with fixed checks
and lists one fix PR for each failed check.

A Ready story waits while an issue in its `depends-on` links is open: `waits #11: depends-on #10 is open`. It
starts after that issue closes. A target in another repository always counts as open.

`aayu mission start <feature>` also starts the mission's Kanha on the tool it records:

| Tool | The Kanha |
|---|---|
| Amp | The thread that drafted the Feature's intent gets the Kanha brief. When that thread is not known or is archived, a new thread starts. |
| Claude Code | A new cloud session (`claude --cloud`). |
| Cursor | One cloud agent. Each later event is a new run on that agent. |
| Codex, Copilot, OpenCode | A local session on your machine. While the machine is off, the mission pauses at the next merge. |

The Kanha is the `parent` in `specs/<feature>/approved.json`. Each story thread names its Feature and its Kanha in
`delegate/<n>/state.json`, and runs on the mission's tool, whatever `--delegate` asks. When the Kanha does not start,
run `aayu mission start <feature>` again: it starts only the Kanha.

In a Tatkal Feature, each person starts a story on their own tool: `aayu mission start <story>`. The plan records the
story's tool and starter, and the story is handed off at once. The Tatkal Kanha orders and tracks it.

## Run it

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu mission <action> <feature>
```

| Input | Required | What it does |
|---|---|---|
| `<action>` | yes | start: a person starts the approved mission, through GitHub's device flow, and the plan records them, their tool for this repository, and the budget; refused for an agent lane. show: print the plan, each number with its source. next: list the mission's stories and Bugs that can start now, in order (priority, slice order, Ready time), and why each other one waits, with no model call; it claims each start on the ledger (missions/\<f>/claims.jsonl), so two runs never start one story, and only the mission's recorded tool runs it. With --json it is an aayudha.work/v1 list: {schema, scope {kind, id}, items [{key, action, reason, subthread}]}. stop: a person stops the mission (or one story or Bug with --story), through GitHub's device flow and from any tool: mission next starts nothing and ship merges none of its PRs; each running Amp, Claude Code, or Cursor thread is told to commit at its next checkpoint and stop, and a thread aayu cannot reach is named; claims, branches, and PRs stay, and other missions and loops run on. resume: a person on the mission's recorded tool continues it (or one story with --story) and starts stories up to its agents. Each control is recorded in the plan's controls; both are refused for an agent lane. gate: run the gate of each milestone whose last story merged on the dev tip (the required checks, aayu verify --check all, and the evals of the seats the milestone touched; no model decides), mark it passed or failed with the dev SHA in missions/\<f>/plan.json, and list one fix per failed check, claimed in one agents slot; a check that failed for its runner reruns once; after [mission] max_no_go failed runs the mission stops and asks a person in aayu inbox. budget: a person raises the budget with a reason, or keeps a fix PR waiting. One of: `start`, `show`, `next`, `gate`, `budget`, `stop`, `resume`. |
| `<feature>` | yes | Feature whose mission to start, show, list, gate, budget, stop, or resume. |
| `--story` | no | With stop or resume: only this story or Bug of the mission; the rest runs on. |
| `--reason` | no | With stop: why, in one line; it is recorded and shown wherever the stop is. With budget --to: why the budget rises. |
| `--end` | no | With stop: end the mission, only by the person who started it. Merged stories stay merged; each open story and Bug loses its claims and goes back to status:ready, its open PRs close, the lock ends, and each locked issue gets one comment. |
| `--pr` | no | With gate: record this fix PR in the plan's gate_fixes, with the failed check its body names on a line `Aayudha-Gate-Fix: <milestone> <check>`; it files no issue. A fix that would pass the budget waits (exit 6), and an inbox card asks a person. |
| `--once-more` | no | With gate: run the stopped gate once more, after the presence check; refused for an agent lane. **Person only.** |
| `--to` | no | With budget: the new budget in USD, after the presence check. **Person only.** |
| `--keep` | no | With budget: keep this fix PR waiting, and close its inbox card. **Person only.** |

<!-- END GENERATED: skill-usage -->

## What you get back

mission-plan.

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **human-identity:** Runs only under a human identity.

<!-- END GENERATED: skill-gates -->

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It runs only under a person's identity, never the bot's.
- `--once-more`: With gate: run the stopped gate once more, after the presence check; refused for an agent lane.
- `--to`: With budget: the new budget in USD, after the presence check.
- `--keep`: With budget: keep this fix PR waiting, and close its inbox card.

<!-- END GENERATED: skill-human -->

## What's next

See [every skill](README.md).
