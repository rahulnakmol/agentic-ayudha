# UX design: host and session on every agent write

This Feature has no command, flag, or screen of its own (#871 Scope). Its value shows in other surfaces.

## Agent-facing surface

- An agent passes nothing new. aayu reads the host and session from the environment, as `Env.session`
  (`internal/cli/asked.go:37`) and `doctor.DetectHost` (`internal/doctor/checks.go:229`) do today. The global
  `--host` flag stays: "host running aayudha: amp, cursor, claude, codex, copilot (default: detected)" (`aayu --help`).
- `aayu actions` prints the actions log today (`runActions`, `internal/cli/actions.go:95`). Planned: each record shows
  its host and session. The output format is not specified in any source.

## Person-facing effect (later Features)

- The inbox card links to the thread it came from (#871 Problem, G4). The card design belongs to the inbox Features
  (#873, #895); #871 only supplies the data.
