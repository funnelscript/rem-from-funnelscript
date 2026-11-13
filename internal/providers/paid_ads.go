package providers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

const (
	metaSearch = "/v1/facebook/adLibrary/search/companies"
	metaAds    = "/v1/facebook/adLibrary/company/ads"
	googleAds  = "/v1/google/company/ads"
	liAds      = "/v1/linkedin/ads/search"
	metaManual = "https://www.facebook.com/ads/library/"
)

func (p *Paid) ads(ctx context.Context, r *model.Result, target model.Target, d model.Discovered, brand string) {
	meta := paidQuery(metaManual, url.Values{"active_status": {"all"}, "ad_type": {"all"}, "country": {"ALL"}, "q": {brand}, "search_type": {"keyword_unordered"}})
	google := paidQuery("https://adstransparency.google.com/", url.Values{"region": {"anywhere"}, "domain": {target.Domain}})
	linkedin := paidQuery("https://www.linkedin.com/ad-library/search", url.Values{"accountOwner": {brand}})
	p.collectMeta(ctx, r, brand, d.Socials, meta)
	p.collectGoogle(ctx, r, target, google)
	p.collectLinkedInAds(ctx, r, brand, d.Socials["linkedin"], linkedin)
	paidUnknown(r, "paid_tiktok", paidQuery("https://library.tiktok.com/ads", url.Values{"adv_name": {brand}}), errors.New("TikTok ad library not checked by this adapter"))
	paidUnknown(r, "paid_reddit", "https://www.reddit.com/search/?q="+url.QueryEscape(brand), errors.New("Reddit ad library not checked by this adapter"))
}

func (p *Paid) collectMeta(ctx context.Context, r *model.Result, brand string, socials map[string]string, manual string) {
	queries := []string{brand}
	if ig := socials["instagram"]; ig != "" && !strings.EqualFold(ig, brand) && p.remaining() >= 4 {
		queries = append(queries, ig)
	}
	var page paidObject
	for _, q := range queries {
		_, candidates, err := paidFetchArray(ctx, p, metaSearch, url.Values{"query": {q}}, "searchResults")
		if err != nil {
			paidUnknown(r, "paid_meta", manual, err)
			return
		}
		page = paidMetaPage(candidates, brand, socials)
		if page != nil {
			break
		}
	}
	id := paidStr(page["page_id"])
	if id == "" {
		paidUnknown(r, "paid_meta", manual, errors.New("no Meta ad library page matched the brand with confidence"))
		return
	}
	query := url.Values{"pageId": {id}, "status": {"ALL"}, "sort_by": {"total_impressions"}}
	all, rows, err := paidFetchArray(ctx, p, metaAds, query, "results")
	if err != nil {
		paidUnknown(r, "paid_meta", manual, err)
		return
	}
	now := p.now().UTC()
	firstSample := rows
	total := paidValue(all["searchResultsCount"])
	data := map[string]any{
		"page":           map[string]any{"id": id, "name": paidStr(page["name"])},
		"totalInLibrary": total,
	}
	var active any
	if p.remaining() > 2 {
		response, activeRows, err := paidFetchArray(ctx, p, metaAds, url.Values{"pageId": {id}, "status": {"ACTIVE"}}, "results")
		if err != nil {
			paidUnknown(r, "paid_meta", manual, fmt.Errorf("active-ad count unavailable: %w", err))
		} else {
			rows = append(rows, activeRows...)
			active = paidValue(response["searchResultsCount"])
		}
	}
	if p.remaining() > 3 {
		query.Set("sort_by", "relevancy_monthly_grouped")
		_, recent, err := paidFetchArray(ctx, p, metaAds, query, "results")
		if err != nil {
			paidUnknown(r, "paid_meta", manual, fmt.Errorf("recent-ad sample unavailable: %w", err))
		} else {
			rows = append(rows, recent...)
		}
	}
	if cursor := paidStr(all["cursor"]); cursor != "" && len(cursor) <= 1500 && p.remaining() > 5 {
		query.Set("sort_by", "total_impressions")
		query.Set("cursor", cursor)
		_, next, err := paidFetchArray(ctx, p, metaAds, query, "results")
		if err == nil {
			rows = append(rows, next...)
		} else {
			paidUnknown(r, "paid_meta", manual, fmt.Errorf("additional ad sample unavailable: %w", err))
		}
	}
	ranked := make(map[string]bool)
	for _, raw := range firstSample[:min(8, len(firstSample))] {
		ranked[paidStr(paidAt(raw, "ad_archive_id"))] = true
	}
	items, stats := paidMetaItems(rows, ranked, now)
	for k, v := range stats {
		data[k] = v
	}
	data["activeNow"] = active
	data["sampled"] = len(items)
	title := fmt.Sprintf("Meta Ads Library: %d distinct ads in sample for %s", len(items), paidStr(page["name"]))
	pageLink := paidQuery(metaManual, url.Values{"active_status": {"all"}, "ad_type": {"all"}, "country": {"ALL"}, "view_all_page_id": {id}})
	paidObservation(r, "meta_ads", "observation", title, pageLink, data, items)
}

