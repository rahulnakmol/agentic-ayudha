# rahulnakmol/agentic-ayudha#449: Decision: native PostHog and Sentry support: capability map, reach, and phased plan

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, decision |
| Body hash | `376e2951510b5d2470ad9102c704d371820ee2b966a06a2cbe5e987c6bab67b2` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/449/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Question

How should aayu support PostHog and Sentry for inspecting, reviewing, and improving a product, and how should it behave when the host (Claude Code, Codex, Cursor, Amp) already has the PostHog MCP, the Sentry MCP, or their CLIs connected? The founder asked on 2026-10-17: "Aayu should have native support to all features PostHog and Sentry provide which can be used to inspect, review and improve the product", and whether aayu works when a host has those MCP servers or CLIs connected. This Decision settles which capabilities aayu reaches, how (native adapter, host MCP, or CLI), which seat and verb uses each, and the build order, before any of it is built. The trust model is fixed and not up for decision: integrations read by default; every write goes through the action catalog with a person's approval; provider output is untrusted data; every call is logged and cited (tool, window, row limit); host MCP tools sit behind read-only allowlists; keys carry the least scope and doctor checks them.

**Does aayu work when a host has the PostHog or Sentry MCP or CLI connected? Today (dev `104233e`, verified): it runs next to them, but it does not govern them.**

- aayu's own adapters use their own read-only keys (`aayu auth set-secret`, human-only), never the host's MCP or CLI credentials, so nothing conflicts (`internal/integrations/integrations.go`, `Resolve`).
- No skill calls a host MCP tool or a provider CLI. `investigate` and `runbook` only print a `hogql` or `sentry` template for a person to run (`skills/investigate/SKILL.md` §3; `internal/production/runbook.go:26`).
- **Gap:** the pre-tool hook checks only the `[budget]` pause and human-only `aayu` commands (`internal/hosts/hook.go`, `RunHook` case `pre-tool`). An agent in that host can call a write tool (PostHog `update-feature-flag`, `experiment-launch`; Sentry `update_issue`, `create_alert_rule`) or run `sentry-cli releases new` or `posthog-cli api call …` without the catalog or a person's approval. Nothing is logged or citable. The Amp plugin calls `aayu hook pre-tool` without passing the tool event at all (`hosts/amp/aayudha.ts:128`).
- **Two provider facts make a name allowlist harder.** PostHog's MCP defaults to "CLI mode" in Claude Code and Codex: a single `exec` tool through which every read and write passes. Sentry's hosted MCP requests write scopes (`project:write team:write event:write alerts:write`) through OAuth whichever skills you pick.

Why now: the gap above exists today for anyone who connects those servers, and four seats (Production Engineer, Data & AI Architect, Revenue Architect, UX) are waiting on the same evidence.

### Options

1. **Hybrid, governance first (recommended).** First govern host MCP and CLI calls: read-only allowlists enforced by the hooks, and every call logged and citable. Then grow the native read-only adapters, seat by seat, for the evidence each seat's checks and gates rely on. Use the host MCP and CLI, under the allowlist, for breadth and exploration. Every write is a catalog action.
2. **Native everything.** A native REST adapter for every capability in the map, and no host MCP use.
3. **Host MCP and CLI only.** Keep the native adapters as they are; skills use whatever MCP or CLI the host has connected, behind allowlists.
4. **Docs only.** Document today's support (the docs overhaul's integrations guide) and change nothing.

### Evaluation

**Capability map.** Every PostHog and Sentry capability that helps inspect, review, or improve a product, from the official docs, API schemas, MCP tool definitions, and CLIs, read on 2026-10-17. The sources: PostHog's OpenAPI spec, `NodeKind`, scopes, and MCP `tool-definitions-all.json` at `8bb4013`, 1,117 tools of which 601 are annotated read-only; Sentry's `sentry-api-schema`, permissions, rate-limit, and pricing docs, the `sentry-mcp` toolkit at `5044167` (76 tools), and `sentry-cli` 3.8.0. The full research note, with every URL and the 1,117-row PostHog tool table, is in the private research repository: `rahulnakmol/agentic-ayudha-research`, `posthog-sentry-capabilities-2026-10.md`.

