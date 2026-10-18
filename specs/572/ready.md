# rahulnakmol/agentic-ayudha#572: Decision: a person's attention to agent PRs by exception: approve intent per Story, a narrow always-a-person set, sampled audit after merge, and one batched inbox

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `a611dcf78015fd7523f832e58a98147c3eb96935e705e6ff8d33d32fcdc48818` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/572/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

Should a person's attention to agent pull requests scale per PR or by exception, and which rules make "by exception" safe?

Why now: 100 pull requests merged into `dev` in about 12 hours (2026-10-17 17:42 to 2026-10-18 05:46 UTC). Under today's rules, a person would have had to pass 90 of them, one at a time, and 73 even after every layer's streak was full (measurement below). The founder asked: "We can't put a huge cognitive load on a human to ship every PR; what if cloud agents create 1000 PRs a week?" At that rate, today's rules ask for about 730 passes a week.

Unchanged under every option: the floors in `CONSTRAINTS.md` and their waivers (rule 7), the different-family rule (Decision #469: only rung A or B lets an agent merge without a person), and the human-only actions (accepting a Decision, publishing, approving a catalog action, `seat`, `init`, `halt` and `resume`).

### Options

1. **Approve intent, not diffs; keep a narrow always-a-person set; audit samples after merge; batch the rest (recommended).** The person approves each Story once, at Ready. For a Story whose plan touches a never layer, they also approve the plan once (`aayu plan <n> --approve`). The Story's PRs then ship without a per-PR pass when all of these hold: rung A or B at the head, the floors hold, and the PR's footprint stays inside the approved plan. A narrow set of paths always needs a person per PR. A sample of autonomous merges goes into the inbox after merge. A correction resets the layer's streak and opens a revert PR. On the measured 100, this asks for 21 per-PR passes, 15 Story approvals, and the never-layer plan approvals, all in batched inbox sessions. After merge, a person also looks at every autonomous merge while its layer's streak is below 20, then 10 to 50 % of them by tier. Those looks never block a merge. Cost: about 6 to 8 days of build in four stories. It also needs the agent bot on the code-owner ruleset's bypass list for protected paths.
2. **Keep per-PR passes, and batch them.** Today's rules stay. The only change is the inbox's ready-to-ship batch: one device code per session ships every PR the person passes. On the measured 100, that is 90 passes now and 73 once the streaks are full. Cost: about 1 day, already being built. It lowers the clicks, not the decisions.
3. **Risk tiers per PR, as #544 point 6 states.** Low-tier PRs merge autonomously after the streak. Medium and high tiers need a person per PR, and later two. Every never-layer and protected-path PR is high, so at least 63 of the 100 need a person, plus the medium tier. Cost: about 1 day. Load grows with PR count.
4. **Merge on proof, audit after.** Everything that passes the floors with rung A or B merges, except the narrow set. A person samples merged PRs, and a correction reverts. This has the lowest load (21 per-PR passes, plus samples). But a never-layer change lands with no person having seen its intent, and an escape is found only after it is on `dev`. Cost: about 3 days.

### Evaluation

**Measurement (last 100 merged PRs into `dev`, 2026-10-18).** The commands:

- `gh pr list --repo rahulnakmol/agentic-ayudha --base dev --state merged --limit 100 --json number,title,mergedAt,files,labels,author,body`, with `gh api --paginate repos/rahulnakmol/agentic-ayudha/pulls/<n>/files` for #462 (166 files) and #446 (565 files), where the list stops at 100 files.
- Footprints come from `verify.ReadLayers` and `LayerGlobs.Footprint` (`internal/verify/footprint.go`) over `LAYERS.md`, the same code ship uses. Protected paths are the patterns in `.github/CODEOWNERS`. A throwaway test ran both and was deleted.
- `aayu status --streak` gave the streak table.

| Measure | PRs of 100 |
|---|---|
| Touch a never layer (`[autonomy] never` = L04, L07, L09; `internal/config/defaults.toml:54`) | 26 (L04 4, L07 13, L09 14) |
| Touch a protected path | 56 (`skills/` 36, `.github/` 14, `internal/config/defaults.toml` 4, `seats/` 4, `CONSTRAINTS.md` 2, `.agents/` 2, host hooks and plugin files 6) |
| Both | 19 |
| Either | 63 |
| No layer at all (docs only and similar; `shipStreak` never treats an empty footprint as autonomous, `internal/cli/shippass.go:115`) | 10 |
| Need a person under today's rules with rung B on, streaks starting at 0 as today | 90 |
| The same, with every layer's streak already at 20 (`[autonomy] streak = 20`, `defaults.toml:53`) | 73 |
| Touch the narrow set: `.github/workflows/`, `.github/CODEOWNERS`, `internal/secrets/`, `internal/identity/`, the release path (`internal/cli/release.go`, `.goreleaser.yaml`, `install.sh`), `SECURITY.md`, `docs/threat-model.md` | 21 |
| Linked to a Story (`Refs #n`, where #n is a Story) | 23 PRs for 15 Stories |

The 90 comes from replaying the PRs in merge order. Each layer's streak starts at 0 and grows by one per merge, assuming no corrections and rung B on every PR. A PR needs a person if it touches a never layer, a protected path, no layer, or any layer whose streak is below 20 (`internal/cli/ship.go:274`, `shippass.go:115`). This assumption is optimistic, so the real numbers are higher.

**Why the streak counts nothing today.** `aayu status --streak` reports 0 agent merges in every layer of the 100 scanned. All 100 PRs were opened by `rahulnakmol`, the agents using the founder's token. Only #558 carries `agent-authored`, the label `autonomy.Rules.Agent` reads (`internal/autonomy/autonomy.go:112`). #558 then shows two "non-bot-commit" corrections, for commits by `ampagent`, because `Detect` counts any commit by a login that is not a bot, and that lacks an `Aayudha-Checkpoint` trailer, as a person's correction. The ship-alignment PR fixes the labelling and the commit attribution. It is needed under every option, so it is not wasted.

**The directions weighed.**

- **Approve intent, not diffs.** The pieces exist. Ready snapshots bind the Completion Goal (`internal/ready/ready.go`, `ready.LedgerPath`). `plan --approve` records the Engineering Manager's review of a never-layer plan, and ship already requires it (`internal/cli/ship.go`, `PlanRecord`). The new rule is a third kind of human pass, `intent`. It holds when the Story's Ready approval was a person's, any never-layer plan was approved by a person, the PR's footprint is a subset of the plan's footprint, and rung A or B holds at the head. A PR that leaves its plan's footprint falls back to a per-PR pass. Ready today is posted by `aayu ready` under the bot identity. Option 1 adds a human-only `aayu ready <n> --approve` (or the inbox's batch), so that the intent approval is a person's.
- **Sampled audit after merge.** `autonomy.Rules.Detect` already finds corrections (changes requested, a person's commit, a revert, a linked Bug or Incident within `[autonomy] correction_days`, the `aayu:correction` label). The additions:
  - Every autonomous merge is sampled into the inbox at a rate set by its tier and its layer's streak: 100 % while the layer's streak is below 20, then 10 % for low, 25 % for medium and 50 % for high.
  - A person's "this is a correction" in the inbox labels the PR, which resets the layer's streak through the existing rules.
  - aayu opens a revert PR, which ships through the same gates.
- **Risk tiers.** `inspect.RiskOf` (`internal/inspect/risk.go`) stays the deterministic tier. Under option 1, a tier decides the sampling rate and the evidence asked for. It does not decide a per-PR pass, except for the narrow set. The narrow set lives in protected configuration (`[autonomy] always_person`, in `internal/config/defaults.toml`), so only a protected-path change moves it.
- **Batching.** Whatever still needs a person comes in one `aayu inbox` session, under one device code: the ready-to-ship batch being built now.
- **#544 point 6 in light of this.** Point 6's tiers should gate sampling and the narrow set, not every never-layer or protected-path PR. "Two people for high once the pod has two humans" applies to the narrow set only. Point 6 was not built yet, so changing it costs nothing.

**Costs and risks of option 1.**

- About 6 to 8 days in four stories:
  1. Counting (labels, commit attribution, ship by agents): the PR in flight.
  2. The intent pass, and `ready --approve`.
  3. Sampling, the correction action and the revert PR.
  4. The narrow set as protected configuration, and #544 point 6 rewritten.
- Protected paths outside the narrow set (`skills/`, `seats/`, host hooks) would merge without a code owner's per-PR review. That needs the agent App on the code-owner ruleset's bypass list, which is a protected-path and admin change. Decision #488's identity separation must be in place first, so that the bypass belongs to the bot, not to a token an agent holds.
- An intent approval is only as good as the plan. A vague plan approves a wide footprint. The plan check should refuse a plan whose footprint is broader than its Story's layers.

### Recommendation

Option 1, because it moves the person's attention to the two places where judgment is cheapest and most valuable: the Story's intent before work starts, and a sample after merge. It keeps a per-PR look only where a mistake leaks credentials, security policy, or a release. On the measured 100, the per-PR passes drop from 73 at best (option 2) to 21, and everything left is batched into one inbox session. Build it in the order of the four stories above. Story 1 (the ship-alignment PR) proceeds now under every option. Stories 2 to 4 wait for acceptance.

The door: not reversible in practice. It changes what counts as a person's pass, and that change is written into every ship record and audit. It is marked not reversible below, so that it never applies provisionally when the SLA lapses.

What would change this: if Decision #488's identity separation cannot land (agents keep holding a token that can bypass rulesets), keep protected paths per-PR. That leaves option 1 without its protected-path relief: 61 per-PR passes on the measured 100 (protected or narrow-set paths). If the first month's sampled audits find corrections in more than 5 % of autonomous merges, raise the sampling rate before widening anything.

### Reversible

false
````