func paidMetaPage(rows []any, brand string, socials map[string]string) paidObject {
	var match paidObject
	best := 0
	tied := false
	for _, raw := range rows {
		row := paidObj(raw)
		if paidStr(row["page_id"]) == "" {
			continue
		}
		score := 0
		switch {
		case socials["instagram"] != "" && strings.EqualFold(strings.TrimPrefix(socials["instagram"], "@"), paidStr(row["ig_username"])):
			score = 3
		case socials["facebook"] != "" && strings.EqualFold(socials["facebook"], paidStr(row["page_alias"])):
			score = 2
		case paidNorm(brand) != "" && paidNorm(brand) == paidNorm(paidStr(row["name"])):
			score = 1
		}
		if score > best {
			match, best, tied = row, score, false
		} else if score > 0 && score == best && paidStr(match["page_id"]) != paidStr(row["page_id"]) {
			tied = true
		}
	}
	if tied {
		return nil
	}
	return match
}

func paidMetaItems(rows []any, ranked map[string]bool, now time.Time) ([]model.Content, map[string]any) {
	items := make([]model.Content, 0, min(len(rows), 100))
	seen := make(map[string]bool)
	formats, calls, landings, campaigns := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	customPages, campaignTokens, providerTokens := map[string]int{}, map[string]int{}, map[string]int{}
	active, launched, appStoreCount := 0, 0, 0
	var lastRan time.Time
	for _, raw := range rows {
		if len(items) >= 100 {
			break
		}
		ad := paidObj(raw)
		id := paidStr(ad["ad_archive_id"])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		snapshot := paidObj(ad["snapshot"])
		cards := paidItems(snapshot["cards"])
		firstCard := paidFirstObj(cards)
		title := paidClip(paidFirst(paidStr(snapshot["title"]), paidStr(firstCard["title"])), 160)
		body := paidClip(paidFirst(paidStr(paidAt(snapshot, "body", "text")), paidStr(firstCard["body"])), 500)
		format, cta := paidStr(snapshot["display_format"]), paidStr(snapshot["cta_text"])
		paidCount(formats, format)
		paidCount(calls, cta)
		links := append([]any{snapshot["link_url"]}, paidItems(snapshot["extra_links"])...)
		for _, link := range links {
			u, err := url.Parse(paidURL(paidStr(link)))
			if err != nil || u.Hostname() == "" {
				continue
			}
			paidCount(landings, strings.TrimPrefix(u.Hostname(), "www.")+u.EscapedPath())
			paidCount(campaigns, u.Query().Get("utm_campaign"))
			if u.Hostname() == "apps.apple.com" || u.Hostname() == "itunes.apple.com" {
				appStoreCount++
				paidCount(customPages, u.Query().Get("ppid"))
				paidCount(campaignTokens, u.Query().Get("ct"))
				paidCount(providerTokens, u.Query().Get("pt"))
			}
		}
		thumb, video := paidMetaMedia(snapshot, cards)
		status := ""
		start := paidDate(ad["start_date"])
		if start.IsZero() {
			start = paidDate(ad["start_date_string"])
		}
		end := paidDate(ad["end_date"])
		if running, ok := ad["is_active"].(bool); ok {
			if running {
				status, end = "active", now
				active++
			} else {
				status = "inactive"
			}
		}
		if end.After(lastRan) {
			lastRan = end
		}
		metrics := paidMetrics(map[string]any{"variants": ad["collation_count"]})
		if !start.IsZero() && !end.Before(start) {
			metrics["runningDays"] = max(1, float64(end.Unix()-start.Unix())/86400)
		}
		if !start.IsZero() && !start.After(now) && now.Sub(start) <= 30*24*time.Hour {
			launched++
		}
		tags := paidTags(format, cta)
		if ranked[id] {
			tags = append(tags, "top by impressions")
		}
		landing := paidFirst(paidURL(paidStr(ad["url"])), paidQuery(metaManual, url.Values{"id": {id}}))
		items = append(items, model.Content{ID: "meta_ads:" + id, Platform: "meta_ads", Kind: "ad", URL: landing, Title: title, Text: body, Thumb: thumb, Video: video, Author: paidStr(ad["page_name"]), Date: paidDateText(start), Status: status, Metrics: metrics, Tags: tags})
	}
	running := append([]model.Content(nil), items...)
	sort.SliceStable(running, func(i, j int) bool { return running[i].Metrics["runningDays"] > running[j].Metrics["runningDays"] })
	var activeLongest []model.Content
	for _, item := range running {
		if item.Status == "active" {
			activeLongest = append(activeLongest, item)
		}
	}
	return items, map[string]any{
		"activeInSample": active, "lastRan": paidDateText(lastRan), "launchedLast30d": launched,
		"formats": formats, "ctas": calls, "landingPages": landings, "utmCampaigns": campaigns,
		"longestRunning": paidAdViews(running[:min(8, len(running))]), "longestRunningActive": paidAdViews(activeLongest[:min(5, len(activeLongest))]),
		"mostShown":     paidAdViews(items[:min(8, len(items))]),
		"appStoreLinks": map[string]any{"linksToAppStore": appStoreCount, "customProductPages": customPages, "campaignTokens": campaignTokens, "providerTokens": providerTokens},
	}
}

