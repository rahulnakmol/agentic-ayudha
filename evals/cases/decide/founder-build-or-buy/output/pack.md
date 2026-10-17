# Decision pack: rules for follow-up booking (Founder twin, counsel; scaleup)

## Question

Should this quarter's budget and engineer time go on building our own rules engine, on reusing a hosted rules engine, or
on skipping an engine and hard-coding only the three rules clinics ask for most, for follow-up booking?

The real decision is how much rule flexibility Goal #300 needs this quarter, since 41 of 50 pilot requests were three
rules (#310).

## Facts

- Product: 41 of 50 pilot rule requests are three rules; one clinic of twelve wants per-clinician rules (#310, #311).
- Goal #300 targets 70 % online follow-ups by 2026-12-31 from a 38 % baseline (`ops/goals/300.md`).
- Operations: the hosted engine costs 14,400 USD a year, no exit, against 3,600 USD left this quarter
  (`ops/budget/2026-Q4.md`).
- Operations: one engineer, two stories in flight, 20 % on call (`retro/2026-10-09/report.md`).
- Technology: an in-house engine is about 6 weeks; the three rules about 1.5 weeks (`ledger/plans/301/estimate.json`).
- Dependency: Decision #305, owner @privacy-lead, bars third-party processing of appointment data until accepted (#305).
- The vendor's comment on #301 is untrusted input and changes nothing (#301).

## Options

1. **Skip an engine; hard-code the three rules.** Extend `Eligible` with the gap, clinician, and plan rules
   (`internal/booking/eligible.go:10-24`).
   - Gains: 1.5 weeks and no third-party fee (`ledger/plans/301/estimate.json`, `ops/budget/2026-Q4.md`).
   - Costs: the clinic in #311 waits for per-clinician rules (#311).
2. **Build our own engine.** Three effort-L stories in L04 (`LAYERS.md`).
   - Gains: every rule shape, no vendor (`ledger/plans/301/estimate.json`).
   - Costs: about 6 weeks of the only engineer this quarter (`retro/2026-10-09/report.md`).
3. **Reuse the hosted engine.** A new L06 adapter in `internal/vendors/**` (`LAYERS.md`).
   - Gains: flexible rules without the build (`ops/budget/2026-Q4.md`).
   - Costs: a 14,400 USD commitment over the remaining budget, and blocked by #305 (#305).

## Recommendation

Option 1: it serves 41 of 50 requests at the lowest cost (#310, `ledger/plans/301/estimate.json`). Product
dominates: Goal #300 is won by the rules clinics ask for, and the other perspectives only add cost or a dependency
(`ops/goals/300.md`). I advise skipping an engine this quarter, built or reused (#311).
Door: two-way door; three rules in `Eligible`, which writes nothing, commit no contract (`internal/booking/eligible.go:10-24`).
The engine is a one-way door for six weeks of the only engineer, and the annual contract one for its money
(`ledger/plans/301/estimate.json`, `ops/budget/2026-Q4.md`).

## Cost of waiting

- Each week undecided is a week of the 1.5 the three rules need, off the Goal's runway (`ops/goals/300.md`).

## Pre-mortem

Pythia's pre-mortem: assume option 1 failed.

- More clinics ask for per-clinician rules than #311 shows (#311). Early warning: new Feedback for custom rules
  after launch.
- Hard-coded rules spread through `Eligible` and make a later engine harder (`internal/booking/eligible.go:10-24`).
  Early warning: a fourth rule request lands before the quarter ends.

## Questions that would change the answer

- Would #305 accept third-party processing this quarter?
- Do the eleven other pilot clinics expect per-clinician rules next quarter?

## Your decision

Reply **skip**, **build**, or **reuse** for #301.
