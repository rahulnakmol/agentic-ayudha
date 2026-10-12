# Solo-mode bypass reconciliation (hand-run, #3) — 2026-10-12

Inputs: `gh api --paginate "repos/rahulnakmol/agentic-ayudha/rulesets/rule-suites?rule_suite_result=bypass&time_period=day"` filtered
to suites pushed after 2026-10-11T22:15Z (after reconciliation #2), fetched 2026-10-12T00:30Z; each `after_sha` resolved with
`commits/<sha>/pulls`; each merged PR checked for a comment starting "Solo-mode bypass:"; ledger commits read with `git log`.
Policy: CONTRIBUTING.md "Solo-mode policy"; BUILD-PLAN Appendix B D1, D2. Automated by `aayu audit` in WP7.6.

## Protected-path merges into `dev` (solo-mode bypasses)

| Rule suite | After | Pushed at | PR | Prior comment | Bypassed rule |
|---|---|---|---|---|---|
| 4472093694 | `d096083` | 2026-10-11T22:16:25Z | #16 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/16#issuecomment-6114478423) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472199375 | `862ae9e` | 2026-10-11T22:30:40Z | #17 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/17#issuecomment-6114617397) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472243301 | `2093d17` | 2026-10-11T22:37:01Z | #18 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/18#issuecomment-6114636261) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472441821 | `a1e32fa` | 2026-10-11T23:06:44Z | #22 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/22#issuecomment-6114916506) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472539969 | `57f1a87` | 2026-10-11T23:22:18Z | #23 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/23#issuecomment-6115056045) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472638417 | `c728471` | 2026-10-11T23:39:00Z | #28 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/28#issuecomment-6115193091) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472675731 | `d9c27e3` | 2026-10-11T23:45:34Z | #73 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/73#issuecomment-6115245695) | `required_status_checks` (protected-paths-codeowner-review) |
| 4472774786 | `f33dce4` | 2026-10-12T00:01:58Z | #75 | [comment](https://github.com/rahulnakmol/agentic-ayudha/pull/75#issuecomment-6115374980) | `required_status_checks` (protected-paths-codeowner-review) |

All 8 have a prior "Solo-mode bypass:" comment; in each, every other required check was green.

## `ledger` writes under the founder identity (D1 day-one rule)

| Rule suites | Commits | Classification |
|---|---|---|
| 4472087606 | `9af665f` | Expected: ledger push of reconciliation #2 |
| 4472680925 … 4472701315 (34 suites) | `c92f35f` … `7e6bf1d` | Expected: `aayu ready` Ready snapshots `specs: Ready snapshot of #38` … `#71` (34 Stories, ground rule 1), human-fallback identity, 2026-10-11T23:46–23:50Z |

Result: 43 bypasses, all accounted for: 8 solo-mode protected-path merges (#16, #17, #18, #22, #23, #28, #73, #75)
and 35 founder-identity ledger writes. No unexplained bypass. The rolling promotion of 2026-10-12 and this report's own push appear
in the next reconciliation.
