# Release notes and changelogs that readers keep

The `release` and `changelog` kinds, part by part. The rules come from a study of the release notes of 20
open-source projects and 20 products (private research note `release-notes-2026-10.md`, §3 and §5). Every part here
still obeys the claims rules in SKILL.md: a part with no source is left out, never filled in.

A reader opens release notes to answer three questions: what do I get, what must I do, and what breaks. The notes
people keep answer all three on the first screen. Then they use the same shape every release, so the reader learns
it once. None of this needs an adjective.

## The skeleton

```markdown
# <product> <version>

<date>. <One sentence: the headline Features, and the version they follow.>

**Read this if you are:**       <- only when readers need different sections

- **On your own:** <the sections you need, and why, if a source gives the why.>
- **In a small team:** <…>
- **In a medium team:** <…>

## Upgrade first            <- only when a source names a step

1. <One action: the exact command or setting.> <How you see that it worked.> <Who: you alone, an admin, or the
   team's agreement.>

## Breaking changes         <- only when a source names one

| Change | What replaces it | Who it affects | What you do |
|---|---|---|---|

## <Feature title> ([#N](<url>))

- <Verb-first: what changed, one bullet per criterion.>
- <Who it helps, as a need.>
- <On your own: … In a team: …>   <- only when a source shows they differ
- <How to start: the control or command the criterion names.>
- <A limit from the scope, when the reader would expect more.>

<The count of the changes you cannot describe, when there are any.>

## Thanks                   <- only when a source names a person

<@login for <the work>.>
```

At a startup stage this fits on one page. With no step and no break, it is the title, the summary, the short reader
block when readers need different sections, one section per proven Feature, and the count, in at most 400
words. When the parts do not fit, keep every proven Feature with what changed and its link, and cut first the
limits, then the solo and team lines, then how to start. Never drop a Feature, a step, or a break to fit: if the
page is still over 400 words, keep it complete, and say in your reply how long it is and why.

## Readers: from one person to a medium team

The notes serve three readers, sized as the team stages are: a person on their own, a small team (up to 15 people),
and a medium team (up to 150). They read the same facts and need different parts first. The team stage
(`[team] stage`) says which reader comes first and how deep each part goes. It never changes what you may claim.

The reader the request or the brief names comes first. Without one, the stage decides:

| Stage | Reader first | Reader block | Who on each step | Solo or team difference |
|---|---|---|---|---|
| startup | on your own | at most three short lines | a word: "You alone", "Admin", "Team" | a short clause, only where it differs |
| scaleup | small or medium team | three lines, each with a reason that traces | one sentence: who, and why (the review or the permission) | its own sentence, only where it differs |
| enterprise | medium team | three lines, each with a reason that traces | one sentence, as at scaleup | its own sentence, only where it differs |

## Each part

### The summary sentence

- Name the headline Features in the reader's words, as the criteria say them. Name the version this release follows when
  the grounding lists it: "v2.3.0 follows v2.2.0 and adds …". Never "Since v2.2.0, you can …": that dates the
  Feature to the earlier release.
- One sentence. It is the only part most readers read, so it carries the release's main change, not a mood. Write
  "This release lets you pin a note to the top of a list", not "This release is about focus".
- A theme in the title ("v2.3.0: notes you keep in view") is a claim like any other. Use one only when it restates a
  criterion.

### Read this if you are…

- Right after the summary, when readers need different sections: a step that needs an admin or the team's
  agreement, or a change whose meaning differs on your own and in a team, as a source shows. When every reader needs
  the same sections, leave the block out: a block that says the same thing three times is filler.
- One line for each reader: on your own, in a small team, in a medium team. Leave out a reader whose line would only
  repeat another's.
- Each line points to sections: "On your own: read Upgrade first, then Pinned notes." A reason in a line is a claim:
  "because the step needs an admin" needs its source like any other sentence.
- Never sort readers by how much they will like the release. Sort them by what they must read.

### Upgrade first

- Put it before the changes. The upgrade is the one part that can hurt the reader.
- Write a step only when a criterion or an inspect judgement names it. Never invent a command,
  flag, setting, or path to complete a sequence.
- One action a step, the exact text the reader types in a code span, then the check: what the reader sees when it
  worked, as the criterion's expected result says it.
- Say who can do each step: you alone, an admin (a person with the permission the step needs, named as the source
  names it), or the team's agreement (a review, an approval, or a shared setting others rely on). Take it from the
  source: the criterion or the inspect judgement that names the permission, review, or approval. When no source says,
  leave it off, and tell the human in your reply so they can add it from a source.
