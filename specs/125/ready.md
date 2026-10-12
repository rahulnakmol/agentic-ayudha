# rahulnakmol/agentic-ayudha#125: Example Co employees get benefits answers within a week (benefits-qa example)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, goal |
| Body hash | `88c1a9fc2c2bca6d745c8d9b862d5521bd4975d7688f1a2d9e52838b9f339baa` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/125/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Employees of the fictional employer Example Co get an answer to their benefits questions within a week of first asking, quoted from the plan documents, instead of waiting on an HR ticket. This is the outcome of the benefits-qa example product (`examples/benefits-qa/`, docs/BUILD-PLAN.md WP8.1), built by the aayudha pod from `grill goal` to `release`. Every number is from synthetic, seeded data (V3); no real person or employer is measured.

### Beneficiaries

employees (synthetic: the fictional Example Co workforce)

### Metric

activation

Activation as `aayu measure` computes it from an Application Insights export with this repository's `[measure]` settings: of the synthetic employees acquired by their first `benefits_visit` in the window, the share who receive an `answer_grounded` (an answer quoted from the plan documents, by the product or by an HR specialist) within 7 days, over complete weekly cohorts.

### Baseline

0.258 (31 of 120 synthetic employees activated, cohorts of 2026-08-10 and 2026-08-17, both complete) before the product: every question an HR ticket. Measured on 2026-10-12 with `aayu measure --appinsights examples/benefits-qa/analytics/testdata/hr-ticket-seed1-2026-08-10.jsonl --metric activation --from 2026-08-10 --to 2026-09-07`; the export (SHA-256 `013572375d8b3675824c3e52e7169610c9c81d8b76251d7f1441049180e4d5ae`) is the seeded HR-ticket simulation `simulate -channel hr-ticket -seed 1 -from 2026-08-10` (120 employees, 60 % of tickets answered after 1 to 10 days), committed with the analytics story so the baseline can be re-derived. Snapshot: `.aayudha/state/measure/all-20261012T072505Z.sqlite` (SHA-256 `dacf9df5116843f951f68cde98cfb84720366a61dad65337312df3520b21b61f`, kept on the measuring machine; rebuilt from the export by the same command).

### Target

>= 0.60 activation over the 28 days after the example ships, measured the same way on the product channel's export
````
