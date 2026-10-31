# rahulnakmol/agentic-ayudha#1191: aayu's loop agents carry AAYU_AGENT=1 only in their prompt, so a loop command without it reaches the device flow

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `4725e80bcbbc8751a3e11579be32dcfd83a46c33b808f87f1af73cd63faf1491` |
| Completion Goal hash | `b9da33011c81c2166aceb1b001b1bb3fd4ca0c5ed0270f76b81df0311097227f` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1191/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a scheduled firing of a loop runs `aayu hook loop:<name>` as its first step, with `AMP_THREAD_ID=T-loop` set; expected: the hook records T-loop as a loop lane in the machine's state before it prints its verdict, so before the agent reads any task content
2. [api-trace] state: the same thread then runs, without `AAYU_AGENT=1`, each of `aayu ready <feature>`, `aayu mission start <f>`, `aayu ready <f> --part intent`, `aayu ship <pr> --solo-bypass`, `aayu inspect <pr> --approve`, and `aayu inspect <pr> --dismiss <id>`; expected: each is refused for an agent lane (the loop lane), before any device code is shown, and nothing is written
3. [api-trace] state: the person's own Terminal tab in an orb where no loop ran, with `AMP_THREAD_ID=T-person` set; expected: `aayu ready <feature>` reaches the device flow and approves after the person's code, as Bug #1182 made it
4. [api-trace] state: `aayu install --loops --run-now <loop>`, and the `amp -ox` setup thread of `aayu install --loops --host amp --apply`; expected: each agent process aayu starts has `AAYU_AGENT=1` in its environment, not only in its prompt

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

aayu's loop agents carry `AAYU_AGENT=1` only in their prompt, so a loop command without it reaches the device flow. `AgentPrompt` writes `Run \`AAYU_AGENT=1 aayu <verb>\`` into the text the agent reads (`internal/loops/loops.go:682`). The host starts each scheduled firing: an Amp schedule or orb spawner, a Cursor automation, or a Codex scheduled task. The agent's process has only the host's marker, such as `AMP_THREAD_ID`. Since Bug #1182, `laneRefusal` refuses only `AAYU_AGENT=1` and CI. So a loop agent that runs `aayu ready <feature>` or `aayu mission start <f>` without the prefix starts a device flow, and it can show the code to a person. `ready --part`, `ship --solo-bypass`, and `inspect --approve` already have the same gap. CodeRabbit raised it on PR #1188 (4428671377), and @rahulnakmol accepted it as this follow-up.

### Steps to reproduce

1. Amp's schedule for the loop build-ready fires: a fresh thread with `AMP_THREAD_ID` set and no `AAYU_AGENT`.
2. The thread runs `aayu hook loop:build-ready`, which passes.
3. Steered by task content, the agent runs `aayu ready 874 --agents 10` without the prefix.

### Expected behaviour

Step 3 is refused for an agent lane, before any device code is shown, as for `AAYU_AGENT=1`.

### Actual behaviour

Step 3 prints a device code and waits for a person to enter it.

### Completion Goal

1. [api-trace] state: a scheduled firing of a loop runs `aayu hook loop:<name>` as its first step, with `AMP_THREAD_ID=T-loop` set; expected: the hook records T-loop as a loop lane in the machine's state before it prints its verdict, so before the agent reads any task content
2. [api-trace] state: the same thread then runs, without `AAYU_AGENT=1`, each of `aayu ready <feature>`, `aayu mission start <f>`, `aayu ready <f> --part intent`, `aayu ship <pr> --solo-bypass`, `aayu inspect <pr> --approve`, and `aayu inspect <pr> --dismiss <id>`; expected: each is refused for an agent lane (the loop lane), before any device code is shown, and nothing is written
3. [api-trace] state: the person's own Terminal tab in an orb where no loop ran, with `AMP_THREAD_ID=T-person` set; expected: `aayu ready <feature>` reaches the device flow and approves after the person's code, as Bug #1182 made it
4. [api-trace] state: `aayu install --loops --run-now <loop>`, and the `amp -ox` setup thread of `aayu install --loops --host amp --apply`; expected: each agent process aayu starts has `AAYU_AGENT=1` in its environment, not only in its prompt

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Fire a loop with a host marker, then run each presence-checked verb from that thread without the prefix; run the same verb from a person's Terminal in an orb where no loop ran; start a loop with --run-now and the Amp setup thread.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake device flow, and a fake agent CLI; never production

### Checkpoints

1. A loop thread is recorded by its first step, and every presence-checked verb from it is refused before a code (criteria 1, 2)
2. A person's Terminal still reaches the device flow, and the processes aayu starts carry the variable (criteria 3, 4)

### Technical Owner

engineering-manager

### Links

depends-on #1182

### Assumptions

#### Notes

The refusal reads one place: `laneRefusal` and `presenceCheck` (`internal/cli/presence.go`) treat a marker value that the loop hook recorded as an agent lane, the same as `AAYU_AGENT=1`. The record lives in the machine's state directory, so it covers the orb or machine where the loop ran. A person's Terminal tab in the orb of a loop thread inherits that thread's marker and is refused too: the person runs the step in an orb where no loop ran. Accepted gap: Cursor's marker `CURSOR_AGENT=1` carries no thread ID, so a Cursor loop is covered only by its prompt prefix. A Cursor automation that sets `AAYU_AGENT=1` in its environment, or a Cursor marker with a per-run ID, would close it. Setting `AAYU_AGENT=1` as an Amp project variable was not chosen: it would also refuse the person's own Terminal tab, the case Bug #1182 fixed.

#### Review guidance

- Check: criterion 1: the hook records the thread before it prints its verdict, and a failed record makes the hook exit non-zero, so the loop stops.
- Check: criterion 2: every caller of `presenceCheck` and `laneRefusal` reads the loop record; grep for both.
- Ignore: the wording of the refusal outside the lane name.
- Risk: a recorded marker value that a later person's shell reuses on the same machine; Amp thread IDs are unique, so only the loop's own orb is affected.

#### Behaviours

1. says: A loop thread never reaches the device flow, with or without the prompt's prefix; rule: never; source: criteria 1, 2
2. says: A person's own Terminal still approves with the device code; rule: limit; source: criteria 3
3. says: Each agent process aayu starts carries AAYU_AGENT=1 in its environment; rule: always; source: criteria 4

### Provenance

Request: "aayu's loop agents carry AAYU_AGENT=1 only in their prompt, so a loop command without it reaches the device flow"
1. stated (request): the loop's first step records the thread's marker before the agent reads any task content
2. stated (request): a later presence-checked verb in that thread is refused before any device code is shown
3. stated (request): the person's own Terminal tab in an orb where no loop ran still reaches the device flow
4. stated (request): --run-now and the setup thread set AAYU_AGENT=1 in the process env
````
