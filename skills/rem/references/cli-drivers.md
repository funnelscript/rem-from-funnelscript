# REM CLI & Driver Reference

The REM CLI provides a unified command line interface for marketing intelligence, browser extraction drivers, and clean-room SaaS reverse engineering.

## 1. Quick Commands Overview

```sh
# Display CLI commands and options
./rem --help

# Run system diagnostic checks
./rem doctor

# Validate DAG tasks in .re-ledger/
./rem ledger check

# Automatically extract live web app into a knowledge base
./rem extract https://linear.app --dir re-linear

# Initialize a clean-room knowledge base
./rem kb init "Linear" --dir re-linear

# Validate KB structure, evidence tags, and sandbox ledger
./rem kb check re-linear
./rem kb check re-linear --closing

# Compile interactive self-contained HTML report
./rem kb report re-linear --out re-linear/REPORT.html

# Pack handoff bundle for downstream coding agents
./rem kb pack re-linear --out linear-handoff.zip

# Audit launch barriers and regulatory moats
./rem kb barriers re-linear
```

---

## 2. Browser Driver Commands (`tools/driver/extract.ts`)

The automated browser extraction driver uses Bun and Playwright:

```sh
# Run driver directly
bun run tools/driver/extract.ts <url> [options]

# Options:
#   --dir <path>         Output directory (defaults to re-<domain>)
#   --headless <bool>    Run browser headlessly (default: true)
#   --mask <bool>        Apply two-tier DOM PII redaction (default: true)
#   --annotate <bool>    Overlay numbered visual UI annotations (default: true)
#   --timeout <ms>       Navigation timeout in ms (default: 30000)
#   --viewport <w>x<h>   Viewport size (default: 1440x900)
#   --delay <ms>         Post-load stabilization delay (default: 1500)
```

---

## 3. Automated Client Setup (`rem setup`)

Register REM MCP server automatically into installed AI clients:

```sh
# Preview registration plan
./rem setup --dry-run

# Register into Claude Code and Codex
./rem setup --yes

# Specify target clients
./rem setup --yes --client claude_code,codex
```
