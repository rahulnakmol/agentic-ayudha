---
description: The single best next aayudha step for this repository, and who takes it.
allowed-tools: Bash(aayu next --json)
---

# aayudha next step

Run `aayu next --json` in the repository root and read `command`, `why`, and `human_only`.

- When `human_only` is false, tell me the step in one line and take it with the matching aayudha skill (for
  `aayu build 12`, the build skill on story 12). Asking for the next step is not asking for a story: before you
  build one, ask me "Shall I build #12?", and on yes pass my words with `aayu build 12 --asked "<my words>"`.
- When `human_only` is true, the step is for me: show me the exact command to run in my own terminal, and do not
  run it yourself.