func paidAdViews(items []model.Content) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"runningDays": item.Metrics["runningDays"], "active": item.Status == "active", "started": item.Date, "title": item.Title, "body": item.Text, "hasVideo": item.Video != "", "url": item.URL})
	}
	return out
}

func paidMetaMedia(snapshot paidObject, cards []any) (thumb, video string) {
	for _, raw := range append([]any{snapshot}, cards...) {
		card := paidObj(raw)
		v := paidFirstObj(paidItems(card["videos"]))
		img := paidFirstObj(paidItems(card["images"]))
		video = paidFirst(video, paidURL(paidFirst(paidStr(v["video_hd_url"]), paidStr(v["video_sd_url"]), paidStr(card["video_hd_url"]), paidStr(card["video_sd_url"]))))
		thumb = paidFirst(thumb, paidURL(paidFirst(paidStr(v["video_preview_image_url"]), paidStr(img["resized_image_url"]), paidStr(img["original_image_url"]), paidStr(card["resized_image_url"]), paidStr(card["original_image_url"]))))
	}
	return thumb, video
}

func (p *Paid) collectGoogle(ctx context.Context, r *model.Result, target model.Target, manual string) {
	if target.Domain == "" {
		paidUnknown(r, "paid_google", manual, errors.New("no website domain to check"))
		return
	}
	domain := target.Domain
	var rows []any
	var body paidObject
	var err error
	for _, candidate := range append([]string{domain}, target.Aliases[:min(2, len(target.Aliases))]...) {
		if candidate != domain && p.remaining() < 3 {
			break
		}
		res, ads, requestErr := paidFetchArray(ctx, p, googleAds, url.Values{"domain": {candidate}}, "ads")
		if requestErr != nil {
			err = requestErr
			break
		}
		body, rows, domain = res, ads, candidate
		if len(rows) > 0 {
			break
		}
	}
	if err != nil {
		paidUnknown(r, "paid_google", manual, err)
		if body == nil {
			return
		}
	}
	var content []model.Content
	formats, advertisers := map[string]int{}, map[string]int{}
	recent := 0
	now := p.now()
	for _, raw := range rows[:min(60, len(rows))] {
		ad := paidObj(raw)
		advertiser, format := paidStr(ad["advertiserName"]), paidStr(ad["format"])
		paidCount(formats, format)
		paidCount(advertisers, advertiser)
		last := paidDate(ad["lastShown"])
		if !last.IsZero() && !last.After(now) && now.Sub(last) <= 30*24*time.Hour {
			recent++
		}
		link := paidURL(paidStr(ad["adUrl"]))
		id := paidFirst(paidStr(ad["creativeId"]), link)
		content = append(content, model.Content{ID: "google_ads:" + id, Platform: "google_ads", Kind: "ad", URL: link, Title: paidClip(format+" ad by "+advertiser, 160), Thumb: paidURL(paidStr(ad["imageUrl"])), Author: advertiser, Date: paidDateText(paidDate(ad["firstShown"])), Tags: paidTags(format)})
	}
	paidObservation(r, "google_ads", "observation", fmt.Sprintf("Google Ads Transparency: %d ads returned for %s", len(rows), domain), paidQuery("https://adstransparency.google.com/", url.Values{"region": {"anywhere"}, "domain": {domain}}), map[string]any{"domain": domain, "estimate": paidValue(body["number_of_ads_estimate"]), "sampled": len(rows), "formats": formats, "advertisers": advertisers, "shownLast30d": recent, "sample": content[:min(8, len(content))]}, content)
}