Legend. **R/W**: what the capability does at the provider. **Reach**: N = aayu's native read-only adapter (`internal/integrations/<provider>`, reproducible source query, works in CI and loops), M = a host-connected MCP server behind aayu's read-only allowlist, C = the provider CLI behind the same allowlist, K = the ops action catalog with a person's approval (every write). **Seat**: PE Production Engineer (L07), DA Data & AI Architect (L05, L08), RA Revenue Architect (no layer), UX UX Researcher & Evolutionist (L01), DX DevOps & DevEx Tinkerer (L09). Evidence is cited as `query qN` from the call log (tool, window, row limit, rows returned, result digest). Phase numbers refer to the plan below.

#### PostHog

| Capability | R/W | Reach | Verb · seat · layer | Evidence it yields | Cost and quota | Phase |
|---|---|---|---|---|---|---|
| HogQL / SQL (`HogQLQuery`, `query:read`) | R | N (today: baselines, readout arms), M `execute-sql`, C `posthog-cli exp query run` | measure, chart · DA · L08; investigate · PE · L07 | Any bounded aggregate over events, persons, sessions, warehouse | `/query`: 2,400/h and 240/min per project, 3 concurrent, 10 s, 100 rows default (50K max); hourly bytes budget (429 `api_queries_budget_exceeded`); no read fee | today; P1 widens to bounded `hogql:` queries |
| Trends (`TrendsQuery`) | R | N; M `query-trends` | measure, chart · DA · L08 | Time series with breakdowns | Same `/query` budget | P2 |
| Funnels (`FunnelsQuery`, correlation) | R | N; M `query-funnel` | measure · DA; experiment baseline · RA | Step counts and time to convert (rates computed by aayu) | Same | P2 |
| Retention (`RetentionQuery`) | R | N; M `query-retention` | measure (`retention`) · DA · L08 | Cohort × interval counts | Same | P2 |
| Stickiness, lifecycle | R | N; M `query-stickiness`, `query-lifecycle` | measure, chart · DA · L08 | Days active; new, returning, resurrecting, dormant | Same | P2 |
| Paths (`PathsQuery`) | R | M `query-paths`; N later if a gate needs it | review, research · UX · L01 | Journeys between pages or events | Same | P4 |
| Cohorts | R / W | N (definitions and counts); M `cohorts-list`, `cohorts-retrieve`; writes K | measure population · DA | Segment definition and size | Analytics 240/min, 1,200/h per key | P2 |
| Persons and groups | R / W | Aggregates only through HogQL; person rows and M `persons-*` off unless `allow_pii` | grill, measure · DA | Counts; person properties are PII | Same | P2 (aggregates) |
| Session replay: list and metadata (`RecordingsQuery`) | R | N (IDs, URLs, durations, console-error and rage-click counts); M `query-session-recordings-list`, `session-recording-get` | review, research · UX · L01; investigate (affected sessions) · PE | Which sessions hit a route or error, as links, never content | Analytics limit; reads not billed | P4 |
| Session replay: snapshots | R | **Not reached.** Private API, raw DOM, network bodies, console | — | — | — | never |
| Replay Vision and AI summaries | R/W, paid | Off by default; any run is K | — | AI observations (untrusted) | Vision credits (2,500 free/month), AI credits | not planned |
| Heatmaps | R / W | N or M `heatmaps-list`, `heatmaps-events` | review, polish · UX · L01 | Click and scroll density per URL | Analytics limit | P4 |
| Web analytics (`WebOverviewQuery`, `WebStatsTableQuery`, `WebVitalsQuery`, …) | R | N; M `query-web-overview`, `query-web-stats`, `query-web-vitals` | measure, chart · DA; review · UX (vitals per path) | Visitors, bounce, top paths, Core Web Vitals | `/query` budget | P2, P4 |
| Feature flags | R / W | Read N (status, rollout, blast radius, activity) and M `feature-flag-get-*`; enable, disable, roll out K | experiment, readout · RA; respond rollback · PE | Rollout state and history; who a flag change reaches | Analytics limit | P3 read; P5 write |
| Experiments (`ExperimentQuery`, results, exposure, health) | R / W | Read N and M `experiment-get`, `experiment-results-get`; launch, pause, end, ship variant K | readout (cross-check; aayu's own statistics stay the gate) · RA | Variant exposure, PostHog's result, health checks | Billed with flag requests, not reads | P3 read; P5 write |
| Surveys | R / W | Read N (stats, responses as untrusted text) and M `survey-get`, `survey-stats`; create, launch, stop K; `summarize_responses` off (LLM) | research (Feedback items) · UX; readout (qualitative) · RA | Response counts and verbatims (PII redacted) | Billed per response captured; reads free | P4 read; P5 write |
| Error tracking (`ErrorTrackingQuery`, issues, stack frames) | R / W | Read N and M `query-error-tracking-*`; assign, merge, resolve K | investigate · PE · L07 | Issue, occurrences by release, frames mapped to `path:line` at the deployed revision | Billed per exception ingested; reads free | P1 read; P5 write |
| Logs (`LogsQuery`) | R | N; M `query-logs` | investigate · PE · L07 | Log lines (redacted, bounded) | Billed per GB ingested; reads free | P1 |
| LLM observability (`TracesQuery`, `$ai_*` events) | R / W | Read N (aggregates: cost, latency, errors by model); M `query-llm-traces-list`; evaluations and prompts K | measure, chart · DA · L05 | Model cost and latency; raw prompts stay out by default | Billed per AI event ingested; reads free | P2 |
| Annotations | R / W | Read N (deploy and incident markers); create K (auto only after the same definition merged as human) | investigate timeline · PE; chart · DA; release marker · DX | Markers on the timeline | CRUD 480/min | P1 read; P5 write |
| Data warehouse (sources, tables, saved views) | R / W | Read N (warehouse tables through HogQL; sync health); setup, resync, materialize K | measure, chart · DA · L08 | Joined business data; freshness of each source | Billed per row synced: a resync spends money, never automatic | P2 read; P5 write |
| Dashboards and saved insights | R / W | M `dashboard-get`, `insight-get`; N runs a saved insight by ID when chart cites it | chart · DA | The team's own curated KPI, cited by ID | Analytics limit | P2 |
| Actions, event and property definitions | R / W | N; M `read-data-schema` | measure, experiment (does the event exist?) · DA, RA | Taxonomy; catches a misspelt metric before a baseline | CRUD limit | P2 |
| Insight alerts and subscriptions | R / W | Read N; create K | runbook (the alert exists) · PE | What is watched, at what threshold | CRUD limit | P1 read; P5 write |
| Revenue analytics (HogQL views) | R | N through HogQL | readout, North Star · RA | MRR, charges, customers | Warehouse rows | P3 |
| Ingestion health (`health_issues/`) | R | N | doctor · DX; measure data trust · DA | Whether the numbers can be trusted | CRUD limit | P0 |
| PostHog AI, notebooks, product tours, CDP, endpoints | R/W | Not reached by aayu (no inspect value, or paid AI); a person may still use them | — | — | AI credits | not planned |

#### Sentry

| Capability | R/W | Reach | Verb · seat · layer | Evidence it yields | Cost and quota | Phase |
|---|---|---|---|---|---|---|
| Issues and events (search, detail, latest event, stack trace, tags, breadcrumbs) | R / W | Read N and M `search_issues`, `get_issue_details`, `get_event_stacktrace`; C `sentry-cli issues list`, `sentry issue list/view`; resolve, assign, ignore, note, link K | investigate · PE · L07 | First and last seen by release, frames mapped to `path:line` at the deployed revision | Issues 10 req/s per user; events 30 req/s, 15 concurrent; reads not billed | P1 read; P5 write |
| Errors aggregate (`events?dataset=errors`) | R | N (today: `errors`, `users_affected`) | measure, grill baselines · DA | Counts and affected users | Same | today |
| Tracing and performance (`dataset=spans`, trace detail) | R | N (p95 by span op, transaction, release; trace tree); M `search_traces`, `get_trace_details` | investigate · PE; slo latency SLI · PE | Latency by revision; N+1 shapes | ≤ 20 fields, `per_page` ≤ 100; spans billed at ingest | P1 |
| Insights (web vitals, queries, HTTP, caches, queues, mobile) | R | N through the spans dataset | review (vitals) · UX; investigate (slow queries) · PE | Vitals and slow dependencies by route | Same | P1, P4 |
| Profiling (flamegraph, function trends) | R | M `get_profile`, `get_profile_details`; N later | investigate (performance regressions) · PE | Hot functions by release | Profile hours billed at ingest | P1 (M only) |
| Logs (`dataset=logs`) and metrics (`dataset=tracemetrics`) | R | N; M `search_logs`, `search_metrics` | investigate, slo · PE | Bounded runtime logs and app metrics | Billed at ingest | P1 |
| Releases and release health (crash-free per release, adoption, deploys, commits) | R / W | Read N (today: crash-free sessions and users) and M `find_releases`, `get_release_details`; create, finalize, deploy, set-commits stay in the product's CI (`sentry-cli`), never an agent | investigate (the deployed revision) · PE; slo · PE; release gate · DX (Consulted) | Crash-free rate by release; the release a symptom started in | Sessions: ≤ 10,000 data points, interval ≥ 1 h | today; P1 widens |
| Crons (monitors, check-ins) | R / W | Read N and M `find_monitors`; create, run K or product CI | runbook (each job's monitor exists) · PE; investigate | Missed and failed jobs | 1 monitor included, then billed | P1 read |
| Uptime (detectors, `dataset=uptime_results`) | R / W | Read N and M `find_uptime_monitors`; create, update K | slo availability SLI · PE; investigate | Check results and outages | 1 monitor included, then billed | P1 read; P5 write |
| Alerts (workflows and detectors; the legacy alert-rule API is gone from the public spec) | R / W | Read N (`alerts:read`) and M `find_alert_rules`, `find_metric_monitors`; create, update, delete K | runbook (the alert the entry names exists and matches) · PE; slo burn-rate alert · PE | What is watched, at what threshold | Not billed | P1 read; P5 write |
| User feedback (`issue.category:feedback`, user reports) | R / W | Read N and M `get_issue_user_reports`; resolve, spam K | research (Feedback items, PII redacted) · UX · L01 | The user's own words, URL, linked replay | Not billed | P4 read |
| Session replay (list, rage and dead clicks) | R | N (IDs, URLs, click and error counts); M `search_replays`, `get_replay_details`; segments not reached | review · UX · L01; investigate · PE | Which sessions hit an error or a route, as links | Replays billed at ingest | P4 |
| Seer: existing analysis (autofix state, summary) | R | N (`GET …/autofix/`, `event:read`); quoted as Seer's opinion, never as cause evidence | investigate · PE | A hypothesis to test, labelled as Seer's | Seer subscription ($40 per active contributor per month) | P1 |
| Seer: start root cause or fix (`POST …/autofix/`, `event:write`) | W, paid | K only, opt-in; M `analyze_issue_with_seer` and `sentry issue explain/plan` denied | — (a proposal through the catalog) | Root cause, plan, possibly a PR | Needs Seer seats; legacy plan $1 per run; 25/min per user, 100/h per org | P5, opt-in |
| Dashboards | R / W | M `find_dashboards`, `get_dashboard_details` | chart (cites the team's dashboard) · DA | Curated KPIs | Not billed | P2 (M only) |
| Suspect commits, ownership, code mappings | R / W | N (ownership, code mappings); committers is private API, M only | investigate (who owns the implicated code) · PE | Route the follow-up Bug | Not billed | P1 |
| Source maps and debug files | R / W | Read N (`source-map-debug` when a trace is unsymbolicated); uploads stay in product CI | investigate · PE | Why a stack trace is unreadable | Not billed | P1 |
| Stats and usage (`stats_v2`, dropped events) | R | N; M `find_dropped_events` | doctor · DX; measure data trust · DA | Quota pressure and dropped data behind a number | Not billed | P0 |
| Feature flag change log (`flags/logs/`) | R | N | investigate (a flag change that lines up with errors) · PE | Flag changes on the timeline | Not billed | P1 |

**How aayu reaches each capability (the rule behind the Reach column).**

- **Native adapter** when the number feeds a check, a gate, or a cited baseline: it is reproducible (the exact source query is recorded, as `Baseline.SourceQuery` is today), it works in CI and scheduled loops where no host exists, and its transport is structurally read-only (`integrations.ReadOnly`). This is the path for every seat's deterministic checks.
- **Host MCP** for breadth and exploration in a person's session: the 600-odd PostHog and 50-odd Sentry read tools aayu will never wrap one by one. Only allowlisted read tools pass; each call is logged; a result from a tool that translates natural language with an LLM (Sentry's `search_*` tools) may be explored but never cited as a gate's number, because the same words can produce a different query.
- **CLI** for the same reads when the person uses the CLI instead (`sentry issue list`, `posthog-cli exp query run`). The write commands (`sentry-cli releases new`, `sourcemaps upload`, `posthog-cli sourcemap upload`, `api call <write tool>`) belong to the product's own CI or the catalog, never to an agent lane.
- **Not reached:** PostHog replay snapshots (a private API holding raw DOM, network bodies, and console output), Sentry replay segments, and paid AI runs (PostHog AI, Replay Vision, survey summaries, starting Seer) unless a person approves them as catalog actions.

**The trust model, rule by rule.**

| Rule | Mechanism |
|---|---|
| Integrations read by default | Native adapters keep `integrations.ReadOnly`. Host MCP and CLI calls go through the pre-tool hook, which allows only allowlisted reads (Phase 0). |
| Every write is a catalog action approved by a person | Provider writes are `ops/actions.toml` actions, run by `aayudha-ops.yml` in a GitHub Environment with required reviewers. The write credential lives only there (`ops/README.md`). The hook refuses a write tool or command and names the catalog action to propose (Phase 0, Phase 5). |
| Provider output is untrusted data | Every result is wrapped as untrusted data, as investigate's grounding does today. Free text (issue titles, log lines, survey answers, feedback, Seer's analysis, LLM traces) is quoted, never followed. Secret-shaped values are redacted (`actions.RedactString`). PII is not read by default: person rows, replay content, and raw prompts are off unless `allow_pii` is set for that integration. |
| Every call is logged and cited like aayu's own queries | One call log per work item, extending investigate's `queries.jsonl` (`internal/cli/investigate.go`, `queryLog`). It records the call's channel (adapter, MCP, or CLI), the host, the tool or endpoint, its arguments (redacted), the window, the row limit, the rows returned, the result's SHA-256, and quota headers. Each entry is citable as `query qN`. A host MCP or CLI call is logged by a new `post-tool` hook. A call with no window or no row limit is logged but cannot be cited as evidence. |
| Read-only allowlists for host MCP tools | The allowlists are generated from the providers' own tool definitions, pinned by commit: PostHog `readOnlyHint`, minus AI-consent tools, minus read-labelled tools that need a write scope, minus `persons-*` unless `allow_pii`. For Sentry, the `inspect` skill minus `analyze_issue_with_seer`, with `execute_sentry_tool` judged by its `name` argument. PostHog's `exec` (CLI mode) is refused: doctor asks for `mode=tools&readonly=true`. A project may narrow the list in config, never widen it to a write. |
| Least-scope keys, checked by doctor | Doctor lists the read scopes each enabled capability needs, reports any that are missing, and refuses any write scope, as today. It adds a row for host MCP registrations: PostHog `readonly=true`, `mode=tools`, and a pinned project. For Sentry, `skills=inspect`, Seer disabled, and an org or project path. Hosted OAuth for Sentry is reported as write-capable; doctor recommends a read-only token over `Sentry-Bearer` or stdio `@sentry/mcp-server --skills=inspect`. Stored CLI credentials get the same scope check. |

**Phases.** Each phase is a set of Stories (`grill story --create`, `ready`, `build`), built in small PRs to `dev`. Each capability gets recorded-fixture tests (as in `internal/integrations/providers/testdata/`), a live proof against a real account when one is available (the env-gated pattern of `providers/live_test.go`; otherwise the PR records honest status: fixture-tested, not live-proven), and docs in the integrations guide, coordinated with the docs overhaul.

| Phase | Accountable seat · verbs | What it delivers | Key scopes it adds |
|---|---|---|---|
| **P0: govern host MCP and CLI** | DevOps & DevEx Tinkerer · `hook`, `doctor`, `install`. Code Architect Consulted (L06 `hosts/**`); Production Engineer Consulted (L07 config and secrets) | The pinned allowlists. The pre-tool hook decides MCP tools and provider CLI commands, and the Amp plugin passes the event. A new `post-tool` hook writes the call log, and native adapters write to the same log. The doctor rows for host MCP registrations, CLI credentials, ingestion health, and Sentry dropped events. `aayu install --mcp posthog,sentry` writes the read-only registration for each host. `internal/integrations/**` joins L06 (it is in no layer today, per `verbs.LayerTable`). | none (aayu reads no new data) |
| **P1: live evidence for production** | Production Engineer · `investigate`, `runbook`, `slo` | `investigate --query sentry:"…"` and `posthog:"…"` run live, bounded (≤ 7 days, ≤ 500 rows, as Decision #324 bounds export queries), logged as `qN`. Sentry: issues and events with frames mapped to `path:line` at the deployed revision; spans, logs, and metrics aggregates; trace detail; release health per release; crons; uptime; alerts (workflows and detectors); ownership and code mappings; source-map debug; flag change log; existing Seer analysis, labelled as Seer's. PostHog: error tracking, logs, annotations, insight alerts, bounded HogQL. `runbook` checks that the alert or monitor an entry names exists. `slo` computes crash-free, availability, and latency SLIs from Sentry. The templates stay as the fallback when nothing is configured. This amends #324's telemetry paragraph for Sentry and PostHog. | Sentry `org:read project:read event:read alerts:read`; PostHog `query:read error_tracking:read logs:read annotation:read alert:read` |
| **P2: product analytics for measure and chart** | Data & AI Architect · `measure`, `chart` | PostHog trends, funnels, retention, stickiness, lifecycle, and web analytics as `posthog:<metric>` provider metrics and chart sources. Cohort counts. A taxonomy pre-check (the event exists). Data warehouse tables and source freshness. A saved insight cited by ID. LLM observability aggregates (cost, latency, errors by model). Sentry metrics and `stats_v2` data-trust notes. | PostHog `insight:read cohort:read event_definition:read property_definition:read web_analytics:read llm_analytics:read warehouse_table:read external_data_source:read` |
| **P3: experiments and readout** | Revenue Architect · `experiment`, `readout` | `experiment` checks that the flag and the event exist before it files. `readout` reads the experiment's exposure, health, and PostHog's own result, and shows them next to aayu's statistics. aayu's statistics stay the gate, and a disagreement is flagged. Flag status and blast radius. Revenue views for the North Star. | PostHog `feature_flag:read experiment:read` |
| **P4: experience evidence for review and research** | UX Researcher & Evolutionist · `review`, `research` | Replay metadata from both providers (IDs, URLs, rage and dead clicks, console errors, never content). Heatmaps, paths, and web vitals per path, for the screen under review. Survey answers and Sentry user feedback become Feedback items through `research`: quoted as untrusted text, PII redacted, linked by ID. | PostHog `session_recording:read heatmap:read survey:read` |
| **P5: writes through the catalog** | Production Engineer (owns `ops/**`); each proposing verb's seat | Catalog action templates and one vetted executor. Each operation's write transport allows exactly that method and path; it runs only in `aayudha-ops.yml` after a person approves. PostHog: flag enable, disable, and roll-out percentage; experiment launch, pause, end, and ship; annotation create; survey launch and stop; error-tracking assign and resolve. Sentry: issue resolve, assign, and ignore; alert, detector, uptime, and cron create or update; Seer fix start (opt-in, paid). A deploy annotation may become `auto` only after the same definition has merged as `human` (the existing catalog rule). | Write keys only in the ops environment, never in an agent's environment |

**Cost and quota.** Neither provider bills for API reads: both bill what is captured or ingested (PostHog events, recordings, exceptions, rows synced; Sentry errors, spans, replays, profiles, monitors). What costs money is AI, warehouse resyncs, and quota:

- **AI** is off by default. PostHog AI credits, with a default $150 AI billing limit, and Replay Vision credits. A Seer subscription is $40 per active contributor per month; the legacy plan charged $1 per fix run.
- **A warehouse resync** is billed per row synced, so it is a catalog action and never automatic.
- **PostHog `/query`:** 2,400 per hour and 240 per minute per project, 3 concurrent, 10 s, a default of 100 rows, and an hourly bytes budget shared with the team's own API use.
- **PostHog MCP:** 480 per minute per credential.
- **Sentry:** issues at 10 requests/s per user, events at 30 requests/s with 15 concurrent, and the hosted MCP at 60 per minute per user.

aayu caps the calls and rows per run per integration (`[integrations.<name>]`). The call log records each provider's quota headers, such as `X-PostHog-Query-Budget-Remaining-Bytes` and `X-Sentry-Rate-Limit-Remaining`.

Eval spend follows the golden rule: each phase that changes a skill adds fixture-based cases to the owning seat's group, and every eval result reports its spend.

**Options 2 to 4.**

- **Option 2** builds and maintains hundreds of wrappers against APIs that move under us: Sentry's legacy alert-rule API has already left the public spec, and PostHog's snapshot API is private. It duplicates what the read-only host MCP already offers for exploration. It also delays the governance gap, which exists today.
- **Option 3** fails where no host runs (CI, scheduled loops, `aayu release`). It cannot reproduce a number for a gate, because natural-language search tools are not deterministic. It inherits Sentry's write-capable OAuth tokens and PostHog's `exec` mode.
- **Option 4** leaves the gap open and the seats without evidence.

**What the founder needs to decide or provide.**

1. Accept the hybrid plan and the phase order (P0 first).
2. For live proofs: a PostHog project and a Sentry org with read-only keys, stored as `AAYU_SECRET_POSTHOG` and `AAYU_SECRET_SENTRY` with the `AAYU_LIVE_*` variables the live tests read. Without them each capability ships fixture-tested, marked honestly as not live-proven.
3. The proposing verb for writes outside an incident, since `respond` is incident-only (`internal/cli/respond.go`). Recommended: each owning verb proposes (for example `experiment --launch` for the Revenue Architect, `release` for a deploy annotation), and all of them dispatch the one catalog executor.
4. Paid AI (Seer runs, PostHog AI, Replay Vision): recommended off, with starting Seer available only as an opt-in catalog action.
5. PII: recommended that person rows, replay content, and raw prompts are not read unless `allow_pii` is set per integration.

### Recommendation

Option 1. It closes a trust-model gap that exists today, before adding any reach. It gives each seat the evidence its own checks need, through native adapters that are reproducible and work without a host. It uses the host MCP and CLI for breadth without letting either become a write path. Every call stays logged and citable.

Two-way door until a release tags it: everything lands on `dev` behind config, and the allowlists are data. Once released, the hook's refusals and the `posthog:` and `sentry:` query syntax become public behaviour.

### Reversible

true
````
