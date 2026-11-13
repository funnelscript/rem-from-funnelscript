package collect

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"remgo/internal/fetch"
	"remgo/internal/model"
)

const (
	maxSitemapFiles = 12
	maxSitemapURLs  = 50000
)

var aiBots = []string{"GPTBot", "OAI-SearchBot", "ChatGPT-User", "ClaudeBot", "Claude-SearchBot", "PerplexityBot", "Google-Extended", "CCBot", "Applebot-Extended", "Bytespider"}

type robotsFacts struct {
	Sitemaps      []string          `json:"sitemaps"`
	AIBots        map[string]string `json:"aiBots"`
	DisallowCount int               `json:"disallowCount"`
}

type robotsRule struct {
	allow bool
	path  string
}

type robotsGroup struct {
	agents []string
	rules  []robotsRule
}

func SEO(ctx context.Context, get fetch.Getter, target model.Target) model.Result {
	var result model.Result
	base, err := url.Parse(target.URL)
	if err != nil || base.Hostname() == "" {
		result.Unknowns = append(result.Unknowns, unknown("seo", "No website URL was available", ""))
		return result
	}
	site := origin(base)
	robotsURL, llmsURL := site+"/robots.txt", site+"/llms.txt"
	var robotsResponse fetch.Response
	var robotsErr error
	var pending sync.WaitGroup
	pending.Go(func() { robotsResponse, robotsErr = get.Get(ctx, robotsURL) })
	llmsResponse, llmsErr := get.Get(ctx, llmsURL)
	pending.Wait()
	var robots *robotsFacts
	var llmsText string

	robotsStatus, robotsProblem := textFileStatus(robotsResponse, robotsErr)
	if robotsStatus == "present" {
		data := parseRobots(string(robotsResponse.Body))
		robots = &data
	}
	llmsStatus, llmsProblem := textFileStatus(llmsResponse, llmsErr)
	if llmsStatus == "present" {
		llmsText = clip(strings.TrimSpace(string(llmsResponse.Body)), 1500)
	}
	if robotsProblem != "" {
		result.Unknowns = append(result.Unknowns, unknown("geo", "robots.txt: "+robotsProblem, robotsURL))
	}
	if llmsProblem != "" {
		result.Unknowns = append(result.Unknowns, unknown("geo", "llms.txt: "+llmsProblem, llmsURL))
	}
	if robotsStatus != "unavailable" || llmsStatus != "unavailable" {
		var stance []string
		switch llmsStatus {
		case "present":
			stance = append(stance, "has llms.txt")
		case "absent":
			stance = append(stance, "llms.txt absent")
		default:
			stance = append(stance, "llms.txt unavailable")
		}
		if robots != nil {
			for _, bot := range aiBots {
				if robots.AIBots[bot] == "blocked" {
					stance = append(stance, bot+" blocked at root")
				}
			}
		} else {
			stance = append(stance, "robots.txt "+robotsStatus)
		}
		sourceURL := robotsURL
		if robotsStatus == "unavailable" {
			sourceURL = llmsURL
		}
		result.Evidence = append(result.Evidence, evidence("seo", "observation", "AI-search stance: "+strings.Join(stance, ", "), sourceURL, map[string]any{
			"robots": robots, "robotsStatus": robotsStatus, "robotsURL": robotsURL, "llmsTxt": llmsText, "llmsStatus": llmsStatus, "llmsURL": llmsURL,
		}))
	}
	collectSitemaps(ctx, get, base, robots, &result)
	return result
}

func textFileStatus(response fetch.Response, err error) (status, problem string) {
	if err == nil && (response.Status == 404 || response.Status == 410) {
		return "absent", ""
	}
	if problem := unavailable(response, err); problem != "" {
		return "unavailable", problem
	}
	body := strings.ToLower(strings.TrimSpace(string(response.Body[:min(500, len(response.Body))])))
	if strings.HasPrefix(body, "<!doctype html") || strings.Contains(body, "<html") || strings.HasPrefix(body, "<head") {
		return "unavailable", "Expected a text file but received HTML"
	}
	return "present", ""
}

