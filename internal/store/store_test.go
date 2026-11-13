package store

import (
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"sync"
	"testing"
)

func TestConcurrentRunHistoryAndIndependentOutput(t *testing.T) {
	s := New(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			b := model.Bundle{Target: model.Target{Domain: "example.org"}, Brand: "Example", GeneratedAt: "2026-11-13T01:00:00Z"}
			if _, err := s.Save(b, "# report"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	rows, err := s.List()
	if err != nil || len(rows) != 8 {
		t.Fatalf("%v %v", rows, err)
	}
	for _, row := range rows {
		b, err := s.Load(row.ID)
		if err != nil || b.ID != row.ID || b.Brand != "Example" {
			t.Fatal(b, err)
		}
		r, err := s.Report(row.ID)
		if err != nil || r != "# report" {
			t.Fatal(r, err)
		}
	}
	if _, err = s.Load("../outside"); err == nil {
		t.Fatal("unexpected outside read")
	}
}
