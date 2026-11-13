package collect

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"remgo/internal/fetch"
	"remgo/internal/model"
)

type fixture map[string]fetch.Response

func (f fixture) Get(_ context.Context, rawURL string) (fetch.Response, error) {
	if response, ok := f[rawURL]; ok {
		if response.URL == "" {
			response.URL = rawURL
		}
		return response, nil
	}
	return fetch.Response{}, fmt.Errorf("unavailable: %s", rawURL)
}

func page(body string) fetch.Response {
	return fetch.Response{Status: 200, Body: []byte(body)}
}

func TestWebsiteCollectsObservedSignalsAcrossRedirectAndPublicPages(t *testing.T) {
	web := fixture{
		"https://old.example": {
			URL:    "https://www.new.example/",
			Status: 200,
			Body: []byte(`<html lang="en"><head><title>New Brand | Home</title>
<meta property="og:site_name" content="New Brand"><link rel="apple-touch-icon" href="/icon.png">
<script type="application/ld+json">{"@type":"Organization","founder":{"@type":"Person","name":"Alex Stone"},"sameAs":["https://www.youtube.com/@newbrand"]}</script>
<script src="https://www.googletagmanager.com/gtm.js?id=GTM-HELLO"></script></head>
<body><h1>Ship faster</h1><a href="/pricing">Pricing</a><a href="/about">About</a><a href="/signup">Start free</a>
<a href="/partners">Partners</a><a href="https://www.linkedin.com/company/newbrand">LinkedIn</a>
<a href="https://apps.apple.com/us/app/new/id123456789">Apple</a><a href="https://play.google.com/store/apps/details?id=app.newbrand">Play</a></body></html>`),
		},
		"https://www.new.example/pricing":                      page(`<h1>Plans</h1><p>$24 per seat / month. Try a 14-day free trial.</p>`),
		"https://www.new.example/about":                        page(`<h1>About</h1><p>Jamie Moss founded New Brand.</p>`),
		"https://www.new.example/partners":                     page(`<h1>Partners</h1><p>Earn 20% recurring commission.</p>`),
		"https://www.googletagmanager.com/gtm.js?id=GTM-HELLO": page(`fbq("init","1234567890");`),
	}
	target, found, result := Website(context.Background(), web, model.Target{Kind: "website", URL: "https://old.example", Domain: "old.example"})
	if target.Domain != "new.example" || target.Brand != "New Brand" || len(target.Aliases) != 1 || target.Aliases[0] != "old.example" {
		t.Fatalf("redirect did not update target: %+v", target)
	}
	if found.Socials["linkedin"] != "newbrand" || found.Socials["youtube"] != "@newbrand" || len(found.Logos) != 1 || len(found.AppStoreIDs) != 1 || len(found.PlayIDs) != 1 || len(found.Founders) != 2 {
		t.Fatalf("missing discovery: %+v", found)
	}
	assertEvidenceData(t, result, "Homepage positioning:", `"Ship faster"`, `"Start free"`)
	assertEvidenceData(t, result, "Monetization:", `"perSeat":true`, `"trialDays":14`, `"$24"`)
	assertEvidenceData(t, result, "Ad/attribution pixels:", `"1234567890"`, `"GTM-HELLO"`)
	assertEvidenceData(t, result, "Affiliate / partner", `"20% recurring commission"`)
}

func TestWebsiteRecordsUnavailableGTMWithoutClaimingItWasScanned(t *testing.T) {
	web := fixture{
		"https://company.example": page(`<h1>Company</h1><script src="https://www.googletagmanager.com/gtm.js?id=GTM-HELLO"></script>`),
	}
	_, _, result := Website(context.Background(), web, model.Target{Kind: "website", URL: "https://company.example", Domain: "company.example"})
	assertEvidenceData(t, result, "No ad pixels detected", `"gtmContainersScanned":[]`)
	if len(result.Unknowns) != 1 || result.Unknowns[0].Channel != "tracking" {
		t.Fatalf("missing GTM unknown: %+v", result.Unknowns)
	}
}

