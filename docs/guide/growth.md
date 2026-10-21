# From MVP to scale, with a lean team

Most products start with one or two people and outgrow their process long before they outgrow their code. The usual
fixes are more meetings, more handovers, and more people in the loop. With aayudha, the agent twins carry the work at
every stage, so the people you add go where a person is needed, and the rules that keep the work honest stay the same.

> **Illustrative scenario:** this page follows an imagined product from a two-founder MVP to a widely used,
> commercial product. It is not a customer story, and it reports no results. The commands, settings, team stages, and
> the artefacts each twin writes are real; the company is not.

![An illustrative path in three stages. Startup: two founders, one repository, one-page artefacts. Scaleup: a lean team, a second product stream, people paired with the risky-layer seats, agents earning autonomy on safe layers. Enterprise: several product streams and full artefacts. The floors, the evidence rule, and the human-only steps never change.](media/growth-path.svg)

*The people and the ceremony grow. The floors do not.*

## What the team stage changes, and what it never changes

aayudha reads your **team stage** from `[team] stage` in your config: `startup`, `scaleup`, or `enterprise`. Unset,
it follows `[team] size`: 15 people or fewer is a startup, up to 150 is a scaleup, and more is an enterprise. With no
size, it counts the people paired with seats plus the founder, so a pod of one or two people is a startup.

The stage sets how deep each twin's work goes: a startup gets the one-page version, an enterprise the full artefact.
The same practice, at three depths, from the twins' own playbooks:

| Twin, practice | Startup, then scaleup, then enterprise |
|---|---|
| Arjun, working backwards | Press note → PR/FAQ → Working-backwards document |
| Vishwa, design docs | Decision note → Design doc → RFC |
| Yama, test strategy | Test plan → Test strategy → Master test plan |
| Drona, delivery review | Retro note → Delivery review → Retrospective memo |
| Shesha, SLOs and error budgets | SLO line → SLO set → Error budget policy |
| Nala, progressive delivery | Release step → Rollout stages → Progressive delivery policy |
| Vidhi, findings | Finding list → Audit findings → Audit report |
| Kuber, North Star | North Star line → Metric tree → North Star framework |

What the stage never changes: the floors, the evidence rule (every claim cites its source), and the steps only a
person may take. A startup's one-page test plan still has to clear 90 % coverage of the changed lines, and an
enterprise's master test plan cannot talk its way past a floor either.

## Stage 1: two founders and an MVP

Two founders, one repository, one **pod**: the fourteen seats with their twins, working on one product
([your agentic team](concepts-team.md)). The twins do the interviewing, planning, building, checking, and reporting.

The founders split the human side. The first pairs with the Founder seat, which accepts Decisions; the second pairs
with the seat closest to their craft, such as Code Architect. Pair both: `ship` refuses the solo-mode bypass to anyone
while someone else holds a paired seat, so with both founders paired neither can bypass, and each reviews what the
other's twins built. Pairing the Founder seat also means only the first founder can accept a Decision, even when more
people can write to the repository later.

```sh
aayu seat add founder --human <first-founder-login>
aayu seat add code-architect --human <second-founder-login>
aayu seat list
```

