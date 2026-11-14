package report

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

//go:embed harness/*.md harness/*.json harness/playbooks/*.md
var harness embed.FS

var meta = func() model.Harness {
	h := sha256.New()
	_ = fs.WalkDir(harness, "harness", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := harness.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(b))
		h.Write(b)
		return nil
	})
	return model.Harness{Version: "3", SHA: fmt.Sprintf("%x", h.Sum(nil))}
}()

func Harness() model.Harness { return meta }

func Playbook() string {
	var parts []string
	for _, f := range []string{"system.md", "guardrails.md", "report-template.md", "lessons.md"} {
		b, _ := harness.ReadFile("harness/" + f)
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n\n")
}

func NamedPlaybook(name string) string {
	if strings.ContainsAny(name, "/\\.") {
		return ""
	}
	b, _ := harness.ReadFile("harness/playbooks/" + name + ".md")
	return string(b)
}

type rule struct{ channel, source string }

var rules = []rule{
	{"paid_meta", "meta_ads"}, {"paid_google", "google_ads"}, {"paid_linkedin", "linkedin_ads"}, {"paid_reddit", "reddit_ads"},
	{"paid_tiktok", "tiktok_ads"}, {"paid_apple", "apple_ads"}, {"organic_tiktok", "tiktok"}, {"organic_instagram", "instagram"},
	{"organic_youtube", "youtube"}, {"organic_x", "x"}, {"organic_linkedin", "linkedin"}, {"seo", "seo"}, {"geo", "seo"},
	{"founder_led", "founders"}, {"reddit_marketing", "reddit"}, {"community", "hackernews"}, {"launches", "hackernews"},
	{"open_source", "github"}, {"app_store", "appstore"}, {"lifecycle", "tracking"}, {"affiliate", "website"}, {"sales_led", "website"},
}

func Score(b model.Bundle) []model.ChannelScore {
	out := make([]model.ChannelScore, 0, len(rules))
	for _, r := range rules {
		row := model.ChannelScore{Channel: r.channel, Status: "unknown", Signal: "No confirming evidence", EvidenceIDs: []string{}}
		for _, u := range b.Unknowns {
			if u.Channel == r.channel || u.Channel == r.source {
				row.Signal = u.Reason
				break
			}
		}
		for _, e := range b.Evidence {
			matchSource := e.Source == r.source
			if !matchSource {
				if r.channel == "founder_led" && e.Source == "wikidata" && len(dataSlice(dataMap(e.Data)["founders"])) > 0 {
					matchSource = true
				} else if (r.channel == "community" || r.channel == "reddit_marketing") && (e.Source == "reddit" || e.Source == "hackernews" || e.Source == "github") {
					matchSource = true
				} else if r.channel == "launches" && (e.Source == "hackernews" || e.Source == "github") {
					matchSource = true
				} else if r.channel == "open_source" && e.Source == "github" {
					matchSource = true
				}
			}
			if !matchSource {
				continue
			}
			status, strength := classify(r.channel, e)
			if status == "" {
				continue
			}
			row.Status, row.Strength, row.Signal, row.EvidenceIDs = status, strength, e.Title, []string{e.ID}
			if status == "active" {
				break
			}
		}
		if row.Status == "unknown" {
			if signal := trackingSignal(r.channel, b.Evidence); signal != nil {
				row.Status, row.Strength, row.Signal, row.EvidenceIDs = "likely", 1, "Tracking signal only: "+signal.Title, []string{signal.ID}
			}
		}
		out = append(out, row)
	}
	return out
}

func classify(channel string, e model.Evidence) (string, int) {
	title := strings.ToLower(e.Title)
	d := dataMap(e.Data)
	if channel == "geo" && !strings.Contains(title, "ai-search") && !strings.Contains(title, "robots") && !strings.Contains(title, "llms.txt") {
		return "", 0
	}
	if channel == "seo" && !strings.Contains(title, "sitemap") && !strings.Contains(title, "google results") {
		return "", 0
	}
	if channel == "affiliate" {
		if !strings.Contains(title, "affiliate") {
			return "", 0
		}
		return "likely", 1
	}
	if channel == "sales_led" {
		if d["contactSales"] == true {
			return "likely", 1
		}
		return "", 0
	}
	if channel == "lifecycle" {
		for _, tag := range dataSlice(d["tags"]) {
			cat, _ := dataMap(tag)["category"].(string)
			if cat == "email" || cat == "crm" || cat == "lifecycle" {
				return "likely", 1
			}
		}
		return "", 0
	}
	if strings.HasPrefix(channel, "paid_") {
		if channel == "paid_apple" && d["appId"] == nil {
			return "", 0
		}
		activeMeasured := false
		for _, key := range []string{"activeNow", "shownLast30d", "stillServing", "currentlyServing", "active"} {
			if n, ok := number(d[key]); ok {
				if n > 0 {
					return "active", strength(n)
				}
				activeMeasured = true
			}
		}
		for _, key := range []string{"activeInSample", "liveInSample"} {
			if n, ok := number(d[key]); ok && n > 0 {
				return "active", strength(n)
			}
		}
		for _, key := range []string{"totalInLibrary", "totalAdsLastYear", "estimate", "total"} {
			if n, ok := number(d[key]); ok {
				if n == 0 {
					return "none", 0
				}
				if activeMeasured {
					return "dormant", strength(n)
				}
				return "likely", strength(n)
			}
		}
		for _, key := range []string{"matched", "sampled"} {
			if n, ok := number(d[key]); ok {
				if n == 0 {
					return "none", 0
				}
				return "likely", strength(n)
			}
		}
	}
	if channel == "open_source" {
		if e.Source != "github" {
			return "", 0
		}
		if stars, ok := number(d["stars"]); ok && stars > 0 {
			return "active", strength(stars)
		}
		return "likely", 1
	}
	if (channel == "reddit_marketing" || channel == "community") && e.Source == "reddit" {
		if n, ok := number(d["matchedThreads"]); ok {
			if n == 0 {
				return "none", 0
			}
			score, _ := number(d["totalScore"])
			return "active", strength(score)
		}
	}
	if n, ok := number(d["matchedThreads"]); ok && n == 0 {
		return "none", 0
	}
	if strings.HasPrefix(title, "no ") || strings.Contains(title, "none found") || strings.Contains(title, "no ads") {
		return "none", 0
	}
	if n, ok := number(d["lastPostDaysAgo"]); ok && n >= 90 {
		return "dormant", 1
	}
	if n, ok := number(d["postsPerWeek"]); ok && n >= 1 {
		return "active", strength(n)
	}
	if n, ok := number(d["totalUrls"]); ok && channel == "seo" {
		if n == 0 {
			return "none", 0
		}
		return "active", strength(n)
	}
	return "likely", 1
}

func strength(n float64) int {
	switch {
	case n >= 50:
		return 3
	case n >= 10:
		return 2
	default:
		return 1
	}
}

func trackingSignal(channel string, evidence []model.Evidence) *model.Evidence {
	patterns := map[string][]string{
		"paid_meta": {"meta", "facebook"}, "paid_google": {"google_ads", "google ads"}, "paid_linkedin": {"linkedin"},
		"paid_reddit": {"reddit"}, "paid_tiktok": {"tiktok"},
	}
	if len(patterns[channel]) == 0 {
		return nil
	}
	for _, e := range evidence {
		if e.Source != "tracking" {
			continue
		}
		d := dataMap(e.Data)
		for _, k := range []string{"pixels", "tags"} {
			raw, _ := json.Marshal(d[k])
			for _, p := range patterns[channel] {
				if strings.Contains(strings.ToLower(string(raw)), p) {
					return &e
				}
			}
		}
	}
	return nil
}

func dataMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

func dataSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var s []any
	if json.Unmarshal(b, &s) != nil {
		return nil
	}
	return s
}

