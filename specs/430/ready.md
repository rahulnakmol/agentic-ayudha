# rahulnakmol/agentic-ayudha#430: Agents run release behind the full gate; main needs a person's stable-release request

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `1c063fd8a5d5b7eda88d8e3b4cbb3e0cf59035bf46ba3359744cecd7cc0be191` |
| Completion Goal hash | `51790695b93b1b24e342f28e392189417e82181bba3e4c6c84252184b8cfd8d8` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/430/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: an agent shell (`AAYU_AGENT=1`, no terminal) runs `aayu release <n> --to rolling` on a Ready Release whose `evals.yml` run and `ci.yml` gate are green; expected: it pushes the gated `Rolling snapshot: <ts>` merge commit itself to rolling, tags `rolling-<date>`, and the promotion record names the agent that ran it
2. [api-trace] state: the same agent run with the `ci.yml` gate red; expected: exit 4 saying the rolling gate failed, with nothing pushed to rolling and no tag
3. [api-trace] state: an agent runs `aayu release <n> --to main` with no stable-release request on the Release issue, then with a request written by a bot account, by an account without write access, and before the rolling snapshot; expected: each exits 4 naming the exact request form, with nothing pushed to main and no tag
4. [api-trace] state: a person with write access comments `Stable release: vX.Y.Z` on the Release issue after its rolling snapshot, then an agent runs `aayu release <n> --to main`; expected: it runs `ci.yml` with `gate=stable`, lands `Release: vX.Y.Z`, tags `vX.Y.Z`, waits for `release.yml`, and the promotion record names the request's author and URL
5. [api-trace] state: an agent's shell command that posts a `Stable release:` comment (`gh issue comment`, `gh api`, or `curl`) reaches the pre-tool hook in Claude Code, Codex, and Cursor; expected: the hook denies it and tells the agent to ask the person to post it
6. [api-trace] state: an agent runs `decide --accept`, `ship --solo-bypass`, and `plan --solo-bypass`; expected: each is still refused as a person's act with exit 4

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The founder's policy (AGENTS.md, "Promotion and release (no human approval)") is that an agent promotes on its own once the gate is green, but `aayu release` refuses agents and needs a terminal confirmation, and once #192 lands every host agent is refused. This Story lets an agent run `release` while keeping what makes it safe: the full `ci.yml` gate (with `evals-gate`), the merge commit names, the tags, and `release.yml`. Promoting to main still needs the explicit stable-release request the policy requires, written by a person on the Release issue in a form an agent cannot create through aayu. Accepting Decisions and the `ship` and `plan` solo bypasses stay human-only.

### Completion Goal

1. [api-trace] state: an agent shell (`AAYU_AGENT=1`, no terminal) runs `aayu release <n> --to rolling` on a Ready Release whose `evals.yml` run and `ci.yml` gate are green; expected: it pushes the gated `Rolling snapshot: <ts>` merge commit itself to rolling, tags `rolling-<date>`, and the promotion record names the agent that ran it
2. [api-trace] state: the same agent run with the `ci.yml` gate red; expected: exit 4 saying the rolling gate failed, with nothing pushed to rolling and no tag
3. [api-trace] state: an agent runs `aayu release <n> --to main` with no stable-release request on the Release issue, then with a request written by a bot account, by an account without write access, and before the rolling snapshot; expected: each exits 4 naming the exact request form, with nothing pushed to main and no tag
4. [api-trace] state: a person with write access comments `Stable release: vX.Y.Z` on the Release issue after its rolling snapshot, then an agent runs `aayu release <n> --to main`; expected: it runs `ci.yml` with `gate=stable`, lands `Release: vX.Y.Z`, tags `vX.Y.Z`, waits for `release.yml`, and the promotion record names the request's author and URL
5. [api-trace] state: an agent's shell command that posts a `Stable release:` comment (`gh issue comment`, `gh api`, or `curl`) reaches the pre-tool hook in Claude Code, Codex, and Cursor; expected: the hook denies it and tells the agent to ask the person to post it
6. [api-trace] state: an agent runs `decide --accept`, `ship --solo-bypass`, and `plan --solo-bypass`; expected: each is still refused as a person's act with exit 4

### Measure

Completion Goal criteria observed.

### Threshold

All 6 criteria.

### Scenario

1. Run `go test ./internal/cli -run 'TestRelease'`: the fake GitHub serves the Release, the workflows, and their runs; a scratch bare repository is origin. The agent runs set `AAYU_AGENT=1` and have no terminal.
2. Run `go test ./internal/hosts -run TestPluginHooks` for the pre-tool hook on each host's event shape.
3. Run the existing refusal tests for `decide --accept`, `ship --solo-bypass`, and `plan --solo-bypass`.

### Environment

A checkout of this repository on a branch from origin/dev, with Go and git; the fake GitHub server and scratch git repositories in the tests' temporary directories. No network, no real release, no production data.

### Checkpoints

1. An agent promotes to rolling behind the green gate and is refused on a red one, with the agent named in the record (criteria 1, 2)
2. Promoting to main needs a person's stable-release request, the hook stops an agent from writing one, and the other human-only acts stay human-only (criteria 3, 4, 5, 6)

### Provenance

Request: "Change release so that an agent may run it, keeping everything that makes it safe: it still runs the full gate (ci.yml gate=rolling or gate=stable, including evals-gate) and refuses anything that is not green; it keeps the release conventions: the non-fast-forward merge commit names, tags, and release.yml; --to main still needs the explicit stable-release request the policy requires, encoded so an agent cannot create it itself, for example a request or comment written by a non-bot human account on the Release issue; accepting Decisions and the solo bypasses (ship and plan --solo-bypass) stay human-only; the TTY gate goes for release, because agents have no TTY."
Vague: "safe" → criteria 1, 2, 3, 4, 5, 6
Vague: "fast" → criteria 1, 4
Vague: "stable" → criteria 3, 4
Term: "TTY" → an interactive terminal; "agents have no TTY"
1. stated (request): "an agent may run it" / "it still runs the full gate (ci.yml gate=rolling or gate=stable, including evals-gate)"
2. stated (request): "refuses anything that is not green"
3. stated (request): "--to main still needs the explicit stable-release request the policy requires" / "a non-bot human account on the Release issue"
4. stated (request): "a request or comment written by a non-bot human account on the Release issue" / "the non-fast-forward merge commit names, tags, and release.yml"
5. stated (request): "encoded so an agent cannot create it itself"
6. stated (request): "accepting Decisions and the solo bypasses (ship and plan --solo-bypass) stay human-only"

### Assumptions

- The stable-release request form is a comment on the Release issue whose first line is `Stable release: <the Release's Version>`, written after the rolling snapshot by a non-bot account with write, maintain, or admin access, and carrying no aayudha marker.
- A person at a terminal still confirms in the terminal, and that confirmation is their stable-release request; an agent, or any run without a terminal, needs the comment.
- In solo mode an agent writes with the person's token, so GitHub cannot tell who typed the comment: aayu writes no such comment, and the pre-tool hook refuses an agent's command that would. With a bot identity installed, agents write as the bot and a bot's comment never counts.
````
