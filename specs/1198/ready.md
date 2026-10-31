# rahulnakmol/agentic-ayudha#1198: config validation prints a rejected secret value

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `f12561d8648afbd5128447a8f81da064df67e96d1ce94986ade86ae0e26d672b` |
| Completion Goal hash | `9716f303417d2a1ae4dfc57231c8d7d718c51d10bea0869a28bdc6acf130281c` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1198/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: [models.gateway] secret holds a pasted key; expected: config validation refuses it, names [models.gateway] secret and the rule, and the error text does not contain the value
2. [api-trace] state: [integrations.<name>] secret holds a pasted key; expected: config validation refuses it, names the key and the rule, and the error text does not contain the value
3. [api-trace] state: [identity] machine_user_secret holds a pasted token; expected: config validation refuses it, names the key and the rule, and the error text does not contain the value

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

config validation prints a rejected secret value. `Config.Validate` (internal/config/config.go) refuses a secret field whose value is not a secret name, and its message quotes the value with %q. That check fails exactly when a person pastes the key itself, so the key goes to stderr, terminal logs, and CI output. Three fields do this: `[models.gateway] secret`, `[integrations.<name>] secret`, and `[identity] machine_user_secret`.

### Steps to reproduce

1. Set `[models.gateway] secret = "sk-live-0123456789abcdef"` in the user config, with a `base_url`.
2. Run `aayu config validate`.

### Expected behaviour

The message names the key and the rule, for example `[models.gateway] secret: want the name of a secret (lowercase letters, digits, - and _), never the key`. It never prints the value.

### Actual behaviour

The message prints `[models.gateway] secret = "sk-live-0123456789abcdef": …`, so the key is on stderr.

### Completion Goal

1. [api-trace] state: [models.gateway] secret holds a pasted key; expected: config validation refuses it, names [models.gateway] secret and the rule, and the error text does not contain the value
2. [api-trace] state: [integrations.<name>] secret holds a pasted key; expected: config validation refuses it, names the key and the rule, and the error text does not contain the value
3. [api-trace] state: [identity] machine_user_secret holds a pasted token; expected: config validation refuses it, names the key and the rule, and the error text does not contain the value

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Validate a config with a pasted key in each secret field and check the error text.

### Environment

go test of internal/config (TestRejects); never production

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. The three Validate messages name the key and the rule, with no value (criteria 1, 2, 3)

### Assumptions

#### Notes

- Found in the review of PR #1194 (Bug #1193), whose inbox card first quoted config errors.
- P1: a key on stderr leaks into terminal logs and CI output.

#### Review guidance

- Check: no other Validate message prints a value from a field that holds a secret name.
- Risk: `[models.gateway] base_url` is printed too; a URL with user:token@ would leak the same way.

#### Behaviours

1. says: A pasted key never shows in aayu's output; rule: never; source: criteria 1, 2, 3

### Provenance

Request: "config validation prints a rejected secret value"
1. stated (request): "config validation prints a rejected secret value"
2. stated (request): "config validation prints a rejected secret value"
3. stated (request): "config validation prints a rejected secret value"
````
