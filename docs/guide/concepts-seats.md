# Seats and pairing

A delivery team works because everyone knows their job. aayudha gives its agents the same clarity: fourteen **seats**,
each a role with its own responsibilities, such as Product Strategist or Engineering Manager. Every verb belongs to
exactly one seat, so there is never a question of who is accountable for a step.

![A seat splits into an agent twin, which drafts, runs, and files, and a paired human, who answers, reviews, and approves. Both lead to one complete, checked work item.](img/seats.svg)

*Every seat has an agent twin. A paired human holds the gates.*

## Twins and humans

Each seat has an **agent twin**: the agent half that does the seat's work in your host, such as interviewing,
drafting, building, or checking. A seat runs in one of two modes:

- **Solo:** the twin works alone, and anything that needs a person goes to the founder.
- **Paired:** a person is attached to the seat. They answer the twin's questions, review its work in their layers,
  and give the approvals that only a person may give.

You pair a person with a seat in one command, and you can see every seat's mode at any time:

```sh
aayu seat add product-strategist --human <github-login>
aayu seat list
```

## The fourteen seats

Each twin has an emblem: an object, place, animal, or star tied to its name. The
[branding note](../../design/iconography.md) says what each one shows.

| Emblem | Twin | Seat | In plain words | Some of its verbs |
|---|---|---|---|---|
| ![Atri, Founder's Sage: his star in the Saptarishi, where the handle meets the bowl](../../design/icons/seats/founder.svg) | Atri | Founder | Final authority; sets up repositories and accepts decisions | `init`, `decide` |
| ![Arjun, Product Strategist: an arrow in the centre of the target](../../design/icons/seats/product-strategist.svg) | Arjun | Product Strategist | Turns ideas into clear, checked work | `grill`, `slice`, `ready`, `triage` |
| ![Vishwa, Code Architect: a set-square and a plumb line](../../design/icons/seats/code-architect.svg) | Vishwa | Code Architect | Plans how each story is built, layer by layer | `plan` |
| ![Shuka, Data and AI Architect: a parrot that repeats its source faithfully](../../design/icons/seats/data-ai-architect.svg) | Shuka | Data & AI Architect | Designs the data and evidence behind numbers and AI answers, and measures them | `chart`, `measure` |
| ![Drona, Engineering Manager: a drawn bow, the teacher's test](../../design/icons/seats/engineering-manager.svg) | Drona | Engineering Manager | Reviews, ships, and keeps delivery healthy | `inspect`, `ship`, `status`, `retro` |
| ![Nakul, AI Native Engineer: a horse, for the master of horses](../../design/icons/seats/ai-native-engineer.svg) | Nakul | AI Native Engineer | Builds the software and runs the checks | `build`, `verify`, `evolve` |
| ![Shesha, Production Engineer: an endless coil that holds the world up](../../design/icons/seats/production-engineer.svg) | Shesha | Production Engineer | Runs the software in production: incidents, SLOs, runbooks, rollbacks | `respond` |
| ![Yama, Quality Architect: the gateway every change passes through](../../design/icons/seats/quality-architect.svg) | Yama | Quality Architect | Owns the floors and the proof packs | `proof` |
| ![Hanu, UX Researcher: the leap across the sea to find what people need](../../design/icons/seats/ux-researcher-evolutionist.svg) | Hanu | UX Researcher | Designs and reviews the screens | `research`, `wireframe`, `review`, `polish` |
| ![Nala, DevOps and DevEx Tinkerer: a bridge of floating stones](../../design/icons/seats/devops-devex-tinkerer.svg) | Nala | DevOps Tinkerer | Releases, installs, and keeps the tooling working | `release`, `install`, `doctor` |
| ![Kuber, Revenue Architect: the treasury, rising](../../design/icons/seats/revenue-architect.svg) | Kuber | Revenue Architect | Sets the targets and runs experiments | `experiment`, `readout` |
| ![Sanjay, Mentalist: far sight carried into words](../../design/icons/seats/mentalist.svg) | Sanjay | Mentalist | Explains work to any audience | `brief` |
| ![Sauti, Marketer and Documenter: the storyteller's open book](../../design/icons/seats/marketer-documenter.svg) | Sauti | Marketer and Documenter | Writes docs and launch copy from proven work | `tell` |
| ![Vidhi, Auditor: a palm-leaf record with its seal](../../design/icons/seats/auditor.svg) | Vidhi | Auditor | Checks the record and reports on it | `audit` |

> **In the story:** in benefits-qa, the Product Strategist's twin interviewed the founder for Goal 125, the AI Native
> Engineer's twin built the stories, and the Engineering Manager's twin, running in CI, reviewed each pull request.
> Every seat ran solo, because the repository has one person.

## Why seats help you

- **No gaps.** Every step has an owner, so nothing falls between two agents.
- **Different eyes.** The seat that reviews is not the seat that built, and its model comes from a different vendor.
- **Room to grow.** Start solo. Pair a teammate with the seats they care about when they join, and their reviews
  become required for their layers.

On a small team you do not manage fourteen seats. They all run solo by default; you pair a person only where you want
their review.

Every twin, its layers, and its model lanes are on [your agentic team](concepts-team.md). Full details are in the
[RACI table](../RACI.md). Terms are defined in the [glossary](../../GLOSSARY.md).
> **Under the hood:** seats, verbs, and their RACI are declared once in the verb registry; see
> [the verb registry and code generation](how-registry.md).

## What's next

Seats own parts of the code, called layers. See [the nine layers](concepts-layers.md).
