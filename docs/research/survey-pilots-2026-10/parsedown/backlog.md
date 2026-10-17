# Backlog draft: parsedown

Nothing here is filed yet. Review every item below: it is the exact issue that will be filed (1 goal, 2 initiative, 3 feature, 4 story, 1 bug).

Review digest `3337b82e2ef5ec8a`. To file it after review:

```sh
aayu survey --backlog backlog.json --draft survey.json --create --reviewed 3337b82e2ef5ec8a
```

## Open questions

- Q1 (open): which applications subclass Parsedown or use Parsedown Extra; decides whether protected methods may ever get return types
- Q2 (open): byte-identical where Parsedown differs from CommonMark, or move towards it; no item changes output until answered
- Q3 (open): whether callers rely on instance() sharing settings; no item touches instance() until answered
- Q4 (open): the PHP floor; the items assume 8.2, the version characterised

## 1. Goal: Parsedown keeps rendering our users' Markdown safely while it moves to modern PHP

key `goal`.

````markdown
<!-- aayudha:survey product=parsedown key=goal -->

> From the survey of parsedown: rests on P2, P4.

### Summary

Applications that turn authors' and users' Markdown into HTML (P2), some of it untrusted (P4), get the same HTML from a strictly typed library on PHP 8.2 or later.
````

## 2. Initiative: Pin down today's output before changing the code

key `characterise`, under `goal`.

````markdown
<!-- aayudha:survey product=parsedown key=characterise -->

> From the survey of parsedown: rests on M1, M2, E10, E11.

### Summary

Golden HTML for the fixtures, the options, and the security modes, and tests that run without the network.
````

## 3. Initiative: Modernise the library in place with strict types

key `modern-php`, under `goal`.

````markdown
<!-- aayudha:survey product=parsedown key=modern-php -->

> From the survey of parsedown: rests on M3, M4, E15.

### Summary

Strict types and typed signatures, public API first, extension API behind a compatibility layer, with byte-identical output.
````

## 4. Feature: Golden HTML of the current release

key `golden-html`, under `characterise`.

````markdown
<!-- aayudha:survey product=parsedown key=golden-html -->

> From the survey of parsedown: rests on E10, C1, C2, C4, C5.

### Summary

The HTML Parsedown 1.8.0 produces for the 64 fixtures, the public options, and hostile input in each mode, captured at a pinned commit.

### Problem

Nothing but the fixtures says what the output must be, and the options and security modes have no fixtures of their own (E10).

### Scope

In: G1 (64 fixtures), G2 (text, line, breaks, autolinking off), G3 (hostile input in default, escaped, and safe modes). Out: the CommonMark specification tests, handled in hermetic-commonmark.
````

## 5. Story: Capture golden HTML from Parsedown 1.8.0

key `capture-golden`, under `golden-html`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=parsedown key=capture-golden -->

> From the survey of parsedown: rests on E10, C4, test:test/ParsedownTest.php; proves parity with case sets G1, G2, G3.

### Summary

As the maintainer, I want today's HTML captured for every fixture, option, and security mode, so that each modernisation step is held to it.

### Completion Goal

