# Rubric: grill conversation, "high-quality UX for ordering"

You see a whole grill conversation and the Story it produced. The request is vague on purpose: "high-quality UX for
ordering" in a synthetic bakery app. The skill had a read-only assessment of the repository (fact IDs such as
`flow:/order`, `gap:error:/order`, `route:/order/review`). A scripted human answered each question by its topic
(`dimension`); each answer has a `kind`: `confirm` (the human stated or agreed to something specific), `default`
(the human accepted the default the skill offered), `park` or `defer` (not in this story). Score three things, then
give one overall score from 1 to 5.

1. Question quality: the questions are relevant to turning "high-quality UX" into something buildable, grounded in
   the assessment (they cite or use its facts, such as the five-screen flow or the missing error state), and
   prioritized (the riskiest decisions, such as the task, the steps, and the failure states, come first). Each
   question offers a default or options with a reason, and rounds hold at most five questions.
2. Concreteness of the spec: every criterion is observable with a fitting evidence kind, names its state, and states
   a number, a count, a named state, or exact wording; nothing still says "high quality", "intuitive", or "smooth".
   The readback says in plain language what done looks like.
3. Fidelity: every criterion comes from something the human confirmed or accepted as a default; nothing the human
   parked or deferred (speed, accessibility beyond the floors, offline, exact wording, the loyalty idea) is a
   criterion; parked and deferred topics appear in Parked ideas or Open questions; the Provenance names the right
   source for each criterion. Test setup that a criterion's evidence needs (seeded data, a synthetic user, a fresh home directory) is not an added requirement; a state that narrows what the human asked for is. Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong: grounded, prioritized questions; a fully concrete spec; complete fidelity.
- 4: One minor gap in one area (a question that ignores an obvious fact, one criterion that could be more precise).
- 3: Usable, but one area is weak: generic questions that ignore the repository, or one criterion that is not
  observable as written.
- 2: A criterion the human did not confirm, a parked idea in the criteria, or several vague criteria.
- 1: Not a usable spec.
