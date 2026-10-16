# How leading agent tools write their README and docs, and a plan for aayudha

Research for the founder's request of 2026-10-16, relayed by the orchestrating thread. The request has two phases.
This is phase 1: study how the leading agent tools write their README and docs, learn the patterns without copying
their text, and write a style guide plus a README and docs plan for aayudha. Phase 2 rewrites the README and the
guide to this plan, against the final 0.2 behaviour, after the 0.2 work merges. Owner: the Marketer & Documenter
twin. Status: research and a plan. Nothing in the README or the guide changes in this PR.

Labels: **[V]** checked in source (the file is named), **[M]** measured in this study (the command is named),
**[I]** inferred. All sources were read on 2026-10-16, from the default branch of each repository and from the live
docs sites:

- `can1357/oh-my-pi`: `README.md`, `AGENTS.md`, `docs/`, `.omp/skills/`.
- `badlogic/pi-mono` (now published under the earendil-works name): `packages/coding-agent/README.md`,
  `packages/coding-agent/docs/{index,quickstart,how-pi-works,docs.json}`, `AGENTS.md`.
- `anomalyco/opencode`, formerly `sst/opencode`: `README.md`, `AGENTS.md`, `services/www/src/docs/AGENTS.md`, the
  V1 and V2 docs content, and `opencode.ai/docs`.
- `openai/codex`: `README.md` and `docs/`, plus `developers.openai.com/codex`, its `llms.txt`, and its `.md` pages.
- `anthropics/claude-code`: `README.md` and `CLAUDE.md`, plus `code.claude.com/docs`, its `llms.txt`, and its
  `.md` pages.
- `block/goose`, now upstream at `aaif-goose/goose`: `README.md`, `AGENTS.md`, `documentation/AGENTS.md`,
  `documentation/docs/quickstart.md`, and `documentation/static/llms.txt`.
- `Aider-AI/aider`: `README.md` and `aider/website/docs/`.

aayudha was read at `dev` c802cdd, and the v0.1.0 release was installed for the measurements.

## 1. Summary

- **The best READMEs are doors, not buildings.** Codex (81 lines), Claude Code (72), Goose (64), pi (95), and
  OpenCode (130) say what the tool is, then give one install command, then send you on [V]. oh-my-pi (715 lines) and
  Aider (180) are landing pages that sell first and teach later [V]. aayudha's README (124 lines) does neither. Its
  first screen is internal vocabulary ("twelve seats, nine application layers, and 32 verbs"), a work-package number,
  and a status line that is now false [V].
- **"Installed" is not "first success".** Most READMEs stop at launch. The strong quickstarts name a visible result
  and check it in stages. Claude Code asks the agent about the project, then makes a small change, then uses git.
  Goose ends with a playable game. pi checks `pi --version` before anything can go wrong [V]. aayudha's first hour
  does end in a real result, a checked Goal on GitHub. But it takes 8 typed actions and a pull request [V], and it
  never mentions `aayu tour`, the guided first run the binary already has [V].
- **Agents are now a second readership, served in three layers:**
  1. a scoped `AGENTS.md` (or `CLAUDE.md`) that records what an agent cannot safely infer;
  2. an `llms.txt` index with one line per page;
  3. a clean Markdown twin of every page.

  Codex and Claude Code ship all three [V]. Goose ships an `llms.txt` that contradicts its own README on the licence
  [V], which shows that the agent layer must be generated, not hand-written. aayudha has a strong `AGENTS.md`, but
  no `llms.txt` and no Markdown twins [V].
- **Keep what already works.** No tool studied tells one real project end to end the way aayudha's story does, with
  its setbacks. Only oh-my-pi comes close to aayudha's deep dive with code anchors, and it has no newcomer path
  [V]. The plan keeps both tracks and fixes the doorway in front of them.
- **The plan in one line:** an 80-line README, one first-run path built on `aayu tour` that ends in a checked Goal,
  a situation-led guide layer, a generated `llms.txt` with `.md` twins, and docs-local writing rules that agents
  follow.

## 2. What we studied

