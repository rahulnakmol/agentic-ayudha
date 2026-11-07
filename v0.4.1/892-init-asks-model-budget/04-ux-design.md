# #892 UX design

The surface is the `aayu init` terminal session. A person runs it; an agent cannot (`init` is human-only and needs a
terminal: `Identity: IdentityHuman`, `Gates: GateTTY`, `Surfaces: cliOnly` in `internal/verbs/registry.go`, entry
`Name: "init"`).

## Today on dev

`aayu init --help`, from the binary built at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Set up a product repo: labels, issue types, forms, config, ledger branch, and optionally projects, gates, identity, integrations.

Usage:
  aayu init [flags]

Flags:
      --gates          Install gate workflows.
  -h, --help           help for init
      --identity       Create the GitHub App bot identity.
      --integrations   Configure external integrations.
      --projects       Create Projects v2 views.
      --resume         Resume a partial run from .aayudha/state.json.
      --verify-skill   Write the product's verify skill: an empty feature map and a control tool in the product's stack.
```

The only question init asks on its own is the confirmation in `runInit` (`internal/cli/init.go:144-145`): "init uses
your admin access on `<repo>` (`<visibility>`) to create labels, … Continue? [y/N]".

The spend lines a person sees today come from `aayu build` (`internal/cli/build.go:86-91`), for example "metered spend
0.00 USD, no cost limit: no pay-as-you-go model key or endpoint is configured (subscription use not counted)".

## Planned, not built

The flow below comes from Decision [#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868) option 1, which #918 superseded while it kept the question for #892. The wording is a candidate, not an approved design.

1. For each detected host: "How do you pay for models: subscription or pay as you go?"
2. Pay as you go: "Daily model budget in USD [100]". Below it, what the budget buys at the price of the build lane's
   model for that host, with tokens per story labelled "assumed". With no price, init says so.
3. Subscription: no daily-budget question for agent-lane cost. aayu watches the host's usage limit instead.
4. When an API-key lane is set up, init asks for the API-call cap.
5. With no terminal, init writes nothing and names the keys to set.

No inbox card, web page, or phone screen is in scope for this Feature.

## Agent-facing surface

An agent reads the result through `aayu config show` and through build's spend line. The planned change to
build's and status's wording (FR9 in [02-spec.md](02-spec.md)) is inferred, not specified.

## Accessibility

Init's questions are plain terminal lines read by `Env.askLine` (`internal/cli/manifest.go:24`). The global `--plain`
flag already removes colour and box drawing. The sources state no further accessibility requirement.
