# #888 Technical design

No tech baseline exists (`specs/888` is absent from the ledger). Everything under "Planned" is planned, not built.
It comes from the Scope and Expected paths of [#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888).

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| Respond verb | `runRespond` (`internal/cli/respond.go:142`) | Shows the ops catalog at the default branch's head and the timeline; `--action` proposes; `--postmortem` checks and posts; `--close` files the linked Bug. |
| Propose | `propose` (`internal/cli/respond.go:251`) | Validates a catalog action and dispatches `aayudha-ops.yml`. Calls `requireReviewers` (`:270`). |
| Reviewer check | `requireReviewers` (`internal/cli/respond.go:314-339`) | Refuses an environment with no `required_reviewers` rule, or with no deployment branch policy. |
| Rate limit | `rateLimit` (`internal/cli/respond.go:341`) | Counts the action's runs of `aayudha-ops.yml` in the last hour. |
| Human approval read | `runApprovals`, `humanApproval` (`internal/cli/respond.go:384-411`) | Reads who approved the environment on a run; ignores bots. |
| CI execution | `respondExec` (`internal/cli/respond.go:417`) | Inside `aayudha-ops.yml` (`--ci`), re-verifies the catalog, `requireReviewers` (`:479`), and a human approval (`humanApproval` at `:482`, refusal at `:486`), then runs the action. |
| Ops workflow | `internal/initrepo/aayudha-ops.yml`; catalog `ops/actions.toml`, parsed by `internal/ops/ops.go` | The workflow where a human approves the environment. |
| Presence check | `Env.presenceCheck` (`internal/cli/presence.go:136`) | GitHub's device flow for a human-only step in a shell with an agent marker. |
| Inbox kinds | constants `InboxDecision` to `InboxEvening` (`internal/cli/inbox.go:46-62`); bc78349 adds `InboxOvernight` (`:63`) | No incident kind. |
| Registry entry | `Name: "respond"` in `internal/verbs/registry.go` | `Identity: IdentityHuman`, `Layers: OnlyLayers(L07, L09)`, `Exposure: ExposeNever`. |

Not on `dev`: a `--approve` flag on `respond`, any pending-deployments API call (no `PendingDeployments` or
`pending_deployments` in `internal/`), and `internal/needs` (the Need registry of #873).

## Layers touched

From `LAYERS.md`: L02 (`internal/cli/respond.go`, `inbox.go`), L03 (`internal/verbs/registry.go`), and L07 only if a
key is added under `internal/config/**`. `.aayudha/config.toml` (an Expected path of #888) and `docs/threat-model.md`
match no layer glob. `internal/needs/**` would need a layer glob (`TestEveryInternalPackageInALayer`,
`internal/verbs/gen/main_test.go:20`).

## Planned, not built

1. **Approve flag (P1).** `aayu respond <incident> --approve <run>`: run `requireReviewers` for the run's environment;
   run the presence check; then call GitHub's pending-deployments API for the run with the person's token. Warn when
   the environment has prevent self-review on (F8). Mark the input human-only and one-way in the registry. Update
   `docs/threat-model.md` (#888 Expected paths). Tests in `internal/cli/respond_test.go`.
2. **Card (P2).** An incident-action Need kind in the Now area (#888). If #873's registry is not built, an
   `InboxItem` kind in `internal/cli/inbox.go` is the nearest form (inferred). The card runs P1's command.

## Data and ledger records

No new ledger record is named in the sources. The incident timeline stays a comment on the incident
(`readTimeline`, `writeTimeline`, `internal/cli/respond.go:578-617`).

## Key decisions

No Decision issue is linked from #888. It depends on [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873)
(One Need registry; Feature, v0.4.0, not sliced).
