# packages/design-system

`@aayudha/design-system` (private, ships as source): semantic tokens, density modes, shadcn/ui primitives with
Phosphor icons, and Storybook with axe on every story (WP6.1); seven Fiori-shaped floorplans (WP6.2); SwiftUI
and Compose token exports with a parity test (WP6.3). See [docs/BUILD-PLAN.md](../../docs/BUILD-PLAN.md) M6.

## Develop

Requires Node.js `^22.12 || ^24 || >=26` and pnpm (version pinned by `packageManager` in the root
`package.json`). From the repository root:

```sh
pnpm install --frozen-lockfile
cd packages/design-system
pnpm storybook                              # http://localhost:6006
pnpm exec playwright install chromium       # once, for the Storybook tests
pnpm test                                   # token parity and package policy (Node)
pnpm test:storybook                         # renders every story in Chromium and runs axe
pnpm build-storybook                        # static build in storybook-static/
```

CI runs the same steps in [.github/workflows/design-system.yml](../../.github/workflows/design-system.yml) on
changes under this package.

## Tokens

`tokens/tokens.json` is the only place token values are written. `pnpm tokens` generates:

| Output | Platform | Names |
|---|---|---|
| `src/styles/tokens.css` | Web, CSS custom properties | `--ay-color-text-label` |
| `platforms/swiftui/AyudhaTokens.swift` | SwiftUI (iOS 17 / macOS 14) | `AyudhaTokens.colorTextLabel` |
| `platforms/compose/AyudhaTokens.kt` | Jetpack Compose | `AyudhaTokens.colorTextLabel` |

Do not edit the outputs; `pnpm tokens:check` (CI) fails when they differ from the generator.
`test/parity.test.ts` parses each output on its own and fails unless every platform has the same names with
the same values as `tokens.json`. Units: px in the source; rem (÷ 16) in CSS, points in SwiftUI, dp (sp for
font sizes) in Compose; durations ms in CSS and Compose, seconds in SwiftUI. The `web` group (font stacks) is
CSS-only; native platforms use the system font.

Token names are semantic in the spirit of OpenUI5 theming parameters (brand, shell, field, list, positive,
negative, critical, informative). The names, structure, and values are our own: no SAP theme files, assets,
icons, or fonts are used.

### Density

Two modes define the same names: `cozy` (default, touch: 44 px controls) and `compact` (dense, pointer:
32 px controls). On the web, set `data-density="compact"` on any ancestor; natively, use
`AyudhaTokens.densityCozy` or `AyudhaTokens.densityCompact`. Components size themselves from the density
tokens through Tailwind utilities `h-control`, `h-row`, `size-icon`, `px-control-x`, `gap-stack`,
`p-section`, and `px-page`. Storybook has a Density toolbar; `*Compact` stories run axe in the dense mode.

## Use in an app

```tsx
import "@aayudha/design-system/styles.css" // Tailwind v4 entry: tokens, theme, shadcn variable mapping
import { Button, ListReport } from "@aayudha/design-system"
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

## Floorplans

`src/floorplans/` composes the primitives into seven Fiori-shaped floorplans: initial page, list report,
worklist, object page, overview page, wizard, and flexible column layout. They are layout components with
slots (title, filters, table, sections, steps, columns); content comes from the app. Stories under
`Floorplans` show each with synthetic data.

## Motion and GSAP

GSAP is a **peerDependency and is never vendored or bundled** (BUILD-PLAN Appendix A J3-19);
`test/package-policy.test.ts` enforces this. `src/motion/` plays presets (`fade-in`, `slide-up`,
`slide-in-end`, `expand`) with the motion tokens, converting the token cubic-bezier curves into GSAP eases so
web and native share timing. With `prefers-reduced-motion: reduce` the end state is applied at once.

The Storybook motion controls are **not a visual animation builder**: they only pick existing presets and
tokens to preview them, and cannot create, record, edit, or export animations. Animations are written in
code. This matters because GSAP's licence prohibits use in no-code visual animation builders; keep it that
way.

## Accessibility

`.storybook/preview.tsx` sets `parameters.a11y.test = "error"`, so `pnpm test:storybook` fails on any axe
violation in any story, including portalled content opened by a play function (see the Dialog story). Colour
pairs in `tokens.json` meet WCAG 2.2 AA (4.5:1 text, 3:1 field borders).
