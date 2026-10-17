# Rubric: polish, delivery preferences with three planted defects

Score the polish from 1 to 5: the rewritten page and `polish.md`, the host's note on the PR. polish applies a review
critique by fixing causes with design-system tokens and components, never by lowering a floor, hiding an element from
the capture, or waiving a finding. You read the source; you cannot see it rendered.

Already checked before you see it (do not re-check): the page uses token colours only and keeps every control,
`polish.md` names every kind of finding, and lane B in Chromium (axe, impeccable, the focus-indicator and 44 px
target probes) finds no a11y problem on the page.

What a strong polish has:

- Each finding fixed at its cause: the `outline: none` rule replaced by a `:focus-visible` ring in the focus token on
  every control; the help text moved to a text token that meets 4.5:1 (the label text token); the help button sized
  to the 44 px control height. Not by enlarging only the hit area of one control, removing the help button, or adding
  `aria-hidden`.
- A minimal diff: nothing else changes (copy, layout, behaviour), so the re-run compares like with like.
- The tokens named for what they are (`--ay-color-text-label`, `--ay-color-focus`, the control height), so the fix
  survives a token change; a literal equal to a token value is acceptable only where the page has no token stylesheet,
  and then the note says which token it is.
- `polish.md` maps every finding (lane, element) to its fix, says nothing was waived or hidden, and how to re-run.

- 5: All of the above; a reviewer could approve from the note and the diff.
- 4: As 5 with one minor gap, such as one fix not tied to its token in the note.
- 3: Fixed, but with collateral changes, a fix that treats the symptom (an outline on one control only), or a note
  that does not map findings to fixes.
- 2: A fix that games the check (hiding, shrinking the capture, a waiver), or several findings unaddressed in the note.
- 1: Not a usable polish.
