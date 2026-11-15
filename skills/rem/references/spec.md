# REM Technical Specification & Domain Invariants

Technical schemas, invariants, and domain error definitions for REM.

## 1. Domain Invariants

1. **Clean-Room Boundary Invariant**: Reverse engineering documents observed behavior, UI mechanics, data schemas, and API contracts without decompiling or directly lifting proprietary source code.
2. **Deterministic Ledger Invariant**: Every mutation made during testing must be logged in `sandbox/ledger.csv`. A knowledge base cannot pass closing checks (`--closing`) while unreconciled or open ledger rows exist.
3. **DAG Dependency Invariant**: Tasks declared in `.re-ledger/TASKS.json` must form a directed acyclic graph. Any cycle, unknown dependency, or concurrent write path collision invalidates the build.
4. **Evidence Provenance Invariant**: Every business rule (`BR-###`) and claim in generated reports must cite a verified evidence reference (`EV-###` or `E#`).
5. **Privacy Gate Invariant**: Live browser screenshots stored in knowledge base assets must be subjected to two-tier DOM PII redaction (text masking + avatar blur) prior to disk commit.

---

## 2. Standard ID Grammars

| Prefix | Entity Type | Example | Location |
| :--- | :--- | :--- | :--- |
| `F-###` | Feature Specification | `F-001` | `features/F-###.md` |
| `BR-###` | Business Rule | `BR-001` | `rules/BR-###.md` |
| `FL-###` | State Machine Flow | `FL-001` | `flows/FL-###.md` |
| `ENT-###` | Data Model Entity | `ENT-001` | `data-model/ENT-###.md` |
| `EV-###` | Verified Evidence | `EV-001` | `EVIDENCE.md` or `re-*/` |
| `T-###` | DAG Reconstruction Task | `T-001` | `.re-ledger/TASKS.json` |

---

## 3. Error Codes & Recovery Semantics

| Error Code | Semantic Cause | Remediation Procedure |
| :--- | :--- | :--- |
| `ERR_LEDGER_CYCLE` | Circular task dependency in `.re-ledger/TASKS.json`. | Run `./rem ledger check`, inspect dependency graph, and break cycle. |
| `ERR_WRITE_PATH_CONFLICT` | Two active tasks (`ready`, `in_progress`) target overlapping paths. | Serialize tasks or partition paths into disjoint project directories. |
| `ERR_UNRECONCILED_LEDGER` | Open, fixture, or human rows remain during `--closing` check. | Reconcile or revert sandbox mutations and update `sandbox/ledger.csv`. |
| `ERR_DRIVER_NOT_FOUND` | Browser automation driver missing or Bun runtime absent. | Run `./rem setup` or install Bun and Playwright dependencies. |
