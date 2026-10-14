# 9. Explain it

Good work that nobody understands does not get funded, adopted, or trusted. The last step turns everything the
journey recorded into words for a particular reader: your boss, your users, an auditor. And because every claim must
point at proof, you can share it without worrying about what it overstates.

> **You are here:** step 9 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) ·
> [Ready](journey-3-ready.md) · [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · **Explain**

## For one reader: `brief`

`brief` explains one work item to one audience as a short, accessible web page. You choose the register: `eli5`
(explain it simply), `ste` (Simplified Technical English, short and controlled), `engineer`, or `regulatory`. You also choose the purpose:
`explain`, `ideate`, `innovate`, or `close` (wrap it up). The agent drafts it from the issue, the packs, and the
Result. Then a model from a different vendor hunts for any claim the sources do not support, and an accessibility
check runs on the page. One unsupported claim and the brief is refused.

```sh
aayu brief <goal> --register engineer --purpose close
```

> **In the story:** the closing brief for Goal 125, for engineers. The
> [first run](https://github.com/rahulnakmol/agentic-ayudha/actions/runs/38263522012) was refused: the reviewer
> found one claim the sources did not support. It was fixed, and the
> [second run](https://github.com/rahulnakmol/agentic-ayudha/actions/runs/38264236047) passed with 0 findings and
> 0 serious accessibility issues, citing the five stories' proof packs.

That refusal is the feature. A summary for your boss that cannot overstate is worth more than a polished one that
might.

> **Needs setup:** `brief`'s review pass calls a model directly, so it needs a model key where it runs. This
> repository runs it in a CI workflow that holds the key in a protected environment.

## For everyone: `tell`

`tell` writes docs, guides, changelogs, release notes, and launch copy in your brand's voice. It links a work item
only when that item is a Feature whose shipped stories all have authoritative proof, and it refuses the words your
brand avoids. Publishing is for people only; an agent cannot publish.

This site is written that way too, which is why it names stories by number but links only the three Features with
complete proof.

## For the record: `status` and `audit`

- **`status`** rolls up progress, health, risk, proof coverage, open decisions, and cost for each Goal, Initiative,
  and Feature. Use it for a weekly update.
- **`audit`** writes a report that someone else can reproduce: floor results, proof coverage, waivers, decision
  deadlines, autonomy exceptions, and any write that skipped the log. Every solo-mode bypass is listed in it. Hand it
  to an auditor as is.

## What you get back

- A brief per audience, with every claim traced to a source.
- Docs and release notes that cite only proven work.
- Status rollups for the team, and reproducible audit reports for the record.

> **Well done:** you have been through the whole journey: from an idea to a measured, proven, explained release.
> That is a complete delivery loop, and you did it with an AI team that showed its work at every step.

## What's next

- Start your next Goal: back to [step 1](journey-1-shape.md), with the lessons from `retro`.
- Not sure which command fits a new situation? See [When to use what](when-to-use-what.md).
- Ready to hand more to the agents? See [Adopt at your own pace](adopt.md).
