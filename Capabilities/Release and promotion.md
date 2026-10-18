---
id: capability-release-promotion
type: capability
title: Release and promotion
owner: product-strategist
people: [rahulnakmol]
status: live
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Feature #35", "file GLOSSARY.md", "file docs/guide/how-releases.md", "tag v0.2.0", "PR #443", "file AGENTS.md"]
up: "[[Capabilities]]"
issues: [35]
objectives: ["[[Ship aayudha 0.1]]"]
tags: [type/capability]
---

# Release and promotion

## What it does

aayu release promotes work from dev to rolling, then from rolling to main.
Each step runs a gate in CI on the exact merge commit that will land.
A step to main needs a person's stable-release request.
A tag on main triggers a signed release with provenance.

## Why it exists

Every release must trace back to the commit and workflow that built it.
See [[Small, reversible steps]].

## Who it serves

It serves people who install aayu, and the DevOps & DevEx Tinkerer seat that owns release.
See [[v0.1.0]] and [[v0.2.0]].
