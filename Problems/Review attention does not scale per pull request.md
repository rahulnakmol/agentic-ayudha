---
id: problem-review-attention
type: problem
title: Review attention does not scale per pull request
owner: product-strategist
status: open
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["Decision #572", "Decision #593"]
up: "[[Problems]]"
tags: [type/problem]
---

# Review attention does not scale per pull request

## The problem

Agents open pull requests faster than a person can pass them one at a time.
A pass on every diff spends a person's judgement where it buys the least.

## Who has it

The person who must approve agent work, most of all in a solo pod.
See [[The solo founder]].

## Evidence

Decision #572 measured 100 pull requests merged into dev in about 12 hours.
The rules of that time needed a person to pass 90 of them.
Even with every streak full, 73 would still need a pass.
See [[A person's attention by exception]] and [[Merge on proof into dev]].
