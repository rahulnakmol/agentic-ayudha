# slot-full: the chosen window filled up before Save (#572)

Job: the customer notices, understands why, and still gets a window (research question 2).

```text
┌ Delivery window ─────────────────────────────────────┐
│ ⚠ 18:00–20:00 on Tue 21 Oct filled up while you      │  ← message next to the list,
│   were choosing. Nearest windows still free:         │    announced (WCAG 3.3.1)
│   ( ) 16:00–18:00 Tue 21 Oct                         │
│   ( ) 18:00–20:00 Wed 22 Oct                         │
│ Fieldset legend: "Windows on Tue 21 Oct"             │
│ ( ) 08:00–10:00   Available                          │
│ ( ) 18:00–20:00   Full                               │  ← no longer chosen
│ [ Save delivery window ]                             │
└──────────────────────────────────────────────────────┘
```

- Save is refused with the message, never a silent change of the choice (#572).
- Focus moves to the message so screen-reader and magnifier users find it; its first sentence names the window.
- The two nearest free windows are offered as choices right in the message.
- 390 px: the message sits above the list and wraps; nothing is hidden off screen.
