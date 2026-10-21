# Ethos

How `aayudha` works, and how this repository works on itself. When a change and one of these principles
disagree, the principle wins or the change also changes the principle, by PR.

1. **Proof, not claims.** Work is done when a proof pack shows every Completion Goal criterion met with
   evidence, and the pack is written by CI, not by the agent that did the work. "It works on my machine" and
   "the model says it is fine" are not evidence.
2. **Deterministic checks beat model judgement.** A floor miss (coverage, SAST, secrets, dependencies,
   accessibility, performance) blocks. No model disposition can override it; only a recorded Decision can
   waive it, and `audit` counts every waiver. The thresholds are in [CONSTRAINTS.md](CONSTRAINTS.md).
3. **Humans hold the gates that matter.** Agents open issues and PRs, write code, and run checks. A human
   approves protected-path changes, accepts Decisions, and publishes. Approvals count only from accounts that
   are not bots. Autonomy is earned per layer by a streak that a protected workflow computes, never by a
   counter an agent can write.
4. **One source of truth, generated everywhere else.** The verb registry and the embedded default
   configuration are the sources; README tables, `LAYERS.md`, RACI, skill frontmatter, MCP schemas, and
   `CONSTRAINTS.md` are generated from them, and CI fails on drift.
5. **Every write is attributable.** External writes go through one actions log with an idempotency key; the
   audit source of record is GitHub's own event history. Bypasses are explicit and listed, never silent.
6. **Untrusted text stays data.** Issue bodies, comments, exports, and tool output are wrapped and can never
   trigger a ship, a publish, an incident action, or a merge.
7. **Small, reversible steps.** Work moves in XS–L stories, checkpoint by checkpoint, through `dev`, then
   `rolling`, then `main`. Every release is signed and can be traced to the commit and workflow that built it.
8. **Say what does not exist yet.** A feature planned for a later work package is "n/a until WPx.y". Nothing
   pretends to be done.
9. **Pay for what you use.** Budgets cap model and CI spend, expensive floors run only where the change can
   affect them, and the breaker pages a human instead of spending more.
10. **Fix the system before the prose.** When an agent gets something wrong, the fix goes to the highest layer
    that can make the mistake impossible or caught for good, and most of the effort goes there: first the codebase
    (types, invariants, and boundaries that make the right thing the easy thing), then static checks (lint rules,
    tests, CI checks, `verify`), then prose (skills, `AGENTS.md`, rules) only for what needs judgement, and least
    into human review, which changes what the environment surfaces rather than asking people to look harder.
    `retro --classify` records each correction's layer with its reason; a codebase or check correction becomes a
    Story whose proof is the new type, invariant, rule, or test, and `evolve` rewrites a skill only for a prose one.
