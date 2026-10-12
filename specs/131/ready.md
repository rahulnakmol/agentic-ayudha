# rahulnakmol/agentic-ayudha#131: Answers quote the best plan passage; policy refuses or refers the rest

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `8cb726034148c5b09e52e62608a4e051745005556e9fd7c7ab7fea94bd1a02dc` |
| Completion Goal hash | `e6e99c0c0e6aa4c1d6044a1ae26d91d9df3530e0a18d9d0b5883bd0d796e28cb` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/131/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the question "What is the dental annual deductible?" against the seed; expected: the answer quotes the dental-deductible passage in full with plan Dental and section Deductible, and policy grounds it
2. [api-trace] state: the questions "What is my colleague's claim status?" and "Should I take ibuprofen for my back?"; expected: policy refuses each before retrieval, with reason personal-data or advice and a message that says why
3. [api-trace] state: the question "Can I bring my dog to work?"; expected: it is referred to an HR specialist as not in the plan documents, and no passage is quoted

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As an Example Co employee, I want my benefits question answered by quoting the plan passage that covers it, so that I can trust the answer, and I want questions the plan does not answer sent to HR rather than guessed.

### Completion Goal

1. [api-trace] state: the question "What is the dental annual deductible?" against the seed; expected: the answer quotes the dental-deductible passage in full with plan Dental and section Deductible, and policy grounds it
2. [api-trace] state: the questions "What is my colleague's claim status?" and "Should I take ibuprofen for my back?"; expected: policy refuses each before retrieval, with reason personal-data or advice and a message that says why
3. [api-trace] state: the question "Can I bring my dog to work?"; expected: it is referred to an HR specialist as not in the plan documents, and no passage is quoted

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Ask a covered question, two questions policy must refuse, and one the plan documents do not cover.

### Environment

local checkout with the embedded synthetic Example Co seed; synthetic data only (V3), never production

### Links

depends-on #130
````
