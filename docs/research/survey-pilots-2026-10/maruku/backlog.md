# Backlog draft: maruku

Nothing here is filed yet. Review every item below: it is the exact issue that will be filed (1 goal, 2 initiative, 4 feature, 7 story, 3 decision).

Review digest `a00e8f7ff530fa99`. To file it after review:

```sh
aayu survey --backlog /home/user/workspace/wt-survey-evals/evals/cases/survey/maruku-backlog/output/backlog.json --draft /home/user/workspace/wt-survey-evals/evals/cases/survey/maruku-backlog/survey.json --create --reviewed a00e8f7ff530fa99
```

## 1. Goal: Existing pages keep reading the same while the sites move off obsolete Maruku

key `goal`.

````markdown
<!-- aayudha:survey product=maruku key=goal -->

> From the survey of maruku: rests on P2, P3, P4.

### Summary

Maruku's own readme calls it obsolete (P2). The sites' pages use tables, footnotes, and header attribute lists (P3) and must render to equivalent HTML (P4) on a maintained Markdown library, with Maruku removed.
````

## 2. Initiative: Record how Maruku renders today before anything changes

key `characterise`, under `goal`.

````markdown
<!-- aayudha:survey product=maruku key=characterise -->

> From the survey of maruku: rests on M1, E5.

### Summary

Capture Maruku's results on its own block docs as the reference every later change is held to (M1, E5).
````

## 3. Initiative: Render with a maintained library behind Maruku's entry point

key `replace`, under `goal`.

````markdown
<!-- aayudha:survey product=maruku key=replace -->

> From the survey of maruku: rests on M2, M3, K1.

### Summary

An adapter keeps Maruku.new(text, options).to_html for the render helper (K1, SH1) while a maintained library does the work, then Maruku and the adapter go (M2, M3).
````

## 4. Feature: Maruku's results on its block docs

key `legacy-results`, under `characterise`.

````markdown
<!-- aayudha:survey product=maruku key=legacy-results -->

> From the survey of maruku: rests on E5, M1.

### Summary

The block docs' inputs and expected HTML, run on Maruku, are case set G1 and suite S1.

### Problem

Nothing records which documents Maruku itself renders as expected, so a difference on the new library could be an old failure, not a regression.

### Scope

In: every spec/block_docs document, run on Maruku, as golden outputs (G1) and as the legacy side of the suite (S1). Out: the divs, math, and meta-data documents for parity (Q2).
````

## 5. Story: Capture Maruku's pass, fail, or skip on every block doc

key `capture-legacy`, under `legacy-results`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=maruku key=capture-legacy -->

> From the survey of maruku: rests on E5, M1; proves parity with G1.

### Summary

As the developer moving the sites, I want each block doc's result on Maruku captured, so that the new library is held to what Maruku actually did, failures included.

### Completion Goal

1. [api-trace] state: Maruku at commit 4d88d0e and a cases file with one case per spec/block_docs document (set G1); expected: aayu survey --golden --run writes one result per document and a manifest with the build's SHA-256
2. [api-trace] state: the same cases with --probe; expected: probes.json records no non-deterministic case

### Measure

block docs with a captured result

### Threshold

every spec/block_docs document

### Scenario

Run the G1 capture once on Maruku, then once more under --probe.

### Environment

Ruby 3.1 with the Gemfile's gems; the adapter gem and Maruku 4d88d0e side by side; spec/block_docs only
````

## 6. Feature: Maruku's entry point over the maintained library

key `adapter`, under `replace`.

````markdown
<!-- aayudha:survey product=maruku key=adapter -->

> From the survey of maruku: rests on E1, E2, K1, K3.

### Summary

A Maruku class in an adapter gem keeps Maruku.new(text, options).to_html and to_html_document (SH1), with options passed per document.

### Problem

The render helper calls Maruku.new(text, fenced_code_blocks: true).to_html (Q3); changing every call site and the renderer at once would leave no way to compare.

### Scope

In: to_html and to_html_document with the fenced_code_blocks option (K1, K3). Out: MaRuKu::Globals and every other option (K3), and the maruku command (K4).
````

## 7. Story: Render through the adapter with per-document options and no global state

key `adapter-to-html`, under `adapter`, after `capture-legacy`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=maruku key=adapter-to-html -->

> From the survey of maruku: rests on E1, E2, K1, K3; proves parity with G1; keeps compatibility interfaces K1, K3; does not carry over N1, N3.

### Summary

