# #673 Sequencing

One proposed slice (see [05-implementation-plan.md](05-implementation-plan.md)), so one lane in one wave, not 10.
The work is one page and one sidebar entry; splitting it would put two lanes on the same page.

## Wave 0: person steps

- File P1 as a Story under #673 (person agrees).
- Intent and spec baselines (`aayu ready 673 --part intent|spec`) and Approve to build (`aayu ready 673`), by a
  person. Whether a docs Feature also needs tech and UX baselines is not stated in the sources.
- A person picks the XERJ release to pin.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 guide page and sidebar entry | `docs/guide/guide-xerj.md` (new), `internal/tell/site.go` |

## Hot files and serialized merges

- `internal/tell/site.go` holds every page's sidebar entry. Another docs PR that adds a page conflicts on the same
  list; merge one at a time.

## Steps that need a person

- No path in this slice is in `[autonomy] always_person` or `.github/CODEOWNERS`.
- The PR is not docs-only, because `internal/tell/site.go` is code. So it needs a review by a model of another
  family (AGENTS.md, "Work items").
- XERJ itself must not be installed into the committed `.mcp.json` while writing the page (#673; `.mcp.json` is in
  CODEOWNERS).
