# rahulnakmol/agentic-ayudha#171: Inspect's different-family check trusts the builder's own Aayudha-Model trailer

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `6c223023eebb262322fc3fcf2f31eea13c715e8a9a373ce77dfe10488de59ae6` |
| Completion Goal hash | `ea0f4a9d4e0bb6d621121622882bbec005130057888be7e965b4a3b587aaeeed` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/171/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a pull request whose checkpoint trailers name an Anthropic model while the build was done by the review lane's family; expected: inspect either refuses on the same-family rule using a source the builder cannot write, or its verdict states that the builder family is self-reported and unverified
2. [api-trace] state: a pull request with no `Aayudha-Model` trailer; expected: the inspect verdict names the builder family it used and where that came from
3. [api-trace] state: an honest pull request built by a different family than the reviewer; expected: inspect reviews it as before and its verdict names the builder family and its source

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`inspect` refuses to review when the reviewer's model family equals the builder's, but it learns the builder's model from the pack's `model` field, which `verify` reads from the `Aayudha-Model` commit trailer (`modelTrailer` in `internal/cli/verify.go`) that the builder writes into its own checkpoint commits (`internal/cli/build.go`). Without a trailer it falls back to the build lane's default for the pack's host. Either way the different-family rule rests on what the builder says about itself. The docs state this limit: docs/guide/how-merge-gate.md ("Family check").

### Steps to reproduce

1. In a scratch repository with the merge gate installed, build a Ready story with a model of the review lane's family (OpenAI by default).
2. Write `Aayudha-Model: claude-opus-5-5` as the trailer of each checkpoint commit and push the pull request.
3. Let the merge gate run `aayudha/inspect`.

### Expected behaviour

`inspect` does not treat a builder's own claim about its model as proof of a different family: either the builder family comes from a source the builder cannot write, or the verdict says plainly that the builder family is self-reported and unverified.

### Actual behaviour

Found by reading the code, not yet by a live run: `verify` copies the trailer into the pack's `model`, and inspect's family check compares that value with the review lane's family. A trailer naming an Anthropic model therefore passes the check, and the verdict carries no note that the family was self-reported.

### Completion Goal

1. [api-trace] state: a pull request whose checkpoint trailers name an Anthropic model while the build was done by the review lane's family; expected: inspect either refuses on the same-family rule using a source the builder cannot write, or its verdict states that the builder family is self-reported and unverified
2. [api-trace] state: a pull request with no `Aayudha-Model` trailer; expected: the inspect verdict names the builder family it used and where that came from
3. [api-trace] state: an honest pull request built by a different family than the reviewer; expected: inspect reviews it as before and its verdict names the builder family and its source

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Open three pull requests in a scratch repository (a wrong trailer, no trailer, an honest trailer), let the merge gate run inspect on each against a fake model endpoint, and record each verdict and exit code.

### Environment

local scratch repository with seeded, synthetic data, a fake GitHub server, and a fake model endpoint; never production
````
