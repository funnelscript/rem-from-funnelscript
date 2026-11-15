# REM MCP Tools & Prompts Catalog

The REM MCP server exposes 16 tools and 8 prompts over stdio JSON-RPC for AI coding agents.

## 1. Tool Catalog

| Tool Name | Scope | Description | Primary Parameters |
| :--- | :--- | :--- | :--- |
| `reverse_engineer_marketing` | Intelligence | Collect marketing and GTM evidence for a domain or app store URL. | `url`, `only`, `stealth`, `keyless`, `window` |
| `get_evidence` | Intelligence | Retrieve full evidence records and recorded unknowns from a run. | `run_id`, `ids`, `source`, `include_unknowns` |
| `get_content` | Intelligence | Read collected ads, social posts, videos, and threads. | `run_id`, `platform`, `sort`, `limit` |
| `get_playbook` | Analysis | Read the analysis methodology and channel guide. | `name` |
| `validate_report` | Quality Gate | Check a draft report for evidence grounding and citation errors. | `run_id`, `markdown` |
| `write_report` | Reporting | Render a saved run as a structured Markdown report. | `run_id`, `model`, `engine` |
| `compare_runs` | Intelligence | Compare two temporal snapshots or latest runs for a domain. | `domain`, `run_a`, `run_b` |
| `kb_init` | SaaS RE | Scaffold a new clean-room reverse-engineering knowledge base. | `product`, `dir` |
| `kb_check` | Quality Gate | Validate KB IDs, evidence tags, and sandbox ledger. | `dir`, `closing` |
| `kb_get_script` | Browser Driver | Retrieve extraction JavaScript (`inventory`, `tokens`, `mask`, `annotate`). | `name` |
| `kb_build_report` | Reporting | Compile KB into a self-contained, interactive `REPORT.html`. | `dir`, `out` |
| `kb_pack` | Handoff | Pack KB into an agent-ready handoff ZIP archive. | `dir`, `out`, `full` |
| `kb_barriers` | SaaS RE | Structured audit of launch barriers and compliance moats. | `dir` |
| `kb_extract` | Browser Driver | Automated browser extraction driver (runs DOM analysis and screenshots). | `url`, `dir`, `headless`, `mask_pii`, `annotate` |
| `ledger_check` | Governance | Validate `.re-ledger/` DAG tasks, dependencies, and path conflicts. | `dir` |
| `doctor` | Diagnostics | System diagnostics, engines, credentials, and drivers. | None |

---

## 2. Prompts Catalog

1. `reverse_engineer_saas`: Guided workflow for clean-room SaaS reverse engineering (initialization, extraction, business rules, launch barriers, interactive report).
2. `audit_launch_barriers`: Evaluation of regulatory, security, data residency, and enterprise barriers from `clone/barriers.md`.
3. `clean_room_reconstruct`: Rebuilding documented software independently under strict L1–L4 governance and DAG ledgers.
4. `investigate_company`: End-to-end GTM and public marketing acquisition analysis with cited evidence.
5. `compare_competitors`: Multi-competitor marketing comparison across channels, positioning, and ad creative.
6. `what_changed`: Temporal diff analysis tracking positioning, pricing, or ad changes between runs.
7. `steal_the_playbook`: Adapting observed competitor acquisition tactics into a 30-day execution plan.
8. `audit_unknowns`: Diagnostic review of evidence gaps and recommended manual check links.
