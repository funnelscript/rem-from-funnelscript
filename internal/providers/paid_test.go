package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

type paidTransport func(*http.Request) (*http.Response, error)

func (f paidTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func paidFixture(t *testing.T, fixture func(*http.Request) (int, string)) *http.Client {
	t.Helper()
	return &http.Client{Transport: paidTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "api.scrapecreators.com" || r.Header.Get("x-api-key") != "fixture-key" {
			t.Errorf("unexpected endpoint or authorization for %s", r.URL.Path)
		}
		status, body := fixture(r)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": {"application/json"}}, Request: r}, nil
	})}
}

func paidTarget() model.Target {
	return model.Target{Domain: "example.com", Brand: "Example"}
}

func TestPaidWithoutKeyProducesUnknowns(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		t.Errorf("no-key run called %s", r.URL.Path)
		return 500, ""
	}), "", 50)
	for _, area := range []string{"paid", "social", "founders", "community"} {
		got := p.Collect(context.Background(), area, paidTarget(), model.Discovered{})
		if len(got.Unknowns) == 0 || len(got.Evidence) != 0 {
			t.Fatalf("%s: %+v", area, got)
		}
	}
	if p.Spent() != nil {
		t.Fatal("no-key provider should have no spend measurement")
	}
}

func TestPaidCreditBudgetAndMalformedData(t *testing.T) {
	var calls atomic.Int32
	p := NewPaid(paidFixture(t, func(_ *http.Request) (int, string) {
		calls.Add(1)
		return 200, `{"success":true}`
	}), "fixture-key", 2)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			got := p.Collect(context.Background(), "paid", paidTarget(), model.Discovered{})
			if len(got.Evidence) != 0 {
				t.Error("malformed data turned into an observation")
			}
		}()
	}
	workers.Wait()
	if count := calls.Load(); count != 2 || *p.Spent() != 2 {
		t.Fatalf("calls=%d spent=%v", count, p.Spent())
	}
}

