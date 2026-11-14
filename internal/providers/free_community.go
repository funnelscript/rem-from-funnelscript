package providers

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
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

type redditPost struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Subreddit string  `json:"subreddit"`
	Score     int     `json:"score"`
	Comments  int     `json:"num_comments"`
	Permalink string  `json:"permalink"`
	URL       string  `json:"url"`
	Created   float64 `json:"created_utc"`
}

type redditListing struct {
	Data struct {
		Children []struct {
			Data redditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// RedditPublic queries keyless public search for community discussions about the target.
func RedditPublic(ctx context.Context, get fetch.Getter, target model.Target) model.Result {
	var result model.Result
	queryTarget := target.Domain
	if queryTarget == "" {
		queryTarget = target.Brand
	}
	if queryTarget == "" {
		appendUnknown(&result, "reddit", "No domain or brand available for Reddit search", "https://www.reddit.com/search/")
		return result
	}
	searchURL := "https://www.reddit.com/search.json?q=" + url.QueryEscape(queryTarget) + "&sort=top&t=year&limit=25"
	manual := "https://www.reddit.com/search/?q=" + url.QueryEscape(queryTarget)

	var listing redditListing
	if err := freeJSON(ctx, get, searchURL, &listing); err != nil {
		appendUnknown(&result, "reddit", "Reddit public search unavailable: "+err.Error(), manual)
		return result
	}

	threads := make([]map[string]any, 0, len(listing.Data.Children))
	totalScore := 0
	subreddits := make(map[string]int)

	for _, child := range listing.Data.Children {
		p := child.Data
		if p.Title == "" {
			continue
		}
		totalScore += p.Score
		subreddits[p.Subreddit]++
		if len(threads) < 10 {
			permalink := p.Permalink
			if !strings.HasPrefix(permalink, "http") {
				permalink = "https://www.reddit.com" + permalink
			}
			threads = append(threads, map[string]any{
				"title":     brief(p.Title, 180),
				"subreddit": p.Subreddit,
				"score":     p.Score,
				"comments":  p.Comments,
				"url":       permalink,
				"created":   time.Unix(int64(p.Created), 0).UTC().Format("2006-01-02"),
			})
		}
	}

	if len(threads) == 0 {
		appendObservation(&result, "reddit", "Reddit returned no matching threads in top search results", manual, map[string]any{
			"threads": []any{}, "matchedThreads": 0, "totalScore": 0,
		})
		return result
	}

	topSubs := make([]string, 0, len(subreddits))
	for sub := range subreddits {
		topSubs = append(topSubs, "r/"+sub)
	}
	slices.Sort(topSubs)

	title := fmt.Sprintf("Reddit: %d public threads discussing %s, total score %d across %d subreddits", len(threads), queryTarget, totalScore, len(topSubs))
	appendObservation(&result, "reddit", title, manual, map[string]any{
		"threads":        threads,
		"matchedThreads": len(threads),
		"totalScore":     totalScore,
		"subreddits":     topSubs,
	})
	return result
}

type gitHubRepo struct {
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	Description string   `json:"description"`
	Stars       int      `json:"stargazers_count"`
	Forks       int      `json:"forks_count"`
	OpenIssues  int      `json:"open_issues_count"`
	Topics      []string `json:"topics"`
	PushedAt    string   `json:"pushed_at"`
	License     *struct {
		SPDX string `json:"spdx_id"`
	} `json:"license"`
}

// GitHubPublic extracts public repository intelligence without requiring API tokens.
func GitHubPublic(ctx context.Context, get fetch.Getter, target model.Target, discovered model.Discovered) model.Result {
	var result model.Result
	gh := discovered.Socials["github"]
	if gh == "" {
		return result
	}
	gh = strings.TrimPrefix(gh, "@")
	parts := strings.Split(strings.Trim(gh, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return result
	}

	var repoURL, apiURL string
	if len(parts) >= 2 {
		repoURL = "https://github.com/" + parts[0] + "/" + parts[1]
		apiURL = "https://api.github.com/repos/" + parts[0] + "/" + parts[1]
	} else {
		repoURL = "https://github.com/" + parts[0]
		apiURL = "https://api.github.com/users/" + parts[0] + "/repos?sort=updated&per_page=1"
	}

	if len(parts) >= 2 {
		var repo gitHubRepo
		if err := freeJSON(ctx, get, apiURL, &repo); err != nil {
			appendUnknown(&result, "community_launches", "GitHub public API unavailable: "+err.Error(), repoURL)
			return result
		}
		license := ""
		if repo.License != nil {
			license = repo.License.SPDX
		}
		title := fmt.Sprintf("GitHub: %s with %d stars, %d forks", repo.FullName, repo.Stars, repo.Forks)
		appendObservation(&result, "github", title, repoURL, map[string]any{
			"fullName":   repo.FullName,
			"stars":      repo.Stars,
			"forks":      repo.Forks,
			"openIssues": repo.OpenIssues,
			"topics":     repo.Topics,
			"license":    license,
			"pushedAt":   repo.PushedAt,
		})
	} else {
		var repos []gitHubRepo
		if err := freeJSON(ctx, get, apiURL, &repos); err != nil || len(repos) == 0 {
			appendUnknown(&result, "community_launches", "GitHub public org/user info: no public repos found", repoURL)
			return result
		}
		repo := repos[0]
		title := fmt.Sprintf("GitHub: %s active repo %s with %d stars", parts[0], repo.Name, repo.Stars)
		appendObservation(&result, "github", title, repoURL, map[string]any{
			"fullName": repo.FullName,
			"stars":    repo.Stars,
			"forks":    repo.Forks,
			"pushedAt": repo.PushedAt,
		})
	}
	return result
}
