# Survey: tiny-js

tiny-js is a small embeddable JavaScript interpreter in C++: a host application creates a CTinyJS, registers native C++ functions, and executes or evaluates script text directly from source. It ships a command-line runner that executes script files or a line-by-line interactive mode, and a test runner over 38 numbered scripts. Who embeds it today is not in the code.

Asked for: "This is a legacy C++ JavaScript interpreter with no product docs. Tell me what it does, who it is for, and how we would rebuild it in Rust with the same behaviour.". Indexed at commit `821447737f3d`: 59 files, cpp (3568 lines), javascript (565 lines).

Each claim says how it is known: **observed** (the code shows it), **stated** (a document or comment says it), **inferred** (reasoned from the cited code), **confirmed** (the person said it), or **unknown** (a question asks it). Citations are index fact IDs or `path:line` ranges at the commit above.

## Limits of this survey

- Calls are resolved by name only: 60 of 371 call edges name more than one definition.
- Dynamic dispatch the index cannot follow: `tests/42tests/test004.js:27`, `tests/test021.42.js:7`, `tests/test021.js:3`, `tests/test022.42.js:11`, `tests/test022.js:7`.
- Files with no language the index knows: .gitignore (1).
- Product claims come from documents, inference, or the person; code alone never shows who uses the software or why.

## Engineering

### Architecture

- **E1** (observed) The interpreter is one class, CTinyJS, whose public API is execute, evaluateComplex, evaluate, addNative, getScriptVariable, getVariable, setVariable, and trace, with the symbol table exposed as root. `entry:TinyJS.h:288`, `TinyJS.h:288-330`.
- **E2** (observed) Script text is executed directly by a recursive-descent parser with one method per precedence level (functionCall, factor, unary, term, expression, shift, condition, logic, ternary, base, block, statement); there is no intermediate code. `TinyJS.h:342-354`, `sym:TinyJS.cpp#CTinyJS::statement`, `doc:README.md`.

### Data

- **E3** (observed) Values are CScriptVar nodes whose children are a linked list of CScriptVarLink, so object member and array index lookup walk the list. `TinyJS.h:179-183`, `TinyJS.h:258`, `sym:TinyJS.cpp#CScriptVar::findChild`, `sym:TinyJS.cpp#CScriptVar::getArrayIndex`, `marker:TinyJS.cpp:113`.
- **E4** (stated) Memory is managed by reference counting (ref and unref), so a cycle such as a.foo = a is never freed. `sym:TinyJS.cpp#CScriptVar::unref`, `marker:TinyJS.cpp:109`.

### Interfaces

- **E5** (observed) The command-line runner executes each file argument in one interpreter in order and exits 0; with no arguments it prints a banner and executes stdin line by line (lines up to 2047 bytes) until quit() is called. `entry:Script.cpp:74`, `Script.cpp:86-106`, `sym:Script.cpp#execute_script`.
- **E6** (observed) A script error in file mode prints 'ERROR: ' followed by the message and the position, and the runner exits 1; in interactive mode it prints the error and continues. `Script.cpp:93-95`, `Script.cpp:101-105`, `TinyJS.cpp:1333-1344`.
- **E7** (observed) The command-line runner registers the general built-ins and console.log, print, and dump, but not the Math functions; the test runner registers both sets, so Math.abs and the rest exist in tests but not in the command-line runner. `Script.cpp:78-82`, `run_tests.cpp:215-216`, `sym:TinyJS_MathFunctions.cpp#registerMathFunctions`.
- **E8** (observed) Script can run arbitrary script text at run time through the built-ins exec and eval, which call execute and evaluateComplex on the same interpreter. `sym:TinyJS_Functions.cpp#scExec`, `sym:TinyJS_Functions.cpp#scEval`, `TinyJS_Functions.cpp:232-233`, `io:dynamic-dispatch:tests/test022.js:7`.

### Build

- **E9** (observed) Make builds run\_tests and Script, and CMake builds a static library tiny-js plus tiny-js-cli and tiny-js-tests, all from TinyJS.cpp, TinyJS\_Functions.cpp, and TinyJS\_MathFunctions.cpp; CMake requires C++14 outside Windows. `build:Makefile#run_tests`, `build:Makefile#Script`, `build:CMakeLists.txt#tiny-js`, `build:CMakeLists.txt#tiny-js-cli`, `build:CMakeLists.txt#tiny-js-tests`, `CMakeLists.txt:15-23`.

### Dependencies

- **E10** (observed) The interpreter uses only the C and C++ standard libraries; glibc's mcheck, execinfo, and malloc hooks appear only in the test runner behind MTRACE and INSANE\_MEMORY\_DEBUG. `TinyJS.cpp:122-136`, `TinyJS.h:38-43`, `run_tests.cpp:42-53`, `doc:README.md`.

### Tests