func parseRobots(raw string) robotsFacts {
	facts := robotsFacts{Sitemaps: []string{}, AIBots: make(map[string]string)}
	var groups []robotsGroup
	raw = strings.TrimPrefix(raw, "\uFEFF")
	for _, line := range strings.Split(raw, "\n") {
		line, _, _ = strings.Cut(line, "#")
		directive, value, ok := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(directive)) {
		case "sitemap":
			if value != "" && len(facts.Sitemaps) < 20 {
				facts.Sitemaps = addUnique(facts.Sitemaps, value)
			}
		case "user-agent":
			if value == "" {
				continue
			}
			if len(groups) == 0 || len(groups[len(groups)-1].rules) > 0 {
				groups = append(groups, robotsGroup{})
			}
			current := &groups[len(groups)-1]
			current.agents = addUnique(current.agents, strings.ToLower(value))
		case "allow", "disallow":
			if len(groups) == 0 {
				continue
			}
			allow := strings.EqualFold(strings.TrimSpace(directive), "allow")
			g := &groups[len(groups)-1]
			g.rules = append(g.rules, robotsRule{allow: allow, path: value})
			if !allow && value != "" {
				facts.DisallowCount++
			}
		}
	}
	for _, bot := range aiBots {
		var matched, wildcard []robotsRule
		for _, group := range groups {
			if slices.Contains(group.agents, strings.ToLower(bot)) {
				matched = append(matched, group.rules...)
			}
			if slices.Contains(group.agents, "*") {
				wildcard = append(wildcard, group.rules...)
			}
		}
		if len(matched) == 0 {
			matched = wildcard
		}
		facts.AIBots[bot] = rootPolicy(matched)
	}
	return facts
}

func rootPolicy(rules []robotsRule) string {
	if len(rules) == 0 {
		return "unspecified"
	}
	var disallowRoot, allowRoot, pathException, disallowPath bool
	for _, rule := range rules {
		switch {
		case rule.path == "/" || rule.path == "/*":
			if rule.allow {
				allowRoot = true
			} else {
				disallowRoot = true
			}
		case rule.allow && rule.path != "":
			pathException = true
		case !rule.allow && rule.path != "":
			disallowPath = true
		}
	}
	switch {
	case disallowRoot && !allowRoot && !pathException:
		return "blocked"
	case disallowRoot && !allowRoot, disallowPath:
		return "partial"
	default:
		return "allowed"
	}
}

type sitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type sitemapFile struct {
	XMLName  xml.Name
	URLs     []sitemapEntry `xml:"url"`
	Children []sitemapEntry `xml:"sitemap"`
}

func readSitemap(body []byte) (sitemapFile, error) {
	var doc sitemapFile
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		const maxDecompressed = 20 << 20
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return doc, err
		}
		defer reader.Close()
		body, err = io.ReadAll(io.LimitReader(reader, maxDecompressed+1))
		if err != nil {
			return doc, err
		}
		if len(body) > maxDecompressed {
			return doc, errors.New("decompressed sitemap exceeds the body limit")
		}
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&doc); err != nil {
		return doc, err
	}
	if doc.XMLName.Local != "urlset" && doc.XMLName.Local != "sitemapindex" {
		return doc, errors.New("response was not a sitemap")
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return doc, nil
		}
		if err != nil {
			return doc, err
		}
		if text, ok := token.(xml.CharData); ok && len(bytes.TrimSpace(text)) == 0 {
			continue
		}
		if _, ok := token.(xml.Comment); ok {
			continue
		}
		return doc, errors.New("unexpected data after sitemap")
	}
}

