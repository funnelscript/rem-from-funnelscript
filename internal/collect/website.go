package collect

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"remgo/internal/fetch"
	"remgo/internal/model"
)

func Website(ctx context.Context, get fetch.Getter, target model.Target) (model.Target, model.Discovered, model.Result) {
	found := model.Discovered{Socials: make(map[string]string)}
	var result model.Result
	if target.URL == "" {
		result.Unknowns = append(result.Unknowns, unknown("website", "No developer website was found", ""))
		return target, found, result
	}

	home, err := get.Get(ctx, target.URL)
	if problem := unavailable(home, err); problem != "" {
		result.Unknowns = append(result.Unknowns, unknown("website", problem, target.URL))
		return target, found, result
	}
	homeURL := home.URL
	if homeURL == "" {
		homeURL = target.URL
	}
	base, err := url.Parse(homeURL)
	if err != nil || base.Hostname() == "" {
		result.Unknowns = append(result.Unknowns, unknown("website", "The final homepage URL is invalid", target.URL))
		return target, found, result
	}

	homeDomain := siteHost(base)
	if target.Domain != "" && !withinDomain(homeDomain, target.Domain) {
		target.Aliases = addUnique(target.Aliases, target.Domain)
		target.Domain = homeDomain
	}
	if target.Domain == "" {
		target.Domain = homeDomain
	}
	target.URL = origin(base)

	facts, err := readPage(home.Body, base)
	if err != nil {
		result.Unknowns = append(result.Unknowns, unknown("website", "Could not parse the homepage HTML", homeURL))
		return target, found, result
	}
	titleBrand := strings.TrimSpace(titleSeparator.Split(facts.Title, 2)[0])
	switch {
	case facts.SiteName != "":
		found.BrandName = facts.SiteName
	case facts.Organization != "":
		found.BrandName = facts.Organization
	case titleBrand != "" && len([]rune(titleBrand)) <= 30:
		found.BrandName = titleBrand
	default:
		found.BrandName = target.Brand
	}
	if found.BrandName != "" {
		target.Brand = found.BrandName
	}
	mergeDiscovery(&found, facts, homeURL)
	heading := facts.Title
	if len(facts.H1) > 0 {
		heading = facts.H1[0]
	}
	result.Evidence = append(result.Evidence, evidence("website", "observation", "Homepage positioning: "+strconv.Quote(clip(heading, 120)), homeURL, map[string]any{
		"title": facts.Title, "metaDescription": facts.Description, "h1": facts.H1, "h2": facts.H2,
		"ctas": facts.CTAs, "lang": facts.Lang, "wordCount": len(strings.Fields(facts.Text)),
		"socialProof": socialProof.FindAllString(facts.Text, 5), "jsonLdTypes": facts.JSONLDTypes,
		"rating": facts.Rating,
	}))

	pages := keyPages(base, facts.Links)
	result.Evidence = append(result.Evidence, evidence("website", "observation", "Key pages found: "+keysLabel(pages), homeURL, pages))
	if tags := campaignLinks(facts.Links); len(tags) != 0 {
		result.Evidence = append(result.Evidence, evidence("website", "observation", fmt.Sprintf("%d tagged links on homepage", len(tags)), homeURL, tags))
	}
	collectTracking(ctx, get, home.Body, homeURL, &result)

	type keyPageResult struct {
		response fetch.Response
		err      error
	}
	kinds := [...]string{"pricing", "about", "affiliate", "demo"}
	checked := make([]keyPageResult, len(kinds))
	var pending sync.WaitGroup
	for index, kind := range kinds {
		if pageURL := pages[kind]; pageURL != "" {
			pending.Go(func() { checked[index].response, checked[index].err = get.Get(ctx, pageURL) })
		}
	}
	pending.Wait()
	pricingText, pricingURL := facts.Text, homeURL
	pricingRead := false
	for index, kind := range kinds {
		pageURL := pages[kind]
		if pageURL == "" {
			continue
		}
		response, err := checked[index].response, checked[index].err
		if problem := unavailable(response, err); problem != "" {
			result.Unknowns = append(result.Unknowns, unknown(kind, problem, pageURL))
			continue
		}
		resolved := resolvedPageURL(response.URL, pageURL, base)
		if resolved == nil {
			result.Unknowns = append(result.Unknowns, unknown(kind, "The page redirected outside the target site", pageURL))
			continue
		}
		page, err := readPage(response.Body, resolved)
		if err != nil {
			result.Unknowns = append(result.Unknowns, unknown(kind, "Could not parse the page HTML", pageURL))
			continue
		}
		mergeDiscovery(&found, page, resolved.String())
		data := map[string]any{"h1": page.H1, "h2": page.H2, "ctas": page.CTAs}
		switch kind {
		case "pricing":
			pricingText, pricingURL, pricingRead = page.Text, resolved.String(), true
		case "about":
			data["excerpt"] = clip(page.Text, 1200)
			result.Evidence = append(result.Evidence, evidence("website", "observation", "About/team page", resolved.String(), data))
		case "affiliate":
			data["excerpt"] = clip(page.Text, 800)
			data["commission"] = commissionPattern.FindString(page.Text)
			result.Evidence = append(result.Evidence, evidence("website", "observation", "Affiliate / partner program page exists", resolved.String(), data))
		case "demo":
			result.Evidence = append(result.Evidence, evidence("website", "observation", "Demo / contact-sales page exists", resolved.String(), data))
		}
	}
	pricing := pricingSignals(pricingText)
	pricing["pricingPageFound"] = pages["pricing"] != ""
	pricing["pricingPageRead"] = pricingRead
	result.Evidence = append(result.Evidence,
		evidence("website", "derivation", "Monetization: "+pricingLabel(pricing), pricingURL, pricing),
		evidence("website", "observation", "Linked channels: "+keysLabel(found.Socials), homeURL,
			map[string]any{"socials": found.Socials, "appStoreIds": found.AppStoreIDs, "playIds": found.PlayIDs}),
	)
	sort.SliceStable(found.Logos, func(i, j int) bool { return logoRank(found.Logos[i].Source) < logoRank(found.Logos[j].Source) })
	return target, found, result
}

