# Rubric: grill conversation, "Make the quotes API fast"

You see a whole grill conversation and the Story it produced. The request is vague on purpose: "slow" and "fast" for an HTTP API (`product:api`), a synthetic shipping-quotes service with POST /quotes and GET /quotes/{id} (`route:/quotes`) and no load test or latency budget (`gap:api-budget`). A web app's questions (screens, page loads, spinners) do not apply. A scripted human answered each question by its
topic; each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`, or `defer`.
The assessment's `gate` said `unpack`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the questions are about an API: which endpoint, the percentile and number, the request rate and payload, how it is measured, and whether amounts may change; they cite the route and budget facts and come riskiest first.
2. Concreteness of the spec: the latency criterion names the endpoint, the load (rate, parcels, duration), the seeded data, and the percentile with its number; the amounts criterion names its state and an exact comparison; nothing still says "fast" or "slow". The readback says in plain language what done looks like: the goal, the
   criteria, and the checkpoints the human will see.
3. Fidelity: 300 ms at p95, 10 parcels, 20 a second, the one-minute local load test, and unchanged amounts are the human's; the warm-up request is the accepted default; a CI budget and loading feedback (parked) are not criteria. Test setup that a criterion's evidence needs (seeded data, a synthetic user, a fresh home directory) is not an added requirement; a state that narrows what the human asked for is. Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its state.
- 2: A detail the human did not give (a number, a place, a name), a parked or deferred topic as a criterion, or a
  criterion still vague.
- 1: Not a usable spec.
