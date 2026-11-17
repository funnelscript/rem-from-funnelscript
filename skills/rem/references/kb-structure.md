# Reverse Engineering Knowledge Base Architecture

A REM Knowledge Base (`re-<product>/`) structures behavioral observations, business rules, launch barriers, and coding handoffs into a unified, version-controlled repository on disk.

## Directory Anatomy

```
re-<product>/
├── README.md              # Project summary, target URL, and extraction metadata
├── REPORT.html            # Standalone interactive HTML report compiled by REM
├── inventory/
│   ├── inventory.json     # Output from inventory.js (frameworks, custom tags, test-IDs)
│   └── frameworks.json    # Summary of detected client and server libraries
├── assets/
│   ├── tokens.json        # Output from tokens.js (OKLCH colors, fonts, radii)
│   ├── screenshot-clean.png      # PII-redacted screenshot from mask.js
│   ├── screenshot-annotated.png  # Visual numbered overlay screenshot from annotate.js
│   └── annotations.json          # Index of numbered interactive elements
├── features/
│   ├── F-001-navigation.md       # Primary navigation, sidebar, and breadcrumbs
│   └── F-002-issue-board.md      # Core kanban/list workflows
├── rules/
│   ├── BR-001-sorting.md         # Given/When/Then business rules
│   └── BR-002-permissions.md     # Role-based access control and state transitions
├── flows/
│   └── FL-001-create-issue.md    # Multi-step state machine flows
├── data-model/
│   ├── ENT-001-workspace.md     # Entity definitions, schemas, and relations
│   └── ENT-002-issue.md         # Field attributes, validation, and foreign keys
├── barriers/
│   └── barriers.md              # Launch barriers, compliance moats, lead times
├── handoff/
│   └── BUILD.md                 # Coding-agent instructions for independent rebuild
└── sandbox/
    ├── environment.md           # L1-L4 environment definition and test account details
    └── ledger.csv               # Mandatory write gate audit log
```

---

## Technical File Formats

### 1. Business Rules (`rules/BR-###.md`)
Every rule is formalized using Given/When/Then syntax:

```markdown
# BR-001: Automatic Task Status Progression

- **ID**: BR-001
- **Feature**: [F-002] Issue Board
- **Evidence**: [obs EV-004] Live drag-and-drop observation

## Specification
- **Given** an issue in status `In Progress`
- **When** the operator assigns the issue to a completed pull request
- **Then** the issue status transitions automatically to `Done`
- **And** an activity audit entry is recorded in the issue timeline
```

### 2. Launch Barriers (`barriers/barriers.md`)
Barriers identify non-functional requirements, compliance moats, and infrastructure prerequisites:

```markdown
### BR-001: SOC 2 Type II Compliance & Audit Logs
- Category: Security & Compliance
- Severity: High
- Market weight: Required for enterprise sales
- Lead time: 6 months
- Cost: $25,000 - $40,000
- Mitigation: Adopt pre-certified cloud infrastructure and open-source audit log library
- Evidence: Enterprise pricing tier mandates SOC 2 report
```

### 3. Coding Agent Handoff (`handoff/BUILD.md`)
Synthesizes the entire knowledge base into an actionable instruction brief for downstream coding agents (Antigravity, Claude Code, Cursor, Codex).
