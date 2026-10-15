# The merge gate

The merge gate is one GitHub Actions workflow, `aayudha-merge-gate.yml`, that runs on every non-draft pull request.
It runs `verify` again on the pull request's head commit, asks a reviewer model from a different family to judge
the change, and makes the result available to `ship`. It exists because the builder's own word is not evidence:
the checks that decide a merge must run somewhere the pull request cannot change.

![A pull request update starts the gate workflow taken from the base branch; the merge-gate job verifies the head in a sandbox and keeps the pack as a run artifact, the inspect job reviews it, the pack job checks it and comments, and ship needs all three checks green and the artifact from this run.](img/how-merge-gate.svg)

*Three jobs: one runs the pull request's code with no secrets, one holds the model key and never runs that code, one
checks the pack and comments. None writes the ledger.*

## How it works

### Why pull_request_target

The workflow triggers on `pull_request_target`. GitHub then runs the workflow file as it is on the base branch,
not the copy in the pull request. A pull request that edits the gate, lowers a threshold in the workflow, or swaps
a tool version has no effect on its own run. Its changes apply only after they merge. In this repository,
`.github/` is also a protected path in `.github/CODEOWNERS`, so a change to the gate needs a CODEOWNER review.

That trigger has a known risk: the job runs in the base repository's context, and checking out untrusted code there
can leak secrets. The gate splits the work so that the job running pull request code holds nothing worth stealing.

`aayu init --gates` installs the template from
[internal/initrepo/aayudha-merge-gate.yml](../../internal/initrepo/aayudha-merge-gate.yml) into a product
repository, with the aayudha version filled in. This repository's own copy,
[.github/workflows/aayudha-merge-gate.yml](../../.github/workflows/aayudha-merge-gate.yml), differs in two ways: it
installs aayudha from the `dev` branch, and it installs the web floor tools (Playwright, axe-core, Lighthouse) from the
base branch's `packages/floors` for the example product.

### The three jobs

**`aayudha/merge-gate`** (the `gate` job) has no repository secrets, only the read-only workflow token, and
`verify` scrubs even that from the tools it runs. It:

