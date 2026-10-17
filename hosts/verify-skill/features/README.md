# {{product}} feature map

What a person, or a script, can do with {{product}}, and the behaviour check that proves each capability
(docs/BUILD-PLAN.md WP4.5b). `aayu verify --map` keeps the map honest against the source; `aayu verify --check all`
runs every check.

The map block names the control tool and the surfaces to watch. A surface rule finds the handles a person or script
uses (a page's controls, a command, a flag, a route, an exported function) with a regular expression over the
named files; every handle it finds must appear in some feature, or `verify --map` reports it as unmapped.

```map
{"product": "{{product}}",
 "control": {{control_json}},
 "surfaces": []}
```

## Baseline preconditions

- What the product needs before any check runs (a seed, a fresh working directory, a built binary).

## Conventions

- Assert what a person reads or a script relies on: visible text, an exit code, a status and body, a return value.

## Features

None yet. Add one file per capability (`<id>.md`) and list it here as `1. [Title](<id>.md): one line.`; aayudha's
`verify` skill has the procedure.
