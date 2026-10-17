# aayu prototype 570 (synthetic fixture)

Build the prototype of Feature #570 and write the prototype stage's `--from` directory together with the code it
names. The research, wireframe, and mockup stages are on the ledger, and the design system and the Figma file
follow them (below the task).

## Feature #570: Customers choose a delivery window

A grocery web shop. Today every order is delivered "between 8:00 and 20:00"; customers cannot choose. The delivery
team can now offer two-hour windows for the next three days, each with a capacity.

Stories (Ready):

- #571 Choose a delivery window. State `default`. Completion Goal:
  1. [screenshot] state: a signed-in customer on `/preferences` with tomorrow's windows seeded (two available, one
     full); expected: the windows are listed with their availability, the full one cannot be chosen, and the chosen
     one is marked.
- #572 A window that fills up while choosing. State `slot-full`. Completion Goal:
  1. [screenshot] state: the chosen window becomes full before Save; expected: a message next to the window list
     says it is full and the nearest available windows are offered; the choice is not lost silently.
- #573 Confirmation after saving. State `saved`. Completion Goal:
  1. [screenshot] state: after Save; expected: a confirmation names the chosen window and day.

## The app (what the host read in the repository)

- React 19 with the design system (`@aayudha/design-system`: `Button`, `Badge`, `Table`, `Alert`, `Card`, the
  `ListReport` floorplan), styled with its Tailwind utilities and `--ay-*` tokens. Routes live in `src/app/<route>/page.tsx`.
- Today `src/app/preferences/page.tsx` renders `<PreferencesPage />` from `src/preferences/PreferencesPage.tsx`: the
  customer's address and the notice "We deliver between 8:00 and 20:00".
- OpenFeature is set up with the flagd provider (`src/flags/provider.tsx`); components read flags with
  `useBooleanFlagValue(key, false)` from `@openfeature/react-sdk`. `[design] flags = "flags.flagd.json"` (flagd JSON
  at the repository root, empty today: `{"flags": {}}`). flagd's targeting sees the context property
  `environment` (`preview` or `production`).
- `[verify.environments.preview] url = "https://preview.shop.example"` is the capture target; production is never
  captured.
- Code Connect files sit next to their component as `<Component>.figma.tsx` and use `figma.connect(Component,
  "<figma URL with node-id>", {props, example})` from `@figma/code-connect`.

The delivery API is `GET /api/delivery-windows?days=3` → `[{id, day, start, end, available}]` and
`PUT /api/preferences/window {windowId}` → 200, or 409 `{error: "full", nearest: [window, window]}` when the window
filled up.
