# `tell`: write docs and release notes from proven work

Copy that claims more than the work proves costs trust. `tell` writes docs, release notes, and launch copy in your
brand's voice, and links a piece of work only when it is a Feature whose shipped stories all have authoritative proof.

<!-- BEGIN GENERATED: skill-card -->

| Field | Value |
|---|---|
| In plain words | Writes docs, release notes, and launch copy in your brand's voice, linking only work that is proven. |
| Journey stage | [Explain it](../journey-9-explain.md) |
| Accountable seat | [Marketer & Documenter](../../../seats/marketer-documenter/CHARTER.md) (Sauti) |
| Does the work | Sauti, the Accountable seat's twin, in the [`spec`](../how-models.md) model lane, with the seat's paired human when it has one |
| Consulted | [Mentalist](../../../seats/mentalist/CHARTER.md) (Sanjay), [Product Strategist](../../../seats/product-strategist/CHARTER.md) (Arjun) |
| Informed | [Founder](../../../seats/founder/CHARTER.md) (Atri) |
| Layers it can touch | [L01 Experience](../concepts-layers.md) |
| Who reasons | your agent's own model, inside the skill (publish requires the human identity) |
| Writes as | a person's own GitHub identity |
| Where it runs | the `aayu` command, a skill in your agent, an MCP tool (only with `[mcp] expose_writes` on) |
| The skill your agent reads | [skills/tell/SKILL.md](../../../skills/tell/SKILL.md) |

<!-- END GENERATED: skill-card -->

## When to use it

- A release went out and needs its release notes or changelog entry (`release` or `changelog`, with `--tag`).
- A proven Feature needs a how-to page or a launch announcement (`guide` or `launch`).
- You want to rebuild the docs site with every page checked (`docs --site`).

Explaining one work item to one audience is [`brief`](brief.md).

## Run it

Your agent reads the brief (your voice from `design/brand.yaml`, the proven Features, the UTM tags), drafts, and checks
with `--from`. Publishing is a person's act: run `--publish` yourself, in your own terminal.

```text
Use the tell skill to write the release notes for v0.2.0 from the proven Features, check them with tell, and tell me what you left out and why.
```

<!-- BEGIN GENERATED: skill-usage -->

```sh
aayu tell <kind>
```

| Input | Required | What it does |
|---|---|---|
| `<kind>` | yes | What to write. One of: `docs`, `guide`, `changelog`, `release`, `launch`, `brand`. |
| `--from` | no | Markdown draft the host wrote; tell checks its claims and tone and tags its links. Without it, tell prints the brief: tone, proven Features, and UTM tags. |
| `--tag` | no | Release tag the copy is about (release, changelog); publishing release notes also sets that GitHub Release's notes. |
| `--campaign` | no | UTM campaign; default the tag, else \<kind>-\<YYYY-MM-DD>. |
| `--site` | no | With docs: build the static docs site (README, getting started, verbs, skills, governance pages) into this new or empty directory; every page passes the claims check. |
| `--publish` | no | Publish the checked copy to the ledger (and the tag's GitHub Release); human identity only, logged. **Person only.** |

<!-- END GENERATED: skill-usage -->

After you read the checked draft: `aayu tell release --from notes.md --tag v0.2.0 --publish`.

## What you get back

The checked draft, with every outbound link tagged with `utm_source`, `utm_medium`, and `utm_campaign`. `--publish`
lands it on the [ledger](../../../GLOSSARY.md) at `tell/<kind>/<campaign>.md` and, for release notes, sets the tag's
GitHub Release notes. `--site` builds the docs site; one refused page refuses the whole site.

**You should see** exit 0 and the draft back with tagged links. Exit 4 names what to rewrite: a link to unproven
work, an avoided word, a superlative, or an exclamation mark. See [exit codes](../troubleshooting.md).

## Gates

<!-- BEGIN GENERATED: skill-gates -->

It checks these before it acts, and stops when one fails:

- **authoritative-pack:** An authoritative (CI-written) proof pack exists.
- **human-identity:** Runs only under a human identity.

<!-- END GENERATED: skill-gates -->

An authoritative pack is the proof pack the merge gate wrote in CI, not one an agent pushed.

## What only a person does

<!-- BEGIN GENERATED: skill-human -->

- It writes under a person's own GitHub identity.
- It runs only under a person's identity, never the bot's.
- `--publish`: Publish the checked copy to the ledger (and the tag's GitHub Release); human identity only, logged.

<!-- END GENERATED: skill-human -->

Read the checked draft and your agent's note on what it left out, then publish under your own account. See
[the human pass](../concepts-human-pass.md).

## What's next

- Explain one piece of work to one audience: [`brief`](brief.md).
- The story of the telling: [chapter 9](../journey-9-explain.md).
- [Every skill](README.md).
