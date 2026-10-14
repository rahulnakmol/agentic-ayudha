# 3. Get a story Ready

This is the step that makes everything after it work. Before any code is written, you and the agent agree, in
writing, what "done" means for one story. Later, every check and every review measures the work against exactly
these words, and nothing else.

> **You are here:** step 3 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) · **Ready** ·
> [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## What you do, and what the agent does

**You** read the story's promises and ask: if all of these are true, am I happy? If yes, you say Ready.

**Your agent** uses `grill story` to write the story with you. Its heart is the **Completion Goal**: a numbered list
of criteria, each one observable. Every criterion names the evidence that will prove it (a screenshot, a recording,
a screen-reader transcript, an API trace, or a metric), the situation to test, and the expected result. Then `ready`
checks the story is complete and posts a **Ready snapshot**: a comment that fingerprints the story's text, so any
later edit is caught.

```sh
aayu ready <story> --check    # read-only: lists anything missing
aayu ready <story>            # labels it status:ready and posts the snapshot
```

## In the benefits-qa story

> **In the story:** story 131, "Answers quote the best plan passage; policy refuses or refers the rest". As an
> Example Co employee, I want my question answered by quoting the plan passage that covers it, so that I can trust
> the answer, and I want questions the plan does not answer sent to HR rather than guessed.

Its Completion Goal had three criteria:

1. **API trace.** Ask "What is the dental annual deductible?". Expected: the answer quotes the dental-deductible
   passage in full, with plan Dental and section Deductible, and the policy marks it grounded.
2. **API trace.** Ask "What is my colleague's claim status?" and "Should I take ibuprofen for my back?". Expected:
   each is refused before any search, as personal data or as advice, with a message that says why.
3. **API trace.** Ask "Can I bring my dog to work?". Expected: it is referred to an HR specialist as not in the plan
   documents, and nothing is quoted.

Notice what these do. They describe behaviour a person could check, not code. They include the questions the app
must *not* answer. And they are specific enough that a reviewer can say "met" or "not met" without guessing.

The Ready snapshot recorded two fingerprints: one for the whole story and one for the Completion Goal alone. When
the merge gate later checked the proof, it compared the Completion Goal against that second fingerprint.

## Why the snapshot matters

Without it, "done" quietly moves. Someone edits a criterion halfway through, and the work is judged against
something nobody agreed to. With the snapshot, `build` refuses a story whose text changed after Ready. If the story
truly needs to change, you change it and run `ready` again, and the new version is the one everyone sees.

> **Tip:** write at least one criterion about what must *not* happen. In story 131, the refusals were the criteria
> most likely to catch a real bug.

## What you get back

- A story labelled `status:ready`, with numbered criteria and the kind of evidence each one needs.
- A Ready snapshot comment, with the fingerprints the rest of the journey checks against.

> **Well done:** your agent now has a precise, testable brief. Most bugs start as vague requirements, and you just
> removed the vagueness.

## What's next

The story is Ready, so the agent can [build it](journey-4-build.md).
