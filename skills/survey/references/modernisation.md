# Modernisation notes by stack

The path is the same on every stack: characterise, port or modernise in small steps behind the same interfaces, and
prove each step against the golden outputs. These notes name what usually differs. Treat each as a question to check
in the code, never as a claim about it.

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
- Unseeded `rand()` sequences depend on the C library; they cannot be reproduced portably.
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

## Java or C# to a newer runtime

- Reflection, serialisation formats, and default charset or culture are the usual sources of difference.
- Date and number formatting depends on the default locale; pin it in the golden environment.

## Python 2 to 3, or between Python versions

- Bytes and text, integer division, dictionary ordering, and `repr` of floats change output.

## JavaScript or TypeScript

- Node version changes `Array.prototype.sort` stability guarantees, `Intl` data, and error messages; pin it.
