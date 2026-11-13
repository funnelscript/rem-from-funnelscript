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

type paidSocialPlan struct {
	platform string
	handle   string
	manual   string
}

func (p *Paid) social(ctx context.Context, r *model.Result, target model.Target, d model.Discovered, brand string) {
	handles := make(map[string]string)
	for key, value := range d.Socials {
		handles[key] = strings.TrimPrefix(strings.TrimSpace(value), "@")
	}
	discovered := p.paidDiscoverSocials(ctx, r, target, brand, handles)
	plans := []paidSocialPlan{
		{"tiktok", handles["tiktok"], "https://www.tiktok.com/@" + url.PathEscape(handles["tiktok"])},
		{"instagram", handles["instagram"], "https://www.instagram.com/" + url.PathEscape(handles["instagram"]) + "/"},
		{"youtube", handles["youtube"], paidYouTubeURL(handles["youtube"])},
		{"x", handles["x"], "https://x.com/" + url.PathEscape(handles["x"])},
		{"linkedin", handles["linkedin"], "https://www.linkedin.com/company/" + url.PathEscape(handles["linkedin"])},
	}
	for _, plan := range plans {
		channel := "organic_" + plan.platform
		if plan.handle == "" {
			paidUnknown(r, channel, "", errors.New("no confidently identified account"))
			continue
		}
		data, posts, err := p.socialProfile(ctx, r, plan)
		if err != nil {
			paidUnknown(r, channel, plan.manual, err)
			continue
		}
		data["handle"] = plan.handle
		data["handleSource"] = paidFirst(discovered[plan.platform], "website link or supplied handle")
		data["sampledPosts"] = len(posts)
		ranked := append([]model.Content(nil), posts...)
		sort.SliceStable(ranked, func(i, j int) bool {
			return ranked[i].Metrics["views"]+ranked[i].Metrics["likes"] > ranked[j].Metrics["views"]+ranked[j].Metrics["likes"]
		})
		data["topPosts"] = ranked[:min(5, len(ranked))]
		data["medianViews"] = paidMedian(posts, "views")
		data["medianLikes"] = paidMedian(posts, "likes")
		if plan.platform == "tiktok" || plan.platform == "instagram" {
			data["postsPerWeek"], data["lastPostDaysAgo"] = paidActivity(posts, p.now())
		}
		title := fmt.Sprintf("%s account %s: %d posts in sampled response", plan.platform, plan.handle, len(posts))
		paidObservation(r, plan.platform, "derivation", title, plan.manual, data, posts)
	}
}

