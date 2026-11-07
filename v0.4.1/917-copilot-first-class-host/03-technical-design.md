# Technical design: #917 GitHub Copilot as a first-class host

No technical baseline exists (no `specs/917/` on `origin/ledger`). Part 1 lists what is on `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) that this Feature builds on. Part 2 is the
planned design from [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917). Part 2 is **planned, not built**.

## Layers touched

From the issue's Expected paths and `LAYERS.md`:

| Layer | Paths |
|---|---|
| L02 Interaction / API boundary | `internal/cli/delegate.go`, `internal/cli/models.go` |
| L03 Application / workflow | `internal/delegate/**`, `internal/verbs/registry.go` |
| L05 Intelligence | `internal/models/**` |
| L06 Contracts / integration | `internal/hosts/**`, `hosts/**` |
| L07 Operational data / state | `internal/config/defaults.toml` |
| L09 Platform / operations | `.github/workflows/copilot-setup-steps.yml` |
| (no layer) | `docs/guide/**` |

## Part 1: what exists on dev

| Building block | Where | What it does today |
|---|---|---|
| Copilot host ID | `internal/verbs/registry.go:385-387` (`HostCopilot`), `verbs.Hosts` | Copilot is one of five hosts. Added in [b53f60e2](https://github.com/rahulnakmol/agentic-ayudha/commit/b53f60e2) (#491), before v0.3.0. |
| Install files | `internal/hosts/copilot.go` (`CopilotHooks` = `.github/hooks/aayudha.json`, `CopilotSkills` = `.github/skills`, `CopilotMCPEnv`), `internal/hosts/install.go` | `aayu install --host copilot` writes skills, the MCP entry, and hooks. |
| Live proof | `internal/hosts/copilot.go:42` (`CopilotProven`), `.github/workflows/copilot-live.yml` | "live-proven with Copilot CLI 1.0.98 on 2026-10-17 … VS Code agent mode is not live-proven". |
| Session log and trailers | `internal/hosts/copilot.go` (`RecordCopilotSession` line 193, `CopilotSessionLog` 218, `ReadCopilotSession` 253, `CopilotModels.Trailers` 329, `CopilotFamilies` 350; `CopilotSession` = `.aayudha/state/copilot-session.json`) | Reads `<COPILOT_HOME or ~/.copilot>/session-state/<sessionId>/events.jsonl` and turns the models that served the session into trailers and families. C2 ("records the model that served each Copilot review") and C3 ("Trailers come from the session log") can build on it. |
| Token lookup | `internal/hosts/copilot.go:389` (`CopilotToken`) | Reads `COPILOT_GITHUB_TOKEN`, then `GH_TOKEN`, then `GITHUB_TOKEN`. C3 needs this to change for a receiver. |
| Auto tiers and selectors | `internal/verbs/registry.go` (`CopilotEfficiency`, `CopilotBalance`, `CopilotIntelligence`; `CopilotAutoBalance` = `auto:balance` and the others; `copilotAuto`) | The selectors exist. Only the chore lane uses one (`registry.go:550`). |
| Lane defaults | `internal/verbs/registry.go:453-456` (build), `:520-527` (review), `:541-542` (ui-review) | Build: Claude Sonnet 5.5. Review: GPT-6.1 Sol, chosen to differ from the builder's family. UI review: Gemini 4 Argon (high). |
| Copilot model route | `internal/models/copilot.go` (`CopilotRoute`) | The binary's own ui-review calls go through Copilot CLI when it is installed and signed in. |
| Local Kanha on Copilot | `internal/delegate/local.go` (`LocalBins`, `LocalKanha`, `KanhaStart`, `KanhaResume`, `NewSessionID`) | Starts `copilot -p <brief> --session-id=<uuid> --allow-all-tools`, and resumes with `--resume=<id>`. From [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125, Story #1012). |
| Receiver list | `internal/delegate/state.go:29` (`Receivers`), `internal/delegate/receivers.go` (`CursorAPI`, `AmpCLI`, `ClaudeCloud`, `CodexCloud`) | Four cloud adapters and a local path. No Copilot adapter. |
| Local receiver args | `internal/delegate/local.go` (`LocalArgs`) | A `switch` per receiver. No Copilot case. |
| Delegate CLI | `internal/cli/delegate.go`; flag enum in `internal/verbs/registry.go:762` | The `--delegate` enum is `codex, cursor, claude, amp, opencode, auto`. |
| Family rule | `internal/delegate/state.go` (`Family`) | Codex is OpenAI, Claude is Anthropic, every other receiver is host-managed unless the model names a family. |
| Hand-off time limit | [7239cb04](https://github.com/rahulnakmol/agentic-ayudha/commit/7239cb04) (#1136) | Every hand-off and follow-up ends after two minutes. |
| Plan doctor check | `internal/doctor/checks.go:788` | Doctor checks Copilot hooks. The evidence does not show a plan check. |

## Part 2: planned design (planned, not built)

### C1: Auto by default

- Change the Copilot rows of `builderDefaults()` and the review and ui-review lanes in `internal/verbs/registry.go` to
  `copilotAuto(...)` rows: Balance for build, Intelligence for design (spec, plan), review, and UI review, and
  Efficiency for loops (#917 C1). The registry is the source of generated docs, so `go generate ./...` reruns.
- Keep `[models.<lane>.copilot] model` as the override (#917 C1).
- The issue lists `internal/config/defaults.toml` in Expected paths. It does not say which key changes. Unverified.

### C2: review on Auto

- Record the model that served each Copilot review (#917 C2). Where it is stored is not stated. Inferred: next to the
  review's `Aayudha-Model` evidence that inspect reads.
- Add a Copilot exception to the family check (`models.CheckFamilies`, `internal/models/resolve.go:153`) for a
  Copilot Auto review only (#917 C2, G2).

### C3: local receiver

- Add `Copilot` to `delegate.Receivers` and a `LocalArgs` case that runs `copilot -p` in the worktree at the lane's
  tier (#917 C3). The Kanha's `KanhaResume` form shows that `-p` takes the prompt as its value and that a resume is
  `-p <text> --resume=<id>` (`internal/delegate/local.go`, comment on `KanhaResume`).
- Build the receiver's environment with the Copilot token only (#917 C3). This needs a token source that does not
  fall back to `GH_TOKEN` or `GITHUB_TOKEN` (`CopilotToken`).
- Read trailers (`Aayudha-Model` and others) from the session log (#917 C3).

### C4: cloud receiver

- Add a `CopilotCloud` adapter beside `CodexCloud` in `internal/delegate/receivers.go` with `Handoff`, `Status`, and
  `FollowUp` (pattern of the other adapters). It calls the REST tasks API on `dev`; a follow-up is a new task on the
  PR's head branch (#917 C4). The API's paths are not in the repository. Unverified.
- A live probe decides whether to send `model: "auto"` (#917 C4).
- `aayu install --host copilot` writes `.github/workflows/copilot-setup-steps.yml` and names the secret (#917 C4).
- `aayu doctor` and `aayu build` refuse a plan other than Business or Enterprise (#917 C4, G5).

## Contracts and records

- `delegate/<story>/state.json` on the ledger (`delegate.LedgerPath`) gains Copilot as a receiver. Its `Receiver` and
  `Family` fields are what inspect reads (`internal/delegate/state.go`, `State`).
- The spend record of C4 is shared with #892 ("Copilot CLI and cloud task usage count as Copilot host spend", #917
  Scope). Its format is not stated in either issue. Unverified.

## Decisions

- G1 to G6: founder answers on "the release 0.4.0 Copilot page" (#917 Summary). Not in this repository. Unverified.
- The exception in C2 changes the cross-family review rule in `AGENTS.md` for one host. No Decision issue for it was
  found (search of issues that mention #917, 2026-11-07).
