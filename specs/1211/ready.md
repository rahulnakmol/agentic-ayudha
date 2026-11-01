# rahulnakmol/agentic-ayudha#1211: fix(env): a stale amp environment record stops every Amp mission start, and only Docker can verify it again

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `97a6b1a86225221027018ce456ca1e49b4616f23694a0ff53590584fb454f9f1` |
| Completion Goal hash | `b0a328b4963f1cfa0b22e28dd14a9ad96d05bf3301611289476c86b8299539ef` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1211/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a PR changes `.aayudha/env.toml` or `.agents/setup`, and `.aayudha/env-verified.toml` still has the old fingerprint for a host in `[env] hosts`; expected: a required dev check fails, names each stale host, and gives `aayu install --env --host <host> --apply --verify`
2. [api-trace] state: the amp record is stale, there is no Docker daemon, and build runs in an Amp thread; expected: build verifies amp with the cloud probe, records `[cloud.amp]` at the new fingerprint, and hands the story off
3. [api-trace] state: the amp record is stale and the cloud probe fails; expected: build stops with the probe's last error line and the command to run, and the story goes back to status:ready with its claim released
4. [api-trace] state: the amp record is current; expected: build hands off with no container and no probe, as today
5. [api-trace] state: `aayu mission start <f>` on amp with a stale amp record; expected: start says the record is stale and names the command, before the device flow

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

An Amp mission cannot start a story when the committed environment record is stale and the Kanha's machine has no Docker. `aayu build <n> --delegate amp` runs `ensureCloudEnv` before every hand-off (internal/cli/delegate.go:752). When the host is not `verified`, `agentenv.Ensure` verifies the setup again in a clean `debian:12` container (internal/agentenv/status.go:225-242), and that needs a Docker daemon (internal/agentenv/verify.go:59). The record `.aayudha/env-verified.toml` has `[container.amp]` and `[cloud.amp]` at fingerprint `sha256:b6655dfa4a0f5c48` from 2026-10-16. `.aayudha/env.toml` changed after that, in PR #1123 on 2026-10-27, so the amp host is no longer verified. On 2026-11-01 at 02:10 UTC, right after #1208 (PR #1209) merged, the Kanha of #1170 ran `aayu build 1172 --delegate amp` in its Amp orb. It failed with "no Docker daemon answers (docker info)", and #1172 went back to status:ready. The Kanha runs inside an Amp orb that ran the same `.agents/setup`. No merge to dev says that the record must be verified again.

### Steps to reproduce

1. `.aayudha/env-verified.toml` records amp at a fingerprint, and `.aayudha/env.toml` changes after it.
2. In an Amp orb with no Docker daemon, run `aayu build 1172 --delegate amp` for a story that the mission marks start.

### Expected behaviour

A merge that makes the amp record stale fails a dev check, or a person or agent is told before a mission start. A Kanha in an Amp orb can verify the amp environment with the cloud probe (`aayu install --env --host amp --cloud`), with no Docker.

### Actual behaviour

The hand-off stops with "no Docker daemon answers", and only a machine with Docker can verify the environment again.

### Completion Goal

1. [api-trace] state: a PR changes `.aayudha/env.toml` or `.agents/setup`, and `.aayudha/env-verified.toml` still has the old fingerprint for a host in `[env] hosts`; expected: a required dev check fails, names each stale host, and gives `aayu install --env --host <host> --apply --verify`
2. [api-trace] state: the amp record is stale, there is no Docker daemon, and build runs in an Amp thread; expected: build verifies amp with the cloud probe, records `[cloud.amp]` at the new fingerprint, and hands the story off
3. [api-trace] state: the amp record is stale and the cloud probe fails; expected: build stops with the probe's last error line and the command to run, and the story goes back to status:ready with its claim released
4. [api-trace] state: the amp record is current; expected: build hands off with no container and no probe, as today
5. [api-trace] state: `aayu mission start <f>` on amp with a stale amp record; expected: start says the record is stale and names the command, before the device flow

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Change the env spec in a PR, start a story on amp with a stale record with and without a working cloud probe, start one with a current record, and start a mission with a stale record.

### Environment

go test of internal/agentenv and internal/cli with a fake runner, a fake amp CLI, and a fake GitHub; never production

### Checkpoints

1. A stale record fails a dev check (criteria 1)
2. Build verifies amp in the cloud when it has no Docker (criteria 2, 3, 4)
3. Mission start warns about a stale record before the code (criteria 5)

### Technical Owner

devops-devex-tinkerer

### Assumptions

#### Notes

The cloud probe is the existing `aayu install --env --host amp --cloud` path, which runs the probe in a new orb. In a Kanha thread with no CLI sign-in, it uses the host hand-off of #1208. The hand-off waits for the probe thread's reply with the same 15-minute lapse.

#### Review guidance

- Check: criterion 1: the check reads the hosts of `[env] hosts`, not only amp.
- Check: criterion 3: a failed probe releases the claim.
- Ignore: the wording of the stale-record message.
- Risk: a probe that reports success it did not reach; only the probe's exact last line counts.

#### Behaviours

1. says: A stale environment record is caught at merge, never at a mission start; rule: limit; source: criteria 1, 5
2. says: An Amp Kanha verifies the amp environment with no Docker; rule: limit; source: criteria 2, 3

### Provenance

Request: "Both are run and will wait for 1209 fix as if is building in aayu in aayu on amp it should not depend on the cli"
1. default (scope): a dev check catches a stale record
2. stated (request): building on amp needs no local dependency
3. default (scope): a failed probe releases the claim
4. default (scope): a current record changes nothing
5. default (scope): mission start warns before the code
````
