# Glossary

Terms used across the build plan ([docs/BUILD-PLAN.md](docs/BUILD-PLAN.md)), the docs, and the binary. The
WP in brackets is where the term is built; "n/a until" means it does not exist yet.

| Term | Meaning |
|---|---|
| `aayudha`, `aayu` | The binary and its short alias. One static Go binary. |
| Verb | One of the 32 user-facing commands (`grill`, `build`, `ship`, …) declared in the verb registry. Each runs as a CLI command, and most also as a host skill and an MCP tool. |
| Plumbing command | A supporting command that is not a verb (`config`, `auth`, `halt`, `upgrade`, …). |
| Verb registry | `internal/verbs/registry.go`, the single source for verbs, seats, layers, lanes, and gates. Several files are generated from it [WP0.3]. |
| Seat | One of twelve roles in the pod (Product Strategist, Code Architect, …). Every verb has one Accountable seat; every layer has one owner seat. |
| Twin | The agent half of a seat. A seat is **solo** (twin only) or **paired** (twin plus a human) [WP5.1]. |
| Founder | The seat that holds final authority. Founder A owns the plan; founder B is the escalation contact for Decisions. |
| Layer | One of nine application layers, L01 Experience to L09 Platform / operations. A path belongs to the layer whose glob in [LAYERS.md](LAYERS.md) matches it. |
| Footprint | The set of layers a story or PR touches, computed from its changed paths. |
| Lane | A kind of model work (spec, plan, build, review, ui-review, chore) with a default model per host. The reviewer family must differ from the builder family. |
| Host | The agent environment a skill runs in: Amp, Cursor, or Claude Code. |
| Work item | A GitHub issue of one of ten types (Goal, Initiative, Feature, Story, Bug, Decision, …) with Create, Ready, and Complete sections [WP2.1]. |
| Completion Goal | The numbered, observable criteria a Story or Bug must meet, each with the kind of evidence that proves it. |
| Ready | The stage at which a work item passes validation. `ready` posts a **Ready snapshot** (body hash and Completion Goal hash) that later checks compare against [WP2.7]. |
| Checkpoint | One commit per layer in a build, marked with an `Aayudha-Checkpoint: n/N` trailer [WP4.10, WP5.5]. |
| Floor | A deterministic check with a threshold (coverage, SAST, secrets, dependencies, accessibility, performance, screen reader, DAST, mobile). Defaults are in [CONSTRAINTS.md](CONSTRAINTS.md) [WP4.3–4.9]. |
| Waiver | A recorded Decision that lets one floor miss through; `audit` counts waivers [WP5.6]. |
| Proof pack | `pack.json` plus artifacts that map every Completion Goal criterion to evidence and record floor results for one PR head SHA [WP4.1]. |
| Authoritative pack | A pack written by the merge-gate workflow in CI. Packs pushed by an agent are **advisory** [WP4.8]. |
| `ledger` | An orphan branch holding Ready snapshots, proof packs, action segments, and audit reports. Not for code. |
| Merge gate | The base-branch workflow that re-runs `verify` on the PR head and writes the authoritative pack [WP4.8]. |
| Disposition | The structured verdict of `inspect` (`merge`, `fix`, `rebuild`) or `review` (`recapture`, `rebuild`, `fix`, `ship`). It can downgrade a result, never override a floor. |
| Human pass | An approving review from a human account (`type != Bot`, not a configured bot login) [WP1.4, WP5.5]. |
| Streak | Consecutive correction-free merges on a layer. At `autonomy.streak`, agents may merge on that layer without a human pass, except on never-layers [WP5.7]. |
| Correction | Evidence that an agent's merge needed a human fix: a changes-requested review, a non-bot commit, a revert, a linked Bug, … It resets the streak. |
| Never-layer | A layer in `autonomy.never` that always needs a human CODEOWNER approval, whatever the streak. |
| Bot identity | The GitHub App (or machine-user fallback) that agent lanes and CI write as [WP1.4]. |
| Human fallback | The day-one mode with no bot identity: agents write as the human and their writes are tagged `identity:human-fallback`. |
| Token broker | The workflow that mints a short-lived App installation token for an agent on a laptop, so the App key never leaves repository secrets [WP1.4]. |
| Protected path | A path in [.github/CODEOWNERS](.github/CODEOWNERS) that needs a human CODEOWNER review. |
| Solo-mode bypass | While there is one human, a protected-path PR merges only through the repository admin's recorded ruleset bypass (CONTRIBUTING.md). |
| Decision | A work item with options, evaluation, and a recommendation, accepted by the holder of `[decisions] authority`. An overdue reversible Decision becomes **provisional** [WP5.6]. |
| Business hours | The `[time]` zone and hours that SLA clocks and paging windows count in [WP1.2]. |
| Budget breaker | The circuit breaker that pauses agent lanes and pages a human when a `[budget]` limit trips (exit 6) [WP1.6]. |
| Halt | `aayu halt`: a repository variable every workflow and verb checks before it writes; `aayu resume` clears it [WP5.7]. |
| Promotion ladder | `dev` (integration) → `rolling` (canary snapshot, `rolling-<date>` tag) → `main` (stable, annotated `vX.Y.Z` tag). Each step runs a gate. |
| Promotion gate | The CI suite a promotion must pass: `ci.yml` dispatched with `gate=rolling` or `gate=stable` on the candidate merge commit. |
| Rule 1a, "n/a until" | A feature named before its work package ships reports "not yet available" and is never counted as a failure. |
