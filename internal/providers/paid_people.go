package providers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

const paidGoogleSearch = "/v1/google/search"

func (p *Paid) paidCommunity(ctx context.Context, r *model.Result, target model.Target, d model.Discovered, brand string) {
	manual := paidQuery("https://www.reddit.com/search/", url.Values{"q": {paidFirst(brand, target.Domain)}})
	p.paidReddit(ctx, r, target, d, brand, manual)
	p.paidProductHunt(ctx, r, d, brand)
}

func (p *Paid) paidReddit(ctx context.Context, r *model.Result, target model.Target, d model.Discovered, brand, manual string) {
	queries := []string{fmt.Sprintf("%q", brand)}
	if target.Domain != "" && !strings.EqualFold(target.Domain, brand) && p.remaining() >= 4 {
		queries = append(queries, target.Domain)
	}
	seen := make(map[string]bool)
	var items []model.Content
	affiliated, comparisons, completed := 0, 0, 0
	for _, query := range queries {
		_, rows, err := paidFetchArray(ctx, p, "/v1/reddit/search", url.Values{"query": {query}}, "posts")
		if err != nil {
			paidUnknown(r, "reddit_marketing", manual, err)
			continue
		}
		completed++
		for _, raw := range rows[:min(30, len(rows))] {
			post := paidObj(raw)
			id, title, body := paidStr(post["id"]), paidStr(post["title"]), paidStr(post["selftext"])
			link := paidRedditLink(post)
			if id == "" || link == "" || seen[id] || !paidBrandMention(title+" "+body, brand, target.Domain) {
				continue
			}
			seen[id] = true
			var tags []string
			if paidComparison.MatchString(title) {
				comparisons++
				tags = append(tags, "comparison")
			}
			if paidAffiliation(body, brand, target.Domain) {
				affiliated++
				tags = append(tags, "self-described affiliation")
			}
			items = append(items, model.Content{ID: "reddit:" + id, Platform: "reddit", Kind: "thread", URL: link, Title: paidClip(title, 180), Text: paidClip(body, 500), Author: paidStr(post["author"]), Date: paidDateText(paidDate(paidFirst(paidStr(post["created_at_iso"]), paidStr(post["created_utc"])))), Metrics: paidMetrics(map[string]any{"score": post["score"], "comments": post["num_comments"]}), Tags: tags})
		}
	}
	if completed == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Metrics["score"] > items[j].Metrics["score"] })
	threads := len(items)
	var commentCount, affiliatedComments, checkedThreads int
	for _, thread := range items[:min(3, len(items))] {
		if p.remaining() < 2 {
			break
		}
		_, rows, err := paidFetchArray(ctx, p, "/v1/reddit/post/comments", url.Values{"url": {thread.URL}}, "comments")
		if err != nil {
			paidUnknown(r, "reddit_marketing", thread.URL, fmt.Errorf("thread comments unavailable: %w", err))
			continue
		}
		checkedThreads++
		paidWalkComments(rows, 0, func(comment paidObject) {
			if commentCount >= 30 {
				return
			}
			body, link := paidStr(comment["body"]), paidRedditLink(comment)
			if link == "" || seen[link] || !paidBrandMention(body, brand, target.Domain) {
				return
			}
			seen[link] = true
			commentCount++
			tags := []string{"mentions brand"}
			if paidAffiliation(body, brand, target.Domain) {
				affiliatedComments++
				tags = append(tags, "self-described affiliation")
			}
			items = append(items, model.Content{ID: "reddit_comment:" + link, Platform: "reddit", Kind: "comment", URL: link, Title: thread.Title, Text: paidClip(body, 500), Author: paidStr(comment["author"]), Date: paidDateText(paidDate(paidFirst(paidStr(comment["created_at_iso"]), paidStr(comment["created_utc"])))), Metrics: paidMetrics(map[string]any{"score": comment["score"]}), Tags: tags})
		})
	}
	data := map[string]any{"matchedThreads": threads, "comparisonThreads": comparisons, "threadsCheckedForComments": checkedThreads, "matchingComments": commentCount, "selfDescribedAffiliatedPosts": affiliated, "selfDescribedAffiliatedComments": affiliatedComments, "sample": items[:min(8, len(items))]}
	if linked := d.Socials["reddit"]; linked != "" {
		data["linkedSubreddit"] = linked
	}
	paidObservation(r, "reddit", "derivation", fmt.Sprintf("Reddit sample: %d matching threads and %d matching comments", threads, commentCount), manual, data, items)
}

var (
	paidComparison = regexp.MustCompile(`(?i)\b(vs\.?|versus|alternatives?|compare|recommendations?)\b`)
	paidAffiliated = regexp.MustCompile(`(?i)\b(i built|we built|i founded|we founded|our app|our product|i am the founder|i'm the founder|i cofounded|i co-founded)\b`)
)

