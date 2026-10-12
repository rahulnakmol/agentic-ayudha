# rahulnakmol/agentic-ayudha#129: Ask page (benefits-qa)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, feature |
| Body hash | `26794cbc8393516022edae4a0c3c7aca8824bb64a840dc50a80c17800ba37382` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/129/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Ask page: an accessible web page, built only from the design-system tokens, where an employee types a question and sees the quoted answer with its source, a referral to HR, or a refusal; the command that serves it with the API; and the script that runs it on seeded data for the web floors. Serves Initiative #126.

### Problem

Employees do not call APIs: the grounded answers reach them only through a page that works by keyboard, at 200 % text, with reduced motion, and on a phone-sized target, and that the merge gate can capture on synthetic data.

### Scope

In scope: the server-rendered Ask page and its stylesheet (design-system tokens only), the `benefits-qa` command (page, API, health check, optional event export), and `deploy/serve.sh`, the `[verify] seed` command that builds and starts the example on its synthetic seed. Out of scope: sign-in, real deployment, and a client-side framework.

### Expected paths

examples/benefits-qa/web/**
examples/benefits-qa/cmd/**
examples/benefits-qa/deploy/**

### Links

depends-on #127
depends-on #128
````
