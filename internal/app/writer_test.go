package app

import (
	"context"
	"io"
	"net/http"
	"remgo/internal/model"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAnthropicWriterSendsOnlyReportInputsAndParsesText(t *testing.T) {
	c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		if r.URL.Host != "api.anthropic.com" || r.Header.Get("x-api-key") != "fixture" || !strings.Contains(body, "ref-evidence") || strings.Contains(body, "excluded-media") {
			t.Fatalf("%s %s", r.URL, body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"# Result"}]}`))}, nil
	})}
	b := model.Bundle{Evidence: []model.Evidence{{ID: "E1", Title: "ref-evidence"}}, Content: []model.Content{{Title: "excluded-media"}}}
	got, err := anthropic(context.Background(), c, "fixture", "", "claude-test", prompt(b))
	if err != nil || got != "# Result" {
		t.Fatal(got, err)
	}
}
