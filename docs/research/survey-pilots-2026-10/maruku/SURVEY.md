# Survey: maruku

Maruku is a Ruby library and command-line tool that parses a superset of Markdown (the original syntax, PHP Markdown Extra, and its own meta-data, attribute, div, and math syntax) into a document tree and renders it as HTML, LaTeX, Markdown, or S5 slides. Its readme calls it obsolete and points to kramdown. Which of its extensions and options the person's pages use, and how exactly their HTML must be kept, are unknown and asked. Its own block-doc suite, input and expected HTML per feature, is the natural compatibility reference. It also patches Ruby's Array class for every program that loads it and can execute Ruby code embedded in a document when unsafe features are on.

Asked for: "Our sites render Markdown with Maruku, which its own readme calls obsolete. Tell us what it does, what our existing pages depend on that must keep working, what we should not carry over, and how we move to a maintained Ruby Markdown library without breaking existing pages.". Indexed at commit `4d88d0ed43de`: 282 files, ruby (8253 lines), perl (45 lines), shell (7 lines).

Each claim says how it is known: **observed** (the code shows it), **stated** (a document or comment says it), **inferred** (reasoned from the cited code), **confirmed** (the person said it), or **unknown** (a question asks it). Citations are index fact IDs or `path:line` ranges at the commit above.

## Limits of this survey

- Calls are resolved by name only: 27 of 339 call edges name more than one definition.
- Dynamic dispatch the index cannot follow: `lib/maruku/element.rb:61`, `lib/maruku/ext/math/to_html.rb:51`, `lib/maruku/input/charsource.rb:240`, `lib/maruku/input/parse_doc.rb:118`, `lib/maruku/input_textile2/t2_parser.rb:140`, `lib/maruku/output/to_html.rb:891`, `lib/maruku/output/to_latex.rb:540`, `lib/maruku/output/to_markdown.rb:147`, `spec/block_spec.rb:66`.
- Files with no language the index knows: .md (163), .page (14), .orig (1), .svg (2), .css (4), .xml (1), .template (1), (none) (1), .yaml (2), .yml (2), .editorconfig (1), .7 (1), .yardopts (1), .tex (3), .old (1), .gitignore (1), .simplecov (1), .png (5).
- Product claims come from documents, inference, or the person; code alone never shows who uses the software or why.

## Engineering

### Architecture

- **E1** (observed) Maruku.new(text, options) merges the options into the document's attributes and parses the text into a tree of MDElement nodes; to\_html renders the tree's children as an HTML fragment and appends the footnotes, and to\_html\_document wraps it in an XHTML 1.1 plus MathML document. `lib/maruku/maruku.rb:6-11`, `lib/maruku/output/to_html.rb:68-94`.

### Data

- **E4** (observed) Raw HTML inside a document is parsed by REXML unless html\_parser is 'nokogiri'; when Nokogiri fails to load, Maruku warns once, sets $warned\_nokogiri, and uses REXML for the rest of the process. `lib/maruku/html.rb:29-43`, `trap:ruby-globals:lib/maruku/html.rb:3`.

### Interfaces

- **E2** (observed) Settings come from the document's own attributes first and the process-wide MaRuKu::Globals hash otherwise; the defaults include unsafe\_features off, on\_error :warning, html\_parser 'rexml', fenced\_code\_blocks off, and no math engine. `lib/maruku/defaults.rb:3-38`, `lib/maruku/defaults.rb:40-44`.
- **E3** (observed) The maruku command converts each file named on the command line, or standard input, to HTML, an HTML fragment, LaTeX (and PDF through pdflatex), Markdown, S5, or the parse tree; its -u, -m, and -i options set process-wide Globals, and --ext requires maruku/ext/\<name> for each name given. `entry:bin/maruku:1`, `bin/maruku:27-87`, `bin/maruku:99-130`.

### Dependencies

- **E6** (observed) The gem declares no runtime dependency and Ruby 1.8.7 or later; Nokogiri, syntax highlighting, and itextomml are optional, listed only for testing. `maruku.gemspec:27`, `Gemfile:7-13`.

### Tests

