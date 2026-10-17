# Layers

A changed path belongs to the layer whose glob matches it; `slice` and `plan` compute story footprints from these
globs. `[autonomy] never` lists L04, L07, and L09: an agent does not change them without a plan-time review.

| Layer | Name | Paths |
|---|---|---|
| L02 | Interaction / API boundary | `cmd/**` |
| L07 | Operational data / state | `store/**` |
