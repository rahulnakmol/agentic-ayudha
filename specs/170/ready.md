# rahulnakmol/agentic-ayudha#170: Solo mode tells agents from people only by AAYU_AGENT=1, which skill shell calls do not set

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `e225f89fe50b087004884ceb5d196de740a1a2d9d3173a84f6ac3a8d594393f4` |
| Completion Goal hash | `f981f93e9bdb99912a867acaeffd9c3e25dce9e2be7653d31a530aabf76d41aa` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/170/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ship <pr> --solo-bypass` run from a skill's shell call in each host installed by `aayu install` (Amp, Cursor, Claude Code), without the caller setting `AAYU_AGENT`; expected: refused with exit 4 naming the agent lane, and nothing is merged or written
2. [api-trace] state: the same command run by a person in a terminal outside any host, with every other solo-mode condition met; expected: accepted as before
3. [api-trace] state: `plan --solo-bypass`, `decide --accept`, `release`, and `resume` run from a skill's shell call in each host; expected: each refused as an agent lane with exit 4

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

In solo mode (no bot App, no second paired human), the human-only paths, `ship --solo-bypass` and `plan --solo-bypass` among them, tell an agent lane from a person only by the environment variable `AAYU_AGENT=1` (`Env.Agent` in `internal/cli/env.go`). Only the MCP server sets it. An agent that runs `aayu` through its host's shell from a skill, with the founder's token, is treated as the founder, so nothing technical stops it from posting the "Solo-mode bypass:" comment and running `ship --solo-bypass` itself. The docs state this limit: docs/guide/how-trust.md (the honest-status note under "Solo mode") and docs/threat-model.md (T1).

### Steps to reproduce

1. In a scratch repository in solo mode (no `[identity]` bot App, no paired human), with the founder's token and a pull request whose merge gate is green and that carries a "Solo-mode bypass:" comment, install the skills with `aayu install --host claude --apply`.
2. From a Claude Code session, have the ship skill run `aayu ship <pr> --solo-bypass --dry-run` through the shell tool, without setting `AAYU_AGENT`.
3. Observe whether the agent-lane refusal appears.

### Expected behaviour

A call that comes from an agent host is recognised as an agent lane whether it arrives over MCP or through a skill's shell call, and every human-only path refuses it.

### Actual behaviour

Found by reading the code, not yet by a live run: only the MCP server (`internal/mcp/server.go`) and the rendered loops (`internal/loops/loops.go`) set `AAYU_AGENT=1`; the skills and hooks that `aayu install` writes do not. A shell call from a skill therefore has no marker, `Env.Agent` returns false, and the call is treated as the founder.

### Completion Goal

1. [api-trace] state: `aayu ship <pr> --solo-bypass` run from a skill's shell call in each host installed by `aayu install` (Amp, Cursor, Claude Code), without the caller setting `AAYU_AGENT`; expected: refused with exit 4 naming the agent lane, and nothing is merged or written
2. [api-trace] state: the same command run by a person in a terminal outside any host, with every other solo-mode condition met; expected: accepted as before
3. [api-trace] state: `plan --solo-bypass`, `decide --accept`, `release`, and `resume` run from a skill's shell call in each host; expected: each refused as an agent lane with exit 4

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Install the skills for each host in a scratch solo-mode repository, run each human-only command once from a skill's shell call and once from a plain terminal, and record the exit code and output of each run.

### Environment

local scratch repository with seeded, synthetic data and a fake GitHub server; never production
````