func (p *Paid) collectLinkedInAds(ctx context.Context, r *model.Result, brand, slug, manual string) {
	body, rows, err := paidFetchArray(ctx, p, liAds, url.Values{"company": {brand}}, "ads")
	if err != nil {
		paidUnknown(r, "paid_linkedin", manual, err)
		return
	}
	var content []model.Content
	formats, calls := map[string]int{}, map[string]int{}
	matches := 0
	for _, raw := range rows[:min(100, len(rows))] {
		ad := paidObj(raw)
		name := paidStr(ad["advertiser"])
		profile := paidOnSite(paidStr(ad["advertiserLinkedinPage"]), "linkedin.com", "/company/")
		matchedSlug := slug != "" && profile != nil && strings.EqualFold(strings.Trim(profile.Path, "/"), "company/"+strings.Trim(slug, "/"))
		if !matchedSlug && (paidNorm(brand) == "" || paidNorm(brand) != paidNorm(name)) {
			continue
		}
		matches++
		format, cta := paidStr(ad["adType"]), paidStr(ad["cta"])
		paidCount(formats, format)
		paidCount(calls, cta)
		link := paidURL(paidStr(ad["url"]))
		id := paidFirst(paidStr(ad["id"]), link)
		image := paidURL(paidFirst(paidStr(ad["image"]), paidFirstString(ad["imageSources"]), paidFirstString(ad["thumbnailSources"])))
		content = append(content, model.Content{ID: "linkedin_ads:" + id, Platform: "linkedin_ads", Kind: "ad", URL: link, Title: paidClip(paidStr(ad["headline"]), 160), Text: paidClip(paidStr(ad["description"]), 500), Thumb: image, Video: paidURL(paidFirstString(ad["videoSources"])), Author: name, Date: paidDateText(paidDate(ad["startDate"])), Tags: paidTags(format, cta)})
	}
	paidObservation(r, "linkedin_ads", "observation", fmt.Sprintf("LinkedIn Ad Library: %d advertiser matches in the sample", matches), manual, map[string]any{"matched": matches, "fuzzyResults": paidValue(body["totalAds"]), "adTypes": formats, "ctas": calls, "sample": content[:min(8, len(content))]}, content)
}

func paidCount(counts map[string]int, key string) {
	if key != "" {
		counts[key]++
	}
}

func paidFirstString(raw any) string {
	a := paidItems(raw)
	if len(a) > 0 {
		return paidStr(a[0])
	}
	return ""
}

func paidFirstObj(items []any) paidObject {
	if len(items) > 0 {
		return paidObj(items[0])
	}
	return nil
}

func paidTags(values ...string) []string {
	var tags []string
	for _, v := range values {
		if v != "" {
			tags = append(tags, v)
		}
	}
	return tags
}
