package collect

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"golang.org/x/net/html"
)

type trackingTag struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Implies  string   `json:"implies,omitempty"`
	IDs      []string `json:"ids"`
	FoundIn  []string `json:"foundIn"`
}

type signature struct {
	ID, Name, Category, Channel string
	pattern                     *regexp.Regexp
	idPattern                   *regexp.Regexp
}

func tag(id, name, category, channel, pattern, idPattern string) signature {
	s := signature{ID: id, Name: name, Category: category, Channel: channel, pattern: regexp.MustCompile(pattern)}
	if idPattern != "" {
		s.idPattern = regexp.MustCompile(idPattern)
	}
	return s
}

var signatures = []signature{
	tag("meta_pixel", "Meta Pixel", "ads_pixel", "paid_meta", `(?i)connect\.facebook\.net/[^"']*fbevents\.js|fbq\(\s*['"]init['"]|facebook\.com/tr\?`, `(?i)fbq\(\s*['"]init['"]\s*,\s*['"](\d{8,20})['"]`),
	tag("google_ads", "Google Ads conversion tag", "ads_pixel", "paid_google", `(?i)\bAW-\d{9,12}\b|googleadservices\.com/pagead/conversion`, `(?i)\b(AW-\d{9,12})\b`),
	tag("tiktok_pixel", "TikTok Pixel", "ads_pixel", "paid_tiktok", `(?i)analytics\.tiktok\.com|ttq\.load\(`, `(?i)ttq\.load\(\s*['"]([A-Z0-9]{10,30})['"]`),
	tag("linkedin_insight", "LinkedIn Insight Tag", "ads_pixel", "paid_linkedin", `(?i)snap\.licdn\.com/li\.lms-analytics|_linkedin_partner_id`, `(?i)_linkedin_partner_id\s*=\s*['"]?(\d{4,12})`),
	tag("x_pixel", "X Pixel", "ads_pixel", "paid_x", `(?i)static\.ads-twitter\.com/uwt\.js|twq\(\s*['"](?:config|init)['"]`, `(?i)twq\(\s*['"](?:config|init)['"]\s*,\s*['"]([a-z0-9]{4,10})['"]`),
	tag("reddit_pixel", "Reddit Pixel", "ads_pixel", "paid_reddit", `(?i)redditstatic\.com/ads/pixel\.js|rdt\(\s*['"]init['"]`, `(?i)rdt\(\s*['"]init['"]\s*,\s*['"]([at]2_[a-z0-9]+)['"]`),
	tag("pinterest_tag", "Pinterest Tag", "ads_pixel", "paid_pinterest", `(?i)s\.pinimg\.com/ct/core\.js|pintrk\(\s*['"]load['"]`, `(?i)pintrk\(\s*['"]load['"]\s*,\s*['"](\d{8,16})['"]`),
	tag("snap_pixel", "Snap Pixel", "ads_pixel", "paid_snapchat", `(?i)sc-static\.net/scevent\.min\.js|snaptr\(\s*['"]init['"]`, ""),
	tag("bing_uet", "Microsoft Ads UET", "ads_pixel", "paid_microsoft", `(?i)bat\.bing\.com/bat\.js`, ""),
	tag("quora_pixel", "Quora Pixel", "ads_pixel", "paid_quora", `(?i)a\.quora\.com/qevents\.js`, ""),
	tag("google_dv360", "Google Floodlight", "ads_pixel", "paid_programmatic", `(?i)fls\.doubleclick\.net|\bDC-\d{6,10}\b`, `(?i)\b(DC-\d{6,10})\b`),
	tag("gtm", "Google Tag Manager", "tag_manager", "", `\bGTM-[A-Z0-9]{4,12}\b`, `\b(GTM-[A-Z0-9]{4,12})\b`),
	tag("segment", "Segment", "tag_manager", "", `(?i)cdn\.segment\.com/analytics\.js`, ""),
	tag("rudderstack", "RudderStack", "tag_manager", "", `(?i)cdn\.rudderlabs\.com`, ""),
	tag("ga4", "Google Analytics 4", "analytics", "", `(?i)\bG-[A-Z0-9]{6,12}\b`, `(?i)\b(G-[A-Z0-9]{6,12})\b`),
	tag("posthog", "PostHog", "analytics", "", `(?i)posthog\.init\(|[a-z]+\.i\.posthog\.com|app\.posthog\.com`, ""),
	tag("amplitude", "Amplitude", "analytics", "", `(?i)cdn\.amplitude\.com|amplitude\.getInstance\(`, ""),
	tag("mixpanel", "Mixpanel", "analytics", "", `(?i)cdn\.mxpnl\.com|mixpanel\.init\(`, ""),
	tag("hotjar", "Hotjar", "analytics", "", `(?i)static\.hotjar\.com`, ""),
	tag("clarity", "Microsoft Clarity", "analytics", "", `(?i)clarity\.ms/tag`, ""),
	tag("plausible", "Plausible", "analytics", "", `(?i)plausible\.io/js`, ""),
	tag("fathom", "Fathom", "analytics", "", `(?i)cdn\.usefathom\.com`, ""),
	tag("heap", "Heap", "analytics", "", `(?i)cdn\.heapanalytics\.com`, ""),
	tag("vercel_analytics", "Vercel Analytics", "analytics", "", `(?i)/_vercel/insights`, ""),
	tag("apple_search_ads", "Apple Search Ads attribution", "attribution", "paid_apple", `(?i)searchads\.apple\.com`, ""),
	tag("appsflyer", "AppsFlyer", "attribution", "app_install_ads", `(?i)onelink\.me|appsflyer\.com`, ""),
	tag("branch", "Branch", "attribution", "app_install_ads", `(?i)cdn\.branch\.io|\.app\.link/`, ""),
	tag("adjust", "Adjust", "attribution", "app_install_ads", `(?i)\badj\.st/|app\.adjust\.com`, ""),
	tag("singular", "Singular", "attribution", "app_install_ads", `(?i)sng\.link`, ""),
	tag("rewardful", "Rewardful", "affiliate", "affiliate", `(?i)r\.wdfl\.co|rewardful\.com`, ""),
	tag("firstpromoter", "FirstPromoter", "affiliate", "affiliate", `(?i)firstpromoter\.com`, ""),
	tag("partnerstack", "PartnerStack", "affiliate", "affiliate", `(?i)partnerstack\.com`, ""),
	tag("tolt", "Tolt", "affiliate", "affiliate", `(?i)cdn\.tolt\.io`, ""),
	tag("impact", "impact.com", "affiliate", "affiliate", `(?i)impactradius|impact-ad\.|utt\.impactcdn`, ""),
	tag("dub", "Dub Partners", "affiliate", "affiliate", `(?i)dubcdn\.com|@dub/analytics`, ""),
	tag("growsurf", "GrowSurf", "affiliate", "affiliate", `(?i)growsurf\.com`, ""),
	tag("viral_loops", "Viral Loops", "affiliate", "affiliate", `(?i)viral-loops\.com|viralloops\.com`, ""),
	tag("hubspot", "HubSpot", "email", "lifecycle", `(?i)js\.hs-scripts\.com|js\.hsforms\.net|js\.hs-analytics\.net`, ""),
	tag("klaviyo", "Klaviyo", "email", "lifecycle", `(?i)static\.klaviyo\.com`, ""),
	tag("mailchimp", "Mailchimp", "email", "lifecycle", `(?i)chimpstatic\.com|list-manage\.com`, ""),
	tag("customerio", "Customer.io", "email", "lifecycle", `(?i)assets\.customer\.io`, ""),
	tag("kit", "Kit", "email", "lifecycle", `(?i)convertkit\.com|\.ck\.page`, ""),
	tag("beehiiv", "beehiiv", "email", "lifecycle", `(?i)embeds\.beehiiv\.com|beehiiv\.com/subscribe`, ""),
	tag("loops", "Loops", "email", "lifecycle", `(?i)app\.loops\.so`, ""),
	tag("braze", "Braze", "email", "lifecycle", `(?i)js\.appboycdn\.com|sdk\.iad-\d+\.braze\.com`, ""),
	tag("iterable", "Iterable", "email", "lifecycle", `(?i)iterable\.com/api|js\.iterable\.com`, ""),
	tag("intercom", "Intercom", "chat", "", `(?i)widget\.intercom\.io|js\.intercomcdn\.com`, ""),
	tag("crisp", "Crisp", "chat", "", `(?i)client\.crisp\.chat`, ""),
	tag("drift", "Drift", "chat", "sales_led", `(?i)js\.driftt\.com`, ""),
	tag("zendesk", "Zendesk", "chat", "", `(?i)static\.zdassets\.com`, ""),
	tag("rb2b", "RB2B", "visitor_id", "sales_led", `(?i)rb2b\.com`, ""),
	tag("warmly", "Warmly", "visitor_id", "sales_led", `(?i)warmly\.ai`, ""),
	tag("koala", "Koala", "visitor_id", "sales_led", `(?i)getkoala\.com`, ""),
	tag("clearbit", "Clearbit Reveal", "visitor_id", "sales_led", `(?i)tag\.clearbitscripts\.com|reveal\.clearbit\.com`, ""),
	tag("6sense", "6sense", "visitor_id", "sales_led", `(?i)j\.6sc\.co`, ""),
	tag("demandbase", "Demandbase", "visitor_id", "sales_led", `(?i)tag\.demandbase\.com`, ""),
	tag("leadfeeder", "Leadfeeder", "visitor_id", "sales_led", `(?i)lftracker|sc\.lfeeder\.com`, ""),
	tag("apollo", "Apollo", "visitor_id", "sales_led", `(?i)assets\.apollo\.io/micro/website-tracker`, ""),
	tag("calendly", "Calendly", "sales", "sales_led", `(?i)calendly\.com/`, ""),
	tag("calcom", "Cal.com", "sales", "sales_led", `(?i)cal\.com/[a-z0-9-]+`, ""),
	tag("chilipiper", "Chili Piper", "sales", "sales_led", `(?i)chilipiper\.com`, ""),
	tag("optimizely", "Optimizely", "experimentation", "", `(?i)cdn\.optimizely\.com`, ""),
	tag("vwo", "VWO", "experimentation", "", `(?i)visualwebsiteoptimizer\.com`, ""),
	tag("statsig", "Statsig", "experimentation", "", `(?i)statsig\.com`, ""),
	tag("growthbook", "GrowthBook", "experimentation", "", `(?i)growthbook\.io`, ""),
	tag("trustpilot", "Trustpilot widget", "reviews", "", `(?i)widget\.trustpilot\.com`, ""),
	tag("g2", "G2 badge", "reviews", "", `(?i)g2\.com/products|g2crowd\.com`, ""),
	tag("senja", "Senja testimonials", "reviews", "", `(?i)senja\.io`, ""),
	tag("testimonial_to", "Testimonial.to", "reviews", "", `(?i)testimonial\.to`, ""),
	tag("producthunt_badge", "Product Hunt badge", "reviews", "launches", `(?i)api\.producthunt\.com/widgets/embed-image`, ""),
	tag("stripe", "Stripe", "payments", "", `(?i)js\.stripe\.com|buy\.stripe\.com`, ""),
	tag("paddle", "Paddle", "payments", "", `(?i)cdn\.paddle\.com`, ""),
	tag("lemonsqueezy", "Lemon Squeezy", "payments", "", `(?i)lemonsqueezy\.com`, ""),
	tag("shopify", "Shopify", "payments", "", `(?i)cdn\.shopify\.com`, ""),
	tag("webflow", "Webflow", "site_builder", "", `(?i)data-wf-site|assets\.website-files\.com|cdn\.prod\.website-files\.com`, ""),
	tag("framer", "Framer", "site_builder", "", `(?i)framerusercontent\.com|framer\.com/m/`, ""),
	tag("wordpress", "WordPress", "site_builder", "", `(?i)/wp-content/`, ""),
	tag("nextjs", "Next.js", "site_builder", "", `(?i)/_next/static/`, ""),
	tag("wix", "Wix", "site_builder", "", `(?i)static\.wixstatic\.com`, ""),
	tag("squarespace", "Squarespace", "site_builder", "", `(?i)static1\.squarespace\.com`, ""),
}

