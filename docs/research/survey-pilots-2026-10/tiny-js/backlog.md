# Backlog draft: tiny-js

Nothing here is filed yet. Review every item below: it is the exact issue that will be filed (1 goal, 2 initiative, 4 feature, 6 story, 3 bug).

Review digest `6470412465324337`. To file it after review:

```sh
aayu survey --backlog backlog.json --draft survey.json --create --reviewed 6470412465324337
```

## Open questions

- Q1 (open): which hosts embed tiny-js and through which API; decides the C ABI's scope in rust-api-and-abi
- Q2 (open): keep or correct the known gaps (Array(5), length, prefix ++); no item fixes them until answered
- Q3 (open): untrusted scripts; an execution-limit Story waits for the answer
- Q4 (open): is the command-line runner part of the product
- Q5 (open): must Math.rand's sequence match

## 1. Goal: tiny-js stays a dependency-free scripting engine for its hosts while it moves to Rust

key `goal`.

````markdown
<!-- aayudha:survey product=tiny-js key=goal -->

> From the survey of tiny-js: rests on P1, P2.

### Summary

Applications that embed tiny-js for simple scripting and settings (P1, P2) keep the same behaviour while the engine is rebuilt in Rust.
````

## 2. Initiative: Characterise the C++ interpreter before changing it

key `characterise`, under `goal`.

````markdown
<!-- aayudha:survey product=tiny-js key=characterise -->

> From the survey of tiny-js: rests on M1, E5, E11.

### Summary

Capture what the C++ build does today, as golden outputs, so every later change is measured against it.
````

## 3. Initiative: Rebuild the interpreter in Rust behind the same interfaces

key `rust-engine`, under `goal`.

````markdown
<!-- aayudha:survey product=tiny-js key=rust-engine -->

> From the survey of tiny-js: rests on M2, M3, M4, M5, E1, E2.

### Summary

Port the values, parser, built-ins, and embedding API to Rust, one step at a time, each proved against the golden outputs.
````

## 4. Feature: Golden outputs of the C++ build

key `golden-behaviour`, under `characterise`.

````markdown
<!-- aayudha:survey product=tiny-js key=golden-behaviour -->

> From the survey of tiny-js: rests on E5, E6, E11, C7.

### Summary

The 38 test scripts and the command-line runner's output and errors, captured from the C++ build at a pinned commit.

### Problem

There is no product documentation, so the only definition of correct behaviour is what the C++ build does (E5, E6, E11).

### Scope

In: run_tests per script (G1), the command-line runner's stdout on each script (G2), its error output (G3). Out: the *.42.js tests and tests/42tests, which run_tests never runs (E11); Math.rand sequences (E16).
````

## 5. Story: Capture pass or fail of the 38 test scripts from the C++ build

key `capture-g1`, under `golden-behaviour`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=tiny-js key=capture-g1 -->

> From the survey of tiny-js: rests on E11, test:run_tests.cpp; proves parity with case sets G1.

### Summary

As the maintainer, I want each test script's pass or fail captured from the C++ build, so that the Rust build is held to the same results.

### Completion Goal

1. [api-trace] state: run_tests built with make from commit 8214477 and the cases file for case set G1; expected: aayu survey --golden writes 38 cases, each with exit code 0, and a manifest with the build's SHA-256

### Measure

cases captured with exit 0

### Threshold

38 of 38 in G1

### Scenario

Build with make, then run the G1 capture once.

### Environment

Linux, g++ and make, the repository's tests/ only
````

## 6. Story: Capture the command-line runner's output and errors

key `capture-g2-g3`, under `golden-behaviour`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=tiny-js key=capture-g2-g3 -->

> From the survey of tiny-js: rests on E5, E6, C7; proves parity with case sets G2, G3.

### Summary

As the maintainer, I want the command-line runner's output on every test script and on script errors captured, so that what a person sees stays the same.

### Completion Goal

1. [api-trace] state: Script built with make from commit 8214477 and case set G2 (37 scripts, test019 excluded for Math.rand); expected: aayu survey --golden captures each case's stdout and exit code 0
2. [api-trace] state: case set G3, a syntax error and a call of an undefined function; expected: the capture records an ERROR line with line and column on stdout and exit code 1

### Measure

cases captured

### Threshold

all of G2 and G3

### Scenario

Build with make, then run the G2 and G3 capture once.

