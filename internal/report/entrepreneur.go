package report

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/collect"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

var (
	grievanceKeywords = regexp.MustCompile(`(?i)\b(slow|lag|bloat|crash|freez|expensive|price hike|pricing increase|paywall|forced|complicated|clunky|unusable|confus|broken|sync fail|data loss|buggy|bad support|awful|switched to|deal breaker|worst)\b`)

	themePatterns = map[string]*regexp.Regexp{
		"Performance & Speed":          regexp.MustCompile(`(?i)\b(slow|lag|bloat|crash|freez|heavy|loading|sluggish)\b`),
		"Pricing & Monetization":       regexp.MustCompile(`(?i)\b(expensive|price|pricing|paywall|per seat|forced upgrade|billing|cost)\b`),
		"Complexity & Clunky UI":       regexp.MustCompile(`(?i)\b(complicated|clunky|unusable|confus|steep learning|mess|chaos|bloated)\b`),
		"Sync & Integration Stability": regexp.MustCompile(`(?i)\b(sync|integration|api|rate limit|disconnected|data loss|webhook|broken)\b`),
		"Support & Developer Trust":    regexp.MustCompile(`(?i)\b(support|service|response|ignored|awful|deal breaker|cancelled|left)\b`),
	}
)

// AnalyzeEntrepreneur applies the Replica-Square Entrepreneur methodology to mine verified competitor
// grievances, score their Bayesian time-decay impact, and generate a Day-1 Challenger Displacement Matrix.
func AnalyzeEntrepreneur(b model.Bundle) model.EntrepreneurAnalysis {
	var analysis model.EntrepreneurAnalysis

	// 1. Creative angles deconstruction via REA
	analysis.CreativeAngles = collect.DeconstructCreative(b.Content, b.Evidence)

	// 2. Extract verified customer grievances across evidence items
	now := time.Now().UTC()
	if b.GeneratedAt != "" {
		if t, err := time.Parse(time.RFC3339, b.GeneratedAt); err == nil {
			now = t
		}
	}

	themeScores := make(map[string]float64)
	themeQuotes := make(map[string][]string)
	themeSources := make(map[string]map[string]bool)
	themeEvidence := make(map[string]map[string]bool)

	for _, e := range b.Evidence {
		textList := extractTextFromEvidence(e)
		for _, text := range textList {
			if !grievanceKeywords.MatchString(text) {
				continue
			}

			// Bayesian scoring with 540-day time decay
			baseScore := 0.5 // Unrated forum / review default
			daysDiff := 30.0 // Default 30 days if date is unspecified
			if e.ObservedAt != "" {
				if obsTime, err := time.Parse("2006-01-02", e.ObservedAt[:min(10, len(e.ObservedAt))]); err == nil {
					days := now.Sub(obsTime).Hours() / 24
					if days > 0 {
						daysDiff = days
					}
				}
			}

			timeWeight := math.Pow(0.5, daysDiff/540.0)
			score := baseScore * timeWeight

			// Categorize into themes
			for theme, pat := range themePatterns {
				if pat.MatchString(text) {
					themeScores[theme] += score
					if themeSources[theme] == nil {
						themeSources[theme] = make(map[string]bool)
						themeEvidence[theme] = make(map[string]bool)
					}
					themeSources[theme][e.Source] = true
					if e.ID != "" {
						themeEvidence[theme][e.ID] = true
					}

					cleanQuote := cleanVerbatimQuote(text)
					if cleanQuote != "" && len(themeQuotes[theme]) < 3 {
						themeQuotes[theme] = append(themeQuotes[theme], cleanQuote)
					}
				}
			}
		}
	}

	// 3. Build ranked Grievance list
	var themes []string
	for t := range themeScores {
		themes = append(themes, t)
	}
	sort.Slice(themes, func(i, j int) bool {
		return themeScores[themes[i]] > themeScores[themes[j]]
	})

	for _, t := range themes {
		var srcList []string
		for s := range themeSources[t] {
			srcList = append(srcList, s)
		}
		sort.Strings(srcList)

		var evList []string
		for ev := range themeEvidence[t] {
			evList = append(evList, ev)
		}
		sort.Strings(evList)

		analysis.TopGrievances = append(analysis.TopGrievances, model.Grievance{
			Theme:         t,
			Score:         math.Round(themeScores[t]*100) / 100,
			EvidenceCount: len(evList),
			SampleQuotes:  themeQuotes[t],
			Sources:       srcList,
			EvidenceIDs:   evList,
		})
	}

	// 4. Synthesize Day-1 Displacement Playbook
	analysis.Displacement = generateDisplacementPlays(b.Brand, analysis.TopGrievances)

	return analysis
}

