# Architecture

aayudha is one static Go binary, `aayudha` (alias `aayu`), plus a set of skills that it copies into your coding
host. The host's agent reads a skill and decides what to do. The binary does every deterministic check and every
write. GitHub holds the work, and an orphan `ledger` branch holds the record. This split exists so that the part
that reasons (a model) is never the part that certifies or writes.

![The host's agent reaches the aayu binary through a skill or the MCP server; the binary writes to GitHub, and CI workflows started by GitHub events write the authoritative pack to the ledger branch.](img/how-architecture.svg)

*The agent decides; the binary checks and writes; CI certifies. The binary also writes Ready snapshots and the
actions log to the ledger itself.*

## One binary, embedded assets

Ground rule 2 in [the build plan](../BUILD-PLAN.md) asks for one static binary built with `CGO_ENABLED=0`. The
release build (`.goreleaser.yaml`) and the cross-compile in the `build` job of `ci.yml` both set it. The only
required external tools are `git` and `git-lfs`, because go-git has no LFS or linked-worktree support and the
ledger needs both. `aayu doctor` reports everything else as optional.

The binary carries its own assets. The root package embeds four source trees exactly as committed:

```go
// Assets holds skills/, work-items/, hosts/, and seats/ exactly as committed.
//
//go:embed skills work-items hosts seats
var Assets embed.FS
```

*From `embed.go`.* `cmd/aayudha/main.go` passes `aayudha.Assets` to `cli.Run`, so `aayu install` copies skills
from the binary, not from a checkout. Other packages embed their own files the same way: the default config
(`internal/config/defaults.toml`), the MCP tool schemas (`internal/mcp/schemas/*.json`), the proof pack schema, and
the workflow templates that `aayu init --gates` installs (`internal/initrepo/*.yml`).

`install.sh` installs the binary as `aayudha` and links `aayu` to it (on Windows it copies `aayu.exe`).

## Four surfaces for one verb

The registry (`internal/verbs/registry.go`) declares 33 verbs and 15 plumbing commands. Each command lists the
surfaces it appears on:

- **CLI.** Every command. `internal/cli/root.go` builds one cobra command per registry entry at start-up, with
  positional arguments, flags, and validation taken from the entry's inputs.
- **Skill.** Every verb except `init` and `install`. `skills/<verb>/SKILL.md` has generated frontmatter and a
  hand-written body that tells the host's model which `aayu` calls to make.
- **MCP.** `aayu mcp` serves verbs as Model Context Protocol (MCP) tools over stdio. Read verbs are always exposed.
  Write verbs are exposed only when `[mcp] expose_writes = true` (default false). Human-only verbs (`ship`,
  `release`, `decide`, `respond`, `init`, `install`) are never exposed, and neither are human-only inputs such as
  `ship --solo-bypass`. The server runs each tool call in-process as an agent (`AAYU_AGENT=1`), so `--yes` and
  other human paths are refused.
- **Plumbing.** Commands that are not verbs: `config`, `auth`, `actions`, `models`, `mcp`, `hook`, `halt`,
  `resume`, and others. They have no skill and no lane.

See [the verb registry](how-registry.md) for how one Go file drives all four.

## Hosts

There are four primary hosts: Amp, Cursor, Claude Code, and OpenAI Codex (`verbs.Hosts`). `aayu install --host <host>` plans
the edits and `--apply` writes them. Skill copies are real files, never symlinks, and their hashes go in
`.aayudha/lock.toml`.

| What | Amp | Cursor | Claude Code |
|---|---|---|---|
| Skills | `.agents/skills/<verb>/` | `.agents/skills/<verb>/` | `.claude/skills/<verb>/` |
| MCP server | `amp.mcpServers` in `~/.config/amp/settings.json` | `.cursor/mcp.json` | `.mcp.json`, enabled in `.claude/settings.json` |
| Hooks | `.amp/plugins/aayudha.ts` | `.cursor/hooks.json` | `.claude/settings.json` |
| Agent guidance | `AGENTS.md` sections | `AGENTS.md` sections | `CLAUDE.md` stub that imports `@AGENTS.md` |

The paths are constants in `internal/hosts/install.go` (tabs shown as spaces):

```go
const (
    LockPath       = ".aayudha/lock.toml"
    AgentsSkills   = ".agents/skills" // Amp and Cursor
    ClaudeSkills   = ".claude/skills" // Claude Code
    AgentsMD       = "AGENTS.md"
    ClaudeMD       = "CLAUDE.md"
    ClaudeMCP      = ".mcp.json"
    ClaudeSettings = ".claude/settings.json"
    CursorMCP      = ".cursor/mcp.json"
    CursorHooks    = ".cursor/hooks.json"
    AmpPlugin      = ".amp/plugins/aayudha.ts"
    AmpSettings    = ".config/amp/settings.json" // relative to the user's home
    ServerName     = "aayudha"                   // MCP server name in every host
    LockVersion    = 1
)
```

Every hook runs `"<absolute path to aayu>" hook <name> --host <host>` and nothing else. There are two hooks.
`session-start` reports skill copies that differ from the lock file. `pre-tool` blocks agent tool calls (exit code
2) while the repository is halted or the budget breaker has paused agent lanes. The binary detects the host from
an environment variable the host sets (`AMP_THREAD_ID`, `CLAUDECODE`, `CURSOR_AGENT`, or `CURSOR_TRACE_ID`), or
from `--host`.

## Who reasons

Each command declares a model-calling mode. This is ground rule 5: inside a host, the host's model reasons, and the
binary calls a model itself only for work that must not depend on the host.

```go
// Model-calling modes.
const (
    ModeNone   Mode = "none"   // deterministic, no model
    ModeHost   Mode = "host"   // the host's model reasons inside the skill
    ModeDirect Mode = "direct" // the binary calls a model itself
    ModeHybrid Mode = "hybrid" // the host reasons, plus a direct pass by the binary
)
```

*From `internal/verbs/registry.go`.* In practice:

- **none:** `verify`, `ready`, `ship`, `release`, `status`, `audit`, `measure`, `baseline`, `proof`, `init`,
  `install`, `doctor`, and all plumbing.
- **host:** `grill`, `slice`, `plan`, `build`, `decide`, `research`, `wireframe`, `mockup`, `prototype`, `polish`,
  `experiment`, `readout`, `respond`, `retro`, `evolve`, and `tell`.
- **direct:** `inspect` with `[inspect] reviewer = "model"`; by default it reads an installed agent app's review and
  calls no model (Appendix B S-4.8c).
- **hybrid:** `review` (lane A direct), `brief` (adversarial pass direct), and `triage` (the host's model passes
  `--as`; never in product CI, Appendix B S-2.9b).

The fifth direct use is the eval judge, which is not a verb: `.github/workflows/evals.yml` runs it through
`internal/evals`. When the binary calls a model, it goes through the model gateway in `internal/models`. When the
current host's lane default is a host-managed mode (such as Amp medium), the binary uses the Claude Code defaults
instead (`verbs.DirectHost`). Reviewer lanes must use a different model family from the builder.

## The vagueness gate

A request in words, such as "make onboarding slick with nice transitions", means one thing to the person who says it
and another to the agent that builds it. The **vagueness gate** decides, before any skill acts on such a request,
whether it must be unpacked with the human first. It follows the split above: the binary decides the verdict and
supplies the facts, and the host's model talks with the human through the grill skill.

Every entry point that takes words runs the same deterministic call first: the `build`, `slice`, and `plan` skills,
the `/codex`, `/cursor`, `/claude`, and `/amp` hand-off commands, and the MCP `grill` and `ask` prompts. The call is
`aayu grill story --assess --idea "<words>" --json` (`feature` for slice). Its `gate` field comes from `GateFor` in
`internal/grill/gate.go`, with no model:

- **unpack** when the request has a vague qualifier from a catalogue (kind `vague`: "slick", "fast", "maybe"), an
  abbreviation or jargon term whose meaning the human must confirm (kind `term`: "PDP", "SSO"), or, with no
  qualifier, nothing observable (kind `outcome`: no number, quoted wording, route, command, or flag);
- **pass** otherwise.

The same assessment carries the facts the model works from: the product types found in the checkout (web, api, cli,
library), each term's usual meanings, and the catalogue's dimensions to ask about.

```go
type Gate struct {
    Verdict string       `json:"verdict"`
    Reasons []GateReason `json:"reasons"`
    Next    string       `json:"next"`
}
```

*From `internal/grill/gate.go`.* On `unpack`, the grill skill names each reason to the human, asks the riskiest
questions first, reads back "Here is what done looks like", and continues only after the human confirms. Nothing
enters the spec unconfirmed: each criterion's Provenance line says whether the human stated it, confirmed it, or
accepted a default, quoting their words. For a grilled item, `grill.Check` (run by `grill` and `ready`) requires
exactly one quoted source per criterion and every vague word resolved or deferred. On `pass`, the skill asks
nothing and drafts the work item from the request's own words, cited as `stated (request)`. An issue number skips
the gate, because that item was grilled when it was made Ready.

The CLI holds the same line whatever a host's model does. A verb that takes an issue, given words instead, fails
and points at the gate (from `internal/cli/root.go`):

```go
return nil, Errorf(ExitUsage, "%s: %s %q is a request in words, not an issue: unpack it into a Ready %s first with `aayu grill %s --assess --idea %s`, then run aayu %s on that %s",
    cmd, in.Name, clip(raw, 60), typ, typ, strconv.Quote(raw), cmd, typ)
```

`build --delegate` also refuses a story with no Ready snapshot, so a hand-off cannot skip the gate. For the same flow
from the user's side, see [How aayudha unpacks a vague ask](unpack.md).

> **Honest status:** "vague" cannot be decided by rule, only approximated, so the gate can ask about a request a
> person finds clear. The skill's side is measured by six judge-scored conversation cases that do not gate. On the
> last run (run 38841610907) the concrete request passed with no questions in all five runs, while the suite mean was
> 3.35, below the 3.5 bar.

## GitHub, the ledger, and CI

GitHub is the system of record. Work items are issues, changes are pull requests, and gates are check runs. Every
external write goes through `internal/actions`, which logs it in a hash-chained actions log, honours `--dry-run`,
and adds an idempotency key (ground rule 6).

The `ledger` branch is an orphan branch in the same repository. It holds proof packs (for example
`proof/131/<sha>/authoritative/pack.json`), Ready snapshots under `specs/`, actions-log segments, and audit
reports. It shares no history with `dev`, `rolling`, or `main` and is never merged. `internal/ledger` writes each
change as one commit built in a temporary worktree, under a per-clone file lock, and retries with fetch and
backoff when another writer pushed first.

CI runs the workflows that must not trust the author:

- `aayudha-merge-gate.yml` runs on `pull_request_target`, so the base branch's copy runs. The `gate` job re-runs
  `verify` on the head with no secrets; `inspect` reads an installed agent app's review and never runs pull request
  code; `pack` checks the pack and comments. No job holds an AI key.
- `aayudha-ready-guard.yml` strips `status:ready` when a Ready issue's body drifts from its snapshot.
- `aayudha-streak.yml` computes each layer's autonomy streak from merged-pull-request history.
- `aayudha-model-lanes.yml` runs `review` and `brief` by hand where the model key lives. It, `inspect-live.yml`,
  `models-live.yml`, and `ux-review-live.yml` check aayudha's own model code: never a required check, never a
  promotion gate, never installed into a product repository (Appendix B D-2.9b).
- `ci.yml` builds, tests, lints, and checks generated files; `evals.yml` runs tier-2 skill evals.

> **Honest status:** no bot GitHub App is installed on this repository yet, so the merge gate's `pack` job cannot
> write the ledger. The authoritative pack stays a run artifact, and `aayu ship` lands it under the founder's
> identity. The repository runs in solo mode (one human). v0.1.0 was released on 2026-10-15 (tag v0.1.0, commit
> 78bd4ae on `main`). No test checks that only the declared verbs import `internal/models`; today that holds by
> inspection (`inspect`, `review`, `brief`, and `triage` in `internal/cli`). Product CI never calls a model:
> `TestModelChecksNeverGateAndNeverShip` checks that no workflow `init --gates` installs names a model key.

## Invariants

1. Every command comes from the registry, and every verb has the CLI surface. Enforced in
   `internal/verbs/registry_test.go` (`TestRegistry`, subtest "surfaces follow exposure and kind").
2. A verb is on MCP exactly when its exposure is not `never`, and write verbs need `[mcp] expose_writes`.
   Enforced by `mcp.Exposed` in `internal/mcp/server.go`, with `TestRegistry` and `TestMCPDefaultExposure` in
   `internal/cli/mcp_test.go`.
3. Human-only inputs never reach MCP, and read-only pins (such as `ready --check`) hold while writes are off.
   Enforced in `internal/mcp/server.go` (`TestMCPDefaultExposure`, `TestMCPWritesAndDryRun`).
4. Skill copies match the lock file, or the host is told at session start. Enforced by `hosts.CheckCopies` in
   `internal/hosts/hook.go` (`TestInstallIntoExistingRepo`); `pre-tool` denies tool calls while agent lanes are
   paused (`TestHooks`).
5. Hooks run only `aayu hook <name>`. Enforced by `HookCommand` in `internal/hosts/install.go` and the `hook`
   rule of the skill lint (`TestSkillLint` in `embed_test.go`).
6. Every external write is logged and chained. Enforced in `internal/actions` (`TestChainDetectsTampering`,
   `TestDryRunPlansAndPerformsNothing`).
7. Concurrent ledger writers all land. Enforced in `internal/ledger/ledger.go` (`TestConcurrentWritersAllLand`).
8. The merge gate runs from the base branch and keeps pull request code away from secrets. Enforced by
   `pull_request_target` and job separation in `.github/workflows/aayudha-merge-gate.yml`.
9. A request in words gets a verdict without a model: unpack on a vague qualifier, an unconfirmed term, or nothing
   observable; pass otherwise. Enforced by `GateFor` in `internal/grill/gate.go` (`TestGate` in
   `internal/grill/grill_test.go`, `TestGrillAssess` in `internal/cli/grillassess_test.go`).
10. A verb that takes an issue, given words, fails and names the gate's command; the MCP `ask` prompt wraps the words
    as untrusted and embeds the verdict. Enforced in `internal/cli/root.go` (`TestGrillAssess`) and
    `internal/mcp/server.go` (`TestPromptsResourcesAndExitCodes`).
11. A recorded conversation obeys the verdict: on pass the skill asks nothing, on unpack its first turn asks before
    it writes. Enforced by `GateFloor` in `internal/evals/conversation.go`, run by `TestTier1` on the grill cases,
    such as `grill/bakery-concrete`.

## Where it lives in the code

- [embed.go](../../embed.go) and [cmd/aayudha](../../cmd/aayudha)
- [internal/cli](../../internal/cli): the cobra frame, handlers, and `aayu mcp`
- [internal/verbs/registry.go](../../internal/verbs/registry.go)
- [internal/mcp](../../internal/mcp)
- [internal/hosts](../../internal/hosts) and [hosts](../../hosts)
- [internal/grill/gate.go](../../internal/grill/gate.go): the vagueness gate
- [internal/models](../../internal/models)
- [internal/actions](../../internal/actions) and [internal/ledger](../../internal/ledger)
- [.github/workflows](../../.github/workflows)
- [docs/getting-started.md](../getting-started.md)

## Read next

- [The verb registry and code generation](how-registry.md)
- [The merge gate](how-merge-gate.md)