| Tool | README | First screen | Actions to first useful result | Agent readers | Docs shape | Tone |
|---|---|---|---|---|---|---|
| oh-my-pi | 715 lines, landing page | Hero image, 8 badges, numeric claims; install at line 40 | 2 if a provider is set; the README never connects the steps | Large `AGENTS.md`; reads `CLAUDE.md` and others; skills; no `llms.txt` | Flat technical corpus, each page anchored to source files | Bold, competitive |
| pi | 95 lines, door | Logo, 2 badges, one-line definition, what it deliberately leaves out; install at line 28 | 5 (install, `cd`, `pi`, `/login`, task); 6 with the version check | Short `AGENTS.md`; a table of which mechanism to use when; no `llms.txt` | Overview, then Quickstart, then How it works, then Guides, then Reference | Calm, plain |
| OpenCode | 130 lines, door | Logo, tagline, 3 badges, language links, screenshot; install at line 46 | The README stops at install; the docs need 5 commands and then a prompt | Many scoped `AGENTS.md` files, including docs writing rules; no `llms.txt` | Organised by product surface (Docs, CLI, Build, API) | Terse |
| Codex CLI | 81 lines, door | Tagline, splash image, which Codex product you want, install; no badges | 3 (install, `codex`, prompt) | Documents `AGENTS.md` precedence; `llms.txt`; `.md` twins; full and condensed exports | Platform matrix | Neutral, official |
| Claude Code | 72 lines, door | Title, 2 badges, one paragraph, demo GIF, install | 4 (install, `cd`, `claude`, prompt); 3 as a one-shot | Short `CLAUDE.md` of hard constraints; reads `AGENTS.md`; `llms.txt`; `.md` twins | Getting started, then Concepts, then Workflows, then Extend, then Operate, then Reference | Patient, explicit |
| Goose | 64 lines, router | Name, tagline, 6 badges, three paragraphs, install | 3 commands, 4 menu choices, 1 prompt; ends in a playable game | Root and docs `AGENTS.md`; an advertised `llms.txt` that has drifted | Quickstart, Docs, and Tutorials in the top bar | Friendly, playful |
| Aider | 180 lines, landing page | Logo, tagline, screencast, usage badges; install at line 103 | 5 from the README; 3 to 4 with the one-line installer | Markdown sources; no `AGENTS.md` or `llms.txt` | Shallow task list with transcripts and recordings | Informal |
| **aayudha today** | 124 lines, neither | Repo name, jargon, a WP number, "pre-0.1" (false since 15 Oct); no visual | 8 typed actions, a pull request, and an interview; ends in a checked Goal | Strong `AGENTS.md` with generated seat and work-item sections; no `llms.txt`; no `.md` twins | Start here, then The story, then How it works, Concepts, Reference | Warm, honest; story-led |

The aayudha row comes from `README.md`, `docs/guide/start.md`, and `internal/tell/site.go` [V]. The installer
was measured with `curl … install.sh | AAYU_INSTALL_DIR=… sh`: it verified the signature and checksum, then installed
aayu 0.1.0 in **5.8 s** [M].

## 3. Patterns worth learning

1. **One sentence of identity, written in the reader's words.** Every good opening says what the tool is, where it
   runs, and what you get, before any taxonomy. Codex also says which neighbouring product you might want instead.
2. **Say what it deliberately does not do.** pi defines itself as much by what it leaves out as by what it has. A
   clear non-goal saves a reader from a wrong expectation.
3. **One canonical install, alternatives later.** Mark one method as recommended. Put OS variants and package
   managers after the first-success path.
4. **Name the first success and check it in stages.** Put a check after each step. "The binary runs" comes before
   "the agent understands the project", which comes before "it made a reviewed change". End with an artifact the
   reader can see.
5. **Give the literal first prompt.** Write "ask: …", never "try a prompt".
6. **Define a term at the moment it is needed.** pi explains *model* and *provider* exactly when `/login` appears.
7. **Separate the modes of reading:** quickstart, task guides, concepts, reference, and troubleshooting. Claude
   Code's progression (work first, then mental model, then daily workflows, then customisation, then reference) is
   the clearest. Pick tasks by situation, as Aider does.
8. **Use a "which mechanism when" table.** pi recommends the least powerful mechanism that solves the need. aayudha
   has the same idea for verbs in [When to use what](../guide/when-to-use-what.md).
