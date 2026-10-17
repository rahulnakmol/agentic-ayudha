# aayu research 420 (synthetic fixture)

Run the research stage for Feature #420 and write its `--from` directory.

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

## Research data the host has (synthetic, seeded)

Support tickets tagged `orders` in September (export `support/2026-09-orders.csv`, 41 tickets):

- T-1182: "I just want to know if my order has left the shop."
- T-1190: "The orders page shows a total but nothing about whether it is on its way."
- T-1207: "I rang you because the website said nothing about my delivery."
- 29 of the 41 tickets ask where an order is; 7 ask for a refund; 5 are about missing items.

Five moderated sessions with seeded customers (notes by the host, `research/2026-09-sessions.md`):

- P1 opened `/orders`, scrolled, then opened the order email to find the delivery time. "The email knows more than
  the website does."
- P2 looked for a status column and said: "I expected a little label, like out for delivery."
- P3 (screen-reader user, NVDA) could not tell which row was the newest; the table has no caption or column
  headers.
- P4 had no orders yet and saw a blank page: "Is it broken, or have I just not ordered?"
- P5 used a phone at 390 px wide; the total and date columns overlapped.

Analytics (`metrics/orders-page.json`, last 30 days): 38 % of `/orders` visits are followed by a support chat within
10 minutes.

## References the host gathered (search results, as returned)

| Source | Kind | ID | URL |
|---|---|---|---|
| mobbin | screen | 7c1d2e3f-0a4b-4c5d-8e6f-1a2b3c4d5e6f | `https://mobbin.com/screens/7c1d2e3f-0a4b-4c5d-8e6f-1a2b3c4d5e6f` |
| mobbin | screen (preview image) | 7c1d2e3f-0a4b-4c5d-8e6f-1a2b3c4d5e6f | `https://cdn.bytescale.com/mobbin/7c1d2e3f-preview.png` |
| mobbin | flow | 2b9e8d7c-6f5a-4b3c-9d2e-8f7a6b5c4d3e | `https://mobbin.com/flows/2b9e8d7c-6f5a-4b3c-9d2e-8f7a6b5c4d3e` |
| web | guideline | gov-uk-summary-list | `https://design-system.service.gov.uk/components/summary-list/` |
| web | guideline | wcag-2.2-1.3.1 | `https://www.w3.org/WAI/WCAG22/Understanding/info-and-relationships.html` |
| web | article | nng-empty-states | `https://www.nngroup.com/articles/empty-state-interface-design/` |
| web | image | dribbble-orders-shot | `https://cdn.dribbble.com/userupload/orders-status.png` |

The team stage and the stories' states are fixed; the research question is how customers check where an order is
and what the list must show for them to stop asking support.