- **E11** (observed) run\_tests runs tests/test001.js upwards until the first missing number, and a test passes when the script leaves the global result truthy; on failure it writes the symbol table as JSON to \<file>.fail.js. The \*.42.js files and tests/42tests are not run by it. `sym:run_tests.cpp#run_test`, `run_tests.cpp:217-239`, `run_tests.cpp:265-277`, `test:run_tests.cpp`.

### Risks

- **E12** (observed) The comment's known-issue list is partly stale: it says the postfix increment returns the current value, but the version history says 0.31 fixed it and the code returns the previous value. `marker:TinyJS.cpp:111`, `TinyJS.cpp:103`, `TinyJS.cpp:1772-1781`.
- **E13** (stated) Known language gaps: Array(5) does not create an array of that length, length cannot be set, and there is no prefix increment operator. `marker:TinyJS.cpp:108`, `marker:TinyJS.cpp:110`, `marker:TinyJS.cpp:112`.
- **E14** (observed) run\_test formats '\<filename>.fail.js' into a 64-byte stack buffer with sprintf, so a test path longer than 55 characters overflows it. `run_tests.cpp:228-229`.
- **E15** (observed) Errors are thrown as heap pointers (throw new CScriptException) and execute rethrows a new one, so every caught script error leaks its exception object unless the host deletes it; Script.cpp never does. `TinyJS.cpp:1344`, `Script.cpp:93-95`, `Script.cpp:103-105`.

### Quality

- **E16** (inferred) Math.rand and Math.randInt call rand() and nothing seeds it, so a given build repeats the same sequence on every run; the sequence depends on the C library. `io:random:TinyJS_Functions.cpp:53`, `TinyJS_Functions.cpp:56-61`. Reasoning: No srand call appears in the indexed sources (the index records rand as the only random surface), and the C standard defines an unseeded rand as seeded with 1.
- **E17** (stated) Open TODOs in the source: a va-args helper to call a script function from C++, eval of statements as well as expressions, moving shift into mathsOp, a call whose purpose the author questions, and a better way to parse the left-hand side of a for-in. `marker:TinyJS.cpp:116`, `marker:TinyJS.cpp:117`, `marker:TinyJS.cpp:118`, `marker:TinyJS.cpp:1545`, `marker:TinyJS.cpp:1963`.

## Capability map

| ID | Capability | What it does | Basis | Evidence |
|---|---|---|---|---|
| C1 | Run JavaScript-like scripts | Executes script text with variables, functions, objects with prototype inheritance, arrays, the ternary, shift, and modulo operators, from source. | stated | `doc:README.md`, `README.md:19-25`, `TinyJS.cpp:69-98`, `sym:TinyJS.cpp#CTinyJS::execute` |
| C2 | Evaluate an expression for the host | Returns the value of script text to C++ as a string (evaluate) or a value link (evaluateComplex). | observed | `sym:TinyJS.cpp#CTinyJS::evaluate`, `sym:TinyJS.cpp#CTinyJS::evaluateComplex`, `TinyJS.h:294-302` |
| C3 | Call C++ from script | A host registers a native C++ function under a script name and signature, including methods on built-in classes such as String.substring. | observed | `sym:TinyJS.cpp#CTinyJS::addNative`, `TinyJS.h:304-318` |
| C4 | Read and set script variables from C++ | The host reads or sets a variable by dotted path. | observed | `TinyJS.h:320-325` |
| C5 | JSON in and out | Serialises any value to JSON (JSON.stringify, getJSON) and reads JSON back by evaluating it as script. | observed | `sym:TinyJS.cpp#CScriptVar::getJSON`, `TinyJS_Functions.cpp:149-153`, `io:dynamic-dispatch:tests/test022.js:7` |
| C6 | Built-in library | Provides string, array, integer, object, and JSON built-ins, exec and eval, and a Math library of 29 functions when registered. | observed | `TinyJS_Functions.cpp:232-253`, `TinyJS_MathFunctions.cpp:262-293` |
| C7 | Command-line and interactive runner | Runs script files from the command line or reads statements interactively, printing with print and console.log and dumping the symbol table with dump. | observed | `entry:Script.cpp:74`, `Script.cpp:45-52`, `Script.cpp:80-82` |

## Product

- **P1** (stated) The README addresses developers of applications that need a simple, familiar scripting language with no dependencies beyond the normal C++ libraries. `doc:README.md`, `README.md:6-8`.
- **P2** (stated) The README names two jobs: scripting simple behaviour, and loading and saving settings. `README.md:11-12`.
- **P3** (stated) It is not meant to be fast, full-featured, or standards-compliant, and slow loops are accepted. `README.md:11-17`, `README.md:38-40`.
- **P4** (unknown, Q1) Which applications embed tiny-js today, and which of its APIs they call, is not in the repository.
- **P5** (unknown, Q2) Whether the Rust version must keep today's quirks (the non-standard behaviour in E12 and E13) or may correct them is not in the code.
- **P6** (unknown, Q3) Whether hosts load scripts they do not control, which decides how much exec and eval (E8) must be sandboxed, is not in the code.
- **P7** (stated) The README points to a more complete fork (42tiny-js) and a microcontroller rewrite (Espruino), so the product sits among alternatives. `README.md:31`, `README.md:45-51`.