func (p *Paid) socialProfile(ctx context.Context, r *model.Result, s paidSocialPlan) (map[string]any, []model.Content, error) {
	handle := url.Values{"handle": {s.handle}}
	switch s.platform {
	case "tiktok":
		profile, err := p.get(ctx, "/v1/tiktok/profile", handle)
		if err != nil {
			return nil, nil, err
		}
		stats, user := paidObj(profile["stats"]), paidObj(profile["user"])
		if stats == nil && user == nil {
			return nil, nil, errors.New("TikTok response missing public profile")
		}
		handle.Set("sort_by", "latest")
		handle.Set("include_pinned", "false")
		recent := p.socialFeed(ctx, r, s, "/v3/tiktok/profile/videos", handle, "aweme_list")
		var posts []model.Content
		for _, raw := range recent[:min(35, len(recent))] {
			post := paidObj(raw)
			id := paidStr(post["aweme_id"])
			if id == "" {
				continue
			}
			stat := paidObj(post["statistics"])
			thumb := paidURL(paidFirst(
				paidFirstString(paidAt(post, "video", "dynamic_cover", "url_list")),
				paidFirstString(paidAt(post, "video", "cover", "url_list")),
			))
			video := paidURL(paidFirstString(paidAt(post, "video", "play_addr", "url_list")))
			var tags []string
			if post["is_ad"] == true {
				tags = []string{"marked as ad"}
			}
			link := paidFirst(paidURL(paidStr(post["share_url"])), s.manual+"/video/"+url.PathEscape(id))
			posts = append(posts, model.Content{ID: "tiktok:" + id, Platform: "tiktok", Kind: "video", URL: link, Text: paidClip(paidStr(post["desc"]), 400), Thumb: thumb, Video: video, Author: s.handle, Date: paidDateText(paidDate(post["create_time"])), Tags: tags, Metrics: paidMetrics(map[string]any{"views": stat["play_count"], "likes": stat["digg_count"], "comments": stat["comment_count"], "shares": stat["share_count"]})})
		}
		return map[string]any{"followers": paidValue(stats["followerCount"]), "videos": paidValue(stats["videoCount"]), "bio": paidClip(paidStr(user["signature"]), 300)}, posts, nil
	case "instagram":
		response, rows, err := paidFetchArray(ctx, p, "/v2/instagram/user/posts", handle, "items")
		if err != nil {
			return nil, nil, err
		}
		user := paidObj(response["user"])
		followers := paidValue(user["follower_count"])
		if followers == nil && p.remaining() > 0 {
			profile, err := p.get(ctx, "/v1/instagram/profile", handle)
			if err != nil {
				paidUnknown(r, "organic_instagram", s.manual, fmt.Errorf("public profile unavailable: %w", err))
			} else if profileUser := paidObj(paidAt(profile, "data", "user")); profileUser != nil {
				user = profileUser
				followers = paidValue(paidAt(user, "edge_followed_by", "count"))
			}
		}
		if followers == nil {
			paidUnknown(r, "organic_instagram", s.manual, errors.New("public follower count unavailable"))
		}
		formats := map[string]int{}
		var posts []model.Content
		for _, raw := range rows[:min(35, len(rows))] {
			post := paidObj(raw)
			code := paidStr(post["code"])
			id := paidFirst(paidStr(post["pk"]), paidStr(post["id"]), code)
			if id == "" {
				continue
			}
			link := paidURL(paidStr(post["url"]))
			if link == "" && code != "" {
				link = "https://www.instagram.com/p/" + url.PathEscape(code) + "/"
			}
			format, kind := paidStr(post["product_type"]), "post"
			if format == "clips" {
				kind = "video"
			}
			paidCount(formats, format)
			img := paidFirstObj(paidItems(paidAt(post, "image_versions2", "candidates")))
			video := paidFirstObj(paidItems(post["video_versions"]))
			posts = append(posts, model.Content{ID: "instagram:" + id, Platform: "instagram", Kind: kind, URL: link, Text: paidClip(paidStr(paidAt(post, "caption", "text")), 400), Thumb: paidURL(paidFirst(paidStr(post["display_uri"]), paidStr(img["url"]), paidStr(post["thumbnail_url"]))), Video: paidURL(paidFirst(paidStr(post["video_url"]), paidStr(video["url"]))), Author: s.handle, Date: paidDateText(paidDate(post["taken_at"])), Tags: paidTags(format), Metrics: paidMetrics(map[string]any{"views": paidFirst(paidStr(post["play_count"]), paidStr(post["ig_play_count"]), paidStr(post["view_count"])), "likes": post["like_count"], "comments": post["comment_count"]})})
		}
		return map[string]any{"followers": followers, "bio": paidClip(paidStr(user["biography"]), 300), "formatMix": formats}, posts, nil
	case "youtube":
		ref := strings.SplitN(s.handle, "/", 2)
		if len(ref) == 2 && ref[0] == "channel" {
			handle = url.Values{"channelId": {ref[1]}}
		} else if len(ref) == 2 {
			handle = url.Values{"handle": {ref[1]}}
		}
		profile, err := p.get(ctx, "/v1/youtube/channel", handle)
		if err != nil {
			return nil, nil, err
		}
		if paidFirst(paidStr(profile["channel"]), paidStr(profile["subscriberCount"]), paidStr(profile["handle"]), paidStr(profile["title"])) == "" {
			return nil, nil, errors.New("YouTube response missing public channel")
		}
		handle.Set("sort", "popular")
		rows := p.socialFeed(ctx, r, s, "/v1/youtube/channel-videos", handle, "videos")
		var posts []model.Content
		for _, raw := range rows[:min(25, len(rows))] {
			post := paidObj(raw)
			id := paidStr(post["id"])
			if id == "" {
				continue
			}
			link := paidFirst(paidURL(paidStr(post["url"])), "https://www.youtube.com/watch?v="+url.QueryEscape(id))
			posts = append(posts, model.Content{ID: "youtube:" + id, Platform: "youtube", Kind: "video", URL: link, Title: paidClip(paidStr(post["title"]), 160), Thumb: paidURL(paidStr(post["thumbnail"])), Author: s.handle, Date: paidDateText(paidDate(post["publishedTime"])), Tags: []string{"popular sample"}, Metrics: paidMetrics(map[string]any{"views": post["viewCountInt"], "lengthSeconds": post["lengthSeconds"]})})
		}
		return map[string]any{"subscribers": paidValue(profile["subscriberCount"]), "followers": paidValue(profile["subscriberCount"]), "videos": paidValue(profile["videoCount"]), "views": paidValue(profile["viewCount"]), "description": paidClip(paidStr(profile["description"]), 400), "sampleOrder": "popular"}, posts, nil
	case "x":
		profile, err := p.get(ctx, "/v1/twitter/profile", handle)
		if err != nil {
			return nil, nil, err
		}
		legacy := paidObj(profile["legacy"])
		if legacy == nil {
			return nil, nil, errors.New("X response missing public profile")
		}
		rows := p.socialFeed(ctx, r, s, "/v1/twitter/user-tweets", handle, "tweets")
		return map[string]any{"followers": paidValue(legacy["followers_count"]), "bio": paidClip(paidStr(legacy["description"]), 300), "sampleOrder": "popular"}, paidTweets(rows, s.handle), nil
	case "linkedin":
		profile, err := p.get(ctx, "/v1/linkedin/company", url.Values{"url": {s.manual}})
		if err != nil {
			return nil, nil, err
		}
		name := paidStr(profile["name"])
		if name == "" {
			return nil, nil, errors.New("LinkedIn response missing public company profile")
		}
		rows, ok := profile["posts"].([]any)
		if !ok {
			rows = p.socialFeed(ctx, r, s, "/v1/linkedin/company/posts", url.Values{"url": {s.manual}}, "posts")
		}
		return map[string]any{"name": name, "followers": paidValue(profile["followers"]), "employees": paidValue(profile["employeeCount"]), "industry": paidClip(paidStr(profile["industry"]), 160), "founded": paidStr(profile["founded"])}, paidLinkedInPosts(rows, name, "company page"), nil
	default:
		return nil, nil, errors.New("unsupported social platform")
	}
}

