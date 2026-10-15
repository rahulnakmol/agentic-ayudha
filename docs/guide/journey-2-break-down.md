# 2. The plan

> *Monday, 07:26 UTC.* The Goal is on GitHub. You read it twice and feel the size of it. This is a whole product.
> Where do you even start? The agent has already drafted the first Feature. Two minutes later there are five stories,
> and three of them carry a warning.

## From one big goal to five small stories

Big pieces of work hide risk. Small ones show it early. So the agent breaks the Goal into three Features, and `slice`
breaks each Feature into stories sized XS to L, each one small enough to build, check, and ship in a day.

| Feature | Stories | Layers |
|---|---|---|
| [Grounded answers API](https://github.com/rahulnakmol/agentic-ayudha/issues/127) | 130 Load the plan documents from a synthetic seed (S) | L07 |
| | 131 Quote the best passage; refuse or refer the rest (M) | L04, L05 |
| | 132 `POST /api/ask` answers within its contract (M) | L02, L03, L06 |
| [Answer analytics](https://github.com/rahulnakmol/agentic-ayudha/issues/128) | 133 Export product events; make the synthetic exports reproducible (M) | L08 |
| [Ask page](https://github.com/rahulnakmol/agentic-ayudha/issues/129) | 134 The Ask page shows answers with their sources (M) | L01, L02, L09 |

Each story names the parts of the app it touches, its **layers**: screens, API, rules, data, and so on, nine in all
([the nine layers](concepts-layers.md)). Between them, these five touch all nine. They also name their order: 131
needs 130, and 132 needs 131. And each Feature says what is out of scope. The answers API rules out "any language
model, real plan data, authentication, and the web page". That one sentence saves an argument later.

Then the agent writes a plan for each story, one checkpoint per layer, and `plan` checks it and keeps it on the
`ledger`, the branch where aayudha keeps its records.

## Three plans nobody else could review

Three stories carry a warning. Story 130 changes stored data (L07). Story 131 changes the rules about what may be
answered (L04). Story 134 changes how the app is run (L09). aayudha treats these layers as too risky to change on
an agent's word alone. Before building starts, a person must review the plan, in the role of the Engineering
Manager [seat](concepts-seats.md).

Here is the snag. You are the only person. The agent drafted and checked every plan, so your approval would be the
only review those plans ever got, and it would be recorded as an Engineering Manager's review, a second pair of eyes
that never looked. Worse, agents act under your account until a separate bot identity exists, so an agent could
record that approval in your name without you giving it.

So you do the honest thing. You post a comment on each of the three stories. It starts "Solo-mode bypass:", names
the risky layer, and says plainly that "no such review is possible" and that the stories proceed under a recorded
bypass, "never an approval". Then you run `plan --solo-bypass`, a command agents are refused, and it writes the same
into each plan on the ledger. Months from now, anyone reading these
stories will know exactly what was and was not reviewed.

> **Honest status:** with two or more people, you will never need this. The real review is required, and the plan
> records who gave it. [The human pass and solo mode](concepts-human-pass.md) explains the rule.

## What you do, and what the agent does

**You** agree the split. Does each story deliver something you could show someone? Is anything missing? Is any
story secretly huge? When a real choice comes up, such as two designs or a vendor, ask for `decide` and accept the
option you want.

**Your agent** drafts the Features and stories, sizes them, finds their layers, writes the plans, and refuses
anything sized XL until it is split again.

## What you learn

Writing down what is out of scope is as useful as writing down what is in. And when a safeguard cannot be met, the
honest move is to record that it was skipped, not to pretend.

## The verbs in this step

<!-- BEGIN GENERATED: stage-verbs -->

| Verb | What it does |
|---|---|
| [`slice`](../../skills/slice/SKILL.md) | Splits a Feature into small stories, each with checkable promises, the parts of the code it touches, and its order. |
| [`plan`](../../skills/plan/SKILL.md) | Plans a story one layer at a time: what changes, how it is tested, and how to roll it back. |
| [`decide`](../../skills/decide/SKILL.md) | Records a real choice with its options and a recommendation; only a person with the authority accepts it. |

<!-- END GENERATED: stage-verbs -->

> **Under the hood:** plans and every bypass record live on the ledger branch; see [the ledger branch](how-ledger.md).
> Who may approve what, and how solo mode is checked, is in [trust and identities](how-trust.md).

## What's next

Five stories, in order. But "load the plan documents" could mean almost anything. Before the agent writes a line,
you must decide what done means: [chapter 3, the promises](journey-3-ready.md).

> **You are here:** chapter 2 of 9. [Goal](journey-1-shape.md) · **Plan** · [Promises](journey-3-ready.md) ·
> [Build](journey-4-build.md) · [Gate](journey-5-prove.md) · [Yes](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Number](journey-8-learn.md) · [Telling](journey-9-explain.md)
