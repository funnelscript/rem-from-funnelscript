---
name: rem
description: >
  Unified Reverse Engineering & Clean-Room Software Reconstruction System. Live browser extraction, two-tier DOM PII masking, token extraction, Given/When/Then business rules, launch barrier matrices, interactive REPORT.html, coding-agent handoffs, and MCP tools.
author: REM Team
repository: https://github.com/imMamdouhaboammar/REM
metadata:
  author: REM Team
  repository: https://github.com/imMamdouhaboammar/REM
---

# REM | Reverse Engineering & Software Reconstruction Engine

> Autonomous execution engine for behavioral reverse engineering, clean-room software reconstruction, and acquisition intelligence.

## Why This Skill Is Needed & Target State

**Why this skill is needed:** AI coding agents attempting to study existing SaaS products or rebuild systems independently often resort to ungrounded guessing, miss subtle business rules, violate clean-room boundaries, leak private customer PII in screenshots, or create tangled dependency deadlocks.

**Target state:** Every reverse engineering task follows strict clean-room governance (L1–L4 safety environments, A0–A5 action classes, and 4-step write gates), extracts ground-truth DOM and design tokens via automated browser drivers, codifies Given/When/Then rules and launch barriers, and outputs deterministic handoffs (`REPORT.html`, `BUILD.md`, `.re-ledger/`).

## Invariant Rules

1. **Clean-Room Boundary**: Document observable external behavior, UI mechanics, schemas, and API contracts. Never decompile or lift proprietary source code directly.
2. **Two-Tier Privacy Gate**: All DOM captures and screenshots committed to knowledge bases must pass two-tier PII redaction (`mask.js`: physical text substitution + avatar/viewport blur).
3. **Four-Step Write Gate**: Before any state-mutating action (A2–A4) on a target app, verify Location, Audience, Undo path, and record in `sandbox/ledger.csv`.
4. **DAG Dependency Integrity**: Tasks declared in `.re-ledger/TASKS.json` must be strictly acyclic, reference verified evidence (`EV-###`), and have disjoint active write paths.
5. **Deterministic Falsifiability**: Every business rule in `rules/BR-###.md` must be phrased as `Given/When/Then` with citation tags `[obs EV-###]`.

## Execution Workflows

- [`run`](usecases/run.md): Operational procedure for running REM extractions, checks, reports, and rebuilds.

## Domain References

- [`spec`](references/spec.md): Technical specification, schemas, error boundaries, and recovery semantics.
- [`clean-room-governance`](references/clean-room-governance.md): Safety environment levels (L1–L4), action classes (A0–A5), write gates, and ledger protocol.
- [`extraction-scripts`](references/extraction-scripts.md): Browser extraction scripts (`inventory.js`, `tokens.js`, `mask.js`, `annotate.js`).
- [`kb-structure`](references/kb-structure.md): Knowledge base directory layout, business rules syntax, launch barriers, and handoff files.
- [`mcp-tools`](references/mcp-tools.md): Complete catalog of 16 MCP tools, inputs, outputs, and agent prompts.
- [`cli-drivers`](references/cli-drivers.md): CLI command reference, browser drivers, and automated MCP setup.

## Delegated Skills

- `poteto-principles`: Adhere to the 24 battle-tested engineering principles.

## Attribution & Provenance

- **Engineered by**: [REM Team](https://github.com/imMamdouhaboammar/REM)
- **Omni Skill Compatible**: Validated with [omni-skill](https://github.com/imMamdouhaboammar/omni-skill)