func collectSitemaps(ctx context.Context, get fetch.Getter, base *url.URL, robots *robotsFacts, result *model.Result) {
	site := origin(base)
	var queue []string
	if robots != nil {
		queue = append(queue, robots.Sitemaps...)
	}
	seen := make(map[string]bool)
	locations := make(map[string]bool)
	read := make([]string, 0)
	var entries []sitemapEntry
	var failures, external, absent, inspected int
	limited := false

	readQueue := func() {
		for len(queue) > 0 && inspected < maxSitemapFiles && len(entries) < maxSitemapURLs && ctx.Err() == nil {
			raw := queue[0]
			queue = queue[1:]
			loc := publicLink(base, raw)
			if loc == nil || !withinDomain(siteHost(loc), siteHost(base)) {
				external++
				continue
			}
			url := loc.String()
			if seen[url] {
				continue
			}
			seen[url] = true
			inspected++
			response, err := get.Get(ctx, url)
			if err == nil && (response.Status == 404 || response.Status == 410) {
				absent++
				continue
			}
			if unavailable(response, err) != "" || resolvedPageURL(response.URL, url, base) == nil {
				failures++
				continue
			}
			doc, err := readSitemap(response.Body)
			if err != nil {
				failures++
				continue
			}
			read = append(read, url)
			limited = limited || len(doc.Children) > 30
			for _, item := range doc.Children[:min(30, len(doc.Children))] {
				if loc := publicLink(loc, strings.TrimSpace(item.Loc)); loc != nil {
					queue = append(queue, loc.String())
				}
			}
			if doc.XMLName.Local != "urlset" {
				continue
			}
			for _, entry := range doc.URLs {
				raw := strings.TrimSpace(entry.Loc)
				link := publicLink(loc, raw)
				if link == nil || locations[link.String()] || !withinDomain(siteHost(link), siteHost(base)) {
					continue
				}
				locations[link.String()] = true
				entry.Loc = link.String()
				entries = append(entries, entry)
				if len(entries) >= maxSitemapURLs {
					break
				}
			}
		}
	}
	readQueue()
	if len(read) == 0 && inspected < maxSitemapFiles {
		for _, suffix := range [...]string{"/sitemap.xml", "/sitemap_index.xml", "/sitemap-index.xml"} {
			queue = []string{site + suffix}
			readQueue()
			if len(read) > 0 {
				break
			}
		}
	}
	if len(read) == 0 {
		if failures > 0 || external > 0 || ctx.Err() != nil {
			result.Unknowns = append(result.Unknowns, unknown("seo", "Could not read a sitemap from the checked locations", site+"/sitemap.xml"))
			return
		}
		if absent > 0 {
			result.Evidence = append(result.Evidence, evidence("seo", "observation", "No sitemap returned at the checked locations", site+"/sitemap.xml", map[string]any{"locationsChecked": inspected}))
		}
		return
	}
	analysis := analyzeSitemap(entries, time.Now().UTC())
	analysis["sitemapsRead"] = read
	analysis["sitemapsFailed"] = failures
	analysis["crossSiteSitemapsSkipped"] = external
	analysis["boundedSample"] = failures > 0 || external > 0 || limited || len(queue) > 0 || len(entries) >= maxSitemapURLs || ctx.Err() != nil
	families := 0
	for _, section := range analysis["sections"].([]sitemapSection) {
		if section.Programmatic {
			families++
		}
	}
	result.Evidence = append(result.Evidence, evidence("seo", "derivation",
		fmt.Sprintf("Sitemap: %d URLs read, %d possible programmatic page families", len(entries), families),
		read[0], analysis))
	if analysis["boundedSample"] == true {
		result.Unknowns = append(result.Unknowns, unknown("seo", "Sitemap totals cover readable same-site files within collection limits", read[0]))
	}
}

type sitemapSection struct {
	Path         string   `json:"path"`
	Count        int      `json:"count"`
	Examples     []string `json:"examples"`
	Programmatic bool     `json:"programmatic"`
	FreshLast90d int      `json:"freshLast90d"`
	slugs        []string
}