9. **Put troubleshooting next to the step that fails.** Name the symptom and what to do. Claude Code adds expected
   output, so the reader can tell success from failure.
10. **Treat the instruction file as a public interface.** Codex and Claude Code document discovery, scope,
    precedence, limits, and how to see what the agent loaded.
11. **Write agent files about the exceptions.** Claude Code's `CLAUDE.md` holds only what is costly and not
    obvious, such as protected paths, release rules, and secrets. It holds no directory tour.
12. **Publish an index and clean page bodies.** An `llms.txt` of one-line descriptions, a `.md` twin per page,
    and optionally a full export let an agent fetch only what it needs.
13. **Keep docs writing rules next to the docs.** OpenCode's docs `AGENTS.md` asks for the common task and a working
    example first, edge cases after, and short sections. Agents editing the docs then follow the house style.
14. **Prove features with a situation and a visual.** oh-my-pi pairs each feature with a short scenario, one
    capture, and one deeper link. A scenario teaches faster than a bare feature name.

## 4. Patterns to avoid

- **A landing page that makes you scroll before you can start.** In oh-my-pi and Aider the install sits 40 to
  100 lines down, below the marketing.
- **A README that ends at "installed".** OpenCode, Goose, and Codex leave the first useful result to another page
  or leave it undefined.
- **Hand-written agent indexes.** Goose's `llms.txt` contradicts its README on the licence. If a file is for
  machines, generate it from the same sources as the human pages.
- **Badge walls and borrowed metrics.** Usage counts and trend badges say nothing about what the tool does for you.
- **Noun-only navigation.** OpenCode's V2 docs are easy to look things up in, but hard to learn from. A newcomer
  needs a path, not a catalogue.
- **Very long first-session pages.** Goose illustrates every provider before the first prompt. Hide the variants and
  keep the main path short.

## 5. Where aayudha stands

What works, and must survive phase 2:

- **The story.** Nine chapters plus a prologue and an epilogue, built on real events with real numbers. Each chapter
  goes scene, what happened, what you do and what the agent does, what you learn, the verbs, under the hood, what's
  next. No other tool studied has this.
- **How it works.** Thirteen deep dives with diagrams, real excerpts, invariants with their tests, and code links.
- **Generated truth.** The verb map, the stage-verbs regions, the reference tables, the claims check, and the brand
  voice check all come from the registry and `design/brand.yaml`, so they cannot drift [V].
- **Honest status notes** wherever setup is missing, and the levels in [Adopt at your own pace](../guide/adopt.md).

What does not work yet [V]:

- **The README's first screen is written for the builders.** It says "twelve seats, nine application layers, and 32
  verbs", cites "(WP8.2)", and the title is the repository name, not the product. Below the fold it has a branch
  table and a 32-row verb table before any example.
- **Status has gone stale.** v0.1.0 was released on 2026-10-15, and `install.sh` on `main` now installs it [M]. The
  README still says "pre-0.1", and the README, "Your first hour", the ladder page, and the epilogue still say the
  install line 404s and point to `rolling-2026-10-12.4`.
- **The first run is longer than it needs to be.** "Your first hour" types `doctor`, `init`, `install` twice,
  `ready` twice, and the agent prompt. `aayu tour` already runs the machine check, `init`, and `install` with their
  own confirmations, "in a few prompts" (`internal/cli/tour.go`). The guide mentions `tour` only once, on the
  "Make it yours" page.
- **There is no agent layer for the docs.** The site has no `llms.txt` and no Markdown twins, and `docs/guide` has
  no writing rules of its own.

## 6. Style guide

### Voice

The guide keeps the voice `design/brand.yaml` already records: warm, direct, plain, and encouraging, in short
sentences. It explains why before how, celebrates real progress, and never uses superlatives. Then each kind of page
gets its own register:

