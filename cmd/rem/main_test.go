package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestVersionAndSetupPlan(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"setup", "--dry-run", "--json"}, {"doctor", "--json"}} {
		var out, errOut bytes.Buffer
		if err := run(context.Background(), args, &out, &errOut); err != nil {
			t.Fatal(args, err)
		}
		if out.Len() == 0 || len(args) > 1 && !json.Valid(out.Bytes()) {
			t.Fatal(args, out.String())
		}
	}
}

func TestOptionsBeforeOrAfterTarget(t *testing.T) {
	for _, args := range [][]string{{"--only=website", "example.org", "--no-ai"}, {"example.org", "--only", "website", "--no-ai"}} {
		f, rest, err := parse(args)
		if err != nil || len(rest) != 1 || rest[0] != "example.org" || f.only != "website" || !f.noAI {
			t.Fatal(f, rest, err)
		}
	}
}

func TestNewFlagsParsed(t *testing.T) {
	args := []string{"example.org", "--stealth", "--keyless", "--window=90d"}
	f, rest, err := parse(args)
	if err != nil || len(rest) != 1 || rest[0] != "example.org" || !f.stealth || !f.keyless || f.window != "90d" {
		t.Fatalf("unexpected flags: %+v rest=%v err=%v", f, rest, err)
	}
}

func TestKBCLICommands(t *testing.T) {
	tempDir := t.TempDir()
	var out, errOut bytes.Buffer

	// 1. rem kb init
	if err := run(context.Background(), []string{"kb", "init", "Acme", "--dir=" + tempDir}, &out, &errOut); err != nil {
		t.Fatalf("kb init failed: %v, errOut: %s", err, errOut.String())
	}

	// 2. rem kb check --json
	out.Reset()
	errOut.Reset()
	if err := run(context.Background(), []string{"kb", "check", tempDir, "--json"}, &out, &errOut); err != nil {
		t.Fatalf("kb check failed: %v, errOut: %s", err, errOut.String())
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("kb check did not output valid JSON: %s", out.String())
	}

	// 3. rem kb script
	out.Reset()
	errOut.Reset()
	if err := run(context.Background(), []string{"kb", "script", "inventory"}, &out, &errOut); err != nil {
		t.Fatalf("kb script failed: %v, errOut: %s", err, errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("kb script returned empty output")
	}

	// 4. rem kb report
	out.Reset()
	errOut.Reset()
	if err := run(context.Background(), []string{"kb", "report", tempDir}, &out, &errOut); err != nil {
		t.Fatalf("kb report failed: %v, errOut: %s", err, errOut.String())
	}

	// 5. rem kb pack
	out.Reset()
	errOut.Reset()
	if err := run(context.Background(), []string{"kb", "pack", tempDir}, &out, &errOut); err != nil {
		t.Fatalf("kb pack failed: %v, errOut: %s", err, errOut.String())
	}
}
