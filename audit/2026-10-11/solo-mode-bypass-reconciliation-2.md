# Solo-mode bypass reconciliation (hand-run, #2) — 2026-10-11

Inputs: `gh api --paginate "repos/rahulnakmol/agentic-ayudha/rulesets/rule-suites?rule_suite_result=bypass&time_period=day"`
fetched 2026-10-11T22:20Z; each `after_sha` resolved with `commits/<sha>/pulls`; each merged PR checked for a comment starting
"Solo-mode bypass:"; failing rules from `rulesets/rule-suites/<id>`. Covers suites after reconciliation #1. Policy: CONTRIBUTING.md
"Solo-mode policy" and "Promotion (agent-run)"; BUILD-PLAN Appendix B D1, D2, D-0.4a. Automated by `aayu audit` in WP7.6.

| Rule suite | Ref | After | Pushed at | PR | Bypassed rule (ruleset) | Classification |
|---|---|---|---|---|---|---|
| 4471129496 | `ledger` | `013171f` | 19:45:36Z | — | `update` (ledger-writers) | Expected: founder-identity ledger write of reconciliation #1 (D1 day-one rule) |
| 4471739456 | tag `v0.0.0-d4test.1` | `377b5e3` | 21:20:30Z | — | `creation` (release-tags) | Expected: WP0.4 D4 acceptance test tag (PR2 thread); release.yml refused it ("not on main") |
| 4471743447 | tag `v0.0.0-d4test.1` | deleted | 21:21:03Z | — | `deletion` (release-tags) | Expected: cleanup of the D4 test tag |
| 4471759694 | `dev` | `7bc80dd` | 21:23:37Z | #8 | `required_status_checks` (protected-paths-codeowner-review) | **Solo-mode bypass**, comment [#8](https://github.com/rahulnakmol/agentic-ayudha/pull/8#issuecomment-6113923044) |
| 4471792935 | `dev` | `5fcac2e` | 21:29:05Z | #11 | `required_status_checks` (protected-paths-codeowner-review) | **Solo-mode bypass**, comment [#11](https://github.com/rahulnakmol/agentic-ayudha/pull/11#issuecomment-6113976934) |
| 4471818249 | `dev` | `d754dc5` | 21:33:08Z | #12 | `required_status_checks` (protected-paths-codeowner-review) | **Solo-mode bypass**, comment [#12](https://github.com/rahulnakmol/agentic-ayudha/pull/12#issuecomment-6114014103) |
| 4471851556 | `dev` | `f9d2050` | 21:38:34Z | #9 | `required_status_checks` (protected-paths-codeowner-review) | **Solo-mode bypass**, comment [#9](https://github.com/rahulnakmol/agentic-ayudha/pull/9#issuecomment-6114062959) |
| 4471939759 | `dev` | `3a42650` | 21:53:14Z | #14 | `required_status_checks` (protected-paths-codeowner-review) | **Solo-mode bypass**, comment [#14](https://github.com/rahulnakmol/agentic-ayudha/pull/14#issuecomment-6114211511) |
| 4472075201 | `rolling` | `ea9cb2b` | 22:14:02Z | — | `update` (rolling-main-promotion) | Expected: agent-run promotion "Rolling snapshot: 20261011T2210Z" after the rolling gate passed ([run 38179325066](https://github.com/rahulnakmol/agentic-ayudha/actions/runs/38179325066)); tag `rolling-2026-10-11` |

Result: 9 bypasses, all accounted for; 5 solo-mode bypasses (PRs #8, #9, #11, #12, #14), each with a prior "Solo-mode bypass:"
comment and all other required checks green. No unexplained bypass. This report's own ledger push will appear in the next run.
