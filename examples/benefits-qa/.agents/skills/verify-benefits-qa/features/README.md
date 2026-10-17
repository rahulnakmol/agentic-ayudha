# benefits-qa feature map

What an Example Co employee, or a client of the API, can do in benefits-qa, and the behaviour check that proves
each capability (docs/BUILD-PLAN.md WP4.5b). Read this index, then the feature file for the change.
`aayu verify --map` keeps it honest against the source; `aayu verify --check all` runs every check in the order
below.

```map
{"product": "benefits-qa",
 "control": ["node", "examples/benefits-qa/.agents/skills/verify-benefits-qa/control.mjs"],
 "surfaces": [{"kind": "route", "files": ["examples/benefits-qa/**/*.go"], "exclude": ["**/*_test.go"],
               "pattern": "HandleFunc\\(\"([^\"]+)\""}]}
```

## Baseline preconditions

- aayu starts the app with `[verify] seed` (`examples/benefits-qa/deploy/serve.sh`) on the embedded, synthetic plan
  documents (V3), and passes its URL from `[verify.environments.local]`. Nothing else seeds or changes data.
- Each check starts from a fresh page load; the question travels in the URL (`/?q=…`), so no state carries over.

## Conventions

- Find controls by role and accessible name, or by their label; never by CSS class or position.
- Wait for the result heading, not for a fixed time.
- Assert what the employee reads (the heading, the quoted passage, the Source line, the message), then cross-check
  through `POST /api/ask` where the feature names it.
- A check that cannot run reports `unreachable` with the precondition it lacks; it is never passed another way.

## Features

1. [Ask and get an answer](ask-answer.md): typed and suggested questions answered with a quoted, cited passage.
2. [Referral to HR](refer-to-hr.md): questions the plan documents do not answer go to an HR specialist.
3. [Refusals](refusals.md): another person's records and medical or financial advice are refused.
4. [Ask API](api-ask.md): `POST /api/ask` and its contract, including the errors.
5. [Product events](product-events.md): visits and questions recorded for analytics.
