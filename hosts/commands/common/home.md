---
description: Where this repository stands on the aayudha journey, and the single best next step.
allowed-tools: Bash(aayu --json)
---

# aayudha home

Run `aayu --json` in the repository root. It prints the aayudha home screen as JSON: the journey stage, Ready
stories, stories in progress, story pull requests waiting for a person, blocked stories, failing floors,
measurements due, and `next`.

Tell me, in a few short lines, where the work stands. Name issues and pull requests by number. Then give the next
step from `next.command` and `next.why`.

If `next.human_only` is true, the step is for me to run in my own terminal: say so, and do not run it. When that step
is `aayu inbox`, offer to show what waits with `/aayudha:inbox` (read-only). Otherwise, offer to take it with the
matching aayudha skill.
