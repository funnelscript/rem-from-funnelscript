package providers

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

const (
	appleAdsURL      = "https://adrepository.apple.com/"
	appleAdsPageSize = 50
	appleAdsScope    = "Apple’s public repository covers ads served in the EU. Other markets and individual keywords are outside this data."
)

type appleAd struct {
	ID              string                     `json:"adId"`
	AppID           int64                      `json:"appId"`
	AppName         string                     `json:"appName"`
	Developer       string                     `json:"developerName"`
	Placement       string                     `json:"placement"`
	Format          string                     `json:"format"`
	Country         string                     `json:"countryOrRegion"`
	Audience        map[string]json.RawMessage `json:"audienceRefinement"`
	FirstImpression string                     `json:"firstImpressionDate"`
	LastImpression  string                     `json:"lastImpressionDate"`
	Language        string                     `json:"defaultLanguageTag"`
	Banner          struct {
		Subtitle  string `json:"subtitle"`
		Promotion string `json:"promotionalText"`
		Icon      string `json:"iconPictureUrl"`
	} `json:"adBanner"`
	Assets []appleAdAsset `json:"adAssets"`
}

type appleAdAsset struct {
	Video   string `json:"videoUrl"`
	Picture string `json:"pictureUrl"`
}

type appleAdsPage struct {
	Ads        []appleAd `json:"data"`
	Start      string    `json:"dataStartDate"`
	End        string    `json:"dataEndDate"`
	Pagination struct {
		Total *int `json:"totalResults"`
	} `json:"pagination"`
}

func getAppleAdsPage(ctx context.Context, get fetch.Getter, id string, offset int) (appleAdsPage, error) {
	query := url.Values{
		"ql":     {"type==APP;id==" + id + ";datePreset==LAST_YEAR"},
		"limit":  {strconv.Itoa(appleAdsPageSize)},
		"offset": {strconv.Itoa(offset)},
	}
	var page appleAdsPage
	if err := freeJSON(ctx, get, appleAdsURL+"api/v1/ad-repository-ads?"+query.Encode(), &page); err != nil {
		return page, err
	}
	if page.Ads == nil || page.Pagination.Total == nil || *page.Pagination.Total < len(page.Ads) || *page.Pagination.Total < 0 {
		return page, errors.New("missing or inconsistent ads data and pagination")
	}
	return page, nil
}

// AppleAds records the advertised EU scope and keeps totals separate from sampled creative metrics.
func AppleAds(ctx context.Context, get fetch.Getter, target model.Target, discovered model.Discovered) model.Result {
	var result model.Result
	ids := validIDs(target.AppleID, discovered.AppStoreIDs, storeID, 2)
	if len(ids) == 0 {
		appendUnknown(&result, "paid_apple", "No iOS listing was found for an Apple Ad Repository lookup", appleAdsURL)
		return result
	}
	for _, id := range ids {
		addAppleAds(ctx, get, &result, id)
	}
	brand := discovered.BrandName
	if brand == "" {
		brand = target.Brand
	}
	if len([]rune(brand)) >= 3 && len(result.Evidence) > 0 {
		addAppleAdsSimilarNames(ctx, get, &result, ids, brand)
	}
	return result
}

