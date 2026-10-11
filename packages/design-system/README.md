# packages/design-system

`@aayudha/design-system` (private, ships as source): semantic tokens, density modes, shadcn/ui primitives with
Phosphor icons, and Storybook with axe on every story (WP6.1). Floorplans (WP6.2) and the SwiftUI and Compose
token exports (WP6.3) are n/a until their PR. See [docs/BUILD-PLAN.md](../../docs/BUILD-PLAN.md) M6.

## Develop

Requires Node.js `^22.12 || ^24 || >=26` and pnpm (version pinned by `packageManager` in the root
`package.json`). From the repository root:

```sh
pnpm install --frozen-lockfile
cd packages/design-system
pnpm storybook                              # http://localhost:6006
pnpm exec playwright install chromium       # once, for the Storybook tests
pnpm test                                   # package policy (Node)
pnpm test:storybook                         # renders every story in Chromium and runs axe
pnpm build-storybook                        # static build in storybook-static/
```

CI runs the same steps in [.github/workflows/design-system.yml](../../.github/workflows/design-system.yml) on
changes under this package.

## Tokens

`tokens/tokens.json` is the only place token values are written. `pnpm tokens` generates
`src/styles/tokens.css`: CSS custom properties named after the token path (`color.text.label` ->
`--ay-color-text-label`). Do not edit the output; `pnpm tokens:check` (CI) fails when it differs from the
generator. Units: px in the source, rem (÷ 16) in CSS; durations ms.

Token names are semantic in the spirit of OpenUI5 theming parameters (brand, shell, field, list, positive,
negative, critical, informative). The names, structure, and values are our own: no SAP theme files, assets,
icons, or fonts are used.

### Density

Two modes define the same names: `cozy` (default, touch: 44 px controls) and `compact` (dense, pointer:
32 px controls). On the web, set `data-density="compact"` on any ancestor. Components size themselves from the density
tokens through Tailwind utilities `h-control`, `h-row`, `size-icon`, `px-control-x`, `gap-stack`,
`p-section`, and `px-page`. Storybook has a Density toolbar; `*Compact` stories run axe in the dense mode.

## Use in an app

```tsx
import "@aayudha/design-system/styles.css" // Tailwind v4 entry: tokens, theme, shadcn variable mapping
import { Button } from "@aayudha/design-system"
import { animate } from "@aayudha/design-system/motion" // needs gsap
```

Peer dependencies: `react` and `react-dom` 19, `tailwindcss` 4 (the app compiles `styles.css`), and `gsap` 3
(optional; only for `/motion`).

## Primitives

`src/components/ui/` is shadcn/ui source (MIT) copied with the shadcn CLI and then changed: Lucide icons
replaced by Phosphor, fixed heights and paddings replaced by density tokens, form fields on the field
background token, and object-status `Badge` variants (`positive`, `negative`, `critical`, `informative`,
`neutral`). `src/styles/index.css` maps the shadcn variable names (`primary`, `muted`, `border`, ...) onto the
semantic tokens, so components never use raw colours. Add a component with
`pnpm dlx shadcn@latest add <name>` from this directory, then make the same changes and turn its `@/`
imports into relative ones (the package ships as source, so consumers cannot resolve the alias). Check the
diff to `package.json`: the CLI may add packages (it added an unrelated `cn` package once).

## Motion and GSAP

GSAP is a **peerDependency and is never vendored or bundled** (BUILD-PLAN Appendix A J3-19);
`test/package-policy.test.ts` enforces this. `src/motion/` plays presets (`fade-in`, `slide-up`,
`slide-in-end`, `expand`) with the motion tokens, converting the token cubic-bezier curves into GSAP eases so
GSAP and CSS share timing. With `prefers-reduced-motion: reduce` the end state is applied at once.

The Storybook motion controls are **not a visual animation builder**: they only pick existing presets and
tokens to preview them, and cannot create, record, edit, or export animations. Animations are written in
code. This matters because GSAP's licence prohibits use in no-code visual animation builders; keep it that
way.

## Accessibility

`.storybook/preview.tsx` sets `parameters.a11y.test = "error"`, so `pnpm test:storybook` fails on any axe
violation in any story, including portalled content opened by a play function (see the Dialog story). Colour
pairs in `tokens.json` meet WCAG 2.2 AA (4.5:1 text, 3:1 field borders).