func trackingMarkup(body []byte) string {
	var markup strings.Builder
	inScript := false
	tokens := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return markup.String()
		case html.StartTagToken, html.SelfClosingTagToken:
			node := tokens.Token()
			inScript = node.Data == "script"
			for _, a := range node.Attr {
				if a.Key == "type" && a.Val != "" {
					typ := strings.ToLower(strings.TrimSpace(a.Val))
					inScript = inScript && (typ == "module" || typ == "text/javascript" || typ == "application/javascript")
				}
				switch a.Key {
				case "src", "data-src", "data-wf-site":
					markup.WriteString(a.Key + "=" + a.Val + "\n")
				case "href":
					if node.Data == "link" {
						markup.WriteString(a.Val + "\n")
					}
				}
			}
		case html.TextToken:
			if inScript {
				markup.Write(tokens.Text())
				markup.WriteByte('\n')
			}
		case html.EndTagToken:
			inScript = false
		}
	}
}

func detectTracking(text, location string) []trackingTag {
	var tags []trackingTag
	for _, s := range signatures {
		if !s.pattern.MatchString(text) {
			continue
		}
		tag := trackingTag{ID: s.ID, Name: s.Name, Category: s.Category, Implies: s.Channel, IDs: []string{}, FoundIn: []string{location}}
		if s.idPattern != nil {
			for _, match := range s.idPattern.FindAllStringSubmatch(text, 12) {
				tag.IDs = addUnique(tag.IDs, match[1])
				if len(tag.IDs) >= 5 {
					break
				}
			}
		}
		tags = append(tags, tag)
	}
	return tags
}

