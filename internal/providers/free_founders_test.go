package providers

import (
	"net/url"
	"strings"
	"testing"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

func TestWikidataMatchesOfficialSiteAndFollowsDeveloper(t *testing.T) {
	get := freeFixture(func(u *url.URL) (int, string, error) {
		if u.Host != "www.wikidata.org" || u.Path != "/w/api.php" {
			t.Fatalf("unexpected Wikidata URL %s", u)
		}
		q := u.Query()
		if q.Get("action") == "wbsearchentities" {
			return 200, `{"search":[{"id":"Q3"},{"id":"Q1"}]}`, nil
		}
		switch q.Get("ids") {
		case "Q3|Q1":
			return 200, `{"entities":{"Q3":{"claims":{"P856":[{"mainsnak":{"datavalue":{"value":"https://other.example.net"}}}]}},"Q1":{"claims":{"P856":[{"mainsnak":{"datavalue":{"value":"https://www.example.com/"}}}],"P178":[{"mainsnak":{"datavalue":{"value":{"id":"Q2"}}}}]}}}}`, nil
		case "Q2":
			return 200, `{"entities":{"Q2":{"labels":{"en":{"value":"Example Organization"}},"claims":{"P112":[{"mainsnak":{"datavalue":{"value":{"id":"Q4"}}}}],"P169":[{"mainsnak":{"datavalue":{"value":{"id":"Q4"}}}}],"P154":[{"mainsnak":{"datavalue":{"value":"Example Logo.svg"}}}]}}}}`, nil
		case "Q4":
			return 200, `{"entities":{"Q4":{"labels":{"en":{"value":"Alex Example"}}}}}`, nil
		default:
			t.Fatalf("unexpected IDs %s", q.Get("ids"))
			return 500, "", nil
		}
	})
	discovered, result := FoundersFree(t.Context(), get, model.Target{Domain: "example.com", Brand: "Example"}, model.Discovered{})
	if len(result.Evidence) != 1 || len(result.Unknowns) != 0 || result.Evidence[0].URL != "https://www.wikidata.org/wiki/Q2" {
		t.Fatalf("unexpected evidence %+v", result)
	}
	if len(discovered.Founders) != 1 || discovered.Founders[0].Name != "Alex Example" || discovered.Founders[0].Role != "Founder" || discovered.Founders[0].URL != "https://www.wikidata.org/wiki/Q4" {
		t.Fatalf("unexpected founders %+v", discovered.Founders)
	}
	if len(discovered.Logos) != 1 || !strings.Contains(discovered.Logos[0].URL, "Example%20Logo.svg") {
		t.Fatalf("unexpected logos %+v", discovered.Logos)
	}
}

func TestWikidataUnknownPreservesWebsiteDiscoveries(t *testing.T) {
	get := freeFixture(func(*url.URL) (int, string, error) { return 200, `{"search":[]}`, nil })
	existing := model.Discovered{Founders: []model.Founder{{Name: "Alex Example"}}}
	discovered, result := FoundersFree(t.Context(), get, model.Target{Domain: "example.com", Brand: "Example"}, existing)
	if len(result.Evidence) != 0 || len(result.Unknowns) != 1 || len(discovered.Founders) != 1 {
		t.Fatalf("discoveries=%+v result=%+v", discovered, result)
	}
}
