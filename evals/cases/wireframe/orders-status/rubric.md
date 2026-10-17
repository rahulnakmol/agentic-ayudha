# Rubric: wireframe, orders list delivery status

Score the wireframe stage from 1 to 5: `stage.json` and one low-fidelity wireframe per state. Wireframes show
structure, content, and controls, without styling, so the next stage (design-system mockups) can follow them. The
research stage on the ledger and the stories are the only sources.

Already checked before you see it (do not re-check): the research stage is pinned, every state the Completion Goals
name (`list`, `empty`, `error`) has a wireframe, and the files are of a checkable kind.

What strong wireframes have:

- Each state does what its story's Completion Goal says: `list` shows order number, date, total, and delivery status,
  newest first; `empty` says "No orders yet" with a primary "Start shopping" button; `error` names the failure, offers
  Retry, and shows no stale list.
- The research is used, visibly: status in words (P2), a caption and column headers for the table (P3), a state that
  never looks broken (P4), and a layout at 390 px with no overlapping columns (P5).
- Accessibility designed in, not bolted on: focus order and the first focus stop, what a screen reader announces
  (headings, the error as an alert), status never shown by colour alone, controls big enough to tap.
- Real content in the user's words rather than lorem ipsum, and nothing the stories and research do not support
  (no invented features such as filters, maps, or live tracking).
- No styling decisions (colours, fonts) that belong to the mockup, and no copy of a reference's layout.

- 5: All of the above for every state, concise enough to mock up from directly.
- 4: As 5 with one minor gap, such as one state missing its 390 px or keyboard note.
- 3: Usable, but one state misses part of its Completion Goal, or the research findings are barely used.
- 2: States that contradict their stories, invented features, or no accessibility thinking.
- 1: Not usable as wireframes.