func TestSEOSeparatesCheckedFilesFromUnavailableSources(t *testing.T) {
	base := "https://company.example"
	web := fixture{
		base + "/robots.txt": page("User-agent: GPTBot\nDisallow: /\nUser-agent: ClaudeBot\nDisallow: /private\nSitemap: " + base + "/main.xml"),
		base + "/llms.txt":   {Status: 404},
		base + "/main.xml":   page(`<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>https://company.example/child.xml</loc></sitemap><sitemap><loc>https://outside.example/other.xml</loc></sitemap></sitemapindex>`),
		base + "/child.xml":  page(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://company.example/en/integrations/email?x=1&amp;y=2</loc></url><url><loc>https://company.example/compare/rem-vs-other</loc></url></urlset>`),
	}
	result := SEO(context.Background(), web, model.Target{URL: base, Domain: "company.example"})
	assertEvidenceData(t, result, "Sitemap:", `"totalUrls":2`, `"comparison":1`, `"integrations":1`, `"en"`)
	assertEvidenceData(t, result, "AI-search stance:", `"GPTBot":"blocked"`, `"ClaudeBot":"partial"`, `"llmsStatus":"absent"`)
	missing := SEO(context.Background(), fixture{}, model.Target{URL: base, Domain: "company.example"})
	if len(missing.Unknowns) < 3 {
		t.Fatalf("blocked access was reported as a negative: %+v", missing)
	}
}

func assertEvidenceData(t *testing.T, result model.Result, prefix string, wanted ...string) {
	t.Helper()
	for _, e := range result.Evidence {
		if strings.HasPrefix(e.Title, prefix) {
			b, err := json.Marshal(e.Data)
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range wanted {
				if !strings.Contains(string(b), value) {
					t.Errorf("evidence %q missing %s in %s", e.Title, value, b)
				}
			}
			return
		}
	}
	t.Fatalf("missing evidence starting %q: %+v", prefix, result.Evidence)
}

func TestWebsiteMentionsAreNotTrackingTags(t *testing.T) {
	base := "https://company.example"
	web := fixture{base: page(`<h1>Company</h1><a href="https://rb2b.com/">A referenced product</a>
<script type="application/ld+json">{"url":"https://tag.clearbitscripts.com/example"}</script>`)}
	_, _, result := Website(context.Background(), web, model.Target{Kind: "website", URL: base, Domain: "company.example"})
	for _, evidence := range result.Evidence {
		if strings.HasPrefix(evidence.Title, "Marketing stack:") {
			t.Fatalf("navigation links and linked metadata were treated as installed tags: %+v", evidence)
		}
	}
}

func TestPageSummaryKeepsHeadingBoundaries(t *testing.T) {
	title, heading, description, ctas := PageSummary([]byte(`<title>Market</title><meta name="description" content="Read more"><h1><span>See</span><span>growth</span></h1><button>Book a demo</button>`), "https://company.example")
	if title != "Market" || heading != "See growth" || description != "Read more" || len(ctas) != 1 || ctas[0] != "Book a demo" {
		t.Fatalf("unexpected page summary: %q %q %q %v", title, heading, description, ctas)
	}
}

func TestRobotsSpecificEmptyRuleOverridesWildcard(t *testing.T) {
	result := parseRobots("User-agent: *\nDisallow: /\nUser-agent: GPTBot\nDisallow:\nUser-agent: ClaudeBot\nDisallow: /\nAllow: /docs/\n")
	if result.AIBots["GPTBot"] != "allowed" || result.AIBots["ClaudeBot"] != "partial" || result.AIBots["CCBot"] != "blocked" {
		t.Fatalf("unexpected directives: %+v", result.AIBots)
	}
}

func TestSEOReadsCompressedSitemapWithinCollectionLimits(t *testing.T) {
	base := "https://company.example"
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`<urlset><url><loc>https://company.example/templates/meeting</loc></url></urlset>`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	web := fixture{
		base + "/robots.txt":     page("Sitemap: " + base + "/sitemap.xml.gz"),
		base + "/llms.txt":       {Status: 404},
		base + "/sitemap.xml.gz": {Status: 200, Body: compressed.Bytes()},
	}
	result := SEO(context.Background(), web, model.Target{URL: base, Domain: "company.example"})
	assertEvidenceData(t, result, "Sitemap:", `"totalUrls":1`, `"templates":1`)
}
