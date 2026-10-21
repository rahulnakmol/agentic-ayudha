# Measurement review: #61 retention, run of 2026-10-12

## Result

There is no Result yet, and that is the correct outcome. `measure` refused with exit 4 because no cohort is complete
in the window 2026-10-01 to 2026-10-12, and it wrote nothing to #61 (#61). Retention is activity in days 7 to 13 after
acquisition, and a cohort counts only when its week end plus 14 days falls inside the window
(`internal/measure/measure.go`).

## Cohort completeness

| Cohort week | Acquired | Retained so far | Complete on |
|---|---|---|---|
| 2026-10-01 | 3 212 | 412 | window end of 2026-10-22 (week end 10-08 + 14 days) (#61) |
| 2026-10-08 | 1 480 | 0 | window end of 2026-10-29 (week end 10-15 + 14 days) (#61) |

Working retention out from the first week would give 412 / 3 212 = 0.13 (#61). That is not a reading: users
acquired on 7 October cannot reach day 7 until 14 October, so most of the cohort's retention days lie after the
window. The partial number is biased low, and in a board pack it would read as a fall from the 0.18 baseline (#61)
that has not happened.

## Data quality

- 3 904 of 25 120 rows were skipped, 15.5 %, against under 1 % before 1 October (#57, #61).
- 3 871 of them come from `globex-web` with no `UserId` and no `UserAuthenticatedId` (`analytics/events.go:40-44`,
  #61). Inference: Globex's web client is not sending the signed-in user, so Globex visitors are missing from every
  cohort, which biases any rate toward the other two employers.
- 33 rows have an unparsable `TimeGenerated` (#61), too few to matter.

The data is not broken everywhere; one source is. Skipped rows cannot be repaired after export, so the fix is at the
client.

## Caveats

- Unverified: the share of Globex employees among the 4 692 acquired users; it cannot be known until the field is
  sent.
- The target of at least 0.25 and the baseline of 0.18 are the Revenue Architect's standard (#61); nothing here
  changes them.

## Actions

| Action | Owner | Date |
|---|---|---|
| Find why `globex-web` sends no user ID, per `analytics/events.go:40-44`, and fix it at the client | AI Native Engineer twin, reviewed by the Data & AI Architect twin | before 2026-10-15 |
| For the board pack: report "retention not yet measurable; first complete cohort on 2026-10-22", with the acquired counts above (#61) | Head of product | 2026-10-15 |
| Rerun from the plan's start of 2026-10-01 (#61): `aayu measure 61 --from 2026-10-01 --to 2026-10-22 --appinsights <export through 21 October>` | Data & AI Architect twin | 2026-10-22 |
