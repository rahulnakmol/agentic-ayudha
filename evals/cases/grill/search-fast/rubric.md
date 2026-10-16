# Rubric: grill conversation, "make search fast"

You see a whole grill conversation and the Story it produced. The request is vague on purpose: "make search fast"
in a synthetic bakery app whose `/menu/search` filters every product on each keystroke (`code:lib/search.ts`) and has
no API budget or search test (`gap:api-budget`, `gap:tests:/menu`). A scripted human answered each question by its
topic (`dimension`); each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`,
or `defer`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the questions turn "fast" into an operation, a percentile, a number, conditions, and a way to
   measure it; they are grounded in the assessment and asked riskiest first, each with a default or options and a
   reason.
2. Concreteness of the spec: the criteria name the operation, the percentile, the limit the human gave (200 ms at
   p95), the conditions (the seeded menu), and a fitting evidence kind (`metric`); Measure and Threshold agree with
   them; nothing still says just "fast". The readback says in plain language what done looks like.
3. Fidelity: the number and percentile are the human's, not the skill's; caching and new loading feedback (parked)
   and a CI budget (deferred) are not criteria and appear in Parked ideas or Open questions; Provenance names the
   right source. Test setup that a criterion's evidence needs (seeded data, a synthetic user, a fresh home directory) is not an added requirement; a state that narrows what the human asked for is. Questions and parked ideas never lower the score.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its conditions.
- 2: A number the human did not give, a parked or deferred topic as a criterion, or a criterion still vague.
- 1: Not a usable spec.
