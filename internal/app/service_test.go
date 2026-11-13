package app

import (
	"context"
	"errors"
	"testing"

	"remgo/internal/fetch"
	"remgo/internal/report"
	"remgo/internal/store"
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