func paidBrandMention(s, brand, domain string) bool {
	lower := strings.ToLower(s)
	for _, term := range []string{domain, brand} {
		if utf8.RuneCountInString(term) < 2 {
			continue
		}
		pattern := `(^|[^\pL\pN])` + regexp.QuoteMeta(strings.ToLower(term)) + `($|[^\pL\pN])`
		if ok, _ := regexp.MatchString(pattern, lower); ok {
			return true
		}
	}
	return false
}

func paidAffiliation(text, brand, domain string) bool {
	return paidBrandMention(text, brand, domain) && paidAffiliated.MatchString(text)
}

func paidRedditLink(post paidObject) string {
	for _, link := range []string{paidStr(post["url"]), paidStr(post["permalink"])} {
		if strings.HasPrefix(link, "/r/") {
			link = "https://www.reddit.com" + link
		}
		if u := paidOnSite(link, "reddit.com", "/r/"); u != nil {
			return u.String()
		}
	}
	return ""
}

func paidWalkComments(rows []any, depth int, visit func(paidObject)) {
	if depth >= 4 {
		return
	}
	for _, raw := range rows[:min(35, len(rows))] {
		c := paidObj(raw)
		if c == nil {
			continue
		}
		if nested := paidObj(c["data"]); nested != nil {
			c = nested
		}
		visit(c)
		replies := c["replies"]
		if rows := paidItems(replies); rows != nil {
			paidWalkComments(rows, depth+1, visit)
		} else {
			nested := paidItems(paidAt(replies, "items"))
			if nested == nil {
				nested = paidItems(paidAt(replies, "data", "children"))
			}
			paidWalkComments(nested, depth+1, visit)
		}
	}
}

func (p *Paid) paidProductHunt(ctx context.Context, r *model.Result, d model.Discovered, brand string) {
	manual := "https://www.producthunt.com/search?q=" + url.QueryEscape(brand)
	if handle := d.Socials["producthunt"]; handle != "" {
		manual = "https://www.producthunt.com/products/" + url.PathEscape(handle)
	}
	query := fmt.Sprintf("site:producthunt.com/products %q", brand)
	_, rows, err := paidFetchArray(ctx, p, paidGoogleSearch, url.Values{"query": {query}}, "results")
	if err != nil {
		paidUnknown(r, "producthunt", manual, err)
		return
	}
	matches := make([]map[string]string, 0, 5)
	seen := make(map[string]bool)
	for _, raw := range rows[:min(30, len(rows))] {
		row := paidObj(raw)
		u := paidOnSite(paidStr(row["url"]), "producthunt.com", "/products/")
		if u == nil {
			continue
		}
		path := strings.Split(strings.TrimPrefix(u.Path, "/products/"), "/")[0]
		title := paidStr(row["title"])
		if !paidAccountMatch(path, brand, "") && !paidBrandMention(title, brand, "") {
			continue
		}
		u.RawQuery, u.Fragment = "", ""
		if seen[u.Path] {
			continue
		}
		seen[u.Path] = true
		matches = append(matches, map[string]string{"title": paidClip(title, 160), "url": u.String(), "snippet": paidClip(paidStr(row["description"]), 300)})
		if len(matches) == 5 {
			break
		}
	}
	if len(matches) > 0 {
		manual = matches[0]["url"]
	}
	paidObservation(r, "producthunt", "observation", fmt.Sprintf("Product Hunt: %d matching indexed pages", len(matches)), manual, map[string]any{"matches": len(matches), "pages": matches}, nil)
}

type paidFounder struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	URL       string `json:"url,omitempty"`
	LinkedIn  string `json:"linkedin,omitempty"`
	Source    string `json:"source"`
	Followers any    `json:"linkedinFollowers,omitempty"`
	Photo     string `json:"photo,omitempty"`
	X         any    `json:"x,omitempty"`
}

