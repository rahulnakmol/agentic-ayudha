# Technical design: #893 OpenCode as a host

No technical baseline exists (no `specs/893/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) has. Part 2 is the planned design from
[#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893). Part 2 is **planned, not built**.

## Layers touched

| Layer | Paths (Expected paths of #893) |
|---|---|
| L02 | `internal/cli/init.go`, `internal/cli/asked.go`, `internal/cli/install.go`, `internal/cli/installloops.go`, `internal/cli/verifymaintain.go`, `internal/cli/delegate.go` |
| L03 | `internal/verbs/registry.go`, `loops/**`, `internal/doctor/**`, `internal/agentenv/detect.go`, `internal/initrepo/**`, `internal/loops/**`, `internal/delegate/**` |
| L04 | `internal/verify/maintain.go` |
| L05 | `internal/models/**` |
| L06 | `internal/hosts/opencode.go`, `internal/hosts/hook.go`, `internal/hosts/install.go`, `hosts/opencode/**`, `hosts/README.md`, `hosts/commands/opencode/**` |
| L07 | `internal/config/config.go` |
| L09 | `.github/CODEOWNERS` |
| (no layer) | `docs/threat-model.md`, `docs/guide/hosts/opencode.md` |

## Part 1: what exists on dev

| Building block | Where | Notes |
|---|---|---|
| Host registry | `internal/verbs/registry.go:379-397` (`HostAmp` … `HostCopilot`, `Hosts`) | Five hosts. The pattern for a sixth. |
| Model lanes per host | `internal/verbs/registry.go` (`Lanes`, one `ModelDefault` per host) | A new host needs a row per lane, or a host-managed default. Inferred. |
| The last host added | [b53f60e2](https://github.com/rahulnakmol/agentic-ayudha/commit/b53f60e2) (#491, Copilot) | `internal/hosts/copilot.go` is the nearest template: skills dir, MCP entry, hooks, trust, live proof. |
| Install planner | `internal/hosts/install.go` (`Plan`, `hostFiles`, `SkillDirs`, `HostConfigOwners`, `ExecutableConfig`, `ourHook` regex `(amp\|cursor\|claude\|codex\|copilot)`) | Each host list is closed; OpenCode must be added to each. |
| Agent markers | `internal/hosts/hook.go:275-289` (`AgentMarkers`), `AgentMarker`, `HumanOnlyCommand` | Seven host markers for the five hosts (three for Codex) plus `AGENT` and `AI_AGENT`. |
| Loops | `internal/loops/loops.go:48` (`Hosts`); renders per host | No OpenCode render. |
| Maintainer | `internal/config/config.go:417` (`Maintainer`, enum `amp, claude, codex, cursor`); `internal/verify/maintain.go:36` (`Maintainers`) | The feature-map maintain brief @-mentions the app, or none for Amp. |
| Person's tool | `internal/config/config.go:74-90` (`Hosts.Repos`, `RepoTools`, `RepoTool`), [26537bd6](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd6) (#1194); used by `internal/cli/mission.go:721-731` | `RepoTools` comes from `verbs.Hosts`, so OpenCode cannot be the recorded tool for a mission. |
| Detection | `internal/agentenv/detect.go:237-248` | Finds `opencode`, its version, and sign-in (`opencode auth list`). From [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a). |
| Receiver | `internal/delegate/state.go:20-29` (`OpenCode`, `Receivers`), `internal/delegate/local.go` (`LocalBins`, `LocalArgs` → `run <prompt>`), `internal/cli/delegate.go:334-335` | Always local. From 80af4e1a. |
| Local Kanha | `internal/delegate/local.go` (`LocalKanha`, `KanhaStart` → `run --format json <brief>`, `KanhaResume` → `run --session <id> <event>`, `KanhaSession` reads `sessionID`) | From 80af4e1a. |
| Ready record | `internal/ready/approve.go:48` (`Parent.Host` comment lists opencode) | The mission record already allows the value. |

## Part 2: planned design (planned, not built)

1. **Host ID (893-A).** Add `HostOpenCode` to `verbs.Hosts`, with lane defaults. This also makes `[hosts] repos` accept
   it, because `RepoTools` reads `verbs.Hosts`. Generated docs change.
2. **Install (893-B).** `internal/hosts/opencode.go` with the `opencode.json` MCP entry per config version, skills in
   the folder OpenCode reads (the issue says `.agents/skills`), `hosts/opencode/**` templates, and entries in
   `HostConfigOwners`, `ExecutableConfig`, and the `ourHook` regex as needed. `.github/CODEOWNERS` gets the new
   executable config paths (an Expected path).
3. **Marker (893-C).** An `AgentMarkers` row for OpenCode. The variable name is unverified.
4. **Loops (893-D).** An OpenCode render in `internal/loops/`, `loops/opencode/`, and `aayu install --loops` support
   (`internal/cli/installloops.go`). #911 later replaces per-loop sessions with Kala.
5. **Maintainer (893-E).** Add the OpenCode value to the `Maintainer` enum and `verify.Maintainers`, with no @-mention
   (the Amp pattern). Inferred.
6. **Doctor and init (893-F).** Doctor checks for the new files; `aayu init` and `internal/initrepo/**` offer the host.
7. **Docs (893-G).** `docs/guide/hosts/opencode.md`, `hosts/README.md`, `docs/threat-model.md`.

## Decisions

- S24 of the core design, agreed 2026-10-21 (issue text). Its record is not in this repository. Unverified.
- The OpenCode receiver was built under #889 / Story #1012, although #893 put it out of scope. No Decision issue
  records that change (search of issues that mention #893, 2026-11-07).
