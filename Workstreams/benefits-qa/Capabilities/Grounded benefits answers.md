---
id: capability-benefits-qa-answers
type: capability
title: Grounded benefits answers
owner: product-strategist
people: [rahulnakmol]
status: live
workstream: benefits-qa
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Feature #127", "Feature #128", "Feature #129", "Release #144", "file examples/benefits-qa/README.md"]
up: "[[benefits-qa MOC]]"
issues: [127, 128, 129]
objectives: ["[[Benefits answers within a week]]"]
tags: [type/capability]
---

# Grounded benefits answers

## What it does

An employee asks a benefits question on the Ask page or through the API.
The answer quotes the plan passage that answers it, with its plan and section.
Questions about another person's records, or that ask for advice, are refused.
Questions the documents do not answer go to HR.
Product events go to an Application Insights export that aayu measure reads.

## Why it exists

Before the product, every benefits question was an HR ticket.
Most questions have an answer, word for word, in the plan documents.

## Who it serves

It serves the synthetic employees of Example Co.
Feature #127 is the answers API, Feature #128 the analytics, and Feature #129 the Ask page.
Release #144 shipped it on a rolling snapshot.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #127 Feature: Grounded answers API (benefits-qa).
- Recorded in #128 Feature: Answer analytics (benefits-qa).
- Recorded in #129 Feature: Ask page (benefits-qa).
<!-- END aayudha:brain:record -->

## Implementation history

<!-- BEGIN aayudha:brain:history -->
- Delivered in rolling-2026-10-12.4 by #130, #131, #132, #133, #134.
<!-- END aayudha:brain:history -->
