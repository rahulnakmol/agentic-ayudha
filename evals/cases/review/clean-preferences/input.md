# aayu review 57 (synthetic fixture): state `preferences`, after polish

PR #57 changes the delivery preferences page (`/preferences`) of a grocery web shop. Story #571's screenshot
criterion is the state under review: a signed-in customer on `/preferences`; the page lets them pick a delivery
window and save it, with a help button for what the windows mean.

Lane A (the skill) got the capture only: a screenshot at rest, one after one Tab press, the markup, and the
accessibility tree, at 1280 × 800. It did not see lane B.

Lane B on the same capture, for the judge (impeccable 4.5.2, axe with the WCAG 2.2 AA tags, and the focus-indicator
and 44 px target probes): no findings. Body text is #4f5a6b on #ffffff (7.0:1), every control shows a 2 px outline
in #1f5bd1 on focus, and every control is at least 44 × 44 px. Lane B ships this state.
