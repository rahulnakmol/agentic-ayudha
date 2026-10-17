# survey pilots on five legacy codebases (WP5.9, October 2026)

Decision #287 asked for `aayu survey` to be proved on at least two real open-source legacy codebases of different
stacks, one of them C++, in draft mode, filing only into a scratch repository created and deleted for the test. The
founder's follow-ups added legacy Java, Fortran, and Ruby, the traps a code audit does not reveal, a do-not-carry list,
and backwards compatibility (Appendix B D-5.9h to D-5.9k): those three pilots are in
[Legacy stacks, traps, and compatibility](#legacy-stacks-traps-and-compatibility). This page records what was run,
what the survey found, what it filed, and what the pilots changed in aayu itself. The
files are next to this page in `survey-pilots-2026-10/`, one directory per codebase: the drafts (`survey.json`,
`backlog.json`), their renders (`SURVEY.md`, `backlog.md`), the cases files, and the inputs and driver the cases use
(Parsedown's Markdown inputs are stored as `.txt` here so the repository's Markdown lint skips them).

## The two codebases

| | tiny-js | Parsedown |
|---|---|---|
| What | An embeddable JavaScript interpreter | A Markdown-to-HTML library |
| Stack | C++ (Make and CMake) | PHP (composer) |
| Source | [gfwilliams/tiny-js](https://github.com/gfwilliams/tiny-js) at `8214477`, MIT | [erusev/parsedown](https://github.com/erusev/parsedown) at `4e433a8`, MIT |
| Product docs | A readme; known issues as a comment block at the top of `TinyJS.cpp` | A readme |
| The person's request | "Tell me what it does, who it is for, and how we would rebuild it in Rust with the same behaviour." | "Tell me what it does, who relies on it, and how we would move it to modern PHP (8.2 or later, strict types, typed signatures) without changing a byte of its output." |

Both were surveyed by this thread acting as the host's model, with the binary built from the stacked survey
branches. No person was interviewed: every question stays open, and no backlog item rests on one.

## 1. Index

`aayu survey <checkout>` indexed each in well under a second with no model:

| | tiny-js | Parsedown |
|---|---|---|
| Files | 59: C++ 3,568 lines in 8 files, JavaScript 565 lines in 46 | 141: PHP 2,379 lines in 6 files; 128 test fixtures not indexed (`.md`, `.html`) |
| Build targets | 7 (CMake `tiny-js` library, `tiny-js-cli`, `tiny-js-tests`; Make `run_tests`, `Script`, `all`, `clean`) | composer library `erusev/parsedown` |
| Entry points | 2 `main`, 17 public API methods | 10 public API methods |
| Surfaces | command-line arguments, files, stdout, `rand`, 5 `eval` uses in tests | 1 dynamic dispatch (`$this->$methodName`, 8 sites at `Parsedown.php:209`), files in tests |
| Symbols, calls | 203 symbols, 371 calls (60 ambiguous by name) | 92 symbols, 54 calls |
| Known-issue markers | 11 (the NOTE and TODO block, two inline TODOs) | 0 |
| History | 45 commits, 2009 to 2026, 7 authors | 745 commits, 2013 to 2026, 46 authors |
| Modules (20k-token budget) | 5 | 2 |

## 2. Survey draft

Both drafts pass `aayu survey --draft` (every citation resolves; product claims are never `observed`):

| | tiny-js | Parsedown |
|---|---|---|
| Engineering claims | 17 | 15 |
| Capabilities | 7 | 6 |
| Product claims | 7 (3 unknown) | 7 (2 unknown) |
| Open questions | 5 | 4 |
| Modernisation | Rust, 5 steps, 3 case sets | modern PHP in place, 4 steps, 3 case sets |
| Coverage | modules 4 of 5, entry points 2 of 19, markers 11 of 11 | modules 2 of 2, entry points 10 of 10 |

Findings a reader would not get from the readmes, each cited in the draft:

- **tiny-js's known-issue list is partly stale** (E12). `TinyJS.cpp:111` says postfix `++` returns the current value;
  the version history (`TinyJS.cpp:103`) says 0.31 fixed it, the code (`TinyJS.cpp:1772-1781`) returns the previous
  value, and running `var a=1; var b=a++; print(b);` prints 1. Prefix `++` (`TinyJS.cpp:112`) is still a parse error.
- **tiny-js's command-line runner has no Math library** (E7): `Script.cpp:78-82` registers the general built-ins only;
  the test runner registers both (`run_tests.cpp:215-216`).
- **tiny-js defects**: a 64-byte buffer overflowed by a test path over 55 characters (`run_tests.cpp:228-229`, E14);
  every caught script error leaks its exception object (`TinyJS.cpp:1344`, E15).
- **Parsedown's readme and composer.json disagree** on the PHP floor: "Tested in PHP 7.1+" (`readme.md:16`) against
  `php >=7.2` (`composer.json:16`, E13).
- **Parsedown's markup escaping is not a security setting** (E12): the readme says so (`readme.md:64-74`), and the
  golden outputs show it: `setMarkupEscaped(true)` keeps `href="javascript:alert%281%29"`, while safe mode turns it
  into `javascript%3Aalert%281%29`.
- **Parsedown's extension contract** (E3, E8, R1): block methods are found by name at run time, so protected method
  names are part of the behaviour. Typing them is safe for parameters but breaks every untyped override for return
  types. The draft first had this the other way round; a two-class PHP probe settled it (PHP 8.2: an untyped override
  of `f(array $b)` loads; of `f(array $b): array` is a fatal error), and the skill now tells the model to probe an
  inferred claim when a probe is cheap.
- **Parsedown's CommonMark tests need the network** and read the specification from `master`
  (`test/CommonMarkTestStrict.php:12`, `:40`, E11).

## 3. Interview

Not held: the pilots ran in draft mode with no person. The questions the drafts would ask first, riskiest first, each
with a default and its reason, are in the rendered `SURVEY.md` files. tiny-js: which embedding API hosts call (decides
the C ABI), whether quirks must be kept, untrusted scripts, whether the runner is a product, the `rand` sequence.
Parsedown: which protected methods subclasses override, byte-identical output where it differs from CommonMark,
shared `instance()` state, the PHP floor.

## 4. Golden outputs and parity

Both legacy builds were characterised with `aayu survey --golden ... --run`, then held to parity with `--parity`:

| | tiny-js | Parsedown |
|---|---|---|
| Legacy build | `make` with g++ | PHP 8.2 CLI with mbstring and a 20-line driver (`render.php`: Markdown on stdin, mode flags) |
| Cases | 77: G1, 38 test scripts through `run_tests` (exit code); G2, 37 scripts through `Script` then `print(result)` (stdout, exit; `test019` excluded for `Math.rand`); G3, 2 script errors (the `ERROR` line with its position, exit 1) | 71: G1, the 64 fixtures (`xss*` in safe mode, `strict*` in strict mode); G2, 4 option cases; G3, hostile input in default, escaped, and safe modes |
| Capture | 77 of 77 | 71 of 71; all 64 G1 outputs byte-identical to the shipped `.html` files |
| Parity, unchanged build | 0 differences | 0 differences |
| Parity, a modernisation step | n/a (no Rust port yet) | `declare(strict_types=1)` added: 0 differences |
| Parity, a planted regression | postfix `++` returning the new value: exit 3, `g1-test027` differs | URL neutralisation removed: exit 3, `g1-xss_bad_url` and `g3-hostile-safe` differ, each with the first differing line |

## 5. Backlog and filing

Both backlogs pass `aayu survey --backlog`; the rendered `backlog.md` files are the exact issues that would be filed:

| | tiny-js | Parsedown |
|---|---|---|
| Items | 16: 1 Goal, 2 Initiatives, 4 Features, 6 Stories, 3 Bugs | 11: 1 Goal, 2 Initiatives, 3 Features, 4 Stories, 1 Bug |
| Review digest | `6470412465324337` | `3337b82e2ef5ec8a` |
| Left out until answered | fixing the known gaps (Q2), an execution limit (Q3), the C ABI's scope beyond five calls (Q1) | return types on protected methods (Q1), output changes towards CommonMark (Q2), `instance()` (Q3) |

Filing test (tiny-js) on a private scratch repository created for it and deleted afterwards
(`rahulnakmol/aayu-survey-scratch`): `--create --reviewed 6470412465324337 --limit 10` filed issues #1 to #10, parents
first; the same command filed the remaining six (#11 to #16); a third run, with the local `filed.json` deleted, filed
nothing and edited nothing (it found every item by its hidden marker and the `survey:tiny-js` label). Sub-issue links
and `depends-on` lines used the real numbers (#9 "Port the parser and execution to Rust" depends on #8 and #6), and
`aayu ready 9 --check` passed.

A sample of the generated backlog (tiny-js), as filed:

```text
Goal        #1  tiny-js stays a dependency-free scripting engine for its hosts while it moves to Rust
Initiative  #2  Characterise the C++ interpreter before changing it
Feature     #4    Golden outputs of the C++ build
Story       #5      Capture pass or fail of the 38 test scripts from the C++ build          parity G1
Story       #6      Capture the command-line runner's output and errors                     parity G2, G3
Bug         #14     The known-issue list says postfix ++ is broken, but the code fixed it in 0.31
Bug         #15     run_tests overflows a 64-byte buffer for a long test path
Bug         #16     Every caught script error leaks its exception object
Initiative  #3  Rebuild the interpreter in Rust behind the same interfaces
Feature     #7    Script execution in Rust
Story       #8      Port the lexer and value model to Rust                                  parity G1
Story       #9      Port the parser and execution to Rust                                   parity G1, G2, G3
Feature     #10   Built-in and Math libraries in Rust
Story       #11     Port the built-in and Math functions to Rust                            parity G1, G2
Feature     #12   Embedding API for host applications
Story       #13     Ship the Rust crate and C ABI, then retire the C++ build                parity G1, G2, G3
```

Story #9's Completion Goal, as filed:

```text
1. [api-trace] state: the Rust test runner and case set G1; expected: aayu survey --parity reports no difference
2. [api-trace] state: the Rust command-line runner and case sets G2 and G3; expected: aayu survey --parity reports
   no difference, including the ERROR line's line and column
```

## What the pilots changed in aayu

- The index listed Parsedown's 64 `test/data/*.md` fixtures as product documents; Markdown under a test directory is
  now test data (#305).
- One cases file runs one program, and tiny-js needs two (the test runner and the command-line runner); each cases
  file now has its own capture directory, `golden/<cases file name>/` (#305).
- `SURVEY.md` dropped `block<Type>Continue` (read as an HTML tag) and turned `xss* ... strict*` into emphasis, and had
  two "Risks" headings; text is now escaped outside code spans (#305).
- The skill tells the model to check stated claims against the code and to probe inferred claims when a probe is
  cheap (#313).

## Legacy stacks, traps, and compatibility

Three more pilots ran the founder's follow-ups on real code: the index for each stack, the trap rules, `--probe`
(two identical runs, a changed time zone, locale, and HOME, strace, a checker build, and coverage), `--compat`, and
`--shadow`. No person was interviewed; the Maruku backlog below follows a recorded interview written for the eval
case `survey/maruku-backlog`, not a real one.

| | MINPACK | Maruku | JSON-java |
|---|---|---|---|
| What | Nonlinear equations and least squares | A Markdown-superset library and command | A JSON library |
| Stack | Fortran 77, fixed form (CMake, fpm) | Ruby (gem, Rakefile) | Java (Maven, Gradle) |
| Source | [certik/minpack](https://github.com/certik/minpack) at `12f30bc`, MIT and the Minpack Copyright Notice | [bhollis/maruku](https://github.com/bhollis/maruku) at `4d88d0e`, MIT | [stleary/JSON-java](https://github.com/stleary/JSON-java) at `69383f7`, Public Domain |
| Index | 50 Fortran files, 10,825 lines; 68 symbols, 115 calls; 12 main programs | 51 Ruby files, 8,253 lines; 469 symbols, 339 calls | 85 Java files, 27,928 lines; 1,348 symbols, 3,602 calls |
| Traps the index flagged | COMMON 6, implicit typing 50, SAVE and DATA 44, GO TO 44, Hollerith 7, machine constants 25, build flags 2 | string mutation 18, `send` 8, monkey patch 2, `eval` 5, dynamic require 6, globals 6 | swallowed exceptions 10, default locale 6, serialisation 4, reflection 3, default charset 3 |

What running the legacy build showed, which reading the code could not:

- **MINPACK: the Release flags change the results.** The build sets `-O3 -march=native -ffast-math` for Release in an
  included file, `cmake/UserOverride.cmake`. With that build as the checker, all 6 driver cases printed different
  numbers from the `-O2` capture (`probe:check-differs:hybrd1`: the final residual norm `0.5373479D-12` changes). An
  `-O1 -fcheck=all` build with AddressSanitizer and UndefinedBehaviorSanitizer reported no runtime error, yet 3 of 6
  cases ended with a different exit parameter (`lmder1`: 3 becomes 2). So "the same results" needs a pinned reference
  build and flags, not just the source. Coverage showed `covar` is never reached by the drivers. Two runs, a changed
  time zone, locale, and HOME, and strace found nothing. The whole of `src/` compiles with `-fimplicit-none`, so the
  50 implicit-typing findings are a style to drop, not behaviour to keep. The library also defines the test problems
  `initpt` (3 times) and `hesfcn` (twice), so a linker picks one.
- **MINPACK shadow run.** The maintained modern-Fortran port
  ([fortran-lang/minpack](https://github.com/fortran-lang/minpack) at `c0b5aea`) linked to the legacy drivers through
  a 60-line shim of the F77 names: `--shadow` reported 0 of 6 cases different.
- **Maruku: a frozen-string-literal build fails before rendering.** With `ruby --enable=frozen-string-literal`, 156 of
  the 162 block docs printed nothing, each with `FrozenError` at one of four mutation sites
  (`lib/maruku/input/linesource.rb:83`, `lib/maruku/input/parse_span.rb:452` and `:722`,
  `lib/maruku/output/to_html.rb:71`). Other findings: the Array patch applies to every program that loads Maruku;
  `<?maruku ?>` instructions go to `instance_eval` when unsafe features are on; `nice_date` fails whenever
  `maruku_signature` is on; and `--ext` requires an interpolated path, which the dynamic-require rule now catches.
- **JSON-java: the locale and charset matter.** Under `LC_ALL=C` the JVM wrote every non-ASCII character as `?`; the
  cases now set `LC_ALL=C.UTF-8` and record it. In a shadow run under `ar-SA`, `JSONPointer`'s out-of-bounds message
  printed `٣ elements` in Arabic-Indic digits (`String.format` with the default locale, `JSONPointer.java:231`). Under
  `tr-TR` with `file.encoding=ISO-8859-1`, 3 of 5 cases differed. A `BigDecimal` of `0.10` round-trips as `0.1`.

**Compatibility suite (Maruku).** The block docs became a compatibility suite of 162 tests in 17 areas, run with
`--golden` on Maruku and then `--compat` on kramdown, the library Maruku's readme recommends:

| Area | Tests | Maruku passes | kramdown passes |
|---|---|---|---|
| core | 27 | 27 | 15 |
| lists | 26 | 26 | 18 |
| regressions | 25 | 23 | 16 |
| html | 15 | 12 | 6 |
| math (12) and blahtex (4) | 16 | 3 | 0 |
| code | 9 | 7 | 1 |
| links and images | 9 | 9 | 6 |
| footnotes and abbreviations | 7 | 6 | 1 |
| tables | 7 | 7 | 0 |
| headers | 6 | 6 | 2 |
| attributes | 5 | 4 | 1 |
| divs | 3 | 3 | 0 |
| encoding | 2 | 2 | 0 |
| paragraph rules, references, recover | 5 | 4 | 3 |
| **Total** | **162** | **139** (17 fail, 6 skip) | **69 of the 139** |

Maruku fails 17 of its own docs, so those are not regressions. kramdown is not a drop-in replacement: it renders
none of the tables or divs as Maruku did. That is why the contract asks which syntax the pages use (Q2) before
deciding any level, and why the backlog adds Maruku's syntax as extensions (step M3) before any page switches.

**Contract and do-not-carry (Maruku draft).** The draft names four interfaces: the HTML `to_html` renders (K1), the
Markdown dialect (K2), the options and `MaRuKu::Globals` (K3), and the `maruku` command (K4). All four are left
undecided, each with the question that decides it. Three do-not-carry entries are proposed, each with its evidence,
risk, and replacement: the Array patch (N1), executing embedded Ruby (N2), and process-wide settings (N3). After
the recorded interview, the backlog (eval case `survey/maruku-backlog`, passing `--backlog`) has:

- K1 to K3 at "may differ", each with its recorded difference;
- K4 dropped, with a deprecation path;
- N1 to N3 as Decisions that constrain the Stories porting their code.

```text
Goal        Existing pages keep reading the same while the sites move off obsolete Maruku
Initiative    Record how Maruku renders today before anything changes
Feature         Maruku's results on its block docs
Story             Capture Maruku's pass, fail, or skip on every block doc                       parity G1
Initiative    Render with a maintained library behind Maruku's entry point
Feature         Maruku's entry point over the maintained library
Story             Render through the adapter with per-document options and no global state   G1; keeps K1, K3; avoids N1, N3
Story             Score the adapter on Maruku's block-doc suite per area                      S1; keeps K1
Decision          Do not carry over Maruku's patch to Array                                   constrains the adapter Story
Decision          Do not carry over process-wide settings                                     constrains the adapter Story
Feature         Tables, footnotes, and header attribute lists on the new library
Story             Render tables, footnotes, and header attribute lists as Maruku did          G1, S1; keeps K1, K2; avoids N2
Decision          Do not carry over executing Ruby embedded in documents                      constrains that Story
Feature         Switch the sites to the new library and retire Maruku
Story             Render every page with both renderers and report the differences            G1, S1; keeps K1
Story             Switch the renderer to the adapter, with a tested switch back               G1; keeps K1
Story             Call the new library directly and remove the Maruku adapter class           G1
```

What these pilots changed in aayu: the probe's paths, its environment, the build-flag and Fortran rules, and coverage
(#353); checker errors in `check-differs`, library-path reads, and the interpolated require (#366); and the eval
cases `survey/minpack-traps`, `survey/maruku-compat`, and `survey/maruku-backlog` (#370). The drafts, cases, and
probes are in `survey-pilots-2026-10/minpack/`, `maruku/`, and `json-java/`.
