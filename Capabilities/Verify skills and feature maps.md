---
id: capability-verify-maps
type: capability
title: Verify skills and feature maps
owner: product-strategist
people: [rahulnakmol]
status: live
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["issue #268", "file docs/guide/how-verify-maps.md", "file docs/guide/guide-prove-behaviour.md"]
up: "[[Capabilities]]"
objectives: ["[[Ship aayudha 0.1]]", "[[Prove every Story of aayu itself]]"]
tags: [type/capability]
---

# Verify skills and feature maps

## What it does

verify runs the floors on a change.
A repository can have its own verify skill with feature maps.
Behaviour checks then block a change that breaks a mapped feature.

## Why it exists

Generic floors do not know what a product must do.
A map of the product's features lets proof cover behaviour, not only code.

## Who it serves

It serves the AI Native Engineer seat that runs verify.
It also serves the Quality Architect seat that owns the floors.
It shipped in [[v0.2.0]].
