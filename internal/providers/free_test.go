package providers

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

type freeFixture func(*url.URL) (int, string, error)

func (fixture freeFixture) Get(_ context.Context, rawURL string) (fetch.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fetch.Response{}, err
	}
	status, body, err := fixture(u)
	return fetch.Response{URL: rawURL, Status: status, Body: []byte(body)}, err
}

func TestAppleAppResolvesSellerAndPreservesUnknownRating(t *testing.T) {
	get := freeFixture(func(u *url.URL) (int, string, error) {
		if u.Host != "itunes.apple.com" || u.Path != "/lookup" || u.Query().Get("id") != "123456" {
			t.Fatalf("unexpected URL %s", u)
		}
		return 200, `{"results":[{"trackId":123456,"trackName":"Example App","sellerName":"Jane Smith","sellerUrl":"https://www.example.com/path","artworkUrl512":"https://img.example.com/icon.png","trackViewUrl":"https://apps.apple.com/us/app/id123456","userRatingCount":12}]}`, nil
	})
	target, discovered, result := Apps(t.Context(), get, model.Target{Kind: "app_store", AppleID: "123456"}, model.Discovered{})
	if target.URL != "https://www.example.com" || target.Domain != "example.com" || target.Brand != "Example App" {
		t.Fatalf("unexpected target %+v", target)
	}
	if len(discovered.Logos) != 1 || len(discovered.Founders) != 1 || len(discovered.AppStoreIDs) != 1 {
		t.Fatalf("missing discovery: %+v", discovered)
	}
	if len(result.Evidence) != 1 || len(result.Unknowns) != 0 {
		t.Fatalf("unexpected result %+v", result)
	}
	data := result.Evidence[0].Data.(map[string]any)
	if _, exists := data["rating"]; exists {
		t.Fatalf("missing rating represented as a number: %+v", data)
	}
}

func TestPlayReadsDownloadsAndLabeledDeveloperWebsite(t *testing.T) {
	get := freeFixture(func(u *url.URL) (int, string, error) {
		if u.Host != "play.google.com" || u.Query().Get("id") != "com.example.app" {
			t.Fatalf("unexpected URL %s", u)
		}
		return 200, `<title>Example - Apps on Google Play</title><meta name="description" content="A test app"><a href="https://example.com/support">Website</a><div>10K+</div><div>Downloads</div>`, nil
	})
	target, _, result := Apps(t.Context(), get, model.Target{Kind: "play_store", PlayID: "com.example.app"}, model.Discovered{})
	if target.URL != "https://example.com" || target.Domain != "example.com" || target.Brand != "Example" || len(result.Evidence) != 1 {
		t.Fatalf("target=%+v result=%+v", target, result)
	}
	if data := result.Evidence[0].Data.(map[string]any); data["installs"] != "10K+" || data["description"] != "A test app" {
		t.Fatalf("unexpected Play data %+v", data)
	}
}

func TestHistoryPreservesPartialCaptureAndArchiveCount(t *testing.T) {
	get := freeFixture(func(u *url.URL) (int, string, error) {
		if u.Path == "/cdx/search/cdx" {
			if u.Query().Get("url") != "example.com" || u.Query().Get("collapse") != "timestamp:6" {
				t.Fatalf("unexpected CDX query %s", u)
			}
			return 200, `[["timestamp","statuscode"],["20171112090133","200"],["20141112090133","200"]]`, nil
		}
		if strings.Contains(u.Path, "20141112") {
			return 200, `<title>Original &amp; first</title><h1>Early message</h1><a href="/start">Get started</a>`, nil
		}
		return 429, "Too many requests", nil
	})
	result := History(t.Context(), get, model.Target{Domain: "example.com", URL: "https://example.com"})
	if len(result.Evidence) != 1 || len(result.Unknowns) != 1 {
		t.Fatalf("expected partial history and unknown, got %+v", result)
	}
	data := result.Evidence[0].Data.(map[string]any)
	captures := data["history"].([]map[string]any)
	if data["monthlySnapshots"] != 2 || len(captures) != 1 || captures[0]["date"] != "2014-11-12" || captures[0]["title"] != "Original & first" {
		t.Fatalf("unexpected history %+v", data)
	}
}

