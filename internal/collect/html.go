package collect

import (
	"bytes"
	"encoding/json"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"remgo/internal/model"
)

type pageFacts struct {
	Title        string
	Description  string
	SiteName     string
	Organization string
	Lang         string
	Text         string
	FounderText  string
	H1           []string
	H2           []string
	CTAs         []string
	Links        []*url.URL
	JSONLDTypes  []string
	Logos        []model.Logo
	Founders     []model.Founder
	Rating       map[string]float64
}

// PageSummary extracts compact positioning signals from a publicly fetched HTML page.
func PageSummary(body []byte, rawURL string) (title, heading, description string, ctas []string) {
	base, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", nil
	}
	facts, err := readPage(body, base)
	if err != nil {
		return "", "", "", nil
	}
	if len(facts.H1) > 0 {
		heading = facts.H1[0]
	}
	return facts.Title, heading, facts.Description, facts.CTAs[:min(4, len(facts.CTAs))]
}

func readPage(body []byte, base *url.URL) (pageFacts, error) {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return pageFacts{}, err
	}
	facts := pageFacts{H1: []string{}, H2: []string{}, CTAs: []string{}, JSONLDTypes: []string{}}
	linkSeen := make(map[string]bool)
	var visible, prose strings.Builder
	addLink := func(raw string) {
		link := publicLink(base, raw)
		if link != nil && len(facts.Links) < 2500 && !linkSeen[link.String()] {
			linkSeen[link.String()] = true
			facts.Links = append(facts.Links, link)
		}
	}
	for node := root; node != nil; node = nextNode(node) {
		if node.Type == html.TextNode && node.Parent != nil && (node.Parent.Data == "script" || node.Parent.Data == "style" || node.Parent.Data == "title") {
			continue
		}
		if node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
			visible.WriteByte(' ')
			visible.WriteString(node.Data)
			prose.WriteByte(' ')
			prose.WriteString(cleanText(node.Data))
		}
		if node.Type != html.ElementNode {
			continue
		}
		switch node.Data {
		case "h3", "h4", "h5", "h6", "p", "li", "article", "section", "div", "br", "hr":
			prose.WriteByte('\n')
		case "html":
			facts.Lang = clip(attr(node, "lang"), 24)
		case "title":
			facts.Title = nodeText(node, 240)
		case "h1":
			prose.WriteByte('\n')
			if len(facts.H1) < 3 {
				if text := nodeText(node, 200); text != "" {
					facts.H1 = addUnique(facts.H1, text)
				}
			}
		case "h2":
			prose.WriteByte('\n')
			if len(facts.H2) < 10 {
				if text := nodeText(node, 200); text != "" {
					facts.H2 = addUnique(facts.H2, text)
				}
			}
		case "meta":
			content := clip(strings.TrimSpace(attr(node, "content")), 1000)
			if strings.EqualFold(attr(node, "name"), "description") {
				facts.Description = content
			}
			switch strings.ToLower(attr(node, "property")) {
			case "og:description":
				if facts.Description == "" {
					facts.Description = content
				}
			case "og:site_name":
				facts.SiteName = clip(content, 100)
			case "og:image":
				facts.addLogo(base, content, "og-image")
			}
		case "link":
			rel := strings.ToLower(attr(node, "rel"))
			switch {
			case strings.Contains(rel, "apple-touch-icon"):
				facts.addLogo(base, attr(node, "href"), "apple-touch-icon")
			case strings.Contains(rel, "icon") && !strings.Contains(rel, "mask-icon"):
				facts.addLogo(base, attr(node, "href"), "icon")
			}
		case "a", "button":
			if node.Data == "a" {
				addLink(attr(node, "href"))
			}
			if len(facts.CTAs) < 15 {
				text := nodeText(node, 100)
				if len([]rune(text)) >= 3 && len([]rune(text)) <= 55 && (ctaPattern.MatchString(text) || buttonClass.MatchString(attr(node, "class")) || attr(node, "role") == "button") {
					facts.CTAs = addUnique(facts.CTAs, text)
				}
			}
		case "script":
			if strings.EqualFold(attr(node, "type"), "application/ld+json") && node.FirstChild != nil && len(node.FirstChild.Data) <= 512*1024 {
				var data any
				if json.Unmarshal([]byte(node.FirstChild.Data), &data) == nil {
					readJSONLD(data, 0, base, &facts, addLink)
				}
			}
		}
	}
	facts.Text = cleanText(visible.String())
	facts.FounderText = prose.String()
	return facts, nil
}

func nextNode(node *html.Node) *html.Node {
	if node.FirstChild != nil {
		return node.FirstChild
	}
	for node != nil {
		if node.NextSibling != nil {
			return node.NextSibling
		}
		node = node.Parent
	}
	return nil
}

