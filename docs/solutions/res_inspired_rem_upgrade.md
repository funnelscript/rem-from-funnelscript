# Solution: Upgrading REM with RES SaaS Reverse-Engineering & Clean-Room Reconstruction

## Problem & Context
REM previously provided a high-level text workflow for clean-room reverse engineering and a specialized marketing intelligence Go engine (`rem-from-funnelscript`). However, when reverse-engineering complex live web applications and SaaS products (like Linear, ClickUp, Notion, Pipedrive), REM lacked:
1. Standardized browser DOM and style extraction scripts.
2. A strict safety governance model for live account testing (environment levels, action classes, write gates, change ledgers).
3. A structured, evidence-anchored knowledge base format for documenting screens, components, Given/When/Then business rules, and data model entities.
4. Launch barrier and regulatory moat analysis (SOC 2, ISO 27001, partner API approvals, app store gatekeepers).
5. Automated generation of self-contained, interactive `REPORT.html` files.
6. Automated packaging of clean-room coding-agent handoff bundles (`BUILD.md` + sanitized zips).

## Solution & Architecture
Drawing direct inspiration from Alex Netto's **RES (Reverse Engineering SaaS)** (`alextnetto/res`) and auditing through the **Elephant-Goldfish Model (EGM)** and **Anti-Slop Engineering**, REM was upgraded into a unified reverse-engineering and software reconstruction platform.

### Key Components Built

1. **Embedded Go Knowledge Base Engine (`internal/kb`)**:
   - `Init`: Scaffolds a full SaaS knowledge base with `00-charter.md`, `STATUS.md`, `sources.md`, `glossary.md`, `findings.md`, `questions.md`, `inventory/`, `clone/`, `sandbox/`, and `_templates/`.
   - `Check`: Validates ID uniqueness across 12 namespaces (`FL`, `SRC`, `EV`, `BR`, `F`, `S`, `C`, `R`, `ENT`, `B`, `Q`, `L`), verifies evidence tag citations (`[obs]`, `[net]`, `[doc]`, `[inf]`), and audits `sandbox/ledger.csv`.
   - `Report`: Compiles a standalone, single-file, CSP-hardened interactive `REPORT.html` with tabbed navigation, filtering, and embedded JSON data.
   - `Pack`: Packages the knowledge base into a coding-agent handoff bundle, generating `BUILD.md` and sanitizing internal sandbox/log details.
   - `Barriers`: Categorizes and scores launch obstacles by severity (`blocks launch`, `blocks feature`, `adds lead time`, `adds cost`, `moat`), market weight, and lead times.

2. **Browser Extraction Automation Scripts (`tools/page/` & `internal/kb/assets/scripts/`)**:
   - `inventory.js`: DOM extraction of frameworks (React, Vue, Angular, Svelte, Next, Nuxt), custom tags, test-IDs, ARIA roles, normalized routes, and 20+ analytics/auth SDK globals.
   - `tokens.js`: Design token extractor scanning stylesheets for `--custom-properties`, computed text styles, hex/OKLCH color frequencies, font sizes/weights, border radii, shadows, and spacing.
   - `mask.js`: Two-tier physical PII redaction (physically substituting DOM text nodes, input values, and attributes with synthetic placeholders before applying visual CSS blur) ensuring zero data leakage in screenshots or automated text scraping.
   - `annotate.js`: Visual DOM element boundary highlighting and numbered badges for generating UI anatomy figures.

3. **Unified CLI & Model Context Protocol (MCP) Server**:
   - The Go CLI exposes `rem kb` subcommands: `init`, `check`, `script`, `report`, `pack`, `barriers`.
   - The Go MCP server exposes native tools: `kb_init`, `kb_check`, `kb_get_script`, `kb_build_report`, allowing AI agents across Antigravity, Claude Code, Codex, and Cursor to execute the workflow natively.

4. **Taxonomic Disambiguation**:
   - Disambiguated Data Model Entities (`ENT-###`) from Evidence records (`EV-###` / legacy `E-###`), eliminating collision issues.

5. **Python Pre-Commit Validator Harmonization (`tools/rem.py`)**:
   - Preserved as a lean, zero-dependency pre-commit validator for task DAGs and file references with updated regex matching `(?:EV|E)-[0-9]+` and optional `--kb` delegation to `rem kb check`.

## Verification & Testing
- Full Go test suite passes (`go test ./...` across all 11 packages).
- Unit tests verify `Init`, `Check`, `Script`, `Report`, `Pack`, `ParseBarriers`, and CLI/MCP command routing.
- Python ledger validator passes with zero errors on `.re-ledger/` and `/tmp/test-kb`.
