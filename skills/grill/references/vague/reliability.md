# Vague: reliability (`rel`)

Qualifiers: reliable, reliably, robust, stable, resilient, always available, always on, dependable, bulletproof,
never fails, rock-solid

"Reliable" is a failure the user should not feel, and what happens when it happens anyway. Ask in this order.

## Dimensions

### Failure modes (`rel.failures`)

- Ask: Which failures matter here (a payment provider down, a timeout, a double submit, a lost connection), and
  what should the user see for each?
- Default: the failures the facts show no handling for, each with a visible message and a retry.
- Evidence: screenshot
- Facts: gap, route, code

### No lost or duplicated work (`rel.integrity`)

- Ask: Must a retry or a double click never create a second order or lose what the user typed?
- Default: yes for any action that charges money or creates a record, because duplicates are the costliest bug.
- Evidence: api-trace
- Facts: route, test

### Availability target (`rel.availability`)

- Ask: Is there an availability or error-rate target, over what window?
- Default: none in this story; production availability is not provable on seeded data, so record it as an
  assumption or a parked idea.
- Evidence: metric
- Facts: event

### Recovery (`rel.recovery`)

- Ask: After a failure, how does the user or the system recover: automatic retry, resume where they left off,
  support contact?
- Default: the user can retry from the same screen with their input kept.
- Evidence: recording
- Facts: gap, route
