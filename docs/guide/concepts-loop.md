# The delivery loop

A coding agent can write a change in minutes. What it cannot tell you is who agreed what the change should do, who
checked it, who said yes, and whether it helped anyone. aayudha answers those questions by running every piece of
work through the same seven steps, with an agent twin doing the work at each step and a person holding the gates.

An **agent twin** is the agent half of a **seat**, a role on the team such as Product Strategist
([seats and pairing](concepts-seats.md)). A **gate** is a check a step must pass before the next one starts.

![Seven steps run top to bottom, Shape to Learn. Named agent twins do the work at each step and you answer, decide, or approve. Gates sit between the steps, with the merge gate and then the human pass before Ship, the floors sit inside Prove, Prove sends work back to Build, and Learn starts the next goal.](media/delivery-loop.svg)

*Agents do the work. People hold the gates. Floors hold for everyone.*

## Who does what at each step

| Step | The agent twins | You |
|---|---|---|
| **1. Shape**: [`grill`](skills/grill.md), [`survey`](skills/survey.md) | Arjun interviews you and files a Goal with a baseline and a target; on inherited code, Vishwa surveys it, cites what it does, and drafts a backlog to modernise it | Answer, confirm the readback, choose the target |
| **2. Ready**: [`slice`](skills/slice.md), [`plan`](skills/plan.md), [`chart`](skills/chart.md), [`decide`](skills/decide.md), [`ready`](skills/ready.md) | Arjun splits the work into stories with numbered promises; Vishwa plans each layer; Shuka charts data and AI changes; on a hard choice, the owning seat's twin writes a decision pack | Accept Decisions, review risky plans, say Ready |
| **3. Build**: [`build`](skills/build.md), and for a screen [`wireframe`](skills/wireframe.md) to [`polish`](skills/polish.md) | Nakul builds one checkpoint per layer and checks each; Hanu designs the screens | Ask for the story |
| **4. Prove**: [`verify`](skills/verify.md), [`inspect`](skills/inspect.md), [`proof`](skills/proof.md) | Nakul runs the floors that Yama owns; Drona's review comes from another vendor's agent app | Read the verdict |
| **5. Ship**: [`ship`](skills/ship.md) | Drona checks the evidence again and writes the record | Approve, or record a solo-mode bypass |
| **6. Release**: [`release`](skills/release.md) | Nala promotes behind the full gate; Shesha's error budget can hold it | Ask for a release; request a stable one |
| **7. Learn**: [`measure`](skills/measure.md), [`readout`](skills/readout.md), [`retro`](skills/retro.md), [`evolve`](skills/evolve.md), [`brief`](skills/brief.md), [`tell`](skills/tell.md) | Shuka measures the Goal; Kuber reads out experiments; Drona runs the retro; Nakul evolves skills; Sanjay and Sauti explain | Accept Feedback, review skill changes, publish |

Each verb has one Accountable seat, and that seat's twin does the work. Every twin is named on its skill page.

## Gates and floors

The gates are where work can stop:

- **Baseline:** no target without today's number, measured.
- **Ready snapshot:** the promises are frozen. An edit afterwards drops Ready, so nothing changes quietly.
- **Budget:** a build stops at its budget (by default 8 hours, 25 USD, and 3 failed attempts) and asks a person.
- **Merge gate:** CI re-runs the checks on the exact commit and keeps the authoritative **proof pack**: the record
  that maps each promise to its evidence ([proof packs](concepts-proof.md)).
- **Human pass:** a person's approval of the pull request. A layer that has earned autonomy, a long run of merges
  with no correction, passes without one, except on the risky layers. Working alone, you record a solo-mode bypass
  instead ([the human pass](concepts-human-pass.md)).
- **Stable-release request:** `main` waits for a person's request, never an agent's: their yes at a terminal, or
  their comment on the Release issue.
- **Promotion gate:** a bigger suite at each rung of `dev`, `rolling`, and `main` ([dev, rolling, main](concepts-ladder.md)).

A **floor** is a gate with a fixed number: 90 % of changed lines covered, no high or critical security or dependency
finding, no secrets, and WCAG 2.2 AA for screens ([floors](concepts-floors.md)). A model's verdict can send work
back, but it can never overrule a floor. A miss passes only with a waiver, a Decision a person accepts.

## How the loops run

The seven steps are one loop. Inside it run three faster ones, and around it run loops on a schedule; the picture of
both is in [keep things moving with loops](guide-loops.md).

- **Checkpoint loop:** the builder commits one layer at a time, and `verify` checks each before the next.
- **Review loop:** when `inspect` says `fix`, the builder fixes the same pull request and the gate runs again. After
  two rounds, aayudha stops spending and asks a person.
- **Story loop:** a Ready story becomes a merged pull request with its proof pack and ship record.
- **Goal loop:** stories are released, the Goal is measured the same way as its baseline, and the retro and the next
  Goal start from the result.

The scheduled loops are small files in `loops/`: triage each weekday morning, the retro each Monday, skill evolution
each month, and more. A loop that needs an agent is run by a seat's twin in your agent's own scheduler, as you. A loop
that needs no model, such as the founder digest or the nightly behaviour sweep, runs in GitHub Actions with no twin.

## Where production comes in

Production feeds the loop too. [`triage`](skills/triage.md) labels incoming reports as Feedback, Bug, or Incident.
For an Incident, Shesha, the Production Engineer's twin, runs [`investigate`](skills/investigate.md) from your
telemetry to the deployed code, and a person approves every production action through [`respond`](skills/respond.md).
The fix comes back as a Bug, which goes through Ready like any story.

## The rule

Every change takes the same path. Agent twins do the work, a person passes every gate that only a person may pass,
and no one, person or model, skips a floor without a recorded waiver. Any time, `aayu halt` stops every agent, loop,
and workflow in the repository until `aayu resume`.

Terms are defined in the [glossary](../../GLOSSARY.md). Every verb has [its own page](skills/README.md).
> **Under the hood:** how the merge gate, its sandbox, and the fix cycle work is in [the merge gate](how-merge-gate.md),
> and who may approve what is in [trust and identities](how-trust.md).

## What's next

See the loop on a real day: [the story](journey.md). Or see who the twins are: [seats and pairing](concepts-seats.md).
