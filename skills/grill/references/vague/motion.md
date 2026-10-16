# Vague: motion and transitions (`motion`)

Qualifiers: animation, animations, animated, animate, animates, transition, transitions, motion,
micro-interaction, micro-interactions, fluid, bouncy, springy

"Nice transitions" names a feeling. Motion is concrete only when it names its purpose, the exact moments and
elements that move, a duration and easing budget, what happens under reduced motion, and how it is proven. Ask in
this order and cite the motion facts: the durations the product already uses, a motion library, and whether
anything honours `prefers-reduced-motion`.

## Dimensions

### Purpose (`motion.purpose`)

- Ask: What is each motion for: showing where something came from or went, confirming an action the user took, or
  holding attention while something loads? Which moments of the task need it?
- Default: motion only where it explains a change the user caused (moving between steps, adding an item), because
  decorative motion is the most common redo.
- Evidence: recording
- Facts: flow, route, motion
- Products: web
- Topics: moments, purpose

### Where it applies (`motion.where`)

- Ask: Which screens, elements, and state changes move, and which stay still?
- Default: none chosen for the human: list the moments they name, and park any you propose that they do not confirm.
- Evidence: recording
- Facts: route, component
- Products: web
- Topics: elements, state-changes

### Duration and easing (`motion.budget`)

- Ask: How long may each motion take, and with which easing? Is there a house value to reuse?
- Default: reuse the durations the facts show. When there are none, propose 150 to 300 ms with ease-out for UI
  transitions as an option to confirm, never as a requirement.
- Evidence: metric
- Facts: motion, tokens, gap
- Products: web
- Topics: duration, easing, tokens

### Reduced motion (`motion.reduced`)

- Ask: When the system setting asks for reduced motion, what happens: no movement, a short fade, or the same motion?
- Default: honour `prefers-reduced-motion` by removing movement and keeping at most an opacity change, because
  motion can make some users unwell (WCAG 2.3.3).
- Evidence: recording
- Facts: gap, motion
- Products: web
- Topics: reduced-motion

### Smoothness (`motion.performance`)

- Ask: Must the motion stay smooth on a mid-range phone? May it move the layout?
- Default: animate only transform and opacity, with no layout shift caused by motion, shown by a performance trace;
  a frame-rate number only when the human gives one.
- Evidence: metric
- Facts: floor
- Products: web
- Topics: frame-rate, layout-shift

### How it is proven (`motion.proof`)

- Ask: What shows the motion is right: a recording of each moment at normal and at reduced motion, a metric of each
  duration in the styles, or both?
- Default: a recording of each moment at both settings, and a metric that every duration in the changed styles is
  within the confirmed budget.
- Evidence: recording
- Facts: test, motion
- Products: web
- Topics: recording, duration-metric

### Progress output (`motion.cli-progress`)

- Ask: While a long command runs, what moves: a spinner, a progress bar with counts, or nothing? What happens when the
  output is not a terminal (a pipe, CI) or `--json` is set?
- Default: progress only on a terminal, none in a pipe, in CI, or with `--json`, because scripts read the output.
- Evidence: recording
- Facts: command, product
- Products: cli
- Topics: spinner, non-terminal
