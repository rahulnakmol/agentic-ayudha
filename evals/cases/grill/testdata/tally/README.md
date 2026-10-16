# tally (eval fixture)

A synthetic command-line expense tracker for the grill eval `evals/cases/grill/tally-simple`. It is never built or
run: `aayu grill <type> --assess` reads it as a repository. Every name and number is invented.

## Getting started

1. `tally init --currency GBP` creates `~/.tally/config.toml` and the data directory.
2. `tally add --amount 12.50 --category lunch` records an expense.
3. `tally report --by category` sums them.

`tally add` before `tally init` fails with `open ~/.tally/config.toml: no such file or directory`.
