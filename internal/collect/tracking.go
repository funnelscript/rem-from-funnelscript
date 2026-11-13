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

// Match complete vendor hostnames in markup and GTM text. Resource endings
// also reject lookalike suffixes, while allowing delimiters such as ? and /.
var signatureBoundaries = strings.NewReplacer(
	"<host>", `(?:^|[^a-z0-9._-])(?:[a-z0-9-]+\.)*`,
	"<subdomain>", `(?:^|[^a-z0-9._-])[a-z0-9-]+(?:\.[a-z0-9-]+)*`,
	"<end>", `(?:$|[^a-z0-9._-])`,
)

func tag(id, name, category, channel, pattern, idPattern string) signature {
	s := signature{ID: id, Name: name, Category: category, Channel: channel, pattern: regexp.MustCompile(signatureBoundaries.Replace(pattern))}
	if idPattern != "" {
		s.idPattern = regexp.MustCompile(idPattern)
	}
	return s
}

var signatures = []signature{
	tag("meta_pixel", "Meta Pixel", "ads_pixel", "paid_meta", `(?i)<host>connect\.facebook\.net/[^"']*fbevents\.js<end>|fbq\(\s*['"]init['"]|<host>facebook\.com/tr\?`, `(?i)fbq\(\s*['"]init['"]\s*,\s*['"](\d{8,20})['"]`),
	tag("google_ads", "Google Ads conversion tag", "ads_pixel", "paid_google", `(?i)\bAW-\d{9,12}\b|<host>googleadservices\.com/pagead/conversion<end>`, `(?i)\b(AW-\d{9,12})\b`),
	tag("tiktok_pixel", "TikTok Pixel", "ads_pixel", "paid_tiktok", `(?i)<host>analytics\.tiktok\.com<end>|ttq\.load\(`, `(?i)ttq\.load\(\s*['"]([A-Z0-9]{10,30})['"]`),
	tag("linkedin_insight", "LinkedIn Insight Tag", "ads_pixel", "paid_linkedin", `(?i)<host>snap\.licdn\.com/li\.lms-analytics<end>|_linkedin_partner_id`, `(?i)_linkedin_partner_id\s*=\s*['"]?(\d{4,12})`),
	tag("x_pixel", "X Pixel", "ads_pixel", "paid_x", `(?i)<host>static\.ads-twitter\.com/uwt\.js<end>|twq\(\s*['"](?:config|init)['"]`, `(?i)twq\(\s*['"](?:config|init)['"]\s*,\s*['"]([a-z0-9]{4,10})['"]`),
	tag("reddit_pixel", "Reddit Pixel", "ads_pixel", "paid_reddit", `(?i)<host>redditstatic\.com/ads/pixel\.js<end>|rdt\(\s*['"]init['"]`, `(?i)rdt\(\s*['"]init['"]\s*,\s*['"]([at]2_[a-z0-9]+)['"]`),
	tag("pinterest_tag", "Pinterest Tag", "ads_pixel", "paid_pinterest", `(?i)<host>s\.pinimg\.com/ct/core\.js<end>|pintrk\(\s*['"]load['"]`, `(?i)pintrk\(\s*['"]load['"]\s*,\s*['"](\d{8,16})['"]`),
	tag("snap_pixel", "Snap Pixel", "ads_pixel", "paid_snapchat", `(?i)<host>sc-static\.net/scevent\.min\.js<end>|snaptr\(\s*['"]init['"]`, ""),
	tag("bing_uet", "Microsoft Ads UET", "ads_pixel", "paid_microsoft", `(?i)<host>bat\.bing\.com/bat\.js<end>`, ""),
	tag("quora_pixel", "Quora Pixel", "ads_pixel", "paid_quora", `(?i)<host>a\.quora\.com/qevents\.js<end>`, ""),
	tag("google_dv360", "Google Floodlight", "ads_pixel", "paid_programmatic", `(?i)<host>fls\.doubleclick\.net<end>|\bDC-\d{6,10}\b`, `(?i)\b(DC-\d{6,10})\b`),
	tag("gtm", "Google Tag Manager", "tag_manager", "", `\bGTM-[A-Z0-9]{4,12}\b`, `\b(GTM-[A-Z0-9]{4,12})\b`),
	tag("segment", "Segment", "tag_manager", "", `(?i)<host>cdn\.segment\.com/analytics\.js<end>`, ""),
	tag("rudderstack", "RudderStack", "tag_manager", "", `(?i)<host>cdn\.rudderlabs\.com<end>`, ""),
	tag("ga4", "Google Analytics 4", "analytics", "", `(?i)\bG-[A-Z0-9]{6,12}\b`, `(?i)\b(G-[A-Z0-9]{6,12})\b`),
	tag("posthog", "PostHog", "analytics", "", `(?i)posthog\.init\(|<host>[a-z]+\.i\.posthog\.com<end>|<host>app\.posthog\.com<end>`, ""),
	tag("amplitude", "Amplitude", "analytics", "", `(?i)<host>cdn\.amplitude\.com<end>|amplitude\.getInstance\(`, ""),
	tag("mixpanel", "Mixpanel", "analytics", "", `(?i)<host>cdn\.mxpnl\.com<end>|mixpanel\.init\(`, ""),
	tag("hotjar", "Hotjar", "analytics", "", `(?i)<host>static\.hotjar\.com<end>`, ""),
	tag("clarity", "Microsoft Clarity", "analytics", "", `(?i)<host>clarity\.ms/tag<end>`, ""),
	tag("plausible", "Plausible", "analytics", "", `(?i)<host>plausible\.io/js<end>`, ""),
	tag("fathom", "Fathom", "analytics", "", `(?i)<host>cdn\.usefathom\.com<end>`, ""),
	tag("heap", "Heap", "analytics", "", `(?i)<host>cdn\.heapanalytics\.com<end>`, ""),
	tag("vercel_analytics", "Vercel Analytics", "analytics", "", `(?i)/_vercel/insights`, ""),
	tag("apple_search_ads", "Apple Search Ads attribution", "attribution", "paid_apple", `(?i)<host>searchads\.apple\.com<end>`, ""),
	tag("appsflyer", "AppsFlyer", "attribution", "app_install_ads", `(?i)<host>onelink\.me<end>|<host>appsflyer\.com<end>`, ""),
	tag("branch", "Branch", "attribution", "app_install_ads", `(?i)<host>cdn\.branch\.io<end>|<subdomain>\.app\.link/<end>`, ""),
	tag("adjust", "Adjust", "attribution", "app_install_ads", `(?i)<host>\badj\.st/<end>|<host>app\.adjust\.com<end>`, ""),
	tag("singular", "Singular", "attribution", "app_install_ads", `(?i)<host>sng\.link<end>`, ""),
	tag("rewardful", "Rewardful", "affiliate", "affiliate", `(?i)<host>r\.wdfl\.co<end>|<host>rewardful\.com<end>`, ""),
	tag("firstpromoter", "FirstPromoter", "affiliate", "affiliate", `(?i)<host>firstpromoter\.com<end>`, ""),
	tag("partnerstack", "PartnerStack", "affiliate", "affiliate", `(?i)<host>partnerstack\.com<end>`, ""),
	tag("tolt", "Tolt", "affiliate", "affiliate", `(?i)<host>cdn\.tolt\.io<end>`, ""),
	tag("impact", "impact.com", "affiliate", "affiliate", `(?i)impactradius|impact-ad\.|utt\.impactcdn`, ""),
	tag("dub", "Dub Partners", "affiliate", "affiliate", `(?i)<host>dubcdn\.com<end>|@dub/analytics`, ""),
	tag("growsurf", "GrowSurf", "affiliate", "affiliate", `(?i)<host>growsurf\.com<end>`, ""),
	tag("viral_loops", "Viral Loops", "affiliate", "affiliate", `(?i)<host>viral-loops\.com<end>|<host>viralloops\.com<end>`, ""),
	tag("hubspot", "HubSpot", "email", "lifecycle", `(?i)<host>js\.hs-scripts\.com<end>|<host>js\.hsforms\.net<end>|<host>js\.hs-analytics\.net<end>`, ""),
	tag("klaviyo", "Klaviyo", "email", "lifecycle", `(?i)<host>static\.klaviyo\.com<end>`, ""),
	tag("mailchimp", "Mailchimp", "email", "lifecycle", `(?i)<host>chimpstatic\.com<end>|<host>list-manage\.com<end>`, ""),
	tag("customerio", "Customer.io", "email", "lifecycle", `(?i)<host>assets\.customer\.io<end>`, ""),
	tag("kit", "Kit", "email", "lifecycle", `(?i)<host>convertkit\.com<end>|<subdomain>\.ck\.page<end>`, ""),
	tag("beehiiv", "beehiiv", "email", "lifecycle", `(?i)<host>embeds\.beehiiv\.com<end>|<host>beehiiv\.com/subscribe<end>`, ""),
	tag("loops", "Loops", "email", "lifecycle", `(?i)<host>app\.loops\.so<end>`, ""),
	tag("braze", "Braze", "email", "lifecycle", `(?i)<host>js\.appboycdn\.com<end>|<host>sdk\.iad-\d+\.braze\.com<end>`, ""),
	tag("iterable", "Iterable", "email", "lifecycle", `(?i)<host>iterable\.com/api<end>|<host>js\.iterable\.com<end>`, ""),
	tag("intercom", "Intercom", "chat", "", `(?i)<host>widget\.intercom\.io<end>|<host>js\.intercomcdn\.com<end>`, ""),
	tag("crisp", "Crisp", "chat", "", `(?i)<host>client\.crisp\.chat<end>`, ""),
	tag("drift", "Drift", "chat", "sales_led", `(?i)<host>js\.driftt\.com<end>`, ""),
	tag("zendesk", "Zendesk", "chat", "", `(?i)<host>static\.zdassets\.com<end>`, ""),
	tag("rb2b", "RB2B", "visitor_id", "sales_led", `(?i)<host>rb2b\.com<end>`, ""),
	tag("warmly", "Warmly", "visitor_id", "sales_led", `(?i)<host>warmly\.ai<end>`, ""),
	tag("koala", "Koala", "visitor_id", "sales_led", `(?i)<host>getkoala\.com<end>`, ""),
	tag("clearbit", "Clearbit Reveal", "visitor_id", "sales_led", `(?i)<host>tag\.clearbitscripts\.com<end>|<host>reveal\.clearbit\.com<end>`, ""),
	tag("6sense", "6sense", "visitor_id", "sales_led", `(?i)<host>j\.6sc\.co<end>`, ""),
	tag("demandbase", "Demandbase", "visitor_id", "sales_led", `(?i)<host>tag\.demandbase\.com<end>`, ""),
	tag("leadfeeder", "Leadfeeder", "visitor_id", "sales_led", `(?i)lftracker|<host>sc\.lfeeder\.com<end>`, ""),
	tag("apollo", "Apollo", "visitor_id", "sales_led", `(?i)<host>assets\.apollo\.io/micro/website-tracker<end>`, ""),
	tag("calendly", "Calendly", "sales", "sales_led", `(?i)<host>calendly\.com/<end>`, ""),
	tag("calcom", "Cal.com", "sales", "sales_led", `(?i)<host>cal\.com/[a-z0-9-]+<end>`, ""),
	tag("chilipiper", "Chili Piper", "sales", "sales_led", `(?i)<host>chilipiper\.com<end>`, ""),
	tag("optimizely", "Optimizely", "experimentation", "", `(?i)<host>cdn\.optimizely\.com<end>`, ""),
	tag("vwo", "VWO", "experimentation", "", `(?i)<host>visualwebsiteoptimizer\.com<end>`, ""),
	tag("statsig", "Statsig", "experimentation", "", `(?i)<host>statsig\.com<end>`, ""),
	tag("growthbook", "GrowthBook", "experimentation", "", `(?i)<host>growthbook\.io<end>`, ""),
	tag("trustpilot", "Trustpilot widget", "reviews", "", `(?i)<host>widget\.trustpilot\.com<end>`, ""),
	tag("g2", "G2 badge", "reviews", "", `(?i)<host>g2\.com/products<end>|<host>g2crowd\.com<end>`, ""),
	tag("senja", "Senja testimonials", "reviews", "", `(?i)<host>senja\.io<end>`, ""),
	tag("testimonial_to", "Testimonial.to", "reviews", "", `(?i)<host>testimonial\.to<end>`, ""),
	tag("producthunt_badge", "Product Hunt badge", "reviews", "launches", `(?i)<host>api\.producthunt\.com/widgets/embed-image<end>`, ""),
	tag("stripe", "Stripe", "payments", "", `(?i)<host>js\.stripe\.com<end>|<host>buy\.stripe\.com<end>`, ""),
	tag("paddle", "Paddle", "payments", "", `(?i)<host>cdn\.paddle\.com<end>`, ""),
	tag("lemonsqueezy", "Lemon Squeezy", "payments", "", `(?i)<host>lemonsqueezy\.com<end>`, ""),
	tag("shopify", "Shopify", "payments", "", `(?i)<host>cdn\.shopify\.com<end>`, ""),
	tag("webflow", "Webflow", "site_builder", "", `(?i)data-wf-site|<host>assets\.website-files\.com<end>|<host>cdn\.prod\.website-files\.com<end>`, ""),
	tag("framer", "Framer", "site_builder", "", `(?i)<host>framerusercontent\.com<end>|<host>framer\.com/m/`, ""),
	tag("wordpress", "WordPress", "site_builder", "", `(?i)/wp-content/`, ""),
	tag("nextjs", "Next.js", "site_builder", "", `(?i)/_next/static/`, ""),
	tag("wix", "Wix", "site_builder", "", `(?i)<host>static\.wixstatic\.com<end>`, ""),
	tag("squarespace", "Squarespace", "site_builder", "", `(?i)<host>static1\.squarespace\.com<end>`, ""),
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