func number(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(fmt.Sprint(v), ",", ""), 64)
	return n, err == nil && !math.IsInf(n, 0) && !math.IsNaN(n)
}

var escapeMD = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\\", "\\\\", "|", "\\|", "[", "\\[", "]", "\\]", "\r", " ", "\n", " ")

func Clean(s string) string { return escapeMD.Replace(strings.TrimSpace(s)) }

func SafeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return strings.NewReplacer("(", "%28", ")", "%29", "<", "%3C", ">", "%3E").Replace(u.String())
}

func Markdown(b model.Bundle) string {
	var w strings.Builder
	fmt.Fprintf(&w, "# %s marketing evidence\n\n<!-- remgo:deterministic -->\n\nTarget %s · collected %s · harness %s\n\n", Clean(b.Brand), Clean(b.Target.Input), b.GeneratedAt, b.Harness.SHA)
	fmt.Fprintf(&w, "## Summary\n\n%d evidence items, %d unknowns, %d content items\n\n", len(b.Evidence), len(b.Unknowns), len(b.Content))
	w.WriteString("## Channel scorecard\n\n| Channel | Status | Signal | Evidence |\n|---|---|---|---|\n")
	for _, c := range b.Scorecard {
		fmt.Fprintf(&w, "| %s | %s | %s | %s |\n", Clean(strings.ReplaceAll(c.Channel, "_", " ")), Clean(c.Status), Clean(c.Signal), references(c.EvidenceIDs))
	}
	w.WriteString("\n## Evidence\n\n")
	for _, e := range b.Evidence {
		fmt.Fprintf(&w, "- [%s] %s · %s", e.ID, Clean(e.Title), Clean(e.Kind))
		if u := SafeURL(e.URL); u != "" {
			fmt.Fprintf(&w, " · [source](%s)", u)
		}
		w.WriteByte('\n')
		for _, line := range describe(e) {
			fmt.Fprintf(&w, "  - %s\n", line)
		}
	}
	if len(b.Evidence) == 0 {
		w.WriteString("No evidence was collected for this run.\n")
	}
	w.WriteString("\n## Unknowns\n\n")
	for _, u := range b.Unknowns {
		fmt.Fprintf(&w, "- %s · %s", Clean(u.Channel), Clean(u.Reason))
		if link := SafeURL(u.ManualCheck); link != "" {
			fmt.Fprintf(&w, " · [check manually](%s)", link)
		}
		w.WriteByte('\n')
	}
	if len(b.Unknowns) == 0 {
		w.WriteString("No unknowns recorded for the selected sources.\n")
	}
	if len(b.Sources) > 0 {
		w.WriteString("\n## Source status\n\n")
		for _, s := range b.Sources {
			fmt.Fprintf(&w, "- %s · %s", Clean(s.Source), Clean(s.Status))
			if s.Reason != "" {
				fmt.Fprintf(&w, " · %s", Clean(s.Reason))
			}
			w.WriteByte('\n')
		}
	}
	return w.String()
}