- **E5** (observed) The block-doc suite (spec/block\_spec.rb) reads each spec/block\_docs/\*.md file as a comment, parameters, Markdown input, the expected tree, and the expected to\_html and to\_latex output, and compares HTML through nokogiri-diff, ignoring whitespace-only text nodes; PENDING docs are skipped. `test:spec/block_spec.rb`, `spec/block_spec.rb:46-62`, `spec/block_spec.rb:79-97`.

### Risks

- **E7** (inferred) With maruku\_signature on, rendering a whole HTML document fails: nice\_date calls t.day and t.strftime, and t is never assigned. `lib/maruku/output/to_html.rb:259-263`, `lib/maruku/output/to_html.rb:224-226`. Reasoning: Running Maruku.new('hello', :maruku\_signature => true).to\_html\_document raised NameError: undefined local variable or method `t'. The setting is off by default (lib/maruku/defaults.rb:9).

### Quality

- **E8** (observed) A TODO at the code-execution hook says executed code blocks are never removed from the document. `marker:lib/maruku/input/parse_doc.rb:67`.

## Capability map

| ID | Capability | What it does | Basis | Evidence |
|---|---|---|---|---|
| C1 | Render Markdown to HTML | Turn Markdown with PHP Markdown Extra additions (tables, footnotes, abbreviations, definition lists) into an HTML fragment or a full XHTML document. | observed | `lib/maruku/output/to_html.rb:68-94`, `README.md:5-9` |
| C2 | Use Maruku's own extensions | Attach attributes to elements with attribute lists, add document meta-data, wrap content in divs, and write inline and display math. | observed | `lib/maruku/ext/div.rb:11-15`, `lib/maruku/input/parse_doc.rb:98-106`, `README.md:9-11` |
| C3 | Export to other formats | Render the same document as LaTeX (and PDF), Markdown, or an S5 slideshow. | observed | `bin/maruku:120-143` |
| C4 | Convert files from the command line | Convert one or more files, or standard input, with the maruku command. | observed | `entry:bin/maruku:1`, `bin/maruku:99-113` |

## Product

- **P1** (stated) Maruku is a Markdown-superset interpreter implementing the original syntax, PHP Markdown Extra, and its own meta-data syntax. `README.md:3-9`.
- **P2** (stated) Its readme says it should be considered obsolete, superseded by libraries like kramdown. `README.md:17`.
- **P3** (unknown, Q2) Which of the person's pages and sites depend on which Maruku features is not in the code.
- **P4** (unknown, Q1) What 'without breaking existing pages' means for the HTML (byte-identical or equivalent) is not in the code.

## Traps checked

Places where the stack's semantics differ from what a port would assume, each checked in the code (references/traps.md).

- **T1** (observed) Array gains replace\_each\_string in lib/maruku/element.rb, which lib/maruku.rb requires, so every program that loads Maruku has it; the String additions in input\_textile2 load only through maruku/textile2, which lib/maruku.rb does not require. `trap:ruby-monkey-patch:lib/maruku/element.rb:130`, `lib/maruku/element.rb:130-138`, `lib/maruku.rb:66`, `trap:ruby-monkey-patch:lib/maruku/input_textile2/t2_parser.rb:1`. Rule `ruby-monkey-patch`: Which core classes does Maruku reopen, and is the patch loaded by every program that requires maruku?
- **T2** (observed) Only CharSourceDebug, which runs two character sources side by side and compares them; it is commented out, and CharSource is CharSourceManual. `trap:ruby-method-missing:lib/maruku/input/charsource.rb:240`, `lib/maruku/input/charsource.rb:17-19`, `lib/maruku/input/charsource.rb:234-244`. Rule `ruby-method-missing`: Which class answers unknown methods through method\_missing, and is it ever used?
- **T3** (observed) Yes, when unsafe features are on: each \<?maruku … ?> processing instruction in a document is passed to instance\_eval; the setting is off by default and the command's -u option turns it on process-wide. `trap:ruby-eval:lib/maruku/input/parse_doc.rb:113`, `lib/maruku/input/parse_doc.rb:65-68`, `lib/maruku/input/parse_doc.rb:111-131`, `bin/maruku:34-36`, `lib/maruku/defaults.rb:4`. Rule `ruby-eval`: Does any document content reach eval or instance\_eval?
- **T4** (observed) It builds output by appending to string literals (to\_html, the line source, the parsers), and on the checker build with frozen string literals 156 of the 162 spec cases print nothing, failing with FrozenError at lib/maruku/input/linesource.rb:83. `trap:ruby-string-mutation:lib/maruku/output/to_html.rb:52`, `lib/maruku/output/to_html.rb:68-72`, `lib/maruku/input/linesource.rb:83`, `probe:check-differs:easy`, `probe:check-differs:footnotes`. Reasoning: aayu survey --probe --check-bin legacy-checked.rb (ruby --enable=frozen-string-literal -W:deprecated) recorded check-differs for 156 of 162 cases; rerunning easy.md by hand shows FrozenError: can't modify frozen String at linesource.rb:83. Rule `ruby-string-mutation`: Does the code mutate strings in place, and does it still run when string literals are frozen?
- **T5** (observed) The blahtex math engine calls File.exists?, which references/traps.md lists as removed in Ruby 3.2, so that engine fails there; the default engine is none. `trap:ruby-removed-api:lib/maruku/ext/math/mathml_engines/blahtex.rb:15`, `lib/maruku/defaults.rb:18`. Rule `ruby-removed-api`: Does it call APIs that current Ruby versions removed?
- **T6** (observed) $warned\_nokogiri makes a failed Nokogiri load permanent for the process, so with html\_parser 'nokogiri' the HTML parser depends on whether the gem loads; other globals mark one-time warnings and lazy loads ($syntax\_loaded, $already\_warned\_itex2mml). `trap:ruby-globals:lib/maruku/html.rb:3`, `lib/maruku/html.rb:29-41`, `trap:ruby-globals:lib/maruku/output/to_html.rb:518`. Rule `ruby-globals`: Which global variables change behaviour for the rest of the process?
- **T7** (observed) The maruku command's --ext option requires "maruku/ext/#{name}" for each comma-separated name the user passes. `bin/maruku:85-87`. Rule `ruby-dynamic-require`: Is any require built from input?
- **T8** (observed) Attribute and meta-data keys read from a document become symbols, so a document's "Title:" header is looked up as :title. `trap:ruby-symbol-string-keys:lib/maruku/input/parse_doc.rb:103`, `lib/maruku/input/parse_doc.rb:98-106`. Rule `ruby-symbol-string-keys`: Where do strings from documents become symbols, and does a port keep the same keys?

## Questions for the person

| ID | Question | Why it matters | Default | Status |
|---|---|---|---|---|
| Q1 | Must existing pages render to byte-identical HTML, or is equivalent HTML enough (the same elements, attributes, and text, with whitespace between tags free, as Maruku's own suite compares)? | It sets the level of the HTML interface (K1) and how the suite and the shadow run compare. | Equivalent HTML, compared as block\_spec.rb compares it, since pages are read in browsers and the suite already accepts whitespace changes. | open |
| Q2 | Which Maruku syntax do your pages use beyond Markdown: tables, footnotes, abbreviations, definition lists, attribute lists, meta-data headers, divs, or math? | Each extension a maintained library lacks becomes an adapter or a page migration (K2, M3). | Scan the pages for each extension's markers before deciding, since nothing in this repository says. | open |
| Q3 | Which options do your sites set, and where: per document (Maruku.new(text, options)) or process-wide in MaRuKu::Globals? | Options decide the output (E2) and process-wide settings are on the do-not-carry list (N3). | Carry the options you set as per-document options of the adapter. | open |
| Q4 | Does anything run the maruku command, and with which options? | The command and its flags are an interface only if a build or a person uses them (K4). | Treat the command as unused unless a build script names it. | open |
| Q5 | Does any of your code call replace\_each\_string on an Array, or depend on Maruku having added it? | Maruku adds the method to every Array in the process (T1); dropping it (N1) breaks any such caller. | Drop it; a search of your code for replace\_each\_string settles it. | open |
| Q6 | Do any pages contain \<?maruku … ?> instructions, and does anything render them with unsafe features on (-u or :unsafe\_features)? | Maruku executes such instructions as Ruby when unsafe features are on (T3); not carrying that over (N2) changes those pages. | Do not carry it over; pages that use it get a reviewed replacement. | open |

## Do not carry over

What the modern version deliberately does not reproduce. The person decides each; a confirmed entry becomes a Decision that constrains the Stories porting its code.

| ID | Feature | Evidence | Risk of copying it | Replacement | Decision |
|---|---|---|---|---|---|
| N1 | Reopening Array to add replace\_each\_string | `lib/maruku/element.rb:130-138`; T1 | Every program that loads the library gets a changed core class, which can clash with another library's method of the same name. | A private helper on the document tree, or a refinement used only inside the library. | proposed (Q5) |
| N2 | Executing Ruby code embedded in a document (\<?maruku … ?> with unsafe features on) | `lib/maruku/input/parse_doc.rb:111-131`, `bin/maruku:34-36`; T3 | Whoever writes a page runs code on the machine that renders it. | Never execute document content; anything a page computed becomes a reviewed template helper outside the document. | proposed (Q6) |
| N3 | Process-wide settings in MaRuKu::Globals and sticky globals | `lib/maruku/defaults.rb:3-38`, `lib/maruku/html.rb:29-41`, `bin/maruku:30-58`; T6 | One caller's settings, or one failed gem load, change the output of every other document in the process. | Options passed per document to the adapter, with no global state. | proposed (Q3) |

## Compatibility contract

Every interface others depend on, at the level the person decides: **exact** (must match), **differ** (may differ, with the recorded decision), **drop** (removed, with a deprecation path), or **undecided** (a question asks).

| ID | Kind | Interface | Level | Decided by | Proof | Who depends on it |
|---|---|---|---|---|---|---|
| K1 | api | Maruku.new(text, options).to\_html and to\_html\_document: the HTML each page renders to `lib/maruku/maruku.rb:6-11`, `lib/maruku/output/to_html.rb:68-94` | undecided | Q1 | S1, G1 | the person's sites (which pages: Q2) |
| K2 | file-format | Maruku's Markdown dialect: PHP Markdown Extra plus attribute lists, meta-data, divs, and math `README.md:5-11`, `lib/maruku/ext/div.rb:11-15`, `lib/maruku/input/parse_doc.rb:98-106` | undecided | Q2 | S1 | the pages' authors and their existing documents |
| K3 | config | Document options and MaRuKu::Globals settings `lib/maruku/defaults.rb:3-38`, `lib/maruku/maruku.rb:9` | undecided | Q3 | – | – |
| K4 | cli | The maruku command and its options `bin/maruku:27-87` | undecided | Q4 | – | unknown (Q4) |

## Modernisation path: A maintained Ruby Markdown library

Target from the request. Make Maruku's own block docs the compatibility reference and capture the legacy results, then put the maintained library behind an adapter that keeps Maruku.new(text, options).to\_html, add back only the Maruku extensions the pages use (Q2), and shadow both renderers on the real pages until the differences are the ones the person accepted. (`test:spec/block_spec.rb`, `README.md:17`)

### Steps

1. **M1** Capture the legacy results of the block-doc suite and of the sites' pages. Replaces `mod:spec`. Rests on E5. Parity: G1.
2. **M2** An adapter with Maruku's entry points over the maintained library, with per-document options. Replaces `mod:lib`, `mod:lib/maruku`. After M1. Rests on E1, E2. Does not carry over N1, N3. Parity: G1.
3. **M3** The Maruku-only syntax the pages use, as extensions of the new library. Replaces `mod:lib/maruku/ext`, `mod:lib/maruku/input`. After M2. Rests on C2, T8. Does not carry over N2. Parity: G1.

### Parity case sets

- **G1** The block docs' Markdown inputs and expected HTML. Inputs from `test:spec/block_spec.rb`, `spec/block_docs/tables.md:1-5`, `spec/block_docs/attributes/attributes.md:1-5`; compared equivalent HTML as block\_spec.rb compares it (whitespace-only text nodes ignored) and exit code, until Q1 decides.

### Compatibility suites

- **S1** Maruku's block docs, one test per document. From `test:spec/block_spec.rb`, `spec/block_docs/tables.md:1-5`; scored per area: core, lists, code, tables, html, links-images, headers, footnotes-abbreviations, attributes, divs, encoding, math. Cases: `compat.json`.

### Coexistence before cut-over: shadow

Render every page with Maruku and with the adapter on each build and report the pages whose HTML differs beyond what Q1 allows. Compared on G1, S1. Cut over when S1 passes every test the legacy build passes, except differences the person accepted and recorded, and the sites' pages show no unaccepted difference over a shadow period the person sets. Rollback: Switch the sites' renderer setting back to Maruku, which stays installed until the shims are removed.

### Compatibility shims

- **SH1** for K1: Maruku.new(text, options).to\_html and to\_html\_document keep their names and signatures, turned on by a Maruku class in the adapter gem that calls the new library. Removal: When the sites call the new library directly, deprecate the Maruku class for one release, then remove it..

### Modernisation risks

- **R1** (observed) Maruku-only syntax (attribute lists, divs, meta-data, math) has no counterpart in a plain Markdown library; check each extension the pages use against the candidate before choosing it. `lib/maruku/ext/div.rb:11-15`, `README.md:9-11`.
- **R2** (observed) Raw HTML in pages passes through REXML by default and Nokogiri by option, so the legacy output already depends on the parser setting; capture the goldens with the setting the sites use. `lib/maruku/html.rb:29-43`, `lib/maruku/defaults.rb:15`.

## Coverage

- Modules cited: 9 of 15. Not cited: `mod:(root)`, `mod:docs/exd`, `mod:docs/website`, `mod:lib/maruku/ext/math`, `mod:lib/maruku/output/s5`, `mod:tools`.
- Entry points cited: 1 of 228.
- Known-issue markers cited: 1 of 18. Not cited: `marker:lib/maruku/ext/div.rb:11`, `marker:lib/maruku/ext/math/elements.rb:13`, `marker:lib/maruku/ext/math/parsing.rb:18`, `marker:lib/maruku/ext/math/to_html.rb:55`, `marker:lib/maruku/ext/math/to_html.rb:109`, `marker:lib/maruku/html.rb:199`, `marker:lib/maruku/input/parse_block.rb:115`, `marker:lib/maruku/input/parse_block.rb:470`, `marker:lib/maruku/input/parse_block.rb:605`, `marker:lib/maruku/input/parse_block.rb:650`, `marker:lib/maruku/input/parse_span.rb:669`, `marker:lib/maruku/input_textile2/t2_parser.rb:119`, `marker:lib/maruku/input_textile2/t2_parser.rb:120`, `marker:lib/maruku/input_textile2/t2_parser.rb:121`, `marker:lib/maruku/output/to_html.rb:11`, `marker:lib/maruku/output/to_latex.rb:442`, `marker:lib/maruku/output/to_latex.rb:573`.
- Trap facts checked: 9 of 58. Not checked: `trap:generated-code:lib/maruku/output/s5/fancy.rb:20`, `trap:ruby-define-method:lib/maruku/toc.rb:153`, `trap:ruby-dynamic-require:bin/marutex:4`, `trap:ruby-dynamic-require:spec/block_spec.rb:4`, `trap:ruby-dynamic-require:spec/cli_spec.rb:1`, `trap:ruby-dynamic-require:spec/span_spec.rb:1`, `trap:ruby-dynamic-require:spec/to_html_utf8_spec.rb:3`, `trap:ruby-eval:docs/exd/exd.rb:62`, `trap:ruby-eval:spec/block_spec.rb:66`, `trap:ruby-eval:spec/span_spec.rb:5`, `trap:ruby-eval:spec/to_html_utf8_spec.rb:10`, `trap:ruby-globals:lib/maruku/ext/math/mathml_engines/itex2mml.rb:3`, `trap:ruby-globals:lib/maruku/ext/math/mathml_engines/ritex.rb:4`, `trap:ruby-globals:lib/maruku/ext/math/to_html.rb:76`, `trap:ruby-globals:spec/block_spec.rb:65`, `trap:ruby-process-hooks:bin/maruku:75`, `trap:ruby-process-hooks:docs/website/compose-page.rb:3`, `trap:ruby-process-hooks:lib/maruku/input/charsource.rb:256`, `trap:ruby-process-hooks:lib/maruku/output/s5/fancy.rb:713`, `trap:ruby-send:lib/maruku/element.rb:61` and 29 more (the index lists them).
- Open questions: 6; unknown claims: 2.
