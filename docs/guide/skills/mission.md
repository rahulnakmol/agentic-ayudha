# `mission`

Starts the build of a Feature a person approved, and shows where its mission stands.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Starts a Feature a person approved, shows its mission, says what can start next, checks each milestone, lets a person stop it, and wakes a missed Kanha. |
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

Each mission has a time limit. Approve to build estimates each mission in working hours along its longest chain of
stories. Past `[mission] warn_hours` it warns. When the learned hours alone pass `[mission] max_hours`, only a
person starts it, with `aayu mission start <feature> --reason "…"`. The clock runs while at least one story thread
works. It pauses while every running story waits on a person, a usage limit, or GitHub CI. At `warn_hours` an
inbox card tells you. At `max_hours` new starts wait, and a person runs `aayu mission extend <feature> --reason "…"`.

Events wake the Kanha: a merge, a block, a failed start, an answered ask. Each `aayu mission next` lists every event
since the `events_cursor` in `missions/<feature>/plan.json`, so a Kanha that wakes 3 hours late misses none. The
cursor moves only after the list and its claims are written.

The hourly safety tick, `aayu mission tick`, catches a lost event. The `build-ready` loop runs it each hour, every
day. It wakes a Kanha only when its mission has a free agent and a story that can start, and no event reached that
Kanha in the last hour. Each wake is recorded in the plan, so a Kanha gets at most one wake a tick. A Kanha that
runs on your machine is not woken: it reads its events at its own next check. The tick also syncs each open cloud
pull request of Codex, Cursor, or Claude Code with `aayu build <n> --delegate <receiver>`, and the loop ships the
ones at merge-ready with `aayu ship`. The tick never builds a story: each run is a paid agent turn, 24 a day.

## Run it

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu mission <action> [<feature>]
```

| Input | Required | What it does |
|---|---|---|
| `<action>` | yes | start: a person starts the approved mission, through GitHub's device flow, and the plan records them, their tool for this repository, and the budget, then Start begins the mission's Kanha on that tool; given a Tatkal Feature's story, it starts that story on the person's own tool; refused for an agent lane. show: print the plan, each number with its source. next: list the mission's stories and Bugs that can start now, in order (priority, slice order, Ready time), and why each other one waits, with no model call; it claims each start on the ledger (missions/\<f>/claims.jsonl), so two runs never start one story, and only the mission's recorded tool runs it. It also lists every event since events_cursor in missions/\<f>/plan.json (each story's close and each line of its delegation history), and moves the cursor once the list and its claims are written. With --json it is an aayudha.work/v1 list: {schema, scope {kind, id}, items [{key, action, reason, subthread}], events [{at, story, event}]}; once a mission's clock reaches [mission] max_hours its stories wait. stop: a person stops the mission (or one story or Bug with --story), through GitHub's device flow and from any tool: mission next starts nothing and ship merges none of its PRs; each running Amp, Claude Code, or Cursor thread is told to commit at its next checkpoint and stop, and a thread aayu cannot reach is named; claims, branches, and PRs stay, and other missions and loops run on. resume: a person on the mission's recorded tool continues it (or one story with --story) and starts stories up to its agents. Each control is recorded in the plan's controls; both are refused for an agent lane. gate: run the gate of each milestone whose last story merged on the dev tip (the required checks, aayu verify --check all, and the evals of the seats the milestone touched; no model decides), mark it passed or failed with the dev SHA in missions/\<f>/plan.json, and list one fix per failed check, claimed in one agents slot; a check that failed for its runner reruns once; after [mission] max_no_go failed runs the mission stops and asks a person in aayu inbox. budget: a person raises the budget with a reason, or keeps a fix PR waiting. extend: a person, through GitHub's device flow, gives a mission that reached its time limit another [mission] max_hours, with --reason; refused for an agent lane. tick: the build-ready loop's hourly safety tick, with no model and no Feature. It wakes each started mission's cloud Kanha that has a free agent and a story that can start and was told nothing in the last hour, or a failed story whose retry came due since it was last told, at most once a tick, and records the wake in missions/\<f>/plan.json. Then it syncs each open cloud pull request of Codex, Cursor, or Claude Code once with aayu build \<n> --delegate \<receiver>, and lists those at merge-ready for aayu ship. It never builds, claims, or starts a story. recover: on the mission's recorded tool, and on each Kanha wake, read the record and each host, never the Kanha's memory: re-attach each story whose thread lives, keep each story with a PR on it, and restart each story whose agent is gone with no PR from its branch and brief, under a claim line, so no story has two live agents; a host it cannot read marks its stories unreadable: \<source> and keeps that source's cursor, and a start that got no answer stays maybe started until the host and GitHub show it did not start; a failed step retries after [mission] retry_backoff_minutes, each later retry after twice the wait, up to [mission] retry_max retries, then waits on the person who started the mission; one line per story, and a run that changes nothing writes nothing. retry: a person on the mission's recorded tool runs one more attempt of a story's or Bug's failed step (--story), counted and costed in the plan; a story with a PR syncs it again and never reruns a judged review; refused for an agent lane. One of: `start`, `show`, `next`, `gate`, `budget`, `stop`, `resume`, `extend`, `tick`, `recover`, `retry`. |
| `<feature>` | no | Feature whose mission to start, show, list, gate, budget, stop, resume, extend, recover, or retry; with start, also a Tatkal Feature's story to start; tick takes none. |
| `--story` | no | With stop or resume: only this story or Bug of the mission; the rest runs on. With retry: the story or Bug whose failed step runs once more. |
| `--reason` | no | With stop: why, in one line; it is recorded and shown wherever the stop is. With budget --to: why the budget rises. With start: why the mission starts although its learned working hours pass [mission] max_hours (needed then). With extend: why it may run another [mission] max_hours (needed). |
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
