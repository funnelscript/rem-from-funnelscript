package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"github.com/funnelscript/rem-from-funnelscript/internal/report"
	"github.com/funnelscript/rem-from-funnelscript/internal/store"
)

func TestStdioRunsAgainstSavedEvidence(t *testing.T) {
	runs := store.New(t.TempDir())
	fixture := fixtureBundle("fixture")
	fixture.GeneratedAt = "2026-11-13T00:00:00Z"
	if _, err := runs.Save(fixture, report.Markdown(fixture)); err != nil {
		t.Fatal(err)
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_evidence","arguments":{"run_id":"fixture","ids":["E2"],"include_unknowns":true},"_meta":{"progressToken":"external-client"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_report","arguments":{"run_id":"fixture","engine":"evidence"}}}
`
	var output bytes.Buffer
	if err := Serve(context.Background(), strings.NewReader(input), &output, app.New(runs)); err != nil {
		t.Fatal(err)
	}
	responses := map[string]recordedResponse{}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var response recordedResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatal(err)
		}
		responses[string(response.ID)] = response
	}
	decodeResult := func(id string) string {
		t.Helper()
		response := responses[id]
		var result toolResult
		if response.Error != nil || json.Unmarshal(response.Result, &result) != nil || result.IsError || len(result.Content) != 1 {
			t.Fatalf("request %s got invalid tool result: %s, %#v", id, response.Result, response.Error)
		}
		return result.Content[0].Text
	}
	var retrieved struct {
		Evidence []model.Evidence
		Unknowns []model.Unknown
	}
	if err := json.Unmarshal([]byte(decodeResult("2")), &retrieved); err != nil || len(retrieved.Evidence) != 1 || retrieved.Evidence[0].ID != "E2" || len(retrieved.Unknowns) != 1 {
		t.Fatalf("request did not return saved evidence and unknowns: %#v, %v", retrieved, err)
	}
	if md := decodeResult("3"); !report.Validate(md, fixture).OK || !strings.Contains(md, "[E2]") {
		t.Fatalf("invalid report from stored run: %s", md)
	}
}
