# Traps by stack: questions to check in the code

A trap is a place where the legacy stack's semantics differ from what a reader, or a model, porting the code would
assume. A code audit reads past them because the code looks ordinary; the port then reproduces the wrong behaviour,
or loses behaviour users rely on. `aayu survey` records where each rule below matches as a `trap:<rule>:<path>:<line>`
fact (in the index, `aayu survey`'s summary, and each module pack). A fact says where to look, never what is true.

For each trap fact in a module you read:

1. Ask the rule's questions of the cited code, and of the callers the pack lists.
2. Write what you found as a trap finding (`traps` in [formats.md](formats.md)): the rule, the question you asked
   (`check`), and the answer with its basis: `observed` when the code shows it, `inferred` with reasoning, or
   `unknown` with a question for the person. A probe beats reading when it is cheap: `aayu survey --golden <cases>
   --bin <legacy> --run --probe` runs each case twice and under a changed time zone, locale, and HOME, records files
   read and written, and with `--check-bin` and `--coverage` adds a sanitiser build's reports and the functions the
   cases never reach. Cite its `probe:` facts.
3. If the modern version must not reproduce it, add it to `do_not_carry` with its evidence, the risk of copying it,
   and the replacement, and ask the person to confirm it. The modernisation steps that replace its code list it in
   `avoids`, and the backlog turns it into a Decision that constrains their Stories.

A rule that matched but turns out harmless is still a finding ("checked: `common /state/` is read-only after
`init`"); an unchecked trap fact is listed in `SURVEY.md` as not checked. The questions under "No rule" have no
pattern the index can match; ask them of every module in that stack.

## Any stack

### generated-code

- Which generator wrote this file, and from what input? Port or replace the generator's input, never the output.
- Is the generator still runnable? If not, the generated file is the source of truth, and says so.

### build-semantic-flags

- Which flag changes the program's meaning, not just its speed? `-ffast-math`, `-Ofast`, `/fp:fast`, and
  `-fp-model fast` reorder floating-point operations and drop NaN and infinity handling; `-march=native`, `-xHOST`,
  and `-ffp-contract=fast` let the compiler fuse multiply-adds, so results depend on the build machine;
  `-no-prec-div` relaxes division; `-fdefault-real-8`,
  `-freal-4-real-8`, and `-r8` make every `REAL` a double; `-fno-automatic` makes every local `SAVE`d (static);
  `-fwrapv` defines signed overflow; `-funsigned-char` changes `char` comparisons; `-m32` changes sizes;
  `-Dfile.encoding`, `-Duser.timezone`, and `-source 1.x` pin a JVM's defaults.
- Do the goldens come from a build with these flags? The new build must reproduce the flag's semantics on purpose
  (a decision), or the comparison must state the tolerance the flag introduces.

### No rule (every stack)

- What does the program read from its environment, configuration files, the working directory, or the clock? Record
  each (a probe: run the goldens with a different `TZ`, locale, and `HOME`).
- Which outputs depend on hash or map iteration order, thread scheduling, or memory addresses?
- Which numeric output depends on the platform's float formatting (`%g`, shortest round-trip, locale decimal comma)?
- Which error messages and exit codes do callers parse?

## Fortran

### fortran-common

- Which units share this COMMON block, and does each declare it with the same layout? A block declared with
  different variable lists in different units aliases memory by position, not name.
- Which units write it and which only read it? A COMMON block is global mutable state; record who initialises it
  (often a BLOCK DATA unit or the first call).
- Do not carry over: replace with module variables owned by one module, or with explicit arguments.

### fortran-equivalence

- Which variables share storage, and is the aliasing used to reinterpret bits (a REAL read as INTEGER) or to save
  memory? Both are undefined to reproduce in another language without explicit transmutation.

### fortran-implicit-typing

- With no IMPLICIT NONE, an undeclared name starting I to N is INTEGER and any other is REAL (single precision). Is
  every variable declared? A misspelt name is a new variable, silently zero or garbage.
- Which values are REAL by accident (a name like `XNORM` meant as double)? That decides the precision of results.

### fortran-implicit-rules

- An `IMPLICIT DOUBLE PRECISION (A-H,O-Z)` changes the default for the whole unit; does the port keep each name's
  type?

### fortran-save-data

- `SAVE` and `DATA` make a local static: its value survives between calls, and a DATA-initialised variable is
  initialised once per program, not per call. Which routines keep state between calls, and is any of it a cache or a
  counter callers depend on?
- With `-fno-automatic` in the build, every local behaves this way; check the build flags.

### fortran-goto

- Is the control flow a loop, an early exit, or an error path? Map each label before porting; computed and assigned
  GOTO need a table.

### fortran-arithmetic-if

- `IF (X) 10, 20, 30` branches on negative, zero, or positive; check NaN (it takes none of the comparisons the port
  might write).

### fortran-entry

- An ENTRY statement gives one body several entry points sharing local variables; the port needs separate
  functions with the shared state made explicit.

### fortran-assumed-size

- An array declared `X(*)` or `X(N)` as a dummy argument has no bounds check: does any caller pass a shorter array,
  or a scalar, or an element `A(I)` to mean "the array from I on"? That last idiom is sequence association.

### fortran-custom-bounds

- Arrays are 1-based by default and may start anywhere (`A(0:N)`, `A(-1:N)`). Map every index expression when
  porting to a 0-based language; an off-by-one here changes results silently.

### fortran-single-precision

- Which computations run in REAL (32-bit) rather than DOUBLE PRECISION? Results printed to many digits differ once
  the port uses 64-bit floats; a constant like `0.1` is single precision unless written `0.1D0`.
- Check the build flags too: `-fdefault-real-8` silently promotes every REAL.

### fortran-machine-constants

- `D1MACH`, `R1MACH`, `I1MACH`, `DPMPAR`, `EPSILON`, `TINY`, and `HUGE` encode the machine's arithmetic. Is the
  value computed, or hard-coded for one machine with the others commented out? Which tolerances derive from it?

### fortran-hollerith

- `8H` constants store characters in numeric variables; the port needs real strings, and any comparison on them is a
  comparison of bit patterns.

### fortran-format

- FORMAT statements decide the exact text of every number (`E15.7`, `F10.4`, `1P`, `G`). The golden comparison of
  printed results depends on reproducing them, or on a stated numeric tolerance.

### fortran-stop

- `STOP` or `PAUSE` ends the whole program from inside a library routine: callers of a modern library cannot
  recover. What exit code and message does it produce?

### fortran-external

- EXTERNAL passes a user's procedure into the library (a callback, as MINPACK passes `FCN`). That is the library's
  extension point: its exact argument list and the meaning of each flag argument are a contract for every caller.

### No rule (Fortran)

- Arrays are column-major: `A(I,J)` and `A(I+1,J)` are adjacent. A port to row-major storage must transpose the
  layout or the loops, and any binary file written with unformatted WRITE keeps the column-major order.
- Which compiler and version built the goldens? Intrinsics, I/O formatting, and optimisation of floating-point
  differ between gfortran, ifort/ifx, and others.
- Unformatted sequential files carry compiler-specific record markers; is any such file part of the data contract?

## Ruby

### ruby-monkey-patch

- Which core class does this reopen (`String`, `Array`, `Hash`, `Integer`, `Object`, `Kernel`), and which methods
  does it add or replace? Every caller in the process sees the change, including other gems.
- Do not carry over: a monkey patch becomes a function, a refinement scoped to its users, or a wrapper type.

### ruby-method-missing

- What names does `method_missing` accept, and what does it do with the rest? The index cannot see these calls:
  list the names the callers use (grep, or a probe that logs them).
- Is `respond_to_missing?` defined to match? Without it, duck-typing checks lie.

### ruby-define-method

- Which methods are created at load time, and from what list? Each is a public interface the index does not list.

### ruby-eval

- What code does `instance_eval`, `class_eval`, or `eval` run, and can its input come from a user or a file? Treat
  string eval of external input as a security finding.

### ruby-send

- Which method names reach `send`, and can a caller choose them? `send` reaches private methods; `public_send` does
  not.

### ruby-dynamic-require

- Which files does a computed `require`, `load`, or `autoload` pull in, and in what order? Load order decides which
  monkey patch wins.

### ruby-globals

- Which global variables carry state between parts of the program, and who writes them?

### ruby-string-mutation

- Does the code mutate strings in place (`<<`, `gsub!`, `force_encoding`)? With `# frozen_string_literal: true` a
  mutated literal raises `FrozenError`, and Ruby 3.4 warns even without it (chilled strings); a shared string mutated
  in one place changes it everywhere.
- Which encoding does each string carry, and what does the output encoding depend on?

### ruby-symbol-string-keys

- Are hash keys symbols or strings, and are both accepted? `:a` and `"a"` are different keys; a port to a language
  with one string type must keep the distinction where callers rely on it.

### ruby-removed-api

- Which APIs are gone in current Ruby: `Fixnum` and `Bignum` (3.2), `File.exists?` and `Dir.exists?` (3.2),
  `URI.escape` (3.0), `$SAFE` (3.0), and taint tracking (3.2)? The legacy code only runs on an old Ruby; record which
  version built the goldens.

### ruby-unsafe-load

- `Marshal.load` and `YAML.load` on untrusted input execute or instantiate arbitrary objects; `Psych` 4 changed
  `YAML.load` to safe loading. Which files does it read, and are they part of the data contract?

### ruby-process-hooks

- `at_exit`, `trap`, `exit`, and `abort` inside library code end or alter the host process. Which exit codes does
  the command line use?

### ruby-rails-conventions

- Which behaviour comes from naming conventions rather than code (autoloaded constants, callbacks, `has_many`
  associations, `before_action` filters)? List them as interfaces; the index sees none of them as calls.

### No rule (Ruby)

- Integer is arbitrary precision and never overflows; Float division by zero gives `Infinity`, Integer raises.
  `7 / 2` is 3. A port to fixed-width integers must check every arithmetic path that can grow.
- `nil` and `false` are the only false values: `0` and `""` are true.
- Which Ruby version and which gem versions (`Gemfile.lock`) produced the goldens?

## Java

### java-reflection

- Which classes or members are reached by name (`Class.forName`, `getMethod`, `invoke`)? Those names are an
  interface: renaming them in the port breaks callers the index cannot see.
- Does the library reflect over its callers' objects (as JSON mappers read getters)? Then the naming conventions it
  reads (`getX`, `isX`) are part of its contract.

### java-proxy-di

- Which behaviour does a framework add (dependency injection, transactions, proxies, aspects, lifecycle callbacks)?
  It is not in the annotated method's body; list it per annotation.

### java-classloader

- Which classes are loaded at run time, from where, and by which loader? Plugin systems built on class loaders are
  extension points.

### java-serialization

- Is Java serialisation used for files, caches, sessions, or messages? The byte format and `serialVersionUID` are a
  data contract, and deserialising untrusted input is a security finding.

### java-jni

- Which native libraries are loaded, and for which platforms? They are dependencies the build graph may not show.

### java-default-charset

- `getBytes()`, `new InputStreamReader(in)`, `FileReader`, and `PrintStream` without a charset use the platform
  default (UTF-8 by default since Java 18, JEP 400; the platform's code page before). Which outputs change with it?

### java-default-locale

- `toLowerCase()`, `toUpperCase()`, `String.format`, and number or date formatting without a `Locale` follow the
  default locale (the Turkish dotless i, the decimal comma). Pin the locale in the goldens and decide which the new
  version uses.

### java-default-timezone

- `new Date()`, `Calendar.getInstance()`, and `LocalDate.now()` follow the default time zone. Which outputs change
  with `TZ`?

### java-threads

- Which state is shared between threads, and what guards it (`synchronized`, `volatile`, executors)? The Java
  memory model's guarantees do not carry over to another language's.

### java-mutable-static

- Which static fields change after start-up? They are global mutable state shared by every caller in the JVM.

### java-swallowed-exception

- Which failures does an empty `catch` hide, and do callers see a default value instead? That default is now
  behaviour users may depend on.

### java-internal-api

- `sun.misc.Unsafe` and `com.sun.*` internals are encapsulated or removed in current JDKs; which JDK built the
  goldens?

### java-exit

- `System.exit`, shutdown hooks, and `finalize` (deprecated for removal) inside a library end or alter the host
  process.

### No rule (Java)

- Checked exceptions are part of each public method's signature: which ones do callers catch, and what does the
  port return instead?
- `int` overflows silently and wraps; `/` truncates toward zero; `char` is UTF-16. `equals` and `hashCode`
  contracts decide map behaviour.
- Which public classes, methods, and constants form the API callers compile against? Binary compatibility (removing
  a method, changing a return type) breaks callers without a source change.

## C and C++

### c-unbounded-copy

- Can the destination overflow (`strcpy`, `sprintf`, `gets`, `scanf("%s")`)? An overflow the legacy build survives
  can still shape golden output; run under AddressSanitizer and decide each finding with the person.

### c-global-state

- Which globals change after start-up, and which functions read them? They are hidden inputs to every caller.

### c-setjmp-signal

- `setjmp`/`longjmp` and signal handlers jump past destructors and cleanup; what state is left behind?

### c-locale

- `setlocale`, `strtod`, `atof`, and `printf` follow the C locale (the decimal comma); which outputs change with it?

### c-type-punning

- Unions and pointer casts reinterpret memory; the result depends on layout and endianness.

### c-layout-endianness

- `#pragma pack`, packed structs, and byte-order calls mean a struct's bytes are a file or wire format: a data
  contract.

## PHP

### php-dynamic-names

- `$$name`, `extract`, and `compact` create variables by name; the index cannot follow them.

### php-eval

- `eval`, `create_function`, `assert` with a string, and `preg_replace` with `/e` execute code; what input reaches
  them?

### php-error-suppression

- `@` hides warnings and errors; the call's failure value (often `false` or `null`) becomes behaviour.

### php-removed-api

- `mysql_*`, `ereg`, `each`, `split`, and `mcrypt_*` are removed in PHP 7 or 8; which PHP built the goldens?

### php-globals

- `global $x` and `$GLOBALS` are shared mutable state.

### php-magic-methods

- `__get`, `__set`, `__call`, and `__callStatic` answer names the index cannot see; list the names callers use.

## Python

### python-dynamic

- `exec`, `eval`, `__import__`, `setattr`, and module `__getattr__` reach code and attributes by computed names.

### python-mutable-default

- A mutable default argument is shared between calls: is it a cache on purpose or a bug users now depend on?

### python-global

- Which functions write module state with `global` or `nonlocal`?

### python-2-only

- Python 2 syntax and APIs (`print` statement, `xrange`, `iteritems`, `except E, e`): which interpreter built the
  goldens? Bytes and text, integer division, and dict ordering change output.

### python-unsafe-load

- `pickle`, `marshal`, `shelve`, and `yaml.load` on untrusted input execute code; which files are a data contract?

### python-star-import

- `from x import *` hides where names come from; resolve each before porting.

## JavaScript and TypeScript

### js-prototype-patch

- Which built-in prototypes does the code extend? Every library in the page or process sees the change.

### js-eval

- `eval`, `new Function`, `with`, and string timers run computed code.

### js-globals

- Which globals does the code set on `window` or `global`?

### js-loose-equality

- `==` coerces types (`"0" == 0`, `null == undefined`); which comparisons rely on coercion?

### js-parseint-radix

- `parseInt` without a radix: older engines read a leading zero as octal.

## COBOL

### cobol-redefines

- REDEFINES gives one storage area several layouts; which layout is valid depends on a type field elsewhere. The port
  needs a tagged union, and the record's bytes are a data contract.

### cobol-binary-decimal

- COMP-3 (packed decimal) and COMP fields are exact decimal or binary integers with fixed digits: arithmetic
  truncates or rounds by the picture, not like a float. Never port them to floating point.

### cobol-signed-picture

- `S9` pictures carry the sign in a zone nibble or a separate character; the file format depends on which.

### cobol-edited-picture

- Edited pictures (`Z`, `$`, `,`, `*`, `-`) decide the exact printed text of every amount.

### cobol-alter

- ALTER changes a GO TO's target at run time; the control flow is not in the source text.

### cobol-goto

- Which paragraphs are reached by GO TO and PERFORM THRU ranges? A THRU range runs every paragraph between two
  names, so inserting a paragraph changes behaviour.

### cobol-occurs-depending

- OCCURS DEPENDING ON makes a variable-length record whose length is in another field.

### cobol-copy-replacing

- COPY REPLACING makes one copybook produce different names per program; resolve each use before porting.

### cobol-collating

- EBCDIC and custom collating sequences change sort order and comparisons; ASCII ports sort differently.

### cobol-dynamic-call

- A CALL through a data name chooses the program at run time; list the values it can take.

### cobol-indexed-file

- Indexed and relative files (VSAM, ISAM) are data stores with keys and record layouts: a data contract and a
  migration.

## Visual Basic (VB6, VBA, VBScript, VB.NET)

### vb-on-error

- `On Error Resume Next` continues after any failure with default values; which errors does it hide, and which
  results do users see then?

### vb-no-option-explicit

- Without `Option Explicit`, a misspelt name is a new empty Variant.

### vb-option-base-compare

- `Option Base 1` makes arrays 1-based; `Option Compare Text` makes string comparison case-insensitive (and
  locale-dependent) for the whole module.

### vb-16bit-integer

- In VB6 and VBA, `Integer` is 16-bit (overflow at 32,767) and `Long` is 32-bit; `CInt` rounds half to even (banker's
  rounding).

### vb-variant-currency

- `Variant` coerces silently; `Currency` is a scaled 64-bit integer with four decimals. Port Currency to a decimal
  type, never a float.

### vb-com-late-binding

- `CreateObject` and `GetObject` load COM components by ProgID at run time: each is a dependency the user's machine
  must have registered, and a contract with that component.

### vb-win32-declare

- `Declare` calls into Windows DLLs; each is an OS integration the new platform must replace.

### vb-doevents-sendkeys

- `DoEvents` re-enters the event loop mid-procedure (re-entrancy); `SendKeys` drives other windows.

### vb-registry

- `GetSetting` and `SaveSetting` (and `Reg*` calls) store configuration in the registry: a configuration contract
  to migrate.

### vb-locale-conversion

- `Val` ignores the locale; `CDbl`, `CDate`, `Format`, and `IsNumeric` follow it. Dates and amounts read or printed
  differently on another machine.

### vb-data-access

- DAO, ADO, Jet, and ACE open Access databases (`.mdb`, `.accdb`): the database file, its queries, and its
  relationships are the users' data, to migrate with a tested rollback.

### vb-file-io

- `Open … For Binary/Random` with `Get` and `Put` writes fixed-length records in the VB layout; existing files are a
  data contract.

### vb-fixed-string

- `String * N` is a fixed-length, padded field, usually part of a record layout.

### vb-desktop-integration

- `Printer`, `Clipboard`, `App`, `Screen`, and `Shell` are user-visible desktop behaviour: printing layout, clipboard
  formats, paths, and launched programs. Capture them as user journeys.

### No rule (Visual Basic)

- Which forms (`.frm`) and controls (`.ocx`) does the user see, with which keyboard shortcuts, tab order, and default
  buttons? Which file associations and installer registrations exist?

## Pascal and Delphi

### pascal-absolute

- `absolute` overlays one variable on another's storage, like EQUIVALENCE.

### pascal-with

- `with` blocks resolve names against a record first; a field added later silently changes which name is meant.

### pascal-directives

- `{$R-}`, `{$Q-}`, and `{$I-}` turn off range, overflow, and I/O checks; `{$H-}` makes `string` a ShortString;
  `{$A}` changes record alignment. The goldens depend on them.

### pascal-raw-memory

- `GetMem`, `Move`, `FillChar`, and pointer dereferences manage memory by hand.

### pascal-string-types

- ShortString (255 bytes, length in byte 0), AnsiString (code page), and PChar differ; file records using them are a
  data contract.

### pascal-inline-asm

- `asm` blocks are platform-specific; what do they compute?
