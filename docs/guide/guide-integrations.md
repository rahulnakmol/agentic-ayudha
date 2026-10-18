# Bring in analytics and errors: PostHog and Sentry

A Goal is only as good as its number, and "did it help?" needs the same number again after the release. If your
product's usage lives in PostHog and its errors in Sentry, aayu can read both: it fetches a Goal's baseline instead of
anyone typing it, measures the Goal the same way later, and reads experiments out. It only reads. The key you give it
is checked before any data call, and a key that could write is refused.

![PostHog and Sentry connect through a read-only key that aayu checks first. They feed grill's baseline at Shape and measure at Learn; PostHog feeds experiment, and PostHog or GrowthBook feeds readout. In production, investigate and runbook write bounded query templates that a person runs. aayu never writes to either.](media/integrations.svg)

*Data comes in read-only. Nothing goes back.*

> **Honest status:** both adapters are tested against synthetic replies written from PostHog's and Sentry's API
> references, request by request. Neither has been run against a real PostHog or Sentry account yet: the live tests
> skip without keys. The GitHub adapter has passed its live test.

## When to use which

- **A Goal's baseline from product usage**, such as people who signed up: connect PostHog. [`grill`](skills/grill.md)
  fetches it while it shapes the Goal, and [`measure`](skills/measure.md) measures it again after the release.
- **A Goal's baseline from reliability**, such as crash-free sessions: connect Sentry. The same two verbs read it.
- **Whether your change caused the difference:** connect PostHog. [`experiment`](skills/experiment.md) reads its
  baseline rate from PostHog only, and [`readout`](skills/readout.md) reads PostHog or GrowthBook.
- **A production incident:** connect neither. [`investigate`](skills/investigate.md) and [`runbook`](skills/runbook.md)
  write bounded `hogql` and `sentry` query templates, and a person runs them.

## Connect PostHog

**1. Create a read-only key.** In PostHog, create a personal API key with only the `query:read` scope, limited to your
product's project. aayu asks PostHog which scopes the key has, and refuses it if any scope does not end in `:read`.

**You should see** the new key in PostHog's personal API keys list, showing `query:read` and nothing else. Copy its
value now; PostHog shows it once. Stuck? See [integration problems](troubleshooting.md#integrations).

**2. Add the config section and store the key.** In your own terminal:

```sh
aayu init --integrations
aayu auth set-secret posthog
```

`init --integrations` offers each service in turn; say yes to PostHog. It writes the section below to
`.aayudha/config.toml`. Add your numeric project ID by hand. `auth set-secret` asks for the key on one line and
stores it in your secrets store; it is for people only and refuses agents.

```toml
[integrations.posthog]
secret = "posthog" # set the value with: aayu auth set-secret posthog
scope = "read"
project = "12345"  # your PostHog project ID
# url = "https://eu.posthog.com"  # EU Cloud or self-hosted; the default is https://us.posthog.com
```

**You should see** `stored posthog in the <backend> backend` from `auth set-secret`, and the section above, with your
project ID, in `aayu config show`. Stuck? See [integration problems](troubleshooting.md#integrations).

**3. Check it.**

```sh
aayu doctor
```

**You should see** a line like this under integrations, with your key's label:

```text
posthog: reachable as personal API key <label>; read-only scope verified (query:read per GET /api/personal_api_keys/@current/)
```

A write-capable key shows `reachable, but the key is write-capable` instead, and every read verb refuses it. Stuck?
See [integration problems](troubleshooting.md#integrations).

**What each verb reads:**

- `grill` and `measure` count **events** or **users** (distinct persons) with one HogQL query over a window. Name the
  metric in a Goal as `posthog:users event = 'signed_up'`: the part after the metric is a HogQL filter. The query, the
  window, and the population are written next to the number, so anyone can run it again.
- `experiment` reads the baseline rate of your conversion event, and `readout` counts the people and conversions in
  each arm of your feature flag.

## Connect Sentry

**1. Create a read-only token.** In Sentry, create an auth token with the `org:read`, `event:read`, and
`project:read` scopes and nothing that writes. The metrics come from your organization's sessions and events
endpoints, which need `org:read`. aayu asks Sentry for the token's scopes and refuses any that does not end in
`:read`.

**You should see** the new token in Sentry's auth tokens list with those three scopes. Copy its value now; Sentry shows
it once. Stuck? See [integration problems](troubleshooting.md#integrations).

**2. Add the config section and store the token.** Say yes to Sentry in `aayu init --integrations`. Its prompt
lists only `event:read` and `project:read`; use the three scopes from step 1. Then:

```sh
aayu auth set-secret sentry
```

Add your organization slug and the numeric project ID to the section `init` wrote:

```toml
[integrations.sentry]
secret = "sentry" # set the value with: aayu auth set-secret sentry
scope = "read"
org = "your-org"   # the Sentry organization slug
project = "67890"  # the numeric project ID
# url = "https://sentry.example.com"  # the EU region or self-hosted; the default is https://sentry.io
```

**You should see** `stored sentry in the <backend> backend` from `auth set-secret`, and the section, with your org and
project, in `aayu config show`. Stuck? See [integration problems](troubleshooting.md#integrations).

**3. Check it** with `aayu doctor`. **You should see** `sentry: reachable as …; read-only scope verified (…)`, listing
the token's scopes. Stuck? See [integration problems](troubleshooting.md#integrations).

**What each verb reads:** `grill` and `measure` read four metrics: `crash_free_sessions` and `crash_free_users` (rates
from 0 to 1), and `errors` and `users_affected` (counts). Name one in a Goal as `sentry:crash_free_sessions`; anything
after the metric is a Sentry search query that narrows it.

## What aayu never does

- It never writes to PostHog or Sentry: no flags, no annotations, no issues, no comments. Create an experiment's
  feature flag in PostHog or GrowthBook yourself.
- It never sends a key whose configured scope is `write`, and it refuses a key the provider reports as able to write,
  before the data call.
- It never types a provider's baseline: when a Goal names a PostHog or Sentry metric and the provider cannot answer,
  `grill` refuses to fill the baseline in, and the message says why. A baseline someone typed into the Goal by hand
  is not re-checked against the provider by `aayu ready`. A built-in snapshot metric, such as activation or retention,
  needs no integration: its Baseline is the value `aayu measure` measured from your exports.
- During an incident, it never queries PostHog or Sentry for you. `investigate` and `runbook` write the query, with a
  time window and a row limit, for a person to run.

## If you also connect PostHog's or Sentry's own MCP server

Your agent may have PostHog's or Sentry's own MCP server or CLI connected beside aayu. In Claude Code, Codex, Cursor,
and Amp, the hooks that `aayu install` sets up judge each of their tool calls and CLI commands before it runs, and log
it after:

- **Reads pass.** aayu keeps a read allowlist pinned to each provider's published tool definitions. It also looks
  inside PostHog's single `exec` tool and Sentry's `execute_sentry_tool`.
- **Writes are refused.** Changing a flag, launching an experiment, resolving an issue, or creating an alert or a
  release is a catalog action a person approves. A tool aayu does not know is refused too, and so is one that spends
  paid AI credits, such as Sentry's Seer analysis.
- **Personal data is opt-in for MCP tools.** PostHog's person, support-ticket, and raw LLM conversation tools, and
  Sentry's replay content, attachment, and AI conversation tools, stay refused until `.aayudha/config.toml` sets
  `allow_pii = true` under `[integrations.posthog]` or `[integrations.sentry]`. aayu reads that setting only from the
  repository's config file, not from your user config or the environment.
- **Reads are logged.** The post-tool hook appends each read it let through to
  `.aayudha/state/calls/<session>.jsonl`: the tool, its arguments, the window and row limit it
  asked for, the rows returned, and a SHA-256 of the result, never the result itself. A call that names a window and
  a row limit is citable as evidence, like aayu's own queries; the log says why any other call is not.

**You should see** a refusal that starts with `aayudha:` when an agent tries a write through the provider's MCP
server, with the reason and the route a person takes instead, and a new line in `.aayudha/state/calls/` for each read.
In Codex, trust the three aayudha hooks in `/hooks` first, or none of this runs.

> **Honest status:** the hook has known gaps.
>
> - It recognises a provider's MCP server by name. In Claude Code, Codex, and Amp the registered name must contain
>   `posthog` or `sentry`; in Cursor its URL or command must. Keep the provider's name in it.
> - It sees a provider CLI only when it is the command itself. One wrapped in a subshell, such as
>   `sh -c 'sentry-cli releases new 1.2.3'`, is not checked.
> - `allow_pii` gates MCP tools only, and only those on the lists. PostHog's session-recording tools pass without
>   it, and so does a CLI read that returns personal data, such as `sentry replay view`.
> - `allow_pii` is read from the working tree, so an uncommitted edit takes effect at once. CODEOWNERS review applies
>   only when the change is merged.
> - The call log redacts GitHub, OpenAI, and Slack tokens, not PostHog or Sentry ones. Keep provider tokens in the
>   CLI's config or your environment, never typed on the command line.
>
> Keep the provider's own setup read-only as well: add `?mode=tools&readonly=true` to PostHog's MCP URL, and give
> Sentry's MCP server a read-only token with `skills=inspect`. The full rules are in
> [security](how-security.md#the-main-threats-and-their-controls).

## The other integrations

Every adapter in the catalog only reads: GitHub (delivery baselines from issues and pull requests), GrowthBook
(experiment results), Azure Application Insights, AWS CloudWatch RUM, and the Google Analytics 4 export in BigQuery
(baselines, and `measure --live`), Figma (design files), and Mobbin (design references). Where the provider reports
what a key can do, as PostHog, Sentry, Azure, Google Cloud, and GitHub's classic tokens do, aayu checks it first.
GrowthBook keys and GitHub's fine-grained and App tokens report no scopes, so aayu trusts `scope = "read"` in your
config: give them read-only access. GrowthBook has no
step-by-step guide here yet. The clouds connect with no stored key: see [Azure](../getting-started-azure.md),
[AWS](../getting-started-aws.md), and [Google Cloud](../getting-started-gcp.md).

## What can go wrong

- **`the posthog key is write-capable` or `the sentry key is write-capable`.** The key has a scope that does not end
  in `:read`. Create a new key with only the scopes above, then run `aayu auth set-secret posthog` or
  `aayu auth set-secret sentry` again.
- **`[integrations.posthog] project is not set`, or the Sentry `org` or `project` is wrong.** Add the field by hand to
  `.aayudha/config.toml`. Sentry wants the organization slug and the numeric project ID, not the project's name.
- **`secret "posthog" has no value`.** The key was never stored on this machine. Run `aayu auth set-secret posthog`
  in your own terminal.
- **A 403 from Sentry's sessions or events.** The token lacks `org:read`. Add it, or create a new token.
- **A 401, or no data, from the right key.** Your account is on another region or is self-hosted: set `url` in the
  section, for example `https://eu.posthog.com`.
- **`no data?` for a Sentry metric.** The project had no sessions or events in the window. Check that release health
  is on for the project, or pick another metric.

## Under the hood

Each adapter is in `internal/integrations/<name>/`, and the catalog verbs and `doctor` use is
`internal/integrations/providers/providers.go`. How a baseline is recorded so it can be reproduced is in
[did it work?](guide-learn.md), and how to add an adapter is in [extending aayudha](how-extending.md).

## What's next

- Shape a Goal with a fetched baseline: [`grill`](skills/grill.md).
- Measure it after a release: [did it work?](guide-learn.md).