**You should see** both seats as `paired`, each with its founder's login. Commit the changed files through a pull
request; once it merges, run the `seat add` commands again, which report `policy: two-human` where the repository has the
protected-paths review ruleset ([when a second person joins](guide-solo-founder.md#when-a-second-person-joins)). Add
`escalate_to = "<second-founder-login>"` under `[decisions]` so a Decision past its deadline pages the other founder.

> **Needs setup:** the page goes out when the `decisions` sweep runs: each weekday morning once
> `aayu install --loops` has installed the scheduled loops, or whenever you run `aayu decide` with no flags.

How the agent team helps here: Arjun turns the founders' ideas into Goals with a baseline, Nakul builds each story
behind the floors, and the inbox keeps the human-only steps in one list ([run it alone](guide-solo-founder.md)). Once
the merge gate is set up, Drona also has every change that is not docs-only reviewed by another vendor's agent app.

> **Needs setup:** the cross-vendor review comes with the merge gate and a reviewer app, at Level 2. Until then, a
> person reviews ([adopt at your own pace](adopt.md#level-2-proof-and-the-merge-gate)).

## Stage 2: a lean team and a second product stream

The product works, and the next one is starting. Each **product stream** gets its own repository, and with it its
own pod: its own fourteen twins, its own paired people, and its own record. aayudha does not share one pod across
repositories; it lets the founders see all of them at once.

![An illustrative pair of founders grows one pod into three product streams. Each stream is its own repository with fourteen seats and twins and its own people. The founders see every repository with aayu status --all, and the floors, the evidence rule, and the human-only steps are the same in every pod.](media/product-streams.svg)

*One repository, one pod, per product stream. One fleet view for the founders.*

List the repositories in your own user config, then roll them up:

```toml
[fleet]
repos = ["your-org/product-a", "your-org/product-b"]
```

```sh
aayu status --all
aayu doctor --all
```

**You should see** one line per repository from `status --all`: its first open Goal, how many of its items are
done, how many are blocked, and how many Decisions are near their deadline. `doctor --all` adds one line per
repository with its access and setup checks.

The team grows too. Once the product team passes 15 people, say so in each repository's config, with
`[team] size` or `stage = "scaleup"` under `[team]`; until then the stage stays `startup` and the twins keep the
one-page versions.

People join, and you pair them where a person's judgement matters most. The risky layers come first: Quality
Architect (domain policy, L04), Production Engineer (stored data, L07), and DevOps & DevEx Tinkerer (the platform,
L09). Those layers never earn autonomy and always wait for a person, so a named owner there removes the biggest
wait.

This is also when earned autonomy starts to matter: a layer that has gone 20 merges in a row without a person's
correction no longer waits for a review. The risky layers never do. A bot App helps here, because the agents then
write as their own identity and each correction is recorded on the ledger ([adopt at your own pace](adopt.md),
Level 3).

> **Needs setup:** autonomy needs the merge gate, because `ship` checks a green merge-gate run and its proof pack
> before it looks at the streak. Install it with `aayu init --gates` ([Level 2](adopt.md#level-2-proof-and-the-merge-gate)).

How the agent team helps here: each twin's artefacts grow with the stage, Kuber runs experiments on the new product's
flags, Shesha keeps an error budget that can hold a release back, and Vidhi audits every bypass and waiver.

## Stage 3: a widely used, commercial product

Several product streams, each its own pod. The product team has grown past 150 people, and each repository says so
with `[team] size` or `[team] stage`, so the twins write the full artefacts: a working-backwards document before a
Feature, a master test plan, a retrospective memo, an audit report.

Each stream keeps a small group of people. They pair with the seats where the company needs a named owner, such as
the Auditor for compliance or the Revenue Architect for pricing, and leave the rest to their twins. A stable release
still waits for a person's request, a production action still waits for a person's approval, and a floor miss still
needs a waiver a person accepts.

Set the stage directly in each repository's `.aayudha/config.toml`:

```toml
[team]
stage = "enterprise"
```

**You should see** `stage = "enterprise"` under `[team]` in `aayu config show`, and the twins' next artefacts at full
depth.

## The rule

Grow the people where a person's judgement is needed, and let the twins carry the rest. Raise the ceremony with the
stage. Never trade away a floor, the evidence rule, or a human-only step to go faster.

Terms are defined in the [glossary](../../GLOSSARY.md).
> **Under the hood:** how the stage is resolved is `Config.TeamStage` in `internal/config/config.go`; each twin's
> artefacts at the three stages are in its `seats/<seat>/persona.toml`, rendered into [its charter](../../seats/README.md).

## What's next

- Meet the fourteen twins: [your agentic team](concepts-team.md).
- See the loop every stream runs: [the delivery loop](concepts-loop.md).
- Start today, alone or with two: [your first hour](start.md).
