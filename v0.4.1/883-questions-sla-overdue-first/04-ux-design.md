# #883 UX design

The surfaces are `aayu decide` (the sweep, run by the `decisions` loop) and `aayu inbox`.

## Today on dev

`aayu decide --help` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), first lines:

```text
Record a Decision with options, evaluation, and recommendation; accept it, supersede it, or file a floor waiver.

Usage:
  aayu decide [decision] [flags]
```

Its positional argument says: "Decision issue; without one, the SLA sweep over every open Decision."
(`internal/verbs/registry.go`, entry `Name: "decide"`).

Pages a person gets today (`internal/cli/decide.go:214-218`):

- "Decision #`<n>` is past its SLA: `<title>`"
- "Decision #`<n>` is overdue: `<title>`"

The sweep's per-Decision state line includes "overdue (`<reason>`)" (`internal/cli/decide.go:254-255`).

## Planned, not built

Story [#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940) (under #975) gives the terminal design at
120x34. It says "decisions" and "ask" for agent questions:

- An ask with 3 h left: amber "Due in 3 h · Shuka waits" (criterion 1).
- Three asks past 24 h: the first line of every view "3 decisions passed 24 h · Shuka, Hanu, and Nala wait · take them
  first"; Enter opens Overdue (criterion 2).
- A two-way ask at 26 h: red "Overdue by 2 h · Shuka has waited 26 h. Reversible: the recommendation applies at 48 h
  unless you answer." (criterion 3).
- At 48 h, a card in Decide: "Applied Shuka's recommendation provisionally at 48 h: “Acts on a card”. Ratify or
  revert it in Decide." (criterion 4).
- A one-way ask at 50 h stays in Overdue, red: "Overdue by 26 h · a one-way door never applies on its own. Only you
  can answer it." (criterion 5).

The page text at 24 h for a question is not in any source.

## Agent-facing surface

The asking seat is told when its question gets a provisional answer (#862 option 1). How it is told depends on #880's `aayu ask`, which is not designed in any source read here.

## Accessibility

Story #940 uses colour (amber, red) for age. With `--plain` the text carries the same facts ("Due in 3 h", "Overdue by 2
h"), so colour is not the only signal (inferred from the strings above).
