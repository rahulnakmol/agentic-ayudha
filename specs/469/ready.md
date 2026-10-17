# rahulnakmol/agentic-ayudha#469: Decision: the review ladder: a labelled same-family review by default, a different family when a host includes one, a paid key only when added

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `bcec2dc2ee8c7380f84179385b3f7d99583071bab7fb235d953948b995a217eb` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/469/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

How does aayu review a pull request when the user has only one model family? The plan requires an adversarial review by a model of a different family from the builder's (`internal/verbs/registry.go`, lanes `review` and `ui-review` marked `Adversarial`; `internal/inspect/apps.go`, `ChooseReviewer`). Users with a single subscription (Claude only, ChatGPT only, many Cursor and Grok users) report that this rule leaves them with no valid review. Since 12 October 2026 the problem is wider: Cursor's new plans include only Cursor's own models (Grok 4.7, 4.6, 4.5, Composer 2.5). Claude, GPT and Gemini are list-price pay-as-you-go on Premium and above, and absent on Free and Plus (https://cursor.com/docs/models-and-pricing and https://cursor.com/help/models-and-usage/available-models, read 2026-10-17).

The founder settled the frame on 2026-10-17. This Decision records it and the details settled under the founder's delegated authority, each with its reason:

- **Default: rung C, a labelled same-family review.** Most aayu users have one subscription.
- **Rung A, used automatically, and it counts:** a different family that a host the user already has includes at no extra cost.
- **Rung B, opt-in only:** a pay-as-you-go key the user adds. Never used automatically.
- **Never:** no review at all, or a same-family review labelled as different-family.

Why now: pull request 466 makes inspect run on every non-docs PR. Without this ladder, a single-family user's PRs would all fail closed or fall back to a person, and the different-family rule would stay assumed rather than recorded.

### Options

1. **The review ladder, rung C by default (recommended, founder-approved frame).** Record the builder's family and the rung used on every PR. inspect picks the highest available rung: A, then B if the user configured it, then C. C is labelled honestly everywhere and never earns autonomy.
2. **Strict different-family, fail closed.** No reviewer of another family means a person reviews every PR. This is honest, but unusable for single-subscription users, so they would turn the gate off.
3. **Different-family through a bundled paid key.** aayu pays, or requires a key, for every user. This conflicts with "no AI keys in a product repo's CI" (founder, 2026-10-15) and adds cost and a secret to every repo.
4. **Do nothing.** Same-family reviews keep happening silently where a host has one family, and are counted as if adversarial.

### Evaluation

**The rungs, in the order inspect tries them.**

| Rung | When | Counts toward autonomy | Cost | Label on the PR check, proof pack, status, audit |
|---|---|---|---|---|
| A | An installed host or reviewer app includes a family outside the builder's candidate families, at no extra cost | yes | none beyond the existing subscription | `different-family review (A): <reviewer model> (<family>) vs builder <family>` |
| B | The user added a pay-as-you-go key (`aayu auth set-secret openrouter` or a provider key) and `[inspect] paid_reviewer` names a model; per-PR budget `[budget] per_pr_inspect_max` (default 5 USD) | yes | measured below: about 0.02 to 0.12 USD per review | `different-family review (B, paid): …` |
| C | Default, when neither A nor B is available | **never** | none | `same-family review (C): <reviewer model> (<family>), builder <family>` |
| none | not allowed | — | — | a person must review: inspect fails closed with the human-review fallback |

**Rung A hosts (included families at no extra cost), as of 2026-10-17:**

| Host or app | Families included | Rung A when the builder is | Source |
|---|---|---|---|
| GitHub Copilot (choose a model, or HydraFusion Critique) | anthropic, openai, google (plan and org policy permitting) | any family; Critique's critic is another family by design, and its family must be recorded | Copilot shows the model per response; the hosts thread adding Copilot records it |
| Amp | several (mode-managed) | any family outside the mode's candidates | `internal/verbs/registry.go`, `FamilyHostManaged` |
| Claude Code and Codex, both installed | anthropic, openai | either one | two existing subscriptions |
| Cursor, legacy Pro, Pro+ or Ultra | anthropic, google, xai (openai excluded today, `Excludes` on `HostCursor`) | any family | Cursor's legacy allowance, kept until the user switches |
| Cursor, new plan (any paid tier) | xai (Grok 4.7 lane) | anything but xai | Cursor docs above; the existing "Grok 4.7 on Cursor" review lane |
| Reviewer apps installed on the org (`@claude`, `@codex`, `@cursor`, `@amp`) | the app's family | anything but that family | `[inspect] apps` and `app_families` in `internal/config/defaults.toml` |

