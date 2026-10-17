# Counsel grounding: should we split the survey into sections? (synthetic fixture product repository o/clinic, 2026-10-17)

Everything below is synthetic and was written for this eval. It is what `aayu counsel "should we split the survey into
sections?"` printed (the facts it found on GitHub) plus the metric, ledger, and code excerpts the twin read before
counselling.

## Facts from `aayu counsel`

- Goal #320 "Patients finish the post-visit survey": open; baseline 41 % of started surveys finished in September,
  target 65 % by 2026-12-31 (`ops/goals/320.md`)
- Feedback #330 "The survey is too long": eleven patients in September comments said they gave up part-way; three
  said they lost their answers when the page timed out (#330)
- Feedback #331 "Ask fewer questions": the clinic manager at Northside asks to drop the eight optional questions
  (#331)
- Open Decision: #325 "How long we keep survey answers": open since 2026-10-10, assignee @privacy-lead; it decides
  the retention of partial answers, among others (#325)
- L01 Experience / UI, owner UX Researcher & Evolutionist: `web/survey/**` (LAYERS.md)
- L07 Operational data / state, owner AI Native Engineer: `internal/survey/store/**` (LAYERS.md)
- Layers that always need a person's review: L06, L07 (CONSTRAINTS.md)

## Metric, ledger, and code excerpts

- Metric `ops/metrics/survey-2026-09.md`: 1,240 surveys started, 508 finished (41 %); of the 732 abandoned, 590
  stopped after question 9 or later; median time to finish 6 minutes 40 seconds.
- `web/survey/Form.tsx:14-52`: the survey renders all 24 questions (16 required, 8 optional) on one page.
- `internal/survey/store/answers.go:8-30`: `store.SaveAnswers` writes the answers once, on submit; nothing is saved
  before then, so a timed-out page loses every answer.
- Estimate `ledger/plans/320/estimate.json` (Code Architect twin): sections in the UI only are 1 story of effort S;
  saving partial answers per section is 1 story of effort M in L07, and its retention depends on #325.
- Experiment template `ops/experiments/README.md`: an A/B experiment on survey completion needs two weeks at the
  current volume to detect a 10-point change.
