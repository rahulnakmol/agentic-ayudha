# 2. Break it down

Big pieces of work hide risk. Small ones show it early. In this step, the Goal becomes a few Features, and each
Feature becomes stories small enough to build, check, and ship in a day or less. Then each story gets a plan.

> **You are here:** step 2 of 9. [Shape](journey-1-shape.md) · **Break down** · [Ready](journey-3-ready.md) ·
> [Build](journey-4-build.md) · [Prove](journey-5-prove.md) · [Ship](journey-6-ship.md) ·
> [Release](journey-7-release.md) · [Learn](journey-8-learn.md) · [Explain](journey-9-explain.md)

## What you do, and what the agent does

**You** agree the split. Does each story deliver something you could show someone? Is anything missing? Is any
story secretly huge?

**Your agent** drafts the Features with `grill feature`, then runs `slice` on each one. `slice` checks every
drafted story: its size is XS to L, it has observable criteria, its dependencies are named, and its footprint (the
parts of the code it will touch) is known. A story sized XL is refused and split again. Then the agent writes a
plan per story, one layer at a time, and `plan` checks it.

```sh
aayu slice <feature>                                 # shows the Feature's paths and the layers they touch
aayu slice <feature> --draft slice.json --create     # checks the agent's draft, then files the stories
aayu plan <story> --draft plan.json --create         # checks the plan and keeps it on the ledger
```

You rarely type these. The agent drafts the stories and the plan, and the commands check its draft before anything
is written.

A **layer** is one part of an application, such as the user interface, the API, or the data store. aayudha uses
nine of them, so every change has a clear owner. See [the nine layers](concepts-layers.md).

## In the benefits-qa story

> **In the story:** three Features and five stories, which between them touch all nine layers.

| Feature | Stories | Layers |
|---|---|---|
| [Grounded answers API](https://github.com/rahulnakmol/agentic-ayudha/issues/127) | 130 Load the plan documents from a synthetic seed (S) | L07 |
| | 131 Quote the best passage; refuse or refer the rest (M) | L04, L05 |
| | 132 `POST /api/ask` answers within its contract (M) | L02, L03, L06 |
| [Answer analytics](https://github.com/rahulnakmol/agentic-ayudha/issues/128) | 133 Export product events; make the synthetic exports reproducible (M) | L08 |
| [Ask page](https://github.com/rahulnakmol/agentic-ayudha/issues/129) | 134 The Ask page shows answers with their sources (M) | L01, L02, L09 |

The stories also name their order: 131 depends on 130, and 132 depends on 131. Each Feature says what is out of
scope too. The answers API, for example, rules out "any language model, real plan data, authentication, and the
web page". That sentence saved a lot of arguing later.

Three of the stories touch layers that aayudha treats as too risky to change without a second look: domain policy
(L04), stored data (L07), and the platform (L09). For these, a person reviews the plan before building starts, in
the role of the Engineering Manager [seat](concepts-seats.md). With one person in this repository, nobody else could,
so the founder recorded each skipped review openly instead of calling it an approval
([why that matters](concepts-human-pass.md)).

## When there is a real choice

Sometimes breaking work down surfaces a genuine decision: two designs, a vendor, a trade-off. Ask for `decide`. It
records a Decision issue with the options, how each scores, and a recommendation. Only a person with the authority
for that kind of decision can accept it.

## What you get back

- Features under your Goal, each with a problem, a scope, and what is out of scope.
- Stories sized XS to L, each with its dependencies and the layers it touches.
- A plan per story, kept on the `ledger` branch: a checkpoint per layer, with tests and a rollback.

> **Well done:** the big, scary Goal is now a short list of small, boring stories. Boring is exactly what you want.

## What's next

Before a story is built, everyone has to agree what "done" means. [Get a story Ready](journey-3-ready.md).