**Rung B cost, measured.** These are the real prompts inspect builds (`inspect.Runner`, the review request with the pack, plan, and diff) for six merged PRs of this repository. Input tokens were counted with o200k_base. Output tokens come from the live GPT-6.1 Sol reviews of 12 October (inspect-live run 38195395594: 0.0400, 0.0466 and 0.0508 USD on 2.3K-token prompts, which back-solves to 3.5K to 4.6K output tokens, reasoning included). Prices are `internal/models/prices.go` list rates per million tokens: GPT-6.1 Sol 2 / 10, Claude Sonnet 5.5 2 / 10, Grok 4.7 2 / 6, Gemini 3.8 Flash 0.75 / 3.75.

| PR (diff) | Input tokens | GPT-6.1 Sol or Sonnet 5.5 | Grok 4.7 | Gemini 3.8 Flash |
|---|---|---|---|---|
| 456 (3 KB, small) | 3,218 | 0.05 USD | 0.03 USD | 0.02 USD |
| 428 (8 KB, median) | 4,774 | 0.06 USD | 0.04 USD | 0.02 USD |
| 427 (9 KB) | 5,522 | 0.06 USD | 0.04 USD | 0.02 USD |
| 448 (13 KB) | 6,815 | 0.06 USD | 0.04 USD | 0.02 USD |
| 443 (54 KB, large) | 20,241 | 0.09 to 0.12 USD | 0.07 to 0.09 USD | 0.03 to 0.05 USD |
| 442 (49 KB, large) | 20,431 | 0.09 to 0.12 USD | 0.07 to 0.09 USD | 0.03 to 0.05 USD |

A PR is reviewed at most `MaxCycles` = 2 times (`internal/inspect/inspect.go:37`), so B costs about 0.04 to 0.24 USD per PR. OpenRouter passes through provider list prices; its fee on credit purchases is extra. **Not measured live:** this orb's AI gateway key is rejected (HTTP 401), so these are measured prompt sizes priced at list rates, not billed calls. Tokenizers other than OpenAI's can differ by about 20 %. The large-diff rows assume 4.5K to 8K output tokens.

**Details settled under the founder's delegated authority, each with its reason.**

