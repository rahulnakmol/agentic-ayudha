# polish #64: orders list

Every finding of `.aayudha/review/64/critique.json`, fixed in `orders.html` with design-system tokens:

| Finding (lanes) | Element | Fix |
|---|---|---|
| contrast (A, B: axe, impeccable) | `.row .status` | #9aa1ab (2.6:1, not a token) → status badges on their token pairs: informative (#0b62a0 on #e6f1fa), neutral (#535c69 on #eef0f3), positive (#1c7639 on #e7f4ec), status still in words |
| labels (A, B: axe button-name) | search field, filter button | a visible "Search orders" label tied to the field (the placeholder now only gives an example); the icon button is named "Filter by status" |
| target-size (A, B: probe) | Track buttons | 32 px (compact) → 44 px, the cozy control height, since the page is used on phones |
| copy (A) | Track buttons | each button's name includes its order ("Track order #10482") through visually hidden text; the visible label stays "Track" |

The filter button's behaviour is unchanged, no control was removed or hidden, and no floor was changed. Re-run:
`aayu polish 64`.
