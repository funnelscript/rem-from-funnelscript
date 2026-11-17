# Changelog

## 0.2.0

- Expanded native MCP server to 16 tools and 8 prompts (`kb_init`, `kb_extract`, `kb_check`, `kb_get_script`, `kb_build_report`, `kb_pack`, `kb_barriers`, `ledger_check`, `doctor`, and reverse engineering prompts).
- Added automated headless browser extraction driver (`tools/driver/extract.ts`) powered by Playwright with stealth user-agent emulation, OKLCH design tokens, two-tier DOM PII masking, and numbered visual annotations.
- Implemented SaaS Behavioral Reverse Engineering Knowledge Base engine (`rem kb`) with automated scaffolding, structural validation, interactive `REPORT.html` compilation, and agent-ready handoff ZIP packaging.
- Introduced Clean-Room Software Reconstruction Workflow & Safety Framework (L1–L4 environment governance, A0–A5 action classes, and strict write gates).
- Added official REM Agent Skill (`skills/rem/`) compliant with AgentSkills.io and validated with `@mamdouh-aboammar/omni-skill`.
- Added native DAG task dependency, cycle detection, and write path conflict validator (`.re-ledger/TASKS.json`).
- Hardened cross-platform compatibility across Windows, Linux, and macOS (path separators, JSON escaping, and CI workflow optimizations).
- Added universal one-command installer script (`install.sh`) and executable launcher (`bin/rem.js`).

## 0.1.0

- First public release of the Go implementation of Reverse Engineer Marketing.
- Added a CLI, a loopback browser app and an MCP stdio server with shared run history.
- Added public evidence collection for websites, SEO, tracking, app stores, launch activity and positioning history, plus optional Scrape Creators and strategy writers.
- Added Markdown and JSON exports, citation validation and snapshot comparisons.
- Added five platform builds and checksums. Live verification and provider coverage are described in the README and verification ledger.
