# 3. The promises

> *Monday, 07:29 UTC.* "So what does done look like for the answers story?" the agent asks. You start listing what
> the app should answer. The agent asks about the opposite. "What must it never answer? Write that down too."

## Done, written down before the code

This is the step that makes everything after it work. Before any code exists, you and the agent agree, in
writing, what "done" means for each story. Every later check and every review measures the work against exactly
these words and nothing else.

`grill story` writes the story with them. Its heart is the **Completion Goal**: numbered promises, each one
something a person could check, each naming the evidence that will prove it. For story 131, "answers quote the best
plan passage; policy refuses or refers the rest", there were three:

1. **API trace.** Ask "What is the dental annual deductible?". Expected: the answer quotes the dental-deductible
   passage in full, with plan Dental and section Deductible, and the policy marks it grounded.
2. **API trace.** Ask "What is my colleague's claim status?" and "Should I take ibuprofen for my back?". Expected:
   each is refused before any search, as personal data or as advice, with a message that says why.
3. **API trace.** Ask "Can I bring my dog to work?". Expected: it is referred to an HR specialist as not in the plan
   documents, and nothing is quoted.

Two of the three promises are about what the app must not do. That is the privacy stake from the prologue, turned
into something a machine can check.

Story 130, loading the plan documents, got a promise that looked routine: a seed file with "an email address outside
example.com" must be refused, so that no real person's address can ever end up in a screenshot. Nobody expected that
line to be the hardest promise of the day.

## Freezing the agreement

At 07:29, `ready` checked that story 131 was complete and posted a **Ready snapshot**: a comment with two fingerprints,
one of the whole story and one of the Completion Goal alone. From now on, if anyone edits the story, the fingerprint
no longer matches, the story loses its Ready label, and `build` refuses to start. "Done" cannot quietly move.

## What you do, and what the agent does

**You** read the promises and ask: if all of these are true, am I happy? If yes, you say Ready.

**Your agent** drafts the promises with you, makes each one observable, names the evidence it needs, and runs
`ready` to check and freeze the story.

## What you learn

The promises about what must not happen catch the most bugs, as chapter 5 will show. Write at least one for every
story.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`ready`](../../skills/ready/SKILL.md) | Checks that a work item is complete and freezes exactly what was agreed, so later edits are visible. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** the snapshot and its two hashes are written to the ledger (see [the ledger branch](how-ledger.md)),
> and every proof pack later carries the Completion Goal hash it was checked against (see
> [proof packs](how-proof-packs.md)).

## What's next

Five stories are Ready. You open a terminal, type one command, and go to get coffee:
[chapter 4, the build](journey-4-build.md).

> **You are here:** chapter 3 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) · **Promises** ·
> [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
