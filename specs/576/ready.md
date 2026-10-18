# rahulnakmol/agentic-ayudha#576: Decision: how agents and CI get the bot identity on every repository without a new App, key, and browser flow per repository

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `7fc3253afc43a1d4e5b713f0a857134dcfbb71604b01e98efd3fce2bfb8cd0e0` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/576/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

How should agent lanes and CI get the aayudha bot identity on every repository a person owns, without a new App, a new key, and a browser flow per repository?

Why now: `aayu init --identity` registers a new private App named `aayudha-<repo>` through the manifest flow in a browser (`AppManifest`, `internal/initrepo/extras.go:259`). It puts that App's key in the repository secret `AAYUDHA_APP_PRIVATE_KEY` (`extras.go:317`), and then the person clicks Install. Agents in Amp orbs also need Amp moved to a machine account, because the orb's credential helper hands out the person's own token (docs/guide/how-trust.md, "Approving a batch from your phone").

At 10 or more repositories that is hours of setup. It also blocks three things:
- approval by reply, which needs the bot to be an App (#481, shipped in pull request 501);
- protected-path relief, which needs the agent App on the code-owner ruleset's bypass list (#572);
- approving from Claude Code or Codex on the web.

Unchanged under every option:
- V1: agents and CI write as a bot, never as the person (docs/BUILD-PLAN.md, V1);
- the human pass and the human-only actions;
- Decision #488: no approver credential is reachable from an agent session.

Required under every option: the one-time setup works from a phone and from a cloud agent's terminal. That terminal can be an Amp orb's Terminal tab, Claude Code or Codex on the web, or SSH.
- Every step is either a GitHub or Azure web page a phone can open, or a command that prints an HTTPS link and takes a pasted code.
- No step uses a loopback callback, a local file, or a desktop browser.
- Today's App creation fails this: the founder ran `aayu init --identity` in an orb's Terminal tab from a phone on 2026-10-18. The manifest flow's start page and its `redirect_url` are `http://127.0.0.1:<port>` (`internal/cli/manifest.go`), so nothing could reach them, and no App was created. Bug #578 fixes the flow: HTTPS start and redirect pages, and a pasted code checked against the state. That fix is needed by options 1 to 3.

### Options

1. **One App per owner, reused, with a broker repository.**
   - The first `init` registers `aayudha-<owner>` (manifest flow, once) and installs it on the owner's repositories.
   - Later inits add the new repository to the existing installation by API, with no browser. That is `PUT /user/installations/{id}/repositories/{repo_id}`, which takes a fine-grained PAT or an App user token from a repository admin. It needs no call at all if the App is installed on all repositories.
   - One private repository, `<owner>/aayudha-broker`, holds the only copy of the key. Its `aayudha-token.yml` mints installation tokens narrowed to one repository and the verb's permissions.
   - The catch is CI. A workflow in another repository cannot read the broker's secret. A personal account has no account-level Actions secrets, and `GITHUB_TOKEN` cannot dispatch a workflow in another repository. So either `init` copies the key into each repository's secret, through the broker and the App's own token, or CI stays as it is today.
   - Copying the key means a leak from any one repository mints tokens for all of them; today a leak reaches one.
   - Cost: about 2 days.
2. **A keyless OIDC token exchange, self-hosted (recommended).**
   - Run Chainguard's Octo-STS (Apache-2.0, Go, release v0.12.0 on 2026-10-09) as a separate service on the founder's Azure infrastructure. It sits behind the aigateway Cloudflare tunnel, has its own systemd unit and user, and keeps the `aayudha-<owner>` App key in Azure Key Vault (`KMS_PROVIDER=akv`). Install the App once per owner, as in option 1.
   - Each repository holds only a trust policy, `.github/chainguard/<lane>.sts.yaml`, which says which issuer and subject may get which permissions on that repository. No product repository holds a key.
   - Who can present a token:
     - CI presents its Actions OIDC token (`id-token: write`).
     - An Amp orb presents `amp orb id-token`. That is verified: issuer `https://ampcode.com/api/workload-identity`, with claims `project_id`, `user_id` and `thread_id`, and a caller-chosen audience.
     - Cursor cloud agents present their agent-socket token (issuer `https://api.cursor.com`).
     - Claude Code on the web (Anthropic-hosted) and Codex on the web issue no token a third party can verify. They reach the exchange with a per-environment network secret, which the host's proxy attaches and the agent never sees.
   - Cost: about 4 to 5 days of build, plus about 45 minutes of the founder's time once (Key Vault, a hostname, the App). Ongoing: track Octo-STS releases. It has had 4 advisories, 2 of them high, in 2024 to 2026.
3. **A published aayudha App with a hosted token service, run by the project**, as Codex and Claude do.
   - Any user clicks Install once. The project's service mints every user's tokens.
   - For this founder it is option 2 plus the App made public.
   - It adds central trust: one key, held by the project, can write every installer's repositories, bounded only by the project's code. It also adds an operating duty to strangers: uptime, abuse, rate limits, and liability.
   - Cost: option 2, plus ongoing operations.
4. **A machine user with a PAT per owner.**
   - Create one machine account (GitHub allows one free machine account per person). Invite it to each repository, accept the invitation by API, and store its token as each repository's secret.
   - A fine-grained PAT cannot reach a personal account's repository as a collaborator, so this needs a classic PAT with `repo` scope. That token reaches every repository the machine user can, until it is rotated.
   - Amp can connect to this account, so orbs get its short-lived token and never the person's.
   - It turns approval by reply off: comment approval requires the App (pull request 501, "Reply approval requires the GitHub App").
   - Cost: about 1 day.

### Evaluation

**Verified facts** (GitHub docs, read 2026-10-18, unless marked):

- `PUT /user/installations/{installation_id}/repositories/{repository_id}` works with fine-grained PATs, App user tokens, and installation tokens that hold "GitHub App installation repository access" (write). "The authenticated user must have admin access to the repository." Source: https://docs.github.com/en/rest/apps/installations
- No endpoint creates an installation. The first Install is a click.
- Installation tokens can be narrowed to listed repositories and a subset of permissions, and "expire one hour from the time you create them". Source: https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app
- "Private GitHub Apps can only be installed on the user or organization account of the app owner." So one private App serves one owner.
- Fine-grained PATs do not support "repositories where the user is an outside or repository collaborator". Source: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens
- "You may maintain no more than one free machine account in addition to your free Personal Account." Source: the GitHub Terms of Service.
- Octo-STS:
  - It accepts any OIDC issuer, unless `OCTOSTS_ALLOWED_ISSUERS` or an organisation allowlist limits it.
  - It mints the token with its own App key and narrows it to the trust policy.
  - The public instance's App asks for organisation administration and `workflows: write`, among others, and "Chainguard does not currently offer any level of support". Source: https://github.com/octo-sts/app
- Unverified:
  - whether a GitHub App can be a ruleset bypass actor on a personal repository (the REST enum lists `Integration` and excludes only `OrganizationAdmin` there);
  - whether Octo-STS serves an installation on a personal account rather than an organisation.

**Side by side.** Setup minutes are estimates of the person's time.

| | Today | 1 App per owner + broker | 2 OIDC exchange | 3 Published App | 4 Machine user |
|---|---|---|---|---|---|
| First repository | ~15 min, browser | ~15 min, browser once | ~45 min once, then ~2 | 1 click | ~30 min |
| 10th repository | ~15 min, browser | ~1 min, none | ~1 min, none | 0 | ~3 min |
| Where the secret lives | one key per repo secret | broker secret, plus a copy per repo for CI | Key Vault only | the project's KMS | a classic PAT in every repo secret |
| What a leaked repo secret reaches | that repo | every repo (with copies) | none in repos | none in repos | every repo the account can reach |
| What an agent holds | the person's token (orbs) | a 1 h token, fetched with the person's token | a 1 h token, scoped by the repo's policy | the same as 2 | the PAT, or Amp's short-lived token for the account |
| CI | yes | only with copied keys | yes, Actions OIDC | yes | yes |
| Amp orbs | person's token | person's token | yes, no person's token | yes | yes, if Amp uses the account |
| Cursor cloud | no | no | yes | yes | secret, visible |
| Claude / Codex on the web | no | no | network secret, not visible | network secret | secret, visible |
| Reply approval (#481) | per-repo App | yes | yes | yes | no |
| Bypass for #572 | per-repo App | one App | one App | one App | user bypass unverified |

**What each option removes for the person.**
- Option 1 removes the browser and the Install click from every repository after the first.
- Option 2 removes that as well as every key in a product repository and the broker dispatch. Agents in Actions, Amp and Cursor get the bot without touching the person's token.
- Option 3 removes even the first setup, but only for other users.
- Option 4 removes the App and adds a password, two-factor authentication, and a PAT to rotate.
- Under every option, an orb still holds the person's token until Amp is connected to a non-approver account, because Amp has one GitHub connection per user. That founder step stays, but only once.

**From a phone and a cloud terminal, per option.**
- **Options 1 to 3:** creating the App needs Bug #578. The Install click is a github.com page a phone can open.
- **Option 2 also needs:**
  - the Key Vault and its access, which `az` sets up with a device-code login (a phone can complete it);
  - the App key imported from the code conversion straight into Key Vault, never written to disk;
  - the hostname, which is a Cloudflare dashboard page.
  
  So option 2's one-time setup is a terminal plus a phone, with no desktop step. Unverified: whether the aigateway VM is reachable for this setup from a cloud terminal; the Azure portal and Cloud Shell are the fallback.
- **Option 4:** creating the machine account and its two-factor authentication works in a phone browser. The classic PAT is a github.com page.

**Public Octo-STS instead of self-hosting.**
- Setup would be about 5 minutes.
- But it trusts Chainguard with a broad key that has no support.
- It writes as the shared `octo-sts[bot]`, not `aayudha-<owner>`.
- Its App cannot be narrowed to aayudha's six permissions. Rejected.

**Cost of waiting.**
- Each new repository costs about 15 minutes of browser setup.
- #572's relief and approval by reply stay off wherever the per-repo App was skipped. On this repository that is today, since agents run as `identity:human-fallback`.

**Pre-mortem for option 2.**
1. The exchange is down, so agents cannot write. Mitigation: aayu falls back to the broker workflow or to `identity:human-fallback`, tagged in the audit, never silently.
2. A loose policy, such as a pattern that matches too much (the Octo-STS advisory GHSA-mwqh-2vg8-rhj3). Mitigation: exact subjects only, Octo-STS v0.12.0 or later, and `.github/chainguard/` in the always-a-person set from #572.
3. An agent edits its own policy. Mitigation: the same protected path, with the App kept off that ruleset's bypass list.
4. Amp's issuer keeps minting tokens for a removed workspace member for up to 24 hours (Amp docs). Mitigation: policies name the project, and revocation goes through the policy file.

**Questions that would change the answer.**
- If the founder will not run an internet-facing service, choose option 1, with CI keeping one key per repository.
- If Octo-STS cannot serve a personal-account installation, choose option 2 with a small exchange of our own: about 400 lines that verify a JWKS and mint a token. The alternative is moving the repositories to a free organisation, which also gives organisation secrets and bypass lists.
- If aayudha gains outside users this quarter, choose option 3, built on option 2.

### Recommendation

Option 2, because it is the only one that removes the per-repository key and gives agents a bot token without the person's token, in CI, Amp and Cursor alike. It also keeps the one-App identity that #481 and #572 need. Option 1's reuse of one App and its API install are option 2's first step, so nothing is wasted.

Build it in three stories:
1. One App per owner, with repositories added by API (`init --identity` reuses the App).
2. The exchange deployed with Key Vault, and trust policies written by `init`.
3. `Resolver.Bot` learns the exchange for Actions, Amp and Cursor; the workflows switch from `actions/create-github-app-token`; `doctor` checks the policy.

Run a one-hour spike first: an Octo-STS installation on a personal account, and an `Integration` bypass actor on a personal repository's ruleset.

The door: two-way. The App stays the identity, and only the way tokens are minted changes. It is marked not reversible below so that nothing is built or applied before the founder accepts it.

### Reversible

false
````
