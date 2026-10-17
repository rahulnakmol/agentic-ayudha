# Orders show where each delivery is: research notes (#420)

All data is synthetic and seeded: 41 September support tickets (`support/2026-09-orders.csv`), five moderated
sessions (`research/2026-09-sessions.md`), and 30 days of page analytics (`metrics/orders-page.json`).

## Job story

When I have placed an order and the delivery has not arrived yet, I want to see where it is without leaving the
orders list, so I can plan my day instead of contacting support.

> I just want to know if my order has left the shop. (T-1182)

29 of the 41 tickets ask where an order is, and 38 % of `/orders` visits are followed by a support chat within
10 minutes (`metrics/orders-page.json`).

## What people did

- P1 left the list for the order email to find the delivery time: "The email knows more than the website does."
- P2 looked for a status label: "I expected a little label, like out for delivery."
- P3 (NVDA) could not tell which row was newest: the table has no caption or column headers (WCAG 1.3.1).
- P4 had no orders and saw a blank page: "Is it broken, or have I just not ordered?"
- P5 at 390 px saw the total and date columns overlap.

## Implications for the states

- `list` (#421): a status label in words on every row, newest first, with a caption and column headers.
- `empty` (#422): say there are no orders yet and offer "Start shopping", so a blank page never looks broken.
- `error` (#423): name the failure and offer Retry; never show a stale list.
- Every state must hold at 390 px without overlapping columns.

## Open questions

- Which delivery statuses the shop's system can report (packed, out for delivery, delivered; anything else?).
- Whether customers need a delivery time on the row or only the status. P1 suggests the time.

## References

Pinned in `stage.json` as URLs and IDs only: one Mobbin screen and one flow (order tracking), the GOV.UK summary
list, WCAG 2.2 1.3.1, and the NN/g empty-state article. The Mobbin preview image and the Dribbble image were
dropped: references are pages, never pixels.