func mergeTracking(existing []trackingTag, newTags []trackingTag) []trackingTag {
	for _, added := range newTags {
		if added.ID == "gtm" {
			continue
		}
		matched := false
		for i := range existing {
			current := &existing[i]
			if current.ID != added.ID {
				continue
			}
			matched = true
			for _, id := range added.IDs {
				if len(current.IDs) < 5 {
					current.IDs = addUnique(current.IDs, id)
				}
			}
			for _, location := range added.FoundIn {
				current.FoundIn = addUnique(current.FoundIn, location)
			}
			break
		}
		if !matched {
			existing = append(existing, added)
		}
	}
	return existing
}

func collectTracking(ctx context.Context, get fetch.Getter, homepage []byte, homeURL string, result *model.Result) {
	tags := detectTracking(trackingMarkup(homepage), "page")
	scanned := make([]string, 0, 2)
	var gtmIDs []string
	for _, tag := range tags {
		if tag.ID == "gtm" {
			gtmIDs = tag.IDs[:min(2, len(tag.IDs))]
			break
		}
	}
	for _, id := range gtmIDs {
		rawURL := "https://www.googletagmanager.com/gtm.js?id=" + id
		response, err := get.Get(ctx, rawURL)
		if problem := unavailable(response, err); problem != "" {
			result.Unknowns = append(result.Unknowns, unknown("tracking", "Could not read GTM container "+id+": "+problem, rawURL))
			continue
		}
		scanned = append(scanned, id)
		tags = mergeTracking(tags, detectTracking(string(response.Body), id))
	}
	pixels := make([]trackingTag, 0)
	other := make([]trackingTag, 0)
	var pixelNames, stackNames []string
	byCategory := make(map[string][]string)
	for _, tag := range tags {
		if tag.Category == "ads_pixel" || tag.Category == "attribution" {
			pixels = append(pixels, tag)
			pixelNames = append(pixelNames, tag.Name)
		} else {
			other = append(other, tag)
			stackNames = append(stackNames, tag.Name)
			byCategory[tag.Category] = append(byCategory[tag.Category], tag.Name)
		}
	}
	title := "No ad pixels detected in inspected homepage resources"
	if len(pixels) > 0 {
		title = "Ad/attribution pixels: " + strings.Join(pixelNames, ", ")
	}
	result.Evidence = append(result.Evidence, evidence("tracking", "observation", title, homeURL, map[string]any{"pixels": pixels, "gtmContainersScanned": scanned}))
	if len(other) > 0 {
		result.Evidence = append(result.Evidence, evidence("tracking", "observation", "Marketing stack: "+strings.Join(stackNames, ", "), homeURL, map[string]any{"tags": other, "byCategory": byCategory}))
	}
}
