# 6. Ship it

This is your moment. The checks are green, the verdict is in, and a person gives the final yes. `ship` merges the
pull request and writes down exactly what happened, so anyone can see later who approved what, and on what evidence.

> **You are here:** step 6 of 9. [Shape](journey-1-shape.md) · [Break down](journey-2-break-down.md) ·
> [Ready](journey-3-ready.md) · [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · **Ship** ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## What you do, and what the agent does

**You** approve the pull request on GitHub, then run `ship`. That approval is the **human pass**: a review from a
person's account, never a bot's. `ship` runs only under your identity, so an agent cannot ship on its own.

**`ship`** checks that everything lines up before it merges: the merge gate is green on the head commit, an
authoritative proof pack exists for it, the human pass is there, and, for risky layers, the plan was reviewed. Then
it squash-merges into your integration branch (`dev`, or `main` if that is all you have), writes the story's
Outcome section (the reference calls it the Complete section), releases the agent's lease, and cleans up the branch.

```sh
aayu ship <pr>
```

If any check is missing, `ship` refuses and tells you which one.

## In the benefits-qa story

> **In the story:** pull request 148 was merged at 09:52 UTC, 40 minutes after it was opened.

`ship` then wrote the Outcome of story 131. Here it is, slightly shortened:

- Merged in pull request 148 as commit `f56dc67` on 2026-10-12.
- Authoritative pack: on the ledger, written by the merge gate's run in CI.
- Human pass: none possible. The founder merged under a recorded solo-mode bypass.
- Plan-time review: none. The founder recorded a solo-mode plan-review bypass for layer L04.
- Ship record and the log of every write it made, both on the ledger.

## Solo mode, said plainly

GitHub does not let you approve your own pull request, so a team of one cannot give a real human pass. Instead of
faking one, aayudha records a **solo-mode bypass**, which `audit` lists. With two or more people you will never need
it. See [the human pass and solo mode](concepts-human-pass.md).

> **Tip:** run `aayu status <goal>` after shipping. It rolls up progress and proof coverage for the whole Goal. For
> benefits-qa it reached 5 of 5 stories shipped with authoritative proof.

## What you get back

- A squash-merged commit on `dev`.
- The story's Outcome section: the commit, the pack, who approved, and any bypass, with links.
- A ship record on the ledger.

> **Well done:** you just shipped your first proven story. Every claim about it can be checked by anyone, today or a
> year from now.

## What's next

Merged is not released. [Release it](journey-7-release.md) to get it into people's hands.
