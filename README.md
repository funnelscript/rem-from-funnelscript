# REM | Unified Reverse Engineering & Software Reconstruction System

Use **REM** to document how an existing product behaves, rebuild the agreed behavior independently, or develop a new product from a brief. The system unifies:
1. **Automated Headless Browser Extraction Driver**: Playwright and CDP automation running two-tier DOM PII masking, OKLCH design tokens, tech stack inventory, and numbered visual UI annotations with a single command.
2. **SaaS Behavioral Reverse Engineering & Knowledge Base Engine** (inspired by Alex Netto's `alextnetto/res`): Live browser extraction, privacy-preserving two-tier PII masking, token extraction, Given/When/Then business rules, launch barrier matrices, interactive `REPORT.html`, and coding-agent handoffs (`BUILD.md`).
3. **Clean-Room Software Reconstruction Workflow**: Strict safety environment levels (L1–L4), action classes (A0–A5), write gates, DAG task ledgers (`.re-ledger/`), and cryptographic evidence anchoring.
4. **Public Marketing & Acquisition Intelligence** (`rem-from-funnelscript` Go engine): Stealth anti-bot HTTP/2 fingerprinting, zero-cost public endpoints (Reddit, HN, GitHub), career hiring signals, and deterministic channel scorecards.
5. **Native MCP Server (16 Tools & 8 Prompts)**: Exposes marketing research, automated browser extraction, DAG ledger checks, and SaaS reverse engineering tools directly to AI coding agents (Antigravity, Claude Code, Codex, Cursor).
6. **Official REM Agent Skill (`skills/rem/`)**: Tripartite specification (`SKILL.md`, `references/`, `usecases/`, `evals/`) validated with `@mamdouh-aboammar/omni-skill`.

---

## 1. One-Command Installation

Install REM, its browser automation drivers, agent skills, and MCP configuration with one command:

```sh
# Local installer
./install.sh

# Or via curl from GitHub
curl -fsSL https://raw.githubusercontent.com/funnelscript/rem-from-funnelscript/main/install.sh | bash
```

Alternatively, try REM ad-hoc without global installation:

```sh
# Using Bun
bunx @mamdouh-aboammar/rem doctor

# Using Node.js (npx)
npx @mamdouh-aboammar/rem doctor
```

---

## 2. Automated Browser Extraction & SaaS Knowledge Base Workflow

Explore any live web application or SaaS product safely and package its complete UX, data model, business rules, and build plan into a structured knowledge base on disk:

```sh
# 1. Initialize a new clean-room Knowledge Base (creates re-<product>/)
rem kb init "Linear" --dir re-linear

# 2. Automatically extract DOM inventory, tokens, PII masking & visual annotations
rem extract https://linear.app --dir re-linear

# 3. Validate KB structure, evidence tags, and sandbox ledger
rem kb check re-linear
rem kb check re-linear --closing  # Fails if any open or un-reconciled ledger rows remain

# 4. Compile self-contained interactive HTML report
rem kb report re-linear --out re-linear/REPORT.html

# 5. Pack handoff bundle for downstream coding agents
rem kb pack re-linear --out linear-handoff.zip

# 6. Audit launch barriers and regulatory moats
rem kb barriers re-linear
```

### Safety & Clean-Room Governance

| Environment Level | Definition | Permissions |
| :--- | :--- | :--- |
| **L1 Dedicated** | Isolated trial/sandbox account in a separate profile | All safe sandbox writes (A0–A4) |
| **L2 Workspace** | Dedicated sub-workspace under user account | A0–A3, A4 (workspace creation only) |
| **L3 Container** | Designated sandbox container (e.g. `🧪 [RE] Sandbox`) | A0–A1, A2 (inside container only) |
| **L4 Observe Only** | Production observation, no writes | A0 (read-only navigation & screenshots) |

**The Write Gate:** Before every write action (A2, A3, A4):
1. **Location:** Current breadcrumbs match `sandbox/environment.md`.
2. **Audience:** Only the operator is assigned or notified. Real users are never reached.
3. **Undo:** Explicit reversal path is verified.
4. **Ledger:** Mutation is logged in `sandbox/ledger.csv`.

---

## 3. DAG Task Ledger Validator (`.re-ledger/`)

REM validates `.re-ledger/TASKS.json` DAG dependencies, cycles, verification proofs, and concurrent write-path conflicts natively:

```sh
# Validate .re-ledger task DAG using the compiled Go engine
rem ledger check

# Or using the Python pre-commit helper
python3 tools/rem.py check
```

---

## 4. Public Marketing & GTM Intelligence

Investigate how any company acquires customers, runs ads, and configures tracking:

```sh
# Investigate domain with stealth anti-bot headers and zero-cost public providers
rem linear.app --keyless --stealth --window 30d

# Compare two temporal snapshots
rem diff linear.app

# Run local web dashboard
rem web --port 4321
```

---

## 5. Model Context Protocol (MCP) Server

Connect REM to your AI client (`claude_code`, `codex`, `antigravity`, `cursor`) via stdio:

### Automatic Registration
```sh
rem setup --yes
```

### Manual Configuration
```json
{
  "mcpServers": {
    "rem": {
      "command": "rem",
      "args": ["mcp"]
    }
  }
}
```

### Catalog of 16 MCP Tools

| Tool | Category | Description |
| :--- | :--- | :--- |
| `kb_init` | SaaS RE | Scaffold a clean-room reverse-engineering knowledge base. |
| `kb_extract` | Browser Driver | Automated headless extraction of DOM inventory, tokens, PII masking & annotations. |
| `kb_check` | Quality Gate | Validate knowledge base IDs, evidence tags, and sandbox ledger. |
| `kb_get_script` | Extraction | Retrieve browser extraction scripts (`inventory`, `tokens`, `mask`, `annotate`). |
| `kb_build_report` | Reporting | Compile self-contained interactive `REPORT.html`. |
| `kb_pack` | Handoff | Pack knowledge base into an agent-ready handoff ZIP archive. |
| `kb_barriers` | SaaS RE | Structured audit of launch barriers and compliance moats. |
| `ledger_check` | Governance | Validate `.re-ledger/` DAG tasks, cycle detection, and write path overlaps. |
| `doctor` | Diagnostics | Inspect intelligence sources, engines, credentials, and drivers. |
| `reverse_engineer_marketing` | Intelligence | Run stealth public marketing and GTM intelligence. |
| `get_evidence` | Intelligence | Retrieve cited marketing evidence records and unknowns. |
| `get_content` | Intelligence | Read collected ads, social posts, videos, and threads. |
| `get_playbook` | Analysis | Read analysis playbooks and channel guides. |
| `write_report` | Reporting | Render a saved run as a structured Markdown report. |
| `compare_runs` | Intelligence | Compare temporal changes across two runs. |
| `validate_report` | Quality Gate | Verify report claims against collected evidence. |

---

## 6. Official Agent Skill (`skills/rem/`)

The official REM Skill enables coding agents to autonomously run clean-room reverse engineering and software reconstruction following the [AgentSkills.io](https://skills.sh) open standard:

```sh
# Validate skill across 7 static analysis gates using omni-skill
bunx @mamdouh-aboammar/omni-skill validate skills/rem
```
