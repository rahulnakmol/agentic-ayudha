<!-- markdownlint-configure-file { "MD024": { "siblings_only": true } } -->
# Changelog

What changed in aayu (the `aayu` CLI, its skills, and the workflows it installs), newest version first.

Each item cites the merged pull request it comes from as (#n): pull request n in
`github.com/rahulnakmol/agentic-ayudha`. A Feature is written as Feature #n: issue n, not a pull request. An item says what merged. It does not say the work is proven. A Feature is
proven only when every shipped story under it has an authoritative proof pack (the record the merge gate checks
against the story's criteria), and each version below says which Features that is. Words such as Ready, proof pack,
ledger, and floor are explained in [GLOSSARY.md](GLOSSARY.md).

## v0.3.0

Released 2026-10-22.

With v0.3.0, aayu builds its own repository, agentic-ayudha: agents open its pull requests with the `agent-authored`
label (AGENTS.md, "Every agent PR declares itself"), and one person, @rahulnakmol, is the recorded approver
(`[autonomy] approver` in `.aayudha/config.toml`). With `[autonomy] proof` on,
`aayu ship` merges an agent's Story pull request into dev on proof alone, and on that path a narrow set of paths and settings always
waits for a person (#617). Two new seats, the Production Engineer and the Data & AI Architect, bring
skills for incidents, SLOs, runbooks, and data designs (#337, #293), and `aayu survey` maps legacy code (#292). Every pull request
that is not docs-only now gets a review once it is ready for review and its checks and proof pass, by default from a model you
choose in your own CI, and the merge gate records how independent that review was (#466, #577, #657, #483). You approve the
human-only steps agents queue for you from any device, one at a time in `aayu inbox`, and can accept several Decisions with one
reply (#442, #467, #678, #501). No Feature is claimed as proven for this release: none of its Features has an authoritative
proof pack yet. If you already use aayu, do the six upgrade steps on the
[release page](docs/releases/v0.3.0.md#upgrade) first; each says how to check it and who does it.

### New verbs and skills

- The `build-ready` loop builds Ready work on its own: at 09:00, 11:00, 13:00, 15:00, and 17:00 on weekdays in `[time] tz`, it runs `aayu ship` on its own open pull requests, then `aayu build --next pick` claims at most one Ready Story or Bug, in the order of the current release's accepted baseline, else by priority, then age (#685).
- `[build] in_flight` (1 by default; 0 stops the picks) caps the items from claim to merge; the cap, the loop's file, and every `[budget]` cap of a loop build, CI minutes included, come from the committed config of the integration branch you trust (your user config's `[branches] dev`, else the default branch's committed one, else the default branch, never the checkout, an `AAYU_*` variable, or `--set`), which is also the branch a loop build starts from and opens its pull request against, so a local value can only tighten them, and only a person changes the loop's file or `[build]` (#685, #827).
- `aayu build <n> --stop "<why>"` blocks a build that cannot reach a pull request, and a loop claim with no pull request is blocked 4 hours after its lease runs out, with its owner paged; `aayu ship` refuses a pull request from a story's build branch while the story is `status:blocked`, even when the build's ledger state is missing or torn, and reads that label again just before the merge, so a stop holds after the loop listed it or while ship judges the pull request, and no Ready to ship card is posted; it also refuses a pull request whose body names another story than its `story/<n>` or `bug/<n>` branch (#685, #827, #830).
- `aayu scope <release>` plans a release in its version milestone: the Product Strategist's twin adds, drops, and drafts the plan, and a person accepts it as the baseline from their own terminal or the inbox's Release scope card (#676).
- After the baseline, each change counts as drift in effort points (XS 1, S 2, M 3, L 5, XL 8), held to `[scope] budget_percent`, 20% by default; a change over budget needs an accepted Decision, and a person accepts it (#676).
- `aayu scope --handoff` writes the Release's includes links and posts the plan, and `aayu scope --assign <n> --to this|next|backlog` places a Feedback issue by milestone and horizon label; when two open milestones carry the same release's title, scope stops and asks you to close or rename one rather than guess (#676, #820).
- `aayu brain` keeps the product's lasting knowledge, such as its vision, principles, objectives, objections, decisions, and capabilities, as linked notes on an orphan `brain` branch that Obsidian opens as a vault with a graph; each note is owned by a seat and links issues by number without copying them (#674).
- `aayu brain init` creates the vault, `aayu brain checkpoint` refreshes only the marked generated blocks from the repository's records and keeps every word a person wrote (`aayu release` runs it after landing), and `aayu brain check` fails a broken link, a missing source, a sentence over 20 words, or a pasted Completion Goal line or issue label; on a public repository the `brain` branch is public (#674).
- `aayu investigate` investigates an incident from bounded telemetry queries down to the deployed revision and the implicated code, and proposes a mitigation without running it (#343).
- `aayu investigate --source sentry|posthog` runs its queries against your configured Sentry or PostHog project with a read-only key (#526, #536, #590).
- investigate reads Sentry Seer's existing analysis of an issue as a labelled opinion it never cites, and never starts a paid analysis (#542).
- `aayu slo` and `aayu runbook` define and check SLOs (service level objectives) and runbooks, and gate releases on them: a stale SLO status refuses a release, and while an error budget is spent only Bugs ship (#352).
- `aayu slo` measures a crash-free sessions SLO from Sentry release health over the whole SLO window (#547).
- `aayu runbook` checks each runbook's alerts against the alerts in your Sentry and PostHog projects (#540, #560).
- `aayu respond` writes a checked, blameless postmortem and files the follow-up Bug from the investigation (#358).
- The Production Engineer's skills use the live Sentry and PostHog reads and copy their figures only as the evidence allows (#616).
- A new Production Engineer seat owns production reliability: SLOs, incidents, safe rollouts, and investigation (#337).
- A new Data & AI Architect seat owns `measure` and the new `aayu chart`, which charts an issue's data or AI design before planning (#293, #325).
- `aayu survey` surveys a codebase with no product docs: it indexes the code, checks an evidence-backed survey draft, and renders `SURVEY.md` (#292, #294, #313, #495).
- survey indexes Fortran, COBOL, Pascal, and Visual Basic, and reports traps, a do-not-carry list, and a compatibility contract per stack (#336, #346).
- survey files a backlog only from the reviewed digest, and captures golden outputs from the legacy build to check parity (#298, #305).
- The merge gate holds a modernisation Story to parity with the legacy code's golden outputs when its criteria ask for it (#405).
- The survey skill ends with a self-check turn, and floors cover its capability map, question count, and each case set's runners (#395, #471).
- `aayu counsel` asks Atri, the Founder's Sage, about any topic, issue, path, or pull request, and answers with cited sources and the next step; it gives advice and never a review (#493).
- `aayu inbox` lists every human-only step waiting for you, with the seat twin's recommendation, and acts on each with one keypress (#442).
- The inbox shows one card at a time with the seat's counsel, a finder, and a live refresh; `aayu inbox --list` prints the list for agents (#587).
- The inbox has a loading skeleton, short motion, and colours that follow your terminal theme (#620).
- The full-screen inbox shows a grouped list on the left (Needs you now, Waiting on the review cap, Housekeeping, Done this session) with aligned columns, and the item's card on the right; Enter starts the write in the background, at most two at a time and never two on one pull request or issue, and the card shows its progress as a checklist that keeps every finished step even when a stage repeats, then what changed on GitHub; only housekeeping can be selected (`x`, `X`) or cleared in one go (`C`); Tab moves to the next item that needs you; and `q` during writes asks whether to wait for all of them or only those under way (#707, #784).
- Every inbox refresh reads each Ready to ship card's pull request again (its merge state, the merge gate's checks, every required check, and new commits), and a pull request that cannot ship moves to a "Not ready yet" group with no ship option, a reason such as "#661 can't merge: it conflicts with dev, which took #725 since ship checked it", and its builder's next step; `d` shows the details, the key bar shows `R refresh` and when the inbox last read GitHub, a not-ready card is not counted as a step for you in `aayu next` or on the home screen, and GitHub's brief unknown merge state alone never blocks a card (#802).
- When an inbox action fails, the card says in three plain lines what failed, what already changed on GitHub (or that nothing did), and who acts next, instead of a URL, a status code, and Go error text; `d` shows the detail, `--plain` prints the same lines with a "Detail:" line, a failure that Enter can fix says "Enter retries", a card stops offering an action that cannot work, and a merge that landed before a later step failed shows as merged, with a warning naming the step, a capped card's included (#845, #853).
- When you clear a set of housekeeping with every item done, even a set of one and from any card, the inbox says "Housekeeping cleared: n tidied. Job well done." with a line from the Auditor, and when the last item that needs you is done with nothing skipped or failed, it says "Inbox zero. Job well done." with a line from the Founder's Sage; with motion on, brief confetti falls over the card pane, never over the key bar, and it never follows a failure; with `[ui] motion = false` or `NO_COLOR` only the words show, and `--plain` prints them as one block (#851, #853).
- Every Decision offers at least two numbered options, and you pick one by its own number (#498, #551).
- In solo mode, a reversible Decision applies its recommendation provisionally after 24 business hours; a waiver, a seat change, or a human-only step never does (#445).
- `aayu retro` lists unclassified Feedback and labels each fix Story with the layer it fixes (#340).
- `aayu evolve` classifies each correction by the layer that fixes it for good (#323).
- `aayu tell` refuses a draft that names a Feature without proof packs or uses a superlative (#284).
- `aayu tell` writes release notes in one shape: upgrade steps first, each with a check and who can do it, breaking changes in one table, a "Read this if you are…" block when readers need different sections, and each part only where a source supports it (#666).
- `aayu install --loops` sets up a product's verify sweep from the lockfile it finds (#322).
- `aayu build` stops before taking a Ready story that you did not ask an agent to build, and says what to ask you (#280).
- The Founder twin weighs operations, technology, and product, and counsels you rather than deciding (#309, #394).
- The seat twins take their names from the Hindu epics and the rishi tradition (#462).

### The merge gate and review

- The merge gate reviews every pull request that is not docs-only once it is marked ready for review (drafts wait) and its checks and proof pass, and `aayu build` asks for the review after each push (#466, #577).
- Without `--delegate`, `aayu build` has a reviewer model of another family review a verified checkpoint's branch before each push when such a model can be called and `[budget] per_pr_inspect_max` is above 0 (otherwise the push goes out without it and the PR body says why; a delegated build is reviewed only by the merge gate); on `fix` the push waits until the builder fixes each open item or declines it with `--decline "<id>: <why>"`, for at most three rounds within `[budget] per_pr_inspect_max`; a held checkpoint is pushed to `refs/aayudha/held/<branch>`, so `aayu build --resume` continues from it on another machine; and the PR body's "Session review (self-attested)" section never counts as a review (#663).
- inspect reviews a diff over the 120 kB prompt cap in parts of whole files, in the fewest parts that hold the files in order and at most three, in parallel, each with the same plan and criteria and with the pull request's one budget; the result takes the worst part's disposition, files after the third part and any file whose diff alone is over the cap stay unreviewed with that reason, and a head reviewed before this change is reviewed again in parts (#714, #783).
- The merge gate records which model family built a pull request and which rung of the review ladder reviewed it: A for a reviewer app of another family, B for a paid reviewer of another family, C for a labelled same-family review (#483).
- `aayu doctor` shows the rung your builds get and how to move up a rung (#522).
- The ship record, `aayu audit`, and `aayu status --streak` record each merge's review rung (#518).
- The autonomy streak counts only merges whose review was of another family or a person's; a review of a builder whose family is unknown is rung C (#510, #657).
- `[inspect] reviewer = "ci"`, the default, has the merge gate call the model that the repository's `code-review` environment names: the variables `AAYU_REVIEW_MODEL_URL` and `AAYU_REVIEW_MODEL`, the secret `AAYU_REVIEW_MODEL_KEY`, and the optional `AAYU_REVIEW_MODEL_API_STYLE` (`chat`, `responses`, or `messages`); review apps are optional (#657).
- `aayu init --gates` creates the `code-review` environment with Grok 4.7 at `https://api.x.ai/v1` filled in, lets only the base branches and the default branch deploy to it, and removes every other branch or tag deployment rule (other protection rules, such as required reviewers or a wait timer, stay as they are); on a plan with no environments it says so and continues (#657).
- The CI review fails closed and names the missing setting, refuses a URL with a key, a query, or an operation path such as `/chat/completions`, and skips a docs-only pull request before it reads the settings (#657).
- Rung B is a paid review model of another family: the CI review, or `[inspect] paid_reviewer`, which names the model when `AAYU_REVIEW_MODEL` names none and serves as the fallback (#550, #657).
- The work-items guidance that `aayu init` writes into AGENTS.md names the CI review as the default review, so agents follow the new review loop after you run `aayu init` again (#683).
- `[inspect] paid_effort` sets every review call's reasoning effort (empty: high, or medium for Claude Opus, Claude Fable, and GPT-6 Astra), and a paid review runs only for pull requests whose author has write or admin access to the repository (#579, #657).
- `[inspect] reviewer` names the one review tool: `ci` (the default), `codex`, `coderabbit`, `greptile`, `claude`, `cursor`, or `human` (`model` stays for aayu's own evals); `aayu install --apply` asks for a value the gate cannot use, and Enter picks `ci` (#641, #657).
- A review from CodeRabbit or Greptile counts as rung A whatever the builder's model family (#641).
- When the chosen tool answers with a usage-limit or quota message, or still has not replied when inspect runs again at least 30 minutes after the request (on a push, a rerun of the merge gate, a submitted review, or a comment from an app), the next installed app in a fixed order reviews (Codex, CodeRabbit, Greptile, Claude, Cursor), then the CI review, then a person; when the CI review is the chosen tool and cannot review, the apps review in that order, then a person; the comment names the tool that took over and why (#634, #641, #657). In CI, an app that reviews only when mentioned (Codex, Claude, Cursor) is asked only by `aayu inspect` run outside CI (#644); when the CI model is declined because its family is the builder's or the author lacks write access, the merge gate skips such an app unless someone asked it for that head, and CodeRabbit or Greptile reviews, its wait starting at the head's first inspect run even when `[inspect] app_families` gives it another family, then the approver (#724, #792).
- Every review call asks for fast (priority) serving where the endpoint offers it, and the review budget charges the priority rate for xAI models (#634, #657).
- inspect runs one paid review per head commit, and none on a reply event; a malformed reply is retried once, within the per-PR budget (#623).
- inspect reviews a pull request for at most three rounds, then stops until a person reopens the review, and findings in code the pull request did not change are tagged as pre-existing (#583).
- Agents run `aayu ship` on their Story pull requests; when only your approval is missing, the pull request waits in your inbox as Ready to ship (#577).
- Agent pull requests count toward the autonomy streak (#577).
- With `[autonomy] proof` on, `aayu ship` merges a pull request into your integration branch (`[branches] dev`, `dev` by default) with no person's pass when every required check passes (ship counts a skipped or neutral check as passing) except the CODEOWNERS check `protected-paths`, which the ship record lists instead, the proof pack verifies with its floors holding, inspect is at merge with a review rung listed in `[autonomy] proof_rungs`, and the pull request touches nothing in the narrow set (#617, #621).
- The narrow set is the `always_person` paths, a change to an `always_person_sections` table of an `always_person_config` file (by default 22 tables, from `[identity]` and `[autonomy]` to `[budget]`, `[scope]`, and `[build]`, listed in [CONSTRAINTS.md](CONSTRAINTS.md)), any workflow that holds a credential, any action manifest (`action.yml` or `action.yaml`), and any changed symbolic link; a pull request in it always waits for a person, whether it would merge on proof or on a layer's correction-free streak (#617, #686, #676, #685).
- Merging on proof is off by default, and `proof_rungs` defaults to rungs A, B, and C, so a same-family review counts for merge on proof but never toward the streak (#617, #657).
- Every merge `aayu ship` makes with no person's review, on proof or on a correction-free streak, gets the `aayu:audit` label before it merges (no label, no merge) and an entry on the ledger, and comes to `aayu inbox` as a "Merged unreviewed, to audit" card: looks right records a clean sample; a correction records the sample, adds `aayu:correction`, which resets the streak of each layer an agent's pull request touched, and opens a pull request into the branch it merged into that reverts the change (#661).
- `aayu ship` refuses to merge while a required check other than `protected-paths` is red, whoever passes it; it reads the checks once, so a check that turns red later can still pass under a bypass identity (#617, #632).
- `aayu init --gates` installs a security workflow that scans every push to the integration, rolling, and main branches that `[branches]` names (`dev`, `rolling`, and `main` by default), and each tip nightly, with verify's security floors (#500).
- `aayu verify` now runs the DAST floor, a ZAP scan, by default when `[verify] platforms` lists web or api: its active cross-site scripting and injection rules run only against an app that `[verify] seed` started on a loopback host in that run, or a URL whose environment sets `active_scan = true` in a merged `.aayudha/config.toml`, and every other URL gets a passive scan, which cannot see a reflected XSS (#509).
- The DAST floor runs ZAP under Docker or Podman; a local run with neither skips and leaves the scan to CI, where a missing runtime blocks the floor; a high alert, or a web or api story with no URL to scan, is advisory until `[verify] new_floors_blocking_from` (2026-11-14) and from then blocks ship unless a floor waiver names dast; the security workflow never runs the seed, so it scans only a deployed environment's URL; and `aayu doctor` shows the target, the scan mode, and the runtime (#509).
- verify takes an advisory UI review of its own screenshots (#479).
- A floor waiver counts only when the floor-waiver authority accepted it (#624).
- Agents run `aayu release` behind the full gate; promotion to main needs a person's stable-release request (#443).
- `aayu release --to rolling` promotes only a commit on your integration branch (`[branches] dev`) that already has eval results, and never starts evals itself (#524).
- Every pull request body follows one format, `.github/pull_request_template.md` (what this does, why, what changed, how we know it works, risk and rollback, and the detail for reviewers), which `aayu init` installs and `aayu build` fills from the Story; `aayu inspect` checks it, and on an agent's pull request a missing or empty section or a sentence over 20 words holds the pull request at `fix`, while other pull requests get notes only (#672).
- `aayu build` writes an `Aayudha-Effort` line after the `Aayudha-Model` lines; on Amp it reads the model from the thread's usage table and the effort from its agent mode, and shows a known model by name, such as "Claude Opus 5.5, medium effort" (#672).
- The inspect comment, the Ready snapshot, the Decision pack outline, and every skill follow one writing rule, Simplified Technical English, in the voice of the seat that owns the verb; ship's Ready to ship comment names the approver (#672).

### Identity and trust

- `aayu inbox` shows a "Needs your review" card for each pull request the merge gate holds for a person; the card asks what you checked and writes the same record as `aayu inspect --approve`, so one session's device code approves several pull requests, and nothing is recorded if the head moved since you read the card (#678).
- A human-only command in a shell an agent started, or with no terminal, runs only after you approve it from any device through GitHub's device flow, so it works from a remote shell such as an orb or a runner (#192, #467).
- In a shell with an agent marker, the full-screen inbox asks for that approval on its own first screen: the code in large type, Enter or `o` to open github.com/login/device where a browser can be opened here, `c` to copy the code, a countdown to its expiry, and "Approved by @login" once you approve, with Enter for a new code after one expires; the code is never logged or stored (#725).
- `aayu decide a,b --accept x,y` puts up to 10 Decision acceptances into one batch that a recorded approver accepts with a reply, when the repository has the aayudha App, with one device code as the fallback and for one-way doors (#501).
- `aayu init --gates` protects every host's executable config with CODEOWNERS, and `aayu doctor` fails when it drifts (#538).
- A host's PostHog or Sentry MCP server or CLI may only read; the host guard sees a CLI behind `sh -c`, `env`, and similar wrappers and a server by its registered name, URL, command, or package, but not a CLI started from a script file or an alias, or a server behind a generic proxy (#489, #525).
- Each PostHog or Sentry read the host guard recognises is logged and citable like aayu's own queries, with tokens typed inline redacted (#494, #517).
- Host recordings, replays, and feedback reads need `allow_pii`, which counts only when committed on your integration branch (`[branches] dev`) (#523, #531).
- The host guard covers PostHog and Sentry calls under GitHub Copilot, and a Copilot shell call that exits non-zero is logged as failed (#537, #556).
- `aayu doctor` tries each PostHog and Sentry capability with one cheap read and names the exact fix (#499).
- A checkout's config can no longer send a stored integration key to another host (#602).
- A pull request can no longer change protected settings through `.aayudha/config.json` or a committed `.aayudha/config.local.toml` or `config.local.json`: in CI the local files are skipped with a notice, `[decisions] solo_provisional_after_hours` and `[integrations.<p>] allow_pii` count only from `.aayudha/config.toml`, and the `.aayudha/.gitignore` that `aayu init` writes also ignores `config.local.json` (#660).
- Config keys now match only in their exact case, so a case variant such as `Allow_PII` is refused as an unknown key instead of setting the setting (#660).
- `aayu ship` reads its human-pass policy (the bot logins, the approver, the never layers, the paired seats, and the streak) from the base branch's committed `.aayudha/config.toml` and `LAYERS.md`, and stops, naming the setting, when your checkout, an `AAYU_*` variable, or `--set` would loosen it; a pull request cannot drop a never layer by editing its own `LAYERS.md` (#661).
- inspect reads its own state only from a comment its trusted writer posted, in every reader (#605).
- `aayu init --identity` stops at once in a remote shell, such as an orb's Terminal tab or SSH, and creates no App; on this machine it exits 4 with a next step when no redirect arrives within 10 minutes (#603).
- `aayu init --identity` prints no Install link and writes no secret when the App's conversion fails, and refuses an App with another owner, an unrequested permission or event, or another webhook (#603).
- `aayu seat` creates the codeowner-review ruleset when a second person joins a repository that init adopted (#516).
- `aayu inspect <pr> --approve --reason "<what you checked>"` records the `[autonomy] approver`'s review of the pull request's current head on the ledger, after GitHub's device flow every time, and posts it as the human review the merge gate reads (#647).
- `aayu init` protects the `brain` branch with the ledger's rulesets, so no one can force-push or delete it; when an adopted ruleset does not cover it, init says what to add and never edits the team's ruleset (#674).

### Evals

- Promotion waits on more eval groups: tell (#304), host-skills (#427), and retro in its own group (#447).
- CI fails a pull request that lowers a skill-eval bar without an accepted waiver (#407).

### Installs, hosts, and models

- GitHub Copilot is a host: `aayu install --host copilot --apply` writes its skills, MCP server, hooks, and post-tool hook (#491, #549).
- `aayu install --env --apply` installs a pre-push hook that runs the `[[gate]]` checks in `.aayudha/env.toml`, one local gate that mirrors your required CI jobs (#418).
- The `triage-daily` loop runs at 09:00, 13:00, 17:00, and 21:00 each weekday instead of once each morning, and a loop scheduled by weekday whose local hour falls on the next day in UTC now runs in every timezone; a day-of-month schedule still refuses such an hour (#682).
- A new intake issue is triaged when it opens in Amp and Cursor, through the new `triage-opened` loop, and `triage-daily` still covers Claude Code and Codex (#684).
- In a shell with an agent marker, such as an orb's Terminal tab, `aayu install --loops --apply` asks the person at the terminal for a GitHub device code instead of refusing; with no terminal, or under `AAYU_AGENT=1` or `--ci`, it still refuses (#698, #702).
- `aayu build` has a 60 USD cost limit that applies only to metered spend, and pauses at a subscription's usage limit (#490).
- UI review runs on Gemini 4 Argon through GitHub Copilot when Copilot is signed in, and on Gemini 3.8 Flash at high effort through the AI gateway otherwise (#434, #460, #548).
- `[hosts.cursor] plan` picks lane defaults that your Cursor plan includes (#535).
- `aayu doctor` names Copilot's review model and lists the model families Copilot has enabled (#555).
- Copilot's build lane names `claude-sonnet-5.5` rather than an Auto tier (#574).
- Model calls stream their replies, so a long survey draft or a long review passes the gateway's proxy (#317, #609).
- Eval and budget calls are charged at published list prices, with Anthropic's one-hour cache writes at twice the input price (#321, #448).
- Each host gets one copy of each skill, the same for every teammate, and lists each skill once when the plugin is enabled (#279, #286).
- Codex runs through its own provider and passes the GitHub token variables to the MCP server, and the local Codex receiver keeps `CODEX_HOME` (#359, #392, #453).
- Cursor runs its hooks once, and the plugin's commands come from install (#297).
- Rolling snapshot builds are named after the newest v* release (#492).
- The winget package identifier is `AkmolsInc.AgenticAyudha`, published by Akmols Inc, and `aayu self-update` prints `winget upgrade --id AkmolsInc.AgenticAyudha`; the reverse-DNS identifier `app.akmols.agenticayudha` is reserved for platforms that need one (#342, #831).
- Every skill has its own page in the guide, generated from the verb registry (#455).
- New guides cover the delivery loop, working as a solo founder, growing from an MVP, and connecting PostHog and Sentry read-only (#463, #473, #474, #507).

### Breaking changes

- Removed: `[inspect] reviewer = "auto"` and `"amp"`, with no deprecation release: a repository left on either gets a person's review on every pull request that is not docs-only, once it is ready for review and its checks and proof pass, and `aayu install --apply` asks for a tool again (#641, #657).
- Changed: a repository that names no review tool now uses the CI review: its merge gate fails, naming the missing setting, until a person runs `aayu init --gates` and adds `AAYU_REVIEW_MODEL_KEY`, or names a review app in the pull request that commits the new merge gate (#657).
- Changed: in inspect's human fallback, a reply no longer counts on its own, even from the approver's account: only the `[autonomy] approver`'s approval of that exact head counts, through `aayu inspect <pr> --approve --reason "<what you checked>"` or its card in `aayu inbox` (#678), so a new push needs a new approval and a repository with no `[autonomy] approver` cannot clear the fallback (#647).
- Changed: a `Solo-mode bypass:` comment counts only when it names the pull request's head commit (seven or more hex characters); a bypass written for an earlier head no longer counts after a push, so write the head commit into a hand-written one (#848).

### Fixes

- Every model gateway call gets the model call limit, `CallTimeout`, even when the caller passes the CLI's general 5-minute client, so a CI review that streams past 5 minutes is no longer cut off and left to a review app (#688).
- The merge gate's inspect job now has 30 minutes instead of 15, and one review call, with its retry, has 25 minutes instead of 12, so a long CI review stream is no longer cut off; run `aayu init --gates` to update your workflow (#699).
- With merge on proof off, a pull request that touches the narrow set no longer merges on a layer's correction-free streak alone: `aayu ship` reads the narrow set before the proof setting, stops and names the path or table, and fails closed when it cannot read a file (#686).
- `aayu init --gates` workflows fetch only aayu directly, so a go.googlesource.com outage no longer fails them; run `aayu init --gates` to update (#811).
- `aayu ship` reads the autonomy streak only when no human pass is settled; after an approving review, a merge on proof, or a solo-mode bypass it records the streak as skipped, with the reason, and no longer spends about two minutes reading GitHub history (#703).
- `aayu install --loops --host amp --apply` now records an Amp event loop's subscription: the setup thread updates and enables an existing equivalent subscription with the current instructions, and install waits up to five minutes for the thread's final reply and takes the subscription ID only from it (#710).
- A re-install with `aayu install --loops --host amp --apply` no longer leaves a scheduled loop running on two threads: the new setup thread is asked to pause the old schedule that `loops/active.json` recorded once its own is set, install names that old thread and says nothing confirms the pause, and a recorded ID that names no thread gets a step to pause it by hand (#718).
- When the CI review fails on a head, the inspect comment and the job log now say why, with the gateway's address shown as `<endpoint>` (#689).
- `aayu ship` and `aayu proof push` read the proof media bucket and the name of its secret only from the defaults, your user config, and the integration branch's committed config, in a repository you can push to (an App installation must include it); a checkout, `config.local.toml`, `AAYU_*` variable, or `--set` that names other values stops the run before any secret is read (#681).
- The `.agents/setup` that `aayu install --env` writes now puts aayu on PATH in `~/.local/bin`: in aayu's own repository it builds aayu from the checkout and skips the build when the binary already carries that commit, and in a product repository it installs the pinned release after checking its checksum, so a loop thread no longer builds or fetches aayu itself (#821).
- `aayu brain check` now fails a note that shares its name with an attachment, and generated-block markers that do not pair in order (an END with no BEGIN, a BEGIN with no END, a nested BEGIN, or an END that names another block); `aayu brain checkpoint` stops on the same markers instead of writing a second block beside stale text (#829).
- A GitHub read can no longer hang `aayu inbox`: each GitHub request gives up after the run's HTTP timeout, 5 minutes by default, and one inbox read stops after 3 minutes, even the first, keeps any cards it has, and says GitHub did not answer, so R tries again; a finished refresh now says "Up to date." instead of leaving "Reading what changed on GitHub…" on screen, and a read that is cut off no longer marks a pull request's issue as seen, so the next refresh reads it again (#828).
- The approvals row of `aayu doctor` now warns instead of failing when no approvers are recorded, since then no reply can approve anything, so a fresh macOS or Windows machine whose git credential helper has nothing stored no longer shows a FAIL row; with approvers recorded, a credential that could act as one still fails it, and the installers workflow now also runs when `internal/doctor` changes (#832).
- A person's `/aayu review again` on a pull request stopped at the review cap now starts the merge gate's reply job and runs inspect again on the same head, with no push; the job starts only for the repository's owner, members, and collaborators, aayu still checks the author's write access before it runs inspect again, and a leading space before the command counts; run `aayu init --gates` to update your merge gate (#849).
- In solo mode, `aayu inbox` and `aayu ship` post the Solo-mode bypass only after every ship check has passed, naming the head commit; ship refuses without posting while the pull request conflicts with its base or GitHub has not worked out whether it can merge, and deletes the bypass it posted if the merge then fails (#848).
- `aayu inbox` no longer refuses a choice with "it changed on GitHub since you read it" when only a card's words changed: it refuses only when the item's kind, its head commit, or the chosen option changed, or, for a Decision, its title, its text, whether it is one-way, or whether the option is recommended; the refusal names what changed, and with `-v` the session log lists the other differences (#850).
- A pull request at the review cap that names no Story no longer offers a ship that always refuses; its card says to link a Story or merge it on GitHub (#850).
- A paid CI review whose summary runs over 1200 characters is no longer thrown away, with its findings and verdict, leaving the pull request to a person: inspect cuts the summary at a word boundary and ends it with an ellipsis, and every other limit on the reply still rejects it (#809).
- On Windows, a `.aayudha` that is a file, not a directory, now fails the config load and names the path, as on Linux and macOS; it no longer reads as no config (#908).
- `aayu tell`'s draft check no longer reads a GitHub alert marker, such as `> [!NOTE]`, as an exclamation mark; a `!` in the alert's text is still refused (#666).
- When a reviewer app or a person reviews, the inspect comment's footer now shows the pull request's `[budget] per_pr_inspect_max` cap instead of "of 0.00 USD"; only the footer changed, and a diff reviewed in parts can still spend past the cap (#781, #714).
- A CI review that fails for a passing reason (a 429 or 5xx reply, an overload or server error in the stream, or a stream that ends early or is reset) is retried once, within the same call time limit and the per-PR budget; the failure note hides IP addresses and names the model that failed (#697).
- CodeRabbit's summary block in a PR body no longer changes inspect's cache key, so its edit does not make the gate pay for a second CI review of the same head (#696).
- With `reviewer = "ci"`, an `aayu inspect` or `aayu build` run outside CI no longer asks a review app or a person under your account; inspect reports the merge gate's verdict for the head, or `no-reply` until the gate has one; once the gate records that the CI review failed on that head, a local run asks the fallback app, then a person (#691).
- The chart and measure skills check each number against its own source, name its population and window, and keep a startup note to one page (#312, #315, #341, #348, #356, #369, #373, #377, #385, #386, #402, #411, #420, #423, #425, #429, #437, #456, #464, #472, #528, #534, #539, #559, #573, #581, #591, #600).
- The investigate, slo, runbook, and respond skills keep causes, rule-outs, and figures to the record and cite exact signatures (#375, #380, #387, #389, #412, #424, #428, #438, #452, #461, #527, #543, #563).
- respond names the trigger the evidence shows as a contributing condition, and phrases open questions without presuming their answer (#625).
- decide packs and retro answers say only what their sources state and stay within their length (#281, #296, #303, #316, #345, #351, #357, #361, #365, #368, #372, #433, #440).
- counsel says only what its source says, names a claimed approval without repeating it, and gives no assurance from floors alone (#558, #569, #612).
- The research, wireframe, mockup, prototype, review, polish, and baseline skills keep one choice as one group, reuse the page's design values, and keep a mockup's saved state working (#311, #338, #347, #354, #383, #391, #410, #426, #439, #470, #506, #508, #597, #626).
- The build, verify, and ready skills write the evidence and feature maps without asking, read back the whole Completion Goal, and wait a turn for your yes (#300, #314, #326, #344, #355, #362, #374, #415). build says what an in-step feature map is, and words a wait or a refusal so that it never claims a build has started (#858).
- survey verifies every claim against the code and cites a code path or a run for every fact about behaviour, output, numbers, or defaults (#353, #366, #381, #419, #454, #584, #619).
- tell restates criteria without extrapolating (#291), keeps a negative criterion negative (#857), and gives no reason for the changes it leaves undescribed (#896); slice adds no restrictions the Feature does not state (#308).
- inspect reports only defects it can show from the diff, posts a peer finding once, and counts Codex's security-review findings in its review (#288, #310, #567).
- Skill drafts keep unverified markers out of table cells and leave out claims that something is absent (#421). Every seat's evidence rule now forbids a claim beyond its source (#866).
- The seat evidence check counts Bicep and Terraform citations (#363).
- The `doctor` mcp row counts plugin servers, and the agents row shows versions (#459).
- The design token check skips story references in JS and CSS comments and names each literal's line (#506).
- The keychain probe never starts a session dbus-daemon, and the credential scan never reads WTW variables (#533, #545, #564).
- `aayu ship`, the inspect reply job, and the inbox ignore the skipped merge-gate checks of a reply run, so a comment on a pull request no longer hides the gate's passing checks (#690, #643, #693).

## v0.2.0

Released 2026-10-17.

v0.2.0 moves review out of your CI: the merge gate asks an installed agent app for the review, so a product
repository needs no AI key (#225). verify gains feature maps and behaviour checks (#215, #218). `aayu install --env`
sets up the environment each agent host's cloud works in (#240), and hand-offs reach any agent from any host
(#220, #224). Loops run on schedules in the one tool you pick (#248, #261). No Feature is claimed as proven for this release:
none of its Features has an authoritative proof pack yet.

### New verbs and skills

- `aayu verify --map` checks a feature map, the list of what your product does and how to drive it, for drift (#215).
- The behaviour floor runs a feature map's checks in the merge gate (#218).
- `aayu init --verify-skill` writes a verify skill for your product, and install mirrors it to each host (#227, #235).
- The examples include feature maps for `benefits-qa` and a notes CLI (#223).
- `aayu verify --maintain` posts a brief to keep the maps current, and nightly and weekly loops run verify (#230).
- grill and plan are grounded in the feature maps (#236).
- The feature-map floor is advisory until its `drift_blocking_from` date and blocks from that date (#244).
- For a Bug story, verify reruns its proving tests on the base commit to show they failed before the fix (#229).
- grill unpacks a vague request with questions before any skill acts on it, and a concrete request passes with none (#226).
- A vague request becomes a Story directly, with no parent chain (#274).
- `aayu retro` reports flow and review-quality metrics, without a model and without gating (#217).
- `aayu evolve` needs two accepted Feedback issues for a skill, at most once a month, and watches whether the correction recurs (#221).

### The merge gate and review

- inspect judges missing or weak tests against the criteria, counts tests that add value, and flags weakened checks (#216).
- The proof comment opens with a risk line computed from the change and lists weakened checks (#228).
- A failed required check goes back to the builder, within the fix-cycle cap (#231).

### Identity and trust

- `aayu init` lets people with write access write the ledger and blocks rewrites of it for everyone (#210).

### Installs and hosts

- `aayu install --env` sets up, verifies, and detects the environment each agent host's cloud works in, for any repository (#240).
- `aayu build --delegate` decides which receivers are ready from that detection and checks the cloud environment before a hand-off (#242).
- Every host's plugin offers hand-offs to the other agents, never to itself (#220).
- Each hand-off starts fresh, Claude Code builds in its own cloud sessions, and the receiver's proof preview comes back with the result (#224).
- Hand-offs go to the agent's cloud by default and run locally only with `--local` (#232).
- Local receivers can verify, report what they proved, and keep their report when they change nothing (#270).
- `aayu install --loops` sets up loops on each agent host, including Claude Code routines and Codex scheduled tasks, and `aayu doctor` reports them (#248, #250).
- Loops that need no model run in the free GitHub Actions fallback (#265).
- The daily triage loop runs on weekdays only, and the weekly triage loop is gone (#266).
- `aayu install` adds the default loops in a fresh repository (#272).
- Stories move to in progress when built, `aayu status` counts open pull requests, and story titles drop their type prefix (#275).
- The README, a tour-based first hour, situation guides, and `llms.txt` lead the docs (#255, #258).
- A guide shows how aayu unpacks a vague request, with a real before and after (#239).

### Breaking changes

- Changed: the merge gate no longer calls a model or needs an AI key: `aayu inspect` asks an installed agent app of another family for the review, and falls back to your approver when there is none (#225).
- Removed: the triage workflow from `aayu init --gates`, which now offers to remove an installed one; triage runs from your agent (#243).
- Changed: loops run on schedules only, in the one tool you pick; the event loops added earlier in this cycle (#247) were removed before release (#261).

### Fixes

- grill's conversation keeps declined topics out of the criteria, asks what must not change, and asks one decision per question (#238, #254, #256, #257, #260, #263).
- install archives the Amp probe thread once it reads the reply, and names each Amp loop thread `aayudha loop: <name>` (#245, #262).
- init finishes when the plan has no rulesets, and doctor reports it (#271).
- Codex hooks are trusted, requests route to the right agent, the server registers once, and the person's next step is named (#276).

## v0.1.0

Released 2026-10-15.

v0.1.0 is the first release of aayu. It runs a product's work on GitHub with coding agents: work items as issues
(#18), checkable criteria frozen as Ready (#73), a build by an agent in its own worktree (#109), a merge gate that
checks the proof against the criteria (#86), a person's approval or an earned autonomy streak to ship (#88, #89), and
promotion through rolling to main (#110). Seats with agent twins share the work, and each verb has one accountable
seat (#83, #167).

### Proven Features

Four Features have an authoritative proof pack for every shipped story. Three of them belong to `benefits-qa`, the
example product the thin-slice run built:

- Feature #37, Thin slice run: `aayu doctor` names the floor each missing tool serves (#92).
- Feature #127, Grounded answers API: the plan documents store loads the synthetic Example Co seed (#142); answers quote the top-ranked plan passage, and policy refuses or refers the rest (#148); `POST /api/ask` answers within its JSON Schema contract and records events (#155).
- Feature #128, Answer analytics: product events export to Application Insights, and the synthetic exports reproduce (#158).
- Feature #129, Ask page: the Ask page shows answers with their sources, served on seeded data (#160).

### New verbs and skills

- Each verb is defined once in a registry that generates its skill's frontmatter, the docs tables, and the MCP schemas, and a lint checks the skill format (#1, #17).
- The `aayu` CLI has layered config, and `aayu` alone in a checkout shows a home screen with your next command (#11, #164).
- `aayu next`, `aayu tour`, live progress, seat voices, and shell completions come with the home screen (#164).
- Work items are GitHub issues of ten types, with issue forms and labels (#18).
- `aayu init` sets a repository up, including projects, gates, the bot App, and integrations, and `aayu uninit` removes what it created (#23, #26).
- `aayu ready` checks a story's numbered criteria and posts the Ready snapshot, and the Ready guard removes `status:ready` when the body drifts (#73).
- `aayu status` rolls work up and writes a digest, and triage only labels (#75).
- `aayu grill` is a conversation that turns a vague idea into a concrete spec, and covers all ten work-item types, baselines, and the brand (#101, #166).
- `aayu slice` and `aayu plan` break work down with layer footprints, dependencies, and the Engineering Manager's plan review (#107).
- `aayu build` builds a Ready story in its own worktree with leases, verified checkpoints, a budget, and resume, and `aayu ship` merges it after a person's approval, or on its own once the autonomy streak allows and no never-layer is touched (#89, #109).
- `aayu build --delegate` hands a Ready story to Codex, Cursor, Claude Code, or Amp, and closes the review loop (#187).
- `aayu gc` reclaims expired leases and says how to free a build worktree it keeps (#109, #113).
- `aayu verify` runs floors: diff coverage, SAST, secrets, dependencies, web accessibility, performance, DAST, API, and a screen reader (#81, #84, #85).
- `aayu verify` writes the proof pack, `aayu proof` pushes, lists, fetches, and checks packs, and `aayu ship` lands each pack on the ledger, a branch that holds the record of the work; media stay in the merge gate's run artifact or an optional bucket, never in the ledger or Git LFS (#20, #78, #159, #206).
- research, wireframe, mockup, and prototype work from pinned references behind a Figma diff gate (#94).
- review, polish, and baseline check UI work in two lanes (#99).
- A design system ships tokens, density modes, shadcn primitives, Storybook with axe, seven floorplans, and SwiftUI and Compose token parity (#9, #10).
- `aayu experiment` and `aayu readout` run experiments on PostHog and GrowthBook with a minimum sample size (#82).
- `aayu measure` snapshots exports to SQLite, computes AARRR cohorts and a North Star, and writes Results on Goals and Initiatives (#87).
- `aayu decide` and `aayu respond` work from Decision issues and the ops catalog (#96).
- `aayu brief` explains work in four registers, from plain to regulatory, grounded in each pack's criteria and inspect's verdicts (#97, #108, #115, #119, #139).
- `aayu tell` writes copy that claims only proven Features, tags links, and publishes only under a person's account (#102).
- `aayu tell docs --site` builds the docs site, and the installers are checked on each OS (#123, #137).
- `aayu audit` and `aayu retro` write reproducible reports reconciled against GitHub's event history (#105, #118).
- `aayu evolve` changes a skill only from accepted retro Feedback, eval first, and CI enforces it (#114).
- `aayu release` promotes a Release issue through rolling to main (#110).
- Loops run aayu commands on a schedule in Amp, Cursor, or a GitHub Actions fallback (#104).
- Twelve seats have charters, a generated RACI, personas, practice playbooks by team stage, and `aayu seat add|remove|list` (#83, #167).
- PostHog, Sentry, GitHub, Figma, Mobbin, Azure, AWS, and Google Cloud have read adapters, and `aayu init --integrations` sets the clouds up (#79, #80, #157, #161).

### The merge gate and review

- The merge gate runs verify on the pull request head from the base branch's workflow, and inspect reviews the change (#86).
- inspect reviews agent pull requests with context from the code, filters noise, and in peer mode has two reviewers of other families check each other (#186).
- A plan-review bypass in solo mode is recorded as a bypass, never as an approval (#121).
- `aayu audit` lists late-recorded solo-mode bypasses apart and marks merges after the gate that have no proof (#118).

### Identity and trust

- Secrets live in the OS keychain or an encrypted file, and a token broker keeps a person's identity apart from the bot's; only a person, never a bot, gives ship its human pass (#12).
- Every write goes through an actions log, and model calls go through one gateway with a budget breaker (#14).
- The autonomy streak, corrections, and `aayu halt` and `aayu resume` decide when agents may merge on their own (#88).
- Releases are signed, and a CI suite checks every pull request (#8).

### Evals

- A two-tier skill eval harness scores the skills, and `evolve` needs it to pass (#22, #114).

### Installs and hosts

- `aayu install` writes skills, a lock, MCP registrations, and hooks for each host (#19).
- `aayu mcp` is a stdio MCP server whose tools come from the verb registry (#21).
- `aayu doctor` checks the setup, `aayu upgrade` migrates config files, and telemetry stays local unless you set an endpoint (#15).
- Codex is a host, Claude Code and Codex install from a plugin marketplace, and Amp gets a plugin (#169).
- Guides tell the journey of a real run in the second person, with a page for each agent host (#163, #165, #178, #200).

### Breaking changes

- Changed: proof packs are kept as merge-gate run artifacts and `aayu ship` lands the pack on the ledger; `aayu proof purge` is removed, and a merge gate from an earlier snapshot must be rendered again with `aayu init --gates` (#206).

### Fixes

- The deps floor scans only the lockfiles git tracks and uses osv-scanner's call analysis (#95).
- ship checkpoints its actions with the verb's token, on every exit, into one log per clone (#98).
- axe runs on pages whose Content Security Policy forbids injected scripts (#124).
- build exits 5 when it cannot read the Ready snapshot, and records a checkpoint whose pull request failed (#138).
- release stamps every story a merged pull request refers to, and none that never merged (#146, #149).
- slice keeps stories to the Feature's scope, and grill and plan add nothing the request did not ask for (#111, #117, #150).
- The budget breaker is seeded from the actions log, so it counts every process (#180).
- inspect takes the builder's family from build's ledger record, and reviews a delegated pull request in general mode by default (#182, #195).
- decide keeps a decision pack short enough to decide in five minutes (#176).
- Every workflow job starts with the halt check (#179).
- audit hashes only the configuration it reads, so reports replay across new keys (#185).
- `status --work` finds worktrees by their resolved path (#197).
- verify finds floor tools through PATHEXT on Windows, and the ledger waits for a lock another writer is deleting (#201, #202).
- release retries a push the remote failed to apply (#205).
- goreleaser ignores rolling-* tags (#77).
