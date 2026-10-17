# empty: no orders yet (#422)

Job: a customer with no orders knows the page works and what to do next (P4: "Is it broken, or have I just not ordered?").

```text
┌ Your orders ─────────────────────────────┐
│ No orders yet                            │  ← heading
│ Orders you place appear here, with       │
│ where each delivery is.                  │
│ [ Start shopping ]                       │  ← primary button, first focus stop
└──────────────────────────────────────────┘
```

- The page heading stays "Your orders", so the state is clearly this page, not an error.
- "Start shopping" is the one primary action and goes to the shop's home page.
- 390 px: the same single column; the button spans the width.
