# Create a feature map

For a repository with no map. The result is one PR that adds `.agents/skills/verify-<product>/` and changes no
product code.

1. Run `aayu init --verify-skill`. It writes `SKILL.md`, an empty map, and a control tool in the stack the checkout
   already has (`go.mod`: Go; `package.json`: Node and Playwright; neither: a stub for you to replace in the product's
   stack). `aayu install --apply` then copies the skill to the hosts that do not read `.agents/skills/`.
2. Read the repository: the README, the entry points (routes, pages, commands and flags, endpoints, exported
   functions), and the existing tests. List the capabilities a person or a script relies on, one line each. Do not
   assume a browser or user journeys: a CLI's capabilities are its commands, a library's are its functions.
3. Write the surface rules in the map block so that `aayu verify --map` finds every surface, then one
   `features/<id>.md` per capability, listed in the index. Each feature names its sources and surfaces.
4. Give each capability at least one check of the kind that fits: `ui` for a page flow, `cli` for a command, `api`
   for an HTTP call, `lib` for a function, through the product's own test framework where one exists. Assert what a
   person reads or a script relies on (visible text, an exit code, a status and body, a return value), including one
   failure path where the product has one.
5. Extend the control tool only with steps the checks need, in the product's stack.
6. Prove one check of every kind you used with `aayu verify --check <feature>.<check>`, and look at its artifacts in
   `.aayudha/state/verify/`. Then run `aayu verify --check all`: a check that fails because the product is wrong is a
   Bug to file, not a check to change.
7. Set each feature's `verified_at` to the commit you ran the checks on, run `aayu verify --map` until it is clean,
   and open the PR. The merge gate runs every check of the new map.
