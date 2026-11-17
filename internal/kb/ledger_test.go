package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLedger(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rem-ledger-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Incomplete ledger should fail with missing files
	res, err := CheckLedger(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid || len(res.Errors) != 5 {
		t.Fatalf("expected 5 missing file errors, got %d: %v", len(res.Errors), res.Errors)
	}

	// Create valid ledger files
	_ = os.WriteFile(filepath.Join(tempDir, "STATUS.md"), []byte("# Status\n"), 0o644)
	evidenceContent := `# Evidence
| EV-001 | Linear app UI | https://linear.app |
| EV-002 | GraphQL API | https://api.linear.app |
`
	_ = os.WriteFile(filepath.Join(tempDir, "EVIDENCE.md"), []byte(evidenceContent), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "DECISIONS.md"), []byte("# Decisions\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "PARITY.md"), []byte("# Parity\n"), 0o644)

	validTasks := `[
  {
    "id": "T-001",
    "owner": "engineer",
    "outcome": "Scaffold database",
    "status": "done",
    "depends_on": [],
    "evidence_ids": ["EV-001"],
    "checks": ["go test ./..."],
    "write_paths": ["pkg/db"],
    "verification": "PR #12"
  },
  {
    "id": "T-002",
    "owner": "engineer",
    "outcome": "Implement API",
    "status": "ready",
    "depends_on": ["T-001"],
    "evidence_ids": ["EV-002"],
    "checks": ["go test ./..."],
    "write_paths": ["pkg/api"]
  }
]`
	_ = os.WriteFile(filepath.Join(tempDir, "TASKS.json"), []byte(validTasks), 0o644)

	res, err = CheckLedger(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid || len(res.Errors) > 0 {
		t.Fatalf("expected valid ledger, got errors: %v", res.Errors)
	}
	if res.TaskCount != 2 {
		t.Fatalf("expected 2 tasks, got %d", res.TaskCount)
	}

	// Test cycle detection
	cyclicTasks := `[
  {
    "id": "T-A",
    "owner": "engineer",
    "outcome": "Task A",
    "status": "pending",
    "depends_on": ["T-B"],
    "evidence_ids": ["EV-001"],
    "checks": ["go test ./..."],
    "write_paths": ["pkg/a"]
  },
  {
    "id": "T-B",
    "owner": "engineer",
    "outcome": "Task B",
    "status": "pending",
    "depends_on": ["T-A"],
    "evidence_ids": ["EV-001"],
    "checks": ["go test ./..."],
    "write_paths": ["pkg/b"]
  }
]`
	_ = os.WriteFile(filepath.Join(tempDir, "TASKS.json"), []byte(cyclicTasks), 0o644)
	res, err = CheckLedger(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected cycle error, but ledger marked valid")
	}
}