func (p *Paid) founders(ctx context.Context, r *model.Result, d model.Discovered, brand string) {
	search := fmt.Sprintf("%q (founder OR co-founder OR CEO) site:linkedin.com/in", brand)
	manual := "https://www.google.com/search?q=" + url.QueryEscape(search)
	_, rows, searchErr := paidFetchArray(ctx, p, paidGoogleSearch, url.Values{"query": {search}}, "results")
	if searchErr != nil {
		paidUnknown(r, "founder_led", manual, searchErr)
		return
	}
	founders := make([]paidFounder, 0, 5)
	names, profiles := make(map[string]bool), make(map[string]bool)
	for _, hint := range d.Founders[:min(5, len(d.Founders))] {
		if paidNorm(hint.Name) == "" || names[paidNorm(hint.Name)] {
			continue
		}
		linkedin := ""
		if u := paidOnSite(hint.URL, "linkedin.com", "/in/"); u != nil {
			linkedin = u.String()
			profiles[linkedin] = true
		}
		founders = append(founders, paidFounder{Name: hint.Name, Role: hint.Role, URL: paidURL(hint.URL), LinkedIn: linkedin, Source: "website"})
		names[paidNorm(hint.Name)] = true
	}
	for _, raw := range rows[:min(20, len(rows))] {
		if len(founders) >= 5 {
			break
		}
		row := paidObj(raw)
		u := paidOnSite(paidStr(row["url"]), "linkedin.com", "/in/")
		title := paidStr(row["title"])
		parts := paidFounderSplit.Split(title, -1)
		if u == nil || len(parts) < 2 {
			continue
		}
		role := ""
		for _, part := range parts[1:] {
			if paidFounderAssociation(part, brand) {
				role = part
				break
			}
		}
		description := paidStr(row["description"])
		if role == "" && paidFounderAssociation(description, brand) {
			role = "Founder or CEO per indexed profile snippet"
		}
		if role == "" {
			continue
		}
		name := paidClip(parts[0], 80)
		u.RawQuery, u.Fragment = "", ""
		link := u.String()
		if name == "" || profiles[link] {
			continue
		}
		if names[paidNorm(name)] {
			for i := range founders {
				if paidNorm(founders[i].Name) == paidNorm(name) && founders[i].LinkedIn == "" {
					founders[i].LinkedIn = link
					profiles[link] = true
				}
			}
			continue
		}
		founders = append(founders, paidFounder{Name: name, Role: paidClip(role, 160), URL: link, LinkedIn: link, Source: "indexed LinkedIn result"})
		names[paidNorm(name)], profiles[link] = true, true
	}
	var posts []model.Content
	for i := range founders[:min(3, len(founders))] {
		f := &founders[i]
		if f.LinkedIn == "" || p.remaining() == 0 {
			continue
		}
		profile, err := p.get(ctx, "/v1/linkedin/profile", url.Values{"url": {f.LinkedIn}})
		if err != nil {
			paidUnknown(r, "founder_led", f.LinkedIn, err)
			continue
		}
		if name := paidStr(profile["name"]); name == "" || paidNorm(name) != paidNorm(f.Name) {
			paidUnknown(r, "founder_led", f.LinkedIn, errors.New("public profile name did not confirm the founder match"))
			continue
		}
		f.Followers = paidValue(profile["followers"])
		f.Photo = paidURL(paidStr(profile["image"]))
		posts = append(posts, paidLinkedInPosts(paidItems(profile["recentPosts"]), f.Name, "founder")...)
	}
	if len(founders) > 0 && p.remaining() >= 8 {
		posts = append(posts, p.paidFounderX(ctx, &founders[0])...)
	}
	paidObservation(r, "founders", "observation", fmt.Sprintf("Founders or leaders: %d candidates from website and indexed profiles", len(founders)), manual, founders, posts)
}

var (
	paidFounderSplit = regexp.MustCompile(`\s+[-–|]\s+`)
	paidFounderRole  = regexp.MustCompile(`(?i)\b((?:co[- ]?)?founder|co[- ]?founded|founded|ceo|chief executive)\b(?:\s+(?:at|of|for)\s+|\s*[@,]\s*)`)
)

func paidFounderAssociation(text, brand string) bool {
	where := paidFounderRole.FindStringIndex(text)
	if where == nil {
		return false
	}
	// A role must directly precede the exact brand words.
	after := strings.TrimLeft(strings.TrimSpace(text[where[1]:]), "\"“'‘")
	if len(after) < len(brand) || !strings.EqualFold(after[:len(brand)], brand) {
		return false
	}
	tail := strings.TrimSpace(after[len(brand):])
	return tail == "" || strings.ContainsRune(".,:;|)\"”’", rune(tail[0]))
}

func (p *Paid) paidFounderX(ctx context.Context, founder *paidFounder) []model.Content {
	q := fmt.Sprintf("%q (site:x.com OR site:twitter.com)", founder.Name)
	_, rows, err := paidFetchArray(ctx, p, paidGoogleSearch, url.Values{"query": {q}}, "results")
	if err != nil {
		return nil
	}
	for _, raw := range rows[:min(6, len(rows))] {
		link := paidStr(paidAt(raw, "url"))
		u := paidOnSite(link, "x.com", "/")
		if u == nil {
			u = paidOnSite(link, "twitter.com", "/")
		}
		if u == nil {
			continue
		}
		handle := strings.Trim(u.Path, "/")
		if strings.Contains(handle, "/") || handle == "" {
			continue
		}
		profile, err := p.get(ctx, "/v1/twitter/profile", url.Values{"handle": {handle}})
		if err != nil {
			return nil
		}
		legacy := paidObj(profile["legacy"])
		if name := paidFirst(paidStr(paidAt(profile, "core", "name")), paidStr(legacy["name"])); paidNorm(name) != paidNorm(founder.Name) {
			continue
		}
		founder.X = map[string]any{"handle": handle, "followers": paidValue(legacy["followers_count"]), "url": "https://x.com/" + url.PathEscape(handle)}
		_, tweets, _ := paidFetchArray(ctx, p, "/v1/twitter/user-tweets", url.Values{"handle": {handle}}, "tweets")
		posts := paidTweets(tweets, handle)
		for i := range posts {
			posts[i].Author = founder.Name
			posts[i].Tags = append(posts[i].Tags, "founder")
		}
		return posts
	}
	return nil
}
