# list: orders with their delivery status (#421)

Job: see where a delivery is without leaving the list (research: Job story; P1, P2).

```text
┌ Your orders ─────────────────────────────────────────┐
│ Table caption: "Your orders, newest first"           │
│ Order      Date        Total     Delivery status     │  ← column headers (P3)
│ #10482     14 Oct      €42.10    [Out for delivery]  │
│ #10477     11 Oct      €18.75    [Being packed]      │
│ #10431     2 Oct       €63.00    [Delivered]         │
└──────────────────────────────────────────────────────┘
```

- Status is a label in words, never colour alone (P2: "like out for delivery").
- Each order number is a link to the order; it is the row's only focus stop, so Tab moves order by order.
- Screen reader: the caption names the order of rows; headers announce with each cell (P3).
- 390 px: each row becomes a stacked card (order and status on the first line, date and total on the second), so
  columns never overlap (P5).
