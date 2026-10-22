# aayu mockup 570 (synthetic fixture)

Produce the design-system mockups for Feature #570 and write the mockup stage's `--from` directory. The research and
wireframe stages are on the ledger, and the design system follows them (below the task). The research plan's
usability test will use these mockups.

## Feature #570: Customers choose a delivery window

A grocery web shop. Today every order is delivered "between 8:00 and 20:00"; customers cannot choose. The delivery
team can now offer two-hour windows for the next three days, each with a capacity.

Stories (Ready):

- #571 Choose a delivery window. State `default`. Completion Goal:
  1. [screenshot] state: a signed-in customer on `/preferences` with tomorrow's windows seeded (two available, one
     full); expected: the windows are listed with their availability, the full one cannot be chosen, and the chosen
     one is marked.
- #572 A window that fills up while choosing. State `slot-full`. Completion Goal:
  1. [screenshot] state: the chosen window becomes full before Save; expected: a message next to the window list
     says it is full and the nearest available windows are offered; the choice is not lost silently.
- #573 Confirmation after saving. State `saved`. Completion Goal:
  1. [screenshot] state: after Save; expected: a confirmation names the chosen window and day.

Windows come from the delivery service as `{day, start, end, available}`; there are up to six windows a day for
the next three days. Every state must work at 390 px wide as well as on a desktop.
