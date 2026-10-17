# rahulnakmol/agentic-ayudha#488: Decision: approve agents' human-only steps from any device, as a group, by a comment from a separate human identity, with the device code as fallback

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `8edd784309737247acb951b59afaa3ab4152c11472edefe63d1ebd8045bb7f89` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/488/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

How should a person approve agents' human-only steps from any device, as one group, with a native GitHub action (a formatted comment, or a PR review) instead of a device code per command, in a way that works for agents in the cloud (Amp orbs, Codex cloud, Cursor cloud agents, Copilot cloud agent, Claude Code on the web) and where device flow is blocked (an OAuth App policy, Enterprise Managed Users, GHES)?

Counsel from Atri, the Founder's Sage. One level up, this is not a question about comments or codes. It asks which credential proves a person. An approval is worth exactly as much as the credential no agent holds. The device code works because it mints a fresh credential through the person's own sign-in (`presenceCheck`, `internal/cli/presence.go`, #467). A comment or review by the founder is just as good, but only once no agent can write as the founder. Today every agent here can. The founder asked for this after four codes for four Decisions on 2026-10-17. It constrains Story #481, which waits for this Decision.

### Options

1. **Separate the identities, then approve by comment, with a device code as the fallback (recommended, with the corrections in the Evaluation).** Agents act only as a bot, and no agent can reach the approver's GitHub account. aayu groups pending human-only steps into one numbered batch comment. A default-branch workflow verifies a comment from the approver, `/aayu approve all`, `/aayu approve 1 3`, or `/aayu skip 2`, and applies the steps. Device flow, one code per batch, remains where separation is not in place and, if the founder chooses, for one-way doors.
2. **Device flow only, grouped.** One code covers a batch, the notice reaches the person where they are, and no comment counts. No identity work beyond the built-in client ID (#481), but still one code per batch, and no remote approval where device flow is blocked.
3. **Comment approvals now, trusting the pre-tool hook, with no separation.** Not recommended: any agent holding the founder's token can post `/aayu approve all`.

### Evaluation

**Facts from the code and from this orb (2026-10-17).**

- Every GitHub credential an agent in this orb can reach is the founder's:
  - `GH_TOKEN` is a fine-grained PAT that authenticates as @rahulnakmol (`gh api user`), and `GH_PAT` is a classic PAT.
  - git uses `amp git-credential-helper`, which returns a `ghu_` GitHub App user token for @rahulnakmol, valid for under an hour.
  - That token may write issue comments and PR reviews. A malformed comment POST returned 422 (validation, not 403). A review POST created a pending review on #467, which was deleted at once.
  - Amp's docs: "`git push` and `gh` in the orb run as you". They list no bot-only mode (ampcode.com/docs/github).
- The bot path exists in code but is not set up here:
  - `aayu init --identity` (manifest in `internal/initrepo/extras.go:245`) and the broker workflow `.github/workflows/aayudha-token.yml` exist.
  - The repository has no `AAYUDHA_APP_CLIENT_ID` variable and no secrets.
  - `doctor` only warns "bot identity: not configured" (`internal/doctor/checks.go:115`).
  - 86 issues carry `identity:human-fallback`.
- The App manifest asks for contents, issues, and pull_requests write. The App is a bypass actor only on the ledger ruleset (`internal/initrepo/extras.go:319`).
- Pushes to rolling and main are restricted by the ruleset `rolling-main-promotion`, whose only bypass is the admin role. Agents promote today only because they hold the founder's admin PAT.
- aayu already reads a comment as a person's act once: `Stable release: vX.Y.Z` on a Release (`stableRequest`, `internal/cli/release.go:1091`). In solo mode only the pre-tool hook guards it (`HumanOnlyCommand`, `internal/hosts/hook.go:203`), and Codex cloud, Copilot, and a raw API call never meet that hook.
- A PR approval counts through `HumanPass` (`internal/identity/review.go`): a non-bot account that is not the PR's author.

**Each cloud host, from official docs (2026-10-17).** "Can write as the user" means whether an agent there can comment or review as the person who started it.

| Host | Credential in the agent's reach | Can write as the user | Bot-only possible |
|---|---|---|---|
| Amp orb | The signed-in user's `ghu_` token through the git credential helper, plus any PAT in the environment | Yes (checked here) | Not as a setting. Connect Amp's GitHub as a separate agent-only account (unverified that Amp allows it) |
| Codex cloud | Not documented; PRs go through the ChatGPT connector; reviews post as `chatgpt-codex-connector` | Unverified; yes if a PAT is added as a secret | Yes, by adding no user PAT; to be checked live |
| Cursor cloud agents | The Cursor GitHub App ("not through any single person's credentials"); the token inside the VM is not documented | Unverified; yes if a `GH_TOKEN` secret is added | Yes, by adding no user PAT; to be checked live |
| Copilot cloud agent | Pushes to its `copilot/` branch only; "cannot approve or merge a pull request"; read-only MCP token | No, unless `COPILOT_MCP_GITHUB_PERSONAL_ACCESS_TOKEN` is a user PAT | Yes, by default |
| Claude Code on the web | The user's token, injected by a proxy outside the VM; comments post "using your GitHub account" | Yes | Yes, with organisation bot sessions or self-hosted runners |
| Claude Code GitHub Action | `claude[bot]` installation token | No | Yes, by default |
| Any agent on a person's own machine | The person's `gh` login and keychain | Yes | No; aayu can check this but cannot enforce it |

So the founder's question, whether a bot-only `GH_TOKEN` plus a pushing credential is enough, has the answer no in Amp. The pushing credential is the founder's own account, and it can comment and review. In Amp, separation means the Amp GitHub connection belongs to an agent-only account, never to the approver.

**Corrections to the recommended option.**

1. **The agent account.** Where a host only acts "as you" (Amp), use a second GitHub account for the agent side: a machine account with write access, listed in `[identity] bot_logins` so nothing it writes counts as a person. aayu already has the machine-user identity kind (`internal/identity/bot.go:14`). Elsewhere, use the aayudha App installation token.
2. **Two credentials, not one.**
   - The agent bot (App or machine account) gets no bypass. Agents can mint its token through the broker, so if it held a bypass, every agent would hold one.
   - A second App, the approver, holds the bypass on main and on protected paths. Its private key is an environment secret in an environment limited to the default branch (`dev` here), used only by the approval workflow.
   - Rolling and main get separate rulesets: the agent bot may push rolling (agents promote it on their own, AGENTS.md), and only the approver App may update main.
3. **One open batch per thread, so the approver never types a hash.**
   - The batch comment shows its hash.
   - The workflow recomputes the hash from the ledger's record of the batch, including each item's issue body hash. It refuses when the batch comment or the approval comment was edited (GraphQL `lastEditedAt`), when the approval came before the batch, or when a target changed since.
   - A new batch cannot open on a thread while another is open.
4. **A PR review is read only by default-branch code** (`ship`, the merge gate's `issue_comment` path, or a schedule), never by a `pull_request_review` workflow. That event runs the PR's own workflow file, so a PR could rewrite the check.
5. **What a comment can approve.** Shown as step, with what applies it in brackets.
   - Comment approval, applied by the agent bot: `decide --accept` (issues), `plan --approve` and `inspect --dismiss` (the ledger, where the App is already a bypass actor), and `resume` (needs a new `variables: write` permission).
   - Comment approval, applied by the approver App: `release --to main` (the push to main) and `ship --solo-bypass` / `plan --solo-bypass` (the protected-path merge). Alternatively the person merges in GitHub themselves, and `ship` records it.
   - Never by comment, so a terminal or device flow: `init`, `uninit`, `seat`, `install --apply`, and `upgrade --apply`, which change repository settings or protected files from a checkout; and `auth set-secret` and `self-update`, which act on the person's machine.
6. **Exposure turns it off.**
   - `doctor` in every agent session lists each GitHub credential it can reach and that credential's login: environment tokens, `gh auth token`, and `git credential fill` for the repository.
   - A credential that belongs to an approver records an exposure on the ledger and fails `doctor`.
   - The approval workflow refuses comment approvals while an exposure stands. The approver re-enables them with one device code after rotating the exposed token.
7. **The device code stays.**
   - It is the fallback where separation is absent, with one code per batch and the batch listed on the device notice.
   - It is the only way where comment approvals are off.
   - It can be required for one-way doors through a new option, `[approvals] device_for = ["one-way"]`, covering stable release, never-layer waivers, and solo bypasses.
   - Kept from the earlier request: aayu replies on the same thread with each result ("#324 accepted: option 1, by @x in <comment URL>"). Every failure says what happened and the one next step: an edited comment, an unknown approver, a stale batch, an exposure, an expired or denied code, or missing access.

**Option 2 compared.** It costs least (no Apps, no accounts) and keeps one trust anchor that no agent can forge. But it is still a code per batch, it still needs a terminal or a phone browser, and it fails where device flow is blocked.

**Option 3 compared.** It is quickest to build, but it is forgeable today. Amp gives every orb the founder's token by design, and two of the five cloud hosts run no aayu hook. A forged `/aayu approve all` would accept Decisions and merge protected paths in the founder's name.

**A later addition to option 1.** A GitHub environment approval: a pending job with the approver as required reviewer, approved with one tap and a native mobile push ("Requests to approve a deployment"). It needs the same separation, because the approve-deployment API accepts the reviewer's token, and it approves only the whole batch. It can come after option 1 as an "approve all" button.

**Device flow where it is blocked.**

- An organisation's OAuth App restrictions block access to private organisation resources. aayu's device token reads only `/user`, so it should pass; this is inferred, not verified.
- EMU accounts and GHE.com are unverified.
- GHES 3.9 and later serves device flow at its own host, which aayu does not use today because it hard-codes github.com (`githubWeb`, `internal/cli/presence.go`); that is a gap to fix.
- Where device flow is blocked and separation is absent, there is no remote approval: the person runs the command in their own terminal.

**One-time setup (option 1), written down and not yet done.**

For this repository's founder, about 60 minutes:

1. Create the agent bot: run `aayu init --identity` at your own terminal (the App manifest opens in the browser; the private key goes straight into a repository secret; about 5 minutes). Then install the App on the repository (1 minute).
2. Create the approver App: in GitHub, go to Settings, then Developer settings, then GitHub Apps, then New GitHub App, named `aayudha-approver-agentic-ayudha`, with contents, issues, and pull requests write and no webhook. Generate a private key and put it in a new environment `aayudha-approve`, whose deployment branches are limited to `dev`. Install the App on the repository (about 15 minutes).
3. Rulesets: split `rolling-main-promotion` into a rolling ruleset (bypass: the agent bot) and a main ruleset (bypass: the approver App). Add the approver App to the protected-path rulesets (about 10 minutes).
4. The agent account for Amp: create a GitHub account such as `rahulnakmol-agent` with its own email and two-factor authentication, and give it write access, not admin. In Amp, connect GitHub as that account. Remove `GH_TOKEN` and `GH_PAT` from every Amp environment and secret (about 20 minutes; whether Amp accepts a second account is unverified).
5. Record the approver and the agent account in `.aayudha/config.toml` (`[approvals] approvers = ["rahulnakmol"]`, and the agent account in `[identity] bot_logins`). This is a protected-path PR (about 5 minutes).
6. Run `aayu doctor` in each agent environment you use (Amp, Codex cloud, Cursor, Claude Code on the web) and confirm that no credential belongs to an approver (about 2 minutes each).

For any aayu user: steps 1, 5, and 6 (about 15 minutes), step 2 only for bypass steps, and step 4 only on hosts that act as the user. With none of this, device flow works with no setup through the built-in client ID (#481).

**Honest limits.**

- Comment approvals rest on credential hygiene that aayu can check but cannot enforce on a person's machine. An agent under the approver's own `gh` login, or a PAT pasted into a host's secrets, can forge an approval until `doctor` sees it.
- GitHub records no token or app on a review, and no documented field tells a web or mobile comment from a PAT comment.
- An agent driving a browser signed in as the person could approve by comment, by review, or by device code alike.
- The approver App is the most powerful credential in the repository. Its safety rests on the environment's branch limit and on review of the default branch's workflows.

### Recommendation

Option 1, with the seven corrections above, built in three steps that each work alone:

1. Separation and the `doctor` exposure check, with one device code per batch.
2. Comment approvals for the steps the agent bot can apply.
3. The approver App for stable release and protected-path merges.

The door is two-way: every part sits behind configuration. It is marked not reversible below so that it never applies on its own when the SLA lapses, because it changes what counts as proof of a person.

Operations reading: about an hour of setup ends a code per command and the human-fallback writes. Technology reading: it reuses `init --identity`, the broker, `HumanPass`, and `stableRequest`, and adds one workflow and a second App. Product reading: approving from the phone is what every aayu user needs, but only if no one can forge it. The technology reading dominates, because the credential boundary decides whether the product's promise is true.

What you may not want to hear: agents here lose your admin PAT. Agent-run `--admin` merges and promotions stop until the bots are on the bypass lists, and every protected-path merge becomes a batch item you approve. What would change this counsel: if Amp cannot connect a second GitHub account, or you will keep running agents on a machine signed in as yourself, take option 2.

### Reversible

false

### Links

constrains #481
````