func addAppleAds(ctx context.Context, get fetch.Getter, result *model.Result, id string) {
	first, err := getAppleAdsPage(ctx, get, id, 0)
	if err != nil {
		appendUnknown(result, "paid_apple", fmt.Sprintf("Apple Ad Repository unavailable for app %s: %v", id, err), appleAdsURL)
		return
	}
	total := *first.Pagination.Total
	data := map[string]any{"appId": id, "totalAdsLastYear": total, "window": map[string]string{"start": dayPrefix(first.Start), "end": dayPrefix(first.End)}, "note": appleAdsScope}
	if total == 0 {
		appendObservation(result, "apple_ads", fmt.Sprintf("Apple Search Ads: no ads returned for app %s in the EU repository over the last year", id), appleAdsURL, data)
		return
	}
	pages := []appleAdsPage{first}
	lastOffset := (total - 1) / appleAdsPageSize
	for _, offset := range slices.Compact([]int{lastOffset / 2, lastOffset}) {
		if offset == 0 || ctx.Err() != nil {
			continue
		}
		page, err := getAppleAdsPage(ctx, get, id, offset)
		if err != nil {
			appendUnknown(result, "paid_apple", fmt.Sprintf("Apple Ad Repository sample page %d unavailable for app %s: %v", offset, id, err), appleAdsURL)
			continue
		}
		pages = append(pages, page)
	}
	seen := map[string]bool{}
	ads := make([]appleAd, 0, 3*appleAdsPageSize)
	for _, page := range pages {
		for _, ad := range page.Ads {
			if ad.ID == "" || seen[ad.ID] || strconv.FormatInt(ad.AppID, 10) != id {
				continue
			}
			seen[ad.ID] = true
			ads = append(ads, ad)
		}
	}
	if len(ads) == 0 {
		appendObservation(result, "apple_ads", fmt.Sprintf("Apple Search Ads: repository reported %d EU ads for app %s over the last year", total, id), appleAdsURL, data)
		appendUnknown(result, "paid_apple", "Apple Ad Repository sample contained no usable creative rows for app "+id, appleAdsURL)
		return
	}
	slices.SortStableFunc(ads, func(a, b appleAd) int { return cmp.Compare(b.LastImpression, a.LastImpression) })
	name := brief(ads[0].AppName, 100)
	if name == "" {
		name = id
	}
	end, dateErr := time.Parse(time.DateOnly, dayPrefix(first.End))
	if dateErr != nil {
		appendUnknown(result, "paid_apple", "Apple Ad Repository omitted a usable reporting window end for app "+id, appleAdsURL)
	}
	data["appName"], data["developer"], data["sampled"] = name, brief(ads[0].Developer, 120), len(ads)
	metrics := aggregateAppleAds(ads, end)
	for key, value := range metrics {
		data[key] = value
	}
	evidenceID := "apple-app:" + id
	title := fmt.Sprintf("Apple Search Ads: %d EU ads for %s over the last year, %d creative records sampled", total, name, len(ads))
	result.Evidence = append(result.Evidence, model.Evidence{ID: evidenceID, Source: "apple_ads", Kind: "observation", Title: title, URL: appleAdsURL, Data: data})
	result.Content = append(result.Content, appleAdsCards(ads, end, evidenceID)...)
}

func aggregateAppleAds(ads []appleAd, end time.Time) map[string]any {
	countries, placements, formats := map[string]int{}, map[string]int{}, map[string]int{}
	messages, promotions, languages := map[string]int{}, map[string]int{}, map[string]int{}
	started := map[string]int{}
	targeting := map[string]int{"age": 0, "gender": 0, "location": 0, "customerType": 0, "of": len(ads)}
	firstImpression := ""
	live, withVideo := 0, 0
	for _, ad := range ads {
		for _, dim := range []struct {
			m   map[string]int
			key string
		}{
			{countries, ad.Country}, {placements, ad.Placement}, {formats, ad.Format}, {messages, brief(ad.Banner.Subtitle, 80)}, {promotions, brief(ad.Banner.Promotion, 120)}, {languages, ad.Language},
		} {
			if dim.key != "" {
				dim.m[dim.key]++
			}
		}
		if date := dayPrefix(ad.FirstImpression); date != "" {
			started[date[:7]]++
			if firstImpression == "" || date < firstImpression {
				firstImpression = date
			}
		}
		for _, dimension := range []string{"age", "gender", "location", "customerType"} {
			raw := string(ad.Audience[dimension+"Target"])
			if raw == "true" || raw == `"true"` {
				targeting[dimension]++
			}
		}
		if appleAdStatus(ad, end) == "active" {
			live++
		}
		if slices.ContainsFunc(ad.Assets, func(x appleAdAsset) bool { return externalPage(x.Video) != nil }) {
			withVideo++
		}
	}
	metrics := map[string]any{
		"countries": countries, "placements": placements, "formats": formats, "messages": messages, "promotionalTexts": promotions,
		"languages": languages, "firstImpressionInSample": firstImpression, "startedByMonth": started, "targeting": targeting, "withVideo": withVideo,
	}
	if !end.IsZero() {
		metrics["liveInSample"] = live
		metrics["recencyRule"] = "Sampled ads whose last recorded impression is within seven days of the repository window end."
	}
	return metrics
}