| Page | Register | Shape |
|---|---|---|
| README | Confident, concrete, short | Identity, then proof, then one install, then the first success, then where to go |
| Quickstart | Reassuring and exact | Numbered steps; after each, what you should see; then the next step |
| Story chapter | Narrative, second person | The existing chapter shape, unchanged |
| Situation guide | Imperative | Situation, then the command, then what you get back, then what can go wrong |
| Concept | Explanatory, one picture | Why it exists, the picture, the rule, then a link to the deep dive |
| How it works | Precise, engineer to engineer | Diagram, data structures with excerpts, invariants with their tests, code links |
| Reference | Terse, exhaustive, generated | Never hand-edited |
| Troubleshooting | Symptom first, no blame | What you see, why, then the exact fix command |
| `AGENTS.md` and docs rules | Enforceable, exceptions only | What to do, what never to do, which command proves it |

### Rules

1. **Lead with the reader's problem.** Lead with the outcome, not the architecture. "Seat", "layer", "floor",
   "proof pack", and "ledger" are explained the first time they appear on a page, or linked to the glossary.
2. **One canonical path per task.** One install, one first run, and one command per situation. Put alternatives in a
   collapsed note or on a separate page.
3. **Every step ends with what you should see.** A doctor row, a label, a comment, or a URL. This is the check that
   lets a reader, or an agent, tell success from failure.
4. **Literal prompts and commands, ready to copy.** Every first prompt is written out in full.
5. **The common case first.** Edge cases, flags, and caveats come after the working example.
6. **Honest status, at the step it affects.** "Needs setup" says what is missing and gives the next step. Nothing is
   described as working when it is not built yet, or is not available until something else happens.
7. **Claims trace to evidence.** The existing claims rule stays: link only proven Features; no invented customers,
   numbers, or quotes.
8. **No internal plan vocabulary outside Reference.** No WP numbers, D-rows, or rule numbers. This applies to the
   README too.
9. **Say what aayudha is not.** It does not replace your coding agent, it does not run your production, and it
   never merges without a person's yes.
10. **Generated beats hand-written for anything a machine can derive.** That covers verbs, counts, flags, defaults,
    seat names, and the agent index. When prose must name a generated fact, prefer a generated region.
11. **Short.** About 1 to 3 sentences per paragraph and one idea per sentence. A page that needs more than about
    150 lines of prose should be split.
12. **Accessible by construction.** Alt text states the takeaway, contrast stays AA, nothing depends on colour or
    motion, and every page passes axe with zero serious or critical findings at 1280 px and 390 px.

## 7. README plan (phase 2)

Target: about 80 lines. The first screen, roughly 40 lines, reaches the install command and the first success.

1. **Title `aayudha`**, then one line: the delivery team around the coding agent you already use (Amp, Cursor,
   Claude Code, or Codex). It turns an idea into a measured release, with evidence at every step and a person's yes
   before anything merges.
2. **Three short lines on what you get:** clear work items, a merge gate that proves each change, and a number you
   can measure again. One more line says what it is not.
3. **One visual:** a terminal capture of `aayu tour`, then `aayu next`, as a hand-written inline SVG checked into
   `docs/guide/img/`. No external image host and no GIF. Alt text states the takeaway.
4. **Install:** the one signed command, then `aayu tour`, with no keys and no CI changes (time it in phase 2; do not guess).
   The Go and package-manager alternatives get one line and a link.
5. **Your first success:** the literal prompt that files a Goal, plus `aayu ready <issue>`, and what you will see:
   a Ready label and a snapshot comment.
6. **Where to go next:** four links. The story. Your first hour. When to use what. How it works.
7. **Levels and cost:** three lines (Level 1 free with no keys; Level 2 adds the merge gate, and its key needs follow the 0.2 behaviour;
   Level 3 is autonomy), with a link to Adopt.
8. **Status, licence, contributing, security:** one line each.

The branch table, the verb and plumbing tables, and the host-install details move out of the README into
Reference, where `go generate` renders them. `generate-drift` then still guards them, but outside the doorway. This
needs one change to `internal/verbs/gen`, which today renders the README tables: retarget it to a reference page.

## 8. Docs plan (phase 2)

### Information architecture

The two tracks stay, and the front gets simpler:

