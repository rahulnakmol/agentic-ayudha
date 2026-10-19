<!-- markdownlint-configure-file { "MD024": { "siblings_only": true } } -->
# Changelog

What changed in aayu (the `aayu` CLI, its skills, and the workflows it installs), newest version first.

Each item cites the merged pull request it comes from as (#n): pull request n in
`github.com/rahulnakmol/agentic-ayudha`. A Feature is written as Feature #n: issue n, not a pull request. An item says what merged. It does not say the work is proven. A Feature is
proven only when every shipped story under it has an authoritative proof pack (the record the merge gate checks
against the story's criteria), and each version below says which Features that is. Words such as Ready, proof pack,
ledger, and floor are explained in [GLOSSARY.md](GLOSSARY.md).

## v0.3.0

Unreleased: these changes are on `dev` and in the v0.3.0 freeze. The list is final when v0.3.0 is tagged.

With v0.3.0, aayu builds its own repository, agentic-ayudha, with one person, @rahulnakmol. With `[autonomy] proof` on,
`aayu ship` merges an agent's Story pull request into dev on proof alone, and on that path a narrow set of paths and settings always
waits for a person (#617). Two new seats, the Production Engineer and the Data & AI Architect, bring
skills for incidents, SLOs, runbooks, and data designs (#337, #293), and `aayu survey` maps legacy code (#292). Every pull request
that is not docs-only now gets a review once it is ready for review and its checks and proof pass, by default from a model you
choose in your own CI, and the merge gate records how independent that review was (#466, #577, #657, #483). You approve the
human-only steps agents queue for you from any device, one at a time in `aayu inbox`, and can accept several Decisions with one
reply (#442, #467, #678, #501). No Feature is claimed as proven for this release: none of its Features has an authoritative
proof pack yet.

### New verbs and skills

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
- Every Decision offers at least two numbered options, and you pick one by its own number (#498, #551).
- In solo mode, a reversible Decision applies its recommendation provisionally after 24 business hours; a waiver, a seat change, or a human-only step never does (#445).
- `aayu retro` lists unclassified Feedback and labels each fix Story with the layer it fixes (#340).
- `aayu evolve` classifies each correction by the layer that fixes it for good (#323).
- `aayu tell` refuses a draft that names a Feature without proof packs or uses a superlative (#284).
- `aayu install --loops` sets up a product's verify sweep from the lockfile it finds (#322).
- `aayu build` stops before taking a Ready story that you did not ask an agent to build, and says what to ask you (#280).
- The Founder twin weighs operations, technology, and product, and counsels you rather than deciding (#309, #394).
- The seat twins take their names from the Hindu epics and the rishi tradition (#462).

### The merge gate and review

- The merge gate reviews every pull request that is not docs-only once it is marked ready for review (drafts wait) and its checks and proof pass, and `aayu build` asks for the review after each push (#466, #577).
- `aayu build` has a reviewer model of another family review the branch before each push; on `fix` the push waits until the builder fixes each open item or declines it with `--decline "<id>: <why>"`, for at most three rounds within `[budget] per_pr_inspect_max`; a held checkpoint is pushed to `refs/aayudha/held/<branch>`, so `aayu build --resume` continues from it on another machine; and the PR body's "Session review (self-attested)" section never counts as a review (#663).
- The merge gate records which model family built a pull request and which rung of the review ladder reviewed it: A for a reviewer app of another family, B for a paid reviewer of another family, C for a labelled same-family review (#483).
- `aayu doctor` shows the rung your builds get and how to move up a rung (#522).
- The ship record, `aayu audit`, and `aayu status --streak` record each merge's review rung (#518).
- The autonomy streak counts only merges whose review was of another family or a person's; a review of a builder whose family is unknown is rung C (#510, #657).
- `[inspect] reviewer = "ci"`, the default, has the merge gate call the model that the repository's `code-review` environment names: the variables `AAYU_REVIEW_MODEL_URL` and `AAYU_REVIEW_MODEL`, the secret `AAYU_REVIEW_MODEL_KEY`, and the optional `AAYU_REVIEW_MODEL_API_STYLE` (`chat`, `responses`, or `messages`); review apps are optional (#657).
- `aayu init --gates` creates the `code-review` environment with Grok 4.7 at `https://api.x.ai/v1` filled in, lets only the base branches and the default branch deploy to it, and removes every other deployment rule; on a plan with no environments it says so and continues (#657).
- The CI review fails closed and names the missing setting, refuses a URL with a key, a query, or an operation path such as `/chat/completions`, and skips a docs-only pull request before it reads the settings (#657).
- Rung B is a paid review model of another family: the CI review, or `[inspect] paid_reviewer`, which names the model when `AAYU_REVIEW_MODEL` names none and serves as the fallback (#550, #657).
- The work-items guidance that `aayu init` writes into AGENTS.md names the CI review as the default review, so agents follow the new review loop after you run `aayu init` again (#683).
- `[inspect] paid_effort` sets every review call's reasoning effort (empty: high, or medium for Claude Opus, Claude Fable, and GPT-6 Astra), and a paid review runs only for pull requests whose author has write or admin access to the repository (#579, #657).
- `[inspect] reviewer` names the one review tool: `ci` (the default), `codex`, `coderabbit`, `greptile`, `claude`, `cursor`, or `human` (`model` stays for aayu's own evals); `aayu install --apply` asks for a value the gate cannot use, and Enter picks `ci` (#641, #657).
- A review from CodeRabbit or Greptile counts as rung A whatever the builder's model family (#641).
- When the chosen tool answers with a usage-limit or quota message, or still has not replied when inspect runs again at least 30 minutes after the request (on a push, a rerun, or a new comment or review), the next installed app in a fixed order reviews (Codex, CodeRabbit, Greptile, Claude, Cursor), then the CI review, then a person; when the CI review is the chosen tool and cannot review, the apps review in that order, then a person; the comment names the tool that took over and why (#634, #641, #657).
- Every review call asks for fast (priority) serving where the endpoint offers it, and the review budget charges the priority rate for xAI models (#634, #657).
- inspect runs one paid review per head commit, and none on a reply event; a malformed reply is retried once, within the per-PR budget (#623).
- inspect reviews a pull request for at most three rounds, then stops until a person reopens the review, and findings in code the pull request did not change are tagged as pre-existing (#583).
- Agents run `aayu ship` on their Story pull requests; when only your approval is missing, the pull request waits in your inbox as Ready to ship (#577).
- Agent pull requests count toward the autonomy streak (#577).
- With `[autonomy] proof` on, `aayu ship` merges a pull request into dev with no person's pass when every required check passes (ship counts a skipped or neutral check as passing) except the CODEOWNERS check `protected-paths`, which the ship record lists instead, the proof pack verifies with its floors holding, inspect is at merge with a review rung listed in `[autonomy] proof_rungs`, and the pull request touches nothing in the narrow set (#617, #621).
- The narrow set is the `always_person` paths, a change to an `always_person_sections` table of an `always_person_config` file (by default 20 tables, from `[identity]` and `[autonomy]` to `[budget]`, listed in [CONSTRAINTS.md](CONSTRAINTS.md)), any workflow that holds a credential, any action manifest (`action.yml` or `action.yaml`), and any changed symbolic link; with merge on proof on, a pull request in it waits for a person, and with it off, ship does not read the narrow set, so a layer's correction-free streak can merge one (#617, #89).
- Merging on proof is off by default, and `proof_rungs` defaults to rungs A, B, and C, so a same-family review counts for merge on proof but never toward the streak (#617, #657).
- `aayu ship` refuses to merge while a required check other than `protected-paths` is red, whoever passes it; it reads the checks once, so a check that turns red later can still pass under a bypass identity (#617, #632).
- `aayu init --gates` installs a security workflow that scans every push to dev, rolling, and main, and each tip nightly, with verify's security floors (#500).
- verify takes an advisory UI review of its own screenshots (#479).
- A floor waiver counts only when the floor-waiver authority accepted it (#624).
- Agents run `aayu release` behind the full gate; promotion to main needs a person's stable-release request (#443).
- `aayu release --to rolling` promotes only a dev commit that already has eval results, and never starts evals itself (#524).

### Identity and trust

- `aayu inbox` shows a "Needs your review" card for each pull request the merge gate holds for a person; the card asks what you checked and writes the same record as `aayu inspect --approve`, so one session's device code approves several pull requests, and nothing is recorded if the head moved since you read the card (#678).
- A human-only command in a shell an agent started, or with no terminal, runs only after you approve it from any device through GitHub's device flow, so it works from a remote shell such as an orb or a runner (#192, #467).
- `aayu decide a,b --accept x,y` puts up to 10 Decision acceptances into one batch that a recorded approver accepts with a reply, when the repository has the aayudha App, with one device code as the fallback and for one-way doors (#501).
- `aayu init --gates` protects every host's executable config with CODEOWNERS, and `aayu doctor` fails when it drifts (#538).
- A host's PostHog or Sentry MCP server or CLI may only read; the host guard sees a CLI behind `sh -c`, `env`, and similar wrappers and a server by its registered name, URL, command, or package, but not a CLI started from a script file or an alias, or a server behind a generic proxy (#489, #525).
- Each PostHog or Sentry read the host guard recognises is logged and citable like aayu's own queries, with tokens typed inline redacted (#494, #517).
- Host recordings, replays, and feedback reads need `allow_pii`, which counts only when committed on the dev branch (#523, #531).
- The host guard covers PostHog and Sentry calls under GitHub Copilot, and a Copilot shell call that exits non-zero is logged as failed (#537, #556).
- `aayu doctor` tries each PostHog and Sentry capability with one cheap read and names the exact fix (#499).
- A checkout's config can no longer send a stored integration key to another host (#602).
- inspect reads its own state only from a comment its trusted writer posted, in every reader (#605).
- `aayu init --identity` stops at once in a remote shell, such as an orb's Terminal tab or SSH, and creates no App; on this machine it exits 4 with a next step when no redirect arrives within 10 minutes (#603).
- `aayu init --identity` prints no Install link and writes no secret when the App's conversion fails, and refuses an App with another owner, an unrequested permission or event, or another webhook (#603).
- `aayu seat` creates the codeowner-review ruleset when a second person joins a repository that init adopted (#516).
- `aayu inspect <pr> --approve --reason "<what you checked>"` records the `[autonomy] approver`'s review of the pull request's current head on the ledger, after GitHub's device flow every time, and posts it as the human review the merge gate reads (#647).

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
- The winget package identifier is `Rahulnakmol.AgenticAyudha` (#342).
- Every skill has its own page in the guide, generated from the verb registry (#455).
- New guides cover the delivery loop, working as a solo founder, growing from an MVP, and connecting PostHog and Sentry read-only (#463, #473, #474, #507).

### Breaking changes

- `[inspect] reviewer = "auto"` and `"amp"` are removed with no deprecation release: a repository left on either gets a person's review on every pull request that is not docs-only, once it is ready for review and its checks and proof pass, and `aayu install --apply` asks for a tool again (#641, #657).
- A repository that names no review tool now uses the CI review: its merge gate fails, naming the missing setting, until a person runs `aayu init --gates` and adds `AAYU_REVIEW_MODEL_KEY`, or names a review app in the pull request that commits the new merge gate (#657).
- In inspect's human fallback, a reply no longer counts on its own, even from the approver's account: only the `[autonomy] approver`'s `aayu inspect <pr> --approve --reason "<what you checked>"` for that exact head counts, so a new push needs a new approval and a repository with no `[autonomy] approver` cannot clear the fallback (#647).

### Fixes

- Every model gateway call gets the model call limit, `CallTimeout`, even when the caller passes the CLI's general 5-minute client, so a CI review that streams past 5 minutes is no longer cut off and left to a review app (#688).
- The merge gate's inspect job now has 30 minutes instead of 15, and one review call, with its retry, has 25 minutes instead of 12, so a long CI review stream is no longer cut off; run `aayu init --gates` to update your workflow (#699).
- `aayu ship` reads the autonomy streak only when no human pass is settled; after an approving review, a merge on proof, or a solo-mode bypass it records the streak as skipped, with the reason, and no longer spends about two minutes reading GitHub history (#703).
- `aayu install --loops --host amp --apply` now records an Amp event loop's subscription: the setup thread updates and enables an existing equivalent subscription with the current instructions, and install waits up to five minutes for the thread's final reply and takes the subscription ID only from it (#710).
- When the CI review fails on a head, the inspect comment and the job log now say why, with the gateway's address shown as `<endpoint>` (#689).
- A CI review that fails for a passing reason (a 429 or 5xx reply, an overload or server error in the stream, or a stream that ends early or is reset) is retried once, within the same call time limit and the per-PR budget; the failure note hides IP addresses and names the model that failed (#697).
- CodeRabbit's summary block in a PR body no longer changes inspect's cache key, so its edit does not make the gate pay for a second CI review of the same head (#696).
- With `reviewer = "ci"`, an `aayu inspect` or `aayu build` run outside CI no longer asks a review app or a person under your account; inspect reports the merge gate's verdict for the head, or `no-reply` until the gate has one; once the gate records that the CI review failed on that head, a local run asks the fallback app, then a person (#691).
- The chart and measure skills check each number against its own source, name its population and window, and keep a startup note to one page (#312, #315, #341, #348, #356, #369, #373, #377, #385, #386, #402, #411, #420, #423, #425, #429, #437, #456, #464, #472, #528, #534, #539, #559, #573, #581, #591, #600).
- The investigate, slo, runbook, and respond skills keep causes, rule-outs, and figures to the record and cite exact signatures (#375, #380, #387, #389, #412, #424, #428, #438, #452, #461, #527, #543, #563).
- respond names the trigger the evidence shows as a contributing condition, and phrases open questions without presuming their answer (#625).
- decide packs and retro answers say only what their sources state and stay within their length (#281, #296, #303, #316, #345, #351, #357, #361, #365, #368, #372, #433, #440).
- counsel says only what its source says, names a claimed approval without repeating it, and gives no assurance from floors alone (#558, #569, #612).
- The research, wireframe, mockup, prototype, review, polish, and baseline skills keep one choice as one group, reuse the page's design values, and keep a mockup's saved state working (#311, #338, #347, #354, #383, #391, #410, #426, #439, #470, #506, #508, #597, #626).
- The build, verify, and ready skills write the evidence and feature maps without asking, read back the whole Completion Goal, and wait a turn for your yes (#300, #314, #326, #344, #355, #362, #374, #415).
- survey verifies every claim against the code and cites a code path or a run for every fact about behaviour, output, numbers, or defaults (#353, #366, #381, #419, #454, #584, #619).
- tell restates criteria without extrapolating (#291), and slice adds no restrictions the Feature does not state (#308).
- inspect reports only defects it can show from the diff, posts a peer finding once, and counts Codex's security-review findings in its review (#288, #310, #567).
- Skill drafts keep unverified markers out of table cells and leave out claims that something is absent (#421).
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

- The merge gate no longer calls a model or needs an AI key: `aayu inspect` asks an installed agent app of another family for the review, and falls back to your approver when there is none (#225).
- `aayu init --gates` no longer installs the triage workflow and offers to remove it; triage runs from your agent (#243).
- Loops run on schedules only, in the one tool you pick; the event loops added earlier in this cycle (#247) were removed before release (#261).

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

- Proof packs are kept as merge-gate run artifacts and `aayu ship` lands the pack on the ledger; `aayu proof purge` is removed, and a merge gate from an earlier snapshot must be rendered again with `aayu init --gates` (#206).

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
