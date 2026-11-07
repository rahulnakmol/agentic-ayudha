# Implementation plan: #893 OpenCode as a host

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #893. The code comment "#893 story F" (`internal/delegate/state.go:20-22`) names an unfiled Story. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 893. |

## Delivered

Nothing in #893's Scope is delivered. One commit built adjacent OpenCode support under #889:

| Commit | What it gives #893 |
|---|---|
| [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125, Story #1012) | `agentenv.Detect` finds OpenCode; `delegate.OpenCode` is a local-only receiver; OpenCode runs a mission's local Kanha. The receiver is out of #893's Scope, but it exists. |
| [26537bd6](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd6) (#1194) | `[hosts] repos`, whose allowed tools follow `verbs.Hosts`. Adding the host ID (893-A) makes OpenCode valid there. |

## In review

No open PR refs #893 (`_data/open-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The issue gives no effort and no
estimate.

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 893-A | OpenCode is a host ID, with lane defaults | #871 (what it needs is not stated) | L03 `internal/verbs/registry.go`; generated docs; L05 `internal/models/**` (if lanes need it) |
| 893-B | `aayu install --host opencode` writes skills and the `opencode.json` MCP entry for each config version | 893-A | L06 `internal/hosts/opencode.go` (new), `internal/hosts/install.go`, `hosts/opencode/**`, `hosts/commands/opencode/**`; L09 `.github/CODEOWNERS` |
| 893-C | aayu knows the OpenCode agent marker and refuses human-only verbs there | 893-A | L06 `internal/hosts/hook.go`; L02 `internal/cli/asked.go` |
| 893-D | Loop prompts for OpenCode | 893-A | L03 `internal/loops/**`, `loops/**`; L02 `internal/cli/installloops.go` |
| 893-E | `[verify] maintainer` accepts OpenCode, so it can maintain the feature maps | 893-A | L07 `internal/config/config.go`; L04 `internal/verify/maintain.go`; L02 `internal/cli/verifymaintain.go` |
| 893-F | `aayu doctor` and `aayu init` know OpenCode | 893-B | L03 `internal/doctor/**`, `internal/initrepo/**`; L02 `internal/cli/init.go`, `internal/cli/install.go` |
| 893-G | The OpenCode host guide, the hosts README row, and the threat model | 893-B, 893-C | `docs/guide/hosts/opencode.md` (new), `hosts/README.md`, `docs/threat-model.md` |

### Gaps the sources do not close

- OpenCode's agent marker variable, hook support, and `opencode.json` versions are not in the repository. 893-B and
  893-C each start with a check against OpenCode's own docs or source.
- The issue says "Its PR needs a person to ship" (one PR). This plan splits it into seven slices; each PR that touches
  the protected paths still needs a person.
