# ops

`ops/actions.toml` is the ops action catalog `aayu respond` (WP5.6) runs from. Nothing else can run: an
action that is not in the catalog is refused.

- **Pinned:** respond reads the catalog through the API from the default branch at its head commit and
  passes that commit to `aayudha-ops.yml`, which checks again that the commit is on the default branch. A
  pull request's copy of the catalog is never read, so a catalog edit can be used only after it merges.
- **Protected path:** edits need human CODEOWNER review. A definition can be `auto` only after the same
  definition has merged as `human`: an action whose definition first appears on the default branch as
  `auto` is treated as `human`.
- **Human approval:** an `approval = "human"` action runs only in a GitHub Environment with required
  reviewers that only protected branches may deploy to; respond refuses any other environment, and the
  executor runs only after a human (not a bot) approved it. The production credential lives only in that
  environment: an OIDC token the job mints (preferred, valid for minutes) or the environment secret
  `AAYUDHA_OPS_CREDENTIAL`. Neither is ever in an agent's environment.

```toml
version = 1

[[action]]
name = "restart-api"                      # lowercase letters, digits, -
summary = "Restart the API deployment."
argv_template = ["scripts/ops/restart.sh", "{service}", "--replicas={replicas}"]  # no shell; {param} fills whole values
approval = "human"                        # auto | human
environments = ["production"]             # GitHub Environments; pick one with environment=<name> when several
max_per_hour = 2
rollback = "none"                         # or another action's name
blast_radius = "service"                  # single | service | global (global is never auto)

[action.params.service]
type = "enum"                             # string (with an anchored pattern), int (min, max), enum (values), bool
values = ["api", "worker"]

[action.params.replicas]
type = "int"
min = 1
max = 5
```

String values must match the parameter's anchored pattern and may not start with `-`. An argv template may
not run an inline shell command (`sh -c …`); put the script in the repository.
