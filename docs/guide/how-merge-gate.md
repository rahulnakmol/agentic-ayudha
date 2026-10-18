# The merge gate

The merge gate is one GitHub Actions workflow, `aayudha-merge-gate.yml`, that runs on every non-draft pull request.
It runs `verify` again on the pull request's head commit, reads the review of an agent app from a different family,
and makes the result available to `ship`. It exists because the builder's own word is not evidence:
the checks that decide a merge must run somewhere the pull request cannot change.

![A pull request update starts the gate workflow taken from the base branch; the merge-gate job verifies the head in a sandbox and keeps the pack as a run artifact, the inspect job reviews it, the pack job checks it and comments, and ship needs all three checks green and the artifact from this run.](img/how-merge-gate.svg)

*Three jobs on each update: one runs the pull request's code with no secrets, one reads the reviewer app's reply and
never runs that code, one checks the pack and comments. A fourth re-runs inspect when the reviewer replies. No job
holds an AI key, and none writes the ledger.*

## How it works

### Why pull_request_target

The workflow triggers on `pull_request_target`. GitHub then runs the workflow file as it is on the base branch,
not the copy in the pull request. A pull request that edits the gate, lowers a threshold in the workflow, or swaps
a tool version has no effect on its own run. Its changes apply only after they merge. In this repository,
`.github/` is also a protected path in `.github/CODEOWNERS`, so a change to the gate needs a CODEOWNER review.

That trigger has a known risk: the job runs in the base repository's context, and checking out untrusted code there
can leak secrets. The gate splits the work so that the job running pull request code holds nothing worth stealing, and since 0.2 no
job holds an AI key at all.

`aayu init --gates` installs the template from
[internal/initrepo/aayudha-merge-gate.yml](../../internal/initrepo/aayudha-merge-gate.yml) into a product
repository, with the aayudha version filled in. This repository's own copy,
[.github/workflows/aayudha-merge-gate.yml](../../.github/workflows/aayudha-merge-gate.yml), differs in two ways: it
installs aayudha from the `dev` branch, and it installs the web floor tools (Playwright, axe-core, Lighthouse) from the
base branch's `packages/floors` for the example product.

### The jobs

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