1. **Order A, B, C, with A preferred over B.** A costs nothing, and B exists only for users without A. B is used only when the user both adds a key and names `[inspect] paid_reviewer`, so no review ever spends money by surprise.
2. **The reviewer's family is the reported one, not the planned one.** A rung is recorded from the model the reviewer actually reports in its reply (a `model` field this work adds to the stated reply format, a Copilot per-response model, or else the app's fixed family in `[inspect] app_families`). A reply that names no model, or names a model of the builder's candidate families, is recorded as C even if A was planned. This keeps the label true when a host silently substitutes a model.
3. **The builder's family is recorded, never assumed.** `aayu build` writes `Aayudha-Host:` and `Aayudha-Model:` trailers on every checkpoint commit. A PR opened outside build declares the same two lines in its body. Copilot builds record the reported model per response (the field is agreed with the hosts thread adding Copilot). The family is resolved from the model ID through the registry (`verbs.FamilyOf`).
4. **Host-managed builders (Amp modes).** `Aayudha-Model: host-managed` resolves to the mode's candidate families. For Amp medium, that is anthropic and openai, because Amp may route a mode to either. A reviewer counts as A only if its family is outside every candidate, for example Gemini or Grok for Amp medium. Otherwise the review is C. This is the safe reading of "possibly OpenAI".
5. **An unknown builder never blocks.** With no trailer, no body declaration, and no pack, the candidates are every family. No reviewer can then be shown to differ, so the review runs as C, labelled `same-family review (C): builder family unknown`. It replaces the fail-closed rule first drafted for this work, which would have blocked single-subscription users. A person can still declare the builder in the PR body; editing the body re-runs the gate.
6. **Codex automatic review.** Codex's own review counts only when inspect chose Codex for this PR, and only as C when the builder may be OpenAI (an OpenAI or host-managed builder). inspect reads replies only from the chosen reviewer's login (`inspect.ReadReply`), so Codex's automatic receipt on other PRs is ignored, not counted.
7. **How rung C reviews.** C uses a different model or tier of the same family where the host offers one (Opus builder → Sonnet reviewer; GPT-6.1 Sol builder → GPT-6 Astra; Grok 4.7 builder → Grok 4.6). It runs in a fresh context with no builder transcript, the same adversarial review prompt, the same stated reply format, and the same deterministic floors. A missing or malformed reply fails closed, exactly as in A and B.
8. **What C never earns.** A C review never counts toward the earned-autonomy streak (`autonomy.Rules.Streaks`), never makes a reversible Decision apply provisionally, never raises an autonomy level, and never lets an agent merge without a person. Never-layers (L04, L07, L09) and protected paths (`.github/CODEOWNERS`) need a person's review whatever the rung.
9. **No review is never an option.** When no reviewer of any rung can reply (no app installed, no host reachable from the merge gate), inspect fails closed with the existing human-review fallback, as today.
10. **Composer 2.5 counts as unknown lineage.** Cursor does not publish Composer's base model. So Composer is never treated as a family different from a Grok build, or from any other build: reviews between them are C. Grok 4.7 reviewing a non-xai build is A.
11. **Discovery spends nothing.** `aayu doctor` and `aayu install` derive the reachable families from what is installed and configured (detected hosts, `[inspect] apps`, `[hosts.cursor] plan`, a configured paid key). They make no model call. A declared family is confirmed only by the reviewer's reported model (detail 2).

**Doctor output.** One row tells the user their rung and how to move up:

```text
review rung   C  same-family review: builder anthropic (claude-opus-5-5), no other family reachable
              move up (free): install the Copilot, Codex, or Cursor (legacy plan) reviewer app, or Amp
              move up (paid, opt-in): aayu auth set-secret openrouter, then [inspect] paid_reviewer = "gemini-3.8-flash" (about 0.02 USD a review)
```

```text
review rung   A  different-family review: builder anthropic, reviewer @codex (openai), installed on the org
```

**Cursor lane defaults (a choice in this Decision).** Today's Cursor column (`docs/guide/how-models.md`, from the registry) is build, spec and plan on Claude Opus 5.5, chore on Haiku 5.5, review on Grok 4.7, and ui-review on Gemini. On a new Cursor plan, all of these except Grok are list-price pay-as-you-go.

- **(a) Recommended: a plan key with a safe default.** Add `[hosts.cursor] plan = "x" | "legacy"`, default `"x"`, because aayu cannot tell the plans apart. The safe default never spends list-price money by surprise.
  - On `"x"`: build, spec and plan default to Grok 4.7, and chore to Composer 2.5, which are included. review and ui-review need a non-xai family: rung A from another host the user has, else B if configured, else C, labelled.
  - On `"legacy"`: today's defaults are unchanged.
- **(b) Keep today's defaults.** Cursor users on new plans pay list price for every lane by default. Rejected: surprise spend.
- **(c) Grok everywhere on Cursor.** Rejected: legacy-plan users lose an included different-family reviewer.

No lane changes until this Decision is accepted.

### Recommendation

Option 1, the review ladder with rung C by default, the eleven details above, and Cursor choice (a).

Build order once accepted:

1. Pull request 466 (inspect on every non-draft, non-docs PR) proceeds now, independent of this Decision.
2. Next PR: record the builder's host and model, plus the rung and the reviewer's reported family, on the PR check, the inspect state, and the proof pack. `status` and `audit` show the rung. The autonomy streak skips C reviews.
3. Then the doctor rung row and `[inspect] paid_reviewer` for B.
4. Last, the Cursor plan key and its lane defaults.

### Reversible

true

### Links

constrains #466
````
