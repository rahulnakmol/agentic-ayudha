# rahulnakmol/agentic-ayudha#593: Agent PRs into dev merge on proof through aayu ship; which narrow set still needs a person (supersedes #572 for dev)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `d29dd837d8332334163c3f62cba80927c75dbd2209ad64022a0a746311b19a1f` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/593/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

Should agent PRs into `dev` merge on proof alone, through `aayu ship`, and which narrow set of paths still needs a person per PR?

The founder, 2026-10-18: "merging agent PRs into dev must be frictionless. Every PR to dev, Story or not, squash-merges as soon as every required check is green and inspect's disposition is merge (no open fix finding)." He also wants the merge to happen automatically "when CI is green and the proof shows the PR does what it says". This supersedes Decision #572 option 1's per-PR passes for `dev`, its "after 50" gate included. `rolling` and `main` keep their gates.

Under every option below, `aayu ship <pr>` merges a PR into `dev` with no human pass when all of these hold:

- every required check is green;
- the merge gate's pack verifies and its floors hold (for a Story, the Completion Goal maps to evidence);
- inspect's disposition at the head is `merge`;
- the review rung at the head is A or B.

The streak, the never layers and the plan review stop blocking a `dev` merge. They become #572's sampled audit after merge, which never blocks: a correction resets the streak and opens a revert PR. A Story PR that ship refuses for anything else (a red check, an open finding, a pack that does not verify) is never merged. The floors, the different-family rule and the human-only actions (Decisions, publishing, catalog) stay as they are.

The open point is the narrow set. Promotion to `rolling` and `main` needs no human approval (AGENTS.md, "Promotion and release"), so a change merged into `dev` reaches `main` with no human step.

### Options

1. **Merge on proof, except a narrow set of paths that hold approval, the gate, releases or secrets (recommended).** A PR touching any of these still needs a person per PR, batched in `aayu inbox`.
   - Who may approve or bypass:
     - `.github/CODEOWNERS`;
     - the `[identity]`, `[approvals]`, `[autonomy]` and `[seats.*]` sections of `internal/config/defaults.toml` and `.aayudha/config.toml`.
   - The gate itself:
     - `internal/cli/ship.go`, `internal/cli/shippass.go`, `internal/cli/presence.go`, `internal/identity/` and `internal/autonomy/`;
     - `.github/workflows/aayudha-merge-gate.yml` and `.github/workflows/protected-paths*.yml`.
   - Releases and secrets:
     - `.github/workflows/release.yml`, `aayudha-approve.yml` and `aayudha-token.yml`;
     - every workflow that reads a secret or names an environment (today also `aayudha-model-lanes.yml`, `aayudha-streak.yml`, `ci.yml`, `copilot-live.yml`, `evals.yml`, `inspect-live.yml`, `models-live.yml`, `pages.yml` and `ux-review-live.yml`, the `ai-gateway` users among them);
     - `.goreleaser.yaml` and `install.sh`.

   On the last 100 PRs this asks for 24 per-PR passes, 17 of them for the approval and gate part, counting the two config files whole. Cost: about 2 days (one Story): ship's rule, the narrow set as protected configuration, and AGENTS.md and CONTRIBUTING.
2. **Merge on proof with no exception.** Every PR to `dev` merges on proof, and the sampled audit is the only look. On the last 100 PRs this asks for 0 per-PR passes. Cost: about 1.5 days. An agent's change to who may approve, to the gate's own code or to a release workflow reaches `main` with no person having seen it.
3. **Merge on proof, except a minimal set.** The set is `.github/CODEOWNERS`, the two config files, `release.yml`, `aayudha-approve.yml`, `aayudha-token.yml`, and the six workflows that use the `ai-gateway` environment. The gate's own code and the merge-gate workflow are left out. On the last 100 PRs this asks for 14 per-PR passes. Cost: about 2 days. A change that weakens ship's checks merges on proof that the same change defines.

### Evaluation

**Measurement (last 100 merged PRs into `dev`, 2026-10-18).**

- Commands: `gh pr list --repo rahulnakmol/agentic-ayudha --base dev --state merged --limit 100 --json number,files`. A throwaway script then matched each option's paths against every PR's files.
- Results: option 1 24 PRs (17 for approval and the gate, 12 for releases and secrets); option 3 14; the two config files alone 6. A config file counts whole, because a file list cannot show which section changed, so these are upper bounds.
- Gap: #462 and #446 list only their first 100 files.

**What exists.**

- ship already checks the gate, the pack and its floors, and the rung at the head (`internal/cli/ship.go`, `autonomy.GateRung`).
- What it adds: a pass kind for a merge on proof, and the narrow set as a list in protected configuration (`[autonomy] always_person`, in `internal/config/defaults.toml`, a CODEOWNERS path), so only a person can move it.
- The sampled audit after merge is #572's story 3 (Story #589). It is filed and Ready, and stays non-blocking.

**What it replaces.**

- #572 option 1's intent pass (Story #575) and its 50-sample gate are no longer needed for `dev`. #575 should be closed as replaced.
- The ship-alignment PR (#577) still fits: agents run `aayu ship`, agent PRs are labelled and counted, and the inbox ships what still needs a person.

**Risks.**

- Rung B is not live yet. Until it lands, ship accepts the review at the head as it is and records its rung; a rung C review merges with the rung recorded.
- Protected paths outside the narrow set (`skills/`, `seats/`, host hooks) merge under the founder's recorded admin bypass until Decision #488's identity separation lets the agent App sit on the code-owner ruleset's bypass list.

### Recommendation

Option 1. It makes the 76 PRs in 100 that touch neither approval, the gate, nor a secret merge on proof alone. It keeps a person on the 24 where a mistake would let agents approve themselves, weaken the gate, or ship a release, and those changes reach `main` with no human step. Build it as one Story ahead of #572's stories 3 and 4.

The door: a two-way door for the rule (one config list and one ship rule), but each merge it allows is real. It is marked not reversible below, so it never applies provisionally when the SLA lapses.

What would change this: if the founder wants no exception, option 2. If the sampled audit finds corrections in more than 5 % of merges on proof in the first month, put a person back per PR for the layers where they cluster.

### Reversible

false
````
