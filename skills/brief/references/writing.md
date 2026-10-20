# Writing rule

This rule covers every text a person reads, decides on, or must understand: a PR body, an inspect comment, a Ready
snapshot, a Decision pack, release notes, and a brief. The twin that owns the verb writes the text, in its own voice.

## Simplified Technical English

Write the plain sentences in the manner of Simplified Technical English (ASD-STE100):

- Keep each sentence to 20 words or fewer (`brief.MaxSTEWords`).
- Put one idea in each sentence. Give one instruction in each sentence, and start it with a verb.
- Use the active voice. Say who does the action.
- Use the approved words in [ste-words.md](ste-words.md), each with its one meaning. Technical names pass: code in
  backticks, numbers, acronyms, and names.
- Follow the brand tone in `design/brand.yaml`: plain, direct, and no superlatives.

`brief.LintSTE` checks only three things: sentence length, approved words, and one instruction per sentence. The other
items are writing guidance that no check reads. A task that sets another register (eli5, engineer, regulatory) keeps
that register.

## Voice

Speak in the voice of the seat that owns the verb, from its persona (`seats/<slug>/persona.toml`). Nakul writes the
build PR body, Drona the inspect comment, Arjun the Ready snapshot, Sauti the release notes, and the owning seat a
Decision pack. No check reads personality. The voice changes the tone, never the facts.

## Structure carries facts

Tables, code, and diagrams carry the facts. The 20-word limit does not apply to them. List items and callouts are
sentences, so the limit applies to them. Keep a long fact in a table cell, not in a long sentence.

## Name the person

Name a person by their GitHub login: "@rahulnakmol approves this in `aayu inbox`", not "the founder approves".
Name whoever asked, or whoever owns the step. Keep a role word only when the role is the subject: the Founder seat, a
charter, or a rule about who may accept a Decision. Even there, name the person who holds the role. In product docs
and release notes for every user of aayu, call the reader "you". Name a person only for this repository's own steps.

## Evidence comes first

STE and voice change how you say a claim. They never change whether you cite it. The evidence rule stands: every claim
about the code, architecture, risk, or progress cites its source in the same paragraph or list item.

## PR bodies

The PR body format is in `.github/pull_request_template.md`. Write its plain sections to this rule. `aayu inspect`
checks the sections and the sentence limit on an agent PR.

- Start with a one-line summary and the At a glance table.
- Use bullets, or a before-and-after table, for what changed.
- Add at most one Mermaid diagram, and only when a flow explains the change better than words. Use `flowchart TD`
  with short node labels: GitHub clips a wide left-to-right chart.
- Put a RAID table (Risk, Assumption, Issue, Dependency) in Risk and rollback when the PR has real risk, assumptions,
  open issues, or dependencies. List each PR it stacks on as a Dependency. A person's step (a secret, an approval, an
  inbox pass) is a Dependency that the person owns, by login. A small, self-contained PR has no RAID table.
- Use `> [!WARNING]` for a breaking change or a step a person must do, and `> [!NOTE]` for context.
- Put the technical detail in a `<details>` block in For reviewers.
