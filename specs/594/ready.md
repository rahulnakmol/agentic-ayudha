# rahulnakmol/agentic-ayudha#594: Create the owner's aayudha App once from a phone, with its key converted straight into the token exchange's Key Vault

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `0680025801c22eef12314010ed40d7512c21d01c32a6c974f33491bc20668668` |
| Completion Goal hash | `ce1c50121cf01112a1f0fc7d1b9db62693c209c22f85d72bd1187f7a9e6eac60` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/594/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: aayudha-exchange with a fake GitHub and a fake Key Vault, a setup state registered with an Amp OIDC token whose subject is in the allowlist, and GitHub's redirect reaching `/setup/callback` with the code and that state; expected: one conversion call, one Key Vault key import, a record of the App's ID, slug and client ID; the PEM appears in no response, log line, or file; the callback page loads no third-party resource, sends `Referrer-Policy: no-referrer`, and links the App's Install page
2. [api-trace] state: the callback with an unknown or already used state, and a setup request whose OIDC subject is not in the allowlist; expected: no conversion call, no Key Vault write, and a 4xx answer naming the reason
3. [api-trace] state: the conversion answered 422; expected: no Key Vault write; the page and `GET /v1/setup/{state}` say not to install the App and to delete it at its settings page
4. [api-trace] state: `aayu init --identity` with `[identity] exchange` set, in a shell whose browser cannot reach loopback, and the fake exchange completing setup; expected: the printed start link is HTTPS; the manifest's `redirect_url` is on the configured exchange's host, never loopback or a third-party host; aayu prints the Install link only after the exchange reports the App; when no result arrives before the wait ends, exit 4 with a next step
5. [api-trace] state: `aayu init --identity` on a second repository of the same owner, with the fake exchange reporting the App and an installation that covers that repository, then one that does not; expected: no manifest flow and no secret write; aayu reports the installation as covering the repository, or prints the github.com installation settings link to add it

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want to create each owner's single `aayudha-<owner>` GitHub App once, from my phone, with its private key converted and stored server-side in the token exchange's Azure Key Vault. Then no product repository, terminal, or agent ever holds the key (Decision #576, option 2, story 1 of 3). This Story adds `cmd/aayudha-exchange`, the exchange's setup service, which runs next to Octo-STS on the founder's infrastructure:

- `POST /v1/setup` registers a one-time setup state. It is authorised by a host OIDC token whose issuer and subject are in the service's setup allowlist.
- `GET /setup/callback` is the manifest's `redirect_url`. It checks the state, converts the code at once with `POST /app-manifests/{code}/conversions`, imports the PEM into Key Vault as a non-exportable RSA key, and records the App's ID, slug, and client ID. It then shows the Install link.
- `GET /v1/app?owner=` reports the App and whether its installation covers a repository.

`aayu init --identity` uses it when `[identity] exchange` is set: an HTTPS start link, the exchange's callback as `redirect_url`, then polling for the result. A second repository of the same owner reuses the App with no manifest flow. Without an exchange, Bug #578's github.com paste flow applies.

### Completion Goal

1. [api-trace] state: aayudha-exchange with a fake GitHub and a fake Key Vault, a setup state registered with an Amp OIDC token whose subject is in the allowlist, and GitHub's redirect reaching `/setup/callback` with the code and that state; expected: one conversion call, one Key Vault key import, a record of the App's ID, slug and client ID; the PEM appears in no response, log line, or file; the callback page loads no third-party resource, sends `Referrer-Policy: no-referrer`, and links the App's Install page
2. [api-trace] state: the callback with an unknown or already used state, and a setup request whose OIDC subject is not in the allowlist; expected: no conversion call, no Key Vault write, and a 4xx answer naming the reason
3. [api-trace] state: the conversion answered 422; expected: no Key Vault write; the page and `GET /v1/setup/{state}` say not to install the App and to delete it at its settings page
4. [api-trace] state: `aayu init --identity` with `[identity] exchange` set, in a shell whose browser cannot reach loopback, and the fake exchange completing setup; expected: the printed start link is HTTPS; the manifest's `redirect_url` is on the configured exchange's host, never loopback or a third-party host; aayu prints the Install link only after the exchange reports the App; when no result arrives before the wait ends, exit 4 with a next step
5. [api-trace] state: `aayu init --identity` on a second repository of the same owner, with the fake exchange reporting the App and an installation that covers that repository, then one that does not; expected: no manifest flow and no secret write; aayu reports the installation as covering the repository, or prints the github.com installation settings link to add it

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Run the aayudha-exchange handler tests and the `aayu init --identity` tests in internal/cli against a fake GitHub, a fake Key Vault, and a fake exchange; read the requests, the records, and the exit codes.

### Environment

local checkout and this repository's CI build job on a pull request; fakes shaped from GitHub's REST reference (app manifests, apps, installations) and Azure Key Vault's key import API, read 2026-10-18; no live App or vault

### Checkpoints

1. aayudha-exchange setup: state registration with OIDC, the callback's conversion, and the Key Vault import (criteria 1, 2, 3)
2. `aayu init --identity` through the exchange, and reuse of the owner's App on later repositories (criteria 4, 5)

### Links

- depends-on #578

### Assumptions

- Octo-STS (v0.12.0 or later, Apache-2.0) is the exchange. It reads the key from the same vault (`KMS_PROVIDER=akv`). This service only creates the App and stores its key, and it refuses a second setup for an owner that already has one.
- Deploying both services on the founder's Azure infrastructure is a separate change to rahulnakmol/aigateway: systemd units, the Caddy route, the tunnel hostname, and Key Vault with the VM's managed identity. The founder or that repository's deploy process applies it; this Story ships only code and tests here.
- The spike on 2026-10-18 found that Octo-STS serves a personal-account installation, in code and in a fake-GitHub test at 082d66c. It also found that GitHub accepts an Integration bypass actor on a personal repository's ruleset (201; a non-installed App gets 422). Neither has been proven live with this App yet.

### Provenance

Request: "Then build the stories in order, with #578 (phone-safe App creation) in scope."
Vague: "phone-safe" → criteria 1, 2, 3, 4
1. stated (Decision #576): "the App key imported from the conversion straight into Key Vault, never written to disk"
2. default (security/setup): only an allowlisted host identity can register a setup state, and a state is used once (docs/guide/how-trust.md, untrusted input)
3. stated (security/manifest code): "whoever converts the code first within the hour gets the App's private key"
4. stated (security/manifest code): "prefer a design where the code never reaches a page aayu doesn't control"; Decision #576: "the one-time setup works from a phone and from a cloud agent's terminal"
5. default (correctness/reuse): a second repository of the same owner reuses the App with no manifest flow (Decision #576: option 1's reuse of one App and its API install are option 2's first step)
Term: "exchange" → the self-hosted Octo-STS token exchange of Decision #576, option 2; "the stories in order"
````
