# rahulnakmol/agentic-ayudha#596: Agents and CI get the bot token from the token exchange with their host's OIDC token

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `d04969fe634af99ce0c07b33e9fbb2247a4b3a679dda8e5fddacc2addbbb6c5e` |
| Completion Goal hash | `d24a7b0e677aa048bea06a1e053122f9b687896facddc39e54c43664b88c6af1` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/596/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu status --json` in a fake Actions environment (an OIDC request URL and token) with `[identity] exchange` set and a fake exchange; expected: one OIDC request with the exchange's audience; one exchange request with `scope=<owner>/<repo>` and `identity=aayudha-ci`; GitHub calls made with the minted token; one revoke at exit; the token appears in no output or log
2. [api-trace] state: the same with a fake `amp orb id-token` on PATH and no Actions variables; expected: `amp orb id-token` called with the exchange's audience, and the exchange called with `identity=aayudha-agent`
3. [api-trace] state: the exchange answers 403 for the policy; expected: aayu exits 4, naming the identity, the repository, and the exchange's reason, and makes no write with any other token
4. [api-trace] state: no workload identity is available (no Actions variables, no `amp`, no Cursor socket); expected: `aayu auth status` names the bot identity as `identity:human-fallback` and says no workload identity was found
5. [api-trace] state: `aayu init --gates` with `[identity] exchange` set; expected: the written aayudha workflows request `id-token: write`, mint through the exchange, and reference no `AAYUDHA_APP_PRIVATE_KEY`; `aayu doctor` warns on a repository that still has that secret

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As an agent lane or a CI job, I want aayu to get the bot token from the token exchange by presenting my host's OIDC token, so that no repository holds the App key and no agent uses the person's token to get a bot token (Decision #576, option 2, story 3 of 3). With `[identity] exchange` set, `identity.Resolver.Bot` gets its token from the exchange:

- **In GitHub Actions:** it requests the Actions OIDC token with the exchange's audience and calls the exchange with `identity=aayudha-ci`.
- **In an Amp orb:** it runs `amp orb id-token` with that audience and calls it with `identity=aayudha-agent`.
- **In a Cursor cloud agent:** it uses the agent socket's OIDC token.

Each token is narrowed to the current repository. It is never printed, and aayu revokes it on exit (`DELETE /installation/token`). The aayudha workflows switch from `actions/create-github-app-token` to this path (`id-token: write`). `init --identity` stops writing `AAYUDHA_APP_PRIVATE_KEY`, and `doctor` warns while a repository still holds that secret. On a host with no workload identity (Claude Code or Codex on the web), aayu says so and uses `identity:human-fallback`, as today.

### Completion Goal

1. [api-trace] state: `aayu status --json` in a fake Actions environment (an OIDC request URL and token) with `[identity] exchange` set and a fake exchange; expected: one OIDC request with the exchange's audience; one exchange request with `scope=<owner>/<repo>` and `identity=aayudha-ci`; GitHub calls made with the minted token; one revoke at exit; the token appears in no output or log
2. [api-trace] state: the same with a fake `amp orb id-token` on PATH and no Actions variables; expected: `amp orb id-token` called with the exchange's audience, and the exchange called with `identity=aayudha-agent`
3. [api-trace] state: the exchange answers 403 for the policy; expected: aayu exits 4, naming the identity, the repository, and the exchange's reason, and makes no write with any other token
4. [api-trace] state: no workload identity is available (no Actions variables, no `amp`, no Cursor socket); expected: `aayu auth status` names the bot identity as `identity:human-fallback` and says no workload identity was found
5. [api-trace] state: `aayu init --gates` with `[identity] exchange` set; expected: the written aayudha workflows request `id-token: write`, mint through the exchange, and reference no `AAYUDHA_APP_PRIVATE_KEY`; `aayu doctor` warns on a repository that still has that secret

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Run the identity, auth, init and doctor tests in internal/identity and internal/cli against a fake exchange, a fake Actions OIDC endpoint, a fake `amp` command, and a fake GitHub; read the requests and outputs.

### Environment

local checkout and this repository's CI build job on a pull request; Octo-STS's exchange API at 082d66c (`GET /sts/exchange?scope=&identity=` with a bearer OIDC token), GitHub Actions' OIDC request API, and `amp orb id-token` as observed on 2026-10-18

### Checkpoints

1. `Resolver.Bot` through the exchange for Actions, Amp, and Cursor, with revocation (criteria 1, 2, 3, 4)
2. The workflows and `init`/`doctor` without the App key (criterion 5)

### Links

- depends-on #595

### Assumptions

- Octo-STS takes only OIDC tokens, so Claude Code and Codex on the web, which issue none a third party can verify, keep `identity:human-fallback` until a later change adds a path for them. #576's network-secret path for them is not built here.
- Cursor's socket token is covered by the same code path as Amp's, but it is tested only against a fake.

### Open questions

- deferred: "phone-safe": App creation from a phone belongs to story 1 of Decision #576 and Bug #578; this Story has no setup step

### Provenance

Request: "Then build the stories in order, with #578 (phone-safe App creation) in scope."
Vague: "phone-safe" → deferred: it is about creating the App, which is story 1 of 3; this Story has no setup step on a phone
1. stated (Decision #576): "CI presents its Actions OIDC token (`id-token: write`)."
2. stated (Decision #576): "An Amp orb presents `amp orb id-token`."
3. default (security/no fallback): a refused exchange never falls back to another token silently (docs/guide/how-trust.md, the fallback tag)
4. stated (Decision #576): "Claude Code on the web (Anthropic-hosted) and Codex on the web issue no token a third party can verify."
5. stated (Decision #576): "the workflows switch from `actions/create-github-app-token`"
Term: "exchange" → the self-hosted Octo-STS token exchange of Decision #576, option 2; "the stories in order"
````