## Questions for the person

| ID | Question | Why it matters | Default | Status |
|---|---|---|---|---|
| Q1 | Which applications embed tiny-js today, and do they call it through the C++ classes (CTinyJS, CScriptVar) or only through execute, evaluate, and addNative? | It decides whether the Rust version needs a C ABI for existing hosts, and which API the parity cases must cover. | Keep a C ABI for execute, evaluate, addNative, getVariable, and setVariable only, because the README sells calling C/C++ from script (P1) and those are the documented entry points. | open |
| Q2 | Must the Rust version reproduce today's behaviour exactly, including the known gaps (Array(5), length not settable, no prefix ++), or may it correct them? | It decides whether the golden outputs are the contract or a starting point, and which case sets exclude behaviour. | Reproduce exactly in the first release so existing scripts keep working; correct each gap later as its own Story with a new golden case. | open |
| Q3 | Do any hosts run scripts from users or files they do not control? | If they do, exec and eval and unbounded loops need limits (time, memory) in the Rust version, which changes its design. | Assume scripts are trusted (settings and simple behaviour, P2) and record a limit on execution time as a later Story. | open |
| Q4 | Is the command-line runner part of the product, or a demo of embedding? | If it is a product, its output format and its missing Math functions (E7) become part of the contract. | A demo: keep it for parity tests, register the same built-ins as the test runner only if the answer says so. | open |
| Q5 | Is reproducing the same Math.rand sequence required? | An unseeded C rand() sequence (E16) cannot be reproduced portably in Rust. | No: exclude random output from parity and test only the range. | open |

## Modernisation path: Rust

Target from the request. Characterise first, then port behind the same interfaces: capture golden outputs from the C++ build for the 38 test scripts and the command-line runner, port the lexer and values, then the parser and built-ins, and keep a C ABI shim so existing hosts can switch without code changes (pending Q1). Retire the C++ build only when every case set matches. (`test:run_tests.cpp`, `entry:Script.cpp:74`, `entry:TinyJS.h:288`)

### Steps

1. **M1** Capture golden outputs from the C++ build. Replaces `mod:tests`, `mod:(root)#1`, `mod:(root)#3`. Rests on E5, E6, E11. Parity: G1, G2, G3.
2. **M2** Port the lexer and the value model (reference counting replaced by Rc and a cycle collector or arena). Replaces `mod:(root)#2`. After M1. Rests on E3, E4. Parity: G1.
3. **M3** Port the parser and execution. Replaces `mod:(root)#2`. After M2. Rests on E1, E2, E12, E13. Parity: G1, G2, G3.
4. **M4** Port the built-in and Math libraries. Replaces `mod:(root)#3`. After M3. Rests on C6, E7, E8. Parity: G1, G2.
5. **M5** Expose the embedding API as a Rust crate and a C ABI, then retire the C++ build. Replaces `mod:(root)#1`. After M4. Rests on E1, C2, C3, C4, E9. Parity: G1, G2, G3.

### Parity case sets

- **G1** The 38 numbered test scripts, pass or fail per script. Inputs from `test:run_tests.cpp`, `run_tests.cpp:257-259`; compared exit code only (run\_tests file.js exits 0 on pass); its stdout prints the runner's banner, which a Rust runner need not copy.
- **G2** Command-line runner output on each test script followed by print(result). Inputs from `entry:Script.cpp:74`, `Script.cpp:86-90`; compared exact stdout and exit code. Not kept: Math.rand and Math.randInt sequences (E16; Q5 asks whether they matter).
- **G3** Script errors from the command-line runner. Inputs from `Script.cpp:93-95`, `TinyJS.cpp:1333-1344`; compared exact stdout (the ERROR line with its position) and exit code.

### Modernisation risks

- **R1** (observed) Error messages include the parser's line and column, so a Rust parser must report positions exactly as the C++ lexer does for G3 to pass. `TinyJS.cpp:1340`, `TinyJS.cpp:646`.
- **R2** (observed) Doubles are converted to text with C's %f format (six decimals: 50 prints as 50.000000), which Rust's default formatting does not match. `TinyJS.cpp:961-967`.

## Coverage

- Modules cited: 4 of 5. Not cited: `mod:tests/42tests`.
- Entry points cited: 2 of 19.
- Known-issue markers cited: 11 of 11.
- Open questions: 5; unknown claims: 3.
