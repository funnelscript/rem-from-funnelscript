package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"

	"github.com/funnelscript/rem-from-funnelscript/internal/collect"
	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

const (
	itunesURL  = "https://itunes.apple.com"
	waybackURL = "https://web.archive.org"
)

var (
	storeID       = regexp.MustCompile(`^[0-9]{6,}$`)
	storefront    = regexp.MustCompile(`^[a-z]{2}$`)
	playID        = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(?:\.[a-zA-Z][a-zA-Z0-9_]*)+$`)
	playDownloads = regexp.MustCompile(`(?i)>\s*([0-9][0-9.,]*[KMB]?\+)\s*</div>\s*<div(?:\s[^>]*|)>\s*Downloads\s*<`)
)

func getFree(ctx context.Context, get fetch.Getter, rawURL string) (fetch.Response, error) {
	response, err := get.Get(ctx, rawURL)
	if err != nil {
		return response, err
	}
	if response.Status < 200 || response.Status > 299 {
		return response, fmt.Errorf("HTTP %d", response.Status)
	}
	return response, nil
}

func freeJSON(ctx context.Context, get fetch.Getter, rawURL string, value any) error {
	response, err := getFree(ctx, get, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(response.Body, value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func appendUnknown(result *model.Result, channel, reason, manual string) {
	result.Unknowns = append(result.Unknowns, model.Unknown{Channel: channel, Reason: reason, ManualCheck: manual})
}

func appendObservation(result *model.Result, source, title, sourceURL string, data any) {
	result.Evidence = append(result.Evidence, model.Evidence{Source: source, Kind: "observation", Title: title, URL: sourceURL, Data: data})
}

func brief(value string, maxLength int) string {
	chars := []rune(strings.Join(strings.Fields(value), " "))
	if len(chars) <= maxLength {
		return string(chars)
	}
	return strings.TrimSpace(string(chars[:maxLength-1])) + "…"
}

func dayPrefix(value string) string {
	if len(value) >= 10 {
		if _, err := time.Parse(time.DateOnly, value[:10]); err == nil {
			return value[:10]
		}
	}
	return ""
}

func externalPage(rawURL string) *url.URL {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || fetch.ValidateURL(u) != nil {
		return nil
	}
	return u
}

func matchingHost(rawURL, domain string) bool {
	u := externalPage(rawURL)
	if u == nil || domain == "" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(strings.TrimSuffix(u.Hostname(), ".")), "www.")
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSuffix(domain, ".")), "www.")
	if host == domain {
		return true
	}
	if !strings.HasSuffix(host, "."+domain) && !strings.HasSuffix(domain, "."+host) {
		return false
	}
	hostRoot, hostErr := publicsuffix.EffectiveTLDPlusOne(host)
	domainRoot, domainErr := publicsuffix.EffectiveTLDPlusOne(domain)
	return hostErr == nil && domainErr == nil && hostRoot == domainRoot
}

func sellerOrigin(rawURL string) (origin, domain string) {
	u := externalPage(rawURL)
	if u == nil {
		return "", ""
	}
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSuffix(u.Hostname(), ".")), "www.")
	if net.ParseIP(domain) != nil {
		return "", ""
	}
	return u.Scheme + "://" + u.Host, domain
}

func validIDs(primary string, discovered []string, valid *regexp.Regexp, limit int) []string {
	out := []string{}
	for _, id := range append([]string{primary}, discovered...) {
		if valid.MatchString(id) && !slices.Contains(out, id) {
			out = append(out, id)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// History reads a bounded sample of the public CDX index and retains partial captures.
func History(ctx context.Context, get fetch.Getter, target model.Target) model.Result {
	var result model.Result
	if target.Domain == "" {
		appendUnknown(&result, "positioning_history", "No website domain was resolved", "")
		return result
	}
	manual := waybackURL + "/web/*/" + target.Domain
	query := url.Values{"url": {target.Domain}, "output": {"json"}, "fl": {"timestamp,statuscode"}, "filter": {"statuscode:200"}, "collapse": {"timestamp:6"}, "limit": {"400"}}
	var rawRows [][]string
	if err := freeJSON(ctx, get, waybackURL+"/cdx/search/cdx?"+query.Encode(), &rawRows); err != nil {
		appendUnknown(&result, "positioning_history", "Wayback index unavailable: "+err.Error(), manual)
		return result
	}
	if rawRows == nil || len(rawRows) > 0 && (len(rawRows[0]) == 0 || rawRows[0][0] != "timestamp") {
		appendUnknown(&result, "positioning_history", "Wayback returned an unrecognized index", manual)
		return result
	}
	rows := make([]string, 0, len(rawRows))
	for _, row := range rawRows {
		if len(row) < 2 || row[1] != "200" {
			continue
		}
		if _, err := time.Parse("20060102150405", row[0]); err == nil {
			rows = append(rows, row[0])
		}
	}
	slices.Sort(rows)
	rows = slices.Compact(rows)
	if len(rows) == 0 {
		if len(rawRows) > 1 {
			appendUnknown(&result, "positioning_history", "Wayback index had no usable timestamps", manual)
			return result
		}
		appendObservation(&result, "wayback", "Wayback returned no indexed homepage snapshots for this query", manual, map[string]any{"monthlySnapshots": 0, "history": []map[string]any{}})
		return result
	}

	chosen := chooseArchiveDates(rows, time.Now().UTC())
	history := make([]map[string]any, 0, len(chosen))
	for _, ts := range chosen {
		if ctx.Err() != nil {
			break
		}
		base := target.URL
		if base == "" {
			base = "https://" + target.Domain
		}
		base = strings.TrimRight(base, "/") + "/"
		archive := waybackURL + "/web/" + ts + "id_/" + base
		response, err := getFree(ctx, get, archive)
		if err != nil {
			continue
		}
		title, h1, description, ctas := collect.PageSummary(response.Body, base)
		if title == "" && h1 == "" && description == "" && len(ctas) == 0 {
			continue
		}
		history = append(history, map[string]any{
			"date": ts[:4] + "-" + ts[4:6] + "-" + ts[6:8], "title": brief(title, 140), "h1": brief(h1, 160), "description": brief(description, 220), "ctas": ctas, "url": archive,
		})
	}
	if len(history) < len(chosen) {
		appendUnknown(&result, "positioning_history", fmt.Sprintf("%d of %d selected Wayback snapshots could not be read", len(chosen)-len(history), len(chosen)), manual)
	}
	appendObservation(&result, "wayback", fmt.Sprintf("Wayback indexed %d monthly homepage snapshots and %d captures were read", len(rows), len(history)), manual, map[string]any{
		"firstSeen": rows[0], "monthlySnapshots": len(rows), "history": history, "sampleSize": len(chosen),
	})
	return result
}

func chooseArchiveDates(rows []string, now time.Time) []string {
	if len(rows) == 0 {
		return nil
	}
	chosen := []string{rows[0], rows[len(rows)-1]}
	for _, months := range []int{24, 12, 6} {
		// Use the first of the month so subtracting months from the 31st never skips a month.
		window := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -months, 0).Format("200601")
		for _, row := range rows {
			if row[:6] >= window {
				chosen = append(chosen, row)
				break
			}
		}
	}
	slices.Sort(chosen)
	return slices.Compact(chosen)
}

type appStoreEntry struct {
	TrackID       int64    `json:"trackId"`
	TrackName     string   `json:"trackName"`
	TrackViewURL  string   `json:"trackViewUrl"`
	SellerName    string   `json:"sellerName"`
	SellerURL     string   `json:"sellerUrl"`
	ArtworkURL    string   `json:"artworkUrl512"`
	Genres        []string `json:"genres"`
	Price         string   `json:"formattedPrice"`
	RatingCount   *int     `json:"userRatingCount"`
	Rating        *float64 `json:"averageUserRating"`
	ReleaseDate   string   `json:"releaseDate"`
	UpdateDate    string   `json:"currentVersionReleaseDate"`
	Version       string   `json:"version"`
	ReleaseNotes  string   `json:"releaseNotes"`
	Description   string   `json:"description"`
	Screenshots   []string `json:"screenshotUrls"`
	ContentRating string   `json:"contentAdvisoryRating"`
}

func getAppStore(ctx context.Context, get fetch.Getter, path string, query url.Values) ([]appStoreEntry, error) {
	var envelope struct {
		Results []appStoreEntry `json:"results"`
	}
	if err := freeJSON(ctx, get, itunesURL+path+"?"+query.Encode(), &envelope); err != nil {
		return nil, err
	}
	if envelope.Results == nil {
		return nil, errors.New("missing results array")
	}
	return envelope.Results, nil
}

// Apps resolves a store link to a developer website, then records the public listing facts.
func Apps(ctx context.Context, get fetch.Getter, target model.Target, discovered model.Discovered) (model.Target, model.Discovered, model.Result) {
	var result model.Result
	ids := validIDs(target.AppleID, discovered.AppStoreIDs, storeID, 3)
	brand := discovered.BrandName
	if brand == "" {
		brand = target.Brand
	}
	const manualApple = "https://www.apple.com/app-store/"
	if len(ids) == 0 && brand != "" && target.Domain != "" {
		candidates, err := getAppStore(ctx, get, "/search", url.Values{"entity": {"software"}, "limit": {"10"}, "term": {brand}})
		if err != nil {
			appendUnknown(&result, "app_store", "Apple App Store search unavailable: "+err.Error(), manualApple)
		}
		for _, candidate := range candidates {
			if candidate.TrackID <= 0 || candidate.TrackName == "" || !matchingHost(candidate.SellerURL, target.Domain) {
				continue
			}
			id := fmt.Sprint(candidate.TrackID)
			if !slices.Contains(ids, id) && len(ids) < 3 {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) > 0 {
		query := url.Values{"id": {strings.Join(ids, ",")}}
		if target.Kind == "app_store" {
			if store := externalPage(target.Input); store != nil {
				country, _, _ := strings.Cut(strings.TrimPrefix(strings.ToLower(store.Path), "/"), "/")
				if storefront.MatchString(country) {
					query.Set("country", country)
				}
			}
		}
		apps, err := getAppStore(ctx, get, "/lookup", query)
		if err != nil {
			appendUnknown(&result, "app_store", "Apple App Store lookup unavailable: "+err.Error(), manualApple)
		}
		for _, app := range apps {
			id := fmt.Sprint(app.TrackID)
			if !slices.Contains(ids, id) || app.TrackName == "" {
				continue
			}
			addAppleApp(&result, &target, &discovered, id, app)
		}
		if err == nil && len(result.Evidence) == 0 {
			appendUnknown(&result, "app_store", "No matching iOS listing was returned by the App Store lookup", manualApple)
		}
	}

	if androidIDs := validIDs(target.PlayID, discovered.PlayIDs, playID, 1); len(androidIDs) > 0 {
		addPlayApp(ctx, get, &result, &target, &discovered, androidIDs[0])
	}
	if len(result.Evidence) == 0 && len(result.Unknowns) == 0 {
		appendUnknown(&result, "app_store", "No matching iOS listing or Google Play link was discovered", manualApple)
	}
	return target, discovered, result
}

func addAppleApp(result *model.Result, target *model.Target, discovered *model.Discovered, id string, app appStoreEntry) {
	if !slices.Contains(discovered.AppStoreIDs, id) {
		discovered.AppStoreIDs = append(discovered.AppStoreIDs, id)
	}
	if target.URL == "" {
		target.URL, target.Domain = sellerOrigin(app.SellerURL)
	}
	if target.Brand == "" {
		target.Brand = brief(app.TrackName, 120)
	}
	sourceURL := fmt.Sprintf("https://apps.apple.com/app/id%s", id)
	if link := externalPage(app.TrackViewURL); link != nil && (link.Hostname() == "apps.apple.com" || link.Hostname() == "itunes.apple.com") {
		sourceURL = link.String()
	}
	if image := externalPage(app.ArtworkURL); image != nil {
		discovered.Logos = append(discovered.Logos, model.Logo{URL: image.String(), Source: "appstore"})
	}
	if name := strings.TrimSpace(app.SellerName); personalSellerName(name) {
		discovered.Founders = append(discovered.Founders, model.Founder{Name: name, Role: "App Store seller", URL: sourceURL})
	}
	data := map[string]any{
		"appId": id, "name": brief(app.TrackName, 120), "seller": brief(app.SellerName, 120), "sellerUrl": app.SellerURL,
		"genres": app.Genres, "price": brief(app.Price, 48), "firstReleased": app.ReleaseDate, "lastUpdated": app.UpdateDate, "version": brief(app.Version, 30),
		"releaseNotes": brief(app.ReleaseNotes, 500), "description": brief(app.Description, 1500), "screenshots": len(app.Screenshots), "contentRating": brief(app.ContentRating, 32),
	}
	title := "App Store: " + brief(app.TrackName, 120)
	if app.RatingCount != nil {
		data["ratingCount"] = *app.RatingCount
		title += fmt.Sprintf(", %d ratings", *app.RatingCount)
	}
	if app.Rating != nil {
		data["rating"] = *app.Rating
		title += fmt.Sprintf(", %.2f stars", *app.Rating)
	}
	appendObservation(result, "appstore", title, sourceURL, data)
}

func personalSellerName(name string) bool {
	words := strings.Fields(name)
	if len(words) != 2 {
		return false
	}
	for _, word := range words {
		for pos, ch := range word {
			if pos == 0 && !unicode.IsUpper(ch) || pos > 0 && !unicode.IsLower(ch) {
				return false
			}
		}
	}
	return true
}

func addPlayApp(ctx context.Context, get fetch.Getter, result *model.Result, target *model.Target, discovered *model.Discovered, id string) {
	manual := "https://play.google.com/store/apps/details?" + url.Values{"id": {id}, "hl": {"en_US"}}.Encode()
	response, err := getFree(ctx, get, manual)
	if err != nil {
		appendUnknown(result, "app_store", "Google Play listing unavailable: "+err.Error(), manual)
		return
	}
	label, heading, description, _ := collect.PageSummary(response.Body, manual)
	name := strings.TrimSpace(strings.TrimSuffix(label, " - Apps on Google Play"))
	website := playWebsite(response.Body)
	if heading != "" && label == "" {
		name = heading
	}
	if name == "" || !strings.HasSuffix(label, " - Apps on Google Play") && !playDownloads.Match(response.Body) && website == "" {
		appendUnknown(result, "app_store", "Google Play returned a page without a recognizable app listing", manual)
		return
	}
	if target.Brand == "" {
		target.Brand = brief(name, 120)
	}
	if !slices.Contains(discovered.PlayIDs, id) {
		discovered.PlayIDs = append(discovered.PlayIDs, id)
	}
	if target.URL == "" {
		target.URL, target.Domain = sellerOrigin(website)
	}
	title := "Google Play: " + brief(name, 120)
	data := map[string]any{"playId": id, "name": brief(name, 120), "description": brief(description, 600)}
	if website != "" {
		data["sellerUrl"] = website
	}
	if count := playDownloads.FindSubmatch(response.Body); len(count) > 1 {
		data["installs"] = string(count[1])
		title += ", " + string(count[1]) + " downloads"
	}
	appendObservation(result, "appstore", title, manual, data)
}

func playWebsite(body []byte) string {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var textContent func(*html.Node) string
	textContent = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var out strings.Builder
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			out.WriteString(textContent(child))
			out.WriteByte(' ')
		}
		return out.String()
	}
	var find func(*html.Node) string
	find = func(node *html.Node) string {
		if node.Type == html.ElementNode && node.Data == "a" {
			var href, label string
			for _, attr := range node.Attr {
				switch attr.Key {
				case "href":
					href = attr.Val
				case "aria-label":
					label = attr.Val
				}
			}
			label = strings.ToLower(strings.TrimSpace(label))
			if strings.HasPrefix(label, "website ") || strings.EqualFold(strings.TrimSpace(textContent(node)), "website") {
				if u := externalPage(href); u != nil && u.Hostname() != "play.google.com" {
					return u.String()
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if found := find(child); found != "" {
				return found
			}
		}
		return ""
	}
	return find(root)
}
