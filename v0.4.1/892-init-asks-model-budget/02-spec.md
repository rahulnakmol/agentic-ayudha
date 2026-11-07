# #892 Spec

No spec baseline exists (`specs/892` is absent from the ledger), and no Story carries a Completion Goal. Each
requirement below traces to the Feature issue or a Decision. The "Slice" column names the proposed slice in
[05-implementation-plan.md](05-implementation-plan.md); slices are proposed, not filed.

## Functional requirements

| # | Requirement | Source | Slice |
|---|---|---|---|
| FR1 | `aayu init` asks, for each detected host, "subscription or pay as you go". | [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) Summary; [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) option 1 | P3 |
| FR2 | The answer is written per host under `[budget.hosts]`. #868 gave the values `"subscription"` and `"metered"`. | #892 Scope; [#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868) option 1 (superseded) | P1 |
| FR3 | For pay as you go, init asks for a daily budget. #868 proposed the prompt "Daily model budget in USD [100]". | #892 Summary; #868 option 1 (superseded) | P3 |
| FR4 | Init shows what the budget buys, at the price of the build lane's model for that host. Tokens per story are labelled "assumed" until measured. With no price, init says so. | #892 Scope; #868 option 1 (superseded) | P3 |
| FR5 | The breaker does not count a subscription host's agent-lane cost. aayu watches the host's usage limit instead. | #892 Scope; #868 option 1 (superseded) | P2 |
| FR6 | A separate cap stays on the calls aayu makes with an API key: `ModeDirect` verbs, the merge gate's review, and evals. #918 names the key `[budget] metered_per_release_usd`. | #892 Scope; #868 option 1; #918 option 1 | P1, P2 |
| FR7 | "Subscription" never writes a budget of 0, because 0 turns the whole breaker off. | #868 Evaluation (superseded); `internal/models/gateway.go:175`, `:194` | P1, P3 |
| FR8 | Init writes `[budget]` to `.aayudha/config.toml` only on a terminal. With no terminal it writes nothing and names the keys. | #868 option 1 (superseded) | P3 |
| FR9 | Build and status say which billing mode applies and which cap holds. | inferred from #892 Expected paths (`internal/cli/status.go`, `internal/status/status.go`, `internal/cli/build.go`) | P4 |

FR2 to FR8 rest partly on #868, which Decision #918 superseded. #918 keeps "the budget question in `aayu init`"
for #892 but does not restate the details. Treat the #868 details as candidates for the spec baseline, not as
agreed requirements.

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Security | `init` is human-only and needs a terminal (`Identity: IdentityHuman`, `Gates: GateTTY`). | `internal/verbs/registry.go`, entry `Name: "init"` |
| Security | A config change in `internal/config/` or `.aayudha/config.toml` needs a person to ship. | `.aayudha/config.toml` `[autonomy] always_person`, `always_person_config`; #892 Scope |
| Cost | `per_day_usd` defaults to 100.0 and `per_story_usd` to 60.0. A new default must keep `per_story_usd` at least `autonomy.budget.cost_usd`. | `internal/config/defaults.toml` `[budget]` comment |
| Operability | Every new key in a rendered section needs a row in `internal/config/gen`, or generation fails. | AGENTS.md, "Generated files" |
| Honesty | A price aayu does not have is reported as missing, never guessed. | #868 Evaluation (superseded): no price for "Amp medium" or Copilot Auto tiers |

## Open questions

1. Which key name holds the API-call cap: a new `metered_per_release_usd` (#918) or a per-day cap (#868)?
   Unverified which one the founder wants.
2. Does "subscription" for a host change `autonomy.budget.cost_usd`? The config docs already say a subscription
   reports no cost (`internal/config/gen/main.go:85`). The sources do not say.
