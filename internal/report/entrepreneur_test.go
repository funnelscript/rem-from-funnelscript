package report

import (
	"testing"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

func TestAnalyzeEntrepreneurExtractsRankedGrievancesAndPlays(t *testing.T) {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	bundle := model.Bundle{
		Brand:       "SlowCorp",
		GeneratedAt: nowStr,
		Evidence: []model.Evidence{
			{
				ID:         "E1",
				Source:     "reddit",
				Kind:       "community_discussion",
				Title:      "SlowCorp is so slow and bloated now",
				ObservedAt: "2026-11-01",
				Data: map[string]any{
					"comment": "Ever since the last update, SlowCorp is painfully slow and crashes on large files. Deal breaker.",
				},
			},
			{
				ID:         "E2",
				Source:     "hackernews",
				Kind:       "community_discussion",
				Title:      "SlowCorp pricing increase",
				ObservedAt: "2026-10-15",
				Data: map[string]any{
					"comment": "They announced a huge price hike and forced upgrade to enterprise tier. Switched to an alternative.",
				},
			},
			{
				ID:         "E3",
				Source:     "reddit",
				Kind:       "community_discussion",
				Title:      "Sync failed in SlowCorp",
				ObservedAt: "2026-09-01",
				Data: map[string]any{
					"comment": "Integration broken and webhook sync failed, causing data loss.",
				},
			},
		},
		Content: []model.Content{
			{
				ID:       "c1",
				Kind:     "video",
				Text:     "Tired of waiting for spreadsheets to load?\nOur engine is 10x faster.\nGet started free.",
				Evidence: "E4",
			},
		},
	}

	analysis := AnalyzeEntrepreneur(bundle)

	// 1. Verify Creative Angles
	if len(analysis.CreativeAngles) == 0 {
		t.Fatal("expected creative angles from content, got 0")
	}
	if analysis.CreativeAngles[0].Category != "agitation" && analysis.CreativeAngles[0].Category != "shock_stat" {
		t.Errorf("unexpected creative category: %q", analysis.CreativeAngles[0].Category)
	}

	// 2. Verify Grievances
	if len(analysis.TopGrievances) == 0 {
		t.Fatal("expected top grievances, got 0")
	}

	themeMap := make(map[string]model.Grievance)
	for _, g := range analysis.TopGrievances {
		themeMap[g.Theme] = g
		if g.Score <= 0 {
			t.Errorf("expected positive score for %q, got %f", g.Theme, g.Score)
		}
		if len(g.SampleQuotes) == 0 {
			t.Errorf("expected verbatim quotes for %q", g.Theme)
		}
		if len(g.EvidenceIDs) == 0 {
			t.Errorf("expected evidence IDs for %q", g.Theme)
		}
	}

	if _, ok := themeMap["Performance & Speed"]; !ok {
		t.Errorf("expected Performance & Speed theme from E1")
	}
	if _, ok := themeMap["Pricing & Monetization"]; !ok {
		t.Errorf("expected Pricing & Monetization theme from E2")
	}

	// 3. Verify Displacement Plays
	if len(analysis.Displacement) == 0 {
		t.Fatal("expected displacement plays, got 0")
	}

	var hasPerformancePlay, hasPricingPlay bool
	for _, p := range analysis.Displacement {
		if p.CounterHook == "" || p.AnglePlaybook == "" || p.RecommendedCTA == "" {
			t.Errorf("incomplete displacement play: %#v", p)
		}
		if len(p.EvidenceIDs) == 0 {
			t.Errorf("missing evidence citations in displacement play")
		}
		if p.ImpactROI == "" {
			t.Errorf("missing impact ROI in displacement play")
		}
		if p.TargetWeakness != "" {
			hasPerformancePlay = true
		}
		if p.ImpactROI == "High (Immediate economic switching trigger)" {
			hasPricingPlay = true
		}
	}

	if !hasPerformancePlay || !hasPricingPlay {
		t.Errorf("missing expected displacement play types")
	}
}
