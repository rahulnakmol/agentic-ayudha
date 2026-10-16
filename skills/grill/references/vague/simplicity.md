# Vague: simplicity (`simple`)

Qualifiers: simple, simpler, easy, easier, easy to use, straightforward, effortless, frictionless, hassle-free,
painless, minimal, at a glance

"Simple" is countable: steps, inputs, decisions, and the help a first-time user needs. For a web app that is
screens and fields; for a command-line tool, commands, flags, configuration, and what an error tells you; for an
API, endpoints and required fields; for a library, the calls before the first result. Ask in this order and quote
today's counts from the facts.

## Dimensions

### Steps and inputs (`simple.steps`)

- Ask: How many steps, screens, or fields should the task take at most? Which inputs can be dropped or filled in for
  the user?
- Default: today's count minus every step that only confirms, because each removed step is observable.
- Evidence: recording
- Facts: flow, route
- Products: web
- Topics: step-count, inputs

### First use without help (`simple.learnability`)

- Ask: Must a first-time user finish without help or instructions? What would they need to see to know what to do
  next?
- Default: one clear primary action per screen, named in the criterion by its label.
- Evidence: recording
- Facts: route, component
- Products: web
- Topics: first-use, primary-action

### Commands and flags (`simple.cli-surface`)

- Ask: Which task should be simple, and how many commands and flags should it need? Which flags can get a default or
  be dropped? Today's counts are facts you can quote.
- Default: the task in one command with no required flag, every other flag optional with a default, because each
  required flag is something to look up.
- Evidence: api-trace
- Facts: command, product
- Products: cli
- Topics: command-count, flag-defaults

### First run (`simple.cli-first-run`)

- Ask: What is the first command a new user runs after installing, and must it work with no configuration file or
  setup step?
- Default: one command from install to the first useful output, with no configuration needed.
- Evidence: recording
- Facts: command
- Products: cli
- Topics: first-command, zero-config

### Errors that say what to do (`simple.cli-errors`)

- Ask: When a command fails, what must it print: the cause, the exact command that fixes it, and which exit code?
- Default: every error names its cause and the next command to run, and usage errors exit 2, because a user should
  never need the docs to recover.
- Evidence: api-trace
- Facts: command, test
- Products: cli
- Topics: error-messages, exit-codes

### Help (`simple.cli-help`)

- Ask: What should `--help` show first: one usage line, the commonest commands, an example?
- Default: usage, the three commonest commands, and one example, in at most one screen (24 lines).
- Evidence: api-trace
- Facts: command
- Products: cli
- Topics: help-text, examples

### Endpoints and fields (`simple.api-surface`)

- Ask: How many calls should the task take, and which request fields can be optional or have a default?
- Default: one call for the task with only the fields it cannot infer required.
- Evidence: api-trace
- Facts: route
- Products: api
- Topics: call-count, required-fields

### Calls to a first result (`simple.lib-surface`)

- Ask: How many calls and lines should a first-time user need to get a first result? Which options can have
  defaults?
- Default: one constructor and one call with defaults, shown by a runnable example.
- Evidence: api-trace
- Facts: code, test
- Products: library
- Topics: calls-to-first-result, defaults

### Remembered choices (`simple.memory`)

- Ask: Should the product remember anything for a returning user (last order, address, preferences)?
- Default: nothing new remembered, because storing personal data is its own decision.
- Evidence: screenshot
- Facts: route, command, story
- Products: web, cli
- Topics: remembered-choices

### Words the user knows (`simple.language`)

- Ask: Are there terms users do not understand today? What words should labels use instead?
- Default: keep today's labels unless the human names a confusing one.
- Evidence: screenshot
- Facts: route
- Topics: labels
