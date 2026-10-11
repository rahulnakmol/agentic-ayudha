# Solo-mode bypass reconciliation (hand-run) — 2026-10-11

Inputs: `gh api "repos/rahulnakmol/agentic-ayudha/rulesets/rule-suites?rule_suite_result=bypass&time_period=day"`
fetched 2026-10-11, each `after_sha` resolved to its merged PR with `gh api repos/rahulnakmol/agentic-ayudha/commits/<sha>/pulls`,
and each suite's failing rules read from `gh api repos/rahulnakmol/agentic-ayudha/rulesets/rule-suites/<id>`.
Policy: CONTRIBUTING.md "Solo-mode policy"; docs/BUILD-PLAN.md Appendix B D1, D2. Automated by `aayu audit` in WP7.6.

| Rule suite | Ref | Actor | After | Pushed at | Merged PR | Bypassed rule (ruleset) | Classification |
|---|---|---|---|---|---|---|---|
| 4471124212 | `scratch/wp01-base-2` | rahulnakmol | `12de9de` | 2026-10-11T19:44:44Z | #3 | `required_status_checks` (protected-paths-codeowner-review) | **Solo-mode bypass**: founder merged an unreviewed protected-path PR (`skills/`) through the admin PR-only bypass. WP0.1 acceptance test into a scratch base; never reached `dev`. |
| 4471059806 | `ledger-probe` | (none listed) | `e17ff0a` | 2026-10-11T19:34:07Z | none | `update` (ledger-writers) | Expected: WP0.1 live test D1, deploy-key (machine identity) push to the deleted `ledger-probe` branch. |

Result: 2 bypasses, both accounted for; 1 solo-mode bypass (PR #3). No unexplained bypass.
This report's own push to `ledger` is a founder-identity write under the WP0.1 day-one rule and will appear as
a `ledger-writers` bypass in the next reconciliation.