var (
	titleSeparator    = regexp.MustCompile(`\s[|–—:-]\s`)
	commissionPattern = regexp.MustCompile(`(?i)\b\d{1,3}\s?%[^.]{0,55}(?:commission|revenue|lifetime)`)
	socialProof       = regexp.MustCompile(`(?i)\b(?:trusted by|as seen on|as featured in|backed by)\s+[^.!?]{3,65}|\b(?:over\s+)?\d[\d,.]*\+?\s*(?:k|m|million|thousand)?\s+(?:customers|users|teams|companies|downloads)\b`)
	appleIDPattern    = regexp.MustCompile(`(?:^|/)id([0-9]{6,})(?:/|$)`)
	playIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)+$`)
	pricePattern      = regexp.MustCompile(`(?:[$€£]\s?\d{1,5}(?:[.,]\d{2})?)(?:\s?/\s?(?:month|year|user|seat|week|mo|yr)\b)?`)
	trialPattern      = regexp.MustCompile(`(?i)\b(\d{1,2})[- ]days?\s+(?:free\s+)?trial\b`)
)

var keyPagePatterns = []struct {
	name string
	path *regexp.Regexp
}{
	{"pricing", regexp.MustCompile(`/(pricing|plans|price|upgrade)(/|$)`)},
	{"about", regexp.MustCompile(`/(about|about-us|company|team|our-story|founders?)(/|$)`)},
	{"affiliate", regexp.MustCompile(`/(affiliates?|partners?|referral|refer|ambassadors?|creators?-program|partner-program)(/|$)`)},
	{"customers", regexp.MustCompile(`/(customers|case-studies|stories|testimonials|wall-of-love)(/|$)`)},
	{"blog", regexp.MustCompile(`/(blog|articles|resources|insights|news)(/|$)`)},
	{"careers", regexp.MustCompile(`/(careers|jobs|hiring)(/|$)`)},
	{"changelog", regexp.MustCompile(`/(changelog|releases|whats-new|updates)(/|$)`)},
	{"compare", regexp.MustCompile(`/(vs|compare|alternatives?|comparison)(/|-|$)`)},
	{"integrations", regexp.MustCompile(`/(integrations|apps|marketplace)(/|$)`)},
	{"templates", regexp.MustCompile(`/(templates|gallery|examples|showcase)(/|$)`)},
	{"demo", regexp.MustCompile(`/(demo|book-a-demo|contact-sales|talk-to-sales|get-a-demo)(/|$)`)},
}

func evidence(source, kind, title, rawURL string, data any) model.Evidence {
	return model.Evidence{Source: source, Kind: kind, Title: title, URL: rawURL, Data: data}
}

func unknown(channel, reason, rawURL string) model.Unknown {
	return model.Unknown{Channel: channel, Reason: reason, ManualCheck: rawURL}
}

func unavailable(response fetch.Response, err error) string {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "The request was cancelled or timed out"
		}
		return "The request failed: " + clip(err.Error(), 260)
	}
	if response.Status < 200 || response.Status >= 300 {
		return fmt.Sprintf("HTTP %d", response.Status)
	}
	return ""
}

func keyPages(base *url.URL, links []*url.URL) map[string]string {
	pages := make(map[string]string)
	for _, link := range links {
		if !withinDomain(siteHost(link), siteHost(base)) {
			continue
		}
		for _, kind := range keyPagePatterns {
			if !kind.path.MatchString(strings.ToLower(link.Path)) {
				continue
			}
			pageURL := origin(link) + link.EscapedPath()
			// Prefer the shortest matching path to a nested article.
			if old := pages[kind.name]; old == "" || len(pageURL) < len(old) {
				pages[kind.name] = pageURL
			}
		}
	}
	return pages
}

func resolvedPageURL(rawURL, fallback string, base *url.URL) *url.URL {
	if rawURL == "" {
		rawURL = fallback
	}
	resolved, err := url.Parse(rawURL)
	if err != nil || !withinDomain(siteHost(resolved), siteHost(base)) {
		return nil
	}
	return resolved
}

func siteHost(u *url.URL) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSuffix(u.Hostname(), ".")), "www.")
}

func withinDomain(host, domain string) bool {
	return host != "" && domain != "" && (host == domain || strings.HasSuffix(host, "."+domain))
}

func origin(u *url.URL) string {
	return u.Scheme + "://" + u.Host
}

func clip(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}

func addUnique[T comparable](values []T, value T) []T {
	for _, old := range values {
		if old == value {
			return values
		}
	}
	return append(values, value)
}

func keysLabel[T any](m map[string]T) string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "none found"
	}
	return strings.Join(keys, ", ")
}

func campaignLinks(links []*url.URL) []map[string]string {
	var tags []map[string]string
	for _, link := range links {
		if len(tags) >= 20 {
			break
		}
		query := link.Query()
		isTagged := false
		for k := range query {
			if strings.HasPrefix(k, "utm_") || k == "ref" || k == "via" {
				isTagged = true
				break
			}
		}
		if isTagged {
			source := query.Get("utm_source")
			if source == "" {
				source = query.Get("ref")
			}
			if source == "" {
				source = query.Get("via")
			}
			tags = append(tags, map[string]string{"url": origin(link) + link.EscapedPath(), "source": clip(source, 100), "medium": clip(query.Get("utm_medium"), 100), "campaign": clip(query.Get("utm_campaign"), 100)})
		}
	}
	return tags
}

var pricingPatterns = map[string]*regexp.Regexp{
	"freePlan":       regexp.MustCompile(`(?i)\bfree (?:plan|forever|tier)\b|\$0\b`),
	"freeTrial":      regexp.MustCompile(`(?i)\bfree trial\b|try (?:it )?free|start free`),
	"moneyBack":      regexp.MustCompile(`(?i)money[- ]back|refund guarantee`),
	"contactSales":   regexp.MustCompile(`(?i)contact sales|talk to sales|book a demo|get a demo|request a demo`),
	"perSeat":        regexp.MustCompile(`(?i)per (?:user|seat|member)|/\s?(?:user|seat)`),
	"annualDiscount": regexp.MustCompile(`(?i)billed annually|save \d{1,2}%|annual(?:ly)? .{0,20}(?:save|off|discount)`),
	"lifetimeDeal":   regexp.MustCompile(`(?i)lifetime (?:deal|access|plan)|one[- ]time payment|pay once`),
	"usageBased":     regexp.MustCompile(`(?i)pay as you go|usage[- ]based|per (?:credit|request|minute|token|call)`),
}

func pricingSignals(text string) map[string]any {
	prices := make([]string, 0)
	for _, price := range pricePattern.FindAllString(text, 30) {
		prices = addUnique(prices, strings.TrimSpace(price))
		if len(prices) >= 12 {
			break
		}
	}
	signals := map[string]any{"prices": prices}
	for key, pattern := range pricingPatterns {
		signals[key] = pattern.MatchString(text)
	}
	if trial := trialPattern.FindStringSubmatch(text); len(trial) > 1 {
		signals["trialDays"], _ = strconv.Atoi(trial[1])
		signals["freeTrial"] = true
	}
	return signals
}

func pricingLabel(signals map[string]any) string {
	prices, _ := signals["prices"].([]string)
	var summary []string
	if signals["freePlan"] == true {
		summary = append(summary, "free plan")
	}
	if signals["freeTrial"] == true {
		summary = append(summary, "trial")
	}
	if len(prices) > 0 {
		summary = append(summary, "prices "+strings.Join(prices[:min(4, len(prices))], ", "))
	}
	if signals["perSeat"] == true {
		summary = append(summary, "per seat")
	}
	if signals["usageBased"] == true {
		summary = append(summary, "usage based")
	}
	if signals["lifetimeDeal"] == true {
		summary = append(summary, "one-time payment")
	}
	if signals["contactSales"] == true {
		summary = append(summary, "sales-assisted CTA")
	}
	if len(summary) == 0 {
		return "no price or offer detected in the inspected page"
	}
	return strings.Join(summary, " · ")
}
