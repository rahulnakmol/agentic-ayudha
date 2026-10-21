# Rubric: polish, orders list toolbar and rows

Score the polish from 1 to 5: the rewritten page and `polish.md`, the host's note on the PR. polish applies a review
critique by fixing causes with design-system tokens and components, never by lowering a floor, hiding an element from
the capture, or waiving a finding. You read the source; you cannot see it rendered.

Already checked before you see it (do not re-check): the page uses token colours only and keeps every control,
`polish.md` names the contrast, labels, and target-size findings, and lane B in Chromium (axe, impeccable, the
focus-indicator and 44 px target probes) finds no a11y problem on the page.

The critique has four lane A findings and four lane B findings. Lane B checks only what it can measure; the lane A
findings it cannot measure still count:

- contrast: status text in words on a token pair that meets 4.5:1 (the status pairs: informative, neutral,
  positive), keeping the status readable as text, not colour alone.
- labels: a visible, programmatically tied label for search (a placeholder is not a label) and an accessible name for
  the icon-only filter button that says what it does.
- target-size: the Track buttons at the 44 px control height (cozy), since the page is used on phones.
- copy: each Track button identifiable out of context (its accessible name includes the order), without changing
  what sighted users see unless that is the better fix.

What else a strong polish has: a minimal diff (the filter's behaviour, the rows, and the copy otherwise unchanged);
tokens named for what they are; and a `polish.md` that maps every finding, lane A's copy finding included, to its fix,
says nothing was waived or hidden, and how to re-run.

- 5: All four findings fixed at their causes, minimal diff, a note a reviewer can approve from.
- 4: As 5 with one minor gap.
- 3: The lane B findings fixed but the lane A copy finding ignored, or collateral changes.
- 2: A fix that games the check (hiding, aria-hidden on controls, removing the filter button), or several findings
  unaddressed.
- 1: Not a usable polish.
