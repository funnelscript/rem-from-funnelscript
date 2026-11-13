package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

func TestInvestigationSummaryPreservesBudgetAndEvidenceProvenance(t *testing.T) {
	service := &fakeMarketing{}
	server := marketing{service: service}
	result := toolCall(t, server, "reverse_engineer_marketing", `{"url":"example.com","only":["seo"],"handles":{"x":"example"},"max_credits":0}`)
	if result.IsError || !service.opts.BudgetSet || service.opts.MaxCredits != 0 || service.opts.Engine != "evidence" || service.opts.Handles["x"] != "example" || service.opts.Only[0] != "seo" {
		t.Fatalf("tool did not preserve input and options: %#v, %#v", result, service.opts)
	}
	for _, part := range []string{"run_id: first", "[E1]", "https://example.com", "No key", "get_evidence"} {
		if !strings.Contains(result.Content[0].Text, part) {
			t.Errorf("expected %q in %s", part, result.Content[0].Text)
		}
	}
	if strings.Contains(result.Content[0].Text, "raw evidence body") {
		t.Fatalf("investigation result should only contain the evidence index: %#v", result)
	}
}

func TestGetEvidenceAndContentFilterSavedRun(t *testing.T) {
	server := marketing{service: &fakeMarketing{}}
	got := toolCall(t, server, "get_evidence", `{"run_id":"first","ids":["E2"],"source":"seo"}`)
	var items []model.Evidence
	if got.IsError || json.Unmarshal([]byte(got.Content[0].Text), &items) != nil || len(items) != 1 || items[0].ID != "E2" {
		t.Fatalf("wrong filtered evidence: %#v", got)
	}
	got = toolCall(t, server, "get_evidence", `{"run_id":"first","source":"missing","include_unknowns":true}`)
	var withUnknowns struct {
		Evidence []model.Evidence
		Unknowns []model.Unknown
	}
	if got.IsError || json.Unmarshal([]byte(got.Content[0].Text), &withUnknowns) != nil || withUnknowns.Evidence == nil || len(withUnknowns.Evidence) != 0 || len(withUnknowns.Unknowns) != 1 {
		t.Fatalf("wrong evidence/unknowns result: %#v", got)
	}
	got = toolCall(t, server, "get_content", `{"run_id":"first","platform":"meta_ads","sort":"longest_running","limit":1}`)
	var content []model.Content
	if got.IsError || json.Unmarshal([]byte(got.Content[0].Text), &content) != nil || len(content) != 1 || content[0].ID != "ad2" {
		t.Fatalf("wrong filtered content: %#v", got)
	}
}

func TestToolsDistinguishProtocolAndExecutionErrors(t *testing.T) {
	server := marketing{service: &fakeMarketing{}}
	_, failure := server.handle(context.Background(), "tools/call", json.RawMessage(`{"name":"unknown_tool"}`))
	if failure == nil || failure.Code != -32602 {
		t.Fatalf("expected unknown-tool protocol error: %#v", failure)
	}
	for _, call := range []struct{ name, args string }{
		{"get_evidence", `{"run_id":"missing"}`},
		{"get_content", `{"run_id":"first","limit":101}`},
		{"reverse_engineer_marketing", `{"url":"example.com","max_credits":-1}`},
		{"reverse_engineer_marketing", `{"url":"example.com","only":["unknown"]}`},
		{"compare_runs", `{"domain":"example.com","run_a":"first"}`},
	} {
		if got := toolCall(t, server, call.name, call.args); !got.IsError {
			t.Errorf("%s expected tool error: %#v", call.name, got)
		}
	}
}

func TestCompareSelectsLatestTwoRunsOfRequestedDomain(t *testing.T) {
	service := &fakeMarketing{}
	got := toolCall(t, marketing{service: service}, "compare_runs", `{"domain":"https://www.example.com/pricing"}`)
	if got.IsError || service.domain != "example.com" {
		t.Fatalf("expected normalized comparison domain: %#v, %q", got, service.domain)
	}
}

