# benefits-qa feature map

The maintained source for verifying what an Example Co employee can do in benefits-qa. Read this index, then the
feature file for the change under test. `control-bqa sweep` walks the features in the order listed here.

## Baseline preconditions

- The app runs from `control-bqa up --port <port>` (it calls `deploy/serve.sh`, the `[verify] seed`) on the embedded,
  synthetic plan documents. Nothing else seeds or mutates data.
- `control-bqa doctor --port <port>` passes: the pid on `/healthz` is the one `up` started, built from `HEAD`.
- Each journey starts from a fresh page load; the question travels in the URL (`/?q=…`), so no state carries over.

## Driving conventions

- Find controls by role and accessible name, or by their label. Never by CSS class or position.
- Wait for the result heading, not for a fixed time.
- Assert what the employee reads (the heading, the quoted passage, the Source line, the message), then cross-check
  through `POST /api/ask` where the feature names it.

## Proof and skip reporting

- A screenshot or recording shows the question and the result together; an `api-trace` holds the request, the
  status, and the body.
- Record the feature and journey ID with every artifact (`result.json` does).
- A journey that cannot run reports `unreachable` with the precondition it lacks. It is never reported as passed
  through another path.

## Feature entry contract

Each feature file has an H1, one paragraph of user-visible behaviour, one fenced `feature` block (JSON: `id`,
`layers`, `sources`, `routes`, `verified_at`), and four H2s in this order: `Sub-features`,
`How to get to it (user POV)`, `Driving it with control-bqa` (one fenced `journey` block per journey), `Gotchas`.

## Features

1. [Ask and get an answer](ask-answer.md): typed and suggested questions answered with a quoted, cited passage.
2. [Referral to HR](refer-to-hr.md): questions the plan documents do not answer go to an HR specialist.
3. [Refusals](refusals.md): another person's records and medical or financial advice are refused.
4. [Ask API](api-ask.md): `POST /api/ask` and its contract, including the errors.
5. [Product events](product-events.md): visits and questions recorded for analytics.
