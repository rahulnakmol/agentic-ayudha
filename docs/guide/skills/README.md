# Every skill, one page each

You know what you want to do, but not which verb does it, who answers for it, or what it will refuse. Each page below
covers one verb: what it is for, when to use it, the exact prompt and command, what you get back, its Accountable
seat and agent twin, its gates, and the steps only a person takes.

A **seat** is a role on the delivery team, such as Product Strategist, and its **agent twin** is the agent that does
the seat's work in your host ([seats and pairing](../concepts-seats.md)). A **gate** is a check a verb makes before
it acts. Every verb has exactly one Accountable seat.

You can always ask your agent in plain words instead of typing a command: each page gives a prompt to paste. The
agent runs the verb as a skill, and the same rules apply either way.

## The skills by journey stage

<!-- BEGIN GENERATED: skill-index -->

| Stage | Skill | What it does | Accountable seat (twin) |
|---|---|---|---|
| [Set up](../start.md) | [`init`](init.md) | Sets up a repository for aayudha: labels, issue forms, config, and the records branch; more on request. | Founder (Atri) |
|  | [`install`](install.md) | Installs the aayudha skills, tools, and hooks into your agent, and sets up its cloud environment and loops. | DevOps & DevEx Tinkerer (Nala) |
|  | [`doctor`](doctor.md) | Checks your machine and setup, and says exactly what to fix. | DevOps & DevEx Tinkerer (Nala) |
| [Shape the goal](../journey-1-shape.md) | [`grill`](grill.md) | Interviews you, one question at a time, until an idea is a complete work item, then files it on GitHub. | Product Strategist (Arjun) |
|  | [`survey`](survey.md) | Reads a codebase nobody documented, says what it does with a citation for every claim, and drafts the backlog to modernise it. | Code Architect (Vishwa) |
| [Break it down](../journey-2-break-down.md) | [`slice`](slice.md) | Splits a Feature into small stories, each with checkable promises, the parts of the code it touches, and its order. | Product Strategist (Arjun) |
|  | [`plan`](plan.md) | Plans a story one layer at a time: what changes, how it is tested, and how to roll it back. | Code Architect (Vishwa) |
|  | [`decide`](decide.md) | Records a real choice with its options and a recommendation; only a person with the authority accepts it. | Founder (Atri) |
|  | [`chart`](chart.md) | Before a data or AI change is planned, works out with you which decisions it serves, which data or evidence answers them, and how to prove it right. | Data & AI Architect (Shuka) |
|  | [`scope`](scope.md) | Plans what a release contains, freezes it once a person accepts it, and stops later changes beyond the budget unless a person decides. | Product Strategist (Arjun) |
| [Get a story Ready](../journey-3-ready.md) | [`ready`](ready.md) | Checks that a work item is complete and freezes exactly what was agreed, so later edits are visible. | Product Strategist (Arjun) |
| [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) | [`research`](research.md) | Collects design references and user research for a Feature. | UX Researcher & Evolutionist (Hanu) |
|  | [`wireframe`](wireframe.md) | Sketches the screens a Feature needs, from its research and promises. | UX Researcher & Evolutionist (Hanu) |
|  | [`mockup`](mockup.md) | Turns wireframes into mockups that use your design system. | UX Researcher & Evolutionist (Hanu) |
|  | [`prototype`](prototype.md) | Builds a working prototype behind a flag and compares it with the design. | UX Researcher & Evolutionist (Hanu) |
|  | [`review`](review.md) | Critiques a pull request's screens with a model and with fixed accessibility checks. | UX Researcher & Evolutionist (Hanu) |
|  | [`polish`](polish.md) | Applies a design critique to a pull request and reviews it again. | UX Researcher & Evolutionist (Hanu) |
|  | [`baseline`](baseline.md) | Captures reference screenshots of a commit to compare later changes against. | UX Researcher & Evolutionist (Hanu) |
| [Build it](../journey-4-build.md) | [`build`](build.md) | Writes the code for a Ready story in small checked steps, one layer at a time, and opens a pull request. | AI Native Engineer (Nakul) |
|  | [`mission`](mission.md) | Starts the build of a Feature a person approved, shows where its mission stands, says what can start next, proves each milestone on the dev tip, and lets a person stop it safely. | Engineering Manager (Drona) |
| [Prove it](../journey-5-prove.md) | [`verify`](verify.md) | Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results. | AI Native Engineer (Nakul) |
|  | [`inspect`](inspect.md) | A model from another vendor, or an agent app you choose, reviews the pull request against every promise; aayu reads its reply as merge, fix, or rebuild. | Engineering Manager (Drona) |
|  | [`proof`](proof.md) | Stores, fetches, and checks the proof packs that hold the evidence for each change. | Quality Architect (Yama) |
| [Ship it](../journey-6-ship.md) | [`ship`](ship.md) | Merges a proven pull request after your yes, and writes down who approved what, on which evidence. | Engineering Manager (Drona) |
| [Release it](../journey-7-release.md) | [`release`](release.md) | Moves shipped work up to a snapshot or stable release, after a full gate, and schedules the measurement. | DevOps & DevEx Tinkerer (Nala) |
| [Measure and learn](../journey-8-learn.md) | [`experiment`](experiment.md) | Designs an A/B test for a Feature: what you expect, what you measure, and how many people you need. | Revenue Architect (Kuber) |
|  | [`readout`](readout.md) | Reads out an experiment once it has enough people, and writes down the measured lift. | Revenue Architect (Kuber) |
|  | [`retro`](retro.md) | Compares plans with what happened and files what to improve as feedback you can accept or reject. | Engineering Manager (Drona) |
|  | [`measure`](measure.md) | Measures a Goal against its baseline, the same way as before, and writes the result into the Goal. | Data & AI Architect (Shuka) |
|  | [`evolve`](evolve.md) | Turns accepted feedback into a test for a skill, then a skill change that passes it, for a person to review. | AI Native Engineer (Nakul) |
| [Explain it](../journey-9-explain.md) | [`counsel`](counsel.md) | Asks the founder's sage for a steer on anything, with the evidence behind it and the next step; it advises and never approves or decides. | Founder (Atri) |
|  | [`brain`](brain.md) | Keeps the product's lasting knowledge as linked notes that link to issues and outlive any one person. | Product Strategist (Arjun) |
|  | [`status`](status.md) | Rolls up progress, risk, proof, open decisions, and cost for each Goal, Initiative, and Feature. | Engineering Manager (Drona) |
|  | [`audit`](audit.md) | Writes a report anyone can reproduce: checks, proof, waivers, deadlines, bypasses, and any write that skipped the log. | Auditor (Vidhi) |
|  | [`brief`](brief.md) | Explains a piece of work to one audience in a short page, where every claim is checked against its sources. | Mentalist (Sanjay) |
|  | [`tell`](tell.md) | Writes docs, release notes, and launch copy in your brand's voice, linking only work that is proven. | Marketer & Documenter (Sauti) |
| [Keep it running](../when-to-use-what.md#something-broke-in-production) | [`triage`](triage.md) | Sorts incoming reports into feedback, bugs, and incidents by label, without editing what people wrote. | Product Strategist (Arjun) |
|  | [`respond`](respond.md) | Helps handle an incident: proposes a fix from your runbook, runs it only after a person approves, and keeps the timeline. | Production Engineer (Shesha) |
|  | [`investigate`](investigate.md) | Works out what went wrong in production from your telemetry and the code that was actually deployed, and suggests a fix for a person to approve. | Production Engineer (Shesha) |
|  | [`slo`](slo.md) | Sets how reliable the product must be for its users, measures how much room for failure is left, and holds back risky releases when it runs out. | Production Engineer (Shesha) |
|  | [`runbook`](runbook.md) | Writes down how to spot and fix each known production problem with the approved actions, so whoever is on call can follow it. | Production Engineer (Shesha) |

<!-- END GENERATED: skill-index -->

> **Under the hood:** the tables on these pages (the seat and twin, the inputs, the gates, and the person-only steps)
> are generated from the verb registry, `internal/verbs/registry.go`, by `go generate`. A verb added to the registry
> gets a page, and CI fails when a page drifts from it. See [the verb registry and code generation](../how-registry.md).

## What's next

Not sure where you are? Start from your situation in [when to use what](../when-to-use-what.md).