func TestGetPromptUsesExplicitUnknownsQuery(t *testing.T) {
	server := marketing{}
	raw, failure := server.handle(context.Background(), "prompts/get", json.RawMessage(`{"name":"audit_unknowns","arguments":{"run_id":"example-run"}}`))
	encoded, err := json.Marshal(raw)
	if failure != nil || err != nil || !strings.Contains(string(encoded), "include_unknowns") || !strings.Contains(string(encoded), "example-run") {
		t.Fatalf("wrong audit prompt: %s, %v, %#v", encoded, err, failure)
	}
	_, failure = server.handle(context.Background(), "prompts/get", json.RawMessage(`{"name":"audit_unknowns"}`))
	if failure == nil || failure.Code != -32602 {
		t.Fatalf("expected missing prompt argument error: %#v", failure)
	}
}

func TestListsUpstreamToolAndPromptNames(t *testing.T) {
	server := marketing{}
	for _, list := range []struct {
		method, key string
		names       []string
	}{
		{"tools/list", "tools", []string{"reverse_engineer_marketing", "get_evidence", "get_content", "get_playbook", "compare_runs", "validate_report", "write_report"}},
		{"prompts/list", "prompts", []string{"investigate_company", "compare_competitors", "what_changed", "steal_the_playbook", "audit_unknowns"}},
	} {
		got, failure := server.handle(context.Background(), list.method, nil)
		encoded, err := json.Marshal(got)
		var envelope map[string][]struct{ Name string }
		if err != nil || failure != nil || json.Unmarshal(encoded, &envelope) != nil || len(envelope[list.key]) != len(list.names) {
			t.Fatalf("expected list of %v, got %s, %v, %#v", list.names, encoded, err, failure)
		}
		for _, name := range list.names {
			found := false
			for _, item := range envelope[list.key] {
				found = found || item.Name == name
			}
			if !found {
				t.Errorf("%s omits %s", list.method, name)
			}
		}
	}
}

func TestWriteReportUsesConfiguredEngineAndModel(t *testing.T) {
	service := &fakeMarketing{}
	got := toolCall(t, marketing{service: service}, "write_report", `{"run_id":"first","model":"test-model","engine":"anthropic"}`)
	if got.IsError || service.engine != "anthropic" || service.model != "test-model" || got.Content[0].Text != "# Evidence report" {
		t.Fatalf("wrong write_report result: %#v with %#v", got, service)
	}
}

func toolCall(t *testing.T, server marketing, name, arguments string) toolResult {
	t.Helper()
	got, failure := server.handle(context.Background(), "tools/call", json.RawMessage(`{"name":`+string(quoted(name))+`,"arguments":`+arguments+`}`))
	if failure != nil {
		t.Fatalf("%s: unexpected protocol error %#v", name, failure)
	}
	result, ok := got.(toolResult)
	if !ok || len(result.Content) != 1 {
		t.Fatalf("%s: invalid tool result: %#v", name, got)
	}
	return result
}

type fakeMarketing struct {
	opts   app.Options
	domain string
	engine string
	model  string
}

func (s *fakeMarketing) Investigate(_ context.Context, _ string, opts app.Options) (model.Bundle, error) {
	s.opts = opts
	return fixtureBundle("first"), nil
}

func (s *fakeMarketing) load(id string) (model.Bundle, error) {
	if id == "missing" {
		return model.Bundle{}, errors.New("run not found")
	}
	return fixtureBundle(id), nil
}

func (s *fakeMarketing) latestPair(domain string) (model.Bundle, model.Bundle, error) {
	s.domain = domain
	return fixtureBundle("first"), fixtureBundle("second"), nil
}

func (s *fakeMarketing) WriteReport(_ context.Context, _ model.Bundle, engine, modelName string) (string, error) {
	s.engine, s.model = engine, modelName
	return "# Evidence report", nil
}

func fixtureBundle(id string) model.Bundle {
	return model.Bundle{
		ID:     id,
		Brand:  "Example",
		Target: model.Target{Domain: "example.com"},
		Evidence: []model.Evidence{
			{ID: "E1", Source: "website", Title: "Homepage", URL: "https://example.com", Data: "raw evidence body"},
			{ID: "E2", Source: "seo", Title: "Sitemap"},
		},
		Unknowns: []model.Unknown{{Channel: "paid_meta", Reason: "No key", ManualCheck: "https://example.com/ads"}},
		Content: []model.Content{
			{ID: "ad1", Platform: "meta_ads", Metrics: map[string]float64{"runningDays": 1}},
			{ID: "ad2", Platform: "meta_ads", Metrics: map[string]float64{"runningDays": 10}},
			{ID: "ad3", Platform: "google_ads", Metrics: map[string]float64{"runningDays": 100}},
		},
	}
}
