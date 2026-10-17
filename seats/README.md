# seats

One `seats/<seat>/CHARTER.md` for each of the fourteen seats (WP5.1; the thirteenth, the Data & AI Architect, is Decision #283; the fourteenth, the Production Engineer, is Decision #324). Each charter is hand-authored except the region
between the `BEGIN GENERATED: raci` and `END GENERATED: raci` markers, which `go generate ./...` writes from
`internal/verbs/registry.go` and `internal/verbs/raci.go`: the layers the seat owns and its role in every verb. The
full table is [docs/RACI.md](../docs/RACI.md). `aayu seat add|remove|list` pairs a seat with a human; its onboarding
issue carries the charter. Protected path: changes need a human CODEOWNER review. Embedded by the root `embed.go`.

## Personas (WP5.1b)

Each seat's agent twin has a persona: `seats/<seat>/persona.toml` is the single source. It holds the twin's
name (a figure from the Hindu epics or the rishi tradition, chosen by the founder: short, easy to spell, and never a
principal deity; the persona loader refuses the Trimurti and their consorts, and Greek or Roman names), an optional
title shown in place of the seat beside the name ("Atri, the Founder's Sage"), purpose, temperament and voice, values,
signature
habits and questions, how it pushes back and praises, what it never does, how it grounds itself, how it works with
a paired human, and a practice playbook: disciplines from published large-scale software practice, each with its
public sources and its depth at the three team stages. A twin never claims to have worked anywhere, never imitates
real people, and never wears a company's trademark; the persona loader refuses such text.

`go generate ./...` renders the persona into the charter (between `BEGIN/END GENERATED: persona`), into a block at
the top of the SKILL.md body of every verb the seat is Accountable for (between the same markers, after the title),
and into the AGENTS.md seat section. The MCP `persona` prompt renders it at run time. CI fails on drift, and
`TestPersonaEverywhere` checks that every installed skill copy carries the same block on every host.

| Twin | Seat | Voice in brief |
|---|---|---|
| Arjun | Product Strategist | plain customer language; problem, population, metric; kill criteria up front |
| Vishwa | Code Architect | diagrams and trade-off tables; alternatives before a recommendation |
| Shuka | Data & AI Architect | interviews first; grain, window, and population; the simplest option before a model |
| Drona | Engineering Manager | one level up, numbered causes, a labelled anecdote, one nudge with an owner and a date |
| Nakul | AI Native Engineer | terse and concrete; the failing test, the floor, the next checkpoint |
| Shesha | Production Engineer | timestamped lines in an incident; the deployed revision, bounded queries, two hypotheses |
| Hanu | UX Researcher & Evolutionist | the user's words and the task's steps; critique by goal and heuristic |
| Nala | DevOps & DevEx Tinkerer | the command, the run ID, and the minutes saved |
| Yama | Quality Architect | the risk, the test, the floor, the number; a floor miss stated once |
| Sanjay | Mentalist | the reader's register; pre-mortems and steel men |
| Vidhi | Auditor | criterion, condition, cause, effect, recommendation |
| Sauti | Marketer & Documenter | warm, plain, proof linked; no superlatives |
| Kuber | Revenue Architect | metrics with population, window, and interval |
| Atri | Founder (the Founder's Sage) | counsel, never a ruling: the decision framed, three readings with the dominant one, a recommendation and what would change it, and the question the founder answers |

### Team stage

`[team] stage` (`startup`, `scaleup`, `enterprise`) sets how much ceremony each practice carries; unset, it follows
`[team] size` (15 or fewer people: startup; up to 150: scaleup; more: enterprise), and with no size the pod's humans
(paired seats plus the founder), so a solo pod is a startup. The stage changes depth only, never the floors, the
evidence rule, or human-only gates. The same overrun of story #212 (the eval scenario in
`evals/cases/retro/grounding.md`) at two stages:

| | Startup | Enterprise |
|---|---|---|
| Drona, delivery review | **Retro note**, under a page: the footprint was under-read at Ready, four numbered causes with citations, one labelled analogy, and one nudge with an owner and a date | **Retrospective memo**: summary, context, a checkpoint timeline table, contributing causes, what went well, decisions requested, an actions table with owners and dates, and a data appendix |
| Yama, test strategy | **Test plan**: three risks, one test each at the lowest layer, the floor that guards it, and a thirty-minute charter | **Master test plan**: scope, a risk analysis table, test levels by risk, entry and exit criteria, environments and data, roles and schedule, and traceability to the pack |

The recorded answers are `evals/cases/retro/em-*/output/retro.md` and `evals/cases/proof/qa-*/output/plan.md`.

### Evidence rule

Every claim a twin makes about the code, architecture, risk, or progress cites its source in the same paragraph or
list item (a path with a line or symbol, `LAYERS.md`, an ADR or Decision issue, a pack or `ledger/` path, a
`CONSTRAINTS.md` floor, a commit, a CI run, or a metric or Result). Anecdotes and analogies are labelled and sit next
to the evidence they illustrate. `seats.CheckGrounding` checks it deterministically, and with a grounding text it
refuses a citation the grounding does not contain; `seats.CheckStage` checks the stage's artefact and the one-page
bound. The persona eval cases (`evals/cases/retro/`, `inspect/em-review`, `proof/`) run both in tier 1.

### Decision packs

When a human is stuck, `aayu decide <issue or "question"> --pack` (or the decide skill) routes the question to the
owning seat's twin by topic, and that twin works the decision with the human in its own voice and at the team's
stage: the question in one sentence, grounded facts, two or three options, a recommendation with its door, the cost
of waiting, a pre-mortem by Sanjay, the questions that would change the answer, and the one thing to decide. It is
read-only and applies nothing; the human records the outcome with `aayu decide <n> --accept <option>`. Examples:
`evals/cases/decide/*/output/pack.md`.
