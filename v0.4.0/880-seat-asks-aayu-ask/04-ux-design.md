# UX design: aayu ask

Status: planned, not built. No UX baseline exists. Only the parts below are stated by a source.

## The agent (asking)

- `aayu ask` on an item, with one structured question (#880 Summary). The flags, the question's fields, and the
  output are not specified in any source. Planned, not built.
- Today the agent's only stop is `aayu build <n> --stop "<reason>"`: "Stop this build for good: move the story to
  status:blocked with this reason and page its owner; the lease stays until it expires." (`aayu build --help`).

## The person (answering)

Three surfaces (#880 Summary):

| Surface | Planned | Today |
|---|---|---|
| Inbox | An `ask` card, and an `ask-provisional` card (#880 Scope). Per Decision #862, Answer area 0 to 24 h, Overdue 24 to 48 h, a banner that counts overdue questions. | `aayu inbox` has no ask kind (`internal/cli/inbox.go:46-63` at bc78349). |
| Agent tool | Two-way asks only (#880 Summary). | none |
| GitHub reply | `/aayu answer` on the item, checked as `/aayu approve` is (#880 Scope). | `/aayu approve all <hash>`, `/aayu approve 1 3 <hash>`, `/aayu skip 2 <hash>` on an approval batch (`internal/cli/approve.go:236-246`). |
| Command | `aayu ask --answer` (human-only) (#880 Scope). | none |

## Messages and errors

None is specified. AGENTS.md "Safety" applies: an agent never writes an `/aayu approve` or `/aayu skip` reply; the
same rule for `/aayu answer` is inferred.
