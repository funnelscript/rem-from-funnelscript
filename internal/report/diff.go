package report

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"remgo/internal/model"
)

type Metric struct {
	Name   string   `json:"name"`
	Before *float64 `json:"before"`
	After  *float64 `json:"after"`
	Delta  *float64 `json:"delta"`
}
type Change struct {
	Channel string `json:"channel,omitempty"`
	Before  string `json:"before"`
	After   string `json:"after"`
}
type Diff struct {
	Brand        string          `json:"brand"`
	From         string          `json:"from"`
	To           string          `json:"to"`
	Metrics      []Metric        `json:"metrics"`
	Channels     []Change        `json:"channels"`
	Positioning  *Change         `json:"positioning"`
	Added        []model.Content `json:"added"`
	NoLongerSeen []model.Content `json:"noLongerSeen"`
}

func Compare(a, b model.Bundle) Diff {
	if a.GeneratedAt > b.GeneratedAt {
		a, b = b, a
	}
	d := Diff{Brand: b.Brand, From: a.GeneratedAt, To: b.GeneratedAt, Metrics: []Metric{}, Channels: []Change{}, Added: []model.Content{}, NoLongerSeen: []model.Content{}}
	metrics := [][3]string{
		{"Meta ads in library", "meta_ads", "totalInLibrary"}, {"Meta ads active", "meta_ads", "activeNow"}, {"Meta ads launched in 30 days", "meta_ads", "launchedLast30d"},
		{"Google ads estimate", "google_ads", "estimate"}, {"Apple Search Ads", "apple_ads", "totalAdsLastYear"}, {"TikTok followers", "tiktok", "followers"},
		{"TikTok posts per week", "tiktok", "postsPerWeek"}, {"Instagram followers", "instagram", "followers"}, {"X followers", "x", "followers"},
		{"YouTube subscribers", "youtube", "subscribers"}, {"LinkedIn followers", "linkedin", "followers"}, {"Sitemap URLs", "seo", "totalUrls"},
		{"Content updates in 90 days", "seo", "blogPostsLast90d"}, {"App Store ratings", "appstore", "ratingCount"},
	}
	for _, m := range metrics {
		before, after := metric(a, m[1], m[2]), metric(b, m[1], m[2])
		if before == nil && after == nil {
			continue
		}
		var delta *float64
		if before != nil && after != nil {
			v := math.Round((*after-*before)*100) / 100
			delta = &v
		}
		d.Metrics = append(d.Metrics, Metric{m[0], before, after, delta})
	}
	old, newer := card(a), card(b)
	var channels []string
	for k := range old {
		channels = append(channels, k)
	}
	for k := range newer {
		if _, ok := old[k]; !ok {
			channels = append(channels, k)
		}
	}
	slices.Sort(channels)
	for _, k := range channels {
		if old[k] != newer[k] {
			d.Channels = append(d.Channels, Change{k, old[k], newer[k]})
		}
	}
	before, after := pitch(a), pitch(b)
	if before != "" && after != "" && before != after {
		d.Positioning = &Change{Before: before, After: after}
	}
	d.Added = missingContent(a, b, false)
	d.NoLongerSeen = missingContent(b, a, true)
	return d
}

func metric(b model.Bundle, source, key string) *float64 {
	for _, e := range b.Evidence {
		if e.Source != source {
			continue
		}
		if n, ok := number(dataMap(e.Data)[key]); ok {
			return &n
		}
	}
	return nil
}

func pitch(b model.Bundle) string {
	for _, e := range b.Evidence {
		if e.Source != "website" || !strings.HasPrefix(strings.ToLower(e.Title), "homepage") {
			continue
		}
		d := dataMap(e.Data)
		if h := dataSlice(d["h1"]); len(h) > 0 {
			return fmt.Sprint(h[0])
		}
		if h, ok := d["h1"].(string); ok && h != "" {
			return h
		}
		if title, ok := d["title"].(string); ok {
			return title
		}
	}
	return ""
}

func card(b model.Bundle) map[string]string {
	m := make(map[string]string, len(b.Scorecard))
	for _, c := range b.Scorecard {
		m[c.Channel] = fmt.Sprintf("%s %d/3", c.Status, c.Strength)
	}
	return m
}

func missingContent(reference, other model.Bundle, requireChecked bool) []model.Content {
	ids := make(map[string]bool, len(reference.Content))
	checked := make(map[string]bool)
	for _, c := range reference.Content {
		ids[c.Platform+"\x00"+c.ID] = true
		checked[c.Platform] = true
	}
	for _, e := range reference.Evidence {
		checked[e.Source] = true
	}
	out := []model.Content{}
	for _, c := range other.Content {
		if c.ID == "" || ids[c.Platform+"\x00"+c.ID] || requireChecked && !checked[c.Platform] {
			continue
		}
		out = append(out, c)
	}
	return out
}

func RenderDiff(d Diff) string {
	var w strings.Builder
	fmt.Fprintf(&w, "# Changes for %s\n\n%s to %s\n\n", Clean(d.Brand), d.From, d.To)
	if d.Positioning != nil {
		fmt.Fprintf(&w, "## Positioning\n\n- Before · %s\n- After · %s\n\n", Clean(d.Positioning.Before), Clean(d.Positioning.After))
	}
	w.WriteString("## Metrics\n\n| Metric | Before | After | Change |\n|---|---|---|---|\n")
	val := func(n *float64) string {
		if n == nil {
			return "unknown"
		}
		return fmt.Sprintf("%g", *n)
	}
	for _, m := range d.Metrics {
		fmt.Fprintf(&w, "| %s | %s | %s | %s |\n", Clean(m.Name), val(m.Before), val(m.After), val(m.Delta))
	}
	w.WriteString("\n## Channel changes\n\n")
	for _, c := range d.Channels {
		fmt.Fprintf(&w, "- %s · %s to %s\n", Clean(c.Channel), Clean(c.Before), Clean(c.After))
	}
	fmt.Fprintf(&w, "\n## Content sample changes\n\n%d newly observed, %d no longer seen in a checked sample.\n", len(d.Added), len(d.NoLongerSeen))
	for _, c := range d.Added {
		if u := SafeURL(c.URL); u != "" {
			fmt.Fprintf(&w, "- [%s](%s) · %s\n", Clean(c.Platform), u, Clean(c.Title))
		}
	}
	return w.String()
}
