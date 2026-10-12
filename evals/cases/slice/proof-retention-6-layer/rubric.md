# Rubric: slice, proof retention per artifact kind

Score the slice from 1 to 5. The Feature asks for: (a) per-kind retention in config, migrating the single
`retention_days`; (b) purge by kind; (c) a `proof purge --kind` flag; (d) listing ledger packs through the API;
(e) a scheduled purge workflow. Its expected paths span six layers (L02, L03, L04, L06, L07, L09). The binary has
already checked that every story is XS-L, passes Ready under the Feature, stays within the expected paths, that
every layer has a story, and that dependencies form no cycle; judge what those checks cannot.

Questions the skill lists under "open questions" are not requirements: they never lower the score. Invented
requirements are specifics the Feature does not state: retention values, schedules, sizes, or other numbers of
the skill's own, except where a criterion needs an example value to be observable and says it is a fixture value.

- 5: Every one of (a)-(e) is delivered by some story; each story is independently shippable and its criteria are
  observable with fitting evidence; dependencies are sensible (the migration before purge by kind, purge before
  the flag and the workflow); efforts are plausible; nothing outside the Feature's scope is added.
- 4: As 5, with one minor gap, such as one criterion that could be more precise or one questionable dependency.
- 3: Usable, but one of (a)-(e) is missing or a story is not independently shippable.
- 2: Two or more of (a)-(e) missing, invented scope, or criteria that cannot be observed.
- 1: Not a usable slice.
