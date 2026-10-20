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
| [Set up](../start.md) | [`init`](init.md) | Sets up a repository for aayudha: labels, issue forms, config, and the records branch; more on request. | ![Atri](../../../design/icons/seats/founder.svg) Founder (Atri) |
|  | [`install`](install.md) | Installs the aayudha skills, tools, and hooks into your agent, and sets up its cloud environment and loops. | ![Nala](../../../design/icons/seats/devops-devex-tinkerer.svg) DevOps & DevEx Tinkerer (Nala) |
|  | [`doctor`](doctor.md) | Checks your machine and setup, and says exactly what to fix. | ![Nala](../../../design/icons/seats/devops-devex-tinkerer.svg) DevOps & DevEx Tinkerer (Nala) |
| [Shape the goal](../journey-1-shape.md) | ![grill](../../../design/icons/verbs/grill.svg) [`grill`](grill.md) | Interviews you, one question at a time, until an idea is a complete work item, then files it on GitHub. | ![Arjun](../../../design/icons/seats/product-strategist.svg) Product Strategist (Arjun) |
|  | [`survey`](survey.md) | Reads a codebase nobody documented, says what it does with a citation for every claim, and drafts the backlog to modernise it. | ![Vishwa](../../../design/icons/seats/code-architect.svg) Code Architect (Vishwa) |
| [Break it down](../journey-2-break-down.md) | ![slice](../../../design/icons/verbs/slice.svg) [`slice`](slice.md) | Splits a Feature into small stories, each with checkable promises, the parts of the code it touches, and its order. | ![Arjun](../../../design/icons/seats/product-strategist.svg) Product Strategist (Arjun) |
|  | ![plan](../../../design/icons/verbs/plan.svg) [`plan`](plan.md) | Plans a story one layer at a time: what changes, how it is tested, and how to roll it back. | ![Vishwa](../../../design/icons/seats/code-architect.svg) Code Architect (Vishwa) |
|  | ![decide](../../../design/icons/verbs/decide.svg) [`decide`](decide.md) | Records a real choice with its options and a recommendation; only a person with the authority accepts it. | ![Atri](../../../design/icons/seats/founder.svg) Founder (Atri) |
|  | [`chart`](chart.md) | Before a data or AI change is planned, works out with you which decisions it serves, which data or evidence answers them, and how to prove it right. | ![Shuka](../../../design/icons/seats/data-ai-architect.svg) Data & AI Architect (Shuka) |
| [Get a story Ready](../journey-3-ready.md) | ![ready](../../../design/icons/verbs/ready.svg) [`ready`](ready.md) | Checks that a work item is complete and freezes exactly what was agreed, so later edits are visible. | ![Arjun](../../../design/icons/seats/product-strategist.svg) Product Strategist (Arjun) |
| [Design the screens](../journey-4-build.md#if-your-story-has-a-screen) | [`research`](research.md) | Collects design references and user research for a Feature. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
|  | [`wireframe`](wireframe.md) | Sketches the screens a Feature needs, from its research and promises. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
|  | [`mockup`](mockup.md) | Turns wireframes into mockups that use your design system. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
|  | [`prototype`](prototype.md) | Builds a working prototype behind a flag and compares it with the design. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
|  | [`review`](review.md) | Critiques a pull request's screens with a model and with fixed accessibility checks. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
|  | [`polish`](polish.md) | Applies a design critique to a pull request and reviews it again. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
|  | [`baseline`](baseline.md) | Captures reference screenshots of a commit to compare later changes against. | ![Hanu](../../../design/icons/seats/ux-researcher-evolutionist.svg) UX Researcher & Evolutionist (Hanu) |
| [Build it](../journey-4-build.md) | ![build](../../../design/icons/verbs/build.svg) [`build`](build.md) | Writes the code for a Ready story in small checked steps, one layer at a time, and opens a pull request. | ![Nakul](../../../design/icons/seats/ai-native-engineer.svg) AI Native Engineer (Nakul) |
| [Prove it](../journey-5-prove.md) | ![verify](../../../design/icons/verbs/verify.svg) [`verify`](verify.md) | Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results. | ![Nakul](../../../design/icons/seats/ai-native-engineer.svg) AI Native Engineer (Nakul) |
|  | ![inspect](../../../design/icons/verbs/inspect.svg) [`inspect`](inspect.md) | A model from another vendor, or an agent app you choose, reviews the pull request against every promise; aayu reads its reply as merge, fix, or rebuild. | ![Drona](../../../design/icons/seats/engineering-manager.svg) Engineering Manager (Drona) |
|  | ![proof](../../../design/icons/verbs/proof.svg) [`proof`](proof.md) | Stores, fetches, and checks the proof packs that hold the evidence for each change. | ![Yama](../../../design/icons/seats/quality-architect.svg) Quality Architect (Yama) |
| [Ship it](../journey-6-ship.md) | ![ship](../../../design/icons/verbs/ship.svg) [`ship`](ship.md) | Merges a proven pull request after your yes, and writes down who approved what, on which evidence. | ![Drona](../../../design/icons/seats/engineering-manager.svg) Engineering Manager (Drona) |
| [Release it](../journey-7-release.md) | ![release](../../../design/icons/verbs/release.svg) [`release`](release.md) | Moves shipped work up to a snapshot or stable release, after a full gate, and schedules the measurement. | ![Nala](../../../design/icons/seats/devops-devex-tinkerer.svg) DevOps & DevEx Tinkerer (Nala) |
| [Measure and learn](../journey-8-learn.md) | [`experiment`](experiment.md) | Designs an A/B test for a Feature: what you expect, what you measure, and how many people you need. | ![Kuber](../../../design/icons/seats/revenue-architect.svg) Revenue Architect (Kuber) |
|  | [`readout`](readout.md) | Reads out an experiment once it has enough people, and writes down the measured lift. | ![Kuber](../../../design/icons/seats/revenue-architect.svg) Revenue Architect (Kuber) |
|  | [`retro`](retro.md) | Compares plans with what happened and files what to improve as feedback you can accept or reject. | ![Drona](../../../design/icons/seats/engineering-manager.svg) Engineering Manager (Drona) |
|  | ![measure](../../../design/icons/verbs/measure.svg) [`measure`](measure.md) | Measures a Goal against its baseline, the same way as before, and writes the result into the Goal. | ![Shuka](../../../design/icons/seats/data-ai-architect.svg) Data & AI Architect (Shuka) |
|  | [`evolve`](evolve.md) | Turns accepted feedback into a test for a skill, then a skill change that passes it, for a person to review. | ![Nakul](../../../design/icons/seats/ai-native-engineer.svg) AI Native Engineer (Nakul) |
| [Explain it](../journey-9-explain.md) | ![counsel](../../../design/icons/verbs/counsel.svg) [`counsel`](counsel.md) | Asks the founder's sage for a steer on anything, with the evidence behind it and the next step; it advises and never approves or decides. | ![Atri](../../../design/icons/seats/founder.svg) Founder (Atri) |
|  | [`status`](status.md) | Rolls up progress, risk, proof, open decisions, and cost for each Goal, Initiative, and Feature. | ![Drona](../../../design/icons/seats/engineering-manager.svg) Engineering Manager (Drona) |
|  | [`audit`](audit.md) | Writes a report anyone can reproduce: checks, proof, waivers, deadlines, bypasses, and any write that skipped the log. | ![Vidhi](../../../design/icons/seats/auditor.svg) Auditor (Vidhi) |
|  | [`brief`](brief.md) | Explains a piece of work to one audience in a short page, where every claim is checked against its sources. | ![Sanjay](../../../design/icons/seats/mentalist.svg) Mentalist (Sanjay) |
|  | ![tell](../../../design/icons/verbs/tell.svg) [`tell`](tell.md) | Writes docs, release notes, and launch copy in your brand's voice, linking only work that is proven. | ![Sauti](../../../design/icons/seats/marketer-documenter.svg) Marketer & Documenter (Sauti) |
| [Keep it running](../when-to-use-what.md#something-broke-in-production) | [`triage`](triage.md) | Sorts incoming reports into feedback, bugs, and incidents by label, without editing what people wrote. | ![Arjun](../../../design/icons/seats/product-strategist.svg) Product Strategist (Arjun) |
|  | [`respond`](respond.md) | Helps handle an incident: proposes a fix from your runbook, runs it only after a person approves, and keeps the timeline. | ![Shesha](../../../design/icons/seats/production-engineer.svg) Production Engineer (Shesha) |
|  | [`investigate`](investigate.md) | Works out what went wrong in production from your telemetry and the code that was actually deployed, and suggests a fix for a person to approve. | ![Shesha](../../../design/icons/seats/production-engineer.svg) Production Engineer (Shesha) |
|  | [`slo`](slo.md) | Sets how reliable the product must be for its users, measures how much room for failure is left, and holds back risky releases when it runs out. | ![Shesha](../../../design/icons/seats/production-engineer.svg) Production Engineer (Shesha) |
|  | [`runbook`](runbook.md) | Writes down how to spot and fix each known production problem with the approved actions, so whoever is on call can follow it. | ![Shesha](../../../design/icons/seats/production-engineer.svg) Production Engineer (Shesha) |

<!-- END GENERATED: skill-index -->

> **Under the hood:** the tables on these pages (the seat and twin, the inputs, the gates, and the person-only steps)
> are generated from the verb registry, `internal/verbs/registry.go`, by `go generate`. A verb added to the registry
> gets a page, and CI fails when a page drifts from it. See [the verb registry and code generation](../how-registry.md).

## What's next

Not sure where you are? Start from your situation in [when to use what](../when-to-use-what.md).
