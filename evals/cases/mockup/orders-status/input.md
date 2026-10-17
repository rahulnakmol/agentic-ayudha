# aayu mockup 420 (synthetic fixture)

Produce the design-system mockups for Feature #420 and write the mockup stage's `--from` directory. The research and
wireframe stages are on the ledger, and the design system follows them (below the task).

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

Every state must work at 390 px wide as well as on a desktop.
