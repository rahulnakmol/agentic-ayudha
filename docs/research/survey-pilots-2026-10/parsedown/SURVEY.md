# Survey: parsedown

Parsedown converts Markdown to HTML in one PHP class with no runtime dependency beyond mbstring: text() renders blocks and inlines, line() renders inline Markdown only, and five setters switch hard breaks, markup escaping, URL autolinking, safe mode for untrusted input, and strict mode. Applications extend it by subclassing its protected block and inline methods. The readme names Laravel and several CMSs as users; which of our applications use which options and extension points is not in the code.

Asked for: "Parsedown is a legacy PHP Markdown library our applications depend on; it has a readme and no other product documentation. Tell me what it does, who relies on it, and how we would move it to modern PHP (8.2 or later, strict types, typed signatures) without changing a byte of its output.". Indexed at commit `4e433a8d5707`: 141 files, php (2379 lines).

Each claim says how it is known: **observed** (the code shows it), **stated** (a document or comment says it), **inferred** (reasoned from the cited code), **confirmed** (the person said it), or **unknown** (a question asks it). Citations are index fact IDs or `path:line` ranges at the commit above.

## Limits of this survey

- Calls are resolved by name only: 0 of 54 call edges name more than one definition.
- Dynamic dispatch the index cannot follow: `Parsedown.php:209`.
- Files with no language the index knows: .md (64), .html (64), .yaml (1), .dist (1), .gitattributes (1), .gitignore (1).
- Product claims come from documents, inference, or the person; code alone never shows who uses the software or why.

## Engineering

### Architecture

- **E1** (observed) The whole library is one class, Parsedown, in one file; its public surface is text, line, parse (an alias of text), instance, the five setters, and the version constant 1.8.0. `sym:Parsedown.php#Parsedown`, `entry:Parsedown.php:24`, `entry:Parsedown.php:1130`, `entry:Parsedown.php:1855`, `entry:Parsedown.php:1939`, `Parsedown.php:20`, `Parsedown.php:59-102`.
- **E2** (observed) Block parsing is line based: the first character of each line selects candidate block types from BlockTypes, with indented code as the only unmarked type. `Parsedown.php:126-156`, `sym:Parsedown.php#Parsedown::linesElements`, `doc:readme.md`, `readme.md:78-82`.
- **E4** (observed) Inline parsing scans for the markers in inlineMarkerList and tries the inline types listed for each marker in InlineTypes. `Parsedown.php:1109-1124`, `sym:Parsedown.php#Parsedown::lineElements`.

### Data

- **E5** (observed) Blocks and inlines build a tree of element arrays (name, attributes, text, element, elements, handler, rawHtml) that element and elements render to HTML, escaping text with htmlspecialchars in UTF-8. `sym:Parsedown.php#Parsedown::element`, `sym:Parsedown.php#Parsedown::elements`, `sym:Parsedown.php#Parsedown::handle`, `Parsedown.php:1920-1923`.

### Interfaces

- **E3** (observed) Each block type is handled by methods found by name at run time: block\<Type>, block\<Type>Continue, and block\<Type>Complete, so the method names are part of the behaviour. `io:dynamic-dispatch:Parsedown.php:209`, `Parsedown.php:206-209`, `sym:Parsedown.php#Parsedown::isBlockContinuable`, `sym:Parsedown.php#Parsedown::isBlockCompletable`.
- **E6** (observed) Safe mode sanitises every element: attribute names must match a safe pattern, link and image URLs keep their scheme only if it is on a whitelist (http, https, ftp, mailto, tel, three base64 image types, and others), else ':' becomes '%3A'; raw HTML blocks, inline markup, and comments are escaped. `Parsedown.php:104-120`, `sym:Parsedown.php#Parsedown::sanitiseElement`, `sym:Parsedown.php#Parsedown::filterUnsafeUrlInAttribute`, `Parsedown.php:405`, `Parsedown.php:831`, `Parsedown.php:1465`, `Parsedown.php:1687-1692`.
- **E7** (observed) Raw HTML that an extension produces is escaped in safe mode unless the element sets allowRawHtmlInSafeMode. `Parsedown.php:1732`, `test/SampleExtensions.php:22-40`, `test/ParsedownTest.php:61-76`.
- **E8** (inferred) Applications extend Parsedown by subclassing it and overriding protected methods such as blockFencedCodeComplete, so protected method names, their array arguments, and the BlockTypes and InlineTypes tables are an extension contract. `test/SampleExtensions.php:3-19`, `Parsedown.php:126-150`, `Parsedown.php:1109-1120`, `doc:readme.md`. Reasoning: The test extensions override a protected method and edit its element array, the dispatch tables are protected rather than private, and the readme lists Extensible and a Markdown Extra extension as features (readme.md:14, readme.md:17).

