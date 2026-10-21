# Rubric: survey draft of a legacy PHP Markdown library, modernised in place

The deterministic checks have passed: every citation resolves and each claim's basis is allowed. Judge what they
cannot: whether the claims are true of the code shown, useful, and honest about what is unknown. Compare every claim
you can with the files the host read; a claim the files contradict is a fabrication.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when the draft would
mislead a reader on any point, and 1 when it contains a fabrication (a claim the cited code does not support, a user
the readme does not name presented as fact, or a number nothing measures).

1. **Engineering depth.** It explains the single-class design, line-based block parsing by first character, inline
   parsing by marker, the element tree rendered to HTML, the dynamic dispatch of block methods by name (and what that
   means for a rewrite), safe mode's URL whitelist and escaping, the options, the dependencies, and the tests (the
   fixture pairs and how file names switch modes; the CommonMark tests' network dependency). It cross-checks the
   readme against the code and composer.json and reports any contradiction it can see.
2. **Capability map.** Capabilities are what a PHP developer can do with the library: render documents and inline
   text, the GitHub-flavoured extras, safe rendering of untrusted input, the rendering options, and extension by
   subclassing.
3. **Product level is honest.** Users, jobs, value, and outcomes come from the readme (stated) or are unknown with a
   question; the person's own applications are unknown until asked. The security outcome (XSS-free HTML from
   untrusted input) is stated with the readme's own caveats, including that escaping markup is not a security setting.
4. **Questions are the right ones.** Few, ordered by how much the answer changes the modernisation (which extension
   points subclasses use, whether output must stay identical where it differs from CommonMark, shared instances, the
   PHP floor), each with why it matters and a default with a reason. No question is answered by the skill.
5. **Modernisation path proves parity.** The target is modern PHP, from the request, modernised in place rather than
   rewritten. Characterisation comes first: golden HTML for the fixtures, the options, and hostile input in each
   security mode; each step names its case sets; the risks name what can change output or break extensions (typing
   protected methods that subclasses override, escaping flags).
