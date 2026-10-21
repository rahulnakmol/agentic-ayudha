# Rubric: review lane A, delivery preferences after polish (clean)

Score lane A's critique of one captured state from 1 to 5. Lane A judges the state against Nielsen's 10 heuristics
and walks it as four personas (keyboard-only, screen-reader, low-vision, first-time), from two screenshots, the
markup, and the accessibility tree. You see the markup, the tree, and lane B's measured results (in the task), not
the screenshots.

Already checked before you see it (do not re-check): no focus-indicator, contrast, or target-size finding at
severity 2 or more (lane B measured all three clean), every finding's severity is within its heuristic's score, and
all four personas are walked.

This state is polished: its markup is semantic, its controls are labelled, focus is visible, contrast and targets
meet the floors. A polished page has no findings at severity 2 or above (lane A's own instruction).

What a strong critique of a clean state has:

- No finding at severity 2 or more unless the markup or tree shows a problem a real user would hit; if there is one,
  it names the element and the evidence. A real but small observation (the "?" glyph's visible meaning) is at most
  severity 1.
- Persona walks that say what each persona can do, consistent with the capture (no one blocked).
- No invented or speculative problems (things the state does not show, such as behaviour after Save, are not
  findings above severity 1), no taste, `rebuild` false, and nielsen scores that are low and consistent.

- 5: A calibrated, specific critique with nothing at severity 2 or more, or only a well-evidenced real problem.
- 4: As 5 with one minor lapse, such as a vague cosmetic note.
- 3: One speculative or taste finding at severity 2, or persona walks that do not match the capture.
- 2: Several invented problems, or `rebuild` true.
- 1: Not a usable critique.
