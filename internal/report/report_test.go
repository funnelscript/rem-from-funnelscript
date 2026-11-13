package report

import (
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"strings"
	"testing"
)

func TestEvidenceMarkdownAndUnknownChannel(t *testing.T) {
	b := model.Bundle{Brand: "<script>", Target: model.Target{Input: "example.org"}, Evidence: []model.Evidence{{ID: "E1", Source: "website", Title: "Homepage facts", URL: "https://example.org", Data: map[string]any{"h1": "About us"}}}, Unknowns: []model.Unknown{{Channel: "paid_meta", Reason: "Provider key not configured"}}}
	b.Scorecard = Score(b)
	md := Markdown(b)
	if !strings.Contains(md, "[E1]") || !strings.Contains(md, "Provider key not configured") || strings.Contains(md, "<script>") {
		t.Fatal(md)
	}
	if got := Validate(md, b); !got.OK {
		t.Fatalf("%+v", got)
	}
	if got := Validate(md+"\n[E999] $25k ad spend", b); got.OK {
		t.Fatalf("%+v", got)
	}
	for _, c := range b.Scorecard {
		if c.Channel == "paid_meta" && c.Status != "unknown" {
			t.Fatal(c)
		}
	}
}

func TestDiffDistinguishesMissingFromRemoved(t *testing.T) {
	a := model.Bundle{Target: model.Target{Domain: "example.org"}, Evidence: []model.Evidence{{Source: "meta_ads"}}, Content: []model.Content{{ID: "x", Platform: "meta_ads", Kind: "ad"}}, Scorecard: []model.ChannelScore{{Channel: "paid_meta", Status: "present"}}}
	b := model.Bundle{Target: a.Target, Scorecard: []model.ChannelScore{{Channel: "paid_meta", Status: "unknown"}}}
	d := Compare(a, b)
	if len(d.NoLongerSeen) != 0 || len(d.Channels) != 1 {
		t.Fatalf("%+v", d)
	}
	b.Evidence = []model.Evidence{{Source: "meta_ads"}}
	d = Compare(a, b)
	if len(d.NoLongerSeen) != 1 {
		t.Fatalf("%+v", d)
	}
}

func TestQuotedLiteralsAndMarkerCannotEvadeNarrativeChecks(t *testing.T) {
	b := model.Bundle{Brand: "Example", Evidence: []model.Evidence{{ID: "E1", Source: "website", Title: "Reference [E999] advertises $500 ad spend"}}}
	b.Scorecard = Score(b)
	if got := Validate(Markdown(b), b); !got.OK {
		t.Fatalf("literal source quote: %+v", got)
	}
	fake := "# Arbitrary\n\n<!-- remgo:deterministic -->\n\n## Channel scorecard\n\n## Evidence\n"
	if got := Validate(fake, b); got.OK {
		t.Fatalf("spoofed marker accepted: %+v", got)
	}
}

func TestUnknownAdStatusIsNotDormant(t *testing.T) {
	b := model.Bundle{Evidence: []model.Evidence{{ID: "E1", Source: "meta_ads", Title: "Historical ad count", Data: map[string]any{"totalInLibrary": 100}}}}
	if got := Score(b)[0]; got.Status != "likely" {
		t.Fatal(got)
	}
}
