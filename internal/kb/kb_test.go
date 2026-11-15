package kb

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKBLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	kbPath := filepath.Join(tempDir, "re-testproduct")

	// 1. Test Init
	dir, err := Init(kbPath, "Test Product", false)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if dir != kbPath {
		t.Errorf("Expected dir %s, got %s", kbPath, dir)
	}

	// Verify required scaffold files exist
	for _, req := range []string{"00-charter.md", "STATUS.md", "sandbox/environment.md", "sandbox/ledger.csv", "BUILD.md"} {
		if _, err := os.Stat(filepath.Join(kbPath, req)); os.IsNotExist(err) {
			t.Errorf("Expected file %s missing after Init", req)
		}
	}

	// 2. Test Check
	res, err := Check(kbPath, false)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !res.Valid {
		t.Errorf("Expected valid KB, got errors: %v", res.Errors)
	}

	// 3. Test Script
	for _, scriptName := range []string{"inventory", "tokens", "mask", "annotate"} {
		s, err := Script(scriptName)
		if err != nil {
			t.Errorf("Script %q failed: %v", scriptName, err)
		}
		if !strings.Contains(s, "window.__rem") {
			t.Errorf("Script %q missing window.__rem namespace", scriptName)
		}
	}

	// Test unknown script error
	if _, err := Script("unknown"); err == nil {
		t.Error("Expected error for unknown script, got nil")
	}

	// 4. Test Report
	reportHTML, err := Report(kbPath)
	if err != nil {
		t.Fatalf("Report failed: %v", err)
	}
	if !strings.Contains(reportHTML, "<title>Test Product · Reverse Engineering Knowledge Base Report</title>") {
		t.Errorf("Report HTML missing expected title")
	}
	if !strings.Contains(reportHTML, "Content-Security-Policy") {
		t.Errorf("Report HTML missing Content-Security-Policy")
	}

	// 5. Test Pack
	zipOut := filepath.Join(tempDir, "handoff.zip")
	out, err := Pack(kbPath, zipOut, false)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	if out != zipOut {
		t.Errorf("Expected out %s, got %s", zipOut, out)
	}

	// Inspect zip content
	zr, err := zip.OpenReader(zipOut)
	if err != nil {
		t.Fatalf("Failed reading zip archive: %v", err)
	}
	defer zr.Close()

	foundBuild := false
	foundReport := false
	for _, f := range zr.File {
		if f.Name == "BUILD.md" {
			foundBuild = true
		}
		if f.Name == "REPORT.html" {
			foundReport = true
		}
	}
	if !foundBuild {
		t.Error("Pack zip missing BUILD.md")
	}
	if !foundReport {
		t.Error("Pack zip missing REPORT.html")
	}
}

func TestCheckClosingFailure(t *testing.T) {
	tempDir := t.TempDir()
	kbPath := filepath.Join(tempDir, "re-testclosing")
	_, _ = Init(kbPath, "Closing Product", false)

	// Append an open ledger item
	ledgerPath := filepath.Join(kbPath, "sandbox/ledger.csv")
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("Failed opening ledger: %v", err)
	}
	_, _ = f.WriteString("L-001,2026-11-16T00:00:00Z,A2,create,deal,pipeline/1,deal_1,,delete,open,Active test row\n")
	_ = f.Close()

	// Normal check passes
	res, err := Check(kbPath, false)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !res.Valid {
		t.Errorf("Expected normal check to pass, got errors: %v", res.Errors)
	}
	if res.Stats["openLedgerRows"] != 1 {
		t.Errorf("Expected 1 open row, got %d", res.Stats["openLedgerRows"])
	}

	// Closing check must fail on open row
	resClose, err := Check(kbPath, true)
	if err != nil {
		t.Fatalf("Check closing failed: %v", err)
	}
	if resClose.Valid {
		t.Errorf("Expected closing check to fail on open row, but it passed")
	}
}