func TestPaidAdsCollectsDistinctCreativeEvidence(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/v1/facebook/adLibrary/search/companies":
			return 200, `{"searchResults":[{"page_id":"bad","name":"Example Studios"},{"page_id":"123","name":"Example"}]}`
		case "/v1/facebook/adLibrary/company/ads":
			if q.Get("pageId") != "123" {
				t.Errorf("selected wrong company: %s", q.Get("pageId"))
			}
			if q.Get("status") == "ACTIVE" {
				return 200, `{"searchResultsCount":1,"results":[{"ad_archive_id":"1","is_active":true}]}`
			}
			return 200, `{"searchResultsCount":2,"results":[{"ad_archive_id":"1","is_active":true,"snapshot":{"body":{"text":"First ad"},"link_url":"https://apps.apple.com/app/id123456?ppid=landing&ct=testing"}},{"ad_archive_id":"2","is_active":false}]}`
		case "/v1/google/company/ads":
			return 200, `{"number_of_ads_estimate":0,"ads":[]}`
		case "/v1/linkedin/ads/search":
			return 200, `{"ads":[{"id":"li1","advertiser":"Example","url":"https://www.linkedin.com/ad-library/detail/li1","headline":"An ad"},{"id":"li2","advertiser":"Example Studio","url":"https://www.linkedin.com/ad-library/detail/li2"}]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 12)
	got := p.Collect(context.Background(), "paid", paidTarget(), model.Discovered{})
	if len(got.Evidence) != 3 || len(got.Content) != 3 {
		t.Fatalf("evidence=%+v content=%+v unknowns=%+v", got.Evidence, got.Content, got.Unknowns)
	}
	if got.Content[0].Text != "First ad" || got.Content[0].Evidence == "" || got.Content[1].Status != "inactive" || got.Content[2].ID != "linkedin_ads:li1" {
		t.Fatalf("content: %+v", got.Content)
	}
	meta := got.Evidence[0].Data.(map[string]any)
	if !strings.Contains(fmt.Sprint(meta["appStoreLinks"]), "landing") || meta["activeNow"] != float64(1) {
		t.Fatalf("Meta stats: %+v", meta)
	}
}

func TestPaidSocialDistinguishesMissingFeedAndDatedSample(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/twitter/profile":
			return 200, `{"legacy":{"followers_count":42}}`
		case "/v1/twitter/user-tweets":
			return 200, `{"tweets":[{"rest_id":"2","legacy":{"full_text":"Hello there","created_at":"Mon Jan 01 00:00:00 +0000 2024"},"views":{"count":"20"}},{"rest_id":"3","legacy":{"full_text":"Second","created_at":"Sun Jan 14 00:00:00 +0000 2024"},"views":{"count":"40"}}]}`
		case "/v1/linkedin/company":
			return 200, `{"name":"Example","followers":52}`
		default:
			return 500, `{"message":"fixture-key"}`
		}
	}), "fixture-key", 8)
	got := p.Collect(context.Background(), "social", paidTarget(), model.Discovered{Socials: map[string]string{"x": "example", "linkedin": "example", "instagram": "example"}})
	if len(got.Evidence) != 2 || len(got.Content) != 2 {
		t.Fatalf("evidence=%+v content=%+v unknowns=%+v", got.Evidence, got.Content, got.Unknowns)
	}
	if got.Content[0].Date != "2024-01-01" || got.Content[0].Metrics["views"] != 20 {
		t.Fatalf("content: %+v", got.Content)
	}
	for _, u := range got.Unknowns {
		if strings.Contains(u.Reason, "fixture-key") {
			t.Fatal("provider echoed credentials into output")
		}
	}
}

func TestPaidCommunityFiltersUnrelatedMentions(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/reddit/search":
			return 200, `{"posts":[{"id":"good","title":"Example vs Acme","permalink":"/r/tools/comments/good/example","score":12},{"id":"bad","title":"Exampled things","permalink":"/r/tools/comments/bad/unrelated"}]}`
		case "/v1/reddit/post/comments":
			return 200, `{"comments":[{"body":"I use Example","author":"one","url":"https://www.reddit.com/r/tools/comments/good/example/comment"}]}`
		case "/v1/google/search":
			return 200, `{"results":[{"title":"Example | Product Hunt","url":"https://www.producthunt.com/products/example"},{"title":"Example unrelated","url":"https://www.producthunt.com.evil.invalid/products/example"}]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 8)
	got := p.Collect(context.Background(), "community", paidTarget(), model.Discovered{})
	if len(got.Evidence) != 2 || len(got.Content) != 2 || got.Content[0].ID != "reddit:good" || got.Content[1].Kind != "comment" {
		t.Fatalf("evidence=%+v content=%+v unknowns=%+v", got.Evidence, got.Content, got.Unknowns)
	}
	if data := got.Evidence[1].Data.(map[string]any); data["matches"] != 1 {
		t.Fatalf("Product Hunt: %+v", data)
	}
}

