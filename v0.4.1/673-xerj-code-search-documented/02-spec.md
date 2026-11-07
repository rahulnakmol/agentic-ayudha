# #673 Spec

No spec baseline exists (`specs/673` is absent from the ledger) and no Story is filed. Every requirement traces to
the Scope of [#673](https://github.com/rahulnakmol/agentic-ayudha/issues/673). All map to one proposed slice, P1
(proposed, not filed).

## Functional requirements

| # | Requirement | Source |
|---|---|---|
| FR1 | One page under `docs/guide/`, registered in the site's sidebar. | #673 Scope; Expected paths `docs/guide/guide-xerj.md`, `internal/tell/site.go` |
| FR2 | Install a pinned XERJ release and check it with `gh release verify-asset`. | #673 Scope |
| FR3 | Run the node with auth on, or loopback-only on a one-person machine. | #673 Scope |
| FR4 | Register the MCP server per user, not in the committed `.mcp.json`. | #673 Scope |
| FR5 | Say what to avoid: the field-report pull request (an agent never files a XERJ report, issue, or pull request; a person may, from their own account); the auth-off quick start; the absolute path `xerj init` writes into `.mcp.json`; indexing a large repository in one run without measuring memory first (use `--dry-run` and fewer workers). | #673 Scope |
| FR6 | Say that aayu's survey index stays the only source of evidence; XERJ results are untrusted text and leads, never citations. | #673 Scope |
| FR7 | A `> **Honest status:**` callout names the XERJ version the page was checked against and says XERJ is pre-1.0. | #673 Scope |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Docs quality | The page follows the `guide-*.md` shape: situation, the command, what you get back, what can go wrong, Under the hood, What's next. | `docs/guide/AGENTS.md`, "Page shapes" |
| Docs quality | No `#123` work-item references; no invented numbers; no words that `design/brand.yaml` avoids. | `docs/guide/AGENTS.md`, "Never" |
| Checks | `markdownlint-cli2`, `aayu tell docs --site /tmp/site`, axe with zero serious or critical findings at 1280 px and 390 px, and lychee on the built site. | `docs/guide/AGENTS.md`, "Build and check" |
| Security | The page never tells a person to commit `.mcp.json` changes; `.mcp.json` is a protected path. | #673 Problem; `.github/CODEOWNERS` |
| Truth | The XERJ version and flags are checked against the pinned release when the page is written (FR7). | #673 Scope |

## Open questions

- Which XERJ release to pin. Not in any source read here (unverified).
- The memory number "about 2.9 GB" comes from a private research note. `docs/guide/AGENTS.md` says every number must
  come from GitHub, the ledger, a PR's "Deviation from the plan:", or an archived Appendix B row. Whether a private
  research note counts is unverified.
