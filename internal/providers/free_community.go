package providers

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"remgo/internal/fetch"
	"remgo/internal/model"
)

var storyID = regexp.MustCompile(`^[0-9]+$`)

type hnHit struct {
	ID        string `json:"objectID"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	Points    int    `json:"points"`
	Comments  int    `json:"num_comments"`
}

// Community reads Hacker News search results and checks the linked host before attribution.
func Community(ctx context.Context, get fetch.Getter, target model.Target) model.Result {
	var result model.Result
	if target.Domain == "" {
		appendUnknown(&result, "community", "No website domain was resolved for Hacker News search", "https://hn.algolia.com/")
		return result
	}
	query := url.Values{"query": {target.Domain}, "restrictSearchableAttributes": {"url"}, "hitsPerPage": {"30"}}
	manual := "https://hn.algolia.com/?" + url.Values{"query": {target.Domain}}.Encode()
	var data struct {
		Count *int    `json:"nbHits"`
		Hits  []hnHit `json:"hits"`
	}
	if err := freeJSON(ctx, get, "https://hn.algolia.com/api/v1/search?"+query.Encode(), &data); err != nil {
		appendUnknown(&result, "community", "Hacker News search unavailable: "+err.Error(), manual)
		return result
	}
	if data.Hits == nil {
		appendUnknown(&result, "community", "Hacker News search did not return a hits array", manual)
		return result
	}
	slices.SortStableFunc(data.Hits, func(a, b hnHit) int { return cmp.Compare(b.Points, a.Points) })
	stories := []map[string]any{}
	bestPoints, matches := 0, 0
	showHN := false
	seen := map[string]bool{}
	for _, hit := range data.Hits {
		if !storyID.MatchString(hit.ID) || hit.Title == "" || seen[hit.ID] {
			continue
		}
		matching := matchingHost(hit.URL, target.Domain)
		for _, alias := range target.Aliases {
			matching = matching || matchingHost(hit.URL, alias)
		}
		if !matching {
			continue
		}
		seen[hit.ID] = true
		matches++
		bestPoints = max(bestPoints, hit.Points)
		isLaunch := strings.HasPrefix(strings.ToLower(hit.Title), "show hn:")
		showHN = showHN || isLaunch
		if len(stories) >= 8 {
			continue
		}
		stories = append(stories, map[string]any{
			"title": brief(hit.Title, 180), "points": hit.Points, "comments": hit.Comments, "date": dayPrefix(hit.CreatedAt),
			"showHN": isLaunch, "url": "https://news.ycombinator.com/item?id=" + hit.ID, "submittedUrl": hit.URL,
		})
	}
	title := "Hacker News returned no submissions in the checked search results"
	if matches > 0 {
		title = fmt.Sprintf("Hacker News: %d matching submissions in %d checked results, best %d points", matches, len(data.Hits), bestPoints)
		if showHN {
			title += ", includes a Show HN launch"
		}
	}
	if matches == 0 && len(data.Hits) > 0 && (data.Count == nil || *data.Count > len(data.Hits)) {
		appendUnknown(&result, "community", "No matching links in the checked results and the Hacker News search has additional candidates", manual)
	}
	appendObservation(&result, "hackernews", title, manual, stories)
	return result
}
