# Rubric: prototype, orders list delivery status

Score the prototype stage from 1 to 5: `stage.json`, the flag definition, the product code, and the Code Connect
files. You read the source; you cannot run it or see it rendered, and the figma-diff gate (pixels against the Figma
frames) was not run, so judge what the code determines.

Already checked before you see it (do not re-check): every mockup state (`list`, `empty`, `error`) has a prototype
state with a URL path and a Figma frame, the flag is a flagd boolean that is off by default, every Figma component
listed has a Code Connect mapping, and every colour literal in the code is a design token.

What a strong prototype has:

- Working UI in the product, not a mockup copy: the route reads the flag through OpenFeature (`useBooleanFlagValue`
  with a false default), keeps today's page when it is off, and loads real data from `GET /api/orders`.
- Each state reachable and faithful to its mockup and story: status in words on its status token pair; "No orders
  yet" with "Start shopping"; an error with Retry that never shows stale rows; each state's path lets the preview
  target show it (a seed or fixture that production ignores).
- Built from design-system components and tokens (Badge, Table, Button, Alert, Card; density-following sizes such as
  `h-control`), with the research's accessibility carried through: caption and column headers, the error announced,
  44 px targets, a visible focus ring, a layout that holds at 390 px.
- The flag turned on only for the capture environment (`preview`), never production, through flagd targeting.
- Code Connect that maps each Figma component to the code that implements it, with props and an example a designer
  could use, and components the Figma file names (frame and component nodes) used as given.
- Nothing beyond the stories (no tracking map, no filters) and nothing broken by the flag when it is off.

- 5: All of the above; a reviewer could run review on it next.
- 4: As 5 with one minor gap, such as one state's seed unexplained or one Code Connect file without props.
- 3: Usable, but a state misses part of its story, the flag is not wired into the route, or an accessibility
  property of the mockup is lost.
- 2: Not product code (a static copy of the mockup), flag misuse, or several states broken.
- 1: Not a usable prototype.
