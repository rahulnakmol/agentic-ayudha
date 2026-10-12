# rahulnakmol/agentic-ayudha#134: Ask page shows answers with their sources, served on seeded data

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `c740ce97b5a22ca38233fb6865ae9bea8a95b212ea28116dbe8cfef363b7928e` |
| Completion Goal hash | `280369fef19c85dd5e87ec3a139134a54e521a72bc0e2ec8ab5f66849bf74f46` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/134/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: `/` on seeded data with no question asked; expected: the page shows the heading "Ask about your benefits", a labelled "Your question" field, an Ask button, and three suggested questions, styled only with design-system tokens
2. [screenshot] state: `/?q=What+is+the+dental+annual+deductible%3F` on seeded data; expected: the Answer card quotes the dental deductible passage and names its source "Dental, Deductible"
3. [screenshot] state: `/?q=What+is+my+colleague%27s+claim+status%3F` on seeded data; expected: the page says "We cannot answer this" and explains that another person's records are not shown
4. [api-trace] state: the benefits-qa command started with -addr on a free port and -events; expected: it serves /healthz, the Ask page, and POST /api/ask, appends the visit and answer events to the export, and stops cleanly
5. [api-trace] state: `deploy/serve.sh` run with BQA_PORT set; expected: it builds the example, starts it detached, returns once /healthz answers, and a second run replaces the first instance

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As an Example Co employee, I want an accessible Ask page that shows the quoted answer with its source, a referral, or a refusal, so that I can get benefits answers without an HR ticket.

### Completion Goal

1. [screenshot] state: `/` on seeded data with no question asked; expected: the page shows the heading "Ask about your benefits", a labelled "Your question" field, an Ask button, and three suggested questions, styled only with design-system tokens
2. [screenshot] state: `/?q=What+is+the+dental+annual+deductible%3F` on seeded data; expected: the Answer card quotes the dental deductible passage and names its source "Dental, Deductible"
3. [screenshot] state: `/?q=What+is+my+colleague%27s+claim+status%3F` on seeded data; expected: the page says "We cannot answer this" and explains that another person's records are not shown
4. [api-trace] state: the benefits-qa command started with -addr on a free port and -events; expected: it serves /healthz, the Ask page, and POST /api/ask, appends the visit and answer events to the export, and stops cleanly
5. [api-trace] state: `deploy/serve.sh` run with BQA_PORT set; expected: it builds the example, starts it detached, returns once /healthz answers, and a second run replaces the first instance

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Start the example with deploy/serve.sh on its synthetic seed and open the three page states; run the command and the script under test.

### Environment

the example served by deploy/serve.sh on 127.0.0.1:8787 ([verify] seed) with the embedded synthetic Example Co seed; synthetic data only (V3), never production
````
