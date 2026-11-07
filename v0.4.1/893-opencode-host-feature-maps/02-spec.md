# Specification: #893 OpenCode as a host

No spec baseline exists (no `specs/893/` on `origin/ledger`), and no Story is filed, so no Completion Goal criteria
exist. Requirements trace to [#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893) Scope and to a proposed
slice in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Functional requirements

| ID | Requirement | Proposed slice |
|---|---|---|
| FR1 | OpenCode has a host ID in the registry, so `--host opencode`, `aayu install --host opencode`, and `[hosts] repos = ["owner/name=opencode"]` are valid. | 893-A |
| FR2 | `internal/hosts/opencode.go` plans OpenCode's install: skills where OpenCode reads them, and the `opencode.json` MCP entry for each OpenCode config version. | 893-B |
| FR3 | The agent marker: aayu knows when a command runs inside OpenCode, so human-only verbs are refused there. | 893-C |
| FR4 | Loop prompts for OpenCode. | 893-D |
| FR5 | `[verify] maintainer` accepts the OpenCode value, so OpenCode can maintain the feature maps. | 893-E |
| FR6 | `aayu doctor` and `aayu init` know the host (Expected paths `internal/doctor/**`, `internal/cli/init.go`, `internal/initrepo/**`). | 893-F |
| FR7 | A host guide `docs/guide/hosts/opencode.md`, and a row in `hosts/README.md`. | 893-G |

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Security | Human-only verbs stay refused inside OpenCode. The marker is how aayu tells (`hosts.AgentMarker`; `HumanOnlyCommand`). | `internal/hosts/hook.go`; inferred for OpenCode |
| NFR2 | Security | "Its PR needs a person to ship." | #893 Scope |
| NFR3 | Security | `docs/threat-model.md` covers the new host (an Expected path). | #893 Expected paths |
| NFR4 | Reliability | Copies are real files, never symlinks, recorded with their hashes in `.aayudha/lock.toml`. | `hosts/README.md` |
| NFR5 | Reliability | Every hook runs `"<absolute path to aayu>" hook <name> --host <host>` and nothing else. | `hosts/README.md` |

## Points the sources do not settle

- The environment variable OpenCode sets in its commands (for FR3). Not in the repository. Unverified.
- Whether OpenCode has hooks, and their format. #893 does not list hooks in Scope, but lists `internal/hosts/hook.go`.
  Unverified.
- Which "config versions" of `opencode.json` exist (FR2). Unverified.
- How OpenCode maintains the feature maps without an @-mention. Amp has none either: "its scheduled loop thread reads
  the brief" (`internal/verify/maintain.go`, `Maintainers` comment). Inferred: OpenCode follows Amp's pattern.
