# Rubric: grill conversation, "The PDP needs a CTA for B2B catering orders, and keep PII out of the logs"

You see a whole grill conversation and the Story it produced. The request is abbreviation-heavy on purpose (PDP, CTA, B2B, PII) and names no outcome, in a synthetic bakery app that has no product pages, only product tiles on the menu (`component:ProductTile`). A scripted human answered each question by its
topic; each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`, or `defer`.
The assessment's `gate` said `unpack`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the skill unpacks every abbreviation first, asks about the ambiguous ones (PDP, which does not fit this app; B2B, which has two meanings) before anything else, grounds them in the repository, and then works out who it is for, the outcome, and the smallest slice.
2. Concreteness of the spec: every criterion names its state and something to observe (a button by its label, a count of fields, an API result, a count of log lines); Measure names what is counted (catering requests through the form per week) and Threshold carries the human's number (5 a week); a baseline the human never gave is not invented; each term has its confirmed meaning in Provenance. The readback says in plain language what done looks like: the goal, the
   criteria, and the checkpoints the human will see.
3. Fidelity: the meaning of each term is the human's, not the jargon list's; the button label and the form fields are the human's; prices, invoices, company accounts, and delivery are out and are not criteria. A criterion checks one representative instance (seeded data, a synthetic user, a fresh home directory, one seeded declining card for "a payment fails"): that is test setup, not an added requirement or a narrowing, as long as the expected result keeps the human's whole scope. It narrows only when its wording limits the behaviour itself to a subset the human did not choose ("only for guests", "only going forward"). Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its state.
- 2: A detail the human did not give (a number, a place, a name), a parked or deferred topic as a criterion, or a
  criterion still vague.
- 1: Not a usable spec.
