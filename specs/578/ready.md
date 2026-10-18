# rahulnakmol/agentic-ayudha#578: init --identity cannot complete from a remote shell or a phone, and its timeout is reported as an internal error

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `719723f0e900203e4464c27123b6fea79647964869318081b5194278e8b9fc4e` |
| Completion Goal hash | `8716f03b30967eafdc4f0ae0c52a543682db2cfff3787cd92fc10c8cea137342` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/578/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu init --identity` with the fake GitHub answering the conversion with 422 (the code was already converted); expected: no secret written, no Install link printed, exit 4, and a next step that says not to install the App, to delete it at its settings page, and to rerun
2. [api-trace] state: `aayu init --identity` where no code arrives before the wait ends; expected: exit 4 with "blocked or needs a human" and a next step naming how to rerun; no "internal error"

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu init --identity` cannot complete from a remote shell: an Amp orb's Terminal tab, Claude Code or Codex on the web, SSH, or a phone. The manifest flow (internal/cli/manifest.go `manifestCode`) listens on `127.0.0.1:<random port>`. It tells the person to open `http://127.0.0.1:<port>/` and sets the manifest's `redirect_url` to `http://127.0.0.1:<port>/callback`. A browser on another device can reach neither the start page nor the callback. The command then waits 10 minutes and fails with "timed out waiting for GitHub to redirect back with the App code", a bare error. aayu reports it as exit 1, "internal error (a bug; please report it)", although nothing in aayu broke: the person could not reach the page. The founder hit this on 2026-10-18, running `aayu init --identity` in this repository's orb Terminal tab from a phone. No App was created, because the flow failed before the code conversion.

Deviation from the plan: this Bug's original criteria 1 and 2 (the cross-device flow: an HTTPS start link, a github.com `redirect_url`, and a pasted address checked against the state) are covered by #594's criteria 4 and 2, after Codex's security review r4307058604 on #603 showed that a pasted-code flow through a page aayu does not control cannot be made safe. The Completion Goal keeps the original criteria 3 and 4, renumbered 1 and 2; the "Expected behaviour" item 1 below is delivered by #594. Founder's authorisation, 2026-10-18 (relayed by the orchestrator thread): "Yes", to edit this Bug's body directly, making the Completion Goal criteria 3 and 4 with this note.

### Steps to reproduce

1. In an orb's Terminal tab (or any shell on another machine than the browser), run `aayu init --identity` in a repository with no bot identity, and confirm.
2. Open the printed `http://127.0.0.1:<port>/` link on a phone.
3. Wait 10 minutes.

### Expected behaviour

1. The flow works when the browser is on another device, and the one-time code reaches no page aayu does not control.
   - aayu prints an HTTPS start link the phone can open.
   - The manifest's `redirect_url` is a page on github.com itself, such as this repository's rendered `docs/guide/app-code.md`. That page says: "Copy this page's address and paste it into the terminal."
   - GitHub appends `?code=…&state=…` to that URL as a query string, not a fragment. Only GitHub receives it, and GitHub issued the code anyway. No Pages site, third-party host, script, font or analytics sees it.
   - The person pastes the whole address. aayu reads `code` and `state` from it, checks the state, and converts the code at once with `POST /app-manifests/{code}/conversions`. That endpoint takes no authentication; the docs example sends none.
   - The PEM goes straight into the repository secret.
   - The Install link is shown only after the conversion returns 201. If the conversion fails (404 or 422), someone else may have converted the code first. aayu then says not to install the App and to delete it at its settings page, and rerun.
   - Why the key is safe until Install: an App with no installation can mint no token for any repository, so a code converted by someone else yields a key that reaches nothing as long as the person does not install that App.
   - On a machine whose browser can reach loopback, the current automatic redirect keeps working.
2. A timeout, or a person who does not finish, exits 4 (blocked, needs a human) and names the next step. It is never reported as an internal error.

### Actual behaviour

1. The printed link is a loopback address, which the phone cannot open; and a design that sent the code to the documentation site would expose it to every writer of `docs/`, which is not a protected path (.github/CODEOWNERS), and to the Pages request logs. Even on a laptop browser that could open it, GitHub's redirect to `127.0.0.1` reaches the orb only when the browser runs in the orb.
2. After 10 minutes the command fails with exit 1, "internal error (a bug; please report it)".

### Completion Goal

1. [api-trace] state: `aayu init --identity` with the fake GitHub answering the conversion with 422 (the code was already converted); expected: no secret written, no Install link printed, exit 4, and a next step that says not to install the App, to delete it at its settings page, and to rerun
2. [api-trace] state: `aayu init --identity` where no code arrives before the wait ends; expected: exit 4 with "blocked or needs a human" and a next step naming how to rerun; no "internal error"

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Build aayu from the fix branch and run the init tests in internal/cli against the GitHub fake (`AppManifestCode` hook in internal/cli/init_test.go) and a fake browser that cannot reach loopback.

### Environment

aayu built from dev at 8cbfe15 (internal/cli/manifest.go `manifestCode`, internal/initrepo/extras.go `AppManifest`), run in an Amp orb's Terminal tab opened from a phone, 2026-10-18. Affects any remote shell.

### Checkpoints

1. Install only after a successful conversion, and a failed conversion is treated as a possible theft of the code (criterion 1)
2. A wait that ends without a code exits 4 with a next step (criterion 2)

### Assumptions

- The start link is a static page on the project's documentation site. It only builds and posts the manifest form, and it never receives the code. Its manifest is passed in the URL fragment, so the Pages host sees no part of it.
- If that start page were tampered with, it could change the manifest (a different `redirect_url`), so aayu checks the converted App's name and permissions against the manifest it built before writing the secret.
- Fallback if GitHub rejects or rewrites a github.com `redirect_url`: a page on the documentation site may receive the code only if all of these hold:
  - it lives under a protected path;
  - it loads no third-party scripts, fonts or analytics;
  - it sets `<meta name="referrer" content="no-referrer">`, because GitHub Pages cannot send a Referrer-Policy header;
  - it removes the query with `history.replaceState` on load.
  
  GitHub Pages' own request logs would still hold the code; GitHub runs them and does not give them to the repository's owner.
- Which App the flow creates (one per repository, or one per owner) follows Decision #576 and is not decided by this Bug.

### Provenance

Request: "file a Bug through aayu: init --identity can't complete from a remote shell, and a timeout is reported as an internal error"
1. stated (security/manifest code): "If any of that can't hold, prefer a design where the code never reaches a page aayu doesn't control."
2. stated (correctness/exit code): "a timeout is reported as an internal error"
Term: "remote shell" → any shell whose browser is on another device: an orb's Terminal tab, Claude Code or Codex on the web, SSH; "can't complete from a remote shell"
````
