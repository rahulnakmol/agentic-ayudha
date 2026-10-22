# Rubric: mockup, orders list delivery status

Score the mockup stage from 1 to 5: `stage.json` and one SVG or HTML mockup per wireframe state, built only from the
design system. You read the mockups' source; you cannot see them rendered, so judge what the markup and styles
determine (structure, semantics, token use, sizes, states, copy).

Already checked before you see it (do not re-check): every colour literal is a design-token value, every wireframe
state (`list`, `empty`, `error`) has a mockup, and on each HTML mockup in Chromium lane B found no contrast, focus
indicator, 44 px target, label, or keyboard problem.

What strong mockups have:

- Each mockup follows its wireframe and story: order number, date, total, and delivery status, newest first; "No
  orders yet" with a primary "Start shopping"; an error that names the failure, with Retry and no stale rows.
- The design system used as a system, not imitated: `--ay-*` tokens for colour, type scale, spacing, radii, and
  control and row heights (cozy density, since the page is used on phones); status as a badge-style label whose
  colours are the status token pairs (positive, informative, neutral), with the status in words.
- Semantics that carry the research into the build: a real table with a caption and column headers (P3), an error
  announced as an alert, one primary action per state, links and buttons used for what they are.
- A layout that holds at 390 px (P5) without hiding content, and a focus indicator in the focus token.
- Copy in the user's words, consistent across states (the same page title, the same names for statuses), and no
  feature the stories do not ask for.

- 5: All of the above in every state; a developer could build the prototype from these without guessing.
- 4: As 5 with one minor gap, such as one state's mobile layout unspecified or one hard-coded size where a token
  exists.
- 3: Usable, but one state drifts from its wireframe or story, or semantics are missing in one state (a div table, an
  error that is not announced).
- 2: Several states drift, the design system is imitated rather than used, or the research findings are ignored.
- 1: Not usable as mockups.
