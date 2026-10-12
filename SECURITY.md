# Security policy

This file covers two things: how to report a vulnerability in `aayudha`, and what the maintainer does when
the tool itself is compromised (the tool-incident plan, WP0.1). The threat model is
[docs/BUILD-PLAN.md §0.1](docs/BUILD-PLAN.md#01-trust-model), expanded in
[docs/threat-model.md](docs/threat-model.md).

Features marked "n/a until WPx.y" do not exist yet. Until then the manual step written next to them applies.

## Supported versions

| Version | Supported |
|---|---|
| pre-0.1 (today) | Nothing is supported for production use. Reports are still welcome and are fixed on `dev`. |
| After the first stable release | The current release, plus security patches for the previous two minor versions. Each minor version's end-of-life date is published in its GitHub release notes. |

Only tagged releases from `main` (`v*`) are supported. `dev` and `rolling` snapshots are not.

## Reporting a vulnerability

Report privately through GitHub private vulnerability reporting:
**Security → Report a vulnerability**, or directly at
<https://github.com/rahulnakmol/agentic-ayudha/security/advisories/new>.

Do not open a public issue, pull request, or discussion for a vulnerability.

Include:

- the affected version (`aayu version`) or commit SHA, and the OS/architecture;
- the affected verb, plumbing command, skill, or workflow;
- steps to reproduce, with a minimal repository or input if you can;
- the impact you expect (for example: token exposure, a write outside `internal/actions`, a bypassed floor
  or human gate, prompt injection that causes a write);
- whether you have shared the issue with anyone else.

Do not include real secrets or personal data. Use synthetic values.

### Response targets

This is a one-person project. These are targets, not guarantees:

| Step | Target |
|---|---|
| Acknowledge the report | 5 business days |
| Initial assessment (accepted / not a vulnerability / need more information) | 10 business days |
| Fix or mitigation for a confirmed critical or high issue | 30 days |
| Fix for a confirmed medium or low issue | Next planned release |

If a target slips, the maintainer says so in the advisory thread.

### Coordinated disclosure

- The report stays private in a draft GitHub security advisory while the fix is prepared.
- The default embargo is 90 days from the report, or until a fixed release ships, whichever is first. We can
  agree a different date in the advisory thread.
- A CVE is requested through GitHub when the issue affects a released version.
- Reporters are credited in the advisory unless they ask not to be.

## Release integrity

Releases are built by goreleaser in [release.yml](.github/workflows/release.yml), only for an annotated `v*`
tag whose commit is on `main`. Each release carries `checksums.txt` signed keylessly with cosign
(`checksums.txt.sigstore.json`), an SPDX SBOM per archive, and SLSA build provenance. The signing certificate
names `https://github.com/rahulnakmol/agentic-ayudha/.github/workflows/release.yml@refs/tags/<tag>`.

[install.sh](install.sh) refuses to install unless that signature verifies for the exact tag requested and
the archive matches its signed checksum; CI proves on every change that it refuses a flipped byte, a rewritten
checksum, and a signature from any other workflow. `aayu self-update` will do the same (n/a until WP1.1). By
hand:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/rahulnakmol/agentic-ayudha/.github/workflows/release.yml@refs/tags/vX.Y.Z" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
sha256sum --ignore-missing -c checksums.txt
gh attestation verify aayudha_X.Y.Z_linux_amd64.tar.gz --repo rahulnakmol/agentic-ayudha
```

No release has been published yet. Anything published under this repository's name that does not verify this
way is not a release.

## Tool-incident plan

Scope: `aayudha` itself is the threat. Examples: a malicious or compromised skill in `skills/`, a tampered or
compromised release, a leaked credential used by an agent lane or CI, or prompt injection that made an agent
act.

### Detection signals

- A private report or a public report from a user.
- An unexpected write on GitHub: a commit, PR, comment, label, release, or workflow run that no human or
  agent lane started. Compare GitHub's own event history with `telemetry/actions.jsonl` (automated by
  `aayu audit` reconciliation, n/a until WP7.6; until then by hand).
- `aayu actions verify-chain` reports a broken hash chain (n/a until WP1.5).
- A skill copy differs from `.aayudha/lock.toml` (`aayu doctor`, n/a until WP3.2).
- A release asset whose signature, provenance, or checksum does not verify (see
  [Release integrity](#release-integrity)).
- Secret-scanning, gitleaks, or semgrep alerts; GitHub notices of token or App-key exposure.
- A ruleset bypass in the rule-suite history that is not listed as a solo-mode bypass (see
  [CONTRIBUTING.md](CONTRIBUTING.md#solo-mode-policy)).

### 1. Contain

1. **Halt.** Run `aayu halt --reason "<why>"`. It sets the repository variable `AAYUDHA_HALT`: the first step of
   every aayudha workflow job fails while it is set, every verb refuses to start (exit 4), a running verb stops
   within one poll (30 s), and the host pre-tool hook blocks agent tool calls. It does not cancel workflow runs
   already past their first step and cannot stop a compromised binary that ignores it, so also:
   - cancel in-progress runs (`gh run cancel <id>`) and, if the binary itself is suspect, disable workflows
     (`gh workflow disable <name>`, or Settings → Actions → Disable actions);
   - revoke the tokens agents use (see [Leaked credentials](#leaked-credentials));
   - stop any running agent sessions.
2. **Yank the release** if a release is affected: delete the GitHub release or mark it as compromised in its
   notes, and pull the brew, Scoop, and winget manifests (revert the commit in `homebrew-tap` and `scoop-bucket`;
   close or revert the winget PR).
   Keep the artifacts and their hashes offline for the investigation.
3. **Pull a bad skill**: revert it on `dev` and tell users to re-sync with `aayu install` or pin a good version
   with `aayu install --pin <version>` (n/a until WP3.2).
4. **Open a draft GitHub security advisory** for the incident.

### 2. Eradicate

- Find the root cause and every affected commit, release, and skill version.
- Fix through the emergency path in [CONTRIBUTING.md](CONTRIBUTING.md#emergency-path): branch from
  `rolling` or `main`, merge the fix into `dev` first, cherry-pick the same fix to `rolling`/`main`, run the
  release gate, tag, and record why.
- Rotate every credential the compromised component could reach.

### 3. Recover

1. Ship a new signed release with the fix (cosign signature and SLSA provenance, `release.yml`).
2. Run `aayu resume` to clear the halt variable (and the `[budget]` breaker's pause), re-enable any workflows you
   disabled by hand, and re-issue tokens.
3. Publish the GitHub security advisory with affected versions, fixed version, and the steps users must take
   (upgrade, re-install skills, rotate their own tokens).

### 4. Retro

Within two weeks of recovery, write a post-incident retro: timeline, detection gap, root cause, what
contained it, and follow-up work. Follow-ups are filed as issues on this repository. File the retro as an
issue; `aayu retro` takes this over once it exists (n/a until WP7.6).

### Leaked credentials

Identities are described in [docs/BUILD-PLAN.md §0.1](docs/BUILD-PLAN.md#01-trust-model): humans use their own
token; agent lanes and CI use a bot identity (n/a until WP1.4; until then agents run under the human's
identity and their writes are tagged `identity:human-fallback`).

- **Human PAT or `gh` token**: revoke it in GitHub settings, then create a new one with the narrowest scopes.
- **GitHub App private key**: delete the key in the App settings, generate a new one, and update the
  repository secret the token broker reads (n/a until WP1.4).
- **Machine-user PAT**: revoke it and issue a new one (n/a until WP1.4).
- **Model, integration, and other repository or environment secrets**: rotate at the provider, then update
  the GitHub secret or the local secrets store (`aayu` keychain/env/age backends, n/a until WP1.3).
- Then review GitHub's event history for anything the credential did after the leak window opened.

Leaked secrets in a commit stay in Git history. Rotate them; rewriting history is not a fix.

### Prompt injection via untrusted text

Issue and PR bodies, comments, intake forms, Figma text, Mobbin results, PostHog/Sentry exports, and
dependency output are untrusted (§0.1). The planned controls are: untrusted text is wrapped in delimited
provenance blocks before any model sees it; it can never trigger `respond`, `ship`, `tell publish`, or
ops-catalog execution; each verb has a tool allowlist; child processes run with a scrubbed environment; and
`triage` holds a labels-only capability (WP1.5, WP2.9, WP4.8). None of these exist yet.

If an agent acted on injected instructions: treat it as an incident, follow the plan above, revert the
writes it made, and add the text as an injection fixture so the eval suite catches it.
