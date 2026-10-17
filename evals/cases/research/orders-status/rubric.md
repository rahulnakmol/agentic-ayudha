# Rubric: research, orders list delivery status (startup)

Score the research stage from 1 to 5: `stage.json` and the Markdown notes it names. The skill gathers pinned
references (URLs and IDs only) and writes what was learned from users, for a team at the startup stage, where the
artefact is a Job story ("When …, I want …, so I can …", with one quote or data point behind it) in at most 400
words. The task's research data and search results are the only sources.

Already checked before you see it (do not re-check): every reference is an https page URL with an ID and one the
host gathered (never an image), quotations are word for word from the data, and the notes have the Job story
heading within 400 words.

What a strong research stage has:

- A Job story in the customer's situation and words, grounded in the data (a ticket quote, the 29 of 41 count, or
  the 38 % chat rate), not in the solution ("I want a status column" is a solution, not a motivation).
- Findings that say what people did, each traceable to a session, ticket, or metric, with no finding the data does
  not contain. The screen-reader finding (P3: no caption or headers) and the 390 px finding (P5) are accessibility
  evidence and must not be dropped.
- Implications tied to the three Completion Goal states (`list`, `empty`, `error`), without designing the screens.
- Honest open questions for what the data does not settle (which statuses the shop can report, delivery time or
  status only), rather than guesses stated as findings.
- References chosen because they bear on the job (order tracking, summary lists, empty states, table semantics),
  with a reason for each, and the image results dropped, saying why.

- 5: All of the above, concise and specific; a designer could start wireframes from it.
- 4: As 5 with one minor gap, such as one implication not tied to a state or one reference without a reason.
- 3: Usable, but a finding is overstated or generalised beyond the data (for example "most users" from five
  sessions), an accessibility finding is missing, or the Job story describes a feature instead of a situation.
- 2: Invented or distorted findings, solutions presented as research, or the states ignored.
- 1: Not a usable research stage.
