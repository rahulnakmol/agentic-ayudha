# Vague: user experience (`ux`)

Qualifiers: high quality, high-quality, great UX, good UX, better UX, intuitive, delightful, polished, user-friendly,
seamless, beautiful, modern, clean, smooth, pleasant, professional, engaging, world-class, premium

A UX qualifier names a feeling, not an outcome. Break it into the dimensions below, in this order (highest rework
risk first), and ask about the ones the request and the facts leave open. Each dimension says what to ask, a
sensible default to offer with its reason, the evidence kind that usually proves it, and the assessment facts
(`aayu grill <type> --assess`) to cite when you ask, and the topics a question about it is tagged with
(`<dimension>/<topic>`).

## Dimensions

### Key tasks (`ux.tasks`)

- Ask: Which one to three tasks must feel high quality, for whom, and what does success look like for each (the
  customer finishes the task, sees the result, and knows it worked)?
- Default: the one task the request names, for the user it names, ending on a confirmation the user can see,
  because one well-defined task is buildable and testable in a story.
- Evidence: recording
- Facts: flow, route, story
- Topics: primary-task, success-signal, audience

### Steps and time on task (`ux.steps`)

- Ask: How many screens or steps should the task take, and how long should a first-time and a returning user
  take? The current flow is a fact you can quote.
- Default: no more steps than today and one fewer where a step only confirms, because each extra step loses users;
  time on task only when the human gives a number.
- Evidence: recording
- Facts: flow, route
- Topics: screen-count, step-merge, confirmation-placement, options-placement, time-on-task

### Empty, loading, error, and offline states (`ux.states`)

- Ask: For each screen of the task, what does the user see when there is nothing yet, while it loads, when it
  fails, and when the connection drops? Which of these matter for this story?
- Default: an error state with a way to retry wherever the facts show none (a failed payment or submit must never
  lose the user's input), because missing error states are the most common redo; offline only when the human says
  users order on the move.
- Evidence: screenshot
- Facts: gap, route
- Topics: payment-failure, submit-failure, empty, loading, offline, other-errors

### Mobile and desktop (`ux.platforms`)

- Ask: Where do users do this task: phone, desktop, or both? Which viewport widths must the criteria hold at?
- Default: the platforms in `[verify] platforms`, checked at 390 px and 1280 px for the web, because those are the
  widths the web floors capture.
- Evidence: screenshot
- Facts: floor, route
- Topics: devices, widths

### Consistency with the design system (`ux.consistency`)

- Ask: Must the screens use only the existing design tokens and components? Is anything allowed to look new?
- Default: only existing tokens and components, because a new look is a design decision, not a story detail. A
  screenshot cannot show where a colour came from, so state it as a count over the changed files: 0 colour,
  spacing, or font values outside the tokens and 0 new components.
- Evidence: metric
- Facts: tokens, component, ds
- Topics: tokens-and-components, new-components

### Content and tone (`ux.content`)

- Ask: Is there wording that must appear exactly (button labels, confirmation text, error messages)? What tone
  should messages take?
- Default: the tone in `design/brand.yaml` when it exists; exact wording only when the human gives it, quoted.
- Evidence: screenshot
- Facts: route
- Topics: exact-wording, tone

### Trust signals (`ux.trust`)

- Ask: What does the user need to see to trust the result: price and total before paying, delivery or pickup time,
  a receipt, a way to undo or cancel?
- Default: the total and the pickup or delivery time on the last screen before paying, because surprise costs are
  the main reason people abandon an order.
- Evidence: screenshot
- Facts: route, story
- Topics: before-paying, receipt, undo-or-cancel

### Performance budgets (`ux.performance`)

- Ask: Is any screen or action slow today, and is there a number it must meet? Lighthouse budgets in config apply
  to every story; a criterion is only needed for a tighter or new budget.
- Default: the configured Lighthouse budgets and no extra criterion, because the floors already check them.
- Evidence: metric
- Facts: floor, gap
- Topics: slow-screens, budgets

### Accessibility beyond the floors (`ux.accessibility`)

- Ask: Only when the human raises it: do you need more than the floors (WCAG 2.2 AA, keyboard, 44 px targets, 200 %
  text), such as a screen-reader walkthrough of the task?
- Default: the floors alone, because `verify` enforces them on every story; never add an accessibility criterion
  the human did not ask for.
- Evidence: sr-transcript
- Facts: floor
- Topics: beyond-floors
