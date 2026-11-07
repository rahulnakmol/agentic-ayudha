# UX design: Feature #874

The surface is the terminal (`aayu ready`, `aayu plan`), one marker comment per part on the Feature, and two inbox
cards from Feature [#1170](https://github.com/rahulnakmol/agentic-ayudha/issues/1170). Feature #874 has no UX
baseline: its footprint (L02, L03, L07 in `specs/874/design.json`) has no L01, so the `ux` part is "not needed"
(`partRun.pins` in `internal/cli/ready.go`).

## Commands and flags (delivered)

From the `ready` and `plan` entries in `internal/verbs/registry.go`:

| Command | Who | What it does |
|---|---|---|
| `aayu ready <f> --part <p> --request [--changes <file>] [--fixes <ids>]` | an agent | Drafts the part's next baseline on the ledger and asks a person to approve it; never approves. |
| `aayu ready <f> --part <p>` | a person | Approves the waiting draft through GitHub's device flow every time; refused for an agent lane. |
| `aayu ready <f> --check` | anyone | Read-only. On a Feature it adds a Baselines block with each part's drift. |
| `aayu ready <f> --early-build <story>` | a person | Approves one child story labelled `early-build` within `[baselines] early_build_budget` × `[autonomy.budget] cost_usd`; device flow every time. |
| `aayu plan <feature>` | Vishwa's twin | Reports the footprint and the stories; writes nothing. |
| `aayu plan <feature> --draft <file> [--create]` | Vishwa's twin | Checks the design draft; `--create` writes `specs/<f>/design.json`. |

`--part` takes `intent`, `spec`, `tech`, or `ux`. `--changes` is a JSON array of `{ref, what, why, need}`.
`--fixes` is a comma list of `<part>-v<k>/<seat>/<n>` IDs.

## Messages a person or agent sees (delivered, from the code)

| Moment | Text | Code |
|---|---|---|
| Draft written | "Intent v1 of #874 waits for a person: `aayu ready 874 --part intent`" | `ready.Waits` |
| Approved | "Intent v1 of #874 baselined by @rahulnakmol · hash 914c5edd93d5" (the real comment on #874, 2026-10-30) | `ready.Baselined` |
| Agent lane | "ready --part is a person's decision and is refused for an agent lane (AAYU_AGENT=1)" | `Env.personOnly` |
| No draft | "No spec draft of #874 waits for a person. An agent drafts it: aayu ready 874 --part spec --request" | `partRun.approve` |
| Moved while the code waited | "The intent of #874 changed after the draft (hash … → …). Nothing was baselined. Draft it again." | `partRun.changed` |
| Nothing changed | "Nothing changed since intent v1 of #874. No draft written." | `partRun.request` |
| Early build effect | "Approve the early build #916 of #874. Budget up to $60.00 (1 × [autonomy.budget] cost_usd)." | `ready.EarlyBuildEffect` |
| Early build approved | "@login approved the early build #… of #…. Budget up to $…. Its PR never merges into dev." | `runReadyEarly` |
| Early-build PR | "#916 is an early build: its PR never merges into dev (E1)" | `ready.EarlyBuildHold` |

A version after v1 posts a marker comment with a table "What changed since v&lt;k-1&gt;" (Change, Why it matters, Need);
agent text in the table is escaped (`ready.Comment`, `cell` in `internal/ready/drift.go`).

## Inbox cards (delivered under Feature #1170)

- Baseline card ([#1172](https://github.com/rahulnakmol/agentic-ayudha/issues/1172), PR
  [#1219](https://github.com/rahulnakmol/agentic-ayudha/issues/1219),
  [c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80)): one card per waiting draft, with the
  owner seat's counsel. Approving runs `partRun.approve` pinned to the card's hash, and one device code covers every
  card of the session (`internal/cli/inboxbaseline.go` header comment). An agent lane sees the cards and no approve
  action.

## Agent experience

- `skills/ready/SKILL.md`, section "A Feature's baselines: you draft, a person approves", tells the agent to draft
  with `--request` and never run `--part` without it, and to hand the person the command.
- `skills/plan/SKILL.md`, section "A Feature: write its technical design", gives the draft shape
  `{"feature", "contracts", "layers", "story_order", "rollback", "open_questions"}`.

## Pending UX (planned, not built)

- [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028): plan result lines such as "plan of #853: … 2
  high-risk changes (migration, contract), full path" and "no high-risk change, light path", and one refusal line
  per register gap (CG 1-5).
- [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106): the "Definition of Ready" line in
  `aayu ready <story> --check`, and the refusal "#1201 has 1 open question (#1210, due 14:00). Next: aayu ask
  --answer 1210" (CG 6, 7).
- Accessibility: the sources state no requirement for this Feature beyond plain terminal text.
