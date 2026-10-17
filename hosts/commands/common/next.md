---
description: The single best next aayudha step for this repository, and who takes it.
allowed-tools: Bash(aayu next --json)
---

# aayudha next step

Run `aayu next --json` in the repository root and read `command`, `why`, and `human_only`.

- When `human_only` is false, tell me the step in one line and take it with the matching aayudha skill (for
  `aayu build 12`, the build skill on story 12).
- When `human_only` is true, the step is for me: show me the exact command to run in my own terminal, and do not
  run it yourself.
