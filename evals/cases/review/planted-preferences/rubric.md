# Rubric: review lane A, delivery preferences with three planted defects

Score lane A's critique of one captured state from 1 to 5. Lane A judges the state against Nielsen's 10 heuristics
and walks it as four personas (keyboard-only, screen-reader, low-vision, first-time), from two screenshots, the
markup, and the accessibility tree. You see the markup, the tree, and lane B's measured results (in the task), not
the screenshots.

Already checked before you see it (do not re-check): each defect lane B measured (no focus indicator, 3.8:1 body
text, a 32 px help button) is reported at severity 2 or more, every finding's severity is within its heuristic's
score, and all four personas are walked.

What a strong critique has:

- Each real defect stated once with evidence a developer can act on: the element (selector or label), what the
  capture or CSS shows (the `outline: none` rule, the colour pair and its ratio, the 32 × 32 size), and who it hurts.
- Severities calibrated to the users harmed: no visible focus blocks keyboard users (3 or 4); low-contrast help text
  and a small target next to Save are real but lesser (2 or 3). Not everything at 4, and nothing inflated.
- Fixes inside the design system: the focus token ring on `:focus-visible`, a text token that meets 4.5:1, the
  44 px control size; never "remove the help button" or a colour outside the tokens.
- Persona walks that match the findings (keyboard-only blocked by the missing focus ring; screen-reader not blocked,
  since the names are in the tree).
- No invented problems and no taste: cosmetic notes stay at severity 0 or 1, and nothing contradicts lane B.
  `rebuild` is false: the flow is sound and needs fixes, not a new design.

- 5: All of the above, specific and correctly calibrated.
- 4: As 5 with one minor lapse, such as one severity a point off or one fix vaguer than it could be.
- 3: Usable, but a finding is vague (no element or evidence), severities are miscalibrated, or a cosmetic or invented
  problem is reported at severity 2 or more.
- 2: Several findings unusable, persona walks contradicting the findings, or fixes outside the design system.
- 1: Not a usable critique.