func describe(e model.Evidence) []string {
	fields := map[string][]string{
		"website":    {"h1", "ctas", "prices", "freePlan", "freeTrial", "noCreditCard", "moneyBack", "annualDiscount", "soc2Certified", "contactSales", "growthRoles"},
		"seo":        {"totalUrls", "sections", "patterns", "llmsTxt", "blogPostsLast90d"},
		"tracking":   {"pixels", "tags", "gtmContainersScanned"},
		"meta_ads":   {"totalInLibrary", "activeNow", "launchedLast30d", "formats"},
		"google_ads": {"estimate", "shownLast30d", "formats"},
		"apple_ads":  {"totalAdsLastYear", "active"},
		"wayback":    {"firstSeen", "monthlySnapshots", "history"},
		"appstore":   {"ratingCount", "rating", "installs", "seller"},
		"reddit":     {"matchedThreads", "totalScore", "subreddits"},
		"github":     {"fullName", "stars", "forks", "topics", "license", "pushedAt"},
	}
	d := dataMap(e.Data)
	var out []string
	for _, key := range fields[e.Source] {
		v, ok := d[key]
		if !ok || v == nil {
			continue
		}
		raw, err := json.Marshal(v)
		if err != nil {
			continue
		}
		var normalized any
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		if decoder.Decode(&normalized) == nil {
			raw, _ = json.Marshal(normalized)
		}
		text := []rune(string(raw))
		if len(text) > 240 {
			text = append(text[:240], '…')
		}
		out = append(out, Clean(key)+" · "+Clean(string(text)))
	}
	return out
}

