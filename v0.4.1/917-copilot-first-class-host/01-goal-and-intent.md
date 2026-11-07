# Goal and intent: #917 GitHub Copilot as a first-class host

No intent baseline exists for this Feature: the ledger has no `specs/917/` path (checked on `origin/ledger`,
2026-11-07). Every statement below comes from the Feature issue
[#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) and from the code on `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"GitHub Copilot becomes a first-class aayu host, on this machine and in the cloud, with Auto by default"
([#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917), Summary). The issue says it mirrors #893 (OpenCode).

## Who it serves and the customer outcome

- A team whose only AI tool is Copilot. The guide says "If Copilot is your team's only AI tool, aayudha runs inside it"
  (`docs/guide/hosts/copilot.md`, first paragraph).
- With this Feature, such a team can hand a Ready story to Copilot with `aayu build --delegate copilot`, locally or in
  the cloud, and Copilot can take part where #875 seat reviews and #889 missions say "every installed host"
  ([#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917), Problem).

## Problem

The issue names three problems ([#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917), Problem). Each is
checked against `dev`:

| Problem in the issue | What the code shows on 884d81b |
|---|---|
| No delegate adapter is registered for Copilot (`internal/delegate/state.go:23`). | True. `delegate.Receivers` is `{Codex, Cursor, Claude, Amp, OpenCode}` (`internal/delegate/state.go:29`). The `Copilot` constant exists only for a mission's local Kanha ("its story receivers are #917's", `internal/delegate/state.go:23-25`). |
| The token lookup falls back to `GH_TOKEN` and `GITHUB_TOKEN` (`internal/hosts/copilot.go:389`). | True. `hosts.CopilotToken` reads `COPILOT_GITHUB_TOKEN`, then `GH_TOKEN`, then `GITHUB_TOKEN` (`internal/hosts/copilot.go:389`). |
| Copilot lanes name fixed models. | Partly true. The build, spec, plan, and mission lanes name Claude Sonnet 5.5, and the review lane names GPT-6.1 Sol (`internal/verbs/registry.go:453-456`, `:520-527`). The chore lane already uses Auto Efficiency (`copilotAuto(CopilotEfficiency, "")`, `internal/verbs/registry.go:550`). |

The registry comment at `internal/verbs/registry.go:453-455` records why build uses a named model today: "Balance and
Intelligence picked claude-opus-5.5 for a hard prompt, at twice Sonnet's per-token price". C1 reverses that choice
on the founder's answer G1 (issue text). The answers G1 to G6 are on "the release 0.4.0 Copilot page"; that page is
not in this repository (`docs/` has no such file), so their full text is unverified.

## Success measures

The issue states no numeric measure. The observable outcomes in its Scope are:

1. `aayu models show --host copilot` shows the Auto tier per lane (C1).
2. A Copilot Auto review counts for merge on proof with no family check, and aayu records the model that served it (C2).
3. `aayu build --delegate copilot --local` runs `copilot -p` with only the Copilot token in its environment (C3).
4. `aayu build --delegate copilot` starts a cloud task on `dev`, and records the task's model and usage (C4).
5. On a plan other than Business or Enterprise, `aayu doctor` and `aayu build` refuse clearly (C4, G5).

## Out of scope

From [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917), Scope "Out":

- Assigning a story issue to the Copilot coding agent bot, because the bot would read the untrusted issue body as its
  brief.
- The Copilot code review bot as a reviewer.
- Copilot Pro and Pro+ plans.

Criteria that live in other Features, with no new Stories here: #889 ST13 (a local Copilot mission coordinator; built
in [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a)), #911 (Copilot runs loops), #892
(Copilot usage counts as host spend), and #875 (seat reviews include Copilot once C3 and C4 ship).
