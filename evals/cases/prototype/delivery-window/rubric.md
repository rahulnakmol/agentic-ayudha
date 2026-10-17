# Rubric: prototype, choosing a delivery window

Score the prototype stage from 1 to 5: `stage.json`, the flag definition, the product code, and the Code Connect
files. You read the source; you cannot run it or see it rendered, and the figma-diff gate (pixels against the Figma
frames) was not run, so judge what the code determines.

Already checked before you see it (do not re-check): every mockup state (`default`, `slot-full`, `saved`) has a
prototype state with a URL path and a Figma frame, the flag is a flagd boolean that is off by default, every Figma
component listed has a Code Connect mapping, and every colour literal in the code is a design token.

What a strong prototype has:

- Working UI in the product: the route reads the flag through OpenFeature with a false default and keeps today's page
  when it is off; windows come from `GET /api/delivery-windows`, and Save uses `PUT /api/preferences/window`, with
  its 409 `full` answer driving the slot-full state.
- Each state faithful to its mockup and story: windows by day with availability, a full window disabled and saying
  so; on 409 an announced message that names the window, offers the nearest windows, moves focus to itself, and
  never changes the choice silently; a confirmation naming the day and window.
- The new "Window option" component (Figma 41:3, no code yet) built as a real component from tokens, as a native
  radio sized to the 44 px target with a visible focus ring, and given its own Code Connect mapping; existing
  components (Tabs, Button, Alert) reused rather than rebuilt.
- The flag turned on only for the capture environment (`preview`), never production, through flagd targeting; each
  state's path reachable on the preview target without touching production behaviour.
- Nothing beyond the stories (no fees, no address or payment changes).

- 5: All of the above; a reviewer could run review on it next.
- 4: As 5 with one minor gap.
- 3: Usable, but a state misses part of its story, the 409 path is not handled, the flag is not wired into the route,
  or an accessibility property of the mockup is lost.
- 2: Not product code, flag misuse, or several states broken.
- 1: Not a usable prototype.
