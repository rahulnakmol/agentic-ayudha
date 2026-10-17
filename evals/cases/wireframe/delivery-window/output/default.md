# default: choose a delivery window (#571)

Job: pick a window that fits the day (survey S-17: shift work). Research question 1 (day first or time first) is
open, so the wireframe groups by day and the test checks it.

```text
┌ Delivery window ─────────────────────────────────────┐
│ Choose when we deliver your next order.              │
│ [Tomorrow, Tue 21 Oct] [Wed 22 Oct] [Thu 23 Oct]     │  ← day tabs
│ Fieldset legend: "Windows on Tue 21 Oct"             │
│ ( ) 08:00–10:00   Available                          │
│ (•) 18:00–20:00   Available                          │  ← chosen
│ ( ) 20:00–22:00   Full (cannot be chosen)            │  ← disabled, says why
│ [ Save delivery window ]                             │
└──────────────────────────────────────────────────────┘
```

- One choice from a short list: radio buttons in a fieldset (GOV.UK radios reference).
- A full window stays visible with "Full" in words, so the list does not shift between visits.
- Keyboard: Tab to the day tabs, arrows between days; Tab into the radios, arrows between windows; Tab to Save.
- 390 px: the day tabs scroll horizontally; windows stay one per row.