1. stops if `aayu halt` is in force;
2. checks out the head commit with `persist-credentials: false`;
3. installs aayudha and the pinned floor tools (gitleaks and osv-scanner checked against SHA-256 hashes);
4. runs `go mod download` so the test run can work offline;
5. runs `aayudha verify "$PR" --ci`, writes a floor table to the job summary, and uploads `.aayudha/proof/`, media
   included, as the `aayudha-pack` artifact, kept for `[proof] retention_days` (90 by default) up to the repository's
   artifact retention limit (`retention_days` in `verify`'s JSON).

In CI, `verify` fails the job when a floor misses, and also when the pack does not prove the current head, the
story's Completion Goal hash no longer matches its Ready snapshot, a criterion has no evidence from this run, or the
story has no Ready snapshot. A pull request that names no story passes with nothing to prove.

**`aayudha/inspect`** runs only after the gate job succeeds. It uses the `ai-gateway` environment, which holds the
model keys, and has `pull-requests: write` to post its comment. It checks out the base branch only, downloads the
`aayudha-pack` artifact, and reads the diff through the GitHub API. It never runs pull request code.

**`aayudha/pack`** checks out the base branch, downloads the `aayudha-pack` artifact, and runs `aayudha proof comment`
with the workflow token and `pull-requests: write`. It verifies the pack against the pull request's current head and
the Ready snapshot (every artifact hashed), keeps one comment marked `<!-- aayudha:proof -->` with each criterion's
verdict, the floor table, the SHA-256 of `pack.json`, and links to the artifact and the run, and fails when the pack
does not verify or a floor does not hold. It writes nothing to the ledger. See [proof packs](how-proof-packs.md).

### The verify sandbox

Every floor tool runs as a child process through `verify.Sandbox`:

- **Scrubbed environment.** The child gets an allow-list of variables (`PATH`, locale, Go and Node paths) and a
  fresh temporary `HOME`. Anything that looks like a credential is dropped: `AAYU_*`, `GH_*`, `GITHUB_*`, and any
  name containing `TOKEN`, `SECRET`, or `PASSWORD`. The parent's `HOME`, where `gh` and git keep tokens, is never
  passed.
- **No network where feasible.** On Linux, a floor that does not need the network runs in new user, network, and
  PID (process ID) namespaces. Only loopback is up, and the tool runs as PID 1 of its namespace. When it exits,
  everything it started is killed, so test code cannot outlive the floor and tamper with the pack. The pack records
  `"network": "isolated"`.
- **Degraded mode.** Where unprivileged namespaces are unavailable (macOS, Windows, or a Linux host that forbids them),
  the child gets proxy variables pointing at a closed port and `GOPROXY=off`, recorded as `proxy-blocked`. The gate
  runs `sysctl -w kernel.apparmor_restrict_unprivileged_userns=0` so Ubuntu runners get the full sandbox.
- **Network when needed.** semgrep (rule packs), osv-scanner (vulnerability database), the web floors, and
  `[verify] seed` need the network and record `"network": "allowed"`. They still get the scrubbed environment.

### How inspect works

`aayu inspect <pr> --ci` gathers its input: the pull request title, body, and diff; the story issue; the criteria
from the Ready snapshot; the pack for the current head; and excerpts (first 3,000 bytes) of the pack's text
artifacts. Media never reach the model.

1. **Family check.** The builder's family comes from what aayu recorded on the ledger, outside the pull request: the
   delegation record (`aayu delegate`), or the model `aayu build` wrote for each checkpoint commit
   (`build/<story>/state.json`). It is known only when a record covers the pull request with one family and the pack's
   `model` (from the `Aayudha-Model` trailer, which the builder writes itself) does not contradict it. Otherwise it is
   unknown, and every family the records, the trailer, or (with no record) the build lane names is a candidate. Each
   reviewer is of none of them; with none left, inspect refuses and exits 4. The verdict's first note names the builder
   family and where it came from.
2. **Prompt.** Floor results are given as facts. Everything a person or tool wrote (criteria, story, evidence, pull
   request text, diff capped at 120 kB) is wrapped in `<untrusted>` blocks the system prompt says never to obey.
3. **Structured reply.** The model must answer in a closed JSON Schema: `disposition` (`merge`, `fix`, or
   `rebuild`), a summary, per-criterion verdicts (`met`, `unmet`, `unclear`), and findings with severity P0 to P3.
4. **Enforce.** A floor that does not hold, a P0 or P1 finding, or an unmet criterion turns `merge` into `fix`, with
   the reason recorded in `overridden`. The model can downgrade a disposition; nothing upgrades it past a floor.
5. **Comment.** inspect keeps one comment on the pull request, marked `<!-- aayudha:inspect -->`, and replaces it in
   place. A hidden `aayudha:inspect-state` JSON holds the input hash, the cycle count, and the spend.

State drives three limits:

- **Cache.** The input hash covers the diff, the pack's head, Completion Goal hash, and checks, and the criteria.
  Same inputs, no model call.
- **Cycle cap.** Each `fix` or `rebuild` on a new input counts one cycle. After two, the next run pages a human and
  exits 4 without calling the model.
- **Budget.** Spend per pull request is capped by `[budget] per_pr_inspect_max`, default 5.00 USD. At the cap,
  inspect pages and exits 6.

inspect exits 0 on `merge` and 4 on `fix` or `rebuild`, so its check stays red until the reviewer says `merge`.

> **Honest status:** inspect does not read the story's plan in this repository; its inputs are the pull request,
> the diff, the story, the Ready criteria, and the pack. The `fix` re-entry is a fix in the same pull request. A build
> record holds the model `build` resolved (`AAYU_MODEL`, else the host's build lane), and without a bot App the
> founder's token, which agents use here, can write the ledger, so the verdict calls that family attributed, not
> verified. A pull request made without `aayu build` or `aayu delegate` has no record, so its builder family is unknown.

### How ship consumes the result

`ship` first finds the pack: the `aayudha-pack` artifact of the newest successful `pull_request_target` run of this
workflow on exactly the head commit, with every hash matching `pack.json`. A `pull_request` run of the pull request's
own copy of the workflow, a run on another commit, or an artifact from any other workflow never counts. Then it
checks the newest `aayudha/merge-gate`, `aayudha/inspect`, and `aayudha/pack` check runs on the head (all must be
`success`), verifies the pack against the Ready snapshot, and requires the floors to hold. Only then does it look at
the human pass, and only after that does it land `pack.json` on the ledger, under the human's identity, and merge.

> **Honest status:** the repository runs in solo mode, with one human, so every `aayu ship` here runs under the
> founder's identity with a recorded bypass.

## Key data and file formats

The trigger and the key steps, from
[.github/workflows/aayudha-merge-gate.yml](../../.github/workflows/aayudha-merge-gate.yml):

```yaml
on:
  pull_request_target:
    types: [opened, synchronize, reopened, ready_for_review, edited]

permissions:
  contents: read

concurrency:
  group: aayudha-merge-gate-${{ github.event.pull_request.number }}
  cancel-in-progress: true
…
jobs:
  gate:
    name: aayudha/merge-gate
    if: github.event.pull_request.draft == false
…
      - name: Check out the PR head (run only inside aayu verify's sandbox)
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ${{ github.event.pull_request.head.sha }}
          fetch-depth: 0
          persist-credentials: false
          path: head
…
      - name: Verify the PR head
        id: verify
        working-directory: head
        env:
          GITHUB_TOKEN: ${{ github.token }}
        run: |
          set +e
          aayudha verify "$PR" --ci --repo "$REPO" --json > "$RUNNER_TEMP/verify.json"
…
  inspect:
    name: aayudha/inspect
    needs: gate
    if: needs.gate.outputs.story != '0'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    environment: ai-gateway
    permissions:
      contents: read
      pull-requests: write
      issues: read
```

The inspect output, from [internal/inspect/inspect.go](../../internal/inspect/inspect.go):

```go
// Result is what one inspection decided.
type Result struct {
    PR          int       `json:"pr"`
    Head        string    `json:"head,omitempty"`
    Disposition string    `json:"disposition"`
    Summary     string    `json:"summary"`
    Findings    []Finding `json:"findings"`
    Criteria    []Verdict `json:"criteria"`
    Model       string    `json:"model,omitempty"`
    Overridden  string    `json:"overridden,omitempty"` // why the floors changed the model's disposition
    DiffHash    string    `json:"diff_hash"`
    Cached      bool      `json:"cached,omitempty"`
    Cycle       int       `json:"cycle"`
    CostUSD     float64   `json:"cost_usd"`
    Next        string    `json:"next,omitempty"` // the re-entry step
}
```

The real inspect comment on pull request 148 (story 131), from
`gh api repos/rahulnakmol/agentic-ayudha/issues/148/comments`, shortened:

```text
<!-- aayudha:inspect -->
### inspect: merge

The implementation satisfies all three completion criteria. Retrieval returns the full selected passage with its
citation, policy screens the specified prohibited questions before retrieval, and unsupported questions return an
HR referral without quoted content. …

| Criterion | Verdict | Evidence |
|---|---|---|
| 1 | met | TestAnswerCitesPassage passed against store.Load() and verifies the dental-deductible passage ID, … |
| 2 | met | TestScreenRefuses and TestRespondScreensFirst passed for both specified questions, … |
| 3 | met | TestGroundRefers passed for the dog-at-work question, … |

Reviewer `gpt-6.1-sol` · cycle 1 of 2 · spent 0.37 of 5.00 USD on this PR · inputs `6472237b101b`

<!-- aayudha:inspect-state {"diff_hash":"6472237b101b…","result":{…},"cycles":1,"spent_usd":0.37063500000000005,"hashes":["03c39e12a4aa…","6472237b101b…"]} -->
```

The state shows two inputs. The first review did not return `merge`, which counted as cycle 1; the second, on the
fixed head, returned `merge`, and the spend covers both calls.

> **In the story:** in benefits-qa, five stories took 17 inspect runs. Pull requests 135, 140, 151, and 156 reached
> the two-cycle cap; each was closed with a note and its story rebuilt as a new pull request. See
> [Prove it](journey-5-prove.md).

## Invariants

1. The gate runs the base branch's workflow, so a pull request cannot weaken its own checks. Enforced by the
   `pull_request_target` trigger in
   [internal/initrepo/aayudha-merge-gate.yml](../../internal/initrepo/aayudha-merge-gate.yml).
2. Pull request code runs only in a job without secrets, and only inside the sandbox: scrubbed environment, no
   network for offline floors, own PID namespace. Enforced in [internal/verify/exec.go](../../internal/verify/exec.go)
   and [internal/verify/netns_linux.go](../../internal/verify/netns_linux.go), `TestChildEnvironmentIsScrubbed`,
   `TestChildHasNoNetwork`, `TestChildLeavesNoProcesses`.
3. The gate fails when the pack does not prove the head, the Completion Goal drifted, or a criterion lacks evidence.
   Enforced by `gateProblems` in [internal/cli/verify.go](../../internal/cli/verify.go); `TestVerifyGateCI` covers
   missing evidence, Completion Goal drift, and a pull request with no story.
4. The reviewer's model family differs from the builder's, and from every family the builder may be when it is
   unknown. Enforced by `CheckFamilies` in [internal/models/resolve.go](../../internal/models/resolve.go) and
   `inspect.BuilderOf` (`TestReviewerFamilyMustDiffer`, `TestBuilderOf`, `TestInspectBuilderFamily`).
5. No model disposition overrides a floor, a P0 or P1 finding, or an unmet criterion. Enforced by `Enforce` in
   [internal/inspect/inspect.go](../../internal/inspect/inspect.go), `TestInspectDispositions`.
6. Same inputs never cost a second model call; after two cycles inspect pages without a call; spend stops at the
   per-pull-request budget. Enforced in [internal/inspect/inspect.go](../../internal/inspect/inspect.go),
   `TestInspectDispositions` and `TestInspectMergeBudgetDraft`.
7. `ship` merges only with the three gate checks green on the head and the pack from this workflow's own
   `pull_request_target` run on that head, every hash matching, verifying against the Ready snapshot. Enforced in
   [internal/cli/ship.go](../../internal/cli/ship.go), `TestShipRefusals`.
8. The pack job uses only the workflow token, writes nothing to the ledger, and fails on a pack that does not verify
   or a floor that does not hold. Enforced by `proofComment` in [internal/cli/proof.go](../../internal/cli/proof.go),
   `TestProofCommentInMergeGate`.

## Where it lives in the code

- [internal/initrepo/aayudha-merge-gate.yml](../../internal/initrepo/aayudha-merge-gate.yml): the template products get.
- [.github/workflows/aayudha-merge-gate.yml](../../.github/workflows/aayudha-merge-gate.yml): this repository's gate.
- [internal/cli/verify.go](../../internal/cli/verify.go): `verify --ci` and the gate problems.
- [internal/cli/proof.go](../../internal/cli/proof.go): `proof comment`, the pack job.
- [internal/verify/exec.go](../../internal/verify/exec.go),
  [internal/verify/netns_linux.go](../../internal/verify/netns_linux.go), and
  [internal/verify/netns_other.go](../../internal/verify/netns_other.go): the sandbox.
- [internal/inspect/inspect.go](../../internal/inspect/inspect.go) and
  [internal/cli/inspect.go](../../internal/cli/inspect.go): the review, its state, and the comment.
- [internal/cli/ship.go](../../internal/cli/ship.go): how `ship` reads the gate.

## Read next

- [Proof packs](how-proof-packs.md): what the gate writes.
- [Floors in detail](how-floors.md): what `verify` runs inside the gate.