- When no source names a step, leave the section out. Do not write "No action is needed": that claims something about
  every change.

### Breaking changes

- One place, one table. A change that alters what the reader already uses goes here, never in a Feature's bullets.
- Each row: what changed (Removed, Changed, or Deprecated, then the item), what replaces it, who it affects, and what
  they do. Add the version or date only when a source gives it. Never "in a future release". When no source gives
  a replacement or an action, put a dash in that cell and tell the human in your reply; never fill it with a guess.
- When the scope says who is not affected ("only when you set X"), say it: it lets most readers stop reading.
- When no source names a break, leave the section out. Never write "Nothing breaks".

### One section per proven Feature

- The heading is the Feature's title with its link, `## Pinned notes ([#N](<url>))`. The link is the proof and the
  depth: the reader who wants more follows it.
- Then five short parts, in this order, each from its source:

  | Part | Source | Example |
  |---|---|---|
  | What changed | a met criterion, restated, verb first | "Pin a note, and it stays at the top of its list." |
  | Who it helps | the Feature's problem, as a need | "For people who look for the same note every day." |
  | On your own, in a team | a criterion with a second person, or a setting someone must approve | "On your own, a pin keeps your note in view. In a team, another person who opens the list sees the pin." |
  | How to start | the control or command the criterion names | "Choose **Pin**." (the control the criterion names) |
  | A limit | the scope's exclusions | "Pins apply to one list; a pinned note does not move to other lists." |

- Write the solo and team lines only when a source shows the difference. When the change means the same for
  everyone, one line serves both: do not invent a team benefit.
- Leave out a part with no source. Keep the limit only when a reader would otherwise expect more. Say "does not",
  never "not yet" or "today": those hint at work that is not proven.
- A number appears only from a Result or a criterion, in its own unit and population (and, for a Result, its window).
- A picture is a claim too. Use the pack's own screenshot or recording only when the grounding gives its full URL,
  with alt text that restates the criterion. Never build a URL from an evidence path.

### The closing count

- The last line of the changes counts what shipped and is not described: unproven Features' stories and bugs without
  a Feature. "Two more changes shipped in this release. They are not described here, because their proof is not
  complete yet."
- Never name, link, or describe those changes, and never say what is next.

### Credits

- Name a person only when a source names them: by GitHub login, next to the work, such as "Pinned notes (#N), by
  @login". A bot or a review app is not a credit. With no person named in the sources, write no credits.

## Changelog entries

A changelog entry is a section of a changelog file: `## <version>` and the date, then the summary sentence, then
bullets. "Upgrade:" bullets give each step with who can do it. "Breaking:" bullets start with Removed, Changed, or
Deprecated. Each proven Feature gets one bullet per criterion, starting with "Added", with its link. Then the count.
It has no reader block: a changelog reader scans by version, not by role.

## Check before you hand over

Run `aayu tell release --from notes.md --tag <tag>` until it passes; for a changelog, run
`aayu tell changelog --from changelog.md --tag <tag>`, so its links and its ledger path carry the right kind. Then read the draft once more as a judge would,
because the binary does not check these:

1. Every claim, including the title, the summary, each step, each table row, each code block, and each diagram label,
   restates a criterion, an inspect judgement, a Feature field, or a Result. Nothing from an earlier release is new
   here.
2. On the first screen (the title, the summary, and the reader block), the reader knows what they get, sees that a
   step or a break follows when a source names one, and, when readers need different sections, knows which ones to
   read.
3. Every step comes before the changes, with its check, and with who can do it where a source names who (else your
   reply tells the human it is missing). Every break is in the table (in a changelog, a "Breaking:" bullet), with
   what replaces it where a source names it (else a dash, and your reply says so).
4. Each Feature section has what changed, then who it helps, what it means on your own and in a team, how to
   start, and a limit, in the skeleton's order, each only where a source gives it.
5. The sections are in the skeleton's order, and none is empty.
6. Plain, calm, second person, one idea per sentence; no avoided word, superlative, or exclamation mark.
7. Every number has its unit, population, and source.
8. Credits name only people the sources name, by GitHub login.
9. The count is right, and the page fits the team stage.

## What readers do not keep

- Hype or a mood in place of the change ("a huge update", "this release is all about you").
- A list of commit or pull request titles as the notes.
- A break in a positive section ("Improvements"), or a date given as "a future release".
- Empty headings, or a different section order each release.
- Jokes or filler that hide what changed.
- Text left over from a draft ("we will share a guide before the release").
