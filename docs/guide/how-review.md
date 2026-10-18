# Code review in the merge gate

An agent wrote the change; something other than that agent must read it before it merges. By default the merge gate
does this itself: its inspect job sends the diff, the story's promises, and the floors' results to a review model of
another vendor, and turns the reply into the inspect check. No review app, no subscription quota, and no second tool
to manage. You choose the model and pay for its calls. The cost depends on the model and the diff, and `[budget]
per_pr_inspect_max` caps it for each pull request.

This page sets the review up for one repository. [The merge gate](how-merge-gate.md) explains what the inspect check
decides with the review.

## Set up the code-review environment

The review model's endpoint, ID, and key live in a GitHub environment named `code-review`. Only the base branches
(`dev`, `rolling`, `main`) and the repository's default branch can deploy to it. GitHub checks a `pull_request_target`
run against the default branch, so init always lists it. The gate's inspect job never runs pull request code, so no
pull request can reach the key.

1. Run `aayu init --gates`. It installs the merge gate and creates the `code-review` environment, with its
   deployment branches limited to the base branches and the default branch, and two variables filled in:

   | Name | Kind | What it holds | Filled in with |
   |---|---|---|---|
   | `AAYU_REVIEW_MODEL_URL` | variable | the model endpoint's base URL; it must take the key as a bearer token | `https://api.x.ai/v1` |
   | `AAYU_REVIEW_MODEL` | variable | the model ID | `grok-4.7` |
   | `AAYU_REVIEW_MODEL_KEY` | secret | the endpoint's key | nothing: you add it |
   | `AAYU_REVIEW_MODEL_API_STYLE` | variable, optional | the endpoint's wire format: `chat`, `responses`, or `messages` | nothing: unset, aayu uses the model family's own format |

   You should see an `environment code-review` line in init's report that says to set the secret.

2. Open the repository's **Settings → Environments → code-review** and add the secret `AAYU_REVIEW_MODEL_KEY`: your
   key for the endpoint in `AAYU_REVIEW_MODEL_URL`.

3. To use another model, change the two variables. A gateway such as OpenRouter or your own AI gateway reaches
   several model families with one key. A provider's own API works when it takes a bearer key, as xAI's and OpenAI's
   do. Anthropic's API takes its key in another header, so reach Claude models through a gateway. The URL is a base
   URL ending before `/chat/completions`, `/responses`, or `/messages`, with no key or query in it: aayu refuses one
   that has either. A gateway that takes only one wire format, such as Chat Completions for every model, needs
   `AAYU_REVIEW_MODEL_API_STYLE` set to it (`chat`).

4. Open a pull request. You should see the inspect comment's first line,
   `different-family review (B, paid): grok-4.7 (xai)`, and the inspect check pass or fail with the review's findings.

> **Needs setup:** GitHub has no environments for a private repository on GitHub Free. There, `aayu init --gates`
> says it could not create `code-review`; choose a review app (below), or make the repository public or upgrade.

`[inspect] reviewer = "ci"` in `.aayudha/config.toml` selects this review. It is the default, so a repository that
names no reviewer uses it.

`aayu inspect <pr>` in your own terminal cannot reach the key, so it asks no app and no person. It reports the gate's
verdict for the head, or `no-reply` until the gate's inspect check finishes. You should see
`inspect #42 at 1a2b3c4: merge (cached)` once the gate has reviewed the head.

## Choosing a model

The review model must come from a vendor other than the agent that built the change, so the builder never grades its
own work. aayu checks this on every pull request from the builder's recorded family; a model of the builder's family
is never used, and the gate says so.

| You build with | A review model of another family |
|---|---|
| Claude Code, or Amp on Claude models | `grok-4.7`, or a GPT or Gemini model |
| Codex, or Amp on GPT models | `grok-4.7`, or a Claude or Gemini model |
| Cursor or Grok | a Claude, GPT, or Gemini model |

Every review asks for fast (priority) serving where the endpoint offers it, at reasoning effort high, or medium for
Claude Opus, Claude Fable, and GPT-6 Astra; `[inspect] paid_effort` overrides the effort. `[budget]
per_pr_inspect_max` (5 USD by default) caps what one pull request's reviews may spend; the inspect comment records
what each review cost and the tier it was served at.

## When something is missing

The gate fails closed, and its log and the inspect check say which piece is missing and where to set it, for example:

```text
inspect #42: [inspect] reviewer = "ci" calls the model the code-review environment names, and the secret
AAYU_REVIEW_MODEL_KEY is not set: add it in Settings → Environments → code-review (docs/guide/how-review.md)
```

Logs and comments name the model and never print the key or the endpoint's URL. `aayu doctor`'s `review-rung` row
shows the setup steps; it cannot read the environment, so the gate is where a missing piece shows.

## Review apps, if you want one

A review app (Codex, CodeRabbit, Greptile, Claude, or Cursor) can review instead: install it on the account that owns
the repository, then set `[inspect] reviewer` to its name and add it to `[inspect] apps`. The gate then reads the
app's review of the head. When the app cannot review a head (a usage limit, or no reply within 30 minutes), the next
installed app reviews, then the code-review environment's model, then a person. See
[the merge gate](how-merge-gate.md) for how each app's review is read.

## Invariants

1. The review model's key reaches only the merge gate's inspect job, under the `code-review` environment, and that
   job never runs pull request code. Enforced by `TestModelChecksNeverGateAndNeverShip` in
   [workflows_test.go](../../workflows_test.go).
2. Only the base branches and the default branch can deploy to `code-review`; init removes every other rule, on
   every page of the list. Enforced by `reviewEnvironment` in
   [internal/initrepo/extras.go](../../internal/initrepo/extras.go), `TestInitExtras`.
3. Under `reviewer = "ci"`, a missing endpoint, model, or key fails the inspect check, and a URL with credentials or a
   query, or an unknown wire format, is refused and never echoed. With an app as the reviewer, a bad setting only
   leaves the CI fallback unavailable. Enforced by `useReviewModel` and `missingReviewModel` in
   [internal/cli/inspect.go](../../internal/cli/inspect.go), `TestInspectReviewTools`.
4. A model of the builder's family never reviews, a pull request from an author without write access spends nothing,
   and `[budget] per_pr_inspect_max` caps the spend. Enforced by `paidReviewer` in
   [internal/cli/inspect.go](../../internal/cli/inspect.go), `TestInspectPaidReviewer`.
5. When the CI review cannot run, the installed apps review, then a person. Enforced by `inspect.ChooseReviewer` in
   [internal/inspect/apps.go](../../internal/inspect/apps.go), `TestInspectReviewerFallback`.
6. Under `reviewer = "ci"`, a run outside CI without the key posts nothing; it reports the gate's verdict. Enforced
   by `runInspect` in [internal/cli/inspect.go](../../internal/cli/inspect.go), `TestInspectPaidReviewer`.

## Where it lives in the code

- [internal/cli/inspect.go](../../internal/cli/inspect.go): `useReviewModel`, `missingReviewModel`, `paidReviewer`,
  and `reviewEffort`.
- [internal/initrepo/extras.go](../../internal/initrepo/extras.go): `reviewEnvironment`, which init runs.
- [internal/initrepo/aayudha-merge-gate.yml](../../internal/initrepo/aayudha-merge-gate.yml): the inspect job and its
  environment.
- [internal/doctor/reviewer.go](../../internal/doctor/reviewer.go): doctor's reviewer and review-rung rows.

## Read next

- [The merge gate](how-merge-gate.md): the jobs, the review ladder, and what the inspect check decides.
- [Trust model and identities](how-trust.md): who may merge what, and when a person must.