func attr(node *html.Node, key string) string {
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(node *html.Node, limit int) string {
	var output strings.Builder
	end := node
	for end != nil && end.NextSibling == nil {
		end = end.Parent
	}
	if end != nil {
		end = end.NextSibling
	}
	for current := node.FirstChild; current != end && current != nil; current = nextNode(current) {
		if current.Type == html.TextNode && output.Len() < limit*8 {
			if current.Parent.Data == "span" && current.Parent.PrevSibling != nil && current.PrevSibling == nil {
				output.WriteByte(' ')
			}
			output.WriteString(current.Data)
		}
	}
	return clip(collapseRepeats(cleanText(output.String())), limit)
}

func cleanText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func collapseRepeats(text string) string {
	for repeat := 4; repeat >= 2; repeat-- {
		size := len(text) / repeat
		if size >= 8 && size*repeat == len(text) && strings.Repeat(text[:size], repeat) == text {
			return text[:size]
		}
	}
	return text
}

func publicLink(base *url.URL, raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || len(raw) > 3000 {
		return nil
	}
	parsed, err := base.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil
	}
	parsed.Fragment = ""
	return parsed
}

func (p *pageFacts) addLogo(base *url.URL, raw, source string) {
	logo := publicLink(base, raw)
	if logo != nil && len(p.Logos) < 12 {
		p.Logos = addUnique(p.Logos, model.Logo{URL: logo.String(), Source: source})
	}
}

func logoRank(source string) int {
	switch source {
	case "apple-touch-icon":
		return 0
	case "jsonld":
		return 1
	case "icon":
		return 2
	default:
		return 3
	}
}

func asString(value any) string {
	s, _ := value.(string)
	return s
}

func list(value any) []any {
	if value == nil {
		return nil
	}
	if values, ok := value.([]any); ok {
		return values
	}
	return []any{value}
}

func readJSONLD(value any, depth int, base *url.URL, p *pageFacts, addLink func(string)) {
	if depth > 9 {
		return
	}
	switch value := value.(type) {
	case []any:
		for _, child := range value {
			readJSONLD(child, depth+1, base, p, addLink)
		}
	case map[string]any:
		for _, nodeType := range list(value["@type"]) {
			typ := asString(nodeType)
			if typ != "" && len(p.JSONLDTypes) < 20 {
				p.JSONLDTypes = addUnique(p.JSONLDTypes, clip(typ, 80))
			}
			switch typ {
			case "Organization", "Corporation", "Brand", "SoftwareApplication", "WebSite":
				if p.Organization == "" {
					p.Organization = clip(strings.TrimSpace(asString(value["name"])), 100)
				}
				logo := value["logo"]
				if image, ok := logo.(map[string]any); ok {
					logo = image["url"]
					if logo == nil {
						logo = image["contentUrl"]
					}
				}
				p.addLogo(base, asString(logo), "jsonld")
			case "Person":
				role := asString(value["jobTitle"])
				if founderRole.MatchString(role) {
					p.addFounder(base, value, role)
				}
			}
		}
		for _, link := range list(value["sameAs"]) {
			addLink(asString(link))
		}
		for _, person := range list(value["founder"]) {
			p.addFounder(base, person, "Founder")
		}
		if p.Rating == nil {
			if rating, ok := value["aggregateRating"].(map[string]any); ok {
				number, numberOK := numeric(rating["ratingValue"])
				count, countOK := numeric(rating["ratingCount"])
				if !countOK {
					count, countOK = numeric(rating["reviewCount"])
				}
				if numberOK && countOK && number >= 0 && count >= 0 {
					p.Rating = map[string]float64{"value": number, "count": count}
				}
			}
		}
		for _, key := range []string{"@graph", "mainEntity", "publisher", "provider", "author", "creator", "about", "itemListElement"} {
			readJSONLD(value[key], depth+1, base, p, addLink)
		}
	}
}

func numeric(value any) (float64, bool) {
	switch value := value.(type) {
	case float64:
		return value, true
	case string:
		number, err := strconv.ParseFloat(value, 64)
		return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
	}
	return 0, false
}

var (
	ctaPattern   = regexp.MustCompile(`(?i)^(get started|start|try|sign up|signup|join|book|request|schedule|talk to|contact sales|download|get the app|get .{0,20}free|claim|buy|subscribe|install|add to|create .{0,15}account|see (a )?demo|watch (the )?demo|apply)\b`)
	buttonClass  = regexp.MustCompile(`(?i)\b(btn|button|cta)\b`)
	founderRole  = regexp.MustCompile(`(?i)\b(?:co[- ]?)?founder\b|\bceo\b`)
	namePattern  = `[\p{Lu}][\p{Ll}'’.-]{1,28}(?: (?:[\p{Lu}]\.|[\p{Lu}][\p{Ll}'’.-]{1,28})){1,2}`
	founderProse = []*regexp.Regexp{
		regexp.MustCompile(`(?:[Ff]ounded|[Cc]reated|[Ss]tarted) (?:in [0-9]{4} )?by (` + namePattern + `)\b`),
		regexp.MustCompile(`(` + namePattern + `) (?:co-)?founded\b`),
		regexp.MustCompile(`(` + namePattern + `), (?:[Cc]o-?)?[Ff]ounder\b`),
	}
)

