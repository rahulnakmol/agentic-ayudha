# UX design: Review guidance

Status: planned, not built. No UX baseline exists: the ledger has no `specs/887/baselines/ux-*.json`. The Feature
has no screen of its own. A person sees it as text in commands that exist today, and an agent sees it in the brief
and the review prompt.

## The person

| Surface | Today (884d81b) | Planned |
|---|---|---|
| Story or Bug body | No form field. The text is free under Assumptions. | A `#### Review guidance` block under Assumptions with `- Check:`, `- Ignore:`, and `- Risk:` lines; the issue forms do not change (`specs/887/design.json` contract 1). Example from #1221's own body: `- Check: criterion 3: a missing block warns and does not fail, so the planned Stories are not blocked`. |
| `aayu ready <n> --check` | "Validate only; write nothing." (`aayu ready --help`) | Fails with `Review guidance: an Ignore may not name security, data, concurrency, a floor, or a P0 or P1 area` (#1221 criterion 2). Fails and names a line of another shape (criterion 4). Warns `no Review guidance: the reviewer gets none` (criterion 3). |
| `aayu slice <f> --draft <file>` | "JSON file with the stories the host's model sliced" (`aayu slice --help`) | Refuses a story with a limited Ignore, with Ready's reason, before it creates anything (#1221 criterion 5). |
| PR review comment | inspect posts findings and keeps a Dropped list. | A finding inside an Ignore area that is P1, or security, data, or concurrency, is posted with a note that the Ignore did not apply (#1223 criterion 3). A dropped P3 is listed with `story guidance: ignore` (criterion 2). The summary says which checks it covered (criterion 1). |

## The agent

| Surface | Planned |
|---|---|
| Delegation brief (`aayu build <n> --delegate <receiver>`) | A `## Review guidance` section after the Completion Goal, inside the untrusted quote, from the Ready snapshot (#1222 criteria 1 and 2; #1222 Notes). No section when there is no guidance (criterion 3). |
| Review prompt (`aayu inspect <pr>` and the session review) | A `Story guidance` block with the checks and the risk, inside the untrusted data (#1223 criterion 1). |

## Errors and accessibility

- The error and warning texts above are the exact strings in the Completion Goals. Their wording is an Ignore item in
  #1221 ("Ignore: the wording of the warnings").
- All output is terminal text. `--plain` and `--json` are global flags today (`aayu --help`). No source states an
  accessibility need beyond that.
