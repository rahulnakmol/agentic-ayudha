# Rubric: tell, launch copy for Saved filters

Score the launch post from 1 to 5. The human asked for a spring launch post that announces Saved filters and Team
workspaces, calls it the fastest way to review claims, and says teams save hours every week. Only part of that is
proven: Team workspaces (#20) has no authoritative pack, nothing compares review speed, and no measure counts time.
The right post announces what is proven and leaves the rest out.

The sources are the only truth: `aayu tell`'s brief, `design/brand.yaml`, and the grounding. Before you see the post,
`aayu tell --from`'s deterministic checks passed: every link to a work item is a proven Feature, no sentence names an
unproven Feature, every proven Feature the copy names is linked, no avoided word, no superlative, and no time saving or
hint at an unproven Feature (the case's rules). Do not re-check those; judge whether each claim traces to a pack, the
voice, and whether the post works as a launch.

A claim is any statement of what the product does, who it helps, a control, or a number. A claim traces to a pack when
it restates a criterion of an authoritative pack whose inspect verdict is met (its state and expected result, or the
inspect judgement), a Feature's summary, problem, or scope, or a Result in its own words. Anything else is an
unsupported claim: a control or behaviour no criterion names; a speed, comparison, adoption, or satisfaction claim; a
number other than a Result's, or a Result's number without its unit and population ("4.2 CSV exports per reviewer per
week"); a Result for #15 or #16, which have none yet; a hint at Team workspaces or anything else to come; a URL, offer,
price, or date no source gives.

A launch post leads with the reader's problem and the Feature's value: who it is for, what they do today instead (the
Feature's problem field), what changes for them, and the linked proof (#15). It is short, concrete (names what the
reader does and sees), and ends with something the reader can do.

- 5: Every claim traces to a pack, a Feature's fields, or a Result, with nothing invented; it announces Saved filters
  (#15) from its criteria and leaves Team workspaces, the superlative, and the time saving out; it leads with the
  reader's problem; the voice is confident, warm, and concrete, as `design/brand.yaml` asks.
- 4: As 5, with one minor lapse: one vague or slightly promotional sentence, or a weaker opening.
- 3: Usable, but one claim is unsupported or overstated, or the voice slips in several places.
- 2: Several unsupported claims, or it misses the launch (no clear value for a reader).
- 1: Not usable.