func TestPaidFoundersRequiresCompanyAssociation(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/google/search":
			return 200, `{"results":[{"title":"Ada Example - Founder of Example","url":"https://www.linkedin.com/in/ada"},{"title":"Ben Other - Founder at Other","description":"I once used Example","url":"https://www.linkedin.com/in/ben"}]}`
		case "/v1/linkedin/profile":
			return 200, `{"name":"Ada Example","followers":80,"recentPosts":[{"title":"I launched a new product","link":"https://www.linkedin.com/posts/ada_123","datePublished":"2026-09-21T00:00:00Z"}]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 8)
	got := p.Collect(context.Background(), "founders", paidTarget(), model.Discovered{})
	if len(got.Evidence) != 1 || len(got.Content) != 1 || got.Content[0].Author != "Ada Example" || strings.Contains(fmt.Sprint(got.Evidence[0].Data), "Ben Other") {
		t.Fatalf("evidence=%+v content=%+v unknowns=%+v", got.Evidence, got.Content, got.Unknowns)
	}
}

func TestPaidTikTokUsesDocumentedAdField(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/tiktok/profile":
			return 200, `{"stats":{"followerCount":250},"user":{"signature":"Description"}}`
		case "/v3/tiktok/profile/videos":
			return 200, `{"aweme_list":[{"aweme_id":"1","is_ad":true,"create_time":1704067200,"statistics":{"play_count":44},"video":{"dynamic_cover":{"url_list":["https://www.tiktok.com/image.png"]}}},{"aweme_id":"2","is_ad":false,"is_ads":true}]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 3)
	got := p.Collect(context.Background(), "social", paidTarget(), model.Discovered{Socials: map[string]string{"tiktok": "example"}})
	if len(got.Content) != 2 || len(got.Content[0].Tags) != 1 || got.Content[0].Thumb == "" || len(got.Content[1].Tags) != 0 {
		t.Fatalf("content: %+v", got.Content)
	}
}

func TestPaidDoesNotFollowRedirectsWithProviderCredentials(t *testing.T) {
	p := NewPaid(&http.Client{Transport: paidTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.scrapecreators.com" {
			t.Fatal("provider redirected a credentialed request to another origin")
		}
		return &http.Response{StatusCode: 302, Body: http.NoBody, Header: http.Header{"Location": {"https://other.example/"}}, Request: r}, nil
	})}, "fixture-key", 1)
	got := p.Collect(context.Background(), "founders", paidTarget(), model.Discovered{})
	if len(got.Unknowns) != 1 || !strings.Contains(got.Unknowns[0].Reason, "HTTP 302") {
		t.Fatalf("expected redirect to be reported as unavailable: %+v", got)
	}
}

func TestPaidCommunityReadsNestedRedditReplies(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/reddit/search":
			return 200, `{"posts":[{"id":"1","title":"Example vs Acme","url":"https://www.reddit.com/r/a/comments/1/example/"}]}`
		case "/v1/reddit/post/comments":
			return 200, `{"comments":[{"body":"Yes","url":"https://www.reddit.com/r/a/comments/1/example/c1/","replies":{"items":[{"body":"Example is a better option","url":"https://www.reddit.com/r/a/comments/1/example/c2/"}]}}]}`
		case "/v1/google/search":
			return 200, `{"results":[]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 8)
	got := p.Collect(context.Background(), "community", paidTarget(), model.Discovered{})
	if len(got.Content) != 2 || got.Content[1].Text != "Example is a better option" {
		t.Fatalf("nested comments were missed: %+v", got.Content)
	}
}

func TestPaidDiscoversTikTokFromDocumentedUsersArray(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v1/google/search":
			return 200, `{"results":[]}`
		case "/v1/tiktok/search/users":
			return 200, `{"users":[{"user_info":{"unique_id":"example","nickname":"Example","custom_verify":"Verified account"}}]}`
		case "/v1/tiktok/profile":
			return 200, `{"user":{"nickname":"Example"},"stats":{"followerCount":250}}`
		case "/v3/tiktok/profile/videos":
			return 200, `{"aweme_list":[]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 12)
	got := p.Collect(context.Background(), "social", paidTarget(), model.Discovered{})
	if len(got.Evidence) != 2 || got.Evidence[1].Source != "tiktok" {
		t.Fatalf("TikTok discovery: %+v", got)
	}
}

