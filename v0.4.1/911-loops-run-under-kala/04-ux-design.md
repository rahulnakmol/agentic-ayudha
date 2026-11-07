# UX design: #911 loops run under Kala

The person's surface is `aayu install --loops` in the terminal, the Kala thread in their agent tool, and a "Loops" row
in the inbox. The agent's surface is Kala's wake: `aayu loop --due` and one subthread per loop. Nothing of this
Feature is built.

## Today (delivered building blocks)

`aayu install --help` on [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) (the loop flags):

```text
--loops            Set up this repository's scheduled loops (loops/*.toml) in the one agent tool you pick from those
                   detected: Amp's schedules and Cursor's Terraform with --apply, a ready prompt per loop for Claude
                   Code and Codex; warns when another tool already runs them.
--record string    With --env --host: record a probe reply copied from the host's UI; with --loops --host: record loops
                   as set up there, <loop>=<ID>[,…] or all=<ID> (an empty ID forgets one).
--run-now string   With --loops: test one loop now, once, with the tool's own CLI in this checkout.
```

`aayu hook <name>` is the entry point for the loop guard, `aayu hook loop:<name>` (`internal/cli/loops.go`,
`LoopHookPrefix`). `aayu loop --due` does not exist: on 884d81b, `aayu loop --due` prints
`aayu: unknown flag: --due`.

What the founder sees today on Amp: one scheduled thread per loop, and a new thread for each run
(`internal/loops/loops.go`, `AmpSchedule`, `run_mode` `fresh`).

## Planned (planned, not built)

| Moment | The person or the agent | What happens (from #911) |
|---|---|---|
| Set up | The person runs `aayu install --loops` | One Kala thread opens in their tool. `loops/active.json` names them as owner, their tool, Kala, and each loop's subthread. |
| Each check | Kala wakes every `[loops] check_minutes` (default 15) | Kala runs `aayu loop --due` and gets the due list, with no model. |
| A loop is due | Kala | Kala wakes that loop's subthread with the loop's prompt. The run reports in that subthread. |
| First run of a loop | Kala | Kala creates the loop's subthread. Later runs reuse it. |
| Event loop fires | The host's trigger | The same subthread wakes (for example `triage-opened`). |
| Host with no subthreads | The adapter | It says so, and keeps one session per loop, resumed on each run. |
| Switch tools | The owner changes `[hosts] repos` and reruns install | Only when nothing of theirs is in flight. The loops move in the same step. |
| Owner leaves | Another person runs `aayu install --loops` | They become the owner. |
| See the loops | The person opens `aayu inbox` | The Team view shows a "Loops" row: each loop's last run, its result, and its age (#895). |
| Stop one loop | The person | Halt covers one loop (#911 Scope). The command is not named in the issue. |

The words "Kala" appear only in the docs. Every command, flag, label, and config key is common English (#911 Scope).

## Messages and errors

The issue does not give message text. The existing guard reasons (`loopGuard.Reason`, `internal/cli/loops.go`) are
the model for the due list's `reason` field. Inferred.

## Accessibility

The inbox is a terminal UI with a `--plain` mode (`aayu --help`, global flags). The issue states no other need.
