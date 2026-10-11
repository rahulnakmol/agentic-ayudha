# Rubric: stub skill, CSV export

Score the Story from 1 to 5. The request has three requirements: (a) the CSV holds every claim matching the applied
filters, (b) only the visible columns, (c) the page keeps responding on lists over 10,000 claims.

Evidence kinds mean: `api-trace` shows request and response content, including the bytes of a downloaded file, so it
can prove a CSV's rows and columns; `screenshot` shows one screen at one moment; `recording` shows an interaction over
time; `metric` is a measured number against a limit; `sr-transcript` is what a screen reader announces. Accept any
evidence kind that can show its criterion by these meanings.

- 5: Each of (a), (b), (c) has an observable criterion with fitting evidence; nothing beyond the request is required;
  summary, scenario, and environment are concrete; parent is Feature #7; effort is plausible.
- 4: As 5, with one minor gap, such as one expectation that could be more precise.
- 3: Usable, but one requirement is missing or one criterion is not observable as written.
- 2: Two requirements missing, invented requirements, or evidence that cannot show its criterion.
- 1: Not a usable Story.
