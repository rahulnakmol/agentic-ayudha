# #673 UX design

The surface is one page of the docs site, read by a person and, through the site's Markdown twin and `llms.txt`, by
an agent (`docs/guide/AGENTS.md`, "The site"). There is no command, card, or screen.

## Today on dev

There is no XERJ page. A reader finds only "A guide shows how to run XERJ code search beside aayu, and what to watch
for" under "Planned next, for v0.4" in `docs/releases/v0.3.0.md:462`.

## Planned, not built

A page in the `guide-*.md` shape (`docs/guide/AGENTS.md`, "Page shapes"):

1. Situation: you want local code search for your host's model, and you use aayu.
2. The command: install a pinned XERJ release, verify it with `gh release verify-asset`, and run the node with auth on
   or loopback-only (#673 Scope). Each step ends with "You should see" (`docs/guide/AGENTS.md`, "Always").
3. What you get back: `xerj def` and `xerj search` results for the host's model, as leads, not evidence (#673).
4. What can go wrong: the four conflicts in #673 (field-report PR, `--insecure`, the absolute path in `.mcp.json`,
   memory on a large index), each with what to do instead.
5. Under the hood: the trust boundary stays as it is; aayu's survey index is the only source of evidence (#673).
6. What's next.

A `> **Honest status:**` callout names the XERJ version checked and says XERJ is pre-1.0 (#673).

## Agent-facing surface

An agent reads the page through `llms.txt`. The page must say that an agent never files a XERJ report, issue, or pull
request (#673 Scope).

## Accessibility

Before finishing, open the page at 1280 px and 390 px and run axe with zero serious or critical findings
(`docs/guide/AGENTS.md`, "Build and check"). Images need alt text that states the takeaway (same file, "Always").
