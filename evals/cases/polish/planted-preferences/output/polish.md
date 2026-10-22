# polish #57: delivery preferences

Every finding of `.aayudha/review/57/critique.json`, fixed in `preferences.html` with design-system tokens:

| Finding (lanes) | Element | Fix |
|---|---|---|
| focus-visible (A, B: probe) | the window select, Save, help | `outline: none` on `:focus` replaced by a 2 px `:focus-visible` outline in `--ay-color-focus` (#1f5bd1), offset 2 px |
| contrast (A, B: axe, impeccable) | help text `p` | #838383 (3.8:1, not a token) → `--ay-color-text-label` #4f5a6b (7.0:1) |
| target-size (A, B: probe) | help button | 32 × 32 → 44 × 44 px, the cozy control height (`--ay-size-control-height`) |

No control was removed or hidden, and no floor was changed. Re-run: `aayu polish 57`.
