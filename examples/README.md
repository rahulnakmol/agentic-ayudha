# examples

End-to-end example products built by the pod. `examples/benefits-qa/` is the nine-layer dogfood example of WP8.1: a benefits Q&A product for the fictional employer Example Co, built from `grill goal` to `release` on synthetic, seeded data (Goal #125).

Each product keeps a feature map with behaviour checks (WP4.5b) in its own verify skill, which the merge gate runs:

| Product | What it is | Verify skill | Check kinds |
|---|---|---|---|
| `benefits-qa/` | A web app and its JSON API | `benefits-qa/.agents/skills/verify-benefits-qa/` (Node and Playwright) | ui, api |
| `notes/` | A small CLI and its store library, with no user interface beyond its commands | `notes/.agents/skills/verify-notes/` (Go) | cli, lib |
