# error: the orders could not load (#423)

Job: the customer knows the list failed, that it is not their fault, and can try again; no stale list is shown.

```text
┌ Your orders ─────────────────────────────┐
│ ⚠ We could not load your orders          │  ← alert, announced on arrival
│ This is on our side. Your orders are     │
│ safe. Try again in a moment.             │
│ [ Retry ]                                │  ← primary button, focused after the alert
└──────────────────────────────────────────┘
```

- No table and no cached rows: a stale status would mislead (story #423).
- The message is announced by screen readers (an alert region) and states the cause in words, not an icon alone.
- Retry reloads the list in place; while it loads, the button reads "Retrying…" and is not pressable twice.
- 390 px: one column; the button spans the width.
