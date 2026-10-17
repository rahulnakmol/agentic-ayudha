# Rubric: mockup, choosing a delivery window

Score the mockup stage from 1 to 5: `stage.json` and one SVG or HTML mockup per wireframe state, built only from the
design system. You read the mockups' source; you cannot see them rendered, so judge what the markup and styles
determine (structure, semantics, token use, sizes, states, copy).

Already checked before you see it (do not re-check): every colour literal is a design-token value, every wireframe
state (`default`, `slot-full`, `saved`) has a mockup, and on each HTML mockup in Chromium lane B found no contrast,
focus indicator, 44 px target, label, or keyboard problem.

What strong mockups have:

- Each mockup follows its wireframe and story: windows with availability, a full one that cannot be chosen, the
  chosen one marked; a message next to the list naming the window that filled up, with the nearest free windows
  offered and the choice not lost silently; a confirmation naming the day and window.
- A single choice built as one: native radio inputs in a fieldset with a legend, each labelled by its time and
  availability, sized to the 44 px target with tokens; a full window disabled and saying "Full" in words.
- The full-window message announced (an alert) and placed where the eye and focus go, in the critical or negative
  token pair, with its first words naming the window.
- The design system used as a system: `--ay-*` tokens for colour, type, spacing, radii, and control heights (cozy
  density for a phone-used page); one primary action per state.
- Nothing settled that the research plan leaves open, or such choices marked as assumptions (for example a change
  cut-off nobody confirmed); no invented features (fees, addresses, payment).

- 5: All of the above in every state; ready for the planned usability test and for a prototype.
- 4: As 5 with one minor gap.
- 3: Usable, but one state drifts from its wireframe or story, or the full-window message is not announced.
- 2: Several states drift, the design system is imitated, or invented features.
- 1: Not usable as mockups.