func (p *Paid) socialFeed(ctx context.Context, r *model.Result, s paidSocialPlan, endpoint string, query url.Values, field string) []any {
	_, rows, err := paidFetchArray(ctx, p, endpoint, query, field)
	if err != nil {
		paidUnknown(r, "organic_"+s.platform, s.manual, fmt.Errorf("post sample unavailable: %w", err))
	}
	return rows
}

func paidTweets(rows []any, handle string) []model.Content {
	var posts []model.Content
	for _, raw := range rows[:min(35, len(rows))] {
		post := paidObj(raw)
		legacy := paidObj(post["legacy"])
		id := paidFirst(paidStr(post["rest_id"]), paidStr(legacy["id_str"]))
		if id == "" {
			continue
		}
		link := paidFirst(paidURL(paidStr(post["url"])), "https://x.com/"+url.PathEscape(handle)+"/status/"+url.PathEscape(id))
		media := paidFirstObj(paidItems(paidAt(legacy, "extended_entities", "media")))
		posts = append(posts, model.Content{ID: "x:" + id, Platform: "x", Kind: "post", URL: link, Text: paidClip(paidStr(legacy["full_text"]), 500), Thumb: paidURL(paidStr(media["media_url_https"])), Author: handle, Date: paidDateText(paidDate(legacy["created_at"])), Tags: []string{"popular sample"}, Metrics: paidMetrics(map[string]any{"views": paidAt(post, "views", "count"), "likes": legacy["favorite_count"], "shares": legacy["retweet_count"]})})
	}
	return posts
}

func paidLinkedInPosts(rows []any, name, tag string) []model.Content {
	var posts []model.Content
	for _, raw := range rows[:min(25, len(rows))] {
		post := paidObj(raw)
		link := paidURL(paidFirst(paidStr(post["url"]), paidStr(post["link"])))
		id := paidFirst(paidStr(post["id"]), link)
		if id == "" || link == "" {
			continue
		}
		posts = append(posts, model.Content{ID: "linkedin:" + id, Platform: "linkedin", Kind: "post", URL: link, Text: paidClip(paidFirst(paidStr(post["text"]), paidStr(post["title"])), 500), Author: name, Date: paidDateText(paidDate(post["datePublished"])), Tags: paidTags(tag)})
	}
	return posts
}

func paidYouTubeURL(handle string) string {
	parts := strings.SplitN(strings.TrimPrefix(handle, "@"), "/", 2)
	if len(parts) == 2 && (parts[0] == "channel" || parts[0] == "c" || parts[0] == "user") {
		return "https://www.youtube.com/" + parts[0] + "/" + url.PathEscape(parts[1])
	}
	return "https://www.youtube.com/@" + url.PathEscape(handle)
}

