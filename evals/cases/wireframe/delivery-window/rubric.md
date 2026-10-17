# Rubric: wireframe, choosing a delivery window

Score the wireframe stage from 1 to 5: `stage.json` and one low-fidelity wireframe per state. Wireframes show
structure, content, and controls, without styling, so the next stage (design-system mockups) and the planned
usability test can use them. The research stage on the ledger and the stories are the only sources.

Already checked before you see it (do not re-check): the research stage is pinned, every state the Completion Goals
name (`default`, `slot-full`, `saved`) has a wireframe, and the files are of a checkable kind.

What strong wireframes have:

- Each state does what its story says: `default` lists windows with availability, a full window cannot be chosen,
  and the chosen one is marked; `slot-full` puts a message next to the list saying the window filled up, offers the
  nearest available windows, and never changes the choice silently; `saved` names the chosen day and window.
- One choice from a short list is built as one (radio buttons in a group with a legend), with the full window's
  state in words rather than colour or a greyed-out look alone.
- The full-window message is perceivable without sight: focus moves to it or it is announced, and its text names
  the window (WCAG 3.3.1); this is what the research plan tests with assistive-technology users.
- The research plan's open questions are respected: where the evidence is missing (day first or time first, the
  change cut-off), the wireframe marks the choice as an assumption to test instead of presenting it as settled.
- Works at 390 px, with a keyboard path through days, windows, and Save; real content; nothing the stories do not
  support (no payment, no address changes, no delivery fees).

- 5: All of the above for every state, concise enough to mock up and test directly.
- 4: As 5 with one minor gap.
- 3: Usable, but one state misses part of its Completion Goal, or the full-window message is not accessible.
- 2: States that contradict their stories, invented features, or no accessibility thinking.
- 1: Not usable as wireframes.
