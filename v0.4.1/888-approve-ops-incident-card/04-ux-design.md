# #888 UX design

The surfaces are the `aayu respond` command and an INCIDENT card in `aayu inbox`. Only a person uses either:
`respond` and `inbox` are human-only (`internal/verbs/registry.go`, entries `Name: "respond"` and `plumb("inbox", …)`).

## Today on dev

`aayu respond --help` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Respond to an incident: propose an ops catalog action, execute it only through human approval, keep the timeline.

Usage:
  aayu respond <incident> [flags]

Flags:
      --action string       Catalog action to propose.
      --close               Close the incident: file its linked Bug (carrying the investigation note's implicated code and durable fix) and the retro entry; a Sev0 or Sev1 incident needs its postmortem first.
  -h, --help                help for respond
      --params string       Action parameters as key=value pairs separated by commas.
      --postmortem string   Blameless postmortem (Markdown) to check against the incident, its timeline, and its investigation note, and post on the incident (replaced in place).
```

After `--action`, the person sees "proposed `<action>` … in environment `<env>` (approval human), request `<id>`" and
"awaiting a human reviewer's approval of environment `<env>`: `<run URL>`" (`internal/cli/respond.go:226-228`). The
person then opens the run URL and approves on GitHub.

Refusals a person can meet today (`requireReviewers`, `internal/cli/respond.go:316-339`):

- `respond: GitHub Environment "<env>" does not exist; create it with required reviewers`
- `respond: environment "<env>" has no required reviewers, so an approval = human action cannot run there`
- `respond: environment "<env>" lets any branch deploy; restrict it to protected branches`

`aayu inbox` has no INCIDENT card: no incident kind is in `internal/cli/inbox.go:46-62`.

## Planned, not built

- A flag `--approve <run>` on `respond` (#888). Its help text is not in any source.
- The INCIDENT card. Story [#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947) (under #975) gives the
  terminal design at 120x34:
  - In Now: "INCIDENT · Sev1 · #1204 checkout errors at 4% · open 18 min", the timeline, the action with blast
    radius, undo, rate limit, and environment, and "1 Approve as you · 2 Open on GitHub ↗ · 3 Refuse with a reason"
    (criterion 1).
  - A first line on every view while a Sev0 or Sev1 is open: "Sev1 incident #1204 is open · 18 min · Respond"
    (criterion 2).
  - After 1 and Enter: "Approved as @rahulnakmol · run 1234 started · rollback ready" (criterion 3).
  - With no required reviewers, choice 1 is off: "aayu cannot approve here: the environment production has no
    required reviewers. Add one on GitHub." (criterion 4).
- The F8 warning text for prevent self-review is not in any source.

## Agent-facing surface

None that acts. An agent may read `aayu inbox --list` (read-only, allowed for agents per its help). An agent never
approves: `respond` is `Exposure: ExposeNever` over MCP.

## Accessibility

Story #947 sets the terminal size 120x34 for its screenshots. The global `--plain` flag removes colour and box drawing. The
sources state no other accessibility requirement.
