# Backwards compatibility: the contract, the suite, and coexistence

A modern version is a drop-in replacement only for the interfaces someone depends on, and only to the level the
person decides. This reference says how to derive the contract, prove it, run the two versions together, and move
users and their data across. It generalises what projects that shipped such replacements did, from their own
sources:

- uutils coreutils (Rust) aims to be a drop-in replacement for GNU coreutils and treats differences from GNU as bugs.
  It runs GNU's own test suite against its build every day, publishes the pass count per utility and over time, and
  keeps a list of GNU tests that can never pass for structural reasons, each with its reason, out of the percentage
  ([README](https://github.com/uutils/coreutils/blob/main/README.md),
  [coreutils-tracking](https://github.com/uutils/coreutils-tracking)).
- Ubuntu made it the default in 25.10 with a way back (`oxidizr`, then the `coreutils-from-uutils` and
  `coreutils-from-gnu` packages), shipped it in an interim release first "to maximise real-world testing", and kept
  `cp`, `mv`, and `rm` on GNU in 26.04 LTS while their audit findings stayed open
  ([plan](https://discourse.ubuntu.com/t/carefully-but-purposefully-oxidising-ubuntu/56995),
  [update](https://discourse.ubuntu.com/t/an-update-on-rust-coreutils/80773)). A `date -r` difference broke
  automatic updates in 25.10 because neither suite tested it; the test was added afterwards
  ([discussion](https://news.ycombinator.com/item?id=45686919)). A reference suite proves only what it tests.
  Later, a hard dependency on the new package made the way back harder for users who wanted it: keep the rollback
  path working, not only documented.
- fish moved from C++ to Rust piece by piece, with both languages in one binary until the last caller moved, and
  relied on its end-to-end script and interactive tests, which compare output to expectations, rather than unit
  tests ([The Fish of Theseus](https://fishshell.com/blog/rustport)). Its 4.0 release still lists the deliberate
  incompatible changes for users ([release notes](https://github.com/fish-shell/fish-shell/releases/tag/4.0.0)).
- GitHub's Scientist runs the old and the new code path on the same production inputs, returns the old result, and
  publishes mismatches; it advises keeping write-side duplication until well after cut-over, in case of rollback
  ([Scientist](https://github.com/github/scientist)).
- cminpack, a C port of the Fortran MINPACK, documents that it reports different iteration counts and last-digit
  differences from the original and compares its examples with a numeric tolerance
  ([cminpack](https://github.com/devernay/cminpack)): a numeric contract is "within a stated tolerance" by decision,
  not by accident.
- Microsoft supports the Visual Basic 6 runtime on current Windows ("It Just Works" compatibility) but not the
  third-party ActiveX controls most VB6 applications use
  ([support statement](https://learn.microsoft.com/en-us/previous-versions/visualstudio/visual-basic-6/visual-basic-6-support-policy));
  a desktop rewrite inherits that inventory as its contract.

## 1. Derive the contract

List every interface others depend on in `compatibility` ([formats.md](formats.md)), each citing where the code
provides it. Start from the index: entry points, `io:` surfaces (arguments, environment, files, network, stdout),
public API symbols, build targets, and the trap facts, which often name hidden interfaces (a reflection-driven name,
a `method_missing` name, a record layout). For each stack, look for:

- Command-line tools: each flag and the combinations scripts use, exit codes, stdout and stderr text including
  error messages, environment variables, locale and encoding behaviour, and signals.
- Libraries: the public API and its error model, extension points (callbacks, plugins, `EXTERNAL` routines,
  overridable methods), serialised formats, and the ABI if other languages link to it.
- Services: protocols, message formats, configuration files, and on-disk data.
- Desktop applications: forms and their fields, tab order, keyboard shortcuts and default buttons, menus, printing
  layout, clipboard formats, file associations, registry settings, COM or ActiveX dependencies, and the user's data
  (Access databases, record files).

Say who depends on each, as far as known; an interface nobody knows the consumers of is a question.

## 2. The person decides the level

Each interface gets a level only the person decides (`decided_by` is their request or an answered question):

- `exact`: must match. Proved by case sets compared exactly, a suite, or a replayed user journey.
- `differ`: may differ, with the recorded decision (`difference`): what changes, why, and how users are told.
- `drop`: removed, with a deprecation path (`deprecation`): the warning, the release that warns, the release that
  removes, and what users do instead.
- `undecided`: a question asks; nothing in the backlog may claim to keep it yet.

Ask for levels in the interview by risk: the interfaces with the most consumers and the least evidence first. Offer
a default per interface with its reason (usually `exact` for anything scripts or other programs parse).

## 3. Prove it: golden cases, the legacy suite, and journeys

- **Golden cases** (`case_sets`, `aayu survey --golden`, then `--parity`): characterisation of what the legacy
  build does, including its bugs, compared exactly unless the person decided a tolerance. Capture them from one
  pinned reference build (the commit, the toolchain and its version, the flags) and name it in the case set's
  `reference`: MINPACK's own Release flags change the numbers all six of its test drivers print. Commit the
  capture (`.aayudha/survey/golden/<cases file>/`) and the cases file: the merge gate's `parity` floor replays them
  against each modernisation Story's new build.
- **The legacy suite as the reference** (`suites`, `aayu survey --compat`): when the project has its own tests, turn
  them into a cases file, one test per case with its `area`, run it on the legacy build with `--golden`, then on
  the new build with `--compat`. The score is per area: of the tests the legacy build passes, how many the new
  build passes. A test the legacy build passes and the new one fails is a regression (exit 3); one that fails on
  both is not counted; one that can never pass for a structural reason is marked `unfixable` with its reason and
  left out, never silently. Each run appends to `compat-history.jsonl`, so the score is tracked over time.
- **User journeys** for anything user-visible: record each legacy journey as a behaviour check in a feature map
  (`aayu init --verify-skill`, `aayu verify --map`), then replay the same check against the new application;
  cite it as `check:<product>/<feature>.<check>`. Stdout goldens never prove a screen.
- A suite proves only what it tests: list what no case covers as a gap (the `--probe --coverage` facts name
  functions the cases never reach) and add cases before relying on the score.

## 4. Run both before cut-over

- `coexistence` says how: `shadow` (both run on the same inputs, the old result is used, differences are reported;
  `aayu survey --golden <cases> --bin <legacy> --shadow <new> --run` does this for a cases file), `side-by-side`
  (both installed, users or scripts choose), or `switch` (one at a time with a tested way back).
- Cut-over criteria are observable: a number of runs or days with no difference, the suite score, no open
  regression.
- The rollback is tested, and stays possible: nothing new may depend on the new version alone until the rollback
  window closes.

## 5. Migrate data and configuration with a way back

Each `migration` names what moves (a database, a file format, registry settings, configuration), where the legacy
code reads and writes it, the rollback, and the case sets or suites that prove the migration and its rollback: a
round trip (migrate, use, roll back) compared with the legacy build's own result. Keep the legacy reader working
on its copy until cut-over.

## 6. Shims, each with its removal

Behaviour kept only for old users (an old flag spelling, a legacy output mode, a compatibility switch) is a `shim`
on an interface, with how it is turned on and a removal plan: the condition and the release.

## 7. In the backlog

- A Story or Bug that keeps an interface lists it in `contract`. For an `exact` interface, its `parity` includes
  every case set and suite the contract names and its Completion Goal replays every journey: nothing on the
  must-match list closes on other evidence.
- When the backlog ports anything, every `exact` interface is kept by some Story or Bug.
- Migration, coexistence, and shim work are Stories too, each proving itself with the commands above.
