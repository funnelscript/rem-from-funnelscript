# Operational Procedure for Running REM

Step-by-step execution procedure for reverse engineering, clean-room software reconstruction, and marketing research using REM.

## 1. Prerequisites & Environment Check

Before initiating any investigation or extraction:

```sh
# 1. Inspect engine diagnostics, AI credentials, and browser drivers
./rem doctor
```

Verify that:
- AI writing engine (Anthropic or local evidence fallback) is configured.
- Browser automation drivers (`bun`, `node`, `playwright`) report `true`.

---

## 2. SaaS Behavioral Reverse Engineering Workflow

Follow these 6 stages when studying an existing product:

### Stage 1: Initialize Knowledge Base
Scaffold a clean-room directory for the target product:
```sh
./rem kb init "Linear" --dir re-linear
```

### Stage 2: Automated Browser Extraction
Run the automated headless driver to capture DOM inventory, design tokens, PII masking, and annotations:
```sh
./rem extract https://linear.app --dir re-linear
```
Or via MCP: call `kb_extract` with `url` and `dir`.

### Stage 3: Document Business Rules & Entities
- Formalize business logic in `re-linear/rules/BR-###.md` using Given/When/Then.
- Map domain models in `re-linear/data-model/ENT-###.md`.
- Capture launch barriers in `re-linear/barriers/barriers.md`.

### Stage 4: Validate Knowledge Base Integrity
Run the strict consistency audit:
```sh
./rem kb check re-linear
```

### Stage 5: Compile Standalone Interactive Report
Build the self-contained HTML report:
```sh
./rem kb report re-linear --out re-linear/REPORT.html
```

### Stage 6: Pack Downstream Coding Agent Handoff
Bundle the clean-room specification into a ZIP archive for coding agents:
```sh
./rem kb pack re-linear --out linear-handoff.zip
```

---

## 3. Clean-Room Reconstruction & DAG Ledger Verification

When executing an independent rebuild:

1. Maintain task DAG in `.re-ledger/TASKS.json`.
2. Validate task dependencies and disjoint write paths before touching code:
   ```sh
   ./rem ledger check
   ```
3. Record all test sandbox mutations in `sandbox/ledger.csv`.
4. Prior to release, ensure closing check passes:
   ```sh
   ./rem kb check re-linear --closing
   ```