func extractTextFromEvidence(e model.Evidence) []string {
	var texts []string
	if e.Title != "" {
		texts = append(texts, e.Title)
	}

	switch data := e.Data.(type) {
	case string:
		texts = append(texts, data)
	case map[string]any:
		for _, val := range data {
			if s, ok := val.(string); ok && s != "" {
				texts = append(texts, s)
			} else if slice, ok := val.([]string); ok {
				texts = append(texts, slice...)
			}
		}
	case []any:
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				for _, val := range m {
					if s, ok := val.(string); ok {
						texts = append(texts, s)
					}
				}
			}
		}
	}
	return texts
}

func cleanVerbatimQuote(raw string) string {
	s := strings.TrimSpace(raw)
	lines := strings.Split(s, "\n")
	s = strings.TrimSpace(lines[0])
	if len([]rune(s)) > 160 {
		s = string([]rune(s)[:157]) + "..."
	}
	return s
}

func generateDisplacementPlays(brand string, grievances []model.Grievance) []model.DisplacementPlay {
	var plays []model.DisplacementPlay
	brandName := brand
	if brandName == "" {
		brandName = "Competitor"
	}

	for _, g := range grievances {
		play := model.DisplacementPlay{
			EvidenceIDs: g.EvidenceIDs,
		}

		switch g.Theme {
		case "Performance & Speed":
			play.TargetWeakness = fmt.Sprintf("%s is weighed down by legacy bloat, causing slow load times and lag.", brandName)
			play.CounterHook = fmt.Sprintf("Frustrated with %s running slow? Switch to instant, keyboard-first speed.", brandName)
			play.AnglePlaybook = "Side-by-side performance comparison ad. Highlight sub-50ms interaction benchmarks vs competitor loading spinners."
			play.RecommendedCTA = "Try instant web demo — no signup or credit card required."
			play.ImpactROI = "High (Viral product-led conversion)"

		case "Pricing & Monetization":
			play.TargetWeakness = fmt.Sprintf("%s forces complex per-seat tiers and paywalls essential features.", brandName)
			play.CounterHook = fmt.Sprintf("Done with %s's surprise price hikes? Get transparent, flat pricing forever.", brandName)
			play.AnglePlaybook = "Direct price comparison calculator. Show total annual savings and zero per-seat penalty."
			play.RecommendedCTA = "Lock in flat founding rate with 1-click account migration."
			play.ImpactROI = "High (Immediate economic switching trigger)"

		case "Complexity & Clunky UI":
			play.TargetWeakness = fmt.Sprintf("%s has become bloated with enterprise complexity and steep learning curves.", brandName)
			play.CounterHook = fmt.Sprintf("Software shouldn't feel like a chore. The clean, modern alternative to %s.", brandName)
			play.AnglePlaybook = "Clean UI walkthrough video comparing intuitive 2-click workflows against competitor's multi-step maze."
			play.RecommendedCTA = "Get started in 30 seconds with zero onboarding friction."
			play.ImpactROI = "Fast-Win (Appeals to disenfranchised daily operators)"

		case "Sync & Integration Stability":
			play.TargetWeakness = fmt.Sprintf("%s suffers from brittle integrations and unexpected sync failures.", brandName)
			play.CounterHook = fmt.Sprintf("Tired of broken %s integrations? Bulletproof sync that never drops data.", brandName)
			play.AnglePlaybook = "Reliability proof dossier. Emphasize offline-first local cache and 99.99% webhook delivery guarantees."
			play.RecommendedCTA = "Free migration audit + 60-day money-back guarantee."
			play.ImpactROI = "Strategic (Captures mission-critical enterprise workflows)"

		default:
			play.TargetWeakness = fmt.Sprintf("Customer dissatisfaction with %s service responsiveness and support.", brandName)
			play.CounterHook = fmt.Sprintf("Experience real human support that actually solves your problems. Meet the %s alternative.", brandName)
			play.AnglePlaybook = "Social proof compilation highlighting disgruntled customer switch stories with verbatim quotes."
			play.RecommendedCTA = "Book a dedicated 1-on-1 migration concierge call."
			play.ImpactROI = "Medium (Trust and relationship building)"
		}

		plays = append(plays, play)
	}

	return plays
}
