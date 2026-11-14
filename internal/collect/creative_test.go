package collect

import (
	"testing"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

func TestDeconstructCreativeClassifiesAngles(t *testing.T) {
	contents := []model.Content{
		{
			ID:       "c1",
			Kind:     "video",
			Text:     "Tired of wasting hours managing spreadsheets?\nOur AI automates your workflow in 1 click.\nOver 10,000 teams switch every month.\nGet started free.",
			Evidence: "E1",
		},
		{
			ID:       "c2",
			Kind:     "image",
			Text:     "The best alternative to Jira.\nFast, keyboard-first issue tracking.\n10x faster response time.\nTry free forever.",
			Evidence: "E2",
		},
		{
			ID:       "c3",
			Kind:     "post",
			Text:     "Why we built Linear:\nWe believe software development should feel magical, not bogged down by enterprise bureaucracy.",
			Evidence: "E3",
		},
	}

	evidences := []model.Evidence{
		{
			ID:     "E4",
			Source: "website",
			Kind:   "observation",
			Data: map[string]any{
				"title": "Linear — A better way to build products",
				"h1":    []string{"Issue tracking designed for speed"},
			},
		},
	}

	angles := DeconstructCreative(contents, evidences)
	if len(angles) < 3 {
		t.Fatalf("expected at least 3 creative angles, got %d", len(angles))
	}

	// Verify categories
	categories := make(map[string]bool)
	for _, a := range angles {
		categories[a.Category] = true
	}

	if !categories["agitation"] {
		t.Errorf("expected agitation angle from c1")
	}
	if !categories["us_vs_them"] {
		t.Errorf("expected us_vs_them angle from c2")
	}
	if !categories["founder_story"] {
		t.Errorf("expected founder_story angle from c3")
	}

	// Check REA structure on c1
	var c1Angle *model.CreativeAngle
	for i := range angles {
		if angles[i].Category == "agitation" {
			c1Angle = &angles[i]
			break
		}
	}
	if c1Angle == nil {
		t.Fatal("c1 agitation angle not found")
	}
	if c1Angle.Format != "video" {
		t.Errorf("expected video format, got %q", c1Angle.Format)
	}
	if c1Angle.Proof == "" {
		t.Errorf("expected proof extraction from c1")
	}
	if c1Angle.OfferCTA == "" {
		t.Errorf("expected offer CTA extraction from c1")
	}
}
