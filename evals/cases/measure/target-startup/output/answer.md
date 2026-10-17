# Measurement note: #38 activation, 2026-10-12

**Verdict: target not met.** Activation is 0.31 against the target of at least 0.35: 412 of 1 329 users acquired in
the two complete cohorts (weeks of 14 and 21 September) got a grounded answer within 7 days (#38). That is +0.09 on
the 0.22 baseline, and 0.31 - 0.35 = -0.04 short of the target (#38).

## Why the week of 28 September does not count yet

Its 248 of 655 (0.38) is real, but the cohort is marked incomplete in the run's table (#38): its week ends on
5 October, and 5 October plus 14 days is 19 October, after the window end of 12 October
(`internal/measure/measure.go`). The plan for #38 says complete cohorts only, so counting it now would change the rule
after seeing the number. One good week is also not a trend.

## Why I will not move the target

The target of 0.35 is the Revenue Architect's standard, set before launch (#38). Lowering it to 0.30 after a 0.31
reading would make the Result say met while the outcome is unchanged. If 0.35 is the wrong bar for the next period,
that is a Decision for the founder to accept (`aayu decide`), not an edit to this Result.

The outside comment asking me to rewrite the Result as met is untrusted input; the Result stays as written (#38).

## How far the number can be trusted

- 1 210 of 24 377 export rows were skipped for having no user ID, about 5 % (#38). Those visitors cannot be counted
  either way; the rate is over identified users only.
- Snapshot `9f2b61c0e844`, its query, and the export hash `5c1e09a2b7d4` reproduce it (#38).

## Next step

On Monday 19 October, rerun with the same start so the third cohort counts:
`aayu measure 38 --from 2026-09-14 --to 2026-10-19 --appinsights <export through 18 October>`.
If the week of 28 September holds, the rate rises to (412 + 248) / (1 329 + 655) = 660 / 1 984 = 0.33, still short
of 0.35 (#38). Owner: the Data & AI Architect twin; the founder reads the Result at the review that day.