func paidMedian(posts []model.Content, metric string) any {
	var vals []float64
	for _, post := range posts {
		if v, ok := post.Metrics[metric]; ok {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return nil
	}
	sort.Float64s(vals)
	i := len(vals) / 2
	if len(vals)%2 == 0 {
		return (vals[i-1] + vals[i]) / 2
	}
	return vals[i]
}

func paidActivity(posts []model.Content, now time.Time) (perWeek any, daysAgo any) {
	var earliest, latest time.Time
	count := 0
	for _, post := range posts {
		d := paidDate(post.Date)
		if d.IsZero() || d.After(now) {
			continue
		}
		if earliest.IsZero() || d.Before(earliest) {
			earliest = d
		}
		if d.After(latest) {
			latest = d
		}
		count++
	}
	if !latest.IsZero() {
		daysAgo = int(now.Sub(latest).Hours() / 24)
	}
	if span := latest.Sub(earliest).Hours() / (24 * 7); span >= 1 {
		perWeek = float64(count-1) / span
	}
	return perWeek, daysAgo
}

func (p *Paid) paidDiscoverSocials(ctx context.Context, r *model.Result, target model.Target, brand string, handles map[string]string) map[string]string {
	if p.remaining() < 10 || paidNorm(brand) == "" {
		return nil
	}
	sites := []struct{ platform, domain, prefix string }{
		{"x", "x.com", "/"}, {"instagram", "instagram.com", "/"}, {"linkedin", "linkedin.com", "/company/"}, {"youtube", "youtube.com", "/@"},
	}
	var queries []string
	for _, site := range sites {
		if handles[site.platform] == "" {
			queries = append(queries, "site:"+site.domain+strings.TrimSuffix(site.prefix, "/"))
		}
	}
	discovered := make(map[string]string)
	if len(queries) > 0 {
		q := fmt.Sprintf("%q (%s)", brand, strings.Join(queries, " OR "))
		_, rows, err := paidFetchArray(ctx, p, "/v1/google/search", url.Values{"query": {q}}, "results")
		if err == nil {
			for _, raw := range rows[:min(20, len(rows))] {
				link := paidStr(paidAt(raw, "url"))
				for _, site := range sites {
					u := paidOnSite(link, site.domain, site.prefix)
					if u == nil || handles[site.platform] != "" {
						continue
					}
					handle := strings.Trim(strings.TrimPrefix(u.Path, site.prefix), "/")
					if !strings.Contains(handle, "/") && paidAccountMatch(handle, brand, target.Domain) {
						handles[site.platform], discovered[site.platform] = handle, "brand-matching indexed profile"
					}
				}
			}
		}
	}
	if handles["tiktok"] == "" && p.remaining() >= 8 {
		response, err := p.get(ctx, "/v1/tiktok/search/users", url.Values{"query": {brand}})
		if err == nil {
			rows, ok := response["users"].([]any)
			if !ok {
				rows = paidItems(response["user_list"])
			}
			for _, raw := range rows[:min(20, len(rows))] {
				user := paidObj(paidAt(raw, "user_info"))
				handle := paidStr(user["unique_id"])
				crossMatch := handle != "" && (strings.EqualFold(handle, handles["instagram"]) || strings.EqualFold(handle, handles["x"]))
				verified := user["custom_verify"] == true || paidStr(user["custom_verify"]) != ""
				nameMatch := verified && paidNorm(paidStr(user["nickname"])) == paidNorm(brand) && paidAccountMatch(handle, brand, target.Domain)
				if crossMatch || nameMatch {
					handles["tiktok"], discovered["tiktok"] = handle, "matching public TikTok search profile"
					break
				}
			}
		}
	}
	if len(discovered) > 0 {
		links := make(map[string]string, len(discovered))
		for platform := range discovered {
			links[platform] = handles[platform]
		}
		paidObservation(r, "website", "derivation", "Potential official social accounts matched by name", "https://www.google.com/search?q="+url.QueryEscape(brand), links, nil)
	}
	return discovered
}

func paidAccountMatch(handle, brand, domain string) bool {
	h := paidNorm(handle)
	for _, candidate := range []string{paidNorm(brand), paidNorm(strings.Split(domain, ".")[0])} {
		if len(candidate) < 3 {
			continue
		}
		if h == candidate || h == "get"+candidate || h == candidate+"app" || h == candidate+"hq" || h == candidate+"official" {
			return true
		}
	}
	return false
}
