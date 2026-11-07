# #892 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/892`. This page uses
the Feature issue [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) and Decisions
[#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) (accepted) and
[#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868) (superseded by #918).

## Goal

Setup asks, per host, "subscription or pay as you go", then a daily budget with what it buys, and writes `[budget]`
([#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) Summary).

## Who it serves and the customer outcome

- The person who runs `aayu init` on a product repository (a human-only verb: `init` has `Identity: IdentityHuman`
  and `GateTTY` in `internal/verbs/registry.go`, entry `Name: "init"`).
- Outcome (inferred from the Summary and Problem of #892): after setup, the budget matches how the person pays. A
  subscription host is not stopped by a pay-as-you-go cap, and aayu's own API-key calls still have a cap.

## Problem

Quoted from [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892):

> The budget is configuration, but aayu init does not ask for it, and a budget of 0 also stops the cap on API calls
> (Decision #868, F16, F17).

The code confirms both parts on `dev`:

- `runInit` (`internal/cli/init.go:130-189`) asks one confirmation question and the extras' questions. It asks
  nothing about budget.
- The breaker checks a limit only when it is above 0: `if lim := b.Budget.PerDayUSD; ... lim > 0` in
  `Breaker.Check` and `Breaker.Add` (`internal/models/gateway.go:175`, `:194`). So `per_day_usd = 0` turns the
  daily cap off for every call, including calls that use an API key.

## Scope

In, quoted from [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892):

> the init questions; [budget.hosts]; the breaker skips a subscription host's agent-lane cost; a separate cap for
> API calls; what a budget buys, with assumed tokens labelled.

Decision [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918), option 1, adds: "The budget question in
`aayu init` belongs to #892 (pay as you go or subscription, and `[budget] metered_per_release_usd`)."

## Out of scope

- "a change of the build default model" ([#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892)).
- The repository-wide build cap, wave pick, fair turns, review hold, and spend hold of #868. Decision
  [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) dropped them or gave them to other Features.

## Success measures

The sources state no success measure for this Feature. #892 has no Measure section. Decision #868 (superseded)
said the budget stays at `per_day_usd = 100` until `delivery/stories.jsonl` has two weeks of measured cost per
story; whether that rule still holds after #918 is unverified.

## Notes for a later baseline

- #892 says "Its PR needs a person to ship." Its Expected paths include `internal/config/**`, which is in
  `[autonomy] always_person` (`.aayudha/config.toml:75-101`).
- Two key names appear in the sources: `[budget.hosts]` (#892, #868) and `[budget] metered_per_release_usd`
  (#918). Neither exists in `internal/config/config.go` or `internal/config/defaults.toml` on `dev`. The intent
  baseline must pick the names.
