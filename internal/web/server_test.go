package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

func request(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://127.0.0.1:4321"+path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestServesAssetsAndRejectsForeignRequests(t *testing.T) {
	h := newHandler(backend{})
	for _, path := range []string{"/", "/app.js", "/app.css"} {
		w := request(t, h, "GET", path, "")
		if w.Code != http.StatusOK || w.Body.Len() == 0 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatalf("%s: got status %d and headers %v", path, w.Code, w.Header())
		}
	}
	for _, tc := range []struct {
		host   string
		origin string
	}{
		{"other.example:4321", ""},
		{"127.0.0.1:4321", "https://other.example"},
		{"127.0.0.1:4321", "http://127.0.0.1:4322"},
		{"127.0.0.1:4321", "null"},
	} {
		r := httptest.NewRequest("POST", "http://"+tc.host+"/api/runs", nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("host %q origin %q: got %d", tc.host, tc.origin, w.Code)
		}
	}
}

func TestRunProgressAndStoredRun(t *testing.T) {
	h := newHandler(backend{
		doctor: func() map[string]any {
			return map[string]any{"engines": map[string]bool{"evidence": true}}
		},
		investigate: func(ctx context.Context, input string, opts app.Options) (model.Bundle, error) {
			if input != "example.com" || opts.Engine != "evidence" {
				t.Errorf("unexpected investigate request %q, %#v", input, opts)
			}
			opts.OnStage("website", "start")
			opts.OnStage("website", "done")
			return model.Bundle{ID: "example-1", Brand: "Example"}, nil
		},
		list: func() ([]model.Summary, error) { return []model.Summary{}, nil },
		load: func(id string) (model.Bundle, error) {
			return model.Bundle{ID: id, Brand: "Example"}, nil
		},
		report: func(id string) (string, error) { return "Website [E1]", nil },
	})
	w := request(t, h, "POST", "/api/runs", `{"url":"example.com"}`)
	id := readJobID(t, w)
	deadline := time.Now().Add(time.Second)
	for {
		w = request(t, h, "GET", "/api/jobs/"+id, "")
		var state jobStatus
		if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if state.State == "done" {
			if state.RunID != "example-1" || state.Stages["website"] != "done" {
				t.Fatalf("incomplete job state %#v", state)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not finish, last state %#v", state)
		}
		time.Sleep(time.Millisecond)
	}
	w = request(t, h, "GET", "/api/runs/example-1", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Website [E1]") {
		t.Fatalf("stored run response %d: %s", w.Code, w.Body)
	}
	w = request(t, h, "GET", "/api/runs", "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty history response %d: %s", w.Code, w.Body)
	}
}

func TestCancellationAndCapacity(t *testing.T) {
	exited := make(chan struct{}, 2)
	h := newHandler(backend{
		doctor: func() map[string]any {
			return map[string]any{"engines": map[string]bool{"evidence": true}}
		},
		investigate: func(ctx context.Context, _ string, _ app.Options) (model.Bundle, error) {
			<-ctx.Done()
			exited <- struct{}{}
			return model.Bundle{}, ctx.Err()
		},
	})
	w := request(t, h, "POST", "/api/runs", `{"url":"example.com"}`)
	one := readJobID(t, w)
	w = request(t, h, "POST", "/api/runs", `{"url":"example.com"}`)
	two := readJobID(t, w)
	w = request(t, h, "POST", "/api/runs", `{"url":"example.com"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected capacity rejection, got %d", w.Code)
	}
	for _, id := range []string{one, two} {
		w = request(t, h, "POST", "/api/jobs/"+id+"/cancel", "")
		if w.Code != http.StatusAccepted {
			t.Fatalf("cancel status %d", w.Code)
		}
		select {
		case <-exited:
		case <-time.After(time.Second):
			t.Fatal("cancellation did not reach investigation")
		}
	}
}

func TestRejectsInvalidInput(t *testing.T) {
	h := newHandler(backend{
		doctor: func() map[string]any {
			return map[string]any{"engines": map[string]bool{"evidence": true}}
		},
	})
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"url":" "}`, http.StatusBadRequest},
		{`{"url":"example.com","only":["typo"]}`, http.StatusBadRequest},
		{`{"url":"example.com","maxCredits":501}`, http.StatusBadRequest},
		{`{"url":"example.com","engine":"typo"}`, http.StatusBadRequest},
		{`{"url":"example.com"} {}`, http.StatusBadRequest},
		{`{"url":"` + strings.Repeat("x", 70<<10) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		w := request(t, h, "POST", "/api/runs", tc.body)
		if w.Code != tc.code {
			t.Fatalf("payload of %d bytes got %d want %d", len(tc.body), w.Code, tc.code)
		}
	}
}

func TestCompareLatestRuns(t *testing.T) {
	h := newHandler(backend{
		latestPair: func(domain string) (model.Bundle, model.Bundle, error) {
			if domain != "example.com" {
				t.Fatalf("unexpected comparison target %q", domain)
			}
			return model.Bundle{GeneratedAt: "2026-11-11"}, model.Bundle{GeneratedAt: "2026-11-13", Brand: "Example"}, nil
		},
	})
	w := request(t, h, "GET", "/api/compare?domain=example.com", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"from":"2026-11-11","to":"2026-11-13"`) {
		t.Fatalf("comparison response %d: %s", w.Code, w.Body)
	}
	w = request(t, h, "GET", "/api/compare?domain=", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("comparison missing domain response %d: %s", w.Code, w.Body)
	}
}

func TestStartRunAcceptsStealthKeylessAndWindow(t *testing.T) {
	receivedOpts := make(chan app.Options, 1)
	h := newHandler(backend{
		doctor: func() map[string]any {
			return map[string]any{"engines": map[string]bool{"evidence": true}}
		},
		investigate: func(ctx context.Context, input string, opts app.Options) (model.Bundle, error) {
			receivedOpts <- opts
			return model.Bundle{ID: "run-stealth", Brand: "Example"}, nil
		},
	})

	body := `{"url":"example.com","stealth":true,"keyless":true,"window":"30d"}`
	w := request(t, h, "POST", "/api/runs", body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected accepted status, got %d: %s", w.Code, w.Body)
	}
	select {
	case opts := <-receivedOpts:
		if !opts.Stealth {
			t.Errorf("expected Stealth=true, got false")
		}
		if !opts.Keyless {
			t.Errorf("expected Keyless=true, got false")
		}
		if opts.Window != "30d" {
			t.Errorf("expected Window=30d, got %q", opts.Window)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("investigate was not invoked within timeout")
	}
}

func readJobID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if w.Code != http.StatusAccepted {
		t.Fatalf("run response %d: %s", w.Code, w.Body)
	}
	var result struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.JobID == "" {
		t.Fatalf("invalid job response: %s (%v)", w.Body, err)
	}
	return result.JobID
}