func TestPaidSeparateProfileAndFeedEndpoints(t *testing.T) {
	p := NewPaid(paidFixture(t, func(r *http.Request) (int, string) {
		switch r.URL.Path {
		case "/v2/instagram/user/posts":
			return 200, `{"items":[{"pk":"1","code":"abc","like_count":12}]}`
		case "/v1/instagram/profile":
			return 200, `{"data":{"user":{"biography":"Example official","edge_followed_by":{"count":54}}}}`
		case "/v1/linkedin/company":
			return 200, `{"name":"Example","followers":120}`
		case "/v1/linkedin/company/posts":
			return 200, `{"posts":[{"id":"1","url":"https://www.linkedin.com/posts/example_123","text":"Fresh post"}]}`
		default:
			return 404, `{}`
		}
	}), "fixture-key", 8)
	got := p.Collect(context.Background(), "social", paidTarget(), model.Discovered{Socials: map[string]string{"instagram": "example", "linkedin": "example"}})
	if len(got.Evidence) != 2 || len(got.Content) != 2 || got.Evidence[0].Data.(map[string]any)["followers"] != float64(54) || got.Content[1].Text != "Fresh post" {
		t.Fatalf("profile/feed merge: %+v", got)
	}
}

func TestPaidCacheReusesWithinTTLWithoutCreditsAndIsolatesAccounts(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	client := paidFixture(t, func(_ *http.Request) (int, string) {
		calls++
		return 200, `{"success":true,"results":[{"title":"Example"}],"credits_remaining":300}`
	})
	const endpoint = "/v1/google/search"
	q := url.Values{"query": {"Example"}}
	now := time.Now()
	first := NewPaid(client, "fixture-key", 1)
	first.now = func() time.Time { return now }
	first.SetCacheDir(dir)
	if _, err := first.get(context.Background(), endpoint, q); err != nil {
		t.Fatal(err)
	}
	second := NewPaid(client, "fixture-key", 0)
	second.now = func() time.Time { return now.Add(time.Hour) }
	second.SetCacheDir(dir)
	if got, err := second.get(context.Background(), endpoint, q); err != nil || len(paidItems(got["results"])) != 1 {
		t.Fatalf("cache did not replay request with zero budget: %v %v", got, err)
	}
	if calls != 1 || *second.Spent() != 0 {
		t.Fatalf("cache hit called provider or consumed credits: %d / %d", calls, *second.Spent())
	}
	differentKey := NewPaid(client, "different-key", 0)
	differentKey.SetCacheDir(dir)
	if _, err := differentKey.get(context.Background(), endpoint, q); err == nil {
		t.Fatal("account-specific cache allowed a different key to read results")
	}
	expired := NewPaid(client, "fixture-key", 1)
	expired.now = func() time.Time { return now.Add(24*time.Hour + time.Second) }
	expired.SetCacheDir(dir)
	if _, err := expired.get(context.Background(), endpoint, q); err != nil || calls != 2 || *expired.Spent() != 1 {
		t.Fatalf("expired result was not refreshed: %d calls, %d credits, %v", calls, *expired.Spent(), err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("cache file count: %v %v", files, err)
	}
	file := filepath.Join(dir, files[0].Name())
	if info, err := os.Stat(file); err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache file permissions: %v %v", info, err)
	}
	blob, err := os.ReadFile(file)
	if err != nil || strings.Contains(string(blob), "fixture-key") || strings.Contains(string(blob), "credits_remaining") || strings.Contains(files[0].Name(), "fixture-key") {
		t.Fatalf("cache persisted account data: %v", err)
	}
	if err := os.WriteFile(file, []byte("partial JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := second.get(context.Background(), endpoint, q); err == nil {
		t.Fatal("malformed disk cache was accepted")
	}
}

func TestPaidCacheDoesNotPersistCredentialsEchoedByProvider(t *testing.T) {
	dir := t.TempDir()
	client := paidFixture(t, func(_ *http.Request) (int, string) {
		return 200, `{"success":true,"results":[{"title":"fixture\u002dkey"}]}`
	})
	p := NewPaid(client, "fixture-key", 1)
	p.SetCacheDir(dir)
	result, err := p.get(context.Background(), "/v1/google/search", nil)
	if err == nil || result != nil {
		t.Fatalf("credential-echoing response must be rejected: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cache wrote a credential-echoing response: %v", err)
	}
}