func (p *pageFacts) addFounder(base *url.URL, value any, role string) {
	person := model.Founder{Role: clip(role, 80)}
	if obj, ok := value.(map[string]any); ok {
		person.Name = cleanText(asString(obj["name"]))
		if link := publicLink(base, asString(obj["url"])); link != nil {
			person.URL = link.String()
		}
	} else {
		person.Name = cleanText(asString(value))
	}
	if plausiblePerson(person.Name) && len(p.Founders) < 10 {
		p.Founders = append(p.Founders, person)
	}
}

func plausiblePerson(name string) bool {
	parts := strings.Fields(name)
	if len(parts) < 2 || len(parts) > 6 || len([]rune(name)) > 100 {
		return false
	}
	for _, part := range parts {
		if strings.Contains(" founder founders team our meet about company group studio software contact read more ", " "+strings.ToLower(part)+" ") {
			return false
		}
		for _, char := range part {
			if !unicode.IsLetter(char) && !strings.ContainsRune("'’-.", char) {
				return false
			}
		}
	}
	return true
}

func mergeDiscovery(found *model.Discovered, page pageFacts, sourceURL string) {
	for _, logo := range page.Logos {
		if len(found.Logos) < 20 {
			found.Logos = addUnique(found.Logos, logo)
		}
	}
	for _, re := range founderProse {
		for _, match := range re.FindAllStringSubmatch(page.FounderText, 5) {
			if plausiblePerson(match[1]) {
				page.Founders = append(page.Founders, model.Founder{Name: match[1], Role: "Founder"})
			}
		}
	}
	for _, person := range page.Founders {
		if person.URL == "" {
			person.URL = sourceURL
		}
		isNew := len(found.Founders) < 10
		for _, old := range found.Founders {
			if strings.EqualFold(old.Name, person.Name) {
				isNew = false
			}
		}
		if isNew {
			found.Founders = append(found.Founders, person)
		}
	}
	for _, link := range page.Links {
		host := siteHost(link)
		switch host {
		case "apps.apple.com", "itunes.apple.com":
			if m := appleIDPattern.FindStringSubmatch(link.Path); len(m) > 1 && len(found.AppStoreIDs) < 10 {
				found.AppStoreIDs = addUnique(found.AppStoreIDs, m[1])
			}
		case "play.google.com":
			id := link.Query().Get("id")
			if link.Path == "/store/apps/details" && playIDPattern.MatchString(id) && len(found.PlayIDs) < 10 {
				found.PlayIDs = addUnique(found.PlayIDs, id)
			}
		}
		platform, handle := socialAccount(host, link.Path)
		if handle != "" && found.Socials[platform] == "" {
			found.Socials[platform] = handle
		}
	}
}

var handlePattern = regexp.MustCompile(`^@?[A-Za-z0-9][A-Za-z0-9_.%-]{0,79}$`)

func socialAccount(host, path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	first := parts[0]
	if !handlePattern.MatchString(first) {
		return "", ""
	}
	second := ""
	if len(parts) > 1 && handlePattern.MatchString(parts[1]) {
		second = parts[1]
	}
	switch host {
	case "tiktok.com":
		if strings.HasPrefix(first, "@") {
			return "tiktok", first[1:]
		}
	case "instagram.com":
		if len(parts) == 1 && !reservedSocialPath(first) {
			return "instagram", first
		}
	case "youtube.com":
		if strings.HasPrefix(first, "@") {
			return "youtube", first
		}
		if second != "" && (first == "c" || first == "channel" || first == "user") {
			return "youtube", first + "/" + second
		}
	case "x.com", "twitter.com":
		if !reservedSocialPath(first) {
			return "x", first
		}
	case "linkedin.com":
		if first == "company" {
			return "linkedin", second
		}
	case "facebook.com":
		if !reservedSocialPath(first) {
			return "facebook", first
		}
	case "threads.net", "threads.com":
		if strings.HasPrefix(first, "@") {
			return "threads", first[1:]
		}
	case "reddit.com":
		if first == "r" {
			return "reddit", second
		}
	case "github.com":
		if !reservedSocialPath(first) {
			return "github", first
		}
	case "producthunt.com":
		if first == "products" || first == "posts" {
			return "producthunt", second
		}
	case "discord.gg":
		return "discord", host + "/" + first
	case "discord.com":
		if first == "invite" && second != "" {
			return "discord", host + "/invite/" + second
		}
	}
	return "", ""
}

func reservedSocialPath(path string) bool {
	switch strings.ToLower(path) {
	case "p", "reel", "reels", "explore", "accounts", "intent", "share", "sharer", "home", "search", "i", "dialog", "plugins", "tr", "login", "features", "topics":
		return true
	default:
		return false
	}
}