func references(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return "[" + strings.Join(ids, ", ") + "]"
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}
type Validation struct {
	OK     bool    `json:"ok"`
	Score  float64 `json:"score"`
	Checks []Check `json:"checks"`
}

var (
	citation = regexp.MustCompile(`\[(E\d+(?:\s*,\s*E\d+)*)\]`)
	heading  = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	numbered = regexp.MustCompile(`^\s*\d+[.)]\s`)
	bullet   = regexp.MustCompile(`^\s*[-*]\s`)
	spend    = regexp.MustCompile(`(?i)(?:\$\s?\d[\d,.]*\s?[km]?(?:\s*/\s?(?:mo|month|day))?\s*(?:(?:in|of|on)\s+)?(?:ad\s+)?spend|(?:ad(?:vertising)?\s+)?spend\s*(?:of|is|was|:)?\s*\$\s?\d)`)
)

func Validate(md string, b model.Bundle) Validation {
	rendered := strings.TrimSpace(Markdown(b))
	current := strings.TrimSpace(md)
	if current == rendered {
		return validateMarkdown(current, b, true)
	}
	if narrative, ok := strings.CutSuffix(current, "\n\n---\n\n"+rendered); ok {
		return ValidateNarrative(narrative, b)
	}
	return ValidateNarrative(md, b)
}

func validateMarkdown(md string, b model.Bundle, deterministic bool) Validation {
	known := make(map[string]bool, len(b.Evidence))
	for _, e := range b.Evidence {
		known[e.ID] = true
	}
	var dangling []string
	cited := 0
	for _, m := range citation.FindAllStringSubmatch(md, -1) {
		for _, id := range strings.Split(m[1], ",") {
			cited++
			if id = strings.TrimSpace(id); !known[id] {
				dangling = append(dangling, id)
			}
		}
	}
	has := func(want string) bool {
		for _, h := range heading.FindAllStringSubmatch(md, -1) {
			if strings.Contains(strings.ToLower(h[1]), want) {
				return true
			}
		}
		return false
	}
	checks := []Check{
		{"citations resolve", len(dangling) == 0, fmt.Sprintf("%d citations; unresolved: %s", cited, strings.Join(dangling, ", "))},
		{"no unverified spend figures", deterministic || !spend.MatchString(md), "public evidence does not establish ad spend"},
		{"unknowns reported", len(b.Unknowns) == 0 || has("unknowns"), fmt.Sprintf("%d unknowns in evidence", len(b.Unknowns))},
	}
	if deterministic {
		checks = append(checks, Check{"evidence report sections", has("channel scorecard") && has("evidence"), "scorecard and evidence index"})
	} else {
		var missing []string
		for _, want := range []string{"tl;dr", "channel scorecard", "the engine", "paid", "organic social", "seo", "founders", "reddit", "funnel", "steal this playbook", "unknowns"} {
			if !has(want) {
				missing = append(missing, want)
			}
		}
		checks = append(checks, Check{"required sections", len(missing) == 0, strings.Join(missing, ", ")})
		checkLines := func(name string, pattern *regexp.Regexp, minimum int, threshold float64) Check {
			section := false
			total, grounded := 0, 0
			for _, line := range strings.Split(md, "\n") {
				if h := heading.FindStringSubmatch(line); h != nil {
					section = strings.Contains(strings.ToLower(h[1]), name)
					continue
				}
				if section && pattern.MatchString(line) {
					total++
					if citation.MatchString(line) {
						grounded++
					}
				}
			}
			return Check{name + " grounded", total >= minimum && grounded >= int(math.Ceil(float64(total)*threshold)), fmt.Sprintf("%d/%d lines cite evidence", grounded, total)}
		}
		checks = append(checks, checkLines("tl;dr", bullet, 3, 1), checkLines("steal this playbook", numbered, 5, .8))
	}
	pass := 0
	for _, c := range checks {
		if c.OK {
			pass++
		}
	}
	return Validation{OK: pass == len(checks), Score: math.Round(100*float64(pass)/float64(len(checks))) / 100, Checks: checks}
}

func ValidateNarrative(md string, b model.Bundle) Validation {
	return validateMarkdown(md, b, false)
}
