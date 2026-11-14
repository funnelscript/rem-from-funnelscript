package app

import (
	"context"
	"errors"
	"testing"

	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/report"
	"github.com/funnelscript/rem-from-funnelscript/internal/store"
)

type getterFunc func(context.Context, string) (fetch.Response, error)

func (f getterFunc) Get(ctx context.Context, u string) (fetch.Response, error) { return f(ctx, u) }

func TestInvestigationPersistsEvidenceThroughNormalPipeline(t *testing.T) {
	s := New(store.New(t.TempDir()))
	s.get = getterFunc(func(_ context.Context, u string) (fetch.Response, error) {
		return fetch.Response{URL: u, Status: 200, Body: []byte(`<html><head><title>Example workshop</title></head><body><h1>Learn woodworking</h1><a href="https://www.instagram.com/example/">Instagram</a></body></html>`)}, nil
	})
	b, err := s.Investigate(context.Background(), "example.org", Options{Only: []string{"website"}, Engine: "evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID == "" || len(b.Evidence) == 0 || b.Evidence[0].ID != "E1" {
		t.Fatalf("%+v", b)
	}
	md, err := s.Store.Report(b.ID)
	if err != nil || !report.Validate(md, b).OK {
		t.Fatalf("%s %v", md, err)
	}
	if rows, err := s.Store.List(); err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
}

func TestCanceledInvestigationDoesNotPublish(t *testing.T) {
	s := New(store.New(t.TempDir()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Investigate(ctx, "example.org", Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	rows, err := s.Store.List()
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}

func TestInvestigationPopulatesEntrepreneurAnalysis(t *testing.T) {
	s := New(store.New(t.TempDir()))
	s.get = getterFunc(func(_ context.Context, u string) (fetch.Response, error) {
		return fetch.Response{
			URL:    u,
			Status: 200,
			Body: []byte(`<html><head><title>Acme — Fast Modern Issue Tracker</title></head>
<body>
  <h1>Tired of slow, bloated project management software?</h1>
  <p>Our engine is 10x faster than legacy tools. Over 50,000 teams trust Acme.</p>
  <a href="/pricing">Get started free — no credit card required</a>
</body></html>`),
		}, nil
	})

	b, err := s.Investigate(context.Background(), "acme.com", Options{Only: []string{"website"}, Engine: "evidence"})
	if err != nil {
		t.Fatal(err)
	}

	if len(b.Entrepreneur.CreativeAngles) == 0 {
		t.Fatal("expected b.Entrepreneur.CreativeAngles to be populated, got 0")
	}

	foundAgitation := false
	for _, angle := range b.Entrepreneur.CreativeAngles {
		if angle.Category == "agitation" {
			foundAgitation = true
			if len(angle.EvidenceIDs) == 0 {
				t.Errorf("expected evidence ID citation in creative angle")
			}
		}
	}
	if !foundAgitation {
		t.Errorf("expected agitation angle from 'Tired of slow, bloated...'")
	}

	doc := s.Doctor()
	freeSources, ok := doc["freeSources"].([]string)
	if !ok || len(freeSources) < 13 {
		t.Fatalf("unexpected freeSources in doctor: %#v", doc["freeSources"])
	}
}