var (
	localePattern   = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]{2})?$`)
	slugToken       = regexp.MustCompile(`[\pL\pN]+`)
	sitemapPatterns = map[string]*regexp.Regexp{
		"comparison":   regexp.MustCompile(`-vs-|/vs/|/compare/`),
		"alternatives": regexp.MustCompile(`alternative`),
		"integrations": regexp.MustCompile(`/integrations?/`),
		"templates":    regexp.MustCompile(`/templates?/`),
		"tools":        regexp.MustCompile(`/(tools?|free-tools?|generators?|calculators?)/`),
		"glossary":     regexp.MustCompile(`/(glossary|dictionary|terms|definitions?|what-is)\b`),
		"useCases":     regexp.MustCompile(`/(use-cases?|for|solutions?|industries)/`),
		"locations":    regexp.MustCompile(`/(locations?|cities|near-me|in-[a-z-]+)\b`),
		"howTo":        regexp.MustCompile(`how-to-`),
		"listicles":    regexp.MustCompile(`/(best|top)-|-best-|top-\d+`),
	}
)

func analyzeSitemap(entries []sitemapEntry, now time.Time) map[string]any {
	byPath := make(map[string]*sitemapSection)
	patterns := make(map[string]int)
	for name := range sitemapPatterns {
		patterns[name] = 0
	}
	locales := make([]string, 0)
	var recent30, recent90 int
	for _, entry := range entries {
		loc, err := url.Parse(entry.Loc)
		if err != nil {
			continue
		}
		path := strings.ToLower(loc.Path)
		segments := strings.Split(strings.Trim(path, "/"), "/")
		if len(segments) > 1 && localePattern.MatchString(segments[0]) && segments[0] != "vs" && len(locales) < 30 {
			locales = addUnique(locales, segments[0])
			segments = segments[1:]
		}
		group := "/"
		switch {
		case len(segments) > 1:
			group = "/" + segments[0]
		case segments[0] != "":
			group = "/(root pages)"
		}
		section := byPath[group]
		if section == nil {
			section = &sitemapSection{Path: group}
			byPath[group] = section
		}
		section.Count++
		if len(section.Examples) < 4 {
			section.Examples = addUnique(section.Examples, clip(path, 400))
		}
		if len(section.slugs) < 200 {
			section.slugs = append(section.slugs, clip(segments[len(segments)-1], 150))
		}
		for name, re := range sitemapPatterns {
			if re.MatchString(path) {
				patterns[name]++
			}
		}
		modified := sitemapDate(entry.LastMod)
		age := now.Sub(modified)
		if !modified.IsZero() && age >= 0 && age <= 90*24*time.Hour {
			section.FreshLast90d++
			switch group {
			case "/blog", "/articles", "/posts", "/news", "/resources", "/insights", "/learn", "/guides":
				recent90++
				if age <= 30*24*time.Hour {
					recent30++
				}
			}
		}
	}
	sections := make([]sitemapSection, 0, len(byPath))
	for _, section := range byPath {
		section.Programmatic = section.Path != "/" && section.Path != "/(root pages)" && looksTemplated(section.Count, section.slugs)
		sections = append(sections, *section)
	}
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].Count == sections[j].Count {
			return sections[i].Path < sections[j].Path
		}
		return sections[i].Count > sections[j].Count
	})
	return map[string]any{"totalUrls": len(entries), "sections": sections[:min(15, len(sections))], "locales": locales, "patterns": patterns, "blogPostsLast30d": recent30, "blogPostsLast90d": recent90}
}

func sitemapDate(raw string) time.Time {
	for _, layout := range [...]string{time.RFC3339, time.DateOnly} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func looksTemplated(count int, slugs []string) bool {
	if count < 40 || len(slugs) == 0 {
		return false
	}
	shapes := make(map[string]int)
	for _, slug := range slugs {
		shape := slugToken.ReplaceAllString(slug, "w")
		shape = strings.ReplaceAll(strings.ReplaceAll(shape, "-w-w-w", "-many"), "w-w-w-w", "many")
		shapes[shape]++
	}
	for _, found := range shapes {
		if found*2 > len(slugs) {
			return true
		}
	}
	return count >= 200
}
