# grill story: question set

Keyed to the Story sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the story.

## Create

### Summary (`summary`, required)

- Who is this for, what do they want to do, and why? Write it as `As a <who>, I want <what>, so that <why>.`
- Which Feature does it belong to? (the `--parent` issue; a Story needs one at Ready)

## Ready

### Completion Goal (`completion_goal`, required)

Numbered, observable criteria; each one line: `N. [evidence] state: …; expected: …`. The criteria state only what
the request or the parent Feature asks for. Accessibility (screen reader, keyboard, contrast, targets), performance,
and security are floors `verify` enforces on every story; add a criterion for one only when the request or parent
asks for that outcome itself, and otherwise leave it to the floors.

- What state does someone have to be in to see the outcome? For a page, put its path in backticks
  (``state: `/settings` as the seeded admin``) so the web floors can find it.
- What exactly do they see or measure there? When the request asks for a screen-reader outcome, quote the phrases
  the screen reader must speak.
- Which evidence proves it: `screenshot`, `recording`, `sr-transcript` (UI), `api-trace` (an API or CLI call and
  its result), or `metric` (a number)? Every criterion needs exactly one kind.
- For `api-trace` and `metric` criteria: which tests will show it? (they go in `.aayudha/evidence/<story>.toml` when
  the story is built)

### Measure (`measure`, required)

- What do we count to say the story is done? (usually "Completion Goal criteria observed")

### Threshold (`threshold`, required)

- How many of them must hold? (usually "all N criteria")

### Scenario (`scenario`, required)

- Walk through the check: the steps someone takes, in order, to observe every criterion.

### Environment (`environment`, required)

- Where is it observed? Proof captures use seeded, synthetic data only; never production.

### Technical Owner (`technical_owner`, optional)

- Who owns the technical decisions? Leave it empty to get the Code Architect's paired human, or its twin.

### Links (`links`, optional)

- Does this story depend on another one? One line each: `depends-on #N`.

## Labels (not body sections)

- Effort: XS, S, M, or L (an XL story must be sliced first). Passed as `--labels effort:<size>`. Required at Ready:
  when the human has no view, propose your estimate from the criteria and confirm it; never leave it out.
- Priority: P0 to P3. Passed as `--labels priority:<p>`.
