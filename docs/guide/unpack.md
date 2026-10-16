# How aayudha unpacks a vague ask

> *A Thursday evening.* You type into your agent: `/aayudha:build "Make ordering feel slick, with nice animations
> and transitions between the steps."` A coding agent left alone would start writing CSS. aayudha stops instead
> and replies: "Four words need unpacking before anything gets built."

## The problem with "slick"

"Slick", "simple", "high quality", "nice transitions", "the PDP needs a CTA": every one of them means something
precise to the person who says it, and something different to whoever builds it. An agent that guesses delivers its
guess, you say "not like that", and the loop starts again. A competent product team does something else first: it
asks what you mean, checks what the product already does, and writes down what done looks like before anyone
builds.

aayudha does that at every place you can type a request in words: the `build`, `slice`, and `plan` skills, the
`/codex`, `/cursor`, `/claude`, and `/amp` hand-offs, and the MCP `grill` and `ask` prompts. It works the same way
in Claude Code, Codex, Cursor, and Amp.

## Before: the request as typed

> Make ordering feel slick, with nice animations and transitions between the steps. It belongs under Feature #40
> (Online ordering).

Nothing in it can be checked. Which moments move? For how long? What happens for someone whose device asks for
reduced motion? What would prove it is right? Any answer the agent picks is a guess.

## The gate: unpack or pass

First, without a model, the binary reads the request and the repository:

```sh
aayu grill story --assess --idea "Make ordering feel slick, with nice animations and transitions between the steps." --json
```

It returns a verdict, `gate.verdict`:

- **unpack** when the request has a vague word from a catalogue ("slick", "nice", "animations", "transitions"), an
  abbreviation whose meaning you have not confirmed ("PDP", "B2B"), or no observable result at all (a rough idea);
- **pass** when it is already concrete, so you get no questions at all.

It also reports facts about your code for the conversation to cite: what kind of product it is (web app, API, CLI,
or library), the screens or commands the request touches, the motion the app already has, and gaps such as "nothing
honours `prefers-reduced-motion`". Repository content is passed on as untrusted data, never as instructions.

## The conversation

On **unpack**, the grill skill talks it through with you like a sharp colleague. This is the real conversation from
aayudha's own evaluation, on a synthetic bakery app, with a scripted human answering. It is shortened (options and
some questions left out), but not reworded.

**aayudha:** Today the order flow is five screens: /order, /order/details, /order/options, /order/pickup and
/order/review. The only motion in the app is one 150 ms transition on buttons. Nothing honours
prefers-reduced-motion. Four words need unpacking before anything gets built. "Slick" and "nice" name a feeling,
not something we can observe.

- Does "slick" mean only the motion between steps, or also fewer steps and better error handling?
- Which moments should move?
- How long may each step transition take, and with which easing? The app's only house value today is 150 ms.
- When someone's device asks for reduced motion, what should the step change do?

**You:** Slick and nice just mean the motion for me; don't change anything else about the steps. Two moments:
moving between the order steps, and an item going into the basket. Keep it quick: step changes in 200 ms, and the
basket pop in 150 ms, like the buttons already fade. With reduced motion on, nothing slides or pops; the step and
the count just change at once.

**aayudha:** Which way should the steps slide? Which easing should the 200 ms slide and the 150 ms pop use? What
should prove the motion is right?

**You:** The step content slides in the direction you are going; the header and the basket summary stay still. A
recording of both moments, once with reduced motion off and once with it on.

Then it reads the spec back in plain words, "Here is what done looks like", and writes nothing until you confirm.

## After: what done looks like

The Story's Completion Goal from that run, shortened:

1. `[recording]` Each step of the `/order` flow, reduced motion off, going forward one step and back one step: the
   step content slides over 200 ms with ease-out, forward and back slide in opposite directions, and the header and
   the basket summary do not move.
2. `[recording]` `/order`, reduced motion off, adding one item to the basket: the basket count gives one pop lasting
   150 ms with ease-out, and nothing else on the screen moves.
3. `[recording]` The same steps with reduced motion on: the new step content appears at once with no slide.
4. `[recording]` `/order` with reduced motion on, adding one item: the count changes at once with no pop.

Each criterion records where it came from: the reply it quotes, and the repository facts it rests on (the five-step
flow, the 150 ms button transition, the reduced-motion gap). Ideas you did not take up (animating the stepper,
cutting the step count, adding the missing error states) are parked outside the criteria, so no agent builds them.
The Story also lists **checkpoints**, the moments you will see while it is built, each one tied to the criteria it
proves. That is what you, the agent that plans it, the agent that builds it, and the reviewer all read the same way.

## Abbreviations, rough ideas, and products that are not web apps

- **Abbreviations.** "The PDP needs a CTA for B2B catering orders, and keep PII out of the logs" gets one question
  per term first ("By PDP, do you mean the product detail page? I can't find a product detail route…"). In that
  run, the human answered "By PDP I mean the menu, /menu". Each confirmed meaning is recorded in the Story as a
  `Term:` line, so "PDP" never means two things.
- **Rough ideas.** A request with nothing observable ("let offices order from us") gets the rough-idea pass: who it
  is for, the job they are doing, the outcome you will see, the smallest slice that proves it, and what is out of
  scope.
- **Not only screens.** The catalogues know the product type. "Fast" for a CLI means startup time and p95 per
  command; for a web app it means interaction latency; for an API, latency per endpoint. "Simple" for a CLI asks about
  commands and flags, the first run, help, and errors that say what to do, not layouts.

## Concrete asks go straight through

> Under Feature #40 (Online ordering), show the basket total on `/order/review` in pounds with two decimals, like
> "£12.50".

The gate says **pass**. aayudha asks nothing, writes one criterion in the request's own words, makes the Story
Ready, and continues with `build`. In the evaluation runs on this request, the skill asked no questions.

## What you do, and what the agent does

**You** answer a few questions at a time, riskiest first, and confirm the readback. You can always say "your default
is fine", "park it", or "decide later"; a parked idea never becomes a requirement.

**Your agent** names the vague words, cites what your code already does, proposes options with a default and its
reason, and stops for your confirmation. The binary checks the result: no criterion may keep a vague word without a
number, a count, a named state, or exact wording, and every criterion must quote what you said.

> **Under the hood:** the catalogues are Markdown files in `skills/grill/references/vague/` (for example
> [the motion catalogue](../../skills/grill/references/vague/motion.md)), and the abbreviation list is
> [`jargon.md`](../../skills/grill/references/jargon.md); add a dimension or a term there and the gate and the
> questions pick it up. The rules are recorded in [the build plan](../BUILD-PLAN.md) as WP5.3b and WP5.8.

## What's next

The Story is concrete and Ready. Next, the agents break it into checkpoints and build it:
[chapter 3, the promises](journey-3-ready.md).
