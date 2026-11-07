# UX design: #917 GitHub Copilot as a first-class host

The person's surface is the terminal: `aayu models show`, `aayu build --delegate`, `aayu install`, and `aayu doctor`.
The agent's surface is the Copilot CLI session that a receiver starts. Nothing of this Feature is built; the
"today" rows are real output on `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Today (delivered building blocks)

`aayu models show --host copilot` (run in this checkout, 2026-11-07):

```text
LANE       HOST     MODEL                         FAMILY        SOURCE   ROUTE
build      copilot  claude-sonnet-5.5             anthropic     default  direct messages https://api.anthropic.com/v1 (key: anthropic)
spec       copilot  claude-sonnet-5.5             anthropic     default  direct messages https://api.anthropic.com/v1 (key: anthropic)
plan       copilot  claude-sonnet-5.5             anthropic     default  direct messages https://api.anthropic.com/v1 (key: anthropic)
mission    copilot  claude-sonnet-5.5             anthropic     default  direct messages https://api.anthropic.com/v1 (key: anthropic)
review     copilot  gpt-6.1-sol                   openai        default  direct responses https://api.openai.com/v1 (key: openai)
ui-review  copilot  gemini-4-argon (effort high)  google        default  copilot copilot-cli  (key: copilot)
chore      copilot  auto:efficiency               host-managed  default  host-managed (the host picks the model)
```

`aayu build --help` names the `--delegate` values. Copilot is not one of them:

```text
--delegate string   Hand the story to another agent's cloud agent with the delegation brief, or sync it: its PR,
                    inspect's findings, and follow-ups (auto picks the cheapest adequate connected one). OpenCode has
                    no cloud agent, so it always runs locally. One of: codex, cursor, claude, amp, opencode, auto.
--local             With --delegate: run the receiver's CLI on this machine in an aayu worktree, with no GitHub
                    credential; aayu commits, verifies, pushes, and opens the PR.
```

`aayu build <n> --delegate copilot` today:

```text
aayu: build: delegate "copilot": want one of codex, cursor, claude, amp, opencode, auto
  next: aayu build --help
```

## Planned (planned, not built)

| Step | The person types | What the person sees (from the issue) | Source |
|---|---|---|---|
| See lanes | `aayu models show --host copilot` | The Auto tier per lane: Balance for build, Intelligence for spec, plan, review, and UI review, Efficiency for loops. | #917 C1 |
| Override a lane | `[models.<lane>.copilot] model = "..."` in config | The named model in the same table, with source `config`. Inferred from the existing SOURCE column. | #917 C1 |
| Local build | `aayu build <n> --delegate copilot --local` | aayu runs `copilot -p` in the story worktree, then commits, verifies, pushes, and opens the PR, as for other local receivers. | #917 C3; `--local` help |
| Cloud build | `aayu build <n> --delegate copilot` | A cloud task starts on `dev`. A later sync picks up the PR. | #917 C4 |
| Install | `aayu install --host copilot` | The diff now includes `.github/workflows/copilot-setup-steps.yml`, and a line names the secret to set. | #917 C4 |
| Wrong plan | `aayu doctor`, `aayu build --delegate copilot` | A clear refusal that names the plan rule (Business or Enterprise only). The exact words are not stated. | #917 C4 (G5) |

The guide `docs/guide/hosts/copilot.md` gets the new commands. The issue asks for a "Lanes and models" section that says
why each tier is chosen, with the cost note (#917 C1). The issue does not name the guide file for that section.

## Agent-facing surface

- The receiver's prompt is the delegation brief (`internal/delegate/brief.go`), the same for every receiver
  (`internal/delegate/state.go` package comment: "One brief serves every receiver").
- The Copilot session gets the Copilot token only (#917 C3).
- The local Kanha path already passes `--allow-all-tools` (`internal/delegate/local.go`, `KanhaStart`). Whether the
  story receiver uses the same flag is not stated. Unverified.

## Accessibility

The output is plain text tables. The global `--plain` flag removes colour, motion, emoji, and box drawing
(`aayu --help`, global flags). The issue states no other accessibility need.