### Environment

Linux, g++ and make, the repository's tests/ only
````

## 7. Feature: Script execution in Rust

key `rust-execution`, under `rust-engine`.

````markdown
<!-- aayudha:survey product=tiny-js key=rust-execution -->

> From the survey of tiny-js: rests on C1, E2, E3, E4.

### Summary

The lexer, values, and recursive-descent parser in Rust, running the same scripts with the same results.

### Problem

The C++ engine manages memory by reference counting that leaks cycles (E4), keeps children in linked lists (E3), and leaks every caught error (E15).

### Scope

In: lexer, values, parser, execution, error messages with positions (R1). Out: new language features and fixes to the known gaps until Q2 is answered.
````

## 8. Story: Port the lexer and value model to Rust

key `port-values`, under `rust-execution`, after `capture-g1`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=tiny-js key=port-values -->

> From the survey of tiny-js: rests on E3, E4, M2; proves parity with case sets G1.

### Summary

As the maintainer, I want the lexer and values in Rust with ownership instead of manual reference counts, so that cycles no longer leak.

### Completion Goal

1. [api-trace] state: the Rust test runner built from this branch and case set G1; expected: aayu survey --parity reports no difference for every G1 case whose script uses only literals, variables, and objects
2. [api-trace] state: a script that sets a.foo = a, run 10,000 times in one process; expected: resident memory after the last run is within 5% of after the first

### Measure

G1 differences; memory growth

### Threshold

0 differences; 5% growth or less

### Scenario

Replay G1, then run the cycle script in a loop.

### Environment

Linux, cargo, the repository's tests/
````

## 9. Story: Port the parser and execution to Rust

key `port-parser`, under `rust-execution`, after `port-values`, `capture-g2-g3`, labels `effort:L`.

````markdown
<!-- aayudha:survey product=tiny-js key=port-parser -->

> From the survey of tiny-js: rests on E1, E2, R1, M3; proves parity with case sets G1, G2, G3.

### Summary

As the maintainer, I want the parser and execution in Rust, so that every script runs as it did in C++.

### Completion Goal

1. [api-trace] state: the Rust test runner and case set G1; expected: aayu survey --parity reports no difference
2. [api-trace] state: the Rust command-line runner and case sets G2 and G3; expected: aayu survey --parity reports no difference, including the ERROR line's line and column

### Measure

differences from the golden outputs

### Threshold

0 in G1, G2, and G3

### Scenario

Replay the three case sets on the Rust build.

### Environment

Linux, cargo, the repository's tests/
````

## 10. Feature: Built-in and Math libraries in Rust

key `rust-builtins`, under `rust-engine`.

````markdown
<!-- aayudha:survey product=tiny-js key=rust-builtins -->

> From the survey of tiny-js: rests on C6, E7, E8.

### Summary

The string, array, integer, object, JSON, exec, eval, and Math built-ins in Rust.

### Problem

The built-ins are C++ callbacks registered by name (C6); the command-line runner registers only half of them (E7).

### Scope

In: every function registered today, with the same names and signatures. Out: registering Math in the command-line runner, unless Q4 says the runner is part of the product; reproducing Math.rand's sequence (Q5).
````

## 11. Story: Port the built-in and Math functions to Rust

key `port-builtins`, under `rust-builtins`, after `port-parser`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=tiny-js key=port-builtins -->

> From the survey of tiny-js: rests on C6, E8, M4; proves parity with case sets G1, G2.

### Summary

As the maintainer, I want every built-in function in Rust under the same name, so that scripts that call them keep working.

### Completion Goal

1. [api-trace] state: the Rust test runner and case set G1; expected: aayu survey --parity reports no difference
2. [api-trace] state: the Rust command-line runner and case set G2; expected: aayu survey --parity reports no difference
3. [api-trace] state: Math.randInt(1,6) called 1,000 times; expected: every value is an integer from 1 to 6

### Measure

differences; out-of-range values

### Threshold

0; 0

### Scenario

Replay G1 and G2, then call randInt in a loop.

### Environment

Linux, cargo, the repository's tests/
````

## 12. Feature: Embedding API for host applications

key `embedding-api`, under `rust-engine`.

````markdown
<!-- aayudha:survey product=tiny-js key=embedding-api -->

> From the survey of tiny-js: rests on E1, C2, C3, C4.

### Summary

