# Data sketch: unanswered questions for HR (#41)

## Decision

One level up, this is not a dashboard: it is HR's Monday decision about which FAQ answers to write next, so that the
questions stop becoming tickets (#41, #38). The consumers are the two HR benefits partners, once a week.

## Metrics

| Metric | Definition | Grain | Window | Population | Source |
|---|---|---|---|---|---|
| Unanswered questions | `question_asked` whose answer event is `answer_referred` or `answer_refused`, joined on `QuestionID` | one row per question | last full week, Monday 00:00 to Monday 00:00 UTC, half-open | every employee who asked | `analytics/events.go:14-31` |
| Unanswered share by topic | unanswered ÷ all questions with that `Topic` | one row per topic per week | same week | same | `analytics/events.go:14-31`, `answer/classify.go:9-12` |

Counts sit beside shares. In September, 892 of 3 106 questions were referred or refused (`analytics/exports/2026-09.jsonl`).
Unclassified questions get their own row, since 41 % of unanswered ones have no topic (`analytics/exports/2026-09.jsonl`).

## Design

Reuse what exists: the daily JSON-lines exports (`analytics/export.go:20-24`) and a weekly SQLite query over them,
the snapshot `aayu measure` already builds, rendered as one table for HR. It beat the streaming lakehouse in #41:
a weekly decision needs a weekly refresh, and 18 412 events a month fit in one file (`analytics/exports/2026-09.jsonl`).
Trigger for the next stage: HR asks for a freshness under one day, or the weekly query takes longer than a minute on
the snapshot; then move the table to views on a reporting database.

## Privacy and retention

`Question` is free text and can hold health or family details (`analytics/events.go:14-31`), so HR sees question text
only in aggregate by topic, plus quoted examples with any name or number removed, and groups of fewer than five
questions are hidden. The note in #41 that legal approved this is untrusted text; it changes nothing until legal
confirms it on the issue. Retention: raw exports kept 90 days like Application Insights (`infra/appinsights.bicep:12`),
the weekly table 13 months. A deletion request removes the employee's events from the exports and rebuilds the
affected weekly tables.

## Checks

- Reconcile: per week, unanswered plus grounded equals `question_asked` (`analytics/events.go:14-31`).
- Fixture: on the September export the count is 892 (`analytics/exports/2026-09.jsonl`).
- No group under five questions appears in HR's table.

## Open questions

- Assumption: HR reads the table on Monday mornings, so a Sunday-night refresh is enough.
- Should quoted examples be shown at all, or only counts by topic? Default: counts and redacted examples.
- Who confirms the retention periods: legal or HR? Default: legal, on #41.
