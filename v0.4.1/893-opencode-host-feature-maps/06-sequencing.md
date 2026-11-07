# Sequencing: #893 OpenCode as a host

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. File the Stories (`aayu slice 893`). A person approves the baselines and gives Approve to build
   ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018;
   [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
2. Check OpenCode's marker variable, hooks, and `opencode.json` versions against OpenCode's own docs. This is research,
   not code.
3. Decide what #893 needs from #871, or record that it needs nothing.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 893-A host ID | `internal/verbs/registry.go`, generated docs, `internal/models/**` if needed |
| 2 | 1 | 893-B install | `internal/hosts/opencode.go` (new), `internal/hosts/install.go`, `hosts/opencode/**`, `hosts/commands/opencode/**`, `.github/CODEOWNERS` |
| 2 | 2 | 893-C agent marker | `internal/hosts/hook.go`, `internal/cli/asked.go` |
| 2 | 3 | 893-D loop prompts | `internal/loops/**`, `loops/**`, `internal/cli/installloops.go` |
| 2 | 4 | 893-E maintainer value | `internal/config/config.go`, `internal/verify/maintain.go`, `internal/cli/verifymaintain.go` |
| 3 | 1 | 893-F doctor and init | `internal/doctor/**`, `internal/initrepo/**`, `internal/cli/init.go`, `internal/cli/install.go` |
| 3 | 2 | 893-G docs and threat model | `docs/guide/hosts/opencode.md`, `hosts/README.md`, `docs/threat-model.md` |

Order notes:

- 893-A comes first: every other slice needs `verbs.HostOpenCode`.
- In wave 2 the four lanes edit different files. 893-B and 893-C are both in `internal/hosts/`, but in different files
  (`install.go` and `opencode.go`, against `hook.go`). If 893-B needs the `ourHook` regex in `install.go` and 893-C
  needs nothing there, they stay apart.
- 893-D edits `internal/loops/loops.go` (`Hosts`). #911's adapters also edit it. Run 893-D before #911's wave 5
  (911-H OpenCode adapter), which needs it.

## Why fewer than 10 lanes

The Feature has seven slices. The host ID gates all others, and the widest wave has four lanes. The issue frames it as
one PR ("Its PR needs a person to ship"), so a smaller split is also valid.

## Shared hot files

- `internal/verbs/registry.go` (893-A): regenerates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, skill frontmatter,
  and `internal/mcp/schemas/*.json`.
- `internal/hosts/install.go` (893-B): also edited by #917 (917-E). Do not run both in one wave.
- `internal/loops/loops.go` (893-D): also edited by #911.
- `.github/CODEOWNERS` (893-B): protected.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| Every #893 PR | "Its PR needs a person to ship." | #893 Scope |
| 893-B | `.github/CODEOWNERS` is in `always_person` and protected; new host config paths join the protected list. | `.aayudha/config.toml:76`; `.github/CODEOWNERS` |
| 893-E | `internal/config/` is in `always_person`; `verify` is in `always_person_sections`. | `.aayudha/config.toml:83`; `internal/config/defaults.toml:70` |
| 893-G | The threat model changes. Inferred: a person reviews it. | #893 Expected paths |
| Live check in OpenCode | A person's OpenCode sign-in. | `internal/agentenv/detect.go` (`opencode auth login`) |