Hosts create an interpreter, register native functions, execute and evaluate script, and read or set variables, from Rust and through a C ABI.

### Problem

Hosts call the C++ classes directly today (E1); a Rust engine needs an interface they can switch to.

### Scope

In: execute, evaluate, addNative, getVariable, setVariable as a Rust crate and a C ABI. Out: the CScriptVar class API, until Q1 says hosts use it.
````

## 13. Story: Ship the Rust crate and C ABI, then retire the C++ build

key `rust-api-and-abi`, under `embedding-api`, after `port-builtins`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=tiny-js key=rust-api-and-abi -->

> From the survey of tiny-js: rests on E1, C2, C3, C4, E9, M5; proves parity with case sets G1, G2, G3.

### Summary

As the maintainer, I want hosts to embed the Rust engine through the same five calls, so that the C++ build can go.

### Completion Goal

1. [api-trace] state: a C host program that registers print through the C ABI and executes tests/test001.js; expected: it prints the same output as the golden G2 case for test001 and exits 0
2. [api-trace] state: the Rust build and case sets G1, G2, and G3; expected: aayu survey --parity reports no difference in any of them

### Measure

differences from the golden outputs

### Threshold

0

### Scenario

Build the C host against the ABI, then replay the three case sets.

### Environment

Linux, cargo, cc, the repository's tests/
````

## 14. Bug: The known-issue list says postfix ++ is broken, but the code fixed it in 0.31

key `stale-known-issues`, under `golden-behaviour`, labels `effort:XS`.

````markdown
<!-- aayudha:survey product=tiny-js key=stale-known-issues -->

> From the survey of tiny-js: rests on E12, marker:TinyJS.cpp:111.

### Summary

The NOTE block at the top of TinyJS.cpp lists a bug the version history says was fixed and the code no longer has.

### Steps to reproduce

Run Script on: var a=1; var b=a++; print(b);

### Expected behaviour

The NOTE block lists only issues that still occur.

### Actual behaviour

It prints 1 (the previous value, which is correct), while TinyJS.cpp:111 says the current value is returned.

### Completion Goal

1. [api-trace] state: TinyJS.cpp after the fix; expected: the NOTE block no longer lists the postfix increment, and aayu survey --parity on case set G1 reports no difference

### Measure

stale entries in the NOTE block

### Threshold

0

### Scenario

Read the NOTE block against the golden outputs.

### Environment

the repository
````

## 15. Bug: run_tests overflows a 64-byte buffer for a long test path

key `fail-path-overflow`, under `golden-behaviour`, labels `effort:XS`.

````markdown
<!-- aayudha:survey product=tiny-js key=fail-path-overflow -->

> From the survey of tiny-js: rests on E14.

### Summary

A failing test whose path is longer than 55 characters overflows the buffer that holds its .fail.js name.

### Steps to reproduce

Copy a failing test to a path of 60 characters and run run_tests on it.

### Expected behaviour

The runner writes <path>.fail.js and reports FAIL.

### Actual behaviour

sprintf writes past the 64-byte stack buffer fn (run_tests.cpp:228-229).

### Completion Goal

1. [api-trace] state: run_tests built with -fsanitize=address and a failing test at a 60-character path; expected: FAIL is printed, the .fail.js file is written, and the sanitizer reports nothing

### Measure

sanitizer reports

### Threshold

0

### Scenario

Run the long-path failing test under AddressSanitizer.

### Environment

Linux, g++ with AddressSanitizer
````

## 16. Bug: Every caught script error leaks its exception object

key `error-leak`, under `golden-behaviour`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=tiny-js key=error-leak -->

> From the survey of tiny-js: rests on E15.

### Summary

Errors are thrown as heap pointers and the command-line runner never deletes them.

### Steps to reproduce

Run Script interactively and type a line with a syntax error 1,000 times.

### Expected behaviour

Memory stays flat across errors.

### Actual behaviour

Each error allocates a CScriptException that is never freed (TinyJS.cpp:1344, Script.cpp:103-105).

### Completion Goal

1. [api-trace] state: Script under valgrind fed 100 lines with a syntax error; expected: valgrind reports 0 bytes definitely lost, and aayu survey --parity on case set G3 reports no difference

### Measure

bytes definitely lost

### Threshold

0

### Scenario

Feed the error lines on stdin under valgrind.

### Environment

Linux, valgrind
````
