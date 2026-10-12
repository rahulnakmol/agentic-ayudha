# Rubric: grill story, empty claims list

Score the Story from 1 to 5. The request has two requirements: (a) on the empty claims page a new claimant is told
they have no claims yet, and (b) the page shows them how to start a claim. The Story belongs under Feature #30.

Evidence kinds mean: `screenshot` shows one screen at one moment; `recording` shows an interaction over time;
`api-trace` shows a request and its response; `metric` is a measured number against a limit; `sr-transcript` is
what a screen reader announces. Accept any evidence kind that can show its criterion by these meanings.

Questions the skill lists under "open questions" are not requirements: they never lower the score. Wording that
restates the request ("you have no claims yet", "start a claim") is not an invented requirement. Invented
requirements are specifics the request does not state: an API or endpoint, a performance limit, a layout, colours,
analytics, or any number of the skill's own.

- 5: (a) and (b) each have an observable criterion with fitting evidence; nothing beyond the request is required;
  the state names a claimant with no claims; summary, scenario, and environment (seeded, synthetic data, never
  production) are concrete; parent is Feature #30; effort is plausible (XS or S).
- 4: As 5, with one minor gap, such as one expectation that could be more precise.
- 3: Usable, but one requirement is missing or one criterion is not observable as written.
- 2: Both requirements missing, invented requirements, or evidence that cannot show its criterion.
- 1: Not a usable Story.
