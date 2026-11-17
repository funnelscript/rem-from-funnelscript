# Clean-Room Safety & Environment Governance

REM enforces a formal clean-room reverse engineering and software reconstruction doctrine. All interactions with target applications and downstream rebuilds must operate under explicit safety boundaries.

## 1. Safety Environment Levels (L1–L4)

| Level | Name | Environment Definition | Permitted Actions |
| :--- | :--- | :--- | :--- |
| **L1** | **Dedicated Sandbox** | Isolated trial or test account in a dedicated browser profile or virtual environment. | All safe writes (A0–A4). Destructive account operations permitted only within the sandbox. |
| **L2** | **Dedicated Workspace** | Dedicated sub-workspace or organization under an existing account. | A0–A3 freely. A4 permitted only for workspace-scoped settings (never account-wide). |
| **L3** | **Isolated Container** | Designated sandbox container or folder (e.g., `🧪 [RE] Sandbox`) within a live workspace. | A0–A1 freely. A2 permitted strictly inside the designated container. A3–A4 prohibited. |
| **L4** | **Observe Only** | Production observation on live enterprise accounts without write permissions. | A0 only (read-only navigation, DOM inspection, redacted screenshots). |

---

## 2. Action Classes (A0–A5)

| Class | Classification | Description | Example |
| :--- | :--- | :--- | :--- |
| **A0** | **Read / Observe** | Passive inspection, DOM reading, network traffic listening, read-only navigation. | Inspecting DOM elements, viewing public pricing, taking screenshots. |
| **A1** | **Client State** | Ephemeral browser UI changes that do not persist to the remote server. | Expanding dropdowns, toggling local tabs, filtering local tables. |
| **A2** | **Reversible Mutation** | Server-persisted change that can be cleanly deleted or restored. | Creating a test issue `[TEST] Demo`, editing an issue title. |
| **A3** | **Irreversible Mutation** | Server-persisted change that cannot be undone or leaves permanent audit trails. | Inviting real external emails, triggering webhooks, billable actions. |
| **A4** | **Admin / Workspace** | Structural workspace modifications or global configuration toggles. | Changing SSO settings, modifying billing tier, deleting workspaces. |
| **A5** | **Direct Ingestion** | Decompiling proprietary binaries or scraping protected internal databases. | Strictly gated by clean-room protocol. Requires legal sign-off. |

---

## 3. The 4-Step Write Gate Protocol

Before any write action (A2, A3, A4) is executed against a target SaaS system:

1. **Location Verification**: Breadcrumbs and URL match the designated sandbox container specified in `sandbox/environment.md`.
2. **Audience Fencing**: Notifications, assignments, and invites reach only the designated operator account. No third-party or real user is alerted.
3. **Reversal Path**: An explicit undo or delete procedure is documented before performing the action.
4. **Ledger Recording**: The mutation is immediately recorded in `sandbox/ledger.csv`.

---

## 4. The Clean-Room DAG Ledger (`.re-ledger/`)

During independent software reconstruction:
- Tasks are declared in `.re-ledger/TASKS.json` with explicit dependencies (`depends_on`).
- Evidence references (`evidence_ids`) must resolve to verified rows in `EVIDENCE.md`.
- Concurrent active tasks (`ready`, `in_progress`) must declare disjoint `write_paths`.
- Tasks marked `done` must include a concrete `verification` reference (e.g., PR, test run).
- Verify DAG integrity anytime using `./rem ledger check` or MCP tool `ledger_check`.
