# Modernisation notes by stack

The path is the same on every stack: characterise, port or modernise in small steps behind the same interfaces, and
prove each step against the golden outputs. These notes name what usually differs. Treat each as a question to check
in the code, never as a claim about it; [traps.md](traps.md) has the questions per trap rule.

## Any stack

- Capture first. A step that changes code before the goldens exist has nothing to prove parity against.
- Keep the observable interface: command-line arguments and exit codes, stdout and stderr, file formats, network
  messages, and the public API a library's callers use. Decide with the person which of these are the contract.
- Known quirks: reproduce them in the first release unless the person decides otherwise; fix each later as its own
  Story that changes one golden case on purpose.
- Exclude non-determinism from comparison with a reason: time, randomness, memory addresses, hash ordering, thread
  scheduling.
- Text formatting differs between languages: floating-point printing, locale, line endings, and Unicode case rules.

## C or C++ to Rust

- Memory: manual `new`/`delete`, reference counting, and exceptions thrown as pointers become ownership, `Rc`/`Arc`
  with a plan for cycles (weak references, an arena, or a collector), and `Result`.
- Undefined behaviour the C++ build happens to tolerate (buffer overruns, uninitialised reads, signed overflow) can
  show up as golden output; run the legacy build under AddressSanitizer or Valgrind and decide each finding with the
  person before it becomes the contract.
- `printf`-style formatting (`%f`, `%g`, `%d` width and padding) needs explicit formatting in Rust.
- A `rand()` sequence depends on the C library's algorithm and seed: reproducing it means reimplementing that
  library's generator or calling it, so ask whether any output depends on the exact sequence before choosing.
- Hosts that embed a C++ library need a C ABI (`extern "C"`, `cbindgen`) for the calls they make, or a migration
  path; ask which calls they make.
- Build: map each Make or CMake target to a Cargo package or binary target; keep the legacy build until parity holds
  everywhere.

## PHP (modernise in place)

- `declare(strict_types=1)` changes how scalar arguments are coerced for calls made from that file; run parity after
  adding it.
- Parameter types may be added to methods subclasses override (an untyped override is wider), but adding a return
  type breaks every override without one: check for subclasses before typing protected methods.
- `htmlspecialchars` flags and the encoding argument decide escaping; keep them identical.
- `mbstring` and `iconv` behaviour depends on the extension version.

## Fortran (to modern Fortran, C++, Rust, Python, or Julia)

- Decide the numeric contract first: exact text of printed results (FORMAT statements), or results within a stated
  tolerance. A port that changes the order of floating-point operations, or a compiler that fuses multiply-add,
  changes the last digits; cminpack, a C port of MINPACK, documents that it reports different iteration counts and
  last-digit differences against the original and compares with a tolerance instead.
- Precision: check `REAL` against `DOUBLE PRECISION` and the build's `-fdefault-real-8` or `-r8`; the goldens are only
  meaningful with the same promotion.
- Layout: arrays are column-major and usually 1-based; a port to C, Rust, or NumPy chooses a layout once and maps
  every index. Keep the Fortran layout behind any C ABI callers use.
- State: COMMON blocks, `SAVE`, and `DATA` become module or struct state owned in one place; EQUIVALENCE becomes an
  explicit conversion.
- Interfaces: an `EXTERNAL` user routine (a callback) and its flag arguments are the library's contract with every
  caller; keep their meaning, and offer a C ABI (`bind(C)`) when other languages call it.
- An intermediate step that often pays: free-form modern Fortran with `implicit none` and modules first, proved
  against the goldens, then the target language.

## Ruby (to a newer Ruby, or another language)

- Pin the Ruby and gem versions that produced the goldens (`Gemfile.lock`, `.ruby-version`); removed APIs
  (`Fixnum`, `File.exists?`, `URI.escape`) mean an old interpreter may be the only one that runs the legacy code.
- List the behaviour the index cannot see before porting: monkey patches of core classes, `method_missing` names,
  methods made with `define_method`, autoloaded constants, and framework callbacks.
- Strings: encodings and frozen literals; decide what the new version accepts and emits.
- Numbers: Integer never overflows; a port to fixed-width integers needs a decision per arithmetic path.
- Upgrading in place: run the goldens on each Ruby version step, and turn warnings (`ruby -W:deprecated`) into
  findings.

## Visual Basic 6 or VBA desktop applications (to .NET, the web, or Electron)

- The contract is mostly user-visible: forms, keyboard shortcuts and tab order, printing, clipboard formats, file
  associations, registry settings, and the user's data files and Access databases. Capture user journeys from the
  legacy application and replay them against the new one; stdout goldens do not cover a desktop application.
- Inventory every COM component, ActiveX control (`.ocx`), and `Declare` into a Windows DLL; Microsoft supports the
  VB6 runtime on current Windows but not third-party ActiveX controls.
- Data: Access (`.mdb`) and fixed-record files are migrated with a tested rollback, and the old application keeps
  working on its copy until cut-over.
- Arithmetic and text: 16-bit `Integer`, `Currency`, banker's rounding in `CInt`, and locale-dependent `CDate` and
  `Format`.

## COBOL (to Java, C#, or modern COBOL)

- Packed decimal and edited pictures are the numeric contract: use an exact decimal type and reproduce the
  rounding and truncation of each picture.
- Record layouts (REDEFINES, OCCURS DEPENDING ON, copybooks) are file and message formats shared with other
  programs; keep them byte-identical until every reader has moved.
- EBCDIC collating order changes sort results in an ASCII port.

## Java or C# to a newer runtime

- Reflection, serialisation formats, and default charset or culture are the usual sources of difference; JDK 18 made
  UTF-8 the default charset (JEP 400), so a move across it changes every call that relied on the platform default.
- Date and number formatting depends on the default locale; pin it in the golden environment.

## Python 2 to 3, or between Python versions

- Bytes and text, integer division, dictionary ordering, and `repr` of floats change output.

## JavaScript or TypeScript

- Node version changes `Array.prototype.sort` stability guarantees, `Intl` data, and error messages; pin it.
