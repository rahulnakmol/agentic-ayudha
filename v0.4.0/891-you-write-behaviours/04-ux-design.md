# UX design: you write behaviours

Status: planned, not built. No UX baseline exists. The surfaces below come from Decision #867 option 1 and #891.

## The founder

| Surface | Planned | Today (884d81b) |
|---|---|---|
| Decide card | A BEHAVIOUR CHANGE card for a PR that changes an approved check, one card per PR, with the step diff (#867 option 1 and Evaluation). | `aayu inbox` has a `decision` kind and no behaviour kind (`internal/cli/inbox.go:46-63` at bc78349). |
| Approve or refuse | `aayu decide --behaviour <ref> --pr <n> --approve` or `--refuse`, after the presence check (#867 option 1). | `aayu decide` has `--accept`, `--draft`, `--pack`, `--seat`, `--waiver` (`aayu decide --help`). |
| Accept a Decision | `aayu decide <n> --accept <option> --sets <ref>`, `--sets new:"<sentence>"`, or `--sets none` (#867 option 1). | `--accept` alone: "Option to accept, by its number or its name (human identity only)." (`aayu decide --help`). |
| Behaviours view | "the Behaviours view data" (#891 Scope). The view itself and its screen are not specified. | none |

## The agent

| Surface | Planned | Today |
|---|---|---|
| Map check | Writes the steps; the founder writes `says` (#867 option 1). | Writes `id`, `kind`, `evidence`, `requires`, `steps` (`verify.Check`, `internal/verify/featuremap.go:71-77`). |
| Build | `build` stops when its diff changes an approved check; the agent runs `aayu ask` (#867 option 1). | `aayu build` has no such stop; `aayu ask` does not exist (#880). |
| Story criterion | A criterion that ends with `; standing` needs a map check with `source` `#n/k` before ship (#867 option 1). | Criteria use `N. [kind] state: …; expected: …` (AGENTS.md, "Work items"). |
| Verify | `aayu verify --check <ref>` and `--map` (`aayu verify --help`). | as planned; unchanged |

## Messages and errors

None is specified beyond the card name BEHAVIOUR CHANGE.
