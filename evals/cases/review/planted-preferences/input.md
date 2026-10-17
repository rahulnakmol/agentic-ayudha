# aayu review 57 (synthetic fixture): state `preferences`

PR #57 changes the delivery preferences page (`/preferences`) of a grocery web shop. Story #571's screenshot
criterion is the state under review: a signed-in customer on `/preferences`; the page lets them pick a delivery
window and save it, with a help button for what the windows mean.

Lane A (the skill) got the capture only: a screenshot at rest, one after one Tab press, the markup, and the
accessibility tree, at 1280 × 800. It did not see lane B.

Lane B on the same capture, for the judge (impeccable 4.5.2, axe with the WCAG 2.2 AA tags, and the focus-indicator
and 44 px target probes):

- contrast: body text `p` is #838383 on #ffffff, 3.8:1 (axe color-contrast, serious; impeccable low-contrast).
- focus-visible: focusing "Delivery window", "Save preferences", and "Help with delivery windows" changes nothing
  visible (`button:focus, select:focus { outline: none; }`).
- target-size: "Help with delivery windows" is 32 × 32 px; the floor is 44 × 44.

Nothing else was found. The page's tokens: text #1b2330, label text #4f5a6b, focus #1f5bd1.
