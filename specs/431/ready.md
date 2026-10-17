# rahulnakmol/agentic-ayudha#431: aayu inbox: every human-only step with the twin's recommendation, one keypress each

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `92c777857eeca38ba98e8ba6b1f85f6e6f7e9a644902e58e3866a2b47dad1674` |
| Completion Goal hash | `daf1b1a745eed3486739fa624ca3e90c9f50e42b1ecaaf76e369afb3e3288520` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/431/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu inbox` run with AAYU_AGENT=1, and run with no terminal on stdin; expected: each exits 4 with a message that inbox is human-only (or needs a terminal) and makes no GitHub write
2. [api-trace] state: a repository with an open floor-waiver Decision, a provisional reversible Decision, an open Story that another Story's body says it replaces ("Replaces #N"), an open Story whose PR merged into dev with no ship record, an open PR whose inspect comment records the 2-cycle cap, and an open Bug whose fix PR merged into dev; expected: `aayu inbox` on a terminal shows each of the six once, each with a recommendation and the keys accept, choose another, skip, and open
3. [api-trace] state: the person presses accept on every item of criterion 2; expected: the waiver Decision is accepted with its recommended option under the person's login, as `aayu decide --accept` records it; the replaced Story is closed as not planned with a comment naming its replacement; the merged Story moves to status:merged with the merged-without-proof comment; the Bug is closed as completed with a comment naming its fix PR; skip writes nothing, and every write is made with the person's token
4. [api-trace] state: the person presses choose another on a Decision and picks its other option; expected: the Decision is accepted with that option, and the provisional label is cleared on a provisional one
5. [screenshot] state: a repository with inbox items, and one with none; expected: the home screen shows a "Waiting in your inbox" row with the count and `aayu next` names `aayu inbox` as a step for a person; with none, neither mentions inbox
6. [api-trace] state: a host agent's shell command `aayu inbox`, sent to the pre-tool hook; expected: the hook blocks it as a person's command (exit 2)

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder working alone, I want one command that lists every human-only step waiting for me, each with the twin's recommendation, and lets me act on each with one keypress, so that I am not the bottleneck on human-only steps. The human-only gates themselves stay on: there is no switch that turns them off, because in solo mode an agent could flip one through the admin bypass. `aayu inbox` is human-only. It is refused for agents and needs a terminal, and every write it makes goes through aayu under the person's own identity. Today's backlog in this repository is Decision #413 (an eval-bar waiver) and the open Decisions #283, #287, and #324; Stories #331 to #334, each replaced by another Story; Stories #397, #399, and #400, whose PRs merged without a ship record; and PR #192, waiting at inspect's cycle cap.

### Completion Goal

1. [api-trace] state: `aayu inbox` run with AAYU_AGENT=1, and run with no terminal on stdin; expected: each exits 4 with a message that inbox is human-only (or needs a terminal) and makes no GitHub write
2. [api-trace] state: a repository with an open floor-waiver Decision, a provisional reversible Decision, an open Story that another Story's body says it replaces ("Replaces #N"), an open Story whose PR merged into dev with no ship record, an open PR whose inspect comment records the 2-cycle cap, and an open Bug whose fix PR merged into dev; expected: `aayu inbox` on a terminal shows each of the six once, each with a recommendation and the keys accept, choose another, skip, and open
3. [api-trace] state: the person presses accept on every item of criterion 2; expected: the waiver Decision is accepted with its recommended option under the person's login, as `aayu decide --accept` records it; the replaced Story is closed as not planned with a comment naming its replacement; the merged Story moves to status:merged with the merged-without-proof comment; the Bug is closed as completed with a comment naming its fix PR; skip writes nothing, and every write is made with the person's token
4. [api-trace] state: the person presses choose another on a Decision and picks its other option; expected: the Decision is accepted with that option, and the provisional label is cleared on a provisional one
5. [screenshot] state: a repository with inbox items, and one with none; expected: the home screen shows a "Waiting in your inbox" row with the count and `aayu next` names `aayu inbox` as a step for a person; with none, neither mentions inbox
6. [api-trace] state: a host agent's shell command `aayu inbox`, sent to the pre-tool hook; expected: the hook blocks it as a person's command (exit 2)

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Against the fake GitHub, seed one item of each kind in criterion 2 and run `aayu inbox` with a terminal and scripted keypresses: accept on each, then choose another and skip on fresh items, with AAYU_AGENT=1, and with no terminal. Read the writes the fake GitHub received and the token each was made with. Render the home screen and `aayu next` with and without items. Send `aayu inbox` to `aayu hook pre-tool` as a host's shell event.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake, never production

### Checkpoints

1. `go test ./internal/cli -run TestInbox` shows the refusals, the six items with their recommendations, and the writes of accept, choose another, and skip (criteria 1, 2, 3, 4)
2. the home screen and `aayu next` point to `aayu inbox` when it has items, and the pre-tool hook blocks it for agents (criteria 5, 6)

### Assumptions

- "Replaces #N" in an issue's body, in any state, marks open Story or Bug #N as replaced; the person confirms each close
- a Story's PR merged without a ship record when a PR merged into dev links it (a story/<n>- branch or Refs, Closes, Fixes, or Resolves #n) and the Story is still before status:merged; the same link marks a Bug's fix as merged
- the twin's recommendation for a Decision is the Decision's own Recommendation field; for the other kinds it is the Accountable seat's step for that state (close the replaced Story, mark the Story merged, close the Bug, review the capped PR)
- a PR at the cycle cap is reviewed by the person, so its recommendation is to open it; choose another runs `aayu ship` on it, which keeps every gate of ship
- open shows the item's URL and opens it in the browser when one is available

### Provenance

Request: "aayu inbox: one human-only command, refused for agents and needing a TTY, that lists every pending human-only step with the twin's recommendation and lets the person act with one keypress each (accept the recommended option, choose another, skip, or open it)"
Term: "TTY" → an interactive terminal on stdin, where the person presses the keys; "needing a TTY"
1. stated (request): "refused for agents and needing a TTY"
2. stated (request): "lists every pending human-only step with the twin's recommendation" / "Decisions awaiting acceptance, including eval-bar waivers such as #413" / "Stories replaced by another Story" / "Stories whose PR merged without a ship record" / "PRs waiting for a human merge because inspect hit its cycle cap" / "Bugs whose fix merged and that need closing"
3. stated (request): "Every write goes through aayu with the person's identity" / "add the capability inside inbox, human-only, rather than telling the person to use gh"
4. stated (request): "accept the recommended option, choose another, skip, or open it"
5. stated (request): "home and next should point to inbox when it has items"
6. stated (request): "refused for agents"
````