```text
Start here     Welcome · Your first hour (tour-based) · When to use what · Adopt at your own pace
The story      Prologue · 1–9 · Epilogue                                  (unchanged shape)
Guides         one page per situation: a vague idea, a bug report, a PR is ready, prove a UI change,
               production broke, explain it to my boss or auditor, did it work, hand work to another agent
How it works   overview + deep dives                                      (unchanged shape)
Concepts       short pages with one picture each
Troubleshoot   symptom → fix, linked from every step that can fail
Reference      generated: verbs, plumbing, CONSTRAINTS, LAYERS, RACI, README tables, config, threat model
```

- **Your first hour** becomes a single path: install, then `aayu tour`, then file a Goal from the literal prompt,
  then `aayu ready`. A "what you should see" check follows each step. A second, optional half hour gets to Level 2:
  your first proven pull request.
- **Guides** grow from the situation table in When to use what into one short page each, every one ending in a link
  to the story chapter where the verb happens and to its deep dive.
- **Troubleshooting** comes from the real failure modes: `doctor` FAIL rows, exit codes 2 to 6, the merge gate's
  common refusals, a missing bot App, and missing keys. It is linked inline at the step where each one appears.

### The agent layer

- `tell docs --site` also writes `llms.txt`: the site name, a one-line summary, and one line per page, taken from
  each page's first sentence and its sidebar group. It also writes a Markdown twin for every page (`page.md` next to
  `page.html`), with a pointer back to the index. Both are generated from the same sources and checked by the same
  claims rule, so they cannot drift the way a hand-written index does.
- A docs-local `docs/guide/AGENTS.md` holds the rules of §6 that an agent can follow and a reviewer can check. Root
  `AGENTS.md` stays as it is: operational, exception-led, and partly generated.
- The guide's "Your agent" pages state how each host loads aayudha's skills and `AGENTS.md`, and how to check
  what it loaded. Each host page follows the same template.

### Where the 0.2 work lands [I]

The seven features are still landing, so the placements below are a plan to check against the merged behaviour.

| 0.2 feature | Story chapter | Guide or concept | How it works |
|---|---|---|---|
| Merge gate without AI keys | 5, the gate says no | Adopt: Level 2's cost line changes | Merge gate; Model gateway |
| Verify skills and feature maps | 4 and 5 | "Prove a UI or behaviour change" | Floors; Proof packs |
| Retro, evolve, and ratchets | 8, the number | "Did it work, and what do we learn" | Eval harness |
| Unpacking vague asks | 1, the goal | "I have a vague idea" | Verb registry (grill) |
| Cross-agent delegation | 4, the build | "Hand work to another agent"; host pages | Architecture; Trust |
| Agent environments | 4, the build | Your agent: make it yours | Architecture; Security |

Story chapters change only where the 12 October run would have gone differently. New behaviour that the run did not
exercise goes in a short "Since then" note, so the story stays true to the day.

### Phase 2 checklist

1. Rebase on `dev` after the founder says 0.2 has merged, and read each feature's merged PR and docs.
2. Rewrite the README to §7. Move the generated tables to Reference.
3. Rebuild Your first hour around `tour`. Fix every stale "before the first release" note.
4. Add the Guides and Troubleshooting pages. Place the 0.2 features using the table above.
5. Generate `llms.txt` and the `.md` twins, with tests in `TestSiteBuild`.
6. Add `docs/guide/AGENTS.md`.
7. Verify as before:
   - `go test`, lint, and markdownlint;
   - the claims check;
   - axe on every page at 1280 px and 390 px;
   - lychee;
   - screenshots;
   - a fresh first-time reader, plus a fresh agent that answers three questions using only `llms.txt`;
   - a timed first run from a clean orb to a Ready Goal, reported as measured.
8. Record any deviations as Appendix B rows.

## 9. Decisions for the founder

1. **The README's job.** The recommendation is a door: identity, install, first success, and links. The alternative
   is a landing page with a feature tour. The door keeps the README short, and the guide already does the selling.
2. **The stale "before the first release" notes.** v0.1.0 has shipped and the install line works, but four pages
   and the README still say it 404s. These could be fixed in a small PR now, rather than waiting for phase 2.
3. **The README visual.** The recommendation is a static terminal capture of `tour` and `next` as an inline SVG.
   The alternative is a recorded GIF on an external host, which the site's rules exclude.
