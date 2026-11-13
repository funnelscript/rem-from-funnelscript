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