### Dependencies

- **E9** (observed) It requires PHP 7.2 or later and the mbstring extension at run time, and PHPUnit 8.5 or 9.6 for its tests. `dep:composer:php`, `dep:composer:ext-mbstring`, `composer.json:15-21`, `Parsedown.php:188`.

### Tests

- **E10** (observed) ParsedownTest renders each of the 64 test/data/\*.md files and compares with the .html file of the same name, switching safe mode on for names starting xss and strict mode for names starting strict. `test:test/ParsedownTest.php`, `test/ParsedownTest.php:44-59`.
- **E11** (observed) The CommonMark tests download the specification from the master branch of jgm/CommonMark on every run, so they need the network and their cases change when the specification changes. `test:test/CommonMarkTestStrict.php`, `test/CommonMarkTestStrict.php:12`, `test/CommonMarkTestStrict.php:38-43`.

### Risks

- **E12** (stated) Escaping markup (setMarkupEscaped) is not a security setting: it escapes HTML but leaves unsafe link schemes such as javascript: in place; only safe mode sanitises URLs. `readme.md:64-74`, `Parsedown.php:1687-1692`.
- **E13** (observed) The readme says Parsedown is tested in PHP 7.1+, but composer.json requires PHP 7.2 or later. `readme.md:16`, `composer.json:16`.
- **E14** (observed) instance() keeps named instances in a static array, so a setter called on a shared instance (for example setSafeMode(false)) changes the output for every caller that uses that name. `sym:Parsedown.php#Parsedown::instance`, `Parsedown.php:1939-1953`.

### Quality

- **E15** (observed) The code has no type declarations on public method parameters or return values, and no declare(strict\_types=1). `Parsedown.php:24`, `Parsedown.php:59`, `Parsedown.php:1130`, `Parsedown.php:1855`.

## Capability map

| ID | Capability | What it does | Basis | Evidence |
|---|---|---|---|---|
| C1 | Render Markdown to HTML | Converts a Markdown document to an HTML fragment, normalising line endings first. | observed | `sym:Parsedown.php#Parsedown::text`, `Parsedown.php:37-53`, `readme.md:34-38` |
| C2 | Render inline Markdown only | Converts one line of inline Markdown without wrapping it in a paragraph. | observed | `entry:Parsedown.php:1130`, `readme.md:40-44` |
| C3 | GitHub-flavoured extras | Supports tables, fenced code with a language class, strikethrough, and URL autolinking, which can be switched off. | observed | `Parsedown.php:141`, `Parsedown.php:147-149`, `Parsedown.php:1118`, `sym:Parsedown.php#Parsedown::inlineUrl`, `Parsedown.php:1532` |
| C4 | Safe rendering of untrusted input | In safe mode, escapes raw HTML and neutralises link and image URLs whose scheme is not on the whitelist. | observed | `entry:Parsedown.php:86`, `Parsedown.php:104-120`, `sym:Parsedown.php#Parsedown::sanitiseElement`, `readme.md:48-56` |
| C5 | Rendering options | Hard line breaks on every newline, escaped markup for trusted input, and strict mode that requires a space after the # of a heading. | observed | `entry:Parsedown.php:59`, `Parsedown.php:1247`, `entry:Parsedown.php:68`, `entry:Parsedown.php:95`, `Parsedown.php:550` |
| C6 | Extension by subclassing | A subclass can change how any block or inline type is parsed or rendered by overriding its protected methods. | inferred | `test/SampleExtensions.php:3-19`, `io:dynamic-dispatch:Parsedown.php:209` |

## Product

- **P1** (stated) The readme names Laravel, Bolt, Grav, Herbie, Kirby, October, Pico, and Statamic CMSs, phpDocumentor, RaspberryPi.org, and the Symfony Demo as users. `readme.md:88-90`.
- **P2** (stated) Its job is turning Markdown that authors or users write into HTML inside PHP applications, installed with composer or by including one file. `readme.md:7`, `readme.md:19-27`, `composer.json:3`.
- **P3** (stated) The readme sells it as one file with no dependencies, fast, extensible, and GitHub-flavoured. `readme.md:9-17`.
- **P4** (stated) For applications that show user input, the outcome that matters is HTML free from cross-site scripting, with safe mode as the first line of defence and a sanitiser and Content-Security-Policy as further ones. `readme.md:48-62`.
- **P5** (stated) It passes most but not all CommonMark tests, so exact CommonMark compliance is not a property today. `readme.md:84-86`.
- **P6** (unknown, Q1) Which of our applications use Parsedown, with which options, and whether any subclass it or use Parsedown Extra, is not in the code.
- **P7** (unknown, Q2) Whether output that differs from CommonMark today must stay as it is is not in the code.

