# aayu prototype 420 (synthetic fixture)

Build the prototype of Feature #420 and write the prototype stage's `--from` directory together with the code it
names. The research, wireframe, and mockup stages are on the ledger, and the design system and the Figma file
follow them (below the task).

## Feature #420: Orders show where each delivery is

A grocery web shop. Signed-in customers open `/orders` to see past and current orders. Today the list shows the
order number, date, and total only.

Stories (Ready):

- #421 See each order's delivery status. State `list`. Completion Goal:
  1. [screenshot] state: a signed-in customer with three seeded orders (one delivered, one out for delivery, one
     being packed); expected: each row shows the order number, date, total, and delivery status, newest first.
- #422 An empty orders list points to the first order. State `empty`. Completion Goal:
  1. [screenshot] state: a signed-in customer with no orders; expected: "No orders yet" and a primary "Start
     shopping" button.
- #423 The orders list says when it cannot load. State `error`. Completion Goal:
  1. [screenshot] state: the orders API answers 500; expected: an error message with a Retry button and no stale
     list.

## The app (what the host read in the repository)

- React 19 with the design system (`@aayudha/design-system`: `Button`, `Badge`, `Table`, `Alert`, `Card`, the
  `ListReport` floorplan), styled with its Tailwind utilities and `--ay-*` tokens. Routes live in `src/app/<route>/page.tsx`.
- OpenFeature is set up with the flagd provider (`src/flags/provider.tsx`); components read flags with
  `useBooleanFlagValue(key, false)` from `@openfeature/react-sdk`. `[design] flags = "flags.flagd.json"` (flagd JSON
  at the repository root, empty today: `{"flags": {}}`). flagd's targeting sees the context property
  `environment` (`preview` or `production`).
- `[verify.environments.preview] url = "https://preview.shop.example"` is the capture target; production is never
  captured.
- Code Connect files sit next to their component as `<Component>.figma.tsx` and use `figma.connect(Component,
  "<figma URL with node-id>", {props, example})` from `@figma/code-connect`.

The orders API is `GET /api/orders` → `[{id, placedAt, total, status}]`, where `status` is `packed`,
`out_for_delivery`, or `delivered`, sorted newest first by the server.