When the story is a Bug, `verify --ci` also runs a **red-on-base** check: each test its evidence map names is run on
the base commit, with the pull request's test files copied over, in the same sandbox. A test that fails on the base
shows the fix; one that passes there does not. The check is advisory until the date in
`[verify] red_on_base_blocking_from` (`2026-11-13` by default, four weeks after the founder's decision): before it, a
test that is green on the base is reported and skipped; from it, the check fails. Only Go modules are run today.

**`aayudha/inspect`** runs after the gate job succeeds, on every pull request that is ready for review, whether or not
it serves a story. It holds no AI key (founder decision of 2026-10-15): it reads the reviewer app's reply on the pull
request and has `pull-requests: write` to post its comment. It checks out the base branch only, downloads the
`aayudha-pack` artifact when the pull request serves a story, and reads the diff through the GitHub API. It never runs
pull request code. A pull request without a story is judged on the reviewer's disposition and the coverage manifest,
with no criteria. One that changes only documentation passes without a review: files under `docs/`, Markdown at the
repository root, and `README.md` files. Test fixtures, the skills, seats, hosts, work items, evals, and agent and
workflow config never count as documentation. Neither do the steering documents: any `AGENTS.md` or `CLAUDE.md`, the
build plan, the constraints, the contributing guide, the ethos, the glossary, the threat model, and the security
policy. A rename counts by both its old and new paths.

**`aayudha/inspect-reply`** (the `reply` job) runs on `issue_comment` and `pull_request_review` events, never on
`pull_request_target`. It checks out the default branch, holds `actions: write` and read access, and runs
`aayudha inspect "$PR" --ci`. When the chosen reviewer has replied for the head, usable or not, and the head's gate
run has a green `aayudha/merge-gate` and a red `aayudha/inspect`, it re-runs that run's failed jobs, so the inspect
check on the head commit itself carries the verdict. Any other comment changes nothing.

**`aayudha/pack`** checks out the base branch, downloads the `aayudha-pack` artifact, and runs `aayudha proof comment`
with the workflow token and `pull-requests: write`. It verifies the pack against the pull request's current head and
the Ready snapshot (every artifact hashed), keeps one comment marked `<!-- aayudha:proof -->` with each criterion's
verdict, the floor table, the SHA-256 of `pack.json`, and links to the artifact and the run, and fails when the pack
does not verify or a floor does not hold. It writes nothing to the ledger. See [proof packs](how-proof-packs.md).

The proof comment also points the human pass, from the diff alone and with no model. A **risk line** gives a tier:
high for a never-layer file, a protected path, a changed dependency manifest, or a migration; medium for three or
more layers or a large diff (over 400 changed lines or 20 files); low otherwise. It names up to five files to read
first. Under it, **Checks this PR weakens** lists each weakened check, as inspect does. Neither approves or blocks
anything: a pull request may edit its own checks, and a human decides.

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

A product repository's CI holds no AI key (founder decision of 2026-10-15). inspect's reviewer is an agent app
installed on the organisation or account, from a family other than the builder's when one is installed. aayu asks it
with an @-mention and reads its reply deterministically:

| App | Request | Replies as | Family |
|---|---|---|---|
| Codex | `@codex review` | `chatgpt-codex-connector[bot]` | OpenAI |
| CodeRabbit | `@coderabbitai review` | `coderabbitai[bot]` | review service: a different-family review for any builder |
| Greptile | `@greptileai` | `greptile-apps[bot]` | review service: a different-family review for any builder |
| Claude | `@claude review` | `claude[bot]` | Anthropic |
| Cursor | `@cursor` | `cursor[bot]` | none until `[inspect] app_families.cursor` names one: Cursor runs several vendors' models |
| Amp | — | — | cannot review: Amp acts as the account that runs it, so its reply cannot be told from a person's |

`[inspect] apps` lists the apps installed (`aayu doctor`'s `reviewer-apps` row and `aayu install` detect them from
the account's installations, and from the comments, reviews, and checks on the last ten pull requests).
`[inspect] reviewer` names the one review tool: `codex`, `coderabbit`, `greptile`, `claude`, `cursor`, `grok` (the
paid reviewer through the AI gateway, called directly), or `human`. There is no automatic choice (founder,
2026-10-18). `aayu install --apply` asks for it and writes it. Unset, a person reviews every pull request, the inspect
comment says how to choose, and `aayu doctor` fails with the same step. The chosen tool is the only reviewer while it
answers; a same-family app gives a labelled same-family review.

CodeRabbit and Greptile are dedicated review services. Their review counts as different-family whatever the builder's
family. The founder decided this on 2026-10-18: "When CodeRabbit or Greptile is used for code review we don't need to
worry about model family, as they are more robust." It is a deliberate exception to the family rule. aayu reads their
own review of the head: its inline comments on the head commit, with CodeRabbit's Critical, Major, Minor, and Trivial
as P0 to P3, and Greptile's P0 to P2 badges. A high-risk pull request still gets this one reviewer, never two. `aayu doctor`'s
`review-rung` row shows the rung this checkout's builds get, from the detected host's build lane and the listed
apps, and names the apps that would move it up; it calls no model.

The review the gate records, and shows as the first line of the inspect comment, is one of three:

| Review | When | Counts toward autonomy |
|---|---|---|
| Different-family | The reviewer's model is of a family outside every family the builder may be | yes |
| Different-family, paid (rung B) | `reviewer = "grok"`, or no installed app can review the head, and the repository opted into a paid reviewer of another family | yes |
| Same-family | The reviewer is of the builder's family, or the builder's family is unknown | never; never layers and protected paths still need a person |
| Human | A person reviewed | a person's review |

The reviewer's family is the app's family. Only an app whose family is host-managed (Cursor) takes it from the
optional `model` field of its reply; an app of one family that reports another family's model gives a same-family
review. A reply that reports a model of the builder's family is a same-family review, whatever was planned. `aayu ship` refuses an
autonomous pass unless the inspect job of the base branch's `pull_request_target` merge gate run on the head printed a
different-family review in its log, so after a same-family review, or with no such record, a person passes it. A run whose judgement was cached from the state comment does not count; ship reads the newest fresh judgement of the head. A
comment, or a run of the branch's own workflow, never counts. `reviewer = "model"` keeps the 0.1 model lane for a repository that accepts a key in CI.

**Rung B, a paid reviewer (opt-in).** The founder's one deliberate exception to "no AI keys in a product repository's
CI" (2026-10-18), off by default. It needs both steps:

1. `[inspect] paid_reviewer` in `.aayudha/config.toml` names a model of a family the builder cannot be, for example
   `gemini-3.8-flash` for Amp, Claude Code, or Codex builds.
2. The repository adds that provider's pay-as-you-go key as an Actions secret named `AAYU_SECRET_<FAMILY>`:
   `AAYU_SECRET_GOOGLE` for Gemini, `AAYU_SECRET_XAI` for Grok, `AAYU_SECRET_OPENAI`, or `AAYU_SECRET_ANTHROPIC`.

`[inspect] paid_effort` (`low`, `medium`, `high`, or `xhigh`) sets the reasoning effort the paid reviewer is called at;
empty leaves the provider's default.
Every paid review asks for fast serving (`service_tier` `priority`) and is charged at the model's priority rate, 2x the
standard rate for Grok 4.7, so the cap holds. The inspect comment records the tier the reply was served at, and flags
one that is not fast.

**When the chosen tool cannot review.** An app cannot review a head when it replies that it reached a usage limit,
quota, or rate limit (Codex: "You have reached your Codex usage limits for code reviews"), or has not replied 30
minutes after the review request. Then the next installed app reviews, in a fixed order: codex, coderabbit, greptile,
claude, cursor. After the apps comes the paid reviewer, with the same key, cap, and checks, then a person
(`[autonomy] approver`). When `grok` is chosen and cannot review, the apps review in that order, then a person. The
inspect comment's note names each step and why. A same-family app is labelled so at any step, and the paid reviewer
is never of a family the builder may be. The gate notices the 30 minutes only when its inspect job runs again: a push,
a re-run of the failed job, or a bot's comment or a review on the pull request. The 30 minutes run from that app's own
request. CodeRabbit and Greptile review every head unasked, so for them they run from the head's first request; an app
that reviews only when asked (Claude, Cursor) waits until `aayu inspect <pr>` outside CI asks it.

When the gate uses the paid reviewer, the inspect job calls the model with the key and does not @-mention an app. It
reviews a PR that serves no story against its title, description, and diff. `[budget] per_pr_inspect_max` (5 USD by
default) caps one PR's spend across its cycles, and a cap of 0 turns rung B off. Only a PR whose author has write access
to the repository (GitHub's permission level, not the author association) gets a paid review, so a person who can
only read cannot open PRs to spend the key. The review is labelled
`different-family review (B, paid)` everywhere and counts toward autonomy as rung A does. If the secret is missing, the
model is of a family the builder may be, the cap is 0, the paid call fails (a gateway timeout on a large diff), or the PR
has spent its cap, the gate logs why and the apps review. A failed call's cost still counts toward the cap. A failed paid
call is recorded against the head: later runs on that head, such as the one a reply triggers, go straight to the app, and
the next push tries the paid reviewer again. A run triggered by a reply makes no paid call itself. It only re-runs the
head's inspect job. `aayu doctor`'s
`review-rung` row shows B when it is configured. Doctor cannot read repository secrets, so it names the secret the gate
needs. The inspect job checks out the base branch and runs no PR code. The diff reaches the model only as delimited
untrusted data.

1. **Family check.** The builder's family comes from what aayu recorded on the ledger, outside the pull request: the
   delegation record (`aayu delegate`), or the model `aayu build` wrote for each checkpoint commit
   (`build/<story>/state.json`). It is known only when a record covers the pull request with one family and the
   `Aayudha-Model` declarations, which the builder writes itself, do not contradict it. The declarations are every
   `Aayudha-Model` trailer on the pull request's commits (`aayu build` writes one, with `Aayudha-Host`), and the
   same lines in the body of a pull request `aayu build` did not open; a session that changed model declares each
   one. Otherwise every family the records, the declarations, or the build lane names is a candidate, and a
   different-family reviewer is of none of them. Each commit, body, or pack that names a host but no model adds that
   host, whatever the others declare: the host picked the model, so the families its modes may run are candidates
   too (Amp's are Anthropic and OpenAI). Any other such host, a model aayu cannot place
   in a family, or nothing declared at all makes the builder's family unknown: the pull request still gets a
   review, a labelled same-family one, and is never blocked for it. So an OpenAI or Amp builder never gets a
   different-family review from Codex, and Codex's automatic review counts only when the gate chose Codex. The
   verdict's first notes name the builder family, where it came from, and the reviewer.
2. **Request.** `aayu inspect <pr>`, run outside CI by the builder or a person, keeps one comment for the head under
   that person's account: the mention, the criteria, and the stated format, a fenced `aayudha-review` JSON block with
   the head, a disposition (`merge`, `fix`, or `rebuild`), a summary, the files reviewed, findings with severity P0 to
   P3, and a verdict per criterion (`met`, `unmet`, `unclear`). The apps answer the people who connected them, never
   a workflow token, so CI cannot ask. `build` posts the request after each push, and a `delegate` sync posts it
   too; after the last push to a pull request `build` did not open, run `aayu inspect <pr>` yourself. An app's own
   automatic review of the exact head (Codex's) counts as well.
3. **Read.** Only posts by the reviewer's own login count. The newest `aayudha-review` block for the head is checked
   against the schema. Codex will not use a custom format, so its own review counts instead: a receipt naming the
   exact head commit, and its inline findings on that commit, each with a P0 to P3 badge. A person's review counts
   only when the `[autonomy] approver` gave it with `aayu inspect <pr> --approve --reason "<what you checked>"` (story
   639). That command is human-only: it runs through the presence check (the device flow) as the approver every time,
   also on a terminal with no agent marker, records
   the pull request, the head, the approver, the time, and the reason on the ledger (`inspect/approved.jsonl`), and
   posts the review. The gate counts the approver's reply only when such a record names its exact head, so a new push
   needs a new approval. A comment alone, even from the approver's account, does not count: an agent holding the
   person's token could have written it.
4. **Enforce.** A floor that does not hold, a P0 or P1 finding, or an unmet criterion turns `merge` into `fix`. A
   changed file the reviewer did not review fails it too: the sealed coverage manifest lists every changed file as
   reviewed, skipped with a reason (generated, or deleted), or unreviewed (left out of `reviewed`, or past the 120 kB
   diff cap). "No test for X" notes on test files are dropped; a test gap becomes its criterion's verdict instead.
   Tests that only restate the code are counted, never posted. Checks the pull request weakens (fewer assertions, a
   new skip, removed CI steps, `continue-on-error`, a looser threshold) are flagged, not blocked: a pull request may
   edit its own checks, and a human decides.
5. **Fail closed.** No reply for the head is disposition `no-reply`; a reply in neither format, or a block that fails
   the schema, is `malformed-reply`. Both exit 4, so the check stays red; neither counts as a fix cycle.
6. **Comment.** inspect keeps one comment on the pull request, marked `<!-- aayudha:inspect -->`, and replaces it in
   place. A hidden `aayudha:inspect-state` JSON holds the input hash, the cycle count, the review rounds, and the
   coverage manifest. Anyone can post the marker, so the state counts only from a comment by the merge gate's identity
   (the workflow token, the bot App, or the machine user) or by the account the run writes as. That holds for inspect
   in and out of the gate, `inspect --dismiss`, the inbox, a delegated build's follow-up, and `brief`.

State drives three limits:

- **Cache.** The input hash covers the diff, the pack's head, Completion Goal hash, and checks, the criteria, and
  the reply it read. Same inputs, same verdict.
- **Cycle cap.** Each `fix` or `rebuild` on a new input counts one cycle. A `delegate` sync sends inspect's findings
  back to the receiver for two cycles, then pages a human and exits 4. It shares the cap with failed required checks it
  sends back to the builder: when a required check fails on a delegated pull request's head, the sync sends its log tail, redacted and quoted as data, to the
  builder once per head (`protected-paths` and inspect are not sent). The merge gate still decides.
- **Round cap.** A round is one judged review of a distinct head. After three rounds since the last reopen, the next
  head gets no review request and no judged reply: the disposition is `step-back`, inspect exits 4, and the builder
  reconsiders the approach. A comment whose first line is `/aayu review again`, from a person with write or admin
  access to the repository, reopens review for three more rounds; a bot's or a read-only account's does not. From round 2 the request asks only for `git diff <last head>..<head>` plus any P0, and
  lists the earlier findings under "do not repeat". A reply that repeats an earlier P2 or P3 in the same file and
  class has it dropped as `repeat`; a repeated P0 or P1 still blocks but is not posted inline again. A finding that
  names a `class` lists every instance and asks for all of them to be fixed.
- **Pre-existing findings.** A finding whose quote is an unchanged context line of the diff is pre-existing, whether or
  not it names a criterion: it never changes the disposition and is listed apart. Outside CI, inspect files them as
  one Bug naming the commit `git blame` gives for each line (`priority:P1`), once. A P0, and a P1 security finding,
  is never pre-existing: anchored to an unchanged line it may be a control the PR removed nearby, so it blocks.
  `aayu audit` reports the rounds each agent merge's review took to pass.

inspect exits 0 on `merge` and 4 on anything else, so its check stays red until the reviewer says `merge`.

> **Honest status:** Codex's own review judges no criterion, so each is `unclear` unless a P0 or P1 finding names it;
> the pack's evidence proves the criteria. Codex's security review counts as part of it: its inline findings on the
> head are `security` findings with their badge's severity, and while Codex's summary records a security review of the
> head that has not completed, the head has no reply yet (Bug 562). Claude and Cursor were not shown answering in the
> stated format: Claude did not answer on the test repository (not installed there) and Cursor's Bugbot was disabled
> for it. In solo mode the
> person who approves under the human fallback holds the same account the agents use, so that approval is attributed,
> not verified. Live, on `rahulnakmol/aayu-scratch-1792115145474361918` (kept, `TestLiveInspectApp`): PR 4's
> inspect check failed with `no-reply`, `aayu inspect 4` posted `@codex review`, Codex answered with a receipt within
> two minutes, the reply job re-ran the gate (run 38837238758, attempt 2), and the check turned green with `merge`
> from "comment 6206322212 by chatgpt-codex-connector[bot] (Codex's own review of 221013b15cb2)"; PR 5's reviewer,
> Claude, never answered, a person's forged `aayudha-review` block re-ran nothing ("no reply from claude for this
> head; nothing to re-run"), and its check stayed red.

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
  issue_comment:
    types: [created, edited]
  pull_request_review:
    types: [submitted, edited]

permissions:
  contents: read

concurrency:
  group: aayudha-merge-gate-${{ github.event_name == 'pull_request_target' && 'gate' || 'reply' }}-${{ github.event.pull_request.number || github.event.issue.number }}
  cancel-in-progress: true
…
jobs:
  gate:
    name: aayudha/merge-gate
    if: github.event_name == 'pull_request_target' && github.event.pull_request.draft == false
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
    if: github.event_name == 'pull_request_target' && needs.gate.outputs.story != '0'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      contents: read
      pull-requests: write
      issues: read
…
  reply:
    name: aayudha/inspect-reply
    if: >-
      (github.event_name == 'issue_comment' && github.event.issue.pull_request && github.event.issue.state == 'open' &&
      (endsWith(github.event.comment.user.login, '[bot]') || contains(github.event.comment.body, 'aayudha-review'))) ||
      (github.event_name == 'pull_request_review' && github.event.pull_request.state == 'open')
    permissions:
      actions: write
      checks: read
      contents: read
      pull-requests: read
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
    Mode        string    `json:"mode,omitempty"`
    Reviewers   []string  `json:"reviewers,omitempty"` // A, then B in peer mode
    Plan        []Area    `json:"plan,omitempty"`
    Dropped     []Dropped `json:"dropped,omitempty"` // findings not posted, with why
    Notes       []string  `json:"notes,omitempty"`
    Overridden  string    `json:"overridden,omitempty"` // why the floors changed the model's disposition
    DiffHash    string    `json:"diff_hash"`
    Cached      bool      `json:"cached,omitempty"`
    Cycle       int       `json:"cycle"`
    CostUSD     float64   `json:"cost_usd"`
    Next        string    `json:"next,omitempty"` // the re-entry step
    Builder     Builder   `json:"builder"`        // the builder's family and where it came from …
    // Coverage is the sealed manifest of what the review covered; Weakened
    // lists the checks this PR made weaker (flags, not verdicts).
    Coverage *Manifest   `json:"coverage,omitempty"`
    Weakened []Weakening `json:"weakened,omitempty"`
}
```

A 0.1 inspect comment, reviewed by a model in the gate, on pull request 148 (story 131), from
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
4. A review is different-family only when the reviewer's reported family is outside every family the builder may
   be; otherwise it is labelled same-family, and with no app a person reviews. A same-family review never gives an
   autonomous pass. Only the reviewer's own login counts, and a missing or malformed reply fails closed. Enforced by
   `inspect.ChooseReviewer`, `Builder.Rung`, and `inspect.ReadReply` in
   [internal/inspect/apps.go](../../internal/inspect/apps.go) and `inspect.BuilderOf` (`TestReviewLadder`,
   `TestInspectThroughApp`, `TestBuilderOf`, `TestShipAutonomousFootprint`).
5. No model disposition overrides a floor, a P0 or P1 finding, or an unmet criterion. Enforced by `Enforce` in
   [internal/inspect/inspect.go](../../internal/inspect/inspect.go), `TestInspectDispositions`.
6. Same inputs give the same verdict; after three review rounds inspect steps back and pages until a person reopens
   review; a changed file the reviewer did not review fails the gate. Enforced in [internal/inspect/inspect.go](../../internal/inspect/inspect.go) and
   [internal/inspect/coverage.go](../../internal/inspect/coverage.go), `TestInspectDispositions` and
   `TestCoverageManifest`.
7. `ship` merges only with the three gate checks green on the head and the pack from this workflow's own
   `pull_request_target` run on that head, every hash matching, verifying against the Ready snapshot. Enforced in
   [internal/cli/ship.go](../../internal/cli/ship.go), `TestShipRefusals`.
8. The pack job uses only the workflow token, writes nothing to the ledger, and fails on a pack that does not verify
   or a floor that does not hold. Enforced by `proofComment` in [internal/cli/proof.go](../../internal/cli/proof.go),
   `TestProofCommentInMergeGate`.
9. No workflow `init --gates` installs holds a model key or the AI-gateway environment. Enforced by
   `TestModelChecksNeverGateAndNeverShip` in [workflows_test.go](../../workflows_test.go).
10. The proof comment's risk line and weakened-check list come from the diff alone and approve nothing. Enforced by
    `inspect.RiskOf` in [internal/inspect/risk.go](../../internal/inspect/risk.go) and `inspect.Weakened` in
    [internal/inspect/coverage.go](../../internal/inspect/coverage.go), `TestRiskOf`, `TestWeakenedChecks`, and
    `TestProofCommentInMergeGate`.
11. A Bug's proving test that passes on the base fails the gate from `[verify] red_on_base_blocking_from` on, and is
    reported but skipped before it. Enforced by `redOnBase` in
    [internal/cli/redbase.go](../../internal/cli/redbase.go), `TestRedOnBase`.
12. A failed required check on a delegated pull request goes back to the builder once per head, within the cycle cap;
    then a human is paged. Enforced in [internal/cli/delegate.go](../../internal/cli/delegate.go) and
    [internal/cli/cirepair.go](../../internal/cli/cirepair.go), `TestDelegateCIRepair`.

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
- [internal/inspect/apps.go](../../internal/inspect/apps.go) and
  [internal/cli/inspect_app.go](../../internal/cli/inspect_app.go): the reviewer apps, the request, the reply, and
  the reply job's re-run.
- [internal/inspect/risk.go](../../internal/inspect/risk.go): the proof comment's risk line.
- [internal/cli/redbase.go](../../internal/cli/redbase.go): the red-on-base check for Bugs.
- [internal/cli/ship.go](../../internal/cli/ship.go): how `ship` reads the gate.

## Read next

- [Proof packs](how-proof-packs.md): what the gate writes.
- [Floors in detail](how-floors.md): what `verify` runs inside the gate.
