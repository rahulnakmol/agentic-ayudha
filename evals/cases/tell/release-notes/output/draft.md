# Benefits QA v1.5.0

Released 2026-10-16. You can now save a claims filter, reopen it later, and share it with another reviewer. You can
also leave a note on a claim for the next reviewer.

## Saved filters ([#15](https://github.com/acme/benefits-qa/issues/15))

- Apply a filter to the claims list, choose Save filter, and type a name. The name appears under Saved filters in the
  sidebar.
- When you reopen a saved filter, the list shows the claims that match its conditions at that time.
- Copy link gives you a link that opens the claims list with the same filter for another signed-in reviewer. They can
  apply the filter, but they cannot rename or delete it.

This helps reviewers who rebuild the same status and date-range filters by hand each morning.

## Claim notes ([#16](https://github.com/acme/benefits-qa/issues/16))

- On a claim's page, Add note saves a plain-text note with your name and the time you added it.
- Another reviewer who opens the claim sees its notes, newest first.

The reason a claim was held or queried now stays with the claim.

## Also in this release

Two more changes shipped in v1.5.0. They are not described here, because their proof is not complete yet.

Each heading links to the Feature and the evidence that it works.
