# Export the claims you have filtered to a CSV file

Use this guide when you want to audit a set of claims offline. You get a CSV file (comma-separated values, which a
spreadsheet opens) of every claim your filters match, with the columns you choose. This is how
[Export filtered claims (#7)](https://github.com/acme/benefits-qa/issues/7) works.

## Before you start

- Sign in and open the claims list. If this is new to you, start with [Getting started](getting-started.md).
- Filter the list to the claims you want to audit, as in [Filter the claims list](filters.md). The file holds every
  claim the filters match.

## Export your list

1. Choose your columns with the Columns picker. The file holds only the columns shown, in the order shown.
2. Select Export CSV in the list toolbar.
3. Your browser downloads `claims.csv`. Open it in the spreadsheet you use for audits.

## What to expect

- On a list of more than 10,000 claims, the list keeps responding while the file is prepared.
- The export covers the claims list only. PDF files, scheduled exports, and exports of other lists are outside what
  this feature does.

You now have the claims you filtered in one file, ready for your audit.