As the developer moving the sites, I want Maruku.new(text, options).to_html to call the maintained library, so that the render helper keeps working unchanged while the library underneath is replaced.

### Completion Goal

1. [api-trace] state: the adapter as --bin and case set G1 (divs, math, and meta-data documents excluded); expected: aayu survey --parity reports no difference in any case's exit code
2. [api-trace] state: the adapter gem loaded in a Ruby process; expected: Array.instance_methods(false) has no replace_each_string and the adapter defines no MaRuKu::Globals

### Measure

G1 cases whose result differs from Maruku's

### Threshold

0

### Scenario

Swap the --bin of the G1 cases from Maruku to the adapter and run --parity.

### Environment

Ruby 3.1 with the Gemfile's gems; the adapter gem and Maruku 4d88d0e side by side; spec/block_docs only
````

## 8. Story: Score the adapter on Maruku's block-doc suite per area

key `compat-suite`, under `adapter`, after `adapter-to-html`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=maruku key=compat-suite -->

> From the survey of maruku: rests on E5, K1, K2; proves parity with S1; keeps compatibility interfaces K1.

### Summary

As the developer moving the sites, I want the block-doc suite scored per area on the adapter, so that every regression is visible before a page depends on it.

### Completion Goal

1. [api-trace] state: suite S1 with Maruku's results captured and the adapter as --bin; expected: aayu survey --compat appends a score per area to compat-history.jsonl and lists each test Maruku passes that the adapter fails

### Measure

S1 tests Maruku passes that the adapter fails, in the kept areas

### Threshold

a recorded count per area, the baseline the later stories bring to 0

### Scenario

Run --compat for S1 once with the adapter.

### Environment

Ruby 3.1 with the Gemfile's gems; the adapter gem and Maruku 4d88d0e side by side; spec/block_docs only
````

## 9. Feature: Tables, footnotes, and header attribute lists on the new library

key `extensions`, under `replace`.

````markdown
<!-- aayudha:survey product=maruku key=extensions -->

> From the survey of maruku: rests on C1, C2, K2.

### Summary

The Maruku syntax the pages use (Q2) renders to equivalent HTML on the new library.

### Problem

The maintained library's own syntax for these features may differ from Maruku's, and pages rely on Maruku's.

### Scope

In: tables, footnotes, and attribute lists on headers (K2). Out: divs, math, meta-data headers, and <?maruku ?> instructions (K2, N2).
````

## 10. Story: Render tables, footnotes, and header attribute lists as Maruku did

key `port-extensions`, under `extensions`, after `compat-suite`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=maruku key=port-extensions -->

> From the survey of maruku: rests on C2, T8, K2; proves parity with G1, S1; keeps compatibility interfaces K1, K2; does not carry over N2.

### Summary

As an editor of the sites, I want my tables, footnotes, and header attributes to render as before, so that no page changes when the renderer does.

### Completion Goal

1. [api-trace] state: the adapter as --bin and suite S1; expected: aayu survey --compat reports 0 regressions in the tables, footnotes-abbreviations, attributes, and headers areas
2. [api-trace] state: case set G1; expected: aayu survey --parity reports no difference

### Measure

S1 regressions in the tables, footnotes-abbreviations, attributes, and headers areas

### Threshold

0

### Scenario

Run --compat for S1 and --parity for G1 with the adapter.

### Environment

Ruby 3.1 with the Gemfile's gems; the adapter gem and Maruku 4d88d0e side by side; spec/block_docs only
````

## 11. Feature: Switch the sites to the new library and retire Maruku

key `cut-over`, under `replace`.

````markdown
<!-- aayudha:survey product=maruku key=cut-over -->

> From the survey of maruku: rests on E1, K1, K4.

### Summary

The sites render with the adapter after a shadow period, then Maruku, its command, and finally the adapter are removed.

### Problem

A difference on a real page that the block docs do not cover would only show after the switch.

### Scope

In: shadow rendering of every page, the switch and its rollback, the command's removal (K4), and the shim's removal (SH1). Out: new Markdown features.
````

## 12. Story: Render every page with both renderers and report the differences before switching

key `shadow-pages`, under `cut-over`, after `port-extensions`, labels `effort:M`.

````markdown
<!-- aayudha:survey product=maruku key=shadow-pages -->

> From the survey of maruku: rests on E1, E5, K1; proves parity with G1, S1; keeps compatibility interfaces K1.

### Summary

As the developer moving the sites, I want each build to render every page with Maruku and with the adapter and report the pages whose HTML differs, so that the switch happens only when no page changes.

