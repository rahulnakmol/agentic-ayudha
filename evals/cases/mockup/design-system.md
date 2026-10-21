# The design system the host reads (`packages/design-system`, summarised for the eval)

`@aayudha/design-system`: semantic tokens, two density modes, shadcn/ui primitives, and seven floorplans. The token
values are in `src/styles/tokens.css` (given separately); `tokens/tokens.json` is the only place values are written,
and `[design] tokens` points at it.

## Rules that apply to every UI

- Colours come only from tokens: `var(--ay-color-…)`. A colour literal that is not a token value fails `aayu
  mockup` (exit 3) and the prototype's figma-diff gate.
- Type comes from the scale: `--ay-font-size-*` (small 12, body 14, large 16, header-3 20, header-2 24, header-1 32
  px) and `--ay-font-weight-*`; the family is `--ay-font-family-sans`.
- Floors (`CONSTRAINTS.md`, `[verify.floors.a11y]`): WCAG 2.2 AA, body text contrast at least 4.5:1, a visible focus
  indicator on every control that takes focus (the system's ring is `--ay-color-focus`), and touch targets at least
  44 × 44 CSS px (`min_target_px = 44`). Lane B of `review` measures all three in Chromium.
- Density: `cozy` (default, touch) gives 44 px controls (`--ay-size-control-height`, `--ay-size-row-height` 48 px);
  `compact` gives 32 px controls and is for dense pointer-only screens, never for a page used on phones.
- Spacing and radii: `--ay-space-*` and `--ay-radius-*`; motion: `--ay-motion-*`.

## Components (`src/components/ui`)

alert, avatar, badge, breadcrumb, button, card, checkbox, dialog, input, label, progress, select, separator, switch,
table, tabs, textarea. Button sizes `default` and `icon` follow the density (44 px cozy); `xs`, `sm`, `icon-xs`, and
`icon-sm` are fixed small sizes (24–32 px) for compact screens only. Badge variants map to the status tokens
(positive, negative, critical, informative, neutral). There is no radio group component yet: a single choice uses
native radio inputs styled with tokens.

## Floorplans (`src/floorplans`)

shell, list-report (a filterable list of objects), object-page (one object's details), worklist, overview-page,
initial-page, wizard, flexible-column-layout.

## Mockups in HTML

A mockup page loads the tokens with `<link rel="stylesheet" href="tokens.css">` (the stage serves the design system's
`tokens.css` next to each page) and styles everything with the `--ay-*` custom properties.
