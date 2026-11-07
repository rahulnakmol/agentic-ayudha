# #673 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/673`. This page uses
[#673](https://github.com/rahulnakmol/agentic-ayudha/issues/673) only. #673 cites private research note
xerj-2026-10.md, §4.0 and §5; that note was not read for this page.

## Goal

Quoted from #673: "One guide page on how an aayu user can run XERJ (github.com/xerj-org/xerj, Apache-2.0) beside
aayu. With XERJ, the host's model finds code with `xerj def` and `xerj search` instead of reading whole files, and
aayu's trust boundary stays as it is. aayu does not run, proxy, configure, or depend on XERJ."

## Who it serves and the customer outcome

- People who use Claude Code, Cursor, Amp, and Codex and add local code-search tools (#673 Problem).
- Outcome (inferred from #673): they can run XERJ beside aayu without breaking aayu's rules on GitHub writes,
  protected paths, and evidence.

## Problem

From #673: `xerj init` writes to files that `aayu install` also manages (`.mcp.json`, `.claude/skills`,
`AGENTS.md`). Four parts of XERJ conflict with how aayu works, and no aayu page warns about them:

1. XERJ's llms.txt says every agent must open a field-report pull request to xerj-org/xerj. In an aayu repository,
   every GitHub write goes through aayu.
2. The quick start runs `--insecure`, which turns off auth, so every local caller is a superuser.
3. `xerj init` writes an absolute, machine-specific binary path into `.mcp.json` and calls it meant to be committed.
   In aayu, `.mcp.json` is a protected path (it is in `.github/CODEOWNERS`).
4. Indexing the whole aayu checkout pushed the node to about 2.9 GB, and it was killed on a 3.9 GB machine.

These four facts come from #673 and its private research note. They were not re-checked against XERJ for this
page (unverified).

## Scope

In, from #673: one guide page under `docs/guide/`, registered in the site's sidebar, that says how to run XERJ beside
aayu, what to avoid, that aayu's survey index stays the only source of evidence (XERJ results are untrusted text and
leads, never citations), and a `> **Honest status:**` callout naming the XERJ version checked and that XERJ is
pre-1.0.

## Out of scope

Quoted from #673: "any code beyond the sidebar entry; a config key; an MCP passthrough; a call to the XERJ CLI from a
verb; a doctor check; a floor." A pre-tool hook guard against agent writes to xerj-org/xerj "is deferred until there
is evidence that agents act on XERJ's request (founder, 2026-10-18: "Wait")."

## Success measures

The sources state no success measure.