func TestHackerNewsSeparatesEmptyFromFailure(t *testing.T) {
	for _, tc := range []struct {
		name               string
		body               string
		err                error
		evidence, unknowns int
	}{
		{"empty", `{"hits":[],"nbHits":0}`, nil, 1, 0},
		{"unexpected schema", `{"error":"unavailable"}`, nil, 0, 1},
		{"connection failure", "", errors.New("timeout"), 0, 1},
		{"ranking with domain verification", `{"hits":[{"objectID":"2","url":"https://other.example.net/?q=example.com","title":"Wrong brand","points":100},{"objectID":"1","url":"https://www.example.com/start","title":"Show HN: Example","points":12}],"nbHits":2}`, nil, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			get := freeFixture(func(u *url.URL) (int, string, error) {
				if u.Query().Get("query") != "example.com" {
					t.Fatalf("unexpected HN query %s", u)
				}
				return 200, tc.body, tc.err
			})
			result := Community(t.Context(), get, model.Target{Domain: "example.com"})
			if len(result.Evidence) != tc.evidence || len(result.Unknowns) != tc.unknowns {
				t.Fatalf("unexpected result %+v", result)
			}
			if tc.name == "ranking with domain verification" && !strings.Contains(result.Evidence[0].Title, "12 points") {
				t.Fatalf("unrelated result ranked: %+v", result.Evidence[0])
			}
		})
	}
}

func TestAppleAdsSampleIsBoundedAndCardsResolve(t *testing.T) {
	var offsets []string
	get := freeFixture(func(u *url.URL) (int, string, error) {
		if u.Path == "/api/v1/ad-repository-entities" {
			return 200, `{"data":[{"id":123456,"name":"Example"},{"id":654321,"name":"Example Alt"}]}`, nil
		}
		if u.Path != "/api/v1/ad-repository-ads" || u.Query().Get("ql") != "type==APP;id==123456;datePreset==LAST_YEAR" {
			t.Fatalf("unexpected Apple Ads URL %s", u)
		}
		offsets = append(offsets, u.Query().Get("offset"))
		return 200, `{"pagination":{"totalResults":120},"dataStartDate":"2025-11-01","dataEndDate":"2026-11-01","data":[{"adId":"abc","appId":123456,"appName":"Example","developerName":"Example Co","format":"SEARCH","countryOrRegion":"DE","placement":"APPSTORE_SEARCH_TAB","firstImpressionDate":"2026-10-15","lastImpressionDate":"2026-10-30","adBanner":{"subtitle":"Example ad"},"adAssets":[{"pictureUrl":"https://example.com/creative.png"}]}]}`, nil
	})
	result := AppleAds(t.Context(), get, model.Target{AppleID: "123456", Brand: "Example"}, model.Discovered{})
	if strings.Join(offsets, ",") != "0,1,2" || len(result.Evidence) != 2 || len(result.Content) != 1 || len(result.Unknowns) != 0 {
		t.Fatalf("offsets=%v result=%+v", offsets, result)
	}
	card := result.Content[0]
	if card.Evidence == "" || card.Evidence != result.Evidence[0].ID || card.Status != "active" || card.Metrics["runningDays"] != 15 {
		t.Fatalf("unexpected card %+v", card)
	}
}

func TestAppleAdsMissingDataIsUnknown(t *testing.T) {
	get := freeFixture(func(*url.URL) (int, string, error) { return 200, `{"error":"unavailable"}`, nil })
	result := AppleAds(t.Context(), get, model.Target{AppleID: "123456"}, model.Discovered{})
	if len(result.Evidence) != 0 || len(result.Unknowns) != 1 {
		t.Fatalf("expected unknown, got %+v", result)
	}
}

func TestArchiveSelectionUsesCalendarWindows(t *testing.T) {
	rows := []string{"20140102112233", "20241102112233", "20250502112233", "20251102112233", "20260502112233", "20261102112233"}
	want := "20140102112233,20241102112233,20251102112233,20260502112233,20261102112233"
	if got := strings.Join(chooseArchiveDates(rows, time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)), ","); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestHostAttributionDoesNotCrossPublicHostingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		link, domain string
		match        bool
	}{
		{"https://example.com", "blog.example.com", true},
		{"https://a.github.io", "github.io", false},
		{"https://co.uk", "example.co.uk", false},
		{"https://other-example.com", "example.com", false},
	} {
		if got := matchingHost(tc.link, tc.domain); got != tc.match {
			t.Errorf("matchingHost(%q, %q) = %v, want %v", tc.link, tc.domain, got, tc.match)
		}
	}
}

func TestAppleStoreLookupUsesStorefrontFromTarget(t *testing.T) {
	get := freeFixture(func(u *url.URL) (int, string, error) {
		if u.Query().Get("country") != "gb" {
			t.Errorf("GB App Store target requested without storefront: %s", u)
		}
		return 200, `{"results":[]}`, nil
	})
	_, _, _ = Apps(t.Context(), get, model.Target{Input: "https://apps.apple.com/gb/app/name/id123456", Kind: "app_store", AppleID: "123456"}, model.Discovered{})
}
