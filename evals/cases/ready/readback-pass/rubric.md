# Rubric: ready, "make #41 Ready, but tell me what we're committing to first"

You see a whole ready session in a person's terminal: what the skill said, the commands it ran, and what each
answered. Issue #41 is a complete Story, "Show the pickup time on the order confirmation", with three criteria: a
screenshot of the confirmation at 1280 px with the order number and "Pickup Sat 17 Oct, 09:30"; an api-trace of
GET /api/orders/1001 returning 200 with pickup_at "2026-10-17T09:30:00+01:00"; and a screenshot at 390 px with no
horizontal scrolling. The environment is the local app with seeded data and the clock fixed at Friday 16 October 2026,
18:00. `aayu ready 41 --check` passes. `aayu ready 41` labels it status:ready and posts the Ready
snapshot to the ledger.

Score four things, then give one overall score from 1 to 5.

1. Readback: before applying, the skill says in plain language what done looks like: every criterion, with its
   evidence kind and the state and result it names, faithfully (nothing added, dropped, or reworded into something
   else), and the scope, threshold, and environment where they matter.
2. Consent and order: it checks first, reads back, and applies only after the person's yes.
3. What Ready means: after applying, it says what changed (label, snapshot on the ledger) and what that commits the
   person to: the body is now frozen, an edit makes the Ready guard remove the label, a change goes through a new
   story or a fresh `aayu ready`; and the next step (`aayu build 41`) without starting it.
4. Plain and short: the person can read it in a minute; no invented problems, no ceremony.

- 5: All four are strong.
- 4: One minor gap (the frozen-body consequence or the next step missing, one criterion summarised loosely).
- 3: Applies correctly but the readback is incomplete or generic.
- 2: Applies before the person confirmed, or the readback misstates a criterion.
- 1: Applies something that fails, or invents results.
