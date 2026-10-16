# AGENTS.md: writing the guide

Rules for coding agents that edit `docs/guide/` or the root `README.md`. The root [AGENTS.md](../../AGENTS.md) still
applies. The reasons behind these rules are in
[docs/research/docs-style-2026-10.md](../research/docs-style-2026-10.md).

## Build and check

```sh
go generate ./...                     # regenerates the verb map, the stage-verbs regions, and the diagrams
markdownlint-cli2                     # the CI docs job
aayu tell docs --site /tmp/site       # builds the site; it refuses a claim it cannot trace (see below)
```

Before you finish, open every page you changed in a browser at 1280 px and 390 px, run axe on it (zero serious or
critical findings), and run lychee on the built site.

## Never

- Never edit between `<!-- BEGIN GENERATED: … -->` and `<!-- END GENERATED: … -->`, or a `*.svg` that starts with
  "Code generated". Change the source (`internal/verbs/registry.go`, a `img/*.diagram` file) and run `go generate`.
- Never write a work item reference as `#123`. The site links only Features whose shipped stories all have proof, and
  `tell` refuses any other. Write "pull request 148" or "story 131", with no link.
- Never invent a customer, a number, a quote, or a result. Every number in the story comes from GitHub, the ledger, or
  an Appendix B row of [the build plan](../BUILD-PLAN.md).
- Never use the words `design/brand.yaml` avoids (seamless, effortless, guaranteed, magical, …) or exclamation marks.
- Never write work package numbers, Appendix B row IDs, or rule numbers outside the How it works pages' code and
  honest-status sections.
- Never describe something as working when it waits on setup or a later change. Say what is missing and the next step,
  in a `> **Honest status:**` or `> **Needs setup:**` callout at the step it affects.

## Always

- **Lead with the reader's problem**, then the outcome, then the command. Explain a term the first time it appears on
  a page (seat, layer, floor, proof pack, ledger), or link the [glossary](../../GLOSSARY.md).
- **One path per task.** One install, one first run, one command per situation. Alternatives go after, or on their
  own page.
- **End every step with "You should see"**: the label, row, comment, or output that means it worked. Link the
  matching [troubleshooting](troubleshooting.md) section where a step can fail.
- **Write prompts and commands in full**, ready to copy. Never "try a prompt".
- **Short.** One to three sentences per paragraph, one idea per sentence, prose wrapped near 116 columns. Split a page
  past about 150 lines of prose.
- **Images:** alt text states the takeaway, not "diagram of". Diagrams are `img/<name>.diagram` files rendered by
  `go generate`; hand-made pictures go in `media/`, never on an outside host.

## Page shapes

| Page | Shape |
|---|---|
| `README.md` (root) | About 80 lines: what it is, three lines of what you get, what it is not, the first-run picture, install, first run, four links, levels, project |
| `start.md` | Numbered steps, each ending in "You should see" |
| `guide-*.md` | Situation, the command, what you get back, what can go wrong, Under the hood, What's next |
| `journey*.md` | Timed scene, what happened, what you do and what the agent does, what you learn, the generated verbs, Under the hood, What's next, then You are here |
| `how-*.md` | Diagram, How it works, key formats with verbatim excerpts, invariants naming their tests, code links, Read next |
| `concepts-*.md` | Why it exists, one picture, the rule, a link to the deep dive |
| `troubleshooting.md` | What you see, why, the exact fix |

## The story stays true to the day

The story retells the real run of 12 October 2026. When aayudha's behaviour changes, do not rewrite what happened.
Add a `> **Since then:**` callout to the chapter it affects, saying what is different now and linking its guide.

## The site

The sidebar order and groups are `sitePages` in [internal/tell/site.go](../../internal/tell/site.go): add a new page
there. The site also writes a Markdown twin of every page and `llms.txt` for agents, from the same checked text, so
there is nothing extra to maintain.
