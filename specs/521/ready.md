# rahulnakmol/agentic-ayudha#521: Host hook reads allow_pii from the working tree, so an agent can opt itself in

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `66f211dd583c54f3c1ffc8f0ca44a016e04d7bcb3d089fc36aea86a475afedc3` |
| Completion Goal hash | `b76dc4deb067749c2ebba5bc435aed7829f188aa6c15e2c852b078e6d8ce503b` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/521/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the pre-tool hook given a personal-data read while `allow_pii = true` is in the working tree only, then committed past `origin/dev`, then on `origin/dev`, and in a repository with no dev branch to compare with; expected: refused, refused, allowed, and allowed at HEAD

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

An agent can opt itself in to personal data. The pre-tool hook read `[integrations.<provider>] allow_pii` from the working tree (Decision #449: personal data is opt-in, set in the protected `.aayudha/config.toml`). An uncommitted edit, or a commit on the agent's own branch, took effect at once, before CODEOWNERS review of the protected path.

### Steps to reproduce

1. In a repository set up with `aayu install`, add `allow_pii = true` under `[integrations.sentry]` in `.aayudha/config.toml` without committing it.
2. Let the host call Sentry's MCP tool `get_replay_details`.

### Expected behaviour

The call is refused: `allow_pii` counts only as committed on the dev branch.

### Actual behaviour

The call passes.

### Completion Goal

1. [api-trace] state: the pre-tool hook given a personal-data read while `allow_pii = true` is in the working tree only, then committed past `origin/dev`, then on `origin/dev`, and in a repository with no dev branch to compare with; expected: refused, refused, allowed, and allowed at HEAD

### Measure

Completion Goal criteria observed

### Threshold

all 1 criteria

### Scenario

Run the pre-tool hook on a personal-data read as the opt-in moves from the working tree to a branch commit to the dev branch, and read each exit code.

### Environment

local checkout and this repository's CI build job on a pull request; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/cli -run TestHookProviderCalls` shows the opt-in counted only as committed on the dev branch (criteria 1)

### Assumptions

- the opt-in is read at the merge base of HEAD and `origin/<dev branch>`, which only a merged, reviewed change moves; a repository with no such remote branch reads HEAD
- the config is read with git only when a call would be refused without the opt-in, so other tool calls cost nothing more

### Provenance

Request: "allow_pii is read from the uncommitted working tree. It should be read from the committed config at HEAD (or the merge base), so an agent can't switch it on by editing the file."
Term: "HEAD" → the commit a checkout is on; here the merge base of HEAD and the dev branch, falling back to HEAD; "read from the committed config at HEAD (or the merge base)"
1. stated (request): "It should be read from the committed config at HEAD (or the merge base), so an agent can't switch it on by editing the file"
````