### Completion Goal

1. [api-trace] state: every page of the sites rendered on one build with Maruku and with the adapter; expected: a report lists each page whose HTML is not equivalent (whitespace between tags ignored), and lists none
2. [api-trace] state: suite S1 and case set G1 on the adapter; expected: aayu survey --compat reports 0 regressions in the kept areas and aayu survey --parity reports no difference

### Measure

pages whose HTML is not equivalent

### Threshold

0 over the shadow period the person sets

### Scenario

Run the shadow render on each build of the sites until the person's shadow period ends.

### Environment

the sites' build with both renderers installed
````

## 13. Story: Switch the renderer to the adapter, with a tested switch back

key `switch-and-rollback`, under `cut-over`, after `shadow-pages`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=maruku key=switch-and-rollback -->

> From the survey of maruku: rests on E1, K1, K4; proves parity with G1; keeps compatibility interfaces K1.

### Summary

As the developer moving the sites, I want one setting that picks the renderer, tested both ways, so that a bad page after the switch is one change away from Maruku again.

### Completion Goal

1. [api-trace] state: the renderer setting on the adapter; expected: the sites build and case set G1 passes --parity
2. [api-trace] state: the setting switched back to Maruku; expected: the sites build with Maruku and G1 passes --parity against the captured results
3. [api-trace] state: the adapter gem's first release notes; expected: they say the maruku command is not shipped (K4)

### Measure

builds that pass with each setting

### Threshold

2 of 2

### Scenario

Build once with each renderer setting.

### Environment

the sites' build
````

## 14. Story: Call the new library directly and remove the Maruku adapter class

key `remove-shim`, under `cut-over`, after `switch-and-rollback`, labels `effort:S`.

````markdown
<!-- aayudha:survey product=maruku key=remove-shim -->

> From the survey of maruku: rests on E1, K1, M2; proves parity with G1.

### Summary

As the developer of the sites, I want the render helper to call the maintained library itself, so that the adapter (SH1) stops being code we keep.

### Completion Goal

1. [api-trace] state: the render helper calling the new library, the Maruku class deprecated for one release; expected: G1 passes --parity with the helper as --bin
2. [api-trace] state: the release after; expected: the Gemfile has neither maruku nor the adapter gem, and the sites build

### Measure

gems named maruku or the adapter in the Gemfile

### Threshold

0

### Scenario

Deprecate the class for one release, then remove it.

### Environment

the sites' build
````

## 15. Decision: Do not carry over Maruku's patch to Array

key `no-array-patch`, under `adapter`, constrains `adapter-to-html`.

````markdown
<!-- aayudha:survey product=maruku key=no-array-patch -->

> From the survey of maruku: rests on N1, T1.

### Question

Maruku reopens Array to add replace_each_string for every program that loads it (N1, T1). The person said nothing calls it (Q5).

### Options

1. Keep the patch in the adapter. 2. Drop it; any helper the adapter needs is private to it.

### Evaluation

Keeping it changes a core class for every program that loads the adapter and can clash with another library (N1).

### Recommendation

Option 2, as the person confirmed (Q5).

### Reversible

true
````

## 16. Decision: Do not carry over process-wide settings

key `no-global-settings`, under `adapter`, constrains `adapter-to-html`.

````markdown
<!-- aayudha:survey product=maruku key=no-global-settings -->

> From the survey of maruku: rests on N3, T6.

### Question

Maruku reads settings from MaRuKu::Globals and sticky globals such as $warned_nokogiri (N3, T6). The person said the helper passes options per document and never touches Globals (Q3).

### Options

1. Keep a Globals hash in the adapter. 2. Options per document only, no global state.

### Evaluation

With global settings, one caller's settings or one failed gem load change every other document's output in the process (N3).

### Recommendation

Option 2, as the person confirmed (Q3).

### Reversible

true
````

## 17. Decision: Do not carry over executing Ruby embedded in documents

key `no-embedded-code`, under `extensions`, constrains `port-extensions`.

````markdown
<!-- aayudha:survey product=maruku key=no-embedded-code -->

> From the survey of maruku: rests on N2, T3.

### Question

With unsafe features on, Maruku passes each <?maruku ?> instruction to instance_eval (N2, T3). The person said no page has one and unsafe features were never on (Q6).

### Options

1. Support <?maruku ?> in the new library. 2. Never execute document content.

### Evaluation

Whoever writes a page would run code on the machine that renders it (N2).

### Recommendation

Option 2, as the person confirmed (Q6).

### Reversible

false
````