## Questions for the person

| ID | Question | Why it matters | Default | Status |
|---|---|---|---|---|
| Q1 | Which of your applications subclass Parsedown or use Parsedown Extra, and which protected methods or tables do they override? | If any do, the modern version must keep those protected method names and array shapes, or ship a compatibility layer; typing them would break the subclasses. | Assume Parsedown Extra and at least one subclass exist: keep every protected method name and its array arguments in the first modern release, and type only the public methods. | open |
| Q2 | Must the output stay byte-identical where Parsedown differs from CommonMark today, or may the modern version move towards CommonMark? | It decides whether the 64 golden fixtures are the contract or a baseline that later Stories may change one by one. | Byte-identical in the first release, as you asked; any CommonMark change later is its own Story that updates a golden case on purpose. | open |
| Q3 | Do any callers rely on Parsedown::instance() sharing settings between callers? | Shared static instances (E14) are hard to type and test; the answer decides whether to keep, deprecate, or remove them. | Keep instance() and deprecate it, since removing it breaks callers silently. | open |
| Q4 | Which PHP versions must the modern version support? | Typed properties need 7.4, union types 8.0, readonly 8.1; the floor decides which language features the port may use. | PHP 8.2 and later, the version the characterisation ran on. | open |

## Modernisation path: Modern PHP (8.2 or later) with strict types and typed signatures, output unchanged

Target from the request. Characterise the current output first, make the tests hermetic, then modernise in place in small steps (strict types, then typed public API, then a typed extension API behind a compatibility layer), each proved byte-identical against the golden outputs. (`test:test/ParsedownTest.php`, `entry:Parsedown.php:24`)

### Steps

1. **M1** Capture golden HTML for the fixtures, the options, and the security modes. Replaces `mod:(root)`, `mod:test`. Rests on E10, C1, C2, C4, C5. Parity: G1, G2, G3.
2. **M2** Make the CommonMark tests hermetic by pinning the specification. Replaces `mod:test`. After M1. Rests on E11. Parity: G1.
3. **M3** Declare strict types and type the public API. Replaces `mod:(root)`. After M1. Rests on E1, E15, E9. Parity: G1, G2, G3.
4. **M4** Type the extension API behind a compatibility layer for existing subclasses. Replaces `mod:(root)`. After M3. Rests on E3, E8, C6. Parity: G1, G2, G3.

### Parity case sets

- **G1** The 64 Markdown fixtures, xss\* in safe mode and strict\* in strict mode. Inputs from `test:test/ParsedownTest.php`, `test/ParsedownTest.php:44-59`; compared exact stdout (the HTML) and exit code.
- **G2** Public API options: text and line on the readme example, hard breaks, URL autolinking off. Inputs from `readme.md:34-44`, `entry:Parsedown.php:59`, `entry:Parsedown.php:77`; compared exact stdout and exit code.
- **G3** Hostile input in default, escaped, and safe modes. Inputs from `readme.md:48-74`, `sym:Parsedown.php#Parsedown::sanitiseElement`; compared exact stdout and exit code.

### Modernisation risks

- **R1** (inferred) Adding return types to the protected methods breaks every subclass that overrides them without one (a fatal error when the class loads); adding parameter types alone does not. `test/SampleExtensions.php:5`, `test/SampleExtensions.php:24`. Reasoning: PHP lets an override widen a parameter type (an untyped parameter is wider) but requires its return type to be compatible with the parent's; the test extensions declare blockFencedCodeComplete($Block) with no return type. Checked on PHP 8.2: an untyped override of f(array $b) loads, and of f(array $b): array is a fatal error.
- **R2** (observed) Escaping relies on htmlspecialchars with ENT\_QUOTES or ENT\_NOQUOTES in UTF-8; a change of flags changes every fixture's output. `Parsedown.php:1920-1923`.

## Coverage

- Modules cited: 2 of 2.
- Entry points cited: 10 of 10.
- Known-issue markers cited: 0 of 0.
- Open questions: 4; unknown claims: 2.