func appleAdStatus(ad appleAd, end time.Time) string {
	last, err := time.Parse(time.DateOnly, dayPrefix(ad.LastImpression))
	if err != nil || end.IsZero() || last.After(end) {
		return ""
	}
	if end.Sub(last) <= 7*24*time.Hour {
		return "active"
	}
	return "inactive"
}

func appleAdsCards(ads []appleAd, end time.Time, evidenceID string) []model.Content {
	cards := make([]model.Content, 0, min(16, len(ads)))
	seen := map[string]bool{}
	for _, ad := range ads {
		var image, video string
		for _, asset := range ad.Assets {
			if image == "" && externalPage(asset.Picture) != nil {
				image = asset.Picture
			}
			if video == "" && externalPage(asset.Video) != nil {
				video = asset.Video
			}
		}
		key := strings.Join([]string{ad.Format, ad.Banner.Subtitle, image, video}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		if image == "" && externalPage(ad.Banner.Icon) != nil {
			image = ad.Banner.Icon
		}
		title := strings.TrimSpace(ad.AppName)
		if ad.Banner.Subtitle != "" {
			title += ": " + ad.Banner.Subtitle
		}
		card := model.Content{
			ID: "apple:" + ad.ID, Platform: "apple_ads", Kind: "ad", URL: appleAdsURL, Title: brief(title, 180),
			Text:  brief(strings.Join([]string{ad.Format, strings.TrimPrefix(ad.Placement, "APPSTORE_"), ad.Country}, " · "), 120),
			Thumb: image, Video: video, Author: brief(ad.Developer, 120), Date: dayPrefix(ad.FirstImpression), Status: appleAdStatus(ad, end),
			Tags: []string{ad.Country, ad.Format}, Evidence: evidenceID,
		}
		first, firstErr := time.Parse(time.DateOnly, card.Date)
		last, lastErr := time.Parse(time.DateOnly, dayPrefix(ad.LastImpression))
		if firstErr == nil && lastErr == nil && !last.Before(first) {
			card.Metrics = map[string]float64{"runningDays": max(1, last.Sub(first).Hours()/24)}
		}
		cards = append(cards, card)
		if len(cards) >= 16 {
			break
		}
	}
	return cards
}

func addAppleAdsSimilarNames(ctx context.Context, get fetch.Getter, result *model.Result, ids []string, brand string) {
	query := url.Values{"name": {brief(brand, 120)}, "types": {"APP"}, "limit": {"20"}}
	var envelope struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := freeJSON(ctx, get, appleAdsURL+"api/v1/ad-repository-entities?"+query.Encode(), &envelope); err != nil {
		appendUnknown(result, "paid_apple", "Apple Ad Repository name search unavailable: "+err.Error(), appleAdsURL)
		return
	}
	if envelope.Data == nil {
		appendUnknown(result, "paid_apple", "Apple Ad Repository name search did not return a data array", appleAdsURL)
		return
	}
	others := []map[string]any{}
	seen := map[string]bool{}
	for _, entry := range envelope.Data {
		id := strconv.FormatInt(entry.ID, 10)
		if entry.ID <= 0 || entry.Name == "" || slices.Contains(ids, id) || seen[id] {
			continue
		}
		seen[id] = true
		others = append(others, map[string]any{"appId": id, "name": brief(entry.Name, 100), "appStore": "https://apps.apple.com/app/id" + id})
		if len(others) >= 12 {
			break
		}
	}
	if len(others) != 0 {
		appendObservation(result, "apple_ads", fmt.Sprintf("Apple Ad Repository name search also returned %d other apps for %q", len(others), brief(brand, 70)), appleAdsURL, map[string]any{"apps": others, "note": "A name match does not establish brand bidding, affiliation, or imitation."})
	}
}