1. [api-trace] state: Parsedown.php at commit 4e433a8 on PHP 8.2 and the cases file for case sets G1, G2, and G3; expected: aayu survey --golden captures 71 cases with exit code 0 and a manifest naming the commit
2. [api-trace] state: the captured G1 outputs and test/data/*.html; expected: all 64 are byte-identical

### Measure

cases captured; fixtures matching their .html

### Threshold

71; 64 of 64

### Scenario

Run the capture once with the characterisation driver.

### Environment

Linux, PHP 8.2 CLI with mbstring, the repository only
````

## 6. Story: Run the CommonMark tests without the network

key `hermetic-commonmark`, under `golden-html`, after `capture-golden`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=parsedown key=hermetic-commonmark -->

> From the survey of parsedown: rests on E11, M2; proves parity with case sets G1.

### Summary

As the maintainer, I want the CommonMark tests to read a pinned copy of the specification, so that they pass or fail for the same reason every time.

### Completion Goal

1. [api-trace] state: phpunit with the network disabled; expected: CommonMarkTestStrict and CommonMarkTestWeak run against the pinned specification file and report the same counts on two runs
2. [api-trace] state: case set G1; expected: aayu survey --parity reports no difference

### Measure

network requests during the test run

### Threshold

0

### Scenario

Run phpunit twice with outbound network blocked.

### Environment

Linux, PHP 8.2, PHPUnit 9.6, no network
````

## 7. Feature: Strict types and a typed public API

key `typed-public-api`, under `modern-php`.

````markdown
<!-- aayudha:survey product=parsedown key=typed-public-api -->

> From the survey of parsedown: rests on E1, E15, E9.

### Summary

declare(strict_types=1) and typed parameters and returns on text, line, parse, instance, and the five setters.

### Problem

No public method declares a type (E15), so callers learn about wrong arguments only from wrong output.

### Scope

In: the public methods and the version constant. Out: protected methods, which subclasses override (typed-extension-api).
````

## 8. Story: Declare strict types and type the public methods

key `strict-types`, under `typed-public-api`, after `capture-golden`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=parsedown key=strict-types -->

> From the survey of parsedown: rests on E1, E15, M3; proves parity with case sets G1, G2, G3.

### Summary

As a developer calling Parsedown, I want typed public methods under strict types, so that a wrong argument fails at the call instead of producing wrong HTML.

### Completion Goal

1. [api-trace] state: the modernised Parsedown.php on PHP 8.2 and case sets G1, G2, and G3; expected: aayu survey --parity reports no difference in 71 cases
2. [api-trace] state: text(null) called from a file with strict types; expected: a TypeError naming text and its string parameter

### Measure

differences from the golden HTML

### Threshold

0

### Scenario

Replay the three case sets, then call text with null.

### Environment

Linux, PHP 8.2 CLI with mbstring
````

## 9. Feature: Typed extension API that existing subclasses still load with

key `typed-extension-api`, under `modern-php`.

````markdown
<!-- aayudha:survey product=parsedown key=typed-extension-api -->

> From the survey of parsedown: rests on E3, E8, C6.

### Summary

Typed parameters on the protected block and inline methods, without return types that would break overriding subclasses (R1).

### Problem

Extensions override protected methods found by name (E3, E8); adding return types would stop them loading.

### Scope

In: parameter types on protected methods, the BlockTypes and InlineTypes tables unchanged. Out: return types on protected methods until Q1 says which subclasses exist.
````

## 10. Story: Type the protected methods' parameters and keep subclasses loading

key `type-protected-params`, under `typed-extension-api`, after `strict-types`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=parsedown key=type-protected-params -->

> From the survey of parsedown: rests on E3, E8, C6, M4; proves parity with case sets G1, G2, G3.

### Summary

As a maintainer of a Parsedown extension, I want my subclass to keep loading and rendering the same HTML after the library is typed, so that I can upgrade without changes.

### Completion Goal

1. [api-trace] state: test/SampleExtensions.php unchanged, loaded with the modernised Parsedown; expected: both classes load and testRawHtml and testTrustDelegatedRawHtml pass
2. [api-trace] state: case sets G1, G2, and G3; expected: aayu survey --parity reports no difference

### Measure

subclasses that fail to load; differences

### Threshold

0; 0

### Scenario

Run the extension tests, then replay the case sets.

### Environment

Linux, PHP 8.2, PHPUnit 9.6
````

## 11. Bug: The readme says PHP 7.1+ but composer requires 7.2

key `readme-php-version`, under `golden-html`, labels `effort:XS`.

````markdown
<!-- aayudha:survey product=parsedown key=readme-php-version -->

> From the survey of parsedown: rests on E13.

### Summary

The readme and composer.json disagree on the oldest supported PHP version.

### Steps to reproduce

Read readme.md line 16 and composer.json line 16.

### Expected behaviour

Both state the same minimum version.

### Actual behaviour

The readme says Tested in PHP 7.1+; composer.json requires php >=7.2.

### Completion Goal

1. [api-trace] state: readme.md and composer.json after the fix; expected: both name the same minimum PHP version, and composer validate passes

### Measure

documents that disagree

### Threshold

0

### Scenario

Compare the two files.

### Environment

the repository
````
