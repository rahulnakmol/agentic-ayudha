# 9. The telling

> *Monday, 14:31 UTC.* You ask for a one-page close-out of the Goal for the engineers. Four minutes later the
> run fails. The reviewer, a model from another vendor, has found one sentence the sources do not support. The brief
> is refused.

## A summary that is not allowed to overstate

Good work that nobody understands does not get funded, adopted, or trusted. `brief` turns the record of the work
into a short web page for one audience. You choose the register: `eli5` (explain it simply), `ste` (Simplified
Technical English), `engineer`, or `regulatory`. And the purpose: `explain`, `ideate`, `innovate`, or `close`.

The agent drafts it from the issue, the packs, and the Result. Then two things check it. A model from a different
vendor hunts for any claim the sources do not support, and an accessibility check runs on the page. One unsupported
claim, and the brief is refused.

## Refused, then accepted

That is what happened at 14:31. The
[first run](https://github.com/rahulnakmol/agentic-ayudha/actions/runs/38263522012) of
`aayu brief 125 --register engineer --purpose close` found one sentence the sources did not back up, and refused
the brief. The sentence was fixed, and the
[second run](https://github.com/rahulnakmol/agentic-ayudha/actions/runs/38264236047) passed: 0 unsupported claims,
0 serious accessibility problems, and a footer citing the proof packs of all five stories.

That refusal is the feature. A summary for your boss that cannot overstate is worth more than a polished one that
might. And because it cites five proof packs, anyone who doubts a sentence can follow it to the evidence.

## For everyone: tell

`tell` writes docs, guides, changelogs, release notes, and launch copy in your brand's voice. It links a piece of
work only when it is a Feature whose shipped stories all have authoritative proof, and it refuses the words your
brand avoids. Publishing is for people only.

This site is written that way, which is why the story names stories and pull requests by number but links only the
three Features.

## For the record: status and audit

- **`status`** rolls up progress, health, risk, proof, open decisions, and cost per Goal. For Goal 125: 5 of 5
  stories shipped with authoritative proof.
- **`audit`** writes a report someone else can reproduce: floor results, proof coverage, waivers, decision deadlines,
  every solo-mode bypass, and any write that skipped the log. It is what you hand an auditor.

> **Honest status:** the latest audit report on the ledger covers aayudha's own build up to 06:35 on 12 October, before this
> story began. The five solo-mode ships and three plan bypasses from today wait for the next `audit` run, which will
> list each one.

## What you do, and what the agent does

**You** pick the audience and the purpose, read the brief, and share it.

**Your agent** drafts it from the record. The checks refuse anything the record does not support.

## What you learn

Claims that point at evidence travel further than claims that ask for trust.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`status`](skills/status.md) | Rolls up progress, risk, proof, open decisions, and cost for each Goal, Initiative, and Feature. |
| [`audit`](skills/audit.md) | Writes a report anyone can reproduce: checks, proof, waivers, deadlines, bypasses, and any write that skipped the log. |
| [`brief`](skills/brief.md) | Explains a piece of work to one audience in a short page, where every claim is checked against its sources. |
| [`tell`](skills/tell.md) | Writes docs, release notes, and launch copy in your brand's voice, linking only work that is proven. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** `brief`'s reviewer is one of the few places the binary calls a model directly; see
> [the model gateway](how-models.md). How `audit` checks the log against GitHub's own history is in
> [configuration, secrets, and the audit trail](how-config-and-audit.md).

## The end of the day

Just after seven you had a fuzzy wish and a count from an inbox. By 14:47 there was a measured baseline, five
proven stories across all nine layers, a seeded result of 0.917 against a target of 0.60, a close-out brief backed
by five proof packs, a release, and a date in November to check it all again with real people. Along the way the
checks said no again and again, and every no left the product a little more correct and a little more private.

> **Well done:** you followed one idea from a wish to a measured, proven, explained release. That is the whole loop.

## What's next

- See the same loop run on aayudha itself, with its own gates saying no to it: the
  [epilogue, built with itself](built-with-itself.md).
- Want the mechanics? Start [How it works](how-it-works.md).
- Ready to try it? Spend [your first hour](start.md).

> **You are here:** chapter 9 of 9. [Goal](journey-1-shape.md) · [Plan](journey-2-break-down.md) ·
> [Promises](journey-3-ready.md) · [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · **Telling**
